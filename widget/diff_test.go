package widget

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
	tui "github.com/mayahiro/nagitui-go"
)

func mustDiffCodeLine(t *testing.T, value string) CodeLine {
	t.Helper()
	line, err := NewCodeLine(value)
	if err != nil {
		t.Fatal(err)
	}
	return line
}

func mustDiffRange(t *testing.T, start, count uint64) DiffRange {
	t.Helper()
	value, err := NewDiffRange(start, count)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func sampleDiffDocument(t *testing.T) DiffDocument {
	t.Helper()
	hunk := NewDiffHunk(mustDiffRange(t, 1, 3), mustDiffRange(t, 1, 4))
	contextOne, err := NewDiffContextLine(1, 1, mustDiffCodeLine(t, "same"))
	if err != nil {
		t.Fatal(err)
	}
	deletion, err := NewDiffDeletionLine(2, mustDiffCodeLine(t, "old"))
	if err != nil {
		t.Fatal(err)
	}
	addition, err := NewDiffAdditionLine(2, mustDiffCodeLine(t, "new"))
	if err != nil {
		t.Fatal(err)
	}
	more, err := NewDiffAdditionLine(3, mustDiffCodeLine(t, "more"))
	if err != nil {
		t.Fatal(err)
	}
	contextLast, err := NewDiffContextLine(3, 4, mustDiffCodeLine(t, "last"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := NewDiffDocument([]DiffLine{
		NewDiffMetadataLine(mustDiffCodeLine(t, "diff --git a/a.txt b/a.txt")),
		NewDiffHunkLine(hunk, mustDiffCodeLine(t, "@@ -1,3 +1,4 @@")),
		contextOne, deletion, addition, more, contextLast,
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	return document
}

func sampleDiffLayout(t *testing.T, wrap bool) DiffLayout {
	t.Helper()
	layout, err := NewDiffLayout(
		sampleDiffDocument(t),
		DefaultDiffLayoutOptions().WithViewportWidth(12).WithWrap(wrap),
	)
	if err != nil {
		t.Fatal(err)
	}
	return layout
}

func TestDiffRangeAndLineConstructorsRejectInvalidNumbers(t *testing.T) {
	if DiffSide(255).String() != "unknown" || InvalidDiffRangeKind(255).String() != "unknown" {
		t.Fatal("unknown public enum values must remain distinguishable")
	}

	_, err := NewDiffRange(0, 1)
	var rangeError *InvalidDiffRange
	if !errors.As(err, &rangeError) || rangeError.Kind() != InvalidDiffRangeStart {
		t.Fatalf("range error = %v", err)
	}
	_, err = NewDiffRange(^uint64(0), 2)
	if !errors.As(err, &rangeError) || rangeError.Kind() != InvalidDiffRangeOverflow {
		t.Fatalf("overflow error = %v", err)
	}
	empty, err := NewDiffRange(0, 0)
	if err != nil || empty != (DiffRange{}) {
		t.Fatalf("empty = %#v err=%v", empty, err)
	}
	maximum, err := NewDiffRange(^uint64(0), 1)
	if last, ok := maximum.Last(); err != nil || !ok || last != ^uint64(0) {
		t.Fatalf("maximum last=%d ok=%t err=%v", last, ok, err)
	}

	_, err = NewDiffContextLine(0, 1, mustDiffCodeLine(t, "a"))
	var lineError *InvalidDiffLineNumber
	if !errors.As(err, &lineError) || lineError.Side() != DiffSideOld {
		t.Fatalf("old error = %v", err)
	}
	_, err = NewDiffContextLine(1, 0, mustDiffCodeLine(t, "a"))
	if !errors.As(err, &lineError) || lineError.Side() != DiffSideNew {
		t.Fatalf("new error = %v", err)
	}
}

func TestDiffDocumentGeneratesExactUnifiedRanges(t *testing.T) {
	document := sampleDiffDocument(t)
	if document.TextBytes() != 71 {
		t.Fatalf("text bytes = %d", document.TextBytes())
	}
	start, end, ok := document.ByteRangeForLines(0, 2)
	text, copyOK := document.CopyTextForLines(0, 2)
	if !ok || !copyOK || start != 0 || end != 42 ||
		text != "diff --git a/a.txt b/a.txt\n@@ -1,3 +1,4 @@" {
		t.Fatalf("preamble=%q range=%d:%d ok=%t/%t", text, start, end, ok, copyOK)
	}
	start, end, ok = document.ByteRangeForLines(2, 7)
	text, copyOK = document.CopyTextForLines(2, 7)
	if !ok || !copyOK || start != 43 || end != 71 ||
		text != " same\n-old\n+new\n+more\n last\n" {
		t.Fatalf("body=%q range=%d:%d ok=%t/%t", text, start, end, ok, copyOK)
	}
}

func TestDiffDocumentHiddenAndLimits(t *testing.T) {
	hidden, err := NewStyledCodeLine([]tui.TextSpan{
		tui.NewTextSpan("secret", vt.Style{Hidden: true}),
	})
	if err != nil {
		t.Fatal(err)
	}
	visible, _ := NewDiffAdditionLine(1, mustDiffCodeLine(t, "public"))
	secret, _ := NewDiffDeletionLine(1, hidden)
	document, err := NewDiffDocument([]DiffLine{visible, secret}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !document.IsRangeCopyable(0, 1) || document.IsRangeCopyable(1, 2) {
		t.Fatal("hidden copy prefix mismatch")
	}
	if _, ok := document.CopyTextForLines(1, 2); ok {
		t.Fatal("hidden range copied")
	}

	short, _ := NewDiffAdditionLine(1, mustDiffCodeLine(t, "a"))
	_, err = NewDiffDocumentWithLimits(
		[]DiffLine{short, short}, false,
		DefaultDiffDocumentLimits().WithMaxLines(1),
	)
	var resource *DiffDocumentError
	if !errors.As(err, &resource) || resource.Kind() != DiffDocumentLineLimit || resource.Observed() != 2 {
		t.Fatalf("line resource error = %v", err)
	}

	styled, err := NewStyledCodeLine([]tui.TextSpan{
		tui.NewTextSpan("a", vt.Style{}),
		tui.NewTextSpan("b", vt.Style{}),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewDiffDocumentWithLimits(
		[]DiffLine{NewDiffMetadataLine(styled)}, false,
		DefaultDiffDocumentLimits().WithMaxSpans(1),
	)
	if !errors.As(err, &resource) || resource.Kind() != DiffDocumentSpanLimit || resource.Observed() != 2 {
		t.Fatalf("span resource error = %v", err)
	}

	_, err = NewDiffDocumentWithLimits(
		[]DiffLine{short}, false,
		DefaultDiffDocumentLimits().WithMaxTextBytes(1),
	)
	if !errors.As(err, &resource) || resource.Kind() != DiffDocumentTextByteLimit || resource.Observed() != 2 {
		t.Fatalf("text resource error = %v", err)
	}
}

func TestDiffLayoutCacheAndConcurrentClear(t *testing.T) {
	document := sampleDiffDocument(t)
	var cache DiffLayoutCache
	first, err := cache.Resolve(7, document, DiffLayoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.Resolve(7, document, DiffLayoutOptions{})
	if err != nil || first.inner != second.inner {
		t.Fatalf("cache miss err=%v", err)
	}
	cache.Clear()
	third, err := cache.Resolve(7, document, DiffLayoutOptions{})
	if err != nil || first.inner == third.inner {
		t.Fatalf("clear did not replace cache err=%v", err)
	}
}

func TestDiffHunkRangesReserveNumberWidthBeforeBodyLines(t *testing.T) {
	oldRange := mustDiffRange(t, 995, 10)
	newRange := mustDiffRange(t, 9_995, 10)
	document, err := NewDiffDocument([]DiffLine{
		NewDiffHunkLine(NewDiffHunk(oldRange, newRange), mustDiffCodeLine(t, "@@ pending @@")),
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := NewDiffLayout(document, DefaultDiffLayoutOptions().WithViewportWidth(20))
	if err != nil {
		t.Fatal(err)
	}
	if layout.OldNumberWidth() != 4 || layout.NewNumberWidth() != 5 || layout.GutterWidth() != 13 {
		t.Fatalf("widths = %d/%d gutter=%d", layout.OldNumberWidth(), layout.NewNumberWidth(), layout.GutterWidth())
	}
}

func TestDiffDocumentFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t, "widgets/diff-document.txt", "widget-diff-document",
		"operation", "line", "expected-kind", "expected-old", "expected-new",
		"expected-hunk", "start", "end", "expected-copy", "expected-bytes",
	)
	document := sampleDiffDocument(t)
	for _, record := range records {
		switch record.Field("operation") {
		case "line":
			line, ok := document.Line(fixtureInt(t, record.Field("line")))
			if !ok {
				t.Fatalf("case %s: missing line", record.ID)
			}
			if line.Kind().String() != record.Field("expected-kind") {
				t.Errorf("case %s: kind = %s", record.ID, line.Kind())
			}
			oldLine, hasOld := line.OldLine()
			newLine, hasNew := line.NewLine()
			if optionalDiffUint(oldLine, hasOld) != record.Field("expected-old") ||
				optionalDiffUint(newLine, hasNew) != record.Field("expected-new") {
				t.Errorf("case %s: old/new = %s/%s", record.ID, optionalDiffUint(oldLine, hasOld), optionalDiffUint(newLine, hasNew))
			}
			hunkText := "-"
			if hunk, ok := line.HunkMetadata(); ok {
				hunkText = fmt.Sprintf("%d:%d:%d:%d", hunk.OldRange().Start(), hunk.OldRange().Count(), hunk.NewRange().Start(), hunk.NewRange().Count())
			}
			if hunkText != record.Field("expected-hunk") {
				t.Errorf("case %s: hunk = %s", record.ID, hunkText)
			}
		case "copy":
			start, end := fixtureInt(t, record.Field("start")), fixtureInt(t, record.Field("end"))
			byteStart, byteEnd, ok := document.ByteRangeForLines(start, end)
			if !ok || fmt.Sprintf("%d:%d", byteStart, byteEnd) != record.Field("expected-bytes") {
				t.Errorf("case %s: bytes=%d:%d ok=%t", record.ID, byteStart, byteEnd, ok)
			}
			text, ok := document.CopyTextForLines(start, end)
			if !ok || text != record.Text("expected-copy") {
				t.Errorf("case %s: copy=%q ok=%t", record.ID, text, ok)
			}
		default:
			t.Fatalf("case %s: unknown operation", record.ID)
		}
	}
}

func TestDiffLayoutFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t, "widgets/diff-layout.txt", "widget-diff-layout",
		"kind", "input", "old", "new", "profile", "viewport", "tab", "wrap",
		"line-numbers", "expected-gutter", "expected-code-width", "expected-rows",
		"expected-widths", "expected-continuations",
	)
	for _, record := range records {
		content := mustDiffCodeLine(t, record.Text("input"))
		var line DiffLine
		var err error
		switch record.Field("kind") {
		case "context":
			line, err = NewDiffContextLine(fixtureUint64(t, record.Field("old")), fixtureUint64(t, record.Field("new")), content)
		case "addition":
			line, err = NewDiffAdditionLine(fixtureUint64(t, record.Field("new")), content)
		case "deletion":
			line, err = NewDiffDeletionLine(fixtureUint64(t, record.Field("old")), content)
		default:
			t.Fatalf("case %s: unknown kind", record.ID)
		}
		if err != nil {
			t.Fatalf("case %s: line: %v", record.ID, err)
		}
		profile := celltext.ModernWidth()
		if record.Field("profile") == "cjk" {
			profile = celltext.CJKWidth()
		}
		document, err := NewDiffDocument([]DiffLine{line}, false)
		if err != nil {
			t.Fatal(err)
		}
		layout, err := NewDiffLayout(document, DefaultDiffLayoutOptions().
			WithViewportWidth(uint32(fixtureUint64(t, record.Field("viewport")))).
			WithTabWidth(uint8(fixtureUint64(t, record.Field("tab")))).
			WithWrap(fixtureBool(t, record.Field("wrap"))).
			WithLineNumbers(fixtureBool(t, record.Field("line-numbers"))).
			WithWidthProfile(profile))
		if err != nil {
			t.Fatal(err)
		}
		if layout.GutterWidth() != uint32(fixtureUint64(t, record.Field("expected-gutter"))) ||
			layout.CodeWidth() != uint32(fixtureUint64(t, record.Field("expected-code-width"))) {
			t.Errorf("case %s: gutter/code=%d/%d", record.ID, layout.GutterWidth(), layout.CodeWidth())
		}
		var rows, widths, continuations []string
		for index := 0; index < layout.VisualRowCount(); index++ {
			row, _ := layout.codeLayout().row(index)
			var text strings.Builder
			for _, span := range row.spans {
				text.WriteString(span.Text)
			}
			rows = append(rows, text.String())
			widths = append(widths, strconv.FormatUint(uint64(row.width), 10))
			continuations = append(continuations, strconv.FormatBool(row.continuation))
		}
		if strings.Join(rows, "|") != record.Text("expected-rows") ||
			strings.Join(widths, ",") != record.Field("expected-widths") ||
			strings.Join(continuations, ",") != record.Field("expected-continuations") {
			t.Errorf("case %s: rows=%q widths=%q continuations=%q", record.ID, strings.Join(rows, "|"), strings.Join(widths, ","), strings.Join(continuations, ","))
		}
	}
}

func optionalDiffUint(value uint64, present bool) string {
	if !present {
		return "-"
	}
	return strconv.FormatUint(value, 10)
}
