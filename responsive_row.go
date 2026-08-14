package tui

import (
	"sort"

	celltext "github.com/mayahiro/nagi-go/text"
)

// ResponsiveRowPlacement selects the horizontal region used by an item
type ResponsiveRowPlacement uint8

const (
	// ResponsiveRowStart packs an item from the left edge
	ResponsiveRowStart ResponsiveRowPlacement = iota
	// ResponsiveRowCenter centers an item between retained start and end groups
	ResponsiveRowCenter
	// ResponsiveRowEnd packs an item from the right edge
	ResponsiveRowEnd
)

// ResponsiveRowOptions contains layout options for a responsive row
type ResponsiveRowOptions struct {
	// Gap is the number of empty Cells required between retained items
	Gap uint32
	// Height is the exact row height, or zero to use the greatest item height
	Height uint32
}

// DefaultResponsiveRowOptions returns a row with no inter-item gap
func DefaultResponsiveRowOptions() ResponsiveRowOptions {
	return ResponsiveRowOptions{}
}

// ResponsiveRowItem is one eager child supplied to a responsive row
type ResponsiveRowItem[Message any] struct {
	node      Node[Message]
	placement ResponsiveRowPlacement
	priority  uint16
}

// NewResponsiveRowItem returns a start-aligned item with priority zero
func NewResponsiveRowItem[Message any](node Node[Message]) ResponsiveRowItem[Message] {
	return ResponsiveRowItem[Message]{node: node}
}

// Placement sets the horizontal region used by the item
//
// Unknown values use the start region
func (i ResponsiveRowItem[Message]) Placement(placement ResponsiveRowPlacement) ResponsiveRowItem[Message] {
	i.placement = normalizedResponsiveRowPlacement(placement)
	return i
}

// Priority sets the retention priority used under insufficient width
func (i ResponsiveRowItem[Message]) Priority(priority uint16) ResponsiveRowItem[Message] {
	i.priority = priority
	return i
}

// ConfiguredPlacement returns the configured horizontal region
func (i ResponsiveRowItem[Message]) ConfiguredPlacement() ResponsiveRowPlacement {
	return normalizedResponsiveRowPlacement(i.placement)
}

// ConfiguredPriority returns the configured retention priority
func (i ResponsiveRowItem[Message]) ConfiguredPriority() uint16 {
	return i.priority
}

type responsiveRowPayload[Message any] struct {
	items   []ResponsiveRowItem[Message]
	options ResponsiveRowOptions
	cache   *responsiveRowFrame
}

type responsiveRowFrame struct {
	valid  bool
	rect   Rect
	layout resolvedResponsiveRowLayout
}

type responsiveRowMetric struct {
	placement    ResponsiveRowPlacement
	priority     uint16
	desiredWidth uint32
}

type responsiveRowRect struct {
	rect    Rect
	visible bool
}

const inlineResponsiveRowItems = 32

type resolvedResponsiveRowLayout struct {
	inline   [inlineResponsiveRowItems]responsiveRowRect
	overflow []responsiveRowRect
	length   int
}

func newResolvedResponsiveRowLayout(length int) resolvedResponsiveRowLayout {
	var layout resolvedResponsiveRowLayout
	layout.reset(length)
	return layout
}

func (l *resolvedResponsiveRowLayout) slice() []responsiveRowRect {
	if l.length <= inlineResponsiveRowItems {
		return l.inline[:l.length]
	}
	return l.overflow[:l.length]
}

func (l *resolvedResponsiveRowLayout) reset(length int) {
	l.length = length
	if length <= inlineResponsiveRowItems {
		clear(l.inline[:length])
		return
	}
	if cap(l.overflow) < length {
		l.overflow = make([]responsiveRowRect, length)
	} else {
		l.overflow = l.overflow[:length]
		clear(l.overflow)
	}
}

func measureResponsiveRow[Message any](
	responsive *responsiveRowPayload[Message],
	constraints layoutConstraints,
	profile celltext.WidthProfile,
) Size {
	height := constraints.height
	if responsive.options.Height != 0 && (!height.bounded || responsive.options.Height < height.value) {
		height = layoutLimit{value: responsive.options.Height, bounded: true}
	}
	childConstraints := layoutConstraints{height: height}
	var measured Size
	for index := range responsive.items {
		child := responsive.items[index].node.measure(childConstraints, profile)
		measured.Width = saturatingAdd32(measured.Width, max(child.Width, uint32(1)))
		measured.Height = max(measured.Height, child.Height)
	}
	if len(responsive.items) > 1 {
		measured.Width = saturatingAdd32(
			measured.Width,
			saturatingMultiply32(responsive.options.Gap, uint32(len(responsive.items)-1)),
		)
	}
	if responsive.options.Height != 0 {
		measured.Height = responsive.options.Height
	}
	return measured
}

func (r *responsiveRowPayload[Message]) resolvedLayout(
	rect Rect,
	profile celltext.WidthProfile,
) *resolvedResponsiveRowLayout {
	if r.cache != nil && r.cache.valid && r.cache.rect == rect {
		return &r.cache.layout
	}
	layoutRect := rect
	if r.options.Height != 0 {
		layoutRect.Height = min(layoutRect.Height, r.options.Height)
	}
	constraints := layoutConstraints{height: layoutLimit{value: layoutRect.Height, bounded: true}}
	var inlineMetrics [inlineResponsiveRowItems]responsiveRowMetric
	var overflowMetrics []responsiveRowMetric
	var metrics []responsiveRowMetric
	if len(r.items) <= inlineResponsiveRowItems {
		metrics = inlineMetrics[:len(r.items)]
	} else {
		overflowMetrics = make([]responsiveRowMetric, len(r.items))
		metrics = overflowMetrics
	}
	for index := range r.items {
		item := &r.items[index]
		metrics[index] = responsiveRowMetric{
			placement:    normalizedResponsiveRowPlacement(item.placement),
			priority:     item.priority,
			desiredWidth: max(item.node.measure(constraints, profile).Width, uint32(1)),
		}
	}
	if r.cache == nil {
		r.cache = &responsiveRowFrame{}
	}
	r.cache.valid = true
	r.cache.rect = rect
	resolveResponsiveRowLayoutInto(&r.cache.layout, layoutRect, metrics, r.options.Gap)
	return &r.cache.layout
}

func resolveResponsiveRowLayout(rect Rect, metrics []responsiveRowMetric, gap uint32) resolvedResponsiveRowLayout {
	layout := newResolvedResponsiveRowLayout(0)
	resolveResponsiveRowLayoutInto(&layout, rect, metrics, gap)
	return layout
}

func resolveResponsiveRowLayoutInto(
	layout *resolvedResponsiveRowLayout,
	rect Rect,
	metrics []responsiveRowMetric,
	gap uint32,
) {
	layout.reset(len(metrics))
	if rect.Width == 0 || len(metrics) == 0 {
		return
	}
	if len(metrics) <= inlineResponsiveRowItems {
		resolveResponsiveRowLayoutInline(layout, rect, metrics, gap)
		return
	}
	priorityOrder := make([]int, len(metrics))
	for index := range priorityOrder {
		priorityOrder[index] = index
	}
	sort.Slice(priorityOrder, func(left, right int) bool {
		return responsiveRowPriorityBefore(priorityOrder[left], priorityOrder[right], metrics)
	})
	resolveResponsiveRowOrdered(layout, rect, metrics, gap, priorityOrder)
}

func resolveResponsiveRowLayoutInline(
	layout *resolvedResponsiveRowLayout,
	rect Rect,
	metrics []responsiveRowMetric,
	gap uint32,
) {
	var inlineOrder [inlineResponsiveRowItems]int
	priorityOrder := inlineOrder[:len(metrics)]
	for index := range priorityOrder {
		priorityOrder[index] = index
	}
	sortResponsiveRowPriorityInline(priorityOrder, metrics)
	resolveResponsiveRowOrdered(layout, rect, metrics, gap, priorityOrder)
}

func resolveResponsiveRowOrdered(
	layout *resolvedResponsiveRowLayout,
	rect Rect,
	metrics []responsiveRowMetric,
	gap uint32,
	priorityOrder []int,
) {

	resolved := layout.slice()
	remaining := rect.Width
	retained := 0
	for _, index := range priorityOrder {
		desired := max(metrics[index].desiredWidth, uint32(1))
		if retained == 0 {
			allocated := min(desired, remaining)
			resolved[index] = responsiveRowRect{
				rect: Rect{Width: allocated, Height: rect.Height}, visible: true,
			}
			remaining -= allocated
			retained++
			continue
		}
		required := saturatingAdd32(gap, desired)
		if required <= remaining {
			resolved[index] = responsiveRowRect{
				rect: Rect{Width: desired, Height: rect.Height}, visible: true,
			}
			remaining -= required
			retained++
		}
	}

	startWidth, startCount := responsiveRowGroupWidth(metrics, resolved, ResponsiveRowStart, gap)
	centerWidth, centerCount := responsiveRowGroupWidth(metrics, resolved, ResponsiveRowCenter, gap)
	endWidth, endCount := responsiveRowGroupWidth(metrics, resolved, ResponsiveRowEnd, gap)

	placeResponsiveRowGroup(rect, metrics, ResponsiveRowStart, gap, 0, resolved)
	endOffset := rect.Width - endWidth
	placeResponsiveRowGroup(rect, metrics, ResponsiveRowEnd, gap, endOffset, resolved)

	if centerCount != 0 {
		ideal := (rect.Width - centerWidth) / 2
		minimum := startWidth
		if startCount != 0 {
			minimum = saturatingAdd32(minimum, gap)
		}
		maximumBoundary := endOffset
		if endCount != 0 {
			maximumBoundary = maximumBoundary - min(maximumBoundary, gap)
		}
		maximum := maximumBoundary - min(maximumBoundary, centerWidth)
		offset := min(max(ideal, minimum), maximum)
		if minimum > maximum {
			offset = min(minimum, rect.Width-centerWidth)
		}
		placeResponsiveRowGroup(rect, metrics, ResponsiveRowCenter, gap, offset, resolved)
	}
}

func sortResponsiveRowPriorityInline(order []int, metrics []responsiveRowMetric) {
	for index := 1; index < len(order); index++ {
		value := order[index]
		position := index
		for position > 0 && responsiveRowPriorityBefore(value, order[position-1], metrics) {
			order[position] = order[position-1]
			position--
		}
		order[position] = value
	}
}

func responsiveRowPriorityBefore(left, right int, metrics []responsiveRowMetric) bool {
	if metrics[left].priority != metrics[right].priority {
		return metrics[left].priority > metrics[right].priority
	}
	return left < right
}

func responsiveRowGroupWidth(
	metrics []responsiveRowMetric,
	resolved []responsiveRowRect,
	placement ResponsiveRowPlacement,
	gap uint32,
) (uint32, int) {
	var width uint32
	count := 0
	for index, metric := range metrics {
		if !resolved[index].visible || normalizedResponsiveRowPlacement(metric.placement) != placement {
			continue
		}
		if count != 0 {
			width = saturatingAdd32(width, gap)
		}
		width = saturatingAdd32(width, resolved[index].rect.Width)
		count++
	}
	return width, count
}

func placeResponsiveRowGroup(
	parent Rect,
	metrics []responsiveRowMetric,
	placement ResponsiveRowPlacement,
	gap, offset uint32,
	result []responsiveRowRect,
) {
	positioned := 0
	for index, metric := range metrics {
		if !result[index].visible || normalizedResponsiveRowPlacement(metric.placement) != placement {
			continue
		}
		if positioned != 0 {
			offset = saturatingAdd32(offset, gap)
		}
		width := result[index].rect.Width
		result[index].rect = Rect{
			X: saturatingCoordinate(parent.X, offset), Y: parent.Y,
			Width: width, Height: parent.Height,
		}
		offset = saturatingAdd32(offset, width)
		positioned++
	}
}

func normalizedResponsiveRowPlacement(placement ResponsiveRowPlacement) ResponsiveRowPlacement {
	if placement == ResponsiveRowCenter || placement == ResponsiveRowEnd {
		return placement
	}
	return ResponsiveRowStart
}

func saturatingMultiply32(left, right uint32) uint32 {
	product := uint64(left) * uint64(right)
	if product > uint64(^uint32(0)) {
		return ^uint32(0)
	}
	return uint32(product)
}
