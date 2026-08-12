package widget

import (
	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/surface"
)

// ChartPoint is one signed integer coordinate
type ChartPoint struct {
	// X is the horizontal data coordinate
	X int32
	// Y is the vertical data coordinate
	Y int32
}

// ChartSeries is one connected data series
type ChartSeries struct {
	// Name identifies the series to applications and legends
	Name string
	// Points are connected in the supplied order
	Points []ChartPoint
	// Style is used by line and point cells
	Style vt.Style
	// Marker is a one-cell grapheme used at exact points
	Marker string
}

// NewChartSeries returns a connected series with a bullet marker
func NewChartSeries(name string, points []ChartPoint) ChartSeries {
	return ChartSeries{Name: celltext.NormalizeUTF8(name), Points: append([]ChartPoint(nil), points...), Marker: "•"}
}

// ChartStyle contains the visual styles used by a Chart
type ChartStyle struct {
	// Axis is used by the left and bottom axes
	Axis vt.Style
}

// DefaultChartStyle returns the standard chart styles
func DefaultChartStyle() ChartStyle {
	return ChartStyle{Axis: vt.Style{Dim: true}}
}

// Chart is a fixed-cell connected plot over signed integer coordinates
type Chart[Message any] struct {
	series       []ChartSeries
	width        uint16
	height       uint16
	minimumX     int32
	maximumX     int32
	minimumY     int32
	maximumY     int32
	hasBounds    bool
	showAxes     bool
	style        ChartStyle
	widthProfile celltext.WidthProfile
}

// NewChart returns a connected plot using automatic data bounds
func NewChart[Message any](series []ChartSeries, width, height uint16) Chart[Message] {
	return Chart[Message]{
		series: cloneChartSeries(series), width: width, height: height,
		showAxes: true, style: DefaultChartStyle(), widthProfile: celltext.ModernWidth(),
	}
}

// Bounds sets explicit inclusive data bounds
func (c Chart[Message]) Bounds(minimumX, maximumX, minimumY, maximumY int32) Chart[Message] {
	c.minimumX, c.maximumX = minimumX, maximumX
	c.minimumY, c.maximumY = minimumY, maximumY
	c.hasBounds = true
	return c
}

// ShowAxes sets whether the left and bottom axes reserve plot cells
func (c Chart[Message]) ShowAxes(show bool) Chart[Message] {
	c.showAxes = show
	return c
}

// Style replaces the chart styles
func (c Chart[Message]) Style(style ChartStyle) Chart[Message] {
	c.style = style
	return c
}

// WidthProfile sets the terminal cell-width policy used by axes and markers
//
// Pass ViewContext.WidthProfile to keep the widget aligned with its Runtime.
func (c Chart[Message]) WidthProfile(profile celltext.WidthProfile) Chart[Message] {
	c.widthProfile = profile
	return c
}

// Node builds the public semantic node for this chart
func (c Chart[Message]) Node() tui.Node[Message] {
	width, height := uint32(c.width), uint32(c.height)
	drawing, err := surface.New(width, height)
	if err != nil {
		return tui.Spacer[Message](width, height)
	}
	if width == 0 || height == 0 {
		return tui.SurfaceNode[Message](drawing)
	}
	left, bottom := 0, 0
	if c.showAxes {
		vertical, horizontal, corner := "│", "─", "└"
		if celltext.GraphemeWidth(vertical, c.widthProfile) != 1 ||
			celltext.GraphemeWidth(horizontal, c.widthProfile) != 1 ||
			celltext.GraphemeWidth(corner, c.widthProfile) != 1 {
			vertical, horizontal, corner = "|", "-", "+"
		}
		left, bottom = 1, 1
		for y := 0; y < int(height)-bottom; y++ {
			drawing.Write(0, int32(y), vertical, c.style.Axis, c.widthProfile)
		}
		if height > 0 {
			axisY := int32(height - 1)
			for x := 0; x < int(width); x++ {
				drawing.Write(int32(x), axisY, horizontal, c.style.Axis, c.widthProfile)
			}
			drawing.Write(0, axisY, corner, c.style.Axis, c.widthProfile)
		}
	}
	plotWidth := max(int(width)-left, 0)
	plotHeight := max(int(height)-bottom, 0)
	if plotWidth == 0 || plotHeight == 0 {
		return tui.SurfaceNode[Message](drawing)
	}
	minimumX, maximumX, minimumY, maximumY := c.chartBounds()
	for _, series := range c.series {
		marker := chartMarker(series.Marker, c.widthProfile)
		mapped := make([]chartCellPoint, len(series.Points))
		for index, point := range series.Points {
			mapped[index] = chartCellPoint{
				x: left + chartScale(point.X, minimumX, maximumX, plotWidth),
				y: plotHeight - 1 - chartScale(point.Y, minimumY, maximumY, plotHeight),
			}
			if index > 0 {
				drawChartLine(drawing, mapped[index-1], mapped[index], series.Style, c.widthProfile)
			}
		}
		for _, point := range mapped {
			drawing.Write(int32(point.x), int32(point.y), marker, series.Style, c.widthProfile)
		}
	}
	return tui.SurfaceNode[Message](drawing)
}

func cloneChartSeries(series []ChartSeries) []ChartSeries {
	output := append([]ChartSeries(nil), series...)
	for index := range output {
		output[index].Name = celltext.NormalizeUTF8(output[index].Name)
		output[index].Marker = celltext.NormalizeUTF8(output[index].Marker)
		output[index].Points = append([]ChartPoint(nil), output[index].Points...)
	}
	return output
}

func (c Chart[Message]) chartBounds() (int32, int32, int32, int32) {
	if c.hasBounds {
		return normalizedChartBounds(c.minimumX, c.maximumX, c.minimumY, c.maximumY)
	}
	var minimumX, maximumX, minimumY, maximumY int32
	hasPoint := false
	for _, series := range c.series {
		for _, point := range series.Points {
			if !hasPoint {
				minimumX, maximumX, minimumY, maximumY = point.X, point.X, point.Y, point.Y
				hasPoint = true
				continue
			}
			minimumX, maximumX = min(minimumX, point.X), max(maximumX, point.X)
			minimumY, maximumY = min(minimumY, point.Y), max(maximumY, point.Y)
		}
	}
	if !hasPoint {
		return 0, 1, 0, 1
	}
	return normalizedChartBounds(minimumX, maximumX, minimumY, maximumY)
}

func normalizedChartBounds(minimumX, maximumX, minimumY, maximumY int32) (int32, int32, int32, int32) {
	if maximumX <= minimumX {
		if minimumX < int32(^uint32(0)>>1) {
			maximumX = minimumX + 1
		} else {
			minimumX--
		}
	}
	if maximumY <= minimumY {
		if minimumY < int32(^uint32(0)>>1) {
			maximumY = minimumY + 1
		} else {
			minimumY--
		}
	}
	return minimumX, maximumX, minimumY, maximumY
}

func chartScale(value, minimum, maximum int32, cells int) int {
	if cells <= 1 || maximum <= minimum {
		return 0
	}
	value = min(max(value, minimum), maximum)
	numerator := (int64(value) - int64(minimum)) * int64(cells-1)
	return int(numerator / (int64(maximum) - int64(minimum)))
}

func chartMarker(marker string, profile celltext.WidthProfile) string {
	graphemes := celltext.IterateGraphemes(marker)
	grapheme, ok := graphemes.Next()
	if ok && celltext.GraphemeWidth(grapheme.Text, profile) == 1 {
		return grapheme.Text
	}
	if celltext.GraphemeWidth("•", profile) == 1 {
		return "•"
	}
	return "*"
}

type chartCellPoint struct {
	x int
	y int
}

func drawChartLine(drawing *surface.Surface, start, end chartCellPoint, style vt.Style, profile celltext.WidthProfile) {
	x, y := start.x, start.y
	deltaX := absChart(end.x - start.x)
	deltaY := -absChart(end.y - start.y)
	stepX, stepY := -1, -1
	if start.x < end.x {
		stepX = 1
	}
	if start.y < end.y {
		stepY = 1
	}
	errorValue := deltaX + deltaY
	lineGlyph := "·"
	if celltext.GraphemeWidth(lineGlyph, profile) != 1 {
		lineGlyph = "."
	}
	for {
		drawing.Write(int32(x), int32(y), lineGlyph, style, profile)
		if x == end.x && y == end.y {
			return
		}
		doubled := 2 * errorValue
		if doubled >= deltaY {
			errorValue += deltaY
			x += stepX
		}
		if doubled <= deltaX {
			errorValue += deltaX
			y += stepY
		}
	}
}

func absChart(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
