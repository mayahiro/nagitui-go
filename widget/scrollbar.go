package widget

import (
	"github.com/mayahiro/nagi-go/vt"
	"math/bits"
	"strings"

	"github.com/mayahiro/nagitui-go"
)

// ScrollbarOrientation controls the direction in which a Scrollbar is rendered
type ScrollbarOrientation uint8

const (
	// ScrollbarVertical renders one cell per row
	ScrollbarVertical ScrollbarOrientation = iota
	// ScrollbarHorizontal renders all cells in one row
	ScrollbarHorizontal
)

// ScrollbarStyle contains the visual styles used by a Scrollbar
type ScrollbarStyle struct {
	// Track is used by the unoccupied track
	Track vt.Style
	// Thumb is used by the viewport thumb
	Thumb vt.Style
}

// DefaultScrollbarStyle returns the standard scrollbar styles
func DefaultScrollbarStyle() ScrollbarStyle {
	return ScrollbarStyle{Track: vt.Style{Dim: true}}
}

// Scrollbar is a pure view of an application-owned scroll position
type Scrollbar[Message any] struct {
	contentLength  uint64
	viewportLength uint64
	offset         uint64
	trackLength    uint16
	orientation    ScrollbarOrientation
	style          ScrollbarStyle
}

// NewScrollbar returns a vertical scrollbar with the supplied logical lengths
func NewScrollbar[Message any](contentLength, viewportLength, offset uint64, trackLength uint16) Scrollbar[Message] {
	return Scrollbar[Message]{
		contentLength: contentLength, viewportLength: viewportLength,
		offset: offset, trackLength: trackLength, style: DefaultScrollbarStyle(),
	}
}

// Orientation sets the rendering direction
func (s Scrollbar[Message]) Orientation(orientation ScrollbarOrientation) Scrollbar[Message] {
	s.orientation = orientation
	return s
}

// Style replaces the track and thumb styles
func (s Scrollbar[Message]) Style(style ScrollbarStyle) Scrollbar[Message] {
	s.style = style
	return s
}

// Node builds the public semantic node for this scrollbar
func (s Scrollbar[Message]) Node() tui.Node[Message] {
	start, length := scrollbarThumbGeometry(s.contentLength, s.viewportLength, s.offset, s.trackLength)
	before := start
	after := s.trackLength - min(s.trackLength, start+length)
	if s.orientation == ScrollbarHorizontal {
		return tui.Row(
			tui.StyledText[Message](strings.Repeat("─", int(before)), s.style.Track),
			tui.StyledText[Message](strings.Repeat("█", int(length)), s.style.Thumb),
			tui.StyledText[Message](strings.Repeat("─", int(after)), s.style.Track),
		)
	}
	segments := make([]tui.Node[Message], 0, 3)
	appendVerticalScrollbarSegment(&segments, "│", before, s.style.Track)
	appendVerticalScrollbarSegment(&segments, "█", length, s.style.Thumb)
	appendVerticalScrollbarSegment(&segments, "│", after, s.style.Track)
	return tui.Column(segments...)
}

func appendVerticalScrollbarSegment[Message any](segments *[]tui.Node[Message], character string, length uint16, style vt.Style) {
	if length == 0 {
		return
	}
	*segments = append(*segments, tui.StyledText[Message](strings.TrimSuffix(strings.Repeat(character+"\n", int(length)), "\n"), style))
}

func scrollbarThumbGeometry(contentLength, viewportLength, offset uint64, trackLength uint16) (uint16, uint16) {
	if trackLength == 0 {
		return 0, 0
	}
	if contentLength == 0 || viewportLength >= contentLength {
		return 0, trackLength
	}
	length := uint16(min(max(multiplyDivide(viewportLength, uint64(trackLength), contentLength), 1), uint64(trackLength)))
	maximumOffset := contentLength - viewportLength
	maximumStart := trackLength - length
	start := uint16(multiplyDivide(min(offset, maximumOffset), uint64(maximumStart), maximumOffset))
	return start, length
}

func multiplyDivide(value, multiplier, divisor uint64) uint64 {
	high, low := bits.Mul64(value, multiplier)
	quotient, _ := bits.Div64(high, low, divisor)
	return quotient
}
