package widget

import (
	"fmt"
	"strings"
	"sync"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
	tui "github.com/mayahiro/nagitui-go"
)

const (
	// DefaultCodeDocumentMaxLines is the default maximum logical line count
	DefaultCodeDocumentMaxLines uint64 = 100_000
	// DefaultCodeDocumentMaxSpans is the default maximum styled span count
	DefaultCodeDocumentMaxSpans uint64 = 1_000_000
	// DefaultCodeDocumentMaxTextBytes is the default maximum semantic UTF-8 byte count
	DefaultCodeDocumentMaxTextBytes uint64 = 32 * 1024 * 1024
	// DefaultCodeLayoutMaxVisualRows is the default maximum projected visual row count
	DefaultCodeLayoutMaxVisualRows uint64 = 1_000_000
	// DefaultCodeLayoutMaxDisplayBytes is the default maximum expanded display byte count
	DefaultCodeLayoutMaxDisplayBytes uint64 = 64 * 1024 * 1024
	// DefaultCodeLayoutViewportWidth is the default total terminal viewport width
	DefaultCodeLayoutViewportWidth uint32 = 80
	// DefaultCodeLayoutTabWidth is the default tab stop width
	DefaultCodeLayoutTabWidth uint8 = 4
)

// InvalidCodeLineKind identifies one invalid styled-line category
type InvalidCodeLineKind uint8

const (
	// InvalidCodeLineBreak means a CR or LF appeared inside a logical line
	InvalidCodeLineBreak InvalidCodeLineKind = iota
	// InvalidCodeLineGraphemeBoundary means a style boundary split a grapheme
	InvalidCodeLineGraphemeBoundary
)

// String returns the stable language-independent error identifier
func (k InvalidCodeLineKind) String() string {
	if k == InvalidCodeLineGraphemeBoundary {
		return "grapheme-boundary"
	}
	return "line-break"
}

// InvalidCodeLine is a structured invalid-line error
type InvalidCodeLine struct {
	kind       InvalidCodeLineKind
	spanIndex  int
	byteOffset int
}

// Kind returns the stable error category
func (e *InvalidCodeLine) Kind() InvalidCodeLineKind { return e.kind }

// SpanIndex returns the zero-based offending span index
func (e *InvalidCodeLine) SpanIndex() int { return e.spanIndex }

// ByteOffset returns the UTF-8 byte offset in the complete logical line
func (e *InvalidCodeLine) ByteOffset() int { return e.byteOffset }

// Error implements error
func (e *InvalidCodeLine) Error() string {
	return fmt.Sprintf("invalid code line %s at span %d byte %d", e.kind, e.spanIndex, e.byteOffset)
}

// CodeLine is one immutable logical line made from ordered styled spans
type CodeLine struct {
	inner *codeLineInner
}

type codeLineInner struct {
	spans    []tui.TextSpan
	text     string
	copyable bool
}

// NewCodeLine returns one default-style logical line
//
// CR and LF are rejected because line boundaries belong to CodeDocument
func NewCodeLine(text string) (CodeLine, error) {
	return NewStyledCodeLine([]tui.TextSpan{tui.NewTextSpan(text, vt.Style{})})
}

// NewStyledCodeLine returns one logical line from ordered styled spans
//
// Invalid UTF-8 runs are replaced before validation CR and LF are rejected
// and every span boundary must be an extended grapheme boundary
func NewStyledCodeLine(spans []tui.TextSpan) (CodeLine, error) {
	owned := make([]tui.TextSpan, len(spans))
	if len(spans) == 1 {
		normalized := celltext.NormalizeUTF8(spans[0].Text)
		if relative := strings.IndexAny(normalized, "\r\n"); relative >= 0 {
			return CodeLine{}, &InvalidCodeLine{
				kind: InvalidCodeLineBreak, spanIndex: 0, byteOffset: relative,
			}
		}
		owned[0] = tui.NewTextSpan(normalized, spans[0].Style)
		return CodeLine{inner: &codeLineInner{
			spans: owned, copyable: !spans[0].Style.Hidden,
		}}, nil
	}
	var text strings.Builder
	boundaries := make([]int, 0, max(len(spans)-1, 0))
	copyable := true
	for index, span := range spans {
		normalized := celltext.NormalizeUTF8(span.Text)
		if relative := strings.IndexAny(normalized, "\r\n"); relative >= 0 {
			return CodeLine{}, &InvalidCodeLine{
				kind: InvalidCodeLineBreak, spanIndex: index,
				byteOffset: text.Len() + relative,
			}
		}
		owned[index] = tui.NewTextSpan(normalized, span.Style)
		text.WriteString(normalized)
		copyable = copyable && !span.Style.Hidden
		if index+1 < len(spans) {
			boundaries = append(boundaries, text.Len())
		}
	}
	value := text.String()
	for index, boundary := range boundaries {
		if !celltext.IsGraphemeBoundary(value, boundary) {
			return CodeLine{}, &InvalidCodeLine{
				kind: InvalidCodeLineGraphemeBoundary, spanIndex: index,
				byteOffset: boundary,
			}
		}
	}
	return CodeLine{inner: &codeLineInner{spans: owned, text: value, copyable: copyable}}, nil
}

// Text returns semantic UTF-8 line text without a line terminator
func (l CodeLine) Text() string {
	if l.inner == nil {
		return ""
	}
	if len(l.inner.spans) == 1 {
		return l.inner.spans[0].Text
	}
	return l.inner.text
}

// Spans returns an independent copy of styled runs in source order
func (l CodeLine) Spans() []tui.TextSpan {
	if l.inner == nil {
		return nil
	}
	return append([]tui.TextSpan(nil), l.inner.spans...)
}

func (l CodeLine) spans() []tui.TextSpan {
	if l.inner == nil {
		return nil
	}
	return l.inner.spans
}

// IsCopyable reports whether copy callbacks may expose this line
func (l CodeLine) IsCopyable() bool { return l.inner == nil || l.inner.copyable }

// CodeDocumentLimits bounds immutable code source construction
//
// The zero value uses every documented default
type CodeDocumentLimits struct {
	maxLines     uint64
	maxSpans     uint64
	maxTextBytes uint64
}

// DefaultCodeDocumentLimits returns the bounded defaults
func DefaultCodeDocumentLimits() CodeDocumentLimits {
	return CodeDocumentLimits{
		maxLines: DefaultCodeDocumentMaxLines, maxSpans: DefaultCodeDocumentMaxSpans,
		maxTextBytes: DefaultCodeDocumentMaxTextBytes,
	}
}

func (l CodeDocumentLimits) normalized() CodeDocumentLimits {
	if l.maxLines == 0 {
		l.maxLines = DefaultCodeDocumentMaxLines
	}
	if l.maxSpans == 0 {
		l.maxSpans = DefaultCodeDocumentMaxSpans
	}
	if l.maxTextBytes == 0 {
		l.maxTextBytes = DefaultCodeDocumentMaxTextBytes
	}
	return l
}

// MaxLines returns the maximum logical line count
func (l CodeDocumentLimits) MaxLines() uint64 { return l.normalized().maxLines }

// WithMaxLines returns these limits with a maximum logical line count
func (l CodeDocumentLimits) WithMaxLines(value uint64) CodeDocumentLimits {
	if value == 0 {
		value = DefaultCodeDocumentMaxLines
	}
	l.maxLines = value
	return l
}

// MaxSpans returns the maximum styled span count
func (l CodeDocumentLimits) MaxSpans() uint64 { return l.normalized().maxSpans }

// WithMaxSpans returns these limits with a maximum styled span count
func (l CodeDocumentLimits) WithMaxSpans(value uint64) CodeDocumentLimits {
	if value == 0 {
		value = DefaultCodeDocumentMaxSpans
	}
	l.maxSpans = value
	return l
}

// MaxTextBytes returns the maximum complete semantic UTF-8 byte count
func (l CodeDocumentLimits) MaxTextBytes() uint64 { return l.normalized().maxTextBytes }

// WithMaxTextBytes returns these limits with a maximum semantic UTF-8 byte count
func (l CodeDocumentLimits) WithMaxTextBytes(value uint64) CodeDocumentLimits {
	if value == 0 {
		value = DefaultCodeDocumentMaxTextBytes
	}
	l.maxTextBytes = value
	return l
}

// CodeDocumentErrorKind identifies one document resource failure
type CodeDocumentErrorKind uint8

const (
	// CodeDocumentLineLimit means the logical line limit was exceeded
	CodeDocumentLineLimit CodeDocumentErrorKind = iota
	// CodeDocumentSpanLimit means the styled span limit was exceeded
	CodeDocumentSpanLimit
	// CodeDocumentTextByteLimit means the semantic UTF-8 byte limit was exceeded
	CodeDocumentTextByteLimit
)

// String returns the stable language-independent error identifier
func (k CodeDocumentErrorKind) String() string {
	switch k {
	case CodeDocumentSpanLimit:
		return "span-limit"
	case CodeDocumentTextByteLimit:
		return "text-byte-limit"
	default:
		return "line-limit"
	}
}

// CodeDocumentError is a structured document resource failure
type CodeDocumentError struct {
	kind     CodeDocumentErrorKind
	limit    uint64
	observed uint64
}

// Kind returns the stable error category
func (e *CodeDocumentError) Kind() CodeDocumentErrorKind { return e.kind }

// Limit returns the configured limit
func (e *CodeDocumentError) Limit() uint64 { return e.limit }

// Observed returns the first value beyond the configured limit
func (e *CodeDocumentError) Observed() uint64 { return e.observed }

// Error implements error
func (e *CodeDocumentError) Error() string {
	return fmt.Sprintf("code document %s exceeded limit %d with %d", e.kind, e.limit, e.observed)
}

// CodeDocument contains immutable logical lines and complete semantic source text
//
// The zero value is an empty document
type CodeDocument struct {
	inner *codeDocumentInner
}

type codeDocumentInner struct {
	lines           []CodeLine
	text            string
	lineRanges      []codeByteRange
	hiddenPrefix    []uint64
	trailingNewline bool
}

type codeByteRange struct{ start, end int }

// NewCodeDocument builds a document using bounded default limits
func NewCodeDocument(lines []CodeLine, trailingNewline bool) (CodeDocument, error) {
	return NewCodeDocumentWithLimits(lines, trailingNewline, CodeDocumentLimits{})
}

// NewCodeDocumentWithLimits builds a document after validating resource limits
func NewCodeDocumentWithLimits(
	lines []CodeLine,
	trailingNewline bool,
	limits CodeDocumentLimits,
) (CodeDocument, error) {
	limits = limits.normalized()
	if err := checkCodeDocumentLimit(CodeDocumentLineLimit, limits.maxLines, uint64(len(lines))); err != nil {
		return CodeDocument{}, err
	}
	var spans, bytes uint64
	for index, line := range lines {
		spans = saturatingAdd64(spans, uint64(len(line.spans())))
		if err := checkCodeDocumentLimit(CodeDocumentSpanLimit, limits.maxSpans, spans); err != nil {
			return CodeDocument{}, err
		}
		bytes = saturatingAdd64(bytes, uint64(len(line.Text())))
		if index+1 < len(lines) || trailingNewline {
			bytes = saturatingAdd64(bytes, 1)
		}
		if err := checkCodeDocumentLimit(CodeDocumentTextByteLimit, limits.maxTextBytes, bytes); err != nil {
			return CodeDocument{}, err
		}
	}
	owned := append([]CodeLine(nil), lines...)
	trailingNewline = trailingNewline && len(lines) > 0
	var text strings.Builder
	if bytes <= uint64(^uint(0)>>1) {
		text.Grow(int(bytes))
	}
	ranges := make([]codeByteRange, 0, len(lines))
	hidden := make([]uint64, 1, len(lines)+1)
	for index, line := range owned {
		start := text.Len()
		text.WriteString(line.Text())
		ranges = append(ranges, codeByteRange{start: start, end: text.Len()})
		hidden = append(hidden, hidden[len(hidden)-1]+boolUint64(!line.IsCopyable()))
		if index+1 < len(owned) || trailingNewline {
			text.WriteByte('\n')
		}
	}
	return CodeDocument{inner: &codeDocumentInner{
		lines: owned, text: text.String(), lineRanges: ranges,
		hiddenPrefix: hidden, trailingNewline: trailingNewline,
	}}, nil
}

// LineCount returns the logical line count
func (d CodeDocument) LineCount() int {
	if d.inner == nil {
		return 0
	}
	return len(d.inner.lines)
}

// Line returns one logical line by zero-based index
func (d CodeDocument) Line(index int) (CodeLine, bool) {
	if d.inner == nil || index < 0 || index >= len(d.inner.lines) {
		return CodeLine{}, false
	}
	return d.inner.lines[index], true
}

// Text returns the complete semantic UTF-8 source text
func (d CodeDocument) Text() string {
	if d.inner == nil {
		return ""
	}
	return d.inner.text
}

// HasTrailingNewline reports whether the complete source ends with LF
func (d CodeDocument) HasTrailingNewline() bool {
	return d.inner != nil && d.inner.trailingNewline
}

// ByteRangeForLines returns the semantic UTF-8 byte range for ordered logical lines
func (d CodeDocument) ByteRangeForLines(start, end int) (int, int, bool) {
	if start < 0 || start > end || end > d.LineCount() {
		return 0, 0, false
	}
	if start == end {
		if d.inner != nil && start < len(d.inner.lineRanges) {
			value := d.inner.lineRanges[start].start
			return value, value, true
		}
		value := len(d.Text())
		return value, value, true
	}
	byteStart := d.inner.lineRanges[start].start
	byteEnd := d.inner.lineRanges[end-1].end
	if end == d.LineCount() && d.HasTrailingNewline() {
		byteEnd = len(d.Text())
	}
	return byteStart, byteEnd, true
}

// IsRangeCopyable reports whether every line in an ordered range may be copied
func (d CodeDocument) IsRangeCopyable(start, end int) bool {
	if start < 0 || start > end || end > d.LineCount() {
		return false
	}
	if d.inner == nil {
		return start == 0 && end == 0
	}
	return d.inner.hiddenPrefix[start] == d.inner.hiddenPrefix[end]
}

// CodeLayoutOptions controls terminal-dependent code projection
//
// The zero value uses an 80-cell Modern-width no-wrap layout with line numbers
type CodeLayoutOptions struct {
	viewportWidth  uint32
	tabWidth       uint8
	wrap           bool
	lineNumbers    bool
	lineNumbersSet bool
	widthProfile   celltext.WidthProfile
}

// DefaultCodeLayoutOptions returns the terminal projection defaults
func DefaultCodeLayoutOptions() CodeLayoutOptions {
	return CodeLayoutOptions{
		viewportWidth: DefaultCodeLayoutViewportWidth, tabWidth: DefaultCodeLayoutTabWidth,
		lineNumbers: true, lineNumbersSet: true, widthProfile: celltext.ModernWidth(),
	}
}

func (o CodeLayoutOptions) normalized() CodeLayoutOptions {
	if o.viewportWidth == 0 {
		o.viewportWidth = DefaultCodeLayoutViewportWidth
	}
	if o.tabWidth == 0 {
		o.tabWidth = DefaultCodeLayoutTabWidth
	}
	if !o.lineNumbersSet {
		o.lineNumbers = true
		o.lineNumbersSet = true
	}
	return o
}

// ViewportWidth returns the total terminal viewport width
func (o CodeLayoutOptions) ViewportWidth() uint32 { return o.normalized().viewportWidth }

// WithViewportWidth returns these options with a positive viewport width
func (o CodeLayoutOptions) WithViewportWidth(value uint32) CodeLayoutOptions {
	if value == 0 {
		value = DefaultCodeLayoutViewportWidth
	}
	o.viewportWidth = value
	return o
}

// TabWidth returns the positive tab stop width
func (o CodeLayoutOptions) TabWidth() uint8 { return o.normalized().tabWidth }

// WithTabWidth returns these options with a positive tab stop width
func (o CodeLayoutOptions) WithTabWidth(value uint8) CodeLayoutOptions {
	if value == 0 {
		value = DefaultCodeLayoutTabWidth
	}
	o.tabWidth = value
	return o
}

// Wraps reports whether long logical lines hard-wrap at grapheme boundaries
func (o CodeLayoutOptions) Wraps() bool { return o.wrap }

// WithWrap returns these options with hard wrapping enabled or disabled
func (o CodeLayoutOptions) WithWrap(value bool) CodeLayoutOptions { o.wrap = value; return o }

// ShowsLineNumbers reports whether a sticky line-number gutter is reserved
func (o CodeLayoutOptions) ShowsLineNumbers() bool { return o.normalized().lineNumbers }

// WithLineNumbers returns these options with the line-number gutter configured
func (o CodeLayoutOptions) WithLineNumbers(value bool) CodeLayoutOptions {
	o.lineNumbers, o.lineNumbersSet = value, true
	return o
}

// WidthProfile returns the terminal cell-width profile used for projection
func (o CodeLayoutOptions) WidthProfile() celltext.WidthProfile { return o.widthProfile }

// WithWidthProfile returns these options with a replacement cell-width profile
func (o CodeLayoutOptions) WithWidthProfile(value celltext.WidthProfile) CodeLayoutOptions {
	o.widthProfile = value
	return o
}

// CodeLayoutLimits bounds terminal-dependent code projection
//
// The zero value uses every documented default
type CodeLayoutLimits struct {
	maxVisualRows   uint64
	maxDisplayBytes uint64
}

// DefaultCodeLayoutLimits returns the bounded projection defaults
func DefaultCodeLayoutLimits() CodeLayoutLimits {
	return CodeLayoutLimits{
		maxVisualRows:   DefaultCodeLayoutMaxVisualRows,
		maxDisplayBytes: DefaultCodeLayoutMaxDisplayBytes,
	}
}

func (l CodeLayoutLimits) normalized() CodeLayoutLimits {
	if l.maxVisualRows == 0 {
		l.maxVisualRows = DefaultCodeLayoutMaxVisualRows
	}
	if l.maxDisplayBytes == 0 {
		l.maxDisplayBytes = DefaultCodeLayoutMaxDisplayBytes
	}
	return l
}

// MaxVisualRows returns the maximum projected visual row count
func (l CodeLayoutLimits) MaxVisualRows() uint64 { return l.normalized().maxVisualRows }

// WithMaxVisualRows returns these limits with a maximum visual row count
func (l CodeLayoutLimits) WithMaxVisualRows(value uint64) CodeLayoutLimits {
	if value == 0 {
		value = DefaultCodeLayoutMaxVisualRows
	}
	l.maxVisualRows = value
	return l
}

// MaxDisplayBytes returns the maximum expanded display byte count
func (l CodeLayoutLimits) MaxDisplayBytes() uint64 { return l.normalized().maxDisplayBytes }

// WithMaxDisplayBytes returns these limits with a maximum display byte count
func (l CodeLayoutLimits) WithMaxDisplayBytes(value uint64) CodeLayoutLimits {
	if value == 0 {
		value = DefaultCodeLayoutMaxDisplayBytes
	}
	l.maxDisplayBytes = value
	return l
}

// CodeLayoutErrorKind identifies one terminal projection resource failure
type CodeLayoutErrorKind uint8

const (
	// CodeLayoutVisualRowLimit means the visual row limit was exceeded
	CodeLayoutVisualRowLimit CodeLayoutErrorKind = iota
	// CodeLayoutDisplayByteLimit means the expanded display byte limit was exceeded
	CodeLayoutDisplayByteLimit
)

// String returns the stable language-independent error identifier
func (k CodeLayoutErrorKind) String() string {
	if k == CodeLayoutDisplayByteLimit {
		return "display-byte-limit"
	}
	return "visual-row-limit"
}

// CodeLayoutError is a structured terminal projection resource failure
type CodeLayoutError struct {
	kind     CodeLayoutErrorKind
	limit    uint64
	observed uint64
}

// Kind returns the stable error category
func (e *CodeLayoutError) Kind() CodeLayoutErrorKind { return e.kind }

// Limit returns the configured limit
func (e *CodeLayoutError) Limit() uint64 { return e.limit }

// Observed returns the first value beyond the configured limit
func (e *CodeLayoutError) Observed() uint64 { return e.observed }

// Error implements error
func (e *CodeLayoutError) Error() string {
	return fmt.Sprintf("code layout %s exceeded limit %d with %d", e.kind, e.limit, e.observed)
}

type codeVisualRow struct {
	line         int
	continuation bool
	spans        []tui.TextSpan
	checkpoints  []codeRowCheckpoint
	width        uint32
}

type codeRowCheckpoint struct {
	cell uint32
	span int
	byte int
}

// CodeLayout is an immutable terminal projection of one CodeDocument
//
// The zero value is an empty default-width layout
type CodeLayout struct {
	inner *codeLayoutInner
}

// CodeLayoutCache is a single-entry memo for immutable view rebuilding
//
// The application-defined key must change whenever any projection option or
// custom width policy changes Document identity and explicit resource limits
// are compared independently A CodeLayoutCache must not be copied after use
// and its zero value is ready for use Concurrent calls are safe Projection
// runs without the cache lock, so simultaneous misses may duplicate work
type CodeLayoutCache struct {
	mu         sync.Mutex
	generation uint64
	entry      *codeLayoutCacheEntry
}

type codeLayoutCacheEntry struct {
	key      uint64
	document *codeDocumentInner
	limits   CodeLayoutLimits
	layout   CodeLayout
}

// NewCodeLayoutCache returns an empty layout memo
func NewCodeLayoutCache() *CodeLayoutCache { return &CodeLayoutCache{} }

// Resolve returns a cached layout or projects one using bounded default limits
//
// key must represent every option including a Custom width callback identity
// and behavior
func (c *CodeLayoutCache) Resolve(
	key uint64,
	document CodeDocument,
	options CodeLayoutOptions,
) (CodeLayout, error) {
	return c.ResolveWithLimits(key, document, options, CodeLayoutLimits{})
}

// ResolveWithLimits returns a cached layout or projects one using explicit limits
//
// key must represent every option including a Custom width callback identity
// and behavior
func (c *CodeLayoutCache) ResolveWithLimits(
	key uint64,
	document CodeDocument,
	options CodeLayoutOptions,
	limits CodeLayoutLimits,
) (CodeLayout, error) {
	limits = limits.normalized()
	c.mu.Lock()
	if c.entry != nil && c.entry.key == key && c.entry.document == document.inner &&
		c.entry.limits == limits {
		layout := c.entry.layout
		c.mu.Unlock()
		return layout, nil
	}
	generation := c.generation
	c.mu.Unlock()

	layout, err := NewCodeLayoutWithLimits(document, options, limits)
	if err != nil {
		return CodeLayout{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entry != nil && c.entry.key == key && c.entry.document == document.inner &&
		c.entry.limits == limits {
		return c.entry.layout, nil
	}
	if c.generation != generation {
		return layout, nil
	}
	c.entry = &codeLayoutCacheEntry{
		key: key, document: document.inner, limits: limits, layout: layout,
	}
	return layout, nil
}

// Clear removes the cached document and layout
func (c *CodeLayoutCache) Clear() {
	c.mu.Lock()
	c.generation++
	c.entry = nil
	c.mu.Unlock()
}

type codeLayoutInner struct {
	document        CodeDocument
	options         CodeLayoutOptions
	rows            []codeVisualRow
	lineRows        []codeIndexRange
	gutterWidth     uint32
	codeWidth       uint32
	maximumRowWidth uint32
}

type codeIndexRange struct{ start, end int }

// NewCodeLayout projects a document using bounded default layout limits
func NewCodeLayout(document CodeDocument, options CodeLayoutOptions) (CodeLayout, error) {
	return NewCodeLayoutWithLimits(document, options, CodeLayoutLimits{})
}

// NewCodeLayoutWithLimits projects a document after validating resource limits
func NewCodeLayoutWithLimits(
	document CodeDocument,
	options CodeLayoutOptions,
	limits CodeLayoutLimits,
) (CodeLayout, error) {
	options, limits = options.normalized(), limits.normalized()
	gutter, codeWidth := codeLayoutWidths(document, options)
	rowCapacity := document.LineCount()
	if uint64(rowCapacity) > limits.maxVisualRows {
		rowCapacity = int(limits.maxVisualRows)
	}
	builder := codeLayoutBuilder{
		options: options, codeWidth: codeWidth, limits: limits,
		rows: make([]codeVisualRow, 0, rowCapacity),
	}
	lineRows := make([]codeIndexRange, 0, rowCapacity)
	for lineIndex := 0; lineIndex < document.LineCount(); lineIndex++ {
		start := len(builder.rows)
		line, _ := document.Line(lineIndex)
		if err := builder.pushLine(lineIndex, line); err != nil {
			return CodeLayout{}, err
		}
		lineRows = append(lineRows, codeIndexRange{start: start, end: len(builder.rows)})
	}
	return CodeLayout{inner: &codeLayoutInner{
		document: document, options: options, rows: builder.rows, lineRows: lineRows,
		gutterWidth: gutter, codeWidth: codeWidth, maximumRowWidth: builder.maximumRowWidth,
	}}, nil
}

// Document returns the immutable semantic source document
func (l CodeLayout) Document() CodeDocument {
	if l.inner == nil {
		return CodeDocument{}
	}
	return l.inner.document
}

// Options returns the terminal projection options
func (l CodeLayout) Options() CodeLayoutOptions {
	if l.inner == nil {
		return CodeLayoutOptions{}.normalized()
	}
	return l.inner.options
}

// VisualRowCount returns the projected visual row count
func (l CodeLayout) VisualRowCount() int {
	if l.inner == nil {
		return 0
	}
	return len(l.inner.rows)
}

// GutterWidth returns the sticky line-number gutter width
func (l CodeLayout) GutterWidth() uint32 {
	if l.inner == nil {
		return 0
	}
	return l.inner.gutterWidth
}

// CodeWidth returns the code region width after reserving the optional gutter
func (l CodeLayout) CodeWidth() uint32 {
	if l.inner == nil {
		return DefaultCodeLayoutViewportWidth
	}
	return l.inner.codeWidth
}

// MaximumRowWidth returns the widest projected visual row before cropping
func (l CodeLayout) MaximumRowWidth() uint32 {
	if l.inner == nil {
		return 0
	}
	return l.inner.maximumRowWidth
}

func (l CodeLayout) row(index int) (codeVisualRow, bool) {
	if l.inner == nil || index < 0 || index >= len(l.inner.rows) {
		return codeVisualRow{}, false
	}
	return l.inner.rows[index], true
}

func (l CodeLayout) rowsForLine(line int) codeIndexRange {
	if l.inner == nil || line < 0 || line >= len(l.inner.lineRows) {
		return codeIndexRange{}
	}
	return l.inner.lineRows[line]
}

func codeLayoutWidths(document CodeDocument, options CodeLayoutOptions) (uint32, uint32) {
	desired := uint32(decimalDigits(max(document.LineCount(), 1)) + 3)
	gutter := uint32(0)
	if options.lineNumbers && options.viewportWidth > desired {
		gutter = desired
	}
	return gutter, max(options.viewportWidth-gutter, 1)
}

func decimalDigits(value int) int {
	digits := 1
	for value >= 10 {
		value /= 10
		digits++
	}
	return digits
}

type codeLayoutBuilder struct {
	options         CodeLayoutOptions
	codeWidth       uint32
	limits          CodeLayoutLimits
	rows            []codeVisualRow
	displayBytes    uint64
	maximumRowWidth uint32
}

func (b *codeLayoutBuilder) pushLine(lineIndex int, line CodeLine) error {
	if !b.options.wrap && !strings.ContainsRune(line.Text(), '\t') {
		var width uint32
		for _, span := range line.spans() {
			width = saturatingAdd32(width, uint32(celltext.Width(span.Text, b.options.widthProfile)))
		}
		if err := b.addDisplayBytes(len(line.Text())); err != nil {
			return err
		}
		return b.pushRow(lineIndex, false, line.spans(), width)
	}

	continuation, logicalColumn, rowWidth := false, uint32(0), uint32(0)
	row := styledCodeRowBuilder{}
	for _, span := range line.spans() {
		iterator := celltext.IterateGraphemes(span.Text)
		for grapheme, ok := iterator.Next(); ok; grapheme, ok = iterator.Next() {
			if grapheme.Text == "\t" {
				tabWidth := uint32(b.options.tabWidth)
				count := tabWidth - logicalColumn%tabWidth
				for range count {
					if err := b.pushAtom(lineIndex, &continuation, &row, &rowWidth, " ", 1, span.Style); err != nil {
						return err
					}
					logicalColumn++
				}
				continue
			}
			width := uint32(celltext.GraphemeWidth(grapheme.Text, b.options.widthProfile))
			if err := b.pushAtom(lineIndex, &continuation, &row, &rowWidth, grapheme.Text, width, span.Style); err != nil {
				return err
			}
			logicalColumn = saturatingAdd32(logicalColumn, width)
		}
	}
	if !row.empty() || len(b.rows) == 0 || b.rows[len(b.rows)-1].line != lineIndex {
		return b.finishRow(lineIndex, continuation, row, rowWidth)
	}
	return nil
}

func (b *codeLayoutBuilder) pushAtom(
	lineIndex int,
	continuation *bool,
	row *styledCodeRowBuilder,
	rowWidth *uint32,
	text string,
	width uint32,
	style vt.Style,
) error {
	if b.options.wrap && *rowWidth > 0 && saturatingAdd32(*rowWidth, width) > b.codeWidth {
		complete := *row
		*row = styledCodeRowBuilder{}
		if err := b.finishRow(lineIndex, *continuation, complete, *rowWidth); err != nil {
			return err
		}
		*continuation, *rowWidth = true, 0
	}
	if err := b.addDisplayBytes(len(text)); err != nil {
		return err
	}
	row.push(text, style)
	*rowWidth = saturatingAdd32(*rowWidth, width)
	return nil
}

func (b *codeLayoutBuilder) finishRow(
	lineIndex int,
	continuation bool,
	row styledCodeRowBuilder,
	width uint32,
) error {
	spans, _ := row.finish()
	return b.pushRow(lineIndex, continuation, spans, width)
}

func (b *codeLayoutBuilder) pushRow(
	line int,
	continuation bool,
	spans []tui.TextSpan,
	width uint32,
) error {
	observed := uint64(len(b.rows)) + 1
	if err := checkCodeLayoutLimit(CodeLayoutVisualRowLimit, b.limits.maxVisualRows, observed); err != nil {
		return err
	}
	b.maximumRowWidth = max(b.maximumRowWidth, width)
	var checkpoints []codeRowCheckpoint
	if !b.options.wrap && width > codeRowCheckpointCells {
		checkpoints = codeRowCheckpoints(spans, b.options.widthProfile)
	}
	b.rows = append(b.rows, codeVisualRow{
		line: line, continuation: continuation, spans: spans,
		checkpoints: checkpoints, width: width,
	})
	return nil
}

func (b *codeLayoutBuilder) addDisplayBytes(bytes int) error {
	b.displayBytes = saturatingAdd64(b.displayBytes, uint64(bytes))
	return checkCodeLayoutLimit(CodeLayoutDisplayByteLimit, b.limits.maxDisplayBytes, b.displayBytes)
}

type styledCodeRowBuilder struct {
	spans []styledCodeSpanBuilder
}

type styledCodeSpanBuilder struct {
	text  []byte
	style vt.Style
}

func (b *styledCodeRowBuilder) empty() bool { return len(b.spans) == 0 }

func (b *styledCodeRowBuilder) push(text string, style vt.Style) {
	if len(b.spans) > 0 && b.spans[len(b.spans)-1].style == style {
		b.spans[len(b.spans)-1].text = append(b.spans[len(b.spans)-1].text, text...)
		return
	}
	b.spans = append(b.spans, styledCodeSpanBuilder{text: append([]byte(nil), text...), style: style})
}

func (b styledCodeRowBuilder) finish() ([]tui.TextSpan, int) {
	bytes := 0
	spans := make([]tui.TextSpan, len(b.spans))
	for index, span := range b.spans {
		bytes += len(span.text)
		spans[index] = tui.NewTextSpan(string(span.text), span.style)
	}
	return spans, bytes
}

const codeRowCheckpointCells uint32 = 256

func codeRowCheckpoints(
	spans []tui.TextSpan,
	profile celltext.WidthProfile,
) []codeRowCheckpoint {
	var checkpoints []codeRowCheckpoint
	cell, checkpointCell := uint32(0), uint32(0)
	for spanIndex, span := range spans {
		iterator := celltext.IterateGraphemes(span.Text)
		for grapheme, ok := iterator.Next(); ok; grapheme, ok = iterator.Next() {
			if cell-checkpointCell >= codeRowCheckpointCells {
				checkpoints = append(checkpoints, codeRowCheckpoint{
					cell: cell, span: spanIndex, byte: grapheme.Start,
				})
				checkpointCell = cell
			}
			cell = saturatingAdd32(cell, uint32(celltext.GraphemeWidth(grapheme.Text, profile)))
		}
	}
	return checkpoints
}

func checkCodeDocumentLimit(kind CodeDocumentErrorKind, limit, observed uint64) error {
	if observed > limit {
		return &CodeDocumentError{kind: kind, limit: limit, observed: observed}
	}
	return nil
}

func checkCodeLayoutLimit(kind CodeLayoutErrorKind, limit, observed uint64) error {
	if observed > limit {
		return &CodeLayoutError{kind: kind, limit: limit, observed: observed}
	}
	return nil
}

func saturatingAdd64(left, right uint64) uint64 {
	if ^uint64(0)-left < right {
		return ^uint64(0)
	}
	return left + right
}

func saturatingAdd32(left, right uint32) uint32 {
	if ^uint32(0)-left < right {
		return ^uint32(0)
	}
	return left + right
}

func boolUint64(value bool) uint64 {
	if value {
		return 1
	}
	return 0
}
