package widget

import (
	"strings"
	"testing"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
	tui "github.com/mayahiro/nagitui-go"
)

func testCodeLayout(t *testing.T, lines []string, wrap bool, width uint32) CodeLayout {
	t.Helper()
	values := make([]CodeLine, len(lines))
	for index, line := range lines {
		values[index] = mustCodeLine(t, line)
	}
	document, err := NewCodeDocument(values, true)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := NewCodeLayout(document, DefaultCodeLayoutOptions().
		WithViewportWidth(width).WithLineNumbers(false).WithWrap(wrap))
	if err != nil {
		t.Fatal(err)
	}
	return layout
}

func TestCodeViewStateNormalizesSelectionAndOffset(t *testing.T) {
	layout := testCodeLayout(t, []string{"a", "abcdefgh"}, false, 4)
	state := normalizeCodeViewState(
		layout, NewCodeViewStateWithSelection(9, 0).WithHorizontalOffset(99),
	)
	start, end := state.SelectedLines()
	if state.Cursor() != 1 || start != 0 || end != 2 || state.HorizontalOffset() != 4 {
		t.Fatalf("state = %#v range=%d..%d", state, start, end)
	}
	wrapped := testCodeLayout(t, []string{"abcdefgh"}, true, 4)
	if normalizeCodeViewState(wrapped, state).HorizontalOffset() != 0 {
		t.Fatal("wrapped layout retained horizontal offset")
	}
}

func TestCodeViewNavigationAndExtensionAreLineOriented(t *testing.T) {
	layout := testCodeLayout(t, []string{"a", "b", "c"}, false, 4)
	state := codeViewStateForAction(
		layout, NewCodeViewState(1), codeViewExtendNext, 4,
	)
	start, end := state.SelectedLines()
	if state.Cursor() != 2 || start != 1 || end != 3 {
		t.Fatalf("extended = %#v range=%d..%d", state, start, end)
	}
	state = codeViewStateForAction(layout, state, codeViewPrevious, 4)
	if state != NewCodeViewState(1) {
		t.Fatalf("collapsed = %#v", state)
	}
}

func TestCodeViewViewportIsBoundedAndFollowsSelection(t *testing.T) {
	lines := make([]string, 100)
	for index := range lines {
		lines[index] = string(rune('a' + index%26))
	}
	layout := testCodeLayout(t, lines, false, 4)
	window := codeViewVisibleRowWindow(layout, 50, 7)
	if window != (codeIndexRange{start: 47, end: 54}) {
		t.Fatalf("window = %#v", window)
	}
}

func TestCodeViewCellCropPreservesCompleteGraphemes(t *testing.T) {
	spans := []tui.TextSpan{tui.NewTextSpan("A日B", vt.Style{})}
	cropped := sliceCodeSpansByCells(spans, nil, 1, 2, celltext.ModernWidth())
	if len(cropped) != 1 || cropped[0].Text != "日" {
		t.Fatalf("cropped = %#v", cropped)
	}
	insideWide := sliceCodeSpansByCells(spans, nil, 2, 2, celltext.ModernWidth())
	if len(insideWide) != 1 || insideWide[0].Text != "B" {
		t.Fatalf("inside wide = %#v", insideWide)
	}
}

func TestCodeViewNoWrapCropUsesSparseCheckpointsNearLongLineEnd(t *testing.T) {
	layout := testCodeLayout(t, []string{strings.Repeat("a", 1024) + "END"}, false, 4)
	row, _ := layout.row(0)
	if len(row.checkpoints) < 4 {
		t.Fatalf("checkpoints = %d", len(row.checkpoints))
	}
	cropped := sliceCodeSpansByCells(
		row.spans, row.checkpoints, 1024, 3, celltext.ModernWidth(),
	)
	var text strings.Builder
	for _, span := range cropped {
		text.WriteString(span.Text)
	}
	if text.String() != "END" {
		t.Fatalf("cropped = %q", text.String())
	}
}

func TestCodeViewCopyRequestOwnsCompleteSelectedLines(t *testing.T) {
	layout := testCodeLayout(t, []string{"a", "b", "c"}, false, 4)
	context := &codeViewActionContext[struct{}]{
		id: "code", layout: layout, state: NewCodeViewStateWithSelection(1, 0),
		horizontalStep: 4,
	}
	request, ok := codeViewCopyRequest(CodeCopySelection, context)
	if !ok || request.Text != "a\nb" || request.LineStart != 0 || request.LineEnd != 2 ||
		request.ByteStart != 0 || request.ByteEnd != 3 {
		t.Fatalf("request = %#v ok=%t", request, ok)
	}
}

func TestCodeViewPointerShiftExtendsFromExistingAnchor(t *testing.T) {
	layout := testCodeLayout(t, []string{"a", "b", "c"}, false, 4)
	var received CodeViewState
	context := &codeViewActionContext[CodeViewState]{
		id: "code", layout: layout, state: NewCodeViewState(1), horizontalStep: 4,
		onChange: func(state CodeViewState) CodeViewState { received = state; return state },
	}
	event := vt.Event{Kind: vt.EventMouse, Mouse: vt.MouseEvent{
		Kind: vt.MousePress, Button: vt.MouseLeft, Modifiers: vt.Modifiers{Shift: true},
	}}
	_ = codeViewPointerResult(event, 2, context)
	start, end := received.SelectedLines()
	if start != 1 || end != 3 {
		t.Fatalf("received=%#v", received)
	}
}

func TestCodeViewActionDescriptorsDisableCopyWithoutHandler(t *testing.T) {
	layout := testCodeLayout(t, []string{"a"}, false, 4)
	descriptors := codeViewActionDescriptors(true, false, layout, CodeViewState{})
	if descriptors[codeViewCopySelection].Availability() != tui.ActionDisabledPassThrough ||
		descriptors[codeViewCopyDocument].Availability() != tui.ActionDisabledPassThrough {
		t.Fatal("copy descriptors enabled without callback")
	}

	emptyLine := testCodeLayout(t, []string{""}, false, 4)
	descriptors = codeViewActionDescriptors(true, true, emptyLine, CodeViewState{})
	if descriptors[codeViewCopyDocument].Availability() != tui.ActionEnabled {
		t.Fatal("empty line document copy descriptor was disabled")
	}
}

func TestCodeViewRepeatCopyBlocksOnlyEnabledExactBinding(t *testing.T) {
	layout := testCodeLayout(t, []string{"a"}, false, 4)
	selection, document := codeViewCopyActionsEnabled(layout, CodeViewState{}, true)
	event := vt.Event{Kind: vt.EventKey, Key: vt.KeyEvent{
		Code: vt.KeyCharacter, Character: 'c', Action: vt.KeyRepeat,
		Modifiers: vt.Modifiers{Control: true},
	}}
	if !isBlockedCodeViewCopyRepeat(event, selection, document) {
		t.Fatal("enabled selection repeat was not blocked")
	}
	event.Key.Modifiers.Alt = true
	if isBlockedCodeViewCopyRepeat(event, selection, document) {
		t.Fatal("non-exact repeat binding was blocked")
	}
	hidden, err := NewStyledCodeLine([]tui.TextSpan{
		tui.NewTextSpan("secret", vt.Style{Hidden: true}),
	})
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewCodeDocument([]CodeLine{hidden}, false)
	if err != nil {
		t.Fatal(err)
	}
	hiddenLayout, err := NewCodeLayout(source, DefaultCodeLayoutOptions().WithLineNumbers(false))
	if err != nil {
		t.Fatal(err)
	}
	selection, document = codeViewCopyActionsEnabled(hiddenLayout, CodeViewState{}, true)
	event.Key.Modifiers = vt.Modifiers{Control: true}
	if isBlockedCodeViewCopyRepeat(event, selection, document) {
		t.Fatal("disabled hidden copy repeat was blocked")
	}
}

func TestCodeViewDefaultStylesAreAttributeOnly(t *testing.T) {
	style := DefaultCodeViewStyle()
	if !style.LineNumber.Dim || !style.Continuation.Dim || !style.Selection.Reverse ||
		!style.Focused.Underline || !style.Disabled.Dim {
		t.Fatalf("style = %#v", style)
	}
}
