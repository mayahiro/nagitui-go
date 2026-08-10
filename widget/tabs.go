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
//
// Every tab item owns the standard activation action. The root owns the
// previous, next, first, and last navigation actions. Left-button press stays
// raw so keyboard rebinding does not remove pointer selection.
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

// ItemActionDescriptor returns the semantic activation action declared by every tab item
//
// The descriptor is disabled-pass-through when the tabs are disabled or empty.
func (t Tabs[Message]) ItemActionDescriptor() tui.ActionDescriptor {
	return tabsItemActionDescriptor(t.enabled && len(t.items) > 0)
}

// NavigationActionDescriptors returns the ordered semantic navigation actions declared by the root
//
// The order is previous, next, first, and last. Every descriptor is
// disabled-pass-through when the tabs are disabled or empty.
func (t Tabs[Message]) NavigationActionDescriptors() []tui.ActionDescriptor {
	descriptors := tabsNavigationActionDescriptors(t.enabled && len(t.items) > 0)
	return append([]tui.ActionDescriptor(nil), descriptors[:]...)
}

// Node builds the public semantic node for these tabs
func (t Tabs[Message]) Node() tui.Node[Message] {
	selected, hasSelection := navigateSelection(len(t.items), t.selected, navigationNormalize)
	itemDescriptor := tabsItemActionDescriptor(t.enabled && hasSelection)
	navigationDescriptors := tabsNavigationActionDescriptors(t.enabled && hasSelection)
	onSelect := t.onSelect
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
			children = append(children, tui.StyledText[Message](content, style).
				WithID(item.ID).
				OnActions(item.ID, []tui.Action[Message]{
					tui.NewAction[Message](itemDescriptor, nil),
				}))
			continue
		}
		selection := index
		itemID := item.ID
		children = append(children, tui.StyledText[Message](content, style).
			Focusable(itemID).
			WithFocusedStyle(t.style.Focused).
			OnActions(itemID, []tui.Action[Message]{
				tui.NewAction(itemDescriptor, func(tui.ActionEvent) tui.EventResult[Message] {
					return tabSelectionResult(isSelected, selection, itemID, onSelect)
				}),
			}).
			OnEvent(itemID, func(event vt.Event) tui.EventResult[Message] {
				if !isPointerActivationEvent(event) {
					return tui.IgnoreResult[Message]()
				}
				return tabSelectionResult(isSelected, selection, itemID, onSelect)
			}))
	}

	root := tui.Row(children...).WithID(t.id)
	actions := make([]tui.Action[Message], len(navigationDescriptors))
	if !t.enabled || !hasSelection {
		for index, descriptor := range navigationDescriptors {
			actions[index] = tui.NewAction[Message](descriptor, nil)
		}
		return root.OnActions(t.id, actions)
	}
	for index, descriptor := range navigationDescriptors {
		navigation := tabsNavigationAction(index)
		actions[index] = tui.NewAction(descriptor, func(tui.ActionEvent) tui.EventResult[Message] {
			return tabsNavigationResult(navigation, selected, itemIDs, onSelect)
		})
	}
	return root.OnActions(t.id, actions)
}

type tabsNavigationAction uint8

const (
	tabsPrevious tabsNavigationAction = iota
	tabsNext
	tabsFirst
	tabsLast
)

var defaultTabsNavigationActionDescriptors = [4]tui.ActionDescriptor{
	tui.NewActionDescriptor(
		SelectionPreviousActionID,
		selectionPreviousActionLabel,
		[]tui.KeyBinding{repeatableActionBinding(vt.KeyLeft)},
	),
	tui.NewActionDescriptor(
		SelectionNextActionID,
		selectionNextActionLabel,
		[]tui.KeyBinding{repeatableActionBinding(vt.KeyRight)},
	),
	tui.NewActionDescriptor(
		SelectionFirstActionID,
		selectionFirstActionLabel,
		[]tui.KeyBinding{repeatableActionBinding(vt.KeyHome)},
	),
	tui.NewActionDescriptor(
		SelectionLastActionID,
		selectionLastActionLabel,
		[]tui.KeyBinding{repeatableActionBinding(vt.KeyEnd)},
	),
}

func tabsItemActionDescriptor(enabled bool) tui.ActionDescriptor {
	return ActivateActionDescriptor().WithAvailability(tabsActionAvailability(enabled))
}

func tabsNavigationActionDescriptors(enabled bool) [4]tui.ActionDescriptor {
	availability := tabsActionAvailability(enabled)
	return [4]tui.ActionDescriptor{
		defaultTabsNavigationActionDescriptors[0].WithAvailability(availability),
		defaultTabsNavigationActionDescriptors[1].WithAvailability(availability),
		defaultTabsNavigationActionDescriptors[2].WithAvailability(availability),
		defaultTabsNavigationActionDescriptors[3].WithAvailability(availability),
	}
}

func tabsActionAvailability(enabled bool) tui.ActionAvailability {
	if enabled {
		return tui.ActionEnabled
	}
	return tui.ActionDisabledPassThrough
}

func tabSelectionResult[Message any](
	isSelected bool,
	index int,
	focusID tui.NodeID,
	onSelect func(int) Message,
) tui.EventResult[Message] {
	result := tui.ConsumeResult[Message]().Focus(focusID)
	if !isSelected {
		result = result.Emit(onSelect(index))
	}
	return result
}

func tabsNavigationResult[Message any](
	action tabsNavigationAction,
	selected int,
	itemIDs []tui.NodeID,
	onSelect func(int) Message,
) tui.EventResult[Message] {
	navigation := navigationNormalize
	switch action {
	case tabsPrevious:
		navigation = navigationUp
	case tabsNext:
		navigation = navigationDown
	case tabsFirst:
		navigation = navigationHome
	case tabsLast:
		navigation = navigationEnd
	}
	next, _ := navigateSelection(len(itemIDs), selected, navigation)
	result := tui.ConsumeResult[Message]().Focus(itemIDs[next])
	if next != selected {
		result = result.Emit(onSelect(next))
	}
	return result
}
