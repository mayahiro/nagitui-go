package widget

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	celltext "github.com/mayahiro/nagi-go/text"
)

func TestCodeLayoutFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t, "widgets/code-layout.txt", "widget-code-layout",
		"input", "profile", "viewport", "tab", "wrap", "line-numbers",
		"expected-gutter", "expected-code-width", "expected-rows",
		"expected-widths", "expected-continuations",
	)
	for _, record := range records {
		profile := celltext.ModernWidth()
		if record.Field("profile") == "cjk" {
			profile = celltext.CJKWidth()
		} else if record.Field("profile") != "modern" {
			t.Fatalf("case %s: invalid profile %q", record.ID, record.Field("profile"))
		}
		line, err := NewCodeLine(record.Text("input"))
		if err != nil {
			t.Fatalf("case %s: line: %v", record.ID, err)
		}
		document, err := NewCodeDocument([]CodeLine{line}, false)
		if err != nil {
			t.Fatalf("case %s: document: %v", record.ID, err)
		}
		layout, err := NewCodeLayout(document, DefaultCodeLayoutOptions().
			WithViewportWidth(uint32(fixtureUint64(t, record.Field("viewport")))).
			WithTabWidth(uint8(fixtureUint64(t, record.Field("tab")))).
			WithWrap(fixtureBool(t, record.Field("wrap"))).
			WithLineNumbers(fixtureBool(t, record.Field("line-numbers"))).
			WithWidthProfile(profile))
		if err != nil {
			t.Fatalf("case %s: layout: %v", record.ID, err)
		}
		if expected := uint32(fixtureUint64(t, record.Field("expected-gutter"))); layout.GutterWidth() != expected {
			t.Errorf("case %s: gutter = %d, want %d", record.ID, layout.GutterWidth(), expected)
		}
		if expected := uint32(fixtureUint64(t, record.Field("expected-code-width"))); layout.CodeWidth() != expected {
			t.Errorf("case %s: code width = %d, want %d", record.ID, layout.CodeWidth(), expected)
		}
		rows := make([]string, 0, layout.VisualRowCount())
		widths := make([]string, 0, layout.VisualRowCount())
		continuations := make([]string, 0, layout.VisualRowCount())
		for index := 0; index < layout.VisualRowCount(); index++ {
			row, _ := layout.row(index)
			var text strings.Builder
			for _, span := range row.spans {
				text.WriteString(span.Text)
			}
			rows = append(rows, text.String())
			widths = append(widths, strconv.FormatUint(uint64(row.width), 10))
			continuations = append(continuations, strconv.FormatBool(row.continuation))
		}
		if actual, expected := strings.Join(rows, "|"), record.Text("expected-rows"); actual != expected {
			t.Errorf("case %s: rows = %q, want %q", record.ID, actual, expected)
		}
		if actual, expected := strings.Join(widths, ","), record.Field("expected-widths"); actual != expected {
			t.Errorf("case %s: widths = %q, want %q", record.ID, actual, expected)
		}
		if actual, expected := strings.Join(continuations, ","), record.Field("expected-continuations"); actual != expected {
			t.Errorf("case %s: continuations = %q, want %q", record.ID, actual, expected)
		}
	}
}

func TestCodeViewFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t, "widgets/code-view.txt", "widget-code-view",
		"cursor", "anchor", "offset", "wrap", "action", "step",
		"expected-cursor", "expected-anchor", "expected-offset", "expected-copy",
		"expected-lines", "expected-copy-lines", "expected-bytes",
	)
	for _, record := range records {
		layout := testCodeLayout(t, []string{"a", "abcdefgh", "日", "last"}, fixtureBool(t, record.Field("wrap")), 4)
		cursor := fixtureInt(t, record.Field("cursor"))
		state := NewCodeViewState(cursor)
		if record.Field("anchor") != "-" {
			state = NewCodeViewStateWithSelection(cursor, fixtureInt(t, record.Field("anchor")))
		}
		state = state.WithHorizontalOffset(uint32(fixtureUint64(t, record.Field("offset"))))
		var copyKind *CodeCopyKind
		action := record.Field("action")
		if action != "normalize" && action != "copy-selection" && action != "copy-document" {
			state = codeViewStateForAction(
				layout, state, fixtureCodeViewAction(t, action),
				uint32(fixtureUint64(t, record.Field("step"))),
			)
		} else if action == "copy-selection" {
			kind := CodeCopySelection
			copyKind = &kind
		} else if action == "copy-document" {
			kind := CodeCopyDocument
			copyKind = &kind
		}
		state = normalizeCodeViewState(layout, state)
		if expected := fixtureInt(t, record.Field("expected-cursor")); state.Cursor() != expected {
			t.Errorf("case %s: cursor = %d, want %d", record.ID, state.Cursor(), expected)
		}
		anchorText := "-"
		if anchor, ok := state.SelectionAnchor(); ok {
			anchorText = strconv.Itoa(anchor)
		}
		if anchorText != record.Field("expected-anchor") {
			t.Errorf("case %s: anchor = %s, want %s", record.ID, anchorText, record.Field("expected-anchor"))
		}
		if expected := uint32(fixtureUint64(t, record.Field("expected-offset"))); state.HorizontalOffset() != expected {
			t.Errorf("case %s: offset = %d, want %d", record.ID, state.HorizontalOffset(), expected)
		}
		start, end := state.SelectedLines()
		if actual := fmt.Sprintf("%d:%d", start, end); actual != record.Field("expected-lines") {
			t.Errorf("case %s: lines = %s, want %s", record.ID, actual, record.Field("expected-lines"))
		}
		if copyKind != nil {
			context := &codeViewActionContext[struct{}]{id: "code", layout: layout, state: state}
			request, ok := codeViewCopyRequest(*copyKind, context)
			if !ok {
				t.Fatalf("case %s: copy unavailable", record.ID)
			}
			if request.Text != record.Text("expected-copy") {
				t.Errorf("case %s: copy = %q, want %q", record.ID, request.Text, record.Text("expected-copy"))
			}
			if actual := fmt.Sprintf("%d:%d", request.LineStart, request.LineEnd); actual != record.Field("expected-copy-lines") {
				t.Errorf("case %s: copy lines = %s, want %s", record.ID, actual, record.Field("expected-copy-lines"))
			}
			if actual := fmt.Sprintf("%d:%d", request.ByteStart, request.ByteEnd); actual != record.Field("expected-bytes") {
				t.Errorf("case %s: bytes = %s, want %s", record.ID, actual, record.Field("expected-bytes"))
			}
		} else if record.Field("expected-copy") != "-" || record.Field("expected-copy-lines") != "-" || record.Field("expected-bytes") != "-" {
			t.Errorf("case %s: unexpected copy expectation", record.ID)
		}
	}
}

func fixtureCodeViewAction(t *testing.T, value string) codeViewAction {
	t.Helper()
	switch value {
	case "previous":
		return codeViewPrevious
	case "next":
		return codeViewNext
	case "first":
		return codeViewFirst
	case "last":
		return codeViewLast
	case "extend-previous":
		return codeViewExtendPrevious
	case "extend-next":
		return codeViewExtendNext
	case "extend-first":
		return codeViewExtendFirst
	case "extend-last":
		return codeViewExtendLast
	case "horizontal-previous":
		return codeViewHorizontalPrevious
	case "horizontal-next":
		return codeViewHorizontalNext
	case "select-all":
		return codeViewSelectAll
	default:
		t.Fatalf("invalid CodeView action %q", value)
		return codeViewPrevious
	}
}
