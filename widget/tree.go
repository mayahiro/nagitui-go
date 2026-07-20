package widget

import (
	"strings"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

// TreeItem is one preorder item rendered by a Tree
type TreeItem struct {
	// ID is the application-defined stable identity of this item
	ID tui.NodeID
	// Label is the displayed item text
	Label string
	// Depth is the zero-based preorder depth
	Depth uint16
	// HasChildren reports whether the item represents a branch
	HasChildren bool
	// Expanded is application-owned branch expansion state
	Expanded bool
}

// NewTreeLeaf returns a leaf at the supplied zero-based depth
func NewTreeLeaf(id tui.NodeID, label string, depth uint16) TreeItem {
	return TreeItem{ID: id, Label: label, Depth: depth}
}

// NewTreeBranch returns a branch with application-owned expansion state
func NewTreeBranch(id tui.NodeID, label string, depth uint16, expanded bool) TreeItem {
	return TreeItem{ID: id, Label: label, Depth: depth, HasChildren: true, Expanded: expanded}
}

// TreeStyle contains the visual styles used by a Tree
type TreeStyle struct {
	// Normal is used by unselected visible items
	Normal vt.Style
	// Selected is used by the application-selected item
	Selected vt.Style
	// Focused is merged over the selected item while the tree owns focus
	Focused vt.Style
	// Disabled is used by every item while the tree is disabled
	Disabled vt.Style
}

// DefaultTreeStyle returns the standard tree styles
func DefaultTreeStyle() TreeStyle {
	return TreeStyle{
		Selected: vt.Style{Reverse: true}, Focused: vt.Style{Underline: true},
		Disabled: vt.Style{Dim: true},
	}
}

// Tree is a preorder tree with application-owned selection and expansion state
type Tree[Message any] struct {
	id             tui.NodeID
	items          []TreeItem
	selected       int
	viewportHeight int
	enabled        bool
	style          TreeStyle
	onSelect       func(int) Message
	onToggle       func(int, bool) Message
}

// NewTree returns an enabled tree using an original preorder selection index
//
// A nil onSelect function creates a disabled tree
func NewTree[Message any](id tui.NodeID, items []TreeItem, selected int, onSelect func(int) Message) Tree[Message] {
	return Tree[Message]{
		id: id, items: append([]TreeItem(nil), items...), selected: selected,
		enabled: onSelect != nil, style: DefaultTreeStyle(), onSelect: onSelect,
	}
}

// OnToggle sets the handler that receives original preorder index and next expansion state
//
// A nil function removes the expansion handler
func (t Tree[Message]) OnToggle(handler func(int, bool) Message) Tree[Message] {
	t.onToggle = handler
	return t
}

// Enabled sets whether the tree can receive focus and emit messages
func (t Tree[Message]) Enabled(enabled bool) Tree[Message] {
	t.enabled = enabled && t.onSelect != nil
	return t
}

// Style replaces the tree styles
func (t Tree[Message]) Style(style TreeStyle) Tree[Message] {
	t.style = style
	return t
}

// Viewport limits rendering to a deterministic window that follows selection
//
// A non-positive height disables the viewport. In viewport mode the Tree root
// ID remains the single stable keyboard focus target as the window moves.
func (t Tree[Message]) Viewport(height int) Tree[Message] {
	t.viewportHeight = max(height, 0)
	return t
}

// Node builds the public semantic node for this tree
func (t Tree[Message]) Node() tui.Node[Message] {
	visible := treeVisibleIndices(t.items)
	selectedPosition, hasSelection := normalizedTreeSelection(visible, t.selected)
	viewport := t.viewportHeight > 0
	start, end := 0, len(visible)
	if viewport && hasSelection {
		start, end = treeViewportRange(len(visible), selectedPosition, t.viewportHeight)
	} else if viewport {
		start, end = treeViewportRange(len(visible), 0, t.viewportHeight)
	}
	children := make([]tui.Node[Message], 0, end-start)

	navigate := func(event vt.Event) tui.EventResult[Message] {
		if isActivationEvent(event) {
			result := tui.ConsumeResult[Message]().Focus(t.id)
			if hasSelection {
				originalIndex := visible[selectedPosition]
				item := t.items[originalIndex]
				if item.HasChildren && t.onToggle != nil {
					result = result.Emit(t.onToggle(originalIndex, !item.Expanded))
				}
			}
			return result
		}
		action, handled := treeNavigationEvent(event, visible, t.items, selectedPosition)
		if !handled {
			return tui.IgnoreResult[Message]()
		}
		if action.toggle {
			result := tui.ConsumeResult[Message]().Focus(t.id)
			if t.onToggle != nil {
				result = result.Emit(t.onToggle(visible[selectedPosition], action.expanded))
			}
			return result
		}
		result := tui.ConsumeResult[Message]().Focus(t.id)
		if action.position != selectedPosition {
			result = result.Emit(t.onSelect(visible[action.position]))
		}
		return result
	}

	for position := start; position < end; position++ {
		originalIndex := visible[position]
		item := t.items[originalIndex]
		isSelected := hasSelection && position == selectedPosition
		style := t.style.Normal
		if isSelected {
			style = t.style.Selected
		}
		if !t.enabled {
			style = t.style.Disabled
		}
		disclosure := "  "
		if item.HasChildren && item.Expanded {
			disclosure = "▼ "
		} else if item.HasChildren {
			disclosure = "▶ "
		}
		content := strings.Repeat("  ", int(item.Depth)) + disclosure + item.Label
		node := tui.StyledText[Message](content, style)
		if !t.enabled {
			children = append(children, node.WithID(item.ID))
			continue
		}
		selection := originalIndex
		itemID := item.ID
		row := node.WithID(itemID).
			OnEvent(itemID, func(event vt.Event) tui.EventResult[Message] {
				if !isActivationEvent(event) {
					return tui.IgnoreResult[Message]()
				}
				result := tui.ConsumeResult[Message]().Focus(t.id)
				if !isSelected {
					result = result.Emit(t.onSelect(selection))
				}
				if item.HasChildren && t.onToggle != nil {
					result = result.Emit(t.onToggle(selection, !item.Expanded))
				}
				return result
			})
		if isSelected {
			children = append(children, tui.Column(row).
				Focusable(t.id).
				WithFocusedStyle(t.style.Focused).
				OnEvent(t.id, navigate))
			continue
		}
		children = append(children, row)
	}

	root := tui.Column(children...)
	if viewport {
		root = root.WithLength(tui.Fixed(uint32(t.viewportHeight)))
	}
	if t.enabled && hasSelection {
		return root
	}
	return root.WithID(t.id)
}

type treeAction struct {
	position int
	toggle   bool
	expanded bool
}

func treeVisibleIndices(items []TreeItem) []int {
	visible := make([]int, 0, len(items))
	collapsedDepth := uint16(0)
	hasCollapsedDepth := false
	for index, item := range items {
		if hasCollapsedDepth {
			if item.Depth > collapsedDepth {
				continue
			}
			hasCollapsedDepth = false
		}
		visible = append(visible, index)
		if item.HasChildren && !item.Expanded {
			collapsedDepth = item.Depth
			hasCollapsedDepth = true
		}
	}
	return visible
}

func normalizedTreeSelection(visible []int, selected int) (int, bool) {
	if len(visible) == 0 {
		return 0, false
	}
	selected = max(selected, 0)
	position := 0
	for index, original := range visible {
		if original > selected {
			break
		}
		position = index
	}
	return position, true
}

func treeViewportRange(count, selected, height int) (start, end int) {
	if count <= 0 || height <= 0 {
		return 0, max(count, 0)
	}
	height = min(height, count)
	selected = min(max(selected, 0), count-1)
	start = selected - height/2
	start = min(max(start, 0), count-height)
	return start, start + height
}

func treeNavigationEvent(event vt.Event, visible []int, items []TreeItem, selected int) (treeAction, bool) {
	if event.Kind != vt.EventKey || event.Key.Action == vt.KeyRelease {
		return treeAction{}, false
	}
	modifiers := event.Key.Modifiers
	if modifiers.Alt || modifiers.Control || modifiers.Meta {
		return treeAction{}, false
	}
	switch event.Key.Code {
	case vt.KeyUp, vt.KeyDown, vt.KeyHome, vt.KeyEnd:
		action := navigationUp
		switch event.Key.Code {
		case vt.KeyDown:
			action = navigationDown
		case vt.KeyHome:
			action = navigationHome
		case vt.KeyEnd:
			action = navigationEnd
		}
		next, _ := navigateSelection(len(visible), selected, action)
		return treeAction{position: next}, true
	case vt.KeyLeft:
		current := items[visible[selected]]
		if current.HasChildren && current.Expanded {
			return treeAction{toggle: true, expanded: false}, true
		}
		for position := selected - 1; position >= 0; position-- {
			if items[visible[position]].Depth < current.Depth {
				return treeAction{position: position}, true
			}
		}
		return treeAction{position: selected}, true
	case vt.KeyRight:
		current := items[visible[selected]]
		if current.HasChildren && !current.Expanded {
			return treeAction{toggle: true, expanded: true}, true
		}
		child := selected + 1
		if current.HasChildren && child < len(visible) && items[visible[child]].Depth > current.Depth {
			return treeAction{position: child}, true
		}
		return treeAction{position: selected}, true
	default:
		return treeAction{}, false
	}
}
