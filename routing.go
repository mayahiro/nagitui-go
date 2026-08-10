package tui

import "github.com/mayahiro/nagi-go/vt"

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
	redraw        bool
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

// Consumed reports whether a handler consumed the event
func (d EventDispatch) Consumed() bool {
	return d.consumed
}

// Messages returns the number of application messages enqueued by handlers
func (d EventDispatch) Messages() int {
	return d.messages
}

// RedrawRequested reports whether routing explicitly requested a frame
func (d EventDispatch) RedrawRequested() bool {
	return d.redraw
}

type interactiveKind uint8

const (
	interactiveGeneric interactiveKind = iota
	interactiveTextInput
	interactiveScrollViewport
	interactiveModal
)

type nodeRecord struct {
	id         NodeID
	parent     NodeID
	hasParent  bool
	rect       Rect
	clip       Rect
	focusable  bool
	hasHandler bool
	kind       interactiveKind
}

type treeIndex struct {
	records     []nodeRecord
	byID        map[NodeID]int
	focusOrder  []NodeID
	active      map[NodeID]struct{}
	root        NodeID
	hasRoot     bool
	activeModal NodeID
	hasModal    bool
}

func newTreeIndex() treeIndex {
	return treeIndex{byID: make(map[NodeID]int), active: make(map[NodeID]struct{})}
}

func (t *treeIndex) reset() {
	clear(t.records)
	clear(t.focusOrder)
	t.records = t.records[:0]
	t.focusOrder = t.focusOrder[:0]
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
	}
	if record.focusable {
		t.focusOrder = append(t.focusOrder, record.id)
	}
	t.active[record.id] = struct{}{}
	t.byID[record.id] = len(t.records)
	t.records = append(t.records, record)
	return nil
}

func (t *treeIndex) record(id NodeID) (nodeRecord, bool) {
	index, ok := t.byID[id]
	if !ok {
		return nodeRecord{}, false
	}
	return t.records[index], true
}

func (t *treeIndex) route(target NodeID, hasTarget bool) []NodeID {
	if t.hasModal && (!hasTarget || !t.isWithin(target, t.activeModal)) {
		target = t.activeModal
		hasTarget = true
	}
	return t.rawRoute(target, hasTarget)
}

func (t *treeIndex) rawRoute(target NodeID, hasTarget bool) []NodeID {
	var route []NodeID
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

func (t *treeIndex) allowsFocus(id NodeID) bool {
	record, ok := t.record(id)
	return ok && record.focusable && t.allowsInteraction(id)
}

func (t *treeIndex) allowsInteraction(id NodeID) bool {
	_, active := t.active[id]
	return active && (!t.hasModal || t.isWithin(id, t.activeModal))
}

func (t *treeIndex) isWithin(id, ancestor NodeID) bool {
	visited := make(map[NodeID]struct{})
	current := id
	for {
		if current == ancestor {
			return true
		}
		if _, duplicate := visited[current]; duplicate {
			return false
		}
		visited[current] = struct{}{}
		record, ok := t.record(current)
		if !ok || !record.hasParent {
			return false
		}
		current = record.parent
	}
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
