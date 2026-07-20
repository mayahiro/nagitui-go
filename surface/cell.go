package surface

import (
	"errors"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
)

// ErrExpectedSingleGrapheme indicates that cell content was not exactly one
// extended grapheme cluster
var ErrExpectedSingleGrapheme = errors.New("surface: cell content must contain exactly one grapheme")

// CellSpan is the number of grid cells occupied by a leading cell
type CellSpan uint8

const (
	// SpanOne occupies one terminal cell
	SpanOne CellSpan = iota
	// SpanTwo occupies two terminal cells
	SpanTwo
)

// Cells returns the numeric cell count
func (s CellSpan) Cells() int {
	if s == SpanTwo {
		return 2
	}
	return 1
}

// Opacity controls how a cell participates in surface composition
type Opacity uint8

const (
	// Opaque cells replace destination content and style
	Opaque Opacity = iota
	// Transparent cells preserve content and merge style
	Transparent
)

// Cell is one normalized surface cell
//
// Wide graphemes use a SpanTwo leading cell followed by a continuation cell.
// Use Surface drawing methods to place cells while preserving that invariant.
// The zero value is an opaque default-style blank.
type Cell struct {
	content      string
	span         CellSpan
	continuation bool
	style        vt.Style
	opacity      Opacity
}

// NewCell creates an opaque cell from exactly one extended grapheme cluster
//
// A zero-width cluster becomes a one-cell U+FFFD replacement. Invalid UTF-8 is
// normalized by the text package before cluster validation.
func NewCell(grapheme string, style vt.Style, profile celltext.WidthProfile) (Cell, error) {
	clusters := celltext.Graphemes(grapheme)
	if len(clusters) != 1 {
		return Cell{}, ErrExpectedSingleGrapheme
	}
	return cellFromCluster(clusters[0].Text, style, profile), nil
}

// BlankCell returns an opaque one-cell blank using style
func BlankCell(style vt.Style) Cell {
	return Cell{style: style}
}

// TransparentCell returns a style-only transparent cell
func TransparentCell(style vt.Style) Cell {
	return Cell{style: style, opacity: Transparent}
}

// Content returns the grapheme content, or an empty string for a continuation
// or transparent cell
func (c Cell) Content() string {
	if c.continuation || c.opacity == Transparent {
		return ""
	}
	if c.content == "" {
		return " "
	}
	return c.content
}

// Span returns the display span of this cell's grapheme unit
func (c Cell) Span() CellSpan {
	return c.span
}

// Continuation reports whether this is the trailing cell of a wide grapheme
func (c Cell) Continuation() bool {
	return c.continuation
}

// Style returns the cell style
func (c Cell) Style() vt.Style {
	return c.style
}

// Opacity returns the composition opacity
func (c Cell) Opacity() Opacity {
	return c.opacity
}

func cellFromCluster(grapheme string, style vt.Style, profile celltext.WidthProfile) Cell {
	switch celltext.GraphemeWidth(grapheme, profile) {
	case 1:
		return Cell{content: storedContent(grapheme), style: style}
	case 2:
		return Cell{content: storedContent(grapheme), span: SpanTwo, style: style}
	default:
		return Cell{content: "\uFFFD", style: style}
	}
}

func continuationCell(leading Cell) Cell {
	return Cell{
		span:         SpanTwo,
		continuation: true,
		style:        leading.style,
		opacity:      leading.opacity,
	}
}

func storedContent(content string) string {
	if content == " " {
		return ""
	}
	return content
}
