package widget

const defaultTextAreaHistoryLimit = 100

// TextAreaHistory stores bounded application-owned undo and redo state
//
// Record stores content changes as undo steps while cursor, selection, and
// viewport-only changes update the current state without creating a step.
type TextAreaHistory struct {
	current TextAreaState
	undo    []TextAreaState
	redo    []TextAreaState
	limit   int
}

// NewTextAreaHistory returns history retaining up to 100 content changes
func NewTextAreaHistory(initial TextAreaState) *TextAreaHistory {
	return NewTextAreaHistoryWithLimit(initial, defaultTextAreaHistoryLimit)
}

// NewTextAreaHistoryWithLimit returns history with a bounded undo stack
//
// A non-positive limit disables undo recording.
func NewTextAreaHistoryWithLimit(initial TextAreaState, limit int) *TextAreaHistory {
	return &TextAreaHistory{current: normalizeTextAreaState(initial), limit: max(limit, 0)}
}

// Current returns the current editor state
func (h *TextAreaHistory) Current() TextAreaState {
	if h == nil {
		return TextAreaState{}
	}
	return h.current
}

// Record makes next current and records a content change as one undo step
func (h *TextAreaHistory) Record(next TextAreaState) {
	if h == nil {
		return
	}
	next = normalizeTextAreaState(next)
	if next == h.current {
		return
	}
	if next.value != h.current.value {
		if h.limit > 0 {
			h.undo = append(h.undo, h.current)
			if extra := len(h.undo) - h.limit; extra > 0 {
				copy(h.undo, h.undo[extra:])
				h.undo = h.undo[:h.limit]
			}
		}
		h.redo = nil
	}
	h.current = next
}

// Undo restores the previous content state when one is available
func (h *TextAreaHistory) Undo() (TextAreaState, bool) {
	if h == nil || len(h.undo) == 0 {
		return TextAreaState{}, false
	}
	previous := h.undo[len(h.undo)-1]
	h.undo = h.undo[:len(h.undo)-1]
	h.redo = append(h.redo, h.current)
	h.current = previous
	return previous, true
}

// Redo reapplies the next content state when one is available
func (h *TextAreaHistory) Redo() (TextAreaState, bool) {
	if h == nil || len(h.redo) == 0 {
		return TextAreaState{}, false
	}
	next := h.redo[len(h.redo)-1]
	h.redo = h.redo[:len(h.redo)-1]
	if h.limit > 0 {
		h.undo = append(h.undo, h.current)
		if extra := len(h.undo) - h.limit; extra > 0 {
			copy(h.undo, h.undo[extra:])
			h.undo = h.undo[:h.limit]
		}
	}
	h.current = next
	return next, true
}

// Reset replaces current state and clears both history stacks
func (h *TextAreaHistory) Reset(state TextAreaState) {
	if h == nil {
		return
	}
	h.current = normalizeTextAreaState(state)
	h.undo = nil
	h.redo = nil
}
