package tui

import (
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
	text      string
	style     vt.Style
	width     uint32
	space     bool
	breakLine bool
}

type paragraphLine struct {
	units []paragraphUnit
	width uint32
}

func cloneTextSpans(spans []TextSpan) []TextSpan {
	return append([]TextSpan(nil), spans...)
}

func layoutTextSpans(spans []TextSpan, maxWidth uint32, bounded bool, mode WrapMode) []paragraphLine {
	units := textSpanUnits(spans)
	lines := make([]paragraphLine, 0, 1)
	current := paragraphLine{}
	for _, unit := range units {
		if unit.breakLine {
			lines = append(lines, current)
			current = paragraphLine{}
			continue
		}
		current.units = append(current.units, unit)
		current.width = saturatingAdd32(current.width, unit.width)
		if !bounded || mode == WrapNone {
			continue
		}
		for current.width > maxWidth && len(current.units) > 1 {
			split := len(current.units) - 1
			dropSpace := false
			if mode == WrapWord {
				if space := lastParagraphSpace(current.units); space > 0 {
					split = space
					dropSpace = true
				}
			}
			before := paragraphLineFromUnits(trimParagraphSpaces(current.units[:split], false))
			afterStart := split
			if dropSpace {
				afterStart++
			}
			after := current.units[afterStart:]
			if dropSpace {
				after = trimParagraphSpaces(after, true)
			}
			lines = append(lines, before)
			current = paragraphLineFromUnits(after)
		}
	}
	lines = append(lines, current)
	return lines
}

func textSpanUnits(spans []TextSpan) []paragraphUnit {
	units := make([]paragraphUnit, 0)
	for _, span := range spans {
		graphemes := celltext.IterateGraphemes(span.Text)
		for grapheme, ok := graphemes.Next(); ok; grapheme, ok = graphemes.Next() {
			if grapheme.Text == "\r" || grapheme.Text == "\n" || grapheme.Text == "\r\n" {
				units = append(units, paragraphUnit{breakLine: true})
				continue
			}
			width := max(celltext.GraphemeWidth(grapheme.Text, celltext.ModernWidth()), 1)
			units = append(units, paragraphUnit{
				text:  grapheme.Text,
				style: span.Style,
				width: intToUint32(width),
				space: grapheme.Text == " ",
			})
		}
	}
	return units
}

func lastParagraphSpace(units []paragraphUnit) int {
	for index := len(units) - 1; index >= 0; index-- {
		if units[index].space {
			return index
		}
	}
	return -1
}

func trimParagraphSpaces(units []paragraphUnit, leading bool) []paragraphUnit {
	start, end := 0, len(units)
	if leading {
		for start < end && units[start].space {
			start++
		}
	} else {
		for end > start && units[end-1].space {
			end--
		}
	}
	return units[start:end]
}

func paragraphLineFromUnits(units []paragraphUnit) paragraphLine {
	line := paragraphLine{units: append([]paragraphUnit(nil), units...)}
	for _, unit := range line.units {
		line.width = saturatingAdd32(line.width, unit.width)
	}
	return line
}

func measureRichText(spans []TextSpan, options ParagraphOptions, constraints layoutConstraints) Size {
	lines := layoutTextSpans(spans, constraints.width.value, constraints.width.bounded, normalizedWrapMode(options.Wrap))
	var width uint32
	for _, line := range lines {
		width = max(width, line.width)
	}
	return Size{Width: width, Height: intToUint32(len(lines))}
}

func renderRichText(
	target *surface.Surface,
	rect, clip Rect,
	spans []TextSpan,
	options ParagraphOptions,
) {
	if rect.Empty() {
		return
	}
	lines := layoutTextSpans(spans, rect.Width, true, normalizedWrapMode(options.Wrap))
	for lineIndex, line := range lines {
		if uint32(lineIndex) >= rect.Height {
			break
		}
		desired := min(line.width, rect.Width)
		x := int64(saturatingCoordinate(rect.X, horizontalAlignmentOffset(rect.Width, desired, normalizedParagraphAlignment(options.Alignment))))
		right := int64(rect.X) + int64(rect.Width)
		y := saturatingCoordinate(rect.Y, uint32(lineIndex))
		for _, unit := range line.units {
			end := x + int64(unit.width)
			if end > right {
				break
			}
			if containsRenderUnit(clip, x, int64(y), end) {
				target.Write(clampInt64ToInt32(x), y, unit.text, unit.style, celltext.ModernWidth())
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
