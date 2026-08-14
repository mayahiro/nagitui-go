package tui

import (
	"fmt"
)

// VirtualFlowItem is one stable item identity in a VirtualFlowItems order
type VirtualFlowItem struct {
	key NodeID
}

// NewVirtualFlowItem returns an item with one stable key
func NewVirtualFlowItem(key NodeID) VirtualFlowItem {
	return VirtualFlowItem{key: key}
}

// Key returns the stable item key
func (i VirtualFlowItem) Key() NodeID {
	return i.key
}

// DuplicateVirtualFlowItemKeyError reports a repeated stable item key
type DuplicateVirtualFlowItemKeyError struct {
	// Key is the duplicated stable item key
	Key NodeID
}

// Error returns the duplicate item diagnostic
func (e *DuplicateVirtualFlowItemKeyError) Error() string {
	return fmt.Sprintf("duplicate virtual flow item key %s", e.Key)
}

// VirtualFlowItems is an immutable unique item order used by a variable-height flow
type VirtualFlowItems struct {
	inner *virtualFlowItemsData
}

type virtualFlowItemsData struct {
	items []VirtualFlowItem
}

// NewVirtualFlowItems returns a validated immutable item order
func NewVirtualFlowItems(items []VirtualFlowItem) (VirtualFlowItems, error) {
	owned := append([]VirtualFlowItem(nil), items...)
	keys := make(map[NodeID]struct{}, len(owned))
	for _, item := range owned {
		if _, exists := keys[item.key]; exists {
			return VirtualFlowItems{}, &DuplicateVirtualFlowItemKeyError{Key: item.key}
		}
		keys[item.key] = struct{}{}
	}
	return VirtualFlowItems{inner: &virtualFlowItemsData{items: owned}}, nil
}

// Len returns the number of stable items
func (i VirtualFlowItems) Len() int {
	if i.inner == nil {
		return 0
	}
	return len(i.inner.items)
}

// Empty reports whether the item order is empty
func (i VirtualFlowItems) Empty() bool {
	return i.Len() == 0
}

// Item returns one item by current index
func (i VirtualFlowItems) Item(index int) (VirtualFlowItem, bool) {
	if i.inner == nil || index < 0 || index >= len(i.inner.items) {
		return VirtualFlowItem{}, false
	}
	return i.inner.items[index], true
}

// Items returns a copy of the immutable ordered items
func (i VirtualFlowItems) Items() []VirtualFlowItem {
	if i.inner == nil {
		return nil
	}
	return append([]VirtualFlowItem(nil), i.inner.items...)
}

func (i VirtualFlowItems) items() []VirtualFlowItem {
	if i.inner == nil {
		return nil
	}
	return i.inner.items
}

func (i VirtualFlowItems) sharesStorage(other VirtualFlowItems) bool {
	return i.inner == other.inner
}

// VirtualFlowUpdate is application-declared invalidation for one content revision
type VirtualFlowUpdate struct {
	revision         uint64
	previousRevision uint64
	hasPrevious      bool
	changedStart     int
	changedEnd       int
	reset            bool
}

// ResetVirtualFlowUpdate invalidates every item for a new opaque revision
func ResetVirtualFlowUpdate(revision uint64) VirtualFlowUpdate {
	return VirtualFlowUpdate{revision: revision, changedEnd: int(^uint(0) >> 1), reset: true}
}

// ChangedVirtualFlowUpdate invalidates one current index range after a known revision
func ChangedVirtualFlowUpdate(
	revision, previousRevision uint64,
	changedStart, changedEnd int,
) VirtualFlowUpdate {
	return VirtualFlowUpdate{
		revision: revision, previousRevision: previousRevision, hasPrevious: true,
		changedStart: changedStart, changedEnd: changedEnd,
	}
}

// Revision returns the current opaque content revision
func (u VirtualFlowUpdate) Revision() uint64 {
	return u.revision
}

// PreviousRevision returns the required prior revision for partial invalidation
func (u VirtualFlowUpdate) PreviousRevision() (uint64, bool) {
	return u.previousRevision, u.hasPrevious
}

// ChangedRange returns the current item index range that may require remeasurement
func (u VirtualFlowUpdate) ChangedRange() (int, int) {
	return u.changedStart, u.changedEnd
}

func (u VirtualFlowUpdate) resetsAll() bool {
	return u.reset
}

// VirtualFlowItemContext is current item data supplied to virtual flow callbacks
type VirtualFlowItemContext struct {
	// Index is the current item index
	Index int
	// Key is the stable item key
	Key NodeID
	// Width is the flow width available to the item in terminal cells
	Width uint32
}

// VirtualFlowSource contains immutable item callbacks and update metadata
type VirtualFlowSource[Message any] struct {
	items    VirtualFlowItems
	update   VirtualFlowUpdate
	estimate func(VirtualFlowItemContext) uint32
	builder  func(VirtualFlowItemContext) Node[Message]
}

// NewVirtualFlowSource returns a source with one-cell estimates and revision zero
func NewVirtualFlowSource[Message any](
	items VirtualFlowItems,
	builder func(VirtualFlowItemContext) Node[Message],
) VirtualFlowSource[Message] {
	if builder == nil {
		builder = func(VirtualFlowItemContext) Node[Message] { return Spacer[Message](0, 1) }
	}
	return VirtualFlowSource[Message]{
		items: items, update: ResetVirtualFlowUpdate(0),
		estimate: func(VirtualFlowItemContext) uint32 { return 1 },
		builder:  builder,
	}
}

// Update sets the current content invalidation metadata
func (s VirtualFlowSource[Message]) Update(update VirtualFlowUpdate) VirtualFlowSource[Message] {
	s.update = update
	return s
}

// EstimatedHeight sets the width-aware estimated item height callback
func (s VirtualFlowSource[Message]) EstimatedHeight(
	estimate func(VirtualFlowItemContext) uint32,
) VirtualFlowSource[Message] {
	if estimate == nil {
		estimate = func(VirtualFlowItemContext) uint32 { return 1 }
	}
	s.estimate = estimate
	return s
}

// Items returns the immutable stable item order
func (s VirtualFlowSource[Message]) Items() VirtualFlowItems {
	return s.items
}

// CurrentUpdate returns the current content invalidation metadata
func (s VirtualFlowSource[Message]) CurrentUpdate() VirtualFlowUpdate {
	return s.update
}

func (s VirtualFlowSource[Message]) context(index int, width uint32) VirtualFlowItemContext {
	item, ok := s.items.Item(index)
	if !ok {
		panic("nagi-tui: virtual flow index does not belong to the source")
	}
	return VirtualFlowItemContext{Index: index, Key: item.key, Width: width}
}

func (s VirtualFlowSource[Message]) estimatedItemHeight(index int, width uint32) uint32 {
	return max(s.estimate(s.context(index, width)), 1)
}

func (s VirtualFlowSource[Message]) build(index int, width uint32) Node[Message] {
	return s.builder(s.context(index, width))
}

// VirtualFlowOptions controls a vertical variable-height virtual flow
type VirtualFlowOptions[Message any] struct {
	// Overscan is the extra number of terminal cells built before and after the visible range
	Overscan uint32
	// StickToEnd follows content growth while the viewport remains at its end
	StickToEnd bool
	// EnsureFocusedVisible scrolls a built focused descendant into view
	EnsureFocusedVisible bool
	// OnScroll optionally maps user scroll state changes to application messages
	OnScroll func(ScrollState) Message
}

// DefaultVirtualFlowOptions returns one-cell overscan without automatic following
func DefaultVirtualFlowOptions[Message any]() VirtualFlowOptions[Message] {
	return VirtualFlowOptions[Message]{Overscan: 1}
}

// VirtualFlowAnchorAffinity identifies the semantic viewport edge
type VirtualFlowAnchorAffinity uint8

const (
	// VirtualFlowAnchorStart positions the viewport start inside an item
	VirtualFlowAnchorStart VirtualFlowAnchorAffinity = iota
	// VirtualFlowAnchorEnd positions the viewport end relative to an item end
	VirtualFlowAnchorEnd
)

// VirtualFlowAnchor is one retained stable item and intra-item cell position
type VirtualFlowAnchor struct {
	// Key is the stable anchor item key
	Key NodeID
	// Offset is the intra-item cell distance from the semantic edge
	Offset uint32
	// Affinity is the viewport edge represented by this anchor
	Affinity VirtualFlowAnchorAffinity
}

// VirtualFlowState is the resolved scroll and semantic position of one flow
type VirtualFlowState struct {
	// Scroll is the underlying resolved vertical scroll state
	Scroll ScrollState
	// Anchor is the retained stable semantic anchor
	Anchor VirtualFlowAnchor
	// HasAnchor reports whether Anchor is present
	HasAnchor bool
	// VisibleStart is the first visible current item index without overscan
	VisibleStart int
	// VisibleEnd is the exclusive visible current item index without overscan
	VisibleEnd int
	// ItemCount is the current number of stable items
	ItemCount int
}

type virtualFlowInteraction struct {
	items          VirtualFlowItems
	updateRevision uint64
	hasRevision    bool
	width          uint32
	heights        virtualFlowHeightIndex
	measured       []bool
	positions      map[NodeID]int
	state          VirtualFlowState
	hasState       bool
	generation     uint64
}

type preservedVirtualFlowAnchor struct {
	anchor   VirtualFlowAnchor
	oldIndex int
	oldItems VirtualFlowItems
}

type virtualFlowWindow struct {
	offset        uint32
	contentHeight uint32
	visibleStart  int
	visibleEnd    int
	builtStart    int
	builtEnd      int
	generation    uint64
}

func (f *virtualFlowInteraction) captureAnchor(
	offset, viewportHeight uint32,
	followingEnd bool,
) (preservedVirtualFlowAnchor, bool) {
	if f.items.Empty() {
		return preservedVirtualFlowAnchor{}, false
	}
	index := 0
	anchor := VirtualFlowAnchor{Affinity: VirtualFlowAnchorStart}
	if followingEnd {
		index = f.items.Len() - 1
		anchor.Offset = f.totalHeight() - min(f.totalHeight(), saturatingAdd32(offset, viewportHeight))
		anchor.Affinity = VirtualFlowAnchorEnd
	} else {
		var ok bool
		index, ok = f.heights.itemAt(offset)
		if !ok {
			return preservedVirtualFlowAnchor{}, false
		}
		anchor.Offset = offset - min(offset, f.heights.prefix(index))
	}
	item, _ := f.items.Item(index)
	anchor.Key = item.key
	return preservedVirtualFlowAnchor{anchor: anchor, oldIndex: index, oldItems: f.items}, true
}

func reconcileVirtualFlow[Message any](
	f *virtualFlowInteraction,
	source VirtualFlowSource[Message],
	width uint32,
) bool {
	sameItems := f.items.sharesStorage(source.items)
	widthChanged := f.width != width
	changed := false
	estimatesCurrent := false
	if !sameItems {
		type oldMeasurement struct {
			height   uint32
			measured bool
		}
		old := make(map[NodeID]oldMeasurement, f.items.Len())
		for index, item := range f.items.items() {
			old[item.key] = oldMeasurement{height: f.heights.height(index), measured: f.isMeasured(index)}
		}
		heights := make([]uint32, source.items.Len())
		measured := make([]bool, source.items.Len())
		reused := false
		for index, item := range source.items.items() {
			if previous, ok := old[item.key]; ok && !widthChanged {
				heights[index] = previous.height
				measured[index] = previous.measured
				reused = true
			} else {
				heights[index] = source.estimatedItemHeight(index, width)
			}
		}
		f.items = source.items
		f.heights = newVirtualFlowHeightIndex(heights)
		f.measured = measured
		f.rebuildPositions()
		changed = true
		estimatesCurrent = !reused
	} else if widthChanged {
		resetVirtualFlowEstimates(f, source, width, 0, source.items.Len())
		changed = true
		estimatesCurrent = true
	}

	if !f.hasRevision || f.updateRevision != source.update.revision {
		if !estimatesCurrent {
			partial := !source.update.resetsAll() && source.update.hasPrevious &&
				f.hasRevision && source.update.previousRevision == f.updateRevision && !widthChanged
			start, end := 0, source.items.Len()
			if partial {
				start, end = source.update.ChangedRange()
			}
			changed = resetVirtualFlowEstimates(f, source, width, start, end) || changed
		}
		f.updateRevision = source.update.revision
		f.hasRevision = true
	}
	f.width = width
	if changed {
		f.generation++
	}
	return changed
}

func resetVirtualFlowEstimates[Message any](
	f *virtualFlowInteraction,
	source VirtualFlowSource[Message],
	width uint32,
	start, end int,
) bool {
	start = min(max(start, 0), source.items.Len())
	end = max(min(max(end, 0), source.items.Len()), start)
	changed := false
	for index := start; index < end; index++ {
		estimate := source.estimatedItemHeight(index, width)
		changed = f.heights.update(index, estimate) || f.measured[index] || changed
		f.measured[index] = false
	}
	return changed
}

func (f *virtualFlowInteraction) setMeasured(index int, height uint32) bool {
	if index < 0 || index >= len(f.measured) {
		return false
	}
	height = max(height, 1)
	changed := !f.measured[index] || f.heights.height(index) != height
	f.measured[index] = true
	if f.heights.update(index, height) {
		f.generation++
	}
	return changed
}

func (f *virtualFlowInteraction) resolveAnchor(
	preserved preservedVirtualFlowAnchor,
	viewportHeight uint32,
) uint32 {
	index, ok := f.positions[preserved.anchor.Key]
	if !ok {
		for old := preserved.oldIndex + 1; old < preserved.oldItems.Len(); old++ {
			item, _ := preserved.oldItems.Item(old)
			if index, ok = f.positions[item.key]; ok {
				break
			}
		}
	}
	if !ok {
		for old := preserved.oldIndex - 1; old >= 0; old-- {
			item, _ := preserved.oldItems.Item(old)
			if index, ok = f.positions[item.key]; ok {
				break
			}
		}
	}
	if !ok {
		return 0
	}
	if preserved.anchor.Affinity == VirtualFlowAnchorEnd {
		return f.heights.prefix(index+1) - min(
			f.heights.prefix(index+1),
			saturatingAdd32(preserved.anchor.Offset, viewportHeight),
		)
	}
	return saturatingAdd32(
		f.heights.prefix(index),
		min(preserved.anchor.Offset, f.heights.height(index)-1),
	)
}

func (f *virtualFlowInteraction) resolveWindow(
	scroll ScrollState,
	viewportHeight, overscan uint32,
	followingEnd bool,
) virtualFlowWindow {
	visibleStart, visibleEnd := f.heights.cellRange(scroll.Offset.Y, viewportHeight)
	builtOffset := scroll.Offset.Y - min(scroll.Offset.Y, overscan)
	builtLimit := saturatingAdd32(saturatingAdd32(scroll.Offset.Y, viewportHeight), overscan)
	builtHeight := builtLimit - min(builtLimit, builtOffset)
	builtStart, builtEnd := f.heights.cellRange(builtOffset, builtHeight)
	anchor, hasAnchor := f.captureAnchor(scroll.Offset.Y, viewportHeight, followingEnd)
	f.state = VirtualFlowState{
		Scroll: scroll, VisibleStart: visibleStart, VisibleEnd: visibleEnd, ItemCount: f.items.Len(),
	}
	if hasAnchor {
		f.state.Anchor = anchor.anchor
		f.state.HasAnchor = true
	}
	f.hasState = true
	return virtualFlowWindow{
		offset: scroll.Offset.Y, contentHeight: f.totalHeight(),
		visibleStart: visibleStart, visibleEnd: visibleEnd,
		builtStart: builtStart, builtEnd: builtEnd, generation: f.generation,
	}
}

func (f *virtualFlowInteraction) totalHeight() uint32 {
	return f.heights.total()
}

func (f *virtualFlowInteraction) isMeasured(index int) bool {
	return index >= 0 && index < len(f.measured) && f.measured[index]
}

func (f *virtualFlowInteraction) rebuildPositions() {
	f.positions = make(map[NodeID]int, f.items.Len())
	for index, item := range f.items.items() {
		f.positions[item.key] = index
	}
}

func prepareVirtualFlow[Message any](
	interaction *InteractionState,
	id NodeID,
	source VirtualFlowSource[Message],
	width, viewportHeight, overscan uint32,
	stickToEnd bool,
) virtualFlowWindow {
	if interaction.scrolls == nil {
		interaction.scrolls = make(map[NodeID]*scrollInteraction)
	}
	if interaction.virtualFlows == nil {
		interaction.virtualFlows = make(map[NodeID]*virtualFlowInteraction)
	}
	scrollSnapshot := scrollInteraction{}
	if current := interaction.scrolls[id]; current != nil {
		scrollSnapshot = *current
	}
	var preserved preservedVirtualFlowAnchor
	hasPreserved := false
	if current := interaction.virtualFlows[id]; current != nil {
		preserved, hasPreserved = current.captureAnchor(
			scrollSnapshot.state.Offset.Y,
			viewportHeight,
			scrollSnapshot.followingEnd,
		)
	}
	followsEnd := virtualFlowFollowsEndOnPrepare(scrollSnapshot, stickToEnd)
	flow := interaction.virtualFlows[id]
	if flow == nil {
		flow = &virtualFlowInteraction{}
		interaction.virtualFlows[id] = flow
	}
	reconcileVirtualFlow(flow, source, width)
	anchorOffset := uint32(0)
	if hasPreserved {
		anchorOffset = flow.resolveAnchor(preserved, viewportHeight)
	}
	maximum := ScrollOffset{Y: flow.totalHeight() - min(flow.totalHeight(), viewportHeight)}
	scroll := interaction.scrolls[id]
	if scroll == nil {
		scroll = &scrollInteraction{}
		interaction.scrolls[id] = scroll
	}
	if !scroll.hasRequest && !followsEnd && hasPreserved {
		scroll.state.Offset = ScrollOffset{Y: anchorOffset}
	}
	prepared := resolvePreparedScroll(*scroll, maximum, ScrollAxisVertical, stickToEnd)
	scroll.hasRequest = false
	scroll.axis = ScrollAxisVertical
	scroll.stickToEnd = stickToEnd
	scroll.state = prepared.state
	scroll.followingEnd = prepared.followingEnd
	scroll.initialized = true
	return flow.resolveWindow(prepared.state, viewportHeight, overscan, prepared.followingEnd)
}

func applyVirtualFlowMeasurements(
	interaction *InteractionState,
	id NodeID,
	measurements []virtualFlowMeasurement,
	viewportHeight, overscan uint32,
	stickToEnd bool,
) (virtualFlowWindow, bool) {
	flow := interaction.virtualFlows[id]
	if flow == nil {
		return virtualFlowWindow{}, false
	}
	scroll := interaction.scrolls[id]
	if scroll == nil {
		return virtualFlowWindow{}, false
	}
	scrollSnapshot := *scroll
	preserved, hasPreserved := flow.captureAnchor(
		scrollSnapshot.state.Offset.Y,
		viewportHeight,
		scrollSnapshot.followingEnd,
	)
	changed := false
	for _, measurement := range measurements {
		changed = flow.setMeasured(measurement.index, measurement.height) || changed
	}
	if !changed {
		return flow.resolveWindow(
			scrollSnapshot.state,
			viewportHeight,
			overscan,
			scrollSnapshot.followingEnd,
		), true
	}
	followsEnd := virtualFlowFollowsEndOnPrepare(scrollSnapshot, stickToEnd)
	anchorOffset := uint32(0)
	if hasPreserved {
		anchorOffset = flow.resolveAnchor(preserved, viewportHeight)
	}
	maximum := ScrollOffset{Y: flow.totalHeight() - min(flow.totalHeight(), viewportHeight)}
	if !scroll.hasRequest && !followsEnd && hasPreserved {
		scroll.state.Offset = ScrollOffset{Y: anchorOffset}
	}
	prepared := resolvePreparedScroll(*scroll, maximum, ScrollAxisVertical, stickToEnd)
	scroll.hasRequest = false
	scroll.axis = ScrollAxisVertical
	scroll.stickToEnd = stickToEnd
	scroll.state = prepared.state
	scroll.followingEnd = prepared.followingEnd
	scroll.initialized = true
	return flow.resolveWindow(prepared.state, viewportHeight, overscan, prepared.followingEnd), true
}

func virtualFlowItemLayout(
	interaction *InteractionState,
	id NodeID,
	index int,
) (origin, height uint32, measured, ok bool) {
	flow := interaction.virtualFlows[id]
	if flow == nil || index < 0 || index >= flow.items.Len() {
		return 0, 0, false, false
	}
	return flow.heights.prefix(index), flow.heights.height(index), flow.isMeasured(index), true
}

func virtualFlowFollowsEndOnPrepare(scroll scrollInteraction, stickToEnd bool) bool {
	return (!scroll.initialized && stickToEnd) ||
		(scroll.initialized && stickToEnd &&
			(scroll.followingEnd || (!scroll.stickToEnd && scroll.state.AtEnd)))
}

type virtualFlowMeasurement struct {
	index  int
	height uint32
}

type virtualFlowHeightIndex struct {
	heights []uint32
	tree    []uint64
}

func newVirtualFlowHeightIndex(heights []uint32) virtualFlowHeightIndex {
	index := virtualFlowHeightIndex{
		heights: append([]uint32(nil), heights...),
		tree:    make([]uint64, len(heights)+1),
	}
	for position, height := range index.heights {
		index.add(position, uint64(height))
	}
	return index
}

func (i *virtualFlowHeightIndex) height(index int) uint32 {
	if index < 0 || index >= len(i.heights) {
		return 0
	}
	return i.heights[index]
}

func (i *virtualFlowHeightIndex) update(index int, height uint32) bool {
	if index < 0 || index >= len(i.heights) || i.heights[index] == height {
		return false
	}
	previous := i.heights[index]
	i.heights[index] = height
	if height > previous {
		i.add(index, uint64(height-previous))
	} else {
		i.subtract(index, uint64(previous-height))
	}
	return true
}

func (i *virtualFlowHeightIndex) total() uint32 {
	return clampUint64ToUint32(i.sum(len(i.heights)))
}

func (i *virtualFlowHeightIndex) prefix(end int) uint32 {
	return clampUint64ToUint32(i.sum(min(max(end, 0), len(i.heights))))
}

func (i *virtualFlowHeightIndex) itemAt(offset uint32) (int, bool) {
	if len(i.heights) == 0 || uint64(offset) >= i.sum(len(i.heights)) {
		return 0, false
	}
	target := uint64(offset)
	position := 0
	accumulated := uint64(0)
	step := 1
	for step < len(i.tree) {
		step <<= 1
	}
	for step > 0 {
		next := position + step
		if next < len(i.tree) && accumulated+i.tree[next] <= target {
			accumulated += i.tree[next]
			position = next
		}
		step >>= 1
	}
	return min(position, len(i.heights)-1), true
}

func (i *virtualFlowHeightIndex) cellRange(offset, extent uint32) (int, int) {
	if extent == 0 || len(i.heights) == 0 {
		return 0, 0
	}
	total := i.total()
	if offset >= total {
		return len(i.heights), len(i.heights)
	}
	start, _ := i.itemAt(offset)
	endOffset := min(saturatingAdd32(offset, extent), total)
	end := start
	if endOffset > 0 {
		if index, ok := i.itemAt(endOffset - 1); ok {
			end = index + 1
		} else {
			end = len(i.heights)
		}
	}
	return start, max(start, end)
}

func (i *virtualFlowHeightIndex) sum(end int) uint64 {
	total := uint64(0)
	for end > 0 {
		total += i.tree[end]
		end &= end - 1
	}
	return total
}

func (i *virtualFlowHeightIndex) add(index int, value uint64) {
	for position := index + 1; position < len(i.tree); position += position & -position {
		i.tree[position] += value
	}
}

func (i *virtualFlowHeightIndex) subtract(index int, value uint64) {
	for position := index + 1; position < len(i.tree); position += position & -position {
		i.tree[position] -= value
	}
}

func clampUint64ToUint32(value uint64) uint32 {
	return uint32(min(value, uint64(^uint32(0))))
}
