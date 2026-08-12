package tui

import (
	"sort"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
)

// DuplicateNodeIDError indicates that one semantic tree reused a stable ID
type DuplicateNodeIDError struct {
	// ID is the duplicated application-defined identity
	ID NodeID
}

// Error returns the duplicate identity diagnostic
func (e *DuplicateNodeIDError) Error() string {
	return "duplicate NodeID " + e.ID.String()
}

type focusChangeKind uint8

const (
	focusUnchanged focusChangeKind = iota
	focusSet
	focusRelease
)

type pointerChangeKind uint8

const (
	pointerUnchanged pointerChangeKind = iota
	pointerCapture
	pointerRelease
)

// EventResult is the composable result of one semantic node event handler
type EventResult[Message any] struct {
	messages      []Message
	consumed      bool
	focusChange   focusChangeKind
	focusID       NodeID
	pointerChange pointerChangeKind
	pointerID     NodeID
	scrollID      NodeID
	scrollOffset  ScrollOffset
	hasScroll     bool
	redraw        bool
}

// TextHit is one semantic text grapheme or collapsed line boundary under a pointer cell
type TextHit struct {
	start int
	end   int
}

func newTextHit(start, end int) TextHit {
	return TextHit{start: start, end: end}
}

// Start returns the inclusive UTF-8 byte boundary before the hit grapheme
func (h TextHit) Start() int {
	return h.start
}

// End returns the exclusive UTF-8 byte boundary after the hit grapheme
//
// Empty visual regions such as a line boundary have equal Start and End
func (h TextHit) End() int {
	return h.end
}

// PointerViewport is the nearest ancestor ScrollViewport available to a pointer handler
type PointerViewport struct {
	id      NodeID
	axis    ScrollAxis
	state   ScrollState
	visible Rect
}

// ID returns the stable ScrollViewport identity
func (v PointerViewport) ID() NodeID {
	return v.id
}

// Axis returns the axes controlled by the ScrollViewport
func (v PointerViewport) Axis() ScrollAxis {
	return v.axis
}

// State returns the resolved ScrollViewport state at dispatch time
func (v PointerViewport) State() ScrollState {
	return v.state
}

// VisibleRect returns the globally positioned visible ScrollViewport rectangle
func (v PointerViewport) VisibleRect() Rect {
	return v.visible
}

func (v PointerViewport) edgeScrollOffset(event vt.MouseEvent) (ScrollOffset, bool) {
	if event.Kind != vt.MouseMove || v.visible.Empty() {
		return ScrollOffset{}, false
	}
	next := v.state.Offset
	x, y := int64(event.X), int64(event.Y)
	left, top := int64(v.visible.X), int64(v.visible.Y)
	right := left + int64(v.visible.Width)
	bottom := top + int64(v.visible.Height)
	if v.axis.allowsHorizontal() {
		switch {
		case x <= left:
			next.X -= min(next.X, uint32(1))
		case x >= right-1:
			next.X = min(saturatingAdd32(next.X, 1), v.state.Maximum.X)
		}
	}
	if v.axis.allowsVertical() {
		switch {
		case y <= top:
			next.Y -= min(next.Y, uint32(1))
		case y >= bottom-1:
			next.Y = min(saturatingAdd32(next.Y, 1), v.state.Maximum.Y)
		}
	}
	return next, next != v.state.Offset
}

// PointerEventContext contains geometry and resolved text information for one routed mouse event
type PointerEventContext struct {
	event         vt.MouseEvent
	localPosition Point
	bounds        Size
	visibleBounds Rect
	widthProfile  celltext.WidthProfile
	captured      bool
	textHit       TextHit
	hasTextHit    bool
	viewport      PointerViewport
	hasViewport   bool
}

// Event returns the normalized zero-based terminal mouse event
func (c PointerEventContext) Event() vt.MouseEvent {
	return c.event
}

// LocalPosition returns the pointer cell relative to the routed Node origin
//
// Pointer capture may produce coordinates outside Bounds
func (c PointerEventContext) LocalPosition() Point {
	return c.localPosition
}

// Bounds returns the routed Node size
func (c PointerEventContext) Bounds() Size {
	return c.bounds
}

// VisibleBounds returns the Node-visible rectangle in Node-local coordinates
func (c PointerEventContext) VisibleBounds() Rect {
	return c.visibleBounds
}

// WidthProfile returns the Runtime terminal cell-width policy
func (c PointerEventContext) WidthProfile() celltext.WidthProfile {
	return c.widthProfile
}

// IsCaptured reports whether this Node owns pointer capture for the event
func (c PointerEventContext) IsCaptured() bool {
	return c.captured
}

// TextHit returns the rendered paragraph grapheme or line boundary under the pointer
//
// Non-paragraph Nodes return false
func (c PointerEventContext) TextHit() (TextHit, bool) {
	return c.textHit, c.hasTextHit
}

// Viewport returns the nearest ancestor ScrollViewport, when one exists
func (c PointerEventContext) Viewport() (PointerViewport, bool) {
	return c.viewport, c.hasViewport
}

// EdgeScroll returns a one-cell scroll request for a Move at a visible edge
//
// The result is clamped to the current ScrollViewport maximum This method
// does not start a timer and returns false for other event kinds, away from an
// enabled edge, or when the viewport cannot move farther
func (c PointerEventContext) EdgeScroll() (NodeID, ScrollOffset, bool) {
	if !c.hasViewport {
		return "", ScrollOffset{}, false
	}
	offset, ok := c.viewport.edgeScrollOffset(c.event)
	return c.viewport.id, offset, ok
}

// IgnoreResult returns an ignored result that continues ancestor routing
func IgnoreResult[Message any]() EventResult[Message] {
	return EventResult[Message]{}
}

// ConsumeResult returns a consumed result without a message
func ConsumeResult[Message any]() EventResult[Message] {
	return EventResult[Message]{consumed: true}
}

// MessageResult returns a consumed result that emits one application message
func MessageResult[Message any](message Message) EventResult[Message] {
	return EventResult[Message]{messages: []Message{message}, consumed: true}
}

// Emit adds an application message to the result
func (r EventResult[Message]) Emit(message Message) EventResult[Message] {
	r.messages = append(r.messages, message)
	return r
}

// Consume stops ancestor routing after applying the result
func (r EventResult[Message]) Consume() EventResult[Message] {
	r.consumed = true
	return r
}

// Focus requests focus for a stable Node ID
func (r EventResult[Message]) Focus(id NodeID) EventResult[Message] {
	r.focusChange = focusSet
	r.focusID = id
	return r
}

// ReleaseFocus releases node focus
func (r EventResult[Message]) ReleaseFocus() EventResult[Message] {
	r.focusChange = focusRelease
	return r
}

// CapturePointer captures pointer routing for a stable Node ID
func (r EventResult[Message]) CapturePointer(id NodeID) EventResult[Message] {
	r.pointerChange = pointerCapture
	r.pointerID = id
	return r
}

// ReleasePointer releases pointer capture
func (r EventResult[Message]) ReleasePointer() EventResult[Message] {
	r.pointerChange = pointerRelease
	return r
}

// ScrollTo requests a ScrollViewport offset during this event dispatch
//
// The latest request in one result wins The Runtime clamps the request and
// queues a resulting ScrollViewport callback after explicit messages
func (r EventResult[Message]) ScrollTo(id NodeID, offset ScrollOffset) EventResult[Message] {
	r.scrollID = id
	r.scrollOffset = offset
	r.hasScroll = true
	return r
}

// Redraw requests a frame even without an application message
func (r EventResult[Message]) Redraw() EventResult[Message] {
	r.redraw = true
	return r
}

// EventDispatch is the observable outcome of routing one normalized event
type EventDispatch struct {
	consumed bool
	messages int
	redraw   bool
}

// Consumed reports whether routing consumed the event
func (d EventDispatch) Consumed() bool {
	return d.consumed
}

// Messages returns the number of application messages enqueued during routing
//
// This includes an optional ScrollViewport callback produced by a changed
// event-local scroll request
func (d EventDispatch) Messages() int {
	return d.messages
}

// RedrawRequested reports whether routing made an urgent frame necessary
func (d EventDispatch) RedrawRequested() bool {
	return d.redraw
}

type interactiveKind uint8

const (
	interactiveGeneric interactiveKind = iota
	interactiveTextInput
	interactiveScrollViewportVertical
	interactiveScrollViewportHorizontal
	interactiveModal
)

func scrollInteractiveKind(axis ScrollAxis) interactiveKind {
	if axis == ScrollAxisHorizontal {
		return interactiveScrollViewportHorizontal
	}
	return interactiveScrollViewportVertical
}

func (k interactiveKind) scrollAxis() (ScrollAxis, bool) {
	switch k {
	case interactiveScrollViewportVertical:
		return ScrollAxisVertical, true
	case interactiveScrollViewportHorizontal:
		return ScrollAxisHorizontal, true
	default:
		return 0, false
	}
}

func (k interactiveKind) isScrollViewport() bool {
	_, ok := k.scrollAxis()
	return ok
}

type nodeRecord struct {
	id              NodeID
	parent          NodeID
	hasParent       bool
	rect            Rect
	clip            Rect
	focusable       bool
	hasHandler      bool
	blocksUnhandled bool
	kind            interactiveKind
}

type treeIndex struct {
	records          []nodeRecord
	byID             map[NodeID]int
	focusOrder       []NodeID
	revealTargets    []revealTarget
	focusFallbacks   []focusFallbackRecord
	active           map[NodeID]struct{}
	root             NodeID
	hasRoot          bool
	activeModal      NodeID
	hasModal         bool
	activeModalFocus ModalFocusOptions
}

type revealTarget struct {
	viewport NodeID
	target   NodeID
}

type focusFallbackRecord struct {
	record int
	target NodeID
}

func newTreeIndex() treeIndex {
	return treeIndex{byID: make(map[NodeID]int), active: make(map[NodeID]struct{})}
}

func (t *treeIndex) reset() {
	clear(t.records)
	clear(t.focusOrder)
	clear(t.revealTargets)
	clear(t.focusFallbacks)
	t.records = t.records[:0]
	t.focusOrder = t.focusOrder[:0]
	t.revealTargets = t.revealTargets[:0]
	t.focusFallbacks = t.focusFallbacks[:0]
	if t.byID == nil {
		t.byID = make(map[NodeID]int)
	} else {
		clear(t.byID)
	}
	if t.active == nil {
		t.active = make(map[NodeID]struct{})
	} else {
		clear(t.active)
	}
	t.root = ""
	t.hasRoot = false
	t.activeModal = ""
	t.hasModal = false
	t.activeModalFocus = DefaultModalFocusOptions()
}

func (t *treeIndex) register(record nodeRecord, root bool) error {
	if _, duplicate := t.byID[record.id]; duplicate {
		return &DuplicateNodeIDError{ID: record.id}
	}
	if root {
		t.root = record.id
		t.hasRoot = true
	}
	if record.kind == interactiveModal {
		t.activeModal = record.id
		t.hasModal = true
		t.activeModalFocus = DefaultModalFocusOptions()
	}
	if record.focusable {
		t.focusOrder = append(t.focusOrder, record.id)
	}
	t.active[record.id] = struct{}{}
	t.byID[record.id] = len(t.records)
	t.records = append(t.records, record)
	return nil
}

func (t *treeIndex) setActiveModalFocus(id NodeID, focus ModalFocusOptions) {
	if t.hasModal && t.activeModal == id {
		t.activeModalFocus = focus
	}
}

func (t *treeIndex) registerFocusFallback(id, target NodeID) {
	if index, ok := t.byID[id]; ok {
		t.focusFallbacks = append(t.focusFallbacks, focusFallbackRecord{record: index, target: target})
	}
}

func (t *treeIndex) record(id NodeID) (nodeRecord, bool) {
	index, ok := t.byID[id]
	if !ok {
		return nodeRecord{}, false
	}
	return t.records[index], true
}

func (t *treeIndex) focusFallback(id NodeID) (NodeID, bool) {
	record, ok := t.byID[id]
	if !ok {
		return "", false
	}
	position := sort.Search(len(t.focusFallbacks), func(index int) bool {
		return t.focusFallbacks[index].record >= record
	})
	if position == len(t.focusFallbacks) || t.focusFallbacks[position].record != record {
		return "", false
	}
	return t.focusFallbacks[position].target, true
}

func (t *treeIndex) hasExplicitReveal(viewport NodeID) bool {
	for _, target := range t.revealTargets {
		if target.viewport == viewport {
			return true
		}
	}
	return false
}

func (t *treeIndex) route(target NodeID, hasTarget bool) []NodeID {
	return t.routeInto(target, hasTarget, nil)
}

func (t *treeIndex) routeInto(target NodeID, hasTarget bool, route []NodeID) []NodeID {
	if t.hasModal && (!hasTarget || !t.isWithin(target, t.activeModal)) {
		target = t.activeModal
		hasTarget = true
	}
	return t.rawRouteInto(target, hasTarget, route)
}

func (t *treeIndex) rawRoute(target NodeID, hasTarget bool) []NodeID {
	return t.rawRouteInto(target, hasTarget, nil)
}

func (t *treeIndex) rawRouteInto(target NodeID, hasTarget bool, route []NodeID) []NodeID {
	route = route[:0]
	current := target
	remaining := len(t.records) + 1
	for hasTarget && remaining > 0 {
		remaining--
		route = append(route, current)
		record, ok := t.record(current)
		if !ok || !record.hasParent {
			break
		}
		current = record.parent
	}
	if t.hasRoot && !containsNodeID(route, t.root) {
		route = append(route, t.root)
	}
	return route
}

func (t *treeIndex) hitTest(point Point) (NodeID, bool) {
	for index := len(t.records) - 1; index >= 0; index-- {
		record := t.records[index]
		if t.hasModal && !t.isWithin(record.id, t.activeModal) {
			continue
		}
		if record.rect.Intersection(record.clip).Contains(point) &&
			(record.hasHandler || record.focusable || record.kind != interactiveGeneric) {
			return record.id, true
		}
	}
	if t.hasModal {
		return t.activeModal, true
	}
	return "", false
}

func (t *treeIndex) focusScope() []NodeID {
	if !t.hasModal {
		return t.focusOrder
	}
	focus := make([]NodeID, 0, len(t.focusOrder))
	for _, id := range t.focusOrder {
		if t.isWithin(id, t.activeModal) {
			focus = append(focus, id)
		}
	}
	return focus
}

func (t *treeIndex) focusActionOwner(focused NodeID, hasFocus bool) (NodeID, bool) {
	if hasFocus {
		return focused, true
	}
	if t.hasModal {
		return t.activeModal, true
	}
	return t.root, t.hasRoot
}

func (t *treeIndex) allowsFocus(id NodeID) bool {
	record, ok := t.record(id)
	return ok && record.focusable && t.allowsInteraction(id)
}

func (t *treeIndex) allowsInteraction(id NodeID) bool {
	_, active := t.active[id]
	return active && (!t.hasModal || t.isWithin(id, t.activeModal))
}

func (t *treeIndex) isWithin(id, ancestor NodeID) bool {
	current := id
	remaining := len(t.records) + 1
	for remaining > 0 {
		remaining--
		if current == ancestor {
			return true
		}
		record, ok := t.record(current)
		if !ok || !record.hasParent {
			return false
		}
		current = record.parent
	}
	return false
}

func routePath(parents map[NodeID]*NodeID, root, target *NodeID) []NodeID {
	var route []NodeID
	visited := make(map[NodeID]struct{})
	current := target
	for current != nil {
		id := *current
		if _, duplicate := visited[id]; duplicate {
			break
		}
		visited[id] = struct{}{}
		route = append(route, id)
		current = parents[id]
	}
	if root != nil && !containsNodeID(route, *root) {
		route = append(route, *root)
	}
	return route
}

type eventHandler[Message any] func(vt.Event) EventResult[Message]
type pointerEventHandler[Message any] func(PointerEventContext) EventResult[Message]
