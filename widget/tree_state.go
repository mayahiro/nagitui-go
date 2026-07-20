package widget

import "github.com/mayahiro/nagitui-go"

// TreeState stores application-owned selection and expanded branch identities
type TreeState struct {
	selected int
	expanded map[tui.NodeID]bool
}

// NewTreeState returns collapsed state with the supplied preorder selection
func NewTreeState(selected int) *TreeState {
	return &TreeState{selected: max(selected, 0), expanded: make(map[tui.NodeID]bool)}
}

// NewTreeStateFromItems initializes state from item expansion flags
func NewTreeStateFromItems(items []TreeItem, selected int) *TreeState {
	state := NewTreeState(selected)
	for _, item := range items {
		if item.HasChildren && item.Expanded {
			state.expanded[item.ID] = true
		}
	}
	return state
}

// Selected returns the application-owned original preorder selection index
func (s *TreeState) Selected() int {
	if s == nil {
		return 0
	}
	return s.selected
}

// Select replaces the original preorder selection index
func (s *TreeState) Select(index int) {
	if s != nil {
		s.selected = max(index, 0)
	}
}

// IsExpanded reports whether a stable branch identity is expanded
func (s *TreeState) IsExpanded(id tui.NodeID) bool {
	return s != nil && s.expanded[id]
}

// SetExpanded replaces expansion state for a stable branch identity
func (s *TreeState) SetExpanded(id tui.NodeID, expanded bool) {
	if s == nil {
		return
	}
	if s.expanded == nil {
		s.expanded = make(map[tui.NodeID]bool)
	}
	if expanded {
		s.expanded[id] = true
	} else {
		delete(s.expanded, id)
	}
}

// Toggle reverses and returns expansion state for a stable branch identity
func (s *TreeState) Toggle(id tui.NodeID) bool {
	if s == nil {
		return false
	}
	next := !s.IsExpanded(id)
	s.SetExpanded(id, next)
	return next
}

// Apply returns an independent item slice using identity-based expansion state
func (s *TreeState) Apply(items []TreeItem) []TreeItem {
	output := append([]TreeItem(nil), items...)
	for index := range output {
		if output[index].HasChildren {
			output[index].Expanded = s.IsExpanded(output[index].ID)
		}
	}
	return output
}
