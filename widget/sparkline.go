package widget

import (
	"github.com/mayahiro/nagi-go/vt"
	"math/bits"
	"strings"

	"github.com/mayahiro/nagitui-go"
)

var sparklineLevels = [...]string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

// Sparkline is a compact view of unsigned samples
type Sparkline[Message any] struct {
	values    []uint64
	width     uint16
	minimum   uint64
	maximum   uint64
	hasBounds bool
	style     vt.Style
}

// NewSparkline returns a sparkline showing the newest samples within width
func NewSparkline[Message any](values []uint64, width uint16) Sparkline[Message] {
	return Sparkline[Message]{values: append([]uint64(nil), values...), width: width}
}

// Bounds sets an explicit inclusive value range used for scaling
func (s Sparkline[Message]) Bounds(minimum, maximum uint64) Sparkline[Message] {
	s.minimum = minimum
	s.maximum = maximum
	s.hasBounds = true
	return s
}

// Style replaces the sample style
func (s Sparkline[Message]) Style(style vt.Style) Sparkline[Message] {
	s.style = style
	return s
}

// Node builds the public semantic node for this sparkline
func (s Sparkline[Message]) Node() tui.Node[Message] {
	return tui.StyledText[Message](renderSparkline(s.values, s.width, s.minimum, s.maximum, s.hasBounds), s.style)
}

func renderSparkline(values []uint64, width uint16, minimum, maximum uint64, hasBounds bool) string {
	cells := int(width)
	if cells == 0 {
		return ""
	}
	if !hasBounds {
		minimum, maximum = sparklineBounds(values)
	}
	start := max(len(values)-cells, 0)
	visible := values[start:]
	var output strings.Builder
	output.Grow(cells * 3)
	output.WriteString(strings.Repeat(" ", cells-len(visible)))
	for _, value := range visible {
		output.WriteString(sparklineLevels[scaledLevel(value, minimum, maximum, 7)])
	}
	return output.String()
}

func sparklineBounds(values []uint64) (uint64, uint64) {
	if len(values) == 0 {
		return 0, 0
	}
	minimum, maximum := values[0], values[0]
	for _, value := range values[1:] {
		minimum = min(minimum, value)
		maximum = max(maximum, value)
	}
	return minimum, maximum
}

func scaledLevel(value, minimum, maximum, levels uint64) int {
	if maximum <= minimum || levels == 0 {
		return 0
	}
	value = min(max(value, minimum), maximum)
	high, low := bits.Mul64(value-minimum, levels)
	level, _ := bits.Div64(high, low, maximum-minimum)
	return int(level)
}
