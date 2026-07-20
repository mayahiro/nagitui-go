package tui

// TextInputState is Interaction State retained for one TextInput node
type TextInputState struct {
	cursor int
	draft  string
}

// Cursor returns the UTF-8 byte cursor at a grapheme boundary
func (s TextInputState) Cursor() int {
	return s.cursor
}

// ScrollOffset is a two-dimensional ScrollViewport offset in cells
type ScrollOffset struct {
	// X is the horizontal content offset
	X uint32
	// Y is the vertical content offset
	Y uint32
}

// ScrollAxis selects the axes controlled by a ScrollViewport
type ScrollAxis uint8

const (
	// ScrollAxisBoth enables horizontal and vertical scrolling
	ScrollAxisBoth ScrollAxis = iota
	// ScrollAxisVertical enables vertical scrolling only
	ScrollAxisVertical
	// ScrollAxisHorizontal enables horizontal scrolling only
	ScrollAxisHorizontal
)

func (a ScrollAxis) allowsHorizontal() bool {
	return a == ScrollAxisBoth || a == ScrollAxisHorizontal
}

func (a ScrollAxis) allowsVertical() bool {
	return a == ScrollAxisBoth || a == ScrollAxisVertical
}

// ScrollState contains a resolved ScrollViewport position and boundaries
type ScrollState struct {
	// Offset is the current cell offset after clamping
	Offset ScrollOffset
	// Maximum is the greatest valid offset for the current content and viewport
	Maximum ScrollOffset
	// AtStart reports whether every enabled axis is at its beginning
	AtStart bool
	// AtEnd reports whether every enabled axis is at its end
	AtEnd bool
}

type scrollInteraction struct {
	state        ScrollState
	requested    ScrollOffset
	hasRequest   bool
	axis         ScrollAxis
	stickToEnd   bool
	followingEnd bool
	initialized  bool
}

// InteractionState is runtime-owned UI continuity keyed by stable Node IDs
type InteractionState struct {
	focused        NodeID
	hasFocus       bool
	pointerCapture NodeID
	hasCapture     bool
	textInputs     map[NodeID]*TextInputState
	scrolls        map[NodeID]*scrollInteraction
}

// NewInteractionState returns empty Interaction State
func NewInteractionState() *InteractionState {
	return &InteractionState{
		textInputs: make(map[NodeID]*TextInputState),
		scrolls:    make(map[NodeID]*scrollInteraction),
	}
}

// Focused returns the focused Node ID
func (s *InteractionState) Focused() (NodeID, bool) {
	return s.focused, s.hasFocus
}

// PointerCapture returns the node holding pointer capture
func (s *InteractionState) PointerCapture() (NodeID, bool) {
	return s.pointerCapture, s.hasCapture
}

// TextInput returns retained TextInput state for a node
func (s *InteractionState) TextInput(id NodeID) (TextInputState, bool) {
	state, ok := s.textInputs[id]
	if !ok {
		return TextInputState{}, false
	}
	return *state, true
}

// ScrollOffset returns a retained scroll offset, defaulting to zero
func (s *InteractionState) ScrollOffset(id NodeID) ScrollOffset {
	state, _ := s.ScrollState(id)
	return state.Offset
}

// ScrollState returns resolved ScrollViewport state for a node
func (s *InteractionState) ScrollState(id NodeID) (ScrollState, bool) {
	scroll, ok := s.scrolls[id]
	if !ok || !scroll.initialized {
		return ScrollState{}, false
	}
	return scroll.state, true
}

func (s *InteractionState) ensureTextInput(id NodeID, value string) {
	state, ok := s.textInputs[id]
	if !ok {
		s.textInputs[id] = &TextInputState{cursor: len(value), draft: value}
		return
	}
	if state.draft != value {
		state.draft = value
		state.cursor = normalizeTextCursor(value, state.cursor)
	}
}

func (s *InteractionState) requestScroll(id NodeID, requested ScrollOffset) (ScrollState, bool, bool) {
	scroll := s.scrolls[id]
	if scroll == nil {
		scroll = &scrollInteraction{}
		s.scrolls[id] = scroll
	}
	scroll.requested = requested
	scroll.hasRequest = true
	if !scroll.initialized {
		return ScrollState{}, false, false
	}
	previous := scroll.state
	scroll.state = resolveScrollState(scroll.axis, scroll.state.Maximum, requested)
	scroll.followingEnd = scroll.stickToEnd && scroll.state.AtEnd
	return scroll.state, scroll.state != previous, true
}

func (s *InteractionState) prepareScroll(
	id NodeID,
	maximum ScrollOffset,
	axis ScrollAxis,
	stickToEnd bool,
) ScrollState {
	scroll := s.scrolls[id]
	if scroll == nil {
		scroll = &scrollInteraction{}
		s.scrolls[id] = scroll
	}
	wasInitialized := scroll.initialized
	wasSticking := scroll.stickToEnd
	followExistingEnd := wasInitialized && stickToEnd && (scroll.followingEnd || (!wasSticking && scroll.state.AtEnd))
	maximum = normalizeScrollOffset(axis, maximum)
	requested := scroll.state.Offset
	if scroll.hasRequest {
		requested = scroll.requested
	} else if (!wasInitialized && stickToEnd) || followExistingEnd {
		requested = maximum
	}
	hadRequest := scroll.hasRequest
	scroll.hasRequest = false
	scroll.axis = axis
	scroll.stickToEnd = stickToEnd
	scroll.state = resolveScrollState(axis, maximum, requested)
	switch {
	case !stickToEnd:
		scroll.followingEnd = false
	case hadRequest:
		scroll.followingEnd = scroll.state.AtEnd
	case !wasInitialized:
		scroll.followingEnd = true
	default:
		scroll.followingEnd = followExistingEnd
	}
	scroll.initialized = true
	return scroll.state
}

func (s *InteractionState) reconcile(active map[NodeID]struct{}, previous, current []NodeID) {
	s.focused, s.hasFocus = reconcileFocus(previous, current, optionalNodeID(s.focused, s.hasFocus))
	if s.hasCapture {
		if _, ok := active[s.pointerCapture]; !ok {
			s.pointerCapture = ""
			s.hasCapture = false
		}
	}
	for id := range s.textInputs {
		if _, ok := active[id]; !ok {
			delete(s.textInputs, id)
		}
	}
	for id := range s.scrolls {
		if _, ok := active[id]; !ok {
			delete(s.scrolls, id)
		}
	}
}

func normalizeScrollOffset(axis ScrollAxis, offset ScrollOffset) ScrollOffset {
	if !axis.allowsHorizontal() {
		offset.X = 0
	}
	if !axis.allowsVertical() {
		offset.Y = 0
	}
	return offset
}

func resolveScrollState(axis ScrollAxis, maximum, requested ScrollOffset) ScrollState {
	maximum = normalizeScrollOffset(axis, maximum)
	requested = normalizeScrollOffset(axis, requested)
	offset := ScrollOffset{
		X: min(requested.X, maximum.X),
		Y: min(requested.Y, maximum.Y),
	}
	return ScrollState{
		Offset:  offset,
		Maximum: maximum,
		AtStart: (!axis.allowsHorizontal() || offset.X == 0) && (!axis.allowsVertical() || offset.Y == 0),
		AtEnd:   (!axis.allowsHorizontal() || offset.X == maximum.X) && (!axis.allowsVertical() || offset.Y == maximum.Y),
	}
}

func reconcileFocus(previous, current []NodeID, focused *NodeID) (NodeID, bool) {
	if focused == nil {
		return "", false
	}
	if containsNodeID(current, *focused) {
		return *focused, true
	}
	for index, id := range previous {
		if id != *focused {
			continue
		}
		for _, candidate := range previous[index+1:] {
			if containsNodeID(current, candidate) {
				return candidate, true
			}
		}
		for previousIndex := index - 1; previousIndex >= 0; previousIndex-- {
			candidate := previous[previousIndex]
			if containsNodeID(current, candidate) {
				return candidate, true
			}
		}
		break
	}
	if len(current) == 0 {
		return "", false
	}
	return current[0], true
}

func traverseFocus(current []NodeID, focused *NodeID, forward bool) (NodeID, bool) {
	if len(current) == 0 {
		return "", false
	}
	if focused == nil {
		if forward {
			return current[0], true
		}
		return current[len(current)-1], true
	}
	index := -1
	for candidate, id := range current {
		if id == *focused {
			index = candidate
			break
		}
	}
	if index < 0 {
		if forward {
			return current[0], true
		}
		return current[len(current)-1], true
	}
	if forward {
		return current[(index+1)%len(current)], true
	}
	return current[(index+len(current)-1)%len(current)], true
}

func clampScroll(contentWidth, contentHeight, viewportWidth, viewportHeight uint32, requested ScrollOffset) ScrollOffset {
	return ScrollOffset{
		X: min(requested.X, contentWidth-min(contentWidth, viewportWidth)),
		Y: min(requested.Y, contentHeight-min(contentHeight, viewportHeight)),
	}
}

func optionalNodeID(id NodeID, present bool) *NodeID {
	if !present {
		return nil
	}
	return &id
}

func containsNodeID(ids []NodeID, expected NodeID) bool {
	for _, id := range ids {
		if id == expected {
			return true
		}
	}
	return false
}
