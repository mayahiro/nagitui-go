package widget

import (
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

// TabItem is one stable item rendered by Tabs
type TabItem struct {
	// ID is the application-defined stable identity of this tab
	ID tui.NodeID
	// Label is the displayed tab text
	Label string
}

// NewTabItem returns a tab with a stable identity
func NewTabItem(id tui.NodeID, label string) TabItem {
	return TabItem{ID: id, Label: label}
}

// TabsStyle contains the visual styles used by Tabs
type TabsStyle struct {
	// Normal is used by unselected tabs
	Normal vt.Style
	// Selected is used by the application-selected tab
	Selected vt.Style
	// Focused is merged over the tab that owns runtime focus
	Focused vt.Style
	// Disabled is used by every tab while the set is disabled
	Disabled vt.Style
}

// DefaultTabsStyle returns the standard tab styles
func DefaultTabsStyle() TabsStyle {
	return TabsStyle{
		Selected: vt.Style{Reverse: true}, Focused: vt.Style{Underline: true},
		Disabled: vt.Style{Dim: true},
	}
}

// Tabs is a horizontal, keyboard and pointer selectable set of views
type Tabs[Message any] struct {
	id       tui.NodeID
	items    []TabItem
	selected int
	enabled  bool
	style    TabsStyle
	onSelect func(int) Message
}

// NewTabs returns enabled tabs using application-owned selection state
//
// A nil onSelect function creates disabled tabs
func NewTabs[Message any](id tui.NodeID, items []TabItem, selected int, onSelect func(int) Message) Tabs[Message] {
	return Tabs[Message]{
		id: id, items: append([]TabItem(nil), items...), selected: selected,
		enabled: onSelect != nil, style: DefaultTabsStyle(), onSelect: onSelect,
	}
}

// Enabled sets whether tabs can receive focus and emit selection messages
func (t Tabs[Message]) Enabled(enabled bool) Tabs[Message] {
	t.enabled = enabled && t.onSelect != nil
	return t
}

// Style replaces the tab styles
func (t Tabs[Message]) Style(style TabsStyle) Tabs[Message] {
	t.style = style
	return t
}

// Node builds the public semantic node for these tabs
func (t Tabs[Message]) Node() tui.Node[Message] {
	selected, hasSelection := navigateSelection(len(t.items), t.selected, navigationNormalize)
	itemIDs := make([]tui.NodeID, len(t.items))
	children := make([]tui.Node[Message], 0, len(t.items))
	for index, item := range t.items {
		itemIDs[index] = item.ID
		isSelected := hasSelection && index == selected
		content := " " + item.Label + " "
		if isSelected {
			content = "[" + item.Label + "]"
		}
		style := t.style.Normal
		if isSelected {
			style = t.style.Selected
		}
		if !t.enabled {
			style = t.style.Disabled
		}
		if !t.enabled {
			children = append(children, tui.StyledText[Message](content, style).WithID(item.ID))
			continue
		}
		selection := index
		itemID := item.ID
		children = append(children, tui.StyledText[Message](content, style).
			Focusable(itemID).
			WithFocusedStyle(t.style.Focused).
			OnEvent(itemID, func(event vt.Event) tui.EventResult[Message] {
				if !isActivationEvent(event) {
					return tui.IgnoreResult[Message]()
				}
				result := tui.ConsumeResult[Message]().Focus(itemID)
				if !isSelected {
					result = result.Emit(t.onSelect(selection))
				}
				return result
			}))
	}

	root := tui.Row(children...).WithID(t.id)
	if !t.enabled || !hasSelection {
		return root
	}
	return root.OnEvent(t.id, func(event vt.Event) tui.EventResult[Message] {
		next, handled := tabsNavigationEvent(event, len(itemIDs), selected)
		if !handled {
			return tui.IgnoreResult[Message]()
		}
		result := tui.ConsumeResult[Message]().Focus(itemIDs[next])
		if next != selected {
			result = result.Emit(t.onSelect(next))
		}
		return result
	})
}

func tabsNavigationEvent(event vt.Event, count, selected int) (int, bool) {
	if event.Kind != vt.EventKey || event.Key.Action == vt.KeyRelease {
		return 0, false
	}
	modifiers := event.Key.Modifiers
	if modifiers.Alt || modifiers.Control || modifiers.Meta {
		return 0, false
	}
	action := navigationNormalize
	switch event.Key.Code {
	case vt.KeyLeft:
		action = navigationUp
	case vt.KeyRight:
		action = navigationDown
	case vt.KeyHome:
		action = navigationHome
	case vt.KeyEnd:
		action = navigationEnd
	default:
		return 0, false
	}
	return navigateSelection(count, selected, action)
}
