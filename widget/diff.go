package widget

import (
	"fmt"
	"strings"
	"sync"

	celltext "github.com/mayahiro/nagi-go/text"
)

const (
	// DefaultDiffDocumentMaxLines is the default logical line limit
	DefaultDiffDocumentMaxLines uint64 = DefaultCodeDocumentMaxLines
	// DefaultDiffDocumentMaxSpans is the default styled span limit
	DefaultDiffDocumentMaxSpans uint64 = DefaultCodeDocumentMaxSpans
	// DefaultDiffDocumentMaxTextBytes is the default conceptual unified UTF-8 byte limit
	DefaultDiffDocumentMaxTextBytes uint64 = DefaultCodeDocumentMaxTextBytes
)

// DiffLineKind identifies one semantic diff line kind
type DiffLineKind uint8

const (
	// DiffLineMetadata is exact application-provided metadata
	DiffLineMetadata DiffLineKind = iota
	// DiffLineHunk is exact hunk text carrying typed range metadata
	DiffLineHunk
	// DiffLineContext is present on both the old and new sides
	DiffLineContext
	// DiffLineAddition is present only on the new side
	DiffLineAddition
	// DiffLineDeletion is present only on the old side
	DiffLineDeletion
)

// String returns the stable language-independent identifier
func (k DiffLineKind) String() string {
	switch k {
	case DiffLineMetadata:
		return "metadata"
	case DiffLineHunk:
		return "hunk"
	case DiffLineContext:
		return "context"
	case DiffLineAddition:
		return "addition"
	case DiffLineDeletion:
		return "deletion"
	default:
		return "unknown"
	}
}

// Marker returns the optional ASCII unified marker added during copy
func (k DiffLineKind) Marker() (rune, bool) {
	switch k {
	case DiffLineContext:
		return ' ', true
	case DiffLineAddition:
		return '+', true
	case DiffLineDeletion:
		return '-', true
	default:
		return 0, false
	}
}

// DiffSide identifies a side carrying one diff line number
type DiffSide uint8

const (
	// DiffSideOld is the source before the change
	DiffSideOld DiffSide = iota
	// DiffSideNew is the source after the change
	DiffSideNew
)

// String returns the stable language-independent identifier
func (s DiffSide) String() string {
	switch s {
	case DiffSideOld:
		return "old"
	case DiffSideNew:
		return "new"
	default:
		return "unknown"
	}
}

// InvalidDiffLineNumber is a structured non-positive line-number error
type InvalidDiffLineNumber struct {
	side  DiffSide
	value uint64
}

// Side returns the side containing the invalid number
func (e *InvalidDiffLineNumber) Side() DiffSide { return e.side }

// Value returns the rejected line number
func (e *InvalidDiffLineNumber) Value() uint64 { return e.value }

// Error returns a stable diagnostic description
func (e *InvalidDiffLineNumber) Error() string {
	return fmt.Sprintf("invalid %s diff line number %d", e.side, e.value)
}

// InvalidDiffRangeKind identifies one invalid hunk-range category
type InvalidDiffRangeKind uint8

const (
	// InvalidDiffRangeStart means a non-empty range starts at zero
	InvalidDiffRangeStart InvalidDiffRangeKind = iota
	// InvalidDiffRangeOverflow means the inclusive last line exceeds uint64
	InvalidDiffRangeOverflow
)

// String returns the stable language-independent identifier
func (k InvalidDiffRangeKind) String() string {
	switch k {
	case InvalidDiffRangeStart:
		return "start"
	case InvalidDiffRangeOverflow:
		return "overflow"
	default:
		return "unknown"
	}
}

// InvalidDiffRange is a structured invalid hunk-range error
type InvalidDiffRange struct {
	kind         InvalidDiffRangeKind
	start, count uint64
}

// Kind returns the stable error category
func (e *InvalidDiffRange) Kind() InvalidDiffRangeKind { return e.kind }

// Start returns the rejected range start
func (e *InvalidDiffRange) Start() uint64 { return e.start }

// Count returns the rejected range count
func (e *InvalidDiffRange) Count() uint64 { return e.count }

// Error returns a stable diagnostic description
func (e *InvalidDiffRange) Error() string {
	return fmt.Sprintf("invalid diff range %s start %d count %d", e.kind, e.start, e.count)
}

// DiffRange is one validated source range declared by a hunk
type DiffRange struct {
	start, count uint64
}

// NewDiffRange validates and returns one hunk source range
//
// An empty range may start at zero. A non-empty range starts at one or greater
func NewDiffRange(start, count uint64) (DiffRange, error) {
	if count > 0 && start == 0 {
		return DiffRange{}, &InvalidDiffRange{kind: InvalidDiffRangeStart, start: start, count: count}
	}
	if count > 0 && ^uint64(0)-start < count-1 {
		return DiffRange{}, &InvalidDiffRange{kind: InvalidDiffRangeOverflow, start: start, count: count}
	}
	return DiffRange{start: start, count: count}, nil
}

// Start returns the first declared line number
func (r DiffRange) Start() uint64 { return r.start }

// Count returns the declared line count
func (r DiffRange) Count() uint64 { return r.count }

// Last returns the inclusive last line and whether the range is non-empty
func (r DiffRange) Last() (uint64, bool) {
	if r.count == 0 {
		return 0, false
	}
	return r.start + (r.count - 1), true
}

// DiffHunk contains typed old and new ranges for one hunk header
type DiffHunk struct {
	oldRange, newRange DiffRange
}

// NewDiffHunk returns typed hunk metadata from validated ranges
func NewDiffHunk(oldRange, newRange DiffRange) DiffHunk {
	return DiffHunk{oldRange: oldRange, newRange: newRange}
}

// OldRange returns the old-side range
func (h DiffHunk) OldRange() DiffRange { return h.oldRange }

// NewRange returns the new-side range
func (h DiffHunk) NewRange() DiffRange { return h.newRange }

// DiffLine is one immutable typed diff logical line
type DiffLine struct {
	kind             DiffLineKind
	content          CodeLine
	oldLine, newLine uint64
	hasOld, hasNew   bool
	hunk             DiffHunk
	hasHunk          bool
}

// NewDiffMetadataLine returns exact metadata with no line number or marker
func NewDiffMetadataLine(content CodeLine) DiffLine {
	return DiffLine{kind: DiffLineMetadata, content: content}
}

// NewDiffHunkLine returns exact hunk text carrying typed range metadata
func NewDiffHunkLine(hunk DiffHunk, content CodeLine) DiffLine {
	return DiffLine{kind: DiffLineHunk, content: content, hunk: hunk, hasHunk: true}
}

// NewDiffContextLine validates positive old and new line numbers
func NewDiffContextLine(oldLine, newLine uint64, content CodeLine) (DiffLine, error) {
	if err := validateDiffLineNumber(DiffSideOld, oldLine); err != nil {
		return DiffLine{}, err
	}
	if err := validateDiffLineNumber(DiffSideNew, newLine); err != nil {
		return DiffLine{}, err
	}
	return DiffLine{
		kind: DiffLineContext, content: content,
		oldLine: oldLine, newLine: newLine, hasOld: true, hasNew: true,
	}, nil
}

// NewDiffAdditionLine validates one positive new-side line number
func NewDiffAdditionLine(newLine uint64, content CodeLine) (DiffLine, error) {
	if err := validateDiffLineNumber(DiffSideNew, newLine); err != nil {
		return DiffLine{}, err
	}
	return DiffLine{kind: DiffLineAddition, content: content, newLine: newLine, hasNew: true}, nil
}

// NewDiffDeletionLine validates one positive old-side line number
func NewDiffDeletionLine(oldLine uint64, content CodeLine) (DiffLine, error) {
	if err := validateDiffLineNumber(DiffSideOld, oldLine); err != nil {
		return DiffLine{}, err
	}
	return DiffLine{kind: DiffLineDeletion, content: content, oldLine: oldLine, hasOld: true}, nil
}

// Kind returns the semantic line kind
func (l DiffLine) Kind() DiffLineKind { return l.kind }

// Content returns the immutable styled content without a unified marker
func (l DiffLine) Content() CodeLine { return l.content }

// OldLine returns the optional positive old-side line number
func (l DiffLine) OldLine() (uint64, bool) { return l.oldLine, l.hasOld }

// NewLine returns the optional positive new-side line number
func (l DiffLine) NewLine() (uint64, bool) { return l.newLine, l.hasNew }

// HunkMetadata returns typed metadata only for a Hunk line
func (l DiffLine) HunkMetadata() (DiffHunk, bool) { return l.hunk, l.hasHunk }

// IsCopyable reports whether copy callbacks may expose this line
func (l DiffLine) IsCopyable() bool { return l.content.IsCopyable() }

func validateDiffLineNumber(side DiffSide, value uint64) error {
	if value == 0 {
		return &InvalidDiffLineNumber{side: side, value: value}
	}
	return nil
}

// DiffDocumentLimits bounds immutable diff source construction
//
// The zero value uses every documented default
type DiffDocumentLimits struct {
	maxLines, maxSpans, maxTextBytes uint64
}

// DefaultDiffDocumentLimits returns the bounded source defaults
func DefaultDiffDocumentLimits() DiffDocumentLimits {
	return DiffDocumentLimits{
		maxLines: DefaultDiffDocumentMaxLines, maxSpans: DefaultDiffDocumentMaxSpans,
		maxTextBytes: DefaultDiffDocumentMaxTextBytes,
	}
}

func (l DiffDocumentLimits) normalized() DiffDocumentLimits {
	if l.maxLines == 0 {
		l.maxLines = DefaultDiffDocumentMaxLines
	}
	if l.maxSpans == 0 {
		l.maxSpans = DefaultDiffDocumentMaxSpans
	}
	if l.maxTextBytes == 0 {
		l.maxTextBytes = DefaultDiffDocumentMaxTextBytes
	}
	return l
}

// MaxLines returns the maximum logical line count
func (l DiffDocumentLimits) MaxLines() uint64 { return l.normalized().maxLines }

// WithMaxLines returns these limits with a maximum line count
func (l DiffDocumentLimits) WithMaxLines(value uint64) DiffDocumentLimits {
	if value == 0 {
		value = DefaultDiffDocumentMaxLines
	}
	l.maxLines = value
	return l
}

// MaxSpans returns the maximum styled span count
func (l DiffDocumentLimits) MaxSpans() uint64 { return l.normalized().maxSpans }

// WithMaxSpans returns these limits with a maximum span count
func (l DiffDocumentLimits) WithMaxSpans(value uint64) DiffDocumentLimits {
	if value == 0 {
		value = DefaultDiffDocumentMaxSpans
	}
	l.maxSpans = value
	return l
}

// MaxTextBytes returns the maximum conceptual unified UTF-8 byte count
func (l DiffDocumentLimits) MaxTextBytes() uint64 { return l.normalized().maxTextBytes }

// WithMaxTextBytes returns these limits with a maximum conceptual byte count
func (l DiffDocumentLimits) WithMaxTextBytes(value uint64) DiffDocumentLimits {
	if value == 0 {
		value = DefaultDiffDocumentMaxTextBytes
	}
	l.maxTextBytes = value
	return l
}

// DiffDocumentErrorKind identifies one source resource failure
type DiffDocumentErrorKind uint8

const (
	// DiffDocumentLineLimit means the logical line limit was exceeded
	DiffDocumentLineLimit DiffDocumentErrorKind = iota
	// DiffDocumentSpanLimit means the styled span limit was exceeded
	DiffDocumentSpanLimit
	// DiffDocumentTextByteLimit means the conceptual text limit was exceeded
	DiffDocumentTextByteLimit
)

// String returns the stable language-independent identifier
func (k DiffDocumentErrorKind) String() string {
	switch k {
	case DiffDocumentLineLimit:
		return "line-limit"
	case DiffDocumentSpanLimit:
		return "span-limit"
	case DiffDocumentTextByteLimit:
		return "text-byte-limit"
	default:
		return "unknown"
	}
}

// DiffDocumentError is a structured source resource failure
type DiffDocumentError struct {
	kind            DiffDocumentErrorKind
	limit, observed uint64
}

// Kind returns the stable error category
func (e *DiffDocumentError) Kind() DiffDocumentErrorKind { return e.kind }

// Limit returns the configured limit
func (e *DiffDocumentError) Limit() uint64 { return e.limit }

// Observed returns the first value beyond the limit
func (e *DiffDocumentError) Observed() uint64 { return e.observed }

// Error returns a stable diagnostic description
func (e *DiffDocumentError) Error() string {
	return fmt.Sprintf("diff document %s exceeded limit %d with %d", e.kind, e.limit, e.observed)
}

// DiffDocument owns immutable typed lines and conceptual unified source ranges
type DiffDocument struct {
	inner *diffDocumentInner
}

type diffDocumentInner struct {
	lines           []DiffLine
	projection      CodeDocument
	lineRanges      []codeIndexRange
	hiddenPrefix    []uint64
	textBytes       int
	trailingNewline bool
}

// NewDiffDocument builds a document using bounded default limits
func NewDiffDocument(lines []DiffLine, trailingNewline bool) (DiffDocument, error) {
	return NewDiffDocumentWithLimits(lines, trailingNewline, DiffDocumentLimits{})
}

// NewDiffDocumentWithLimits validates and owns source within explicit limits
func NewDiffDocumentWithLimits(
	source []DiffLine,
	trailingNewline bool,
	limits DiffDocumentLimits,
) (DiffDocument, error) {
	limits = limits.normalized()
	lines := make([]DiffLine, 0, min(len(source), saturatingUint64ToInt(limits.maxLines)))
	projectionLines := make([]CodeLine, 0, cap(lines))
	lineRanges := make([]codeIndexRange, 0, cap(lines))
	hiddenPrefix := make([]uint64, 1, cap(lines)+1)
	var spanCount, textBytes uint64
	for _, line := range source {
		lineCount := uint64(len(lines)) + 1
		if err := checkDiffDocumentLimit(DiffDocumentLineLimit, limits.maxLines, lineCount); err != nil {
			return DiffDocument{}, err
		}
		spanCount = saturatingAdd64(spanCount, uint64(len(line.content.spans())))
		if err := checkDiffDocumentLimit(DiffDocumentSpanLimit, limits.maxSpans, spanCount); err != nil {
			return DiffDocument{}, err
		}
		if len(lines) > 0 {
			textBytes = saturatingAdd64(textBytes, 1)
			if err := checkDiffDocumentLimit(DiffDocumentTextByteLimit, limits.maxTextBytes, textBytes); err != nil {
				return DiffDocument{}, err
			}
		}
		start := saturatingUint64ToInt(textBytes)
		if _, ok := line.kind.Marker(); ok {
			textBytes = saturatingAdd64(textBytes, 1)
			if err := checkDiffDocumentLimit(DiffDocumentTextByteLimit, limits.maxTextBytes, textBytes); err != nil {
				return DiffDocument{}, err
			}
		}
		textBytes = saturatingAdd64(textBytes, uint64(len(line.content.Text())))
		if err := checkDiffDocumentLimit(DiffDocumentTextByteLimit, limits.maxTextBytes, textBytes); err != nil {
			return DiffDocument{}, err
		}
		lineRanges = append(lineRanges, codeIndexRange{start: start, end: saturatingUint64ToInt(textBytes)})
		hiddenPrefix = append(hiddenPrefix, hiddenPrefix[len(hiddenPrefix)-1]+boolUint64(!line.IsCopyable()))
		projectionLines = append(projectionLines, line.content)
		lines = append(lines, line)
	}
	if trailingNewline && len(lines) > 0 {
		textBytes = saturatingAdd64(textBytes, 1)
		if err := checkDiffDocumentLimit(DiffDocumentTextByteLimit, limits.maxTextBytes, textBytes); err != nil {
			return DiffDocument{}, err
		}
	}
	projection, err := NewCodeDocumentWithLimits(projectionLines, false, CodeDocumentLimits{}.
		WithMaxLines(limits.maxLines).
		WithMaxSpans(limits.maxSpans).
		WithMaxTextBytes(limits.maxTextBytes))
	if err != nil {
		if sourceError, ok := err.(*CodeDocumentError); ok {
			return DiffDocument{}, mapCodeDocumentError(sourceError)
		}
		return DiffDocument{}, err
	}
	return DiffDocument{inner: &diffDocumentInner{
		lines: lines, projection: projection, lineRanges: lineRanges,
		hiddenPrefix: hiddenPrefix, textBytes: saturatingUint64ToInt(textBytes),
		trailingNewline: trailingNewline && len(lines) > 0,
	}}, nil
}

// LineCount returns the logical diff line count
func (d DiffDocument) LineCount() int {
	if d.inner == nil {
		return 0
	}
	return len(d.inner.lines)
}

// Line returns one typed logical line by zero-based index
func (d DiffDocument) Line(index int) (DiffLine, bool) {
	if d.inner == nil || index < 0 || index >= len(d.inner.lines) {
		return DiffLine{}, false
	}
	return d.inner.lines[index], true
}

// TextBytes returns the conceptual complete unified UTF-8 byte count
func (d DiffDocument) TextBytes() int {
	if d.inner == nil {
		return 0
	}
	return d.inner.textBytes
}

// HasTrailingNewline reports whether conceptual unified text ends with LF
func (d DiffDocument) HasTrailingNewline() bool {
	return d.inner != nil && d.inner.trailingNewline
}

// ByteRangeForLines returns the conceptual UTF-8 range for ordered logical lines
func (d DiffDocument) ByteRangeForLines(start, end int) (int, int, bool) {
	if start < 0 || start > end || end > d.LineCount() {
		return 0, 0, false
	}
	if start == end {
		if d.inner != nil && start < len(d.inner.lineRanges) {
			value := d.inner.lineRanges[start].start
			return value, value, true
		}
		return d.TextBytes(), d.TextBytes(), true
	}
	byteStart := d.inner.lineRanges[start].start
	byteEnd := d.inner.lineRanges[end-1].end
	if end == d.LineCount() && d.HasTrailingNewline() {
		byteEnd = d.TextBytes()
	}
	return byteStart, byteEnd, true
}

// IsRangeCopyable reports whether every line in an ordered range may be copied
func (d DiffDocument) IsRangeCopyable(start, end int) bool {
	if start < 0 || start > end || end > d.LineCount() {
		return false
	}
	if d.inner == nil {
		return start == 0 && end == 0
	}
	return d.inner.hiddenPrefix[start] == d.inner.hiddenPrefix[end]
}

// CopyTextForLines generates independently owned unified text for a copyable range
func (d DiffDocument) CopyTextForLines(start, end int) (string, bool) {
	if !d.IsRangeCopyable(start, end) {
		return "", false
	}
	byteStart, byteEnd, ok := d.ByteRangeForLines(start, end)
	if !ok {
		return "", false
	}
	var output strings.Builder
	output.Grow(byteEnd - byteStart)
	for index := start; index < end; index++ {
		if index > start {
			output.WriteByte('\n')
		}
		line := d.inner.lines[index]
		if marker, present := line.kind.Marker(); present {
			output.WriteRune(marker)
		}
		output.WriteString(line.content.Text())
	}
	if start < end && end == d.LineCount() && d.HasTrailingNewline() {
		output.WriteByte('\n')
	}
	return output.String(), true
}

func (d DiffDocument) projection() CodeDocument {
	if d.inner == nil {
		return CodeDocument{}
	}
	return d.inner.projection
}

func mapCodeDocumentError(error *CodeDocumentError) error {
	kind := DiffDocumentTextByteLimit
	switch error.Kind() {
	case CodeDocumentLineLimit:
		kind = DiffDocumentLineLimit
	case CodeDocumentSpanLimit:
		kind = DiffDocumentSpanLimit
	}
	return &DiffDocumentError{kind: kind, limit: error.Limit(), observed: error.Observed()}
}

func checkDiffDocumentLimit(kind DiffDocumentErrorKind, limit, observed uint64) error {
	if observed > limit {
		return &DiffDocumentError{kind: kind, limit: limit, observed: observed}
	}
	return nil
}

func saturatingUint64ToInt(value uint64) int {
	maximum := uint64(^uint(0) >> 1)
	if value > maximum {
		return int(maximum)
	}
	return int(value)
}

// DiffLayoutOptions controls terminal-dependent diff projection
//
// The zero value uses an 80-cell Modern no-wrap layout with old and new numbers
type DiffLayoutOptions struct {
	viewportWidth  uint32
	tabWidth       uint8
	wrap           bool
	lineNumbers    bool
	lineNumbersSet bool
	widthProfile   celltext.WidthProfile
}

// DefaultDiffLayoutOptions returns the terminal projection defaults
func DefaultDiffLayoutOptions() DiffLayoutOptions {
	return DiffLayoutOptions{
		viewportWidth: DefaultCodeLayoutViewportWidth,
		tabWidth:      DefaultCodeLayoutTabWidth,
		lineNumbers:   true, lineNumbersSet: true,
		widthProfile: celltext.ModernWidth(),
	}
}

func (o DiffLayoutOptions) normalized() DiffLayoutOptions {
	if o.viewportWidth == 0 {
		o.viewportWidth = DefaultCodeLayoutViewportWidth
	}
	if o.tabWidth == 0 {
		o.tabWidth = DefaultCodeLayoutTabWidth
	}
	if !o.lineNumbersSet {
		o.lineNumbers, o.lineNumbersSet = true, true
	}
	return o
}

// ViewportWidth returns the total terminal viewport width
func (o DiffLayoutOptions) ViewportWidth() uint32 { return o.normalized().viewportWidth }

// WithViewportWidth returns these options with a positive viewport width
func (o DiffLayoutOptions) WithViewportWidth(value uint32) DiffLayoutOptions {
	if value == 0 {
		value = DefaultCodeLayoutViewportWidth
	}
	o.viewportWidth = value
	return o
}

// TabWidth returns the positive tab stop width
func (o DiffLayoutOptions) TabWidth() uint8 { return o.normalized().tabWidth }

// WithTabWidth returns these options with a positive tab stop width
func (o DiffLayoutOptions) WithTabWidth(value uint8) DiffLayoutOptions {
	if value == 0 {
		value = DefaultCodeLayoutTabWidth
	}
	o.tabWidth = value
	return o
}

// Wraps reports whether content hard-wraps at grapheme boundaries
func (o DiffLayoutOptions) Wraps() bool { return o.wrap }

// WithWrap returns these options with wrapping enabled or disabled
func (o DiffLayoutOptions) WithWrap(value bool) DiffLayoutOptions { o.wrap = value; return o }

// ShowsLineNumbers reports whether full old and new numbers are requested
func (o DiffLayoutOptions) ShowsLineNumbers() bool { return o.normalized().lineNumbers }

// WithLineNumbers returns these options with line numbers configured
func (o DiffLayoutOptions) WithLineNumbers(value bool) DiffLayoutOptions {
	o.lineNumbers, o.lineNumbersSet = value, true
	return o
}

// WidthProfile returns the terminal cell-width profile
func (o DiffLayoutOptions) WidthProfile() celltext.WidthProfile { return o.widthProfile }

// WithWidthProfile returns these options with a replacement WidthProfile
func (o DiffLayoutOptions) WithWidthProfile(value celltext.WidthProfile) DiffLayoutOptions {
	o.widthProfile = value
	return o
}

// DiffLayoutLimits are the CodeLayout visual-row and display-byte limits
type DiffLayoutLimits = CodeLayoutLimits

// DiffLayoutErrorKind is the CodeLayout projection error category
type DiffLayoutErrorKind = CodeLayoutErrorKind

// DiffLayoutError is a CodeLayout projection resource failure
type DiffLayoutError = CodeLayoutError

// DiffLayout is an immutable terminal projection of one DiffDocument
type DiffLayout struct {
	inner *diffLayoutInner
}

type diffLayoutInner struct {
	document                       DiffDocument
	options                        DiffLayoutOptions
	code                           CodeLayout
	gutterWidth                    uint32
	oldNumberWidth, newNumberWidth uint32
}

// DiffLayoutCache is a concurrency-safe single-entry projection memo
//
// The caller key must change for every option and Custom width behavior change
type DiffLayoutCache struct {
	mu         sync.Mutex
	generation uint64
	entry      *diffLayoutCacheEntry
}

type diffLayoutCacheEntry struct {
	key      uint64
	document *diffDocumentInner
	limits   DiffLayoutLimits
	layout   DiffLayout
}

// Resolve returns a cached layout or projects one using bounded defaults
func (c *DiffLayoutCache) Resolve(
	key uint64,
	document DiffDocument,
	options DiffLayoutOptions,
) (DiffLayout, error) {
	return c.ResolveWithLimits(key, document, options, DiffLayoutLimits{})
}

// ResolveWithLimits returns a cached layout or projects one using explicit limits
func (c *DiffLayoutCache) ResolveWithLimits(
	key uint64,
	document DiffDocument,
	options DiffLayoutOptions,
	limits DiffLayoutLimits,
) (DiffLayout, error) {
	limits = limits.normalized()
	c.mu.Lock()
	if c.entry != nil && c.entry.key == key && c.entry.document == document.inner && c.entry.limits == limits {
		layout := c.entry.layout
		c.mu.Unlock()
		return layout, nil
	}
	generation := c.generation
	c.mu.Unlock()
	layout, err := NewDiffLayoutWithLimits(document, options, limits)
	if err != nil {
		return DiffLayout{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entry != nil && c.entry.key == key && c.entry.document == document.inner && c.entry.limits == limits {
		return c.entry.layout, nil
	}
	if c.generation != generation {
		return layout, nil
	}
	c.entry = &diffLayoutCacheEntry{key: key, document: document.inner, limits: limits, layout: layout}
	return layout, nil
}

// Clear removes the cached document and layout
func (c *DiffLayoutCache) Clear() {
	c.mu.Lock()
	c.generation++
	c.entry = nil
	c.mu.Unlock()
}

// NewDiffLayout projects a document using bounded default limits
func NewDiffLayout(document DiffDocument, options DiffLayoutOptions) (DiffLayout, error) {
	return NewDiffLayoutWithLimits(document, options, DiffLayoutLimits{})
}

// NewDiffLayoutWithLimits validates and projects one diff document
func NewDiffLayoutWithLimits(
	document DiffDocument,
	options DiffLayoutOptions,
	limits DiffLayoutLimits,
) (DiffLayout, error) {
	options = options.normalized()
	gutter, oldWidth, newWidth := diffLayoutWidths(document, options)
	codeWidth := max(options.viewportWidth-gutter, 1)
	code, err := NewCodeLayoutWithLimits(
		document.projection(),
		DefaultCodeLayoutOptions().
			WithViewportWidth(codeWidth).
			WithTabWidth(options.tabWidth).
			WithWrap(options.wrap).
			WithLineNumbers(false).
			WithWidthProfile(options.widthProfile),
		limits,
	)
	if err != nil {
		return DiffLayout{}, err
	}
	return DiffLayout{inner: &diffLayoutInner{
		document: document, options: options, code: code,
		gutterWidth: gutter, oldNumberWidth: oldWidth, newNumberWidth: newWidth,
	}}, nil
}

// Document returns the immutable typed source document
func (l DiffLayout) Document() DiffDocument {
	if l.inner == nil {
		return DiffDocument{}
	}
	return l.inner.document
}

// Options returns the terminal projection options
func (l DiffLayout) Options() DiffLayoutOptions {
	if l.inner == nil {
		return DiffLayoutOptions{}.normalized()
	}
	return l.inner.options
}

// VisualRowCount returns the projected visual row count
func (l DiffLayout) VisualRowCount() int {
	if l.inner == nil {
		return 0
	}
	return l.inner.code.VisualRowCount()
}

// GutterWidth returns the complete sticky gutter width
func (l DiffLayout) GutterWidth() uint32 {
	if l.inner == nil {
		return 0
	}
	return l.inner.gutterWidth
}

// OldNumberWidth returns the displayed old-side field width or zero
func (l DiffLayout) OldNumberWidth() uint32 {
	if l.inner == nil {
		return 0
	}
	return l.inner.oldNumberWidth
}

// NewNumberWidth returns the displayed new-side field width or zero
func (l DiffLayout) NewNumberWidth() uint32 {
	if l.inner == nil {
		return 0
	}
	return l.inner.newNumberWidth
}

// CodeWidth returns the content region width after the sticky gutter
func (l DiffLayout) CodeWidth() uint32 {
	if l.inner == nil {
		return DefaultCodeLayoutViewportWidth
	}
	return l.inner.code.CodeWidth()
}

// MaximumRowWidth returns the widest projected content row before cropping
func (l DiffLayout) MaximumRowWidth() uint32 {
	if l.inner == nil {
		return 0
	}
	return l.inner.code.MaximumRowWidth()
}

func (l DiffLayout) codeLayout() CodeLayout {
	if l.inner == nil {
		return CodeLayout{}
	}
	return l.inner.code
}

func diffLayoutWidths(document DiffDocument, options DiffLayoutOptions) (uint32, uint32, uint32) {
	var maximumOld, maximumNew uint64
	for index := 0; index < document.LineCount(); index++ {
		line, _ := document.Line(index)
		if value, ok := line.OldLine(); ok {
			maximumOld = max(maximumOld, value)
		}
		if value, ok := line.NewLine(); ok {
			maximumNew = max(maximumNew, value)
		}
		if hunk, ok := line.HunkMetadata(); ok {
			if value, present := hunk.OldRange().Last(); present {
				maximumOld = max(maximumOld, value)
			}
			if value, present := hunk.NewRange().Last(); present {
				maximumNew = max(maximumNew, value)
			}
		}
	}
	oldWidth := uint32(decimalDigits64(max(maximumOld, 1)))
	newWidth := uint32(decimalDigits64(max(maximumNew, 1)))
	full := saturatingAdd32(saturatingAdd32(oldWidth, newWidth), 4)
	if options.lineNumbers && options.viewportWidth > full {
		return full, oldWidth, newWidth
	}
	if options.viewportWidth > 2 {
		return 2, 0, 0
	}
	return 0, 0, 0
}

func decimalDigits64(value uint64) int {
	digits := 1
	for value >= 10 {
		value /= 10
		digits++
	}
	return digits
}
