package tui

import (
	"sort"
	"unicode/utf8"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go/surface"
)

// WrapMode controls automatic paragraph line wrapping
type WrapMode uint8

const (
	// WrapWord prefers ASCII-space boundaries and hard-wraps oversized words
	WrapWord WrapMode = iota
	// WrapHard wraps at the last complete grapheme that fits
	WrapHard
	// WrapNone preserves only explicit CR, LF, and CRLF line boundaries
	WrapNone
)

// TextSpan is one styled run of paragraph text
type TextSpan struct {
	// Text is UTF-8 content rendered with Style
	Text string
	// Style applies to every grapheme in Text
	Style vt.Style
}

// NewTextSpan returns one styled text run
func NewTextSpan(text string, style vt.Style) TextSpan {
	return TextSpan{Text: text, Style: style}
}

// WithStyle returns this span with a replacement style
func (s TextSpan) WithStyle(style vt.Style) TextSpan {
	s.Style = style
	return s
}

// ParagraphOptions controls paragraph wrapping and horizontal alignment
//
// The zero value uses word wrapping and start alignment.
type ParagraphOptions struct {
	// Wrap controls automatic line boundaries
	Wrap WrapMode
	// Alignment places each rendered line within the paragraph rectangle
	Alignment HorizontalAlignment
}

// DefaultParagraphOptions returns word wrapping with start alignment
func DefaultParagraphOptions() ParagraphOptions {
	return ParagraphOptions{Wrap: WrapWord, Alignment: AlignStart}
}

type paragraphUnit struct {
	span      int
	start     int
	end       int
	width     uint32
	space     bool
	breakLine bool
}

type paragraphLine struct {
	start int
	end   int
	width uint32
}

type paragraphLayout struct {
	units   []paragraphUnit
	lines   []paragraphLine
	size    Size
	pointer *paragraphPointerMetadata
}

type paragraphPointerMetadata struct {
	spanOffsets []int
	cellStarts  []uint32
}

type richTextLayoutKey struct {
	maxWidth uint32
	bounded  bool
	mode     WrapMode
}

type richTextLayoutEntry struct {
	valid bool
	key   richTextLayoutKey
	lines []paragraphLine
	size  Size
}

type richTextLayoutCache struct {
	unitsReady bool
	units      []paragraphUnit
	pointer    *paragraphPointerMetadata
	entries    [2]richTextLayoutEntry
	next       uint8
}

func cloneTextSpans(spans []TextSpan) []TextSpan {
	return append([]TextSpan(nil), spans...)
}

func (c *richTextLayoutCache) resolve(
	spans []TextSpan,
	maxWidth uint32,
	bounded bool,
	mode WrapMode,
	profile celltext.WidthProfile,
) paragraphLayout {
	key := richTextLayoutKey{maxWidth: maxWidth, bounded: bounded, mode: mode}
	for index := range c.entries {
		entry := &c.entries[index]
		if entry.valid && entry.key == key {
			return paragraphLayout{
				units: c.units, lines: entry.lines, size: entry.size,
				pointer: c.pointer,
			}
		}
	}
	if !c.unitsReady {
		c.units = textSpanUnits(spans, profile)
		c.unitsReady = true
	}
	lines := layoutParagraphUnits(c.units, maxWidth, bounded, mode)
	entry := &c.entries[c.next%uint8(len(c.entries))]
	c.next++
	*entry = richTextLayoutEntry{
		valid: true,
		key:   key,
		lines: lines,
		size:  paragraphLayoutSize(lines),
	}
	return paragraphLayout{
		units: c.units, lines: entry.lines, size: entry.size,
		pointer: c.pointer,
	}
}

func (c *richTextLayoutCache) resolveTextHit(
	spans []TextSpan,
	maxWidth uint32,
	mode WrapMode,
	profile celltext.WidthProfile,
) paragraphLayout {
	if !c.unitsReady {
		c.units = textSpanUnits(spans, profile)
		c.unitsReady = true
	}
	if c.pointer == nil {
		c.pointer = newParagraphPointerMetadata(spans, c.units)
	}
	return c.resolve(spans, maxWidth, true, mode, profile)
}

func resolveRichTextLayout(
	spans []TextSpan,
	cache *richTextLayoutCache,
	maxWidth uint32,
	bounded bool,
	mode WrapMode,
	profile celltext.WidthProfile,
) paragraphLayout {
	mode = normalizedWrapMode(mode)
	if cache != nil {
		return cache.resolve(spans, maxWidth, bounded, mode, profile)
	}
	units := textSpanUnits(spans, profile)
	lines := layoutParagraphUnits(units, maxWidth, bounded, mode)
	return paragraphLayout{
		units: units, lines: lines, size: paragraphLayoutSize(lines),
	}
}

func layoutParagraphUnits(units []paragraphUnit, maxWidth uint32, bounded bool, mode WrapMode) []paragraphLine {
	lines := make([]paragraphLine, 0, 1)
	start := 0
	var width uint32
	for index, unit := range units {
		if unit.breakLine {
			lines = append(lines, paragraphLine{start: start, end: index, width: width})
			start = index + 1
			width = 0
			continue
		}
		width = saturatingAdd32(width, unit.width)
		if !bounded || mode == WrapNone {
			continue
		}
		end := index + 1
		for width > maxWidth && end-start > 1 {
			split := end - 1
			dropSpace := false
			if mode == WrapWord {
				if space := lastParagraphSpace(units, start, end); space > start {
					split = space
					dropSpace = true
				}
			}
			beforeEnd := trimParagraphSpaces(units, start, split, false)
			lines = append(lines, paragraphLine{
				start: start,
				end:   beforeEnd,
				width: paragraphUnitsWidth(units, start, beforeEnd),
			})
			afterStart := split
			if dropSpace {
				afterStart++
				afterStart = trimParagraphSpaces(units, afterStart, end, true)
			}
			start = afterStart
			width = paragraphUnitsWidth(units, start, end)
		}
	}
	lines = append(lines, paragraphLine{start: start, end: len(units), width: width})
	return lines
}

func textSpanUnits(spans []TextSpan, profile celltext.WidthProfile) []paragraphUnit {
	capacity := 0
	for _, span := range spans {
		count := utf8.RuneCountInString(span.Text)
		if count > int(^uint(0)>>1)-capacity {
			capacity = 0
			break
		}
		capacity += count
	}
	units := make([]paragraphUnit, 0, capacity)
	for spanIndex, span := range spans {
		graphemes := celltext.IterateGraphemes(span.Text)
		for grapheme, ok := graphemes.Next(); ok; grapheme, ok = graphemes.Next() {
			if grapheme.Text == "\r" || grapheme.Text == "\n" || grapheme.Text == "\r\n" {
				units = append(units, paragraphUnit{
					span: spanIndex, start: grapheme.Start, end: grapheme.End, breakLine: true,
				})
				continue
			}
			width := max(celltext.GraphemeWidth(grapheme.Text, profile), 1)
			units = append(units, paragraphUnit{
				span: spanIndex, start: grapheme.Start, end: grapheme.End,
				width: intToUint32(width), space: grapheme.Text == " ",
			})
		}
	}
	return units
}

func newParagraphPointerMetadata(spans []TextSpan, units []paragraphUnit) *paragraphPointerMetadata {
	cellStarts := make([]uint32, len(units))
	var logicalCell uint32
	for index, unit := range units {
		cellStarts[index] = logicalCell
		if unit.breakLine {
			logicalCell = 0
		} else {
			logicalCell = saturatingAdd32(logicalCell, unit.width)
		}
	}
	return &paragraphPointerMetadata{
		spanOffsets: paragraphSpanOffsets(spans),
		cellStarts:  cellStarts,
	}
}

func paragraphSpanOffsets(spans []TextSpan) []int {
	offsets := make([]int, len(spans)+1)
	for index, span := range spans {
		offsets[index+1] = saturatingAddInt(offsets[index], len(span.Text))
	}
	return offsets
}

func paragraphTextHit(layout paragraphLayout, width uint32, alignment HorizontalAlignment, position Point) TextHit {
	pointer := layout.pointer
	if pointer == nil {
		panic("pointer metadata is not prepared for text hit")
	}
	documentLen := 0
	if len(pointer.spanOffsets) > 0 {
		documentLen = pointer.spanOffsets[len(pointer.spanOffsets)-1]
	}
	if position.Y < 0 {
		return newTextHit(0, 0)
	}
	lineIndex := int(position.Y)
	if lineIndex >= len(layout.lines) {
		return newTextHit(documentLen, documentLen)
	}
	line := layout.lines[lineIndex]
	lineStart := documentLen
	if line.start < len(layout.units) {
		lineStart = paragraphUnitTextHit(layout, layout.units[line.start]).Start()
	}
	lineEnd := lineStart
	if line.end > line.start {
		lineEnd = paragraphUnitTextHit(layout, layout.units[line.end-1]).End()
	}
	desired := min(line.width, width)
	lineX := int64(horizontalAlignmentOffset(width, desired, normalizedParagraphAlignment(alignment)))
	pointerX := int64(position.X)
	if pointerX < lineX {
		return newTextHit(lineStart, lineStart)
	}
	units := layout.units[line.start:line.end]
	cellStarts := pointer.cellStarts[line.start:line.end]
	relative := uint32(pointerX - lineX)
	lineCellStart := uint32(0)
	if len(cellStarts) > 0 {
		lineCellStart = cellStarts[0]
	}
	target := saturatingAdd32(lineCellStart, relative)
	index := sort.Search(len(units), func(index int) bool {
		unit := units[index]
		return saturatingAdd32(cellStarts[index], unit.width) > target
	})
	if index < len(units) {
		return paragraphUnitTextHit(layout, units[index])
	}
	return newTextHit(lineEnd, lineEnd)
}

func paragraphUnitTextHit(layout paragraphLayout, unit paragraphUnit) TextHit {
	pointer := layout.pointer
	if pointer == nil {
		panic("pointer metadata is not prepared for text hit")
	}
	base := 0
	if unit.span >= 0 && unit.span < len(pointer.spanOffsets) {
		base = pointer.spanOffsets[unit.span]
	} else if len(pointer.spanOffsets) > 0 {
		base = pointer.spanOffsets[len(pointer.spanOffsets)-1]
	}
	return newTextHit(saturatingAddInt(base, unit.start), saturatingAddInt(base, unit.end))
}

func saturatingAddInt(left, right int) int {
	maximum := int(^uint(0) >> 1)
	if right > maximum-left {
		return maximum
	}
	return left + right
}

func lastParagraphSpace(units []paragraphUnit, start, end int) int {
	for index := end - 1; index >= start; index-- {
		if units[index].space {
			return index
		}
	}
	return -1
}

func trimParagraphSpaces(units []paragraphUnit, start, end int, leading bool) int {
	if leading {
		for start < end && units[start].space {
			start++
		}
		return start
	}
	for end > start && units[end-1].space {
		end--
	}
	return end
}

func paragraphUnitsWidth(units []paragraphUnit, start, end int) uint32 {
	var width uint32
	for index := start; index < end; index++ {
		width = saturatingAdd32(width, units[index].width)
	}
	return width
}

func paragraphLayoutSize(lines []paragraphLine) Size {
	var width uint32
	for _, line := range lines {
		width = max(width, line.width)
	}
	return Size{Width: width, Height: intToUint32(len(lines))}
}

func measureRichText(
	spans []TextSpan,
	options ParagraphOptions,
	cache *richTextLayoutCache,
	constraints layoutConstraints,
	profile celltext.WidthProfile,
) Size {
	return resolveRichTextLayout(
		spans,
		cache,
		constraints.width.value,
		constraints.width.bounded,
		options.Wrap,
		profile,
	).size
}

func renderRichText(
	target *surface.Surface,
	rect, clip Rect,
	spans []TextSpan,
	options ParagraphOptions,
	cache *richTextLayoutCache,
	profile celltext.WidthProfile,
) {
	if rect.Empty() {
		return
	}
	layout := resolveRichTextLayout(spans, cache, rect.Width, true, options.Wrap, profile)
	for lineIndex, line := range layout.lines {
		if uint32(lineIndex) >= rect.Height {
			break
		}
		desired := min(line.width, rect.Width)
		x := int64(saturatingCoordinate(rect.X, horizontalAlignmentOffset(rect.Width, desired, normalizedParagraphAlignment(options.Alignment))))
		right := int64(rect.X) + int64(rect.Width)
		y := saturatingCoordinate(rect.Y, uint32(lineIndex))
		for _, unit := range layout.units[line.start:line.end] {
			end := x + int64(unit.width)
			if end > right {
				break
			}
			if containsRenderUnit(clip, x, int64(y), end) {
				target.Write(
					clampInt64ToInt32(x),
					y,
					spans[unit.span].Text[unit.start:unit.end],
					spans[unit.span].Style,
					profile,
				)
			}
			x = end
		}
	}
}

func normalizedWrapMode(mode WrapMode) WrapMode {
	if mode > WrapNone {
		return WrapWord
	}
	return mode
}

func normalizedParagraphAlignment(alignment HorizontalAlignment) HorizontalAlignment {
	if alignment > AlignEnd {
		return AlignStart
	}
	return alignment
}
