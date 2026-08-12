package widget

import (
	"errors"
	"testing"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
	tui "github.com/mayahiro/nagitui-go"
)

func TestCodeLineRejectsBreaksAndSplitGraphemes(t *testing.T) {
	_, err := NewCodeLine("a\nb")
	var invalid *InvalidCodeLine
	if !errors.As(err, &invalid) || invalid.Kind() != InvalidCodeLineBreak {
		t.Fatalf("line break error = %v", err)
	}
	_, err = NewStyledCodeLine([]tui.TextSpan{
		tui.NewTextSpan("e", vt.Style{}), tui.NewTextSpan("\u0301", vt.Style{}),
	})
	if !errors.As(err, &invalid) || invalid.Kind() != InvalidCodeLineGraphemeBoundary {
		t.Fatalf("grapheme boundary error = %v", err)
	}
}

func TestCodeLineNormalizesInvalidUTF8(t *testing.T) {
	line, err := NewCodeLine("a\xffb")
	if err != nil {
		t.Fatal(err)
	}
	if line.Text() != "a\uFFFDb" {
		t.Fatalf("text = %q", line.Text())
	}
}

func TestCodeDocumentPreservesLineRangesAndTrailingNewline(t *testing.T) {
	first := mustCodeLine(t, "a")
	second := mustCodeLine(t, "日")
	document, err := NewCodeDocument([]CodeLine{first, second}, true)
	if err != nil {
		t.Fatal(err)
	}
	if document.Text() != "a\n日\n" {
		t.Fatalf("text = %q", document.Text())
	}
	for _, test := range []struct {
		start, end, byteStart, byteEnd int
	}{
		{0, 1, 0, 1}, {1, 2, 2, 6}, {0, 2, 0, 6},
	} {
		start, end, ok := document.ByteRangeForLines(test.start, test.end)
		if !ok || start != test.byteStart || end != test.byteEnd {
			t.Fatalf("range %d..%d = %d..%d %t", test.start, test.end, start, end, ok)
		}
	}
}

func TestCodeDocumentHiddenLineAffectsOnlyOverlappingCopyRanges(t *testing.T) {
	hidden, err := NewStyledCodeLine([]tui.TextSpan{
		tui.NewTextSpan("secret", vt.Style{Hidden: true}),
	})
	if err != nil {
		t.Fatal(err)
	}
	document, err := NewCodeDocument([]CodeLine{mustCodeLine(t, "public"), hidden}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !document.IsRangeCopyable(0, 1) || document.IsRangeCopyable(1, 2) ||
		document.IsRangeCopyable(0, 2) {
		t.Fatal("hidden prefix did not isolate copy ranges")
	}
}

func TestCodeLayoutExpandsTabsWrapsAndPreservesStyles(t *testing.T) {
	line, err := NewStyledCodeLine([]tui.TextSpan{
		tui.NewTextSpan("A\t", vt.Style{Foreground: vt.IndexedColor(1)}),
		tui.NewTextSpan("日B", vt.Style{}),
	})
	if err != nil {
		t.Fatal(err)
	}
	document, err := NewCodeDocument([]CodeLine{line}, false)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := NewCodeLayout(document, DefaultCodeLayoutOptions().
		WithViewportWidth(8).WithLineNumbers(false).WithWrap(true))
	if err != nil {
		t.Fatal(err)
	}
	row, ok := layout.row(0)
	if !ok || layout.VisualRowCount() != 1 || row.width != 7 {
		t.Fatalf("row = %#v count=%d", row, layout.VisualRowCount())
	}
	if len(row.spans) != 2 || row.spans[0].Text != "A   " || row.spans[1].Text != "日B" {
		t.Fatalf("spans = %#v", row.spans)
	}
}

func TestCodeLayoutNoWrapSharesValidatedSourceSpans(t *testing.T) {
	line := mustCodeLine(t, "shared")
	document, err := NewCodeDocument([]CodeLine{line}, false)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := NewCodeLayout(document, DefaultCodeLayoutOptions())
	if err != nil {
		t.Fatal(err)
	}
	row, ok := layout.row(0)
	if !ok || len(row.spans) != 1 || len(row.checkpoints) != 0 {
		t.Fatalf("row = %#v", row)
	}
	if &row.spans[0] != &line.spans()[0] {
		t.Fatal("no-wrap row copied immutable source spans")
	}
}

func TestCodeLayoutCJKProfileChangesWrapping(t *testing.T) {
	document, err := NewCodeDocument([]CodeLine{mustCodeLine(t, "·A")}, false)
	if err != nil {
		t.Fatal(err)
	}
	base := DefaultCodeLayoutOptions().WithViewportWidth(2).WithLineNumbers(false).WithWrap(true)
	modern, err := NewCodeLayout(document, base)
	if err != nil {
		t.Fatal(err)
	}
	cjk, err := NewCodeLayout(document, base.WithWidthProfile(celltext.CJKWidth()))
	if err != nil {
		t.Fatal(err)
	}
	if modern.VisualRowCount() != 1 || cjk.VisualRowCount() != 2 {
		t.Fatalf("rows modern=%d cjk=%d", modern.VisualRowCount(), cjk.VisualRowCount())
	}
}

func TestCodeSourceAndLayoutLimitsFailBeforePublication(t *testing.T) {
	line := mustCodeLine(t, "abcd")
	_, err := NewCodeDocumentWithLimits(
		[]CodeLine{line, line}, false, CodeDocumentLimits{}.WithMaxLines(1),
	)
	var documentError *CodeDocumentError
	if !errors.As(err, &documentError) || documentError.Kind() != CodeDocumentLineLimit {
		t.Fatalf("document error = %v", err)
	}
	styled, err := NewStyledCodeLine([]tui.TextSpan{
		tui.NewTextSpan("a", vt.Style{}), tui.NewTextSpan("b", vt.Style{}),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewCodeDocumentWithLimits(
		[]CodeLine{styled}, false, CodeDocumentLimits{}.WithMaxSpans(1),
	)
	if !errors.As(err, &documentError) || documentError.Kind() != CodeDocumentSpanLimit ||
		documentError.Observed() != 2 {
		t.Fatalf("span error = %v", err)
	}
	_, err = NewCodeDocumentWithLimits(
		[]CodeLine{mustCodeLine(t, "ab")}, false,
		CodeDocumentLimits{}.WithMaxTextBytes(1),
	)
	if !errors.As(err, &documentError) || documentError.Kind() != CodeDocumentTextByteLimit ||
		documentError.Observed() != 2 {
		t.Fatalf("text error = %v", err)
	}
	document, err := NewCodeDocument([]CodeLine{line}, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewCodeLayoutWithLimits(
		document,
		DefaultCodeLayoutOptions().WithViewportWidth(1).WithLineNumbers(false).WithWrap(true),
		CodeLayoutLimits{}.WithMaxVisualRows(2),
	)
	var layoutError *CodeLayoutError
	if !errors.As(err, &layoutError) || layoutError.Kind() != CodeLayoutVisualRowLimit ||
		layoutError.Observed() != 3 {
		t.Fatalf("layout error = %v", err)
	}
	document, err = NewCodeDocument([]CodeLine{mustCodeLine(t, "ab")}, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewCodeLayoutWithLimits(
		document, DefaultCodeLayoutOptions().WithLineNumbers(false),
		CodeLayoutLimits{}.WithMaxDisplayBytes(1),
	)
	if !errors.As(err, &layoutError) || layoutError.Kind() != CodeLayoutDisplayByteLimit ||
		layoutError.Observed() != 2 {
		t.Fatalf("display error = %v", err)
	}
	document, err = NewCodeDocument([]CodeLine{mustCodeLine(t, "a\t")}, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewCodeLayoutWithLimits(
		document,
		DefaultCodeLayoutOptions().WithLineNumbers(false).WithTabWidth(^uint8(0)),
		CodeLayoutLimits{}.WithMaxDisplayBytes(2),
	)
	if !errors.As(err, &layoutError) || layoutError.Kind() != CodeLayoutDisplayByteLimit ||
		layoutError.Observed() != 3 {
		t.Fatalf("expanded display error = %v", err)
	}
}

func TestCodeInputsAreDefensivelyOwned(t *testing.T) {
	spans := []tui.TextSpan{tui.NewTextSpan("a", vt.Style{})}
	line, err := NewStyledCodeLine(spans)
	if err != nil {
		t.Fatal(err)
	}
	spans[0].Text = "changed"
	lines := []CodeLine{line}
	document, err := NewCodeDocument(lines, false)
	if err != nil {
		t.Fatal(err)
	}
	lines[0] = mustCodeLine(t, "changed")
	if line.Text() != "a" || document.Text() != "a" {
		t.Fatalf("line=%q document=%q", line.Text(), document.Text())
	}
}

func TestCodeLayoutCacheReusesMatchingDocumentKeyAndLimits(t *testing.T) {
	document, err := NewCodeDocument([]CodeLine{mustCodeLine(t, "a")}, false)
	if err != nil {
		t.Fatal(err)
	}
	cache := NewCodeLayoutCache()
	first, err := cache.Resolve(1, document, CodeLayoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.Resolve(1, document, CodeLayoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if first.inner != second.inner {
		t.Fatal("matching cache lookup rebuilt layout")
	}
	changed, err := cache.Resolve(2, document, CodeLayoutOptions{}.WithWrap(true))
	if err != nil {
		t.Fatal(err)
	}
	if first.inner == changed.inner {
		t.Fatal("changed key reused layout")
	}
	cache.Clear()
	cleared, err := cache.Resolve(2, document, CodeLayoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if changed.inner == cleared.inner {
		t.Fatal("clear retained layout")
	}
}

func TestCodeLayoutCacheDoesNotHoldItsLockDuringCustomWidthCallback(t *testing.T) {
	document, err := NewCodeDocument([]CodeLine{mustCodeLine(t, "a")}, false)
	if err != nil {
		t.Fatal(err)
	}
	cache := NewCodeLayoutCache()
	profile := celltext.CustomWidth(celltext.ModernWidth(), func(string) (int, bool) {
		cache.Clear()
		return 0, false
	})
	if _, err := cache.Resolve(
		1, document, DefaultCodeLayoutOptions().WithWidthProfile(profile),
	); err != nil {
		t.Fatal(err)
	}
}

func mustCodeLine(t *testing.T, text string) CodeLine {
	t.Helper()
	line, err := NewCodeLine(text)
	if err != nil {
		t.Fatal(err)
	}
	return line
}
