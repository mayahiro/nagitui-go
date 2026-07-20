package widget

import (
	"github.com/mayahiro/nagi-go/vt"
	"strconv"
	"strings"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagitui-go"
)

// BarChartBar is one labeled unsigned value in a BarChart
type BarChartBar struct {
	// Label identifies the bar
	Label string
	// Value is the unsigned magnitude of the bar
	Value uint64
	// Style is merged over the chart's bar style
	Style vt.Style
}

// NewBarChartBar returns one labeled bar
func NewBarChartBar(label string, value uint64) BarChartBar {
	return BarChartBar{Label: celltext.NormalizeUTF8(label), Value: value}
}

// BarChartStyle contains the visual styles used by a BarChart
type BarChartStyle struct {
	// Label is used by aligned labels
	Label vt.Style
	// Bar is used by completed bar cells
	Bar vt.Style
	// Empty is used by remaining bar cells
	Empty vt.Style
	// Value is used by numeric values
	Value vt.Style
}

// DefaultBarChartStyle returns the standard bar chart styles
func DefaultBarChartStyle() BarChartStyle {
	return BarChartStyle{
		Bar: vt.Style{Bold: true}, Empty: vt.Style{Dim: true}, Value: vt.Style{Dim: true},
	}
}

// BarChart is a horizontal chart with aligned labels and bounded bars
type BarChart[Message any] struct {
	bars       []BarChartBar
	width      uint16
	maximum    uint64
	hasMaximum bool
	showValues bool
	style      BarChartStyle
}

// NewBarChart returns a horizontal chart whose bar portion occupies width cells
func NewBarChart[Message any](bars []BarChartBar, width uint16) BarChart[Message] {
	return BarChart[Message]{
		bars: cloneBarChartBars(bars), width: width, showValues: true, style: DefaultBarChartStyle(),
	}
}

// Maximum sets the shared scaling maximum
//
// Zero is a valid maximum and renders every bar empty.
func (c BarChart[Message]) Maximum(maximum uint64) BarChart[Message] {
	c.maximum = maximum
	c.hasMaximum = true
	return c
}

// ShowValues sets whether numeric values follow each bar
func (c BarChart[Message]) ShowValues(show bool) BarChart[Message] {
	c.showValues = show
	return c
}

// Style replaces the bar chart styles
func (c BarChart[Message]) Style(style BarChartStyle) BarChart[Message] {
	c.style = style
	return c
}

// Node builds the public semantic node for this bar chart
func (c BarChart[Message]) Node() tui.Node[Message] {
	maximum := c.maximum
	if !c.hasMaximum {
		for _, bar := range c.bars {
			maximum = max(maximum, bar.Value)
		}
	}
	labelWidth := 0
	for _, bar := range c.bars {
		labelWidth = max(labelWidth, celltext.Width(bar.Label, celltext.ModernWidth()))
	}
	rows := make([]tui.Node[Message], 0, len(c.bars))
	for _, bar := range c.bars {
		filled := scaledBarCells(bar.Value, maximum, c.width)
		barStyle := c.style.Bar.Merge(bar.Style)
		parts := []tui.Node[Message]{
			tui.StyledText[Message](bar.Label+strings.Repeat(" ", labelWidth-celltext.Width(bar.Label, celltext.ModernWidth())), c.style.Label),
			tui.StyledText[Message](" ", vt.Style{}),
			tui.StyledText[Message](strings.Repeat("█", filled), barStyle),
			tui.StyledText[Message](strings.Repeat("░", int(c.width)-filled), c.style.Empty),
		}
		if c.showValues {
			parts = append(parts,
				tui.StyledText[Message](" ", vt.Style{}),
				tui.StyledText[Message](strconv.FormatUint(bar.Value, 10), c.style.Value),
			)
		}
		rows = append(rows, tui.Row(parts...))
	}
	return tui.Column(rows...)
}

func cloneBarChartBars(bars []BarChartBar) []BarChartBar {
	output := append([]BarChartBar(nil), bars...)
	for index := range output {
		output[index].Label = celltext.NormalizeUTF8(output[index].Label)
	}
	return output
}

func scaledBarCells(value, maximum uint64, width uint16) int {
	if maximum == 0 {
		return 0
	}
	return scaledLevel(value, 0, maximum, uint64(width))
}
