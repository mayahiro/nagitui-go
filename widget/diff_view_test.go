package widget

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	tui "github.com/mayahiro/nagitui-go"
)

func TestDiffViewGutterAndSemanticStyles(t *testing.T) {
	layout := sampleDiffLayout(t, false)
	record, _ := layout.codeLayout().row(3)
	line, _ := layout.Document().Line(3)
	oldLine, hasOld := line.OldLine()
	newLine, hasNew := line.NewLine()
	spans := diffViewRowSpans(
		layout, record, line.Kind(), oldLine, hasOld, newLine, hasNew,
		0, false, false, true, DefaultDiffViewStyle(),
	)
	text := ""
	for _, span := range spans {
		text += span.Text
	}
	if text != "2   - old" {
		t.Fatalf("row = %q", text)
	}
	if spans[1].Style.Foreground != vt.IndexedColor(1) {
		t.Fatalf("marker style = %#v", spans[1].Style)
	}
	source := vt.Style{Foreground: vt.IndexedColor(5), Italic: true}
	resolved := diffKindOverlay(source, DiffLineAddition, DefaultDiffViewStyle())
	if resolved.Foreground != vt.IndexedColor(5) || !resolved.Italic {
		t.Fatalf("source override = %#v", resolved)
	}

	wrapped := sampleDiffLayout(t, true)
	continuation, ok := wrapped.codeLayout().row(1)
	if !ok || !continuation.continuation {
		t.Fatalf("metadata continuation = %#v ok=%t", continuation, ok)
	}
	line, _ = wrapped.Document().Line(0)
	spans = diffViewRowSpans(
		wrapped, continuation, line.Kind(), 0, false, 0, false,
		0, false, false, true, DefaultDiffViewStyle(),
	)
	text = ""
	for _, span := range spans {
		text += span.Text
	}
	if !strings.HasPrefix(text, "    > ") {
		t.Fatalf("continuation row = %q", text)
	}
}

func TestDiffViewDescriptorsAndPointer(t *testing.T) {
	layout := sampleDiffLayout(t, false)
	descriptors := diffViewActionDescriptors(true, false, layout, DiffViewState{})
	if descriptors[codeViewCopySelection].Availability() != tui.ActionDisabledPassThrough ||
		descriptors[codeViewCopyDocument].Availability() != tui.ActionDisabledPassThrough {
		t.Fatal("copy descriptors enabled without handler")
	}
	var received DiffViewState
	context := &diffViewActionContext[DiffViewState]{
		id: "diff", layout: layout, state: NewCodeViewState(1), horizontalStep: 4,
		onChange: func(state DiffViewState) DiffViewState { received = state; return state },
	}
	event := vt.Event{Kind: vt.EventMouse, Mouse: vt.MouseEvent{
		Kind: vt.MousePress, Button: vt.MouseLeft, Modifiers: vt.Modifiers{Shift: true},
	}}
	_ = diffViewPointerResult(event, 2, context)
	start, end := received.SelectedLines()
	if start != 1 || end != 3 {
		t.Fatalf("received=%#v", received)
	}
}

func TestDiffViewFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t, "widgets/diff-view.txt", "widget-diff-view",
		"cursor", "anchor", "offset", "wrap", "action", "step",
		"expected-cursor", "expected-anchor", "expected-offset", "expected-lines",
		"expected-copy", "expected-copy-lines", "expected-bytes",
	)
	for _, record := range records {
		layout := sampleDiffLayout(t, fixtureBool(t, record.Field("wrap")))
		cursor := fixtureInt(t, record.Field("cursor"))
		state := NewCodeViewState(cursor)
		if record.Field("anchor") != "-" {
			state = NewCodeViewStateWithSelection(cursor, fixtureInt(t, record.Field("anchor")))
		}
		state = state.WithHorizontalOffset(uint32(fixtureUint64(t, record.Field("offset"))))
		var copyKind *DiffCopyKind
		action := record.Field("action")
		if action != "normalize" && action != "copy-selection" && action != "copy-document" {
			state = codeViewStateForAction(
				layout.codeLayout(), state, fixtureCodeViewAction(t, action),
				uint32(fixtureUint64(t, record.Field("step"))),
			)
		} else if action == "copy-selection" {
			kind := DiffCopySelection
			copyKind = &kind
		} else if action == "copy-document" {
			kind := DiffCopyDocument
			copyKind = &kind
		}
		state = normalizeCodeViewState(layout.codeLayout(), state)
		if state.Cursor() != fixtureInt(t, record.Field("expected-cursor")) {
			t.Errorf("case %s: cursor=%d", record.ID, state.Cursor())
		}
		anchor := "-"
		if value, ok := state.SelectionAnchor(); ok {
			anchor = strconv.Itoa(value)
		}
		if anchor != record.Field("expected-anchor") ||
			state.HorizontalOffset() != uint32(fixtureUint64(t, record.Field("expected-offset"))) {
			t.Errorf("case %s: anchor/offset=%s/%d", record.ID, anchor, state.HorizontalOffset())
		}
		start, end := state.SelectedLines()
		if fmt.Sprintf("%d:%d", start, end) != record.Field("expected-lines") {
			t.Errorf("case %s: lines=%d:%d", record.ID, start, end)
		}
		if copyKind != nil {
			context := &diffViewActionContext[struct{}]{id: "diff", layout: layout, state: state}
			request, ok := diffViewCopyRequest(*copyKind, context)
			if !ok || request.Text != record.Text("expected-copy") ||
				fmt.Sprintf("%d:%d", request.LineStart, request.LineEnd) != record.Field("expected-copy-lines") ||
				fmt.Sprintf("%d:%d", request.ByteStart, request.ByteEnd) != record.Field("expected-bytes") {
				t.Errorf("case %s: request=%#v ok=%t", record.ID, request, ok)
			}
		} else if record.Field("expected-copy") != "-" {
			t.Errorf("case %s: unexpected copy expectation", record.ID)
		}
	}
}
