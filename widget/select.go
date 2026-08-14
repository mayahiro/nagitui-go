package widget

import (
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

// SelectStyle contains the visual styles used by a Select
type SelectStyle struct {
	// Normal is used while the selector is enabled and unfocused
	Normal vt.Style
	// Focused is merged over the selector while it owns focus
	Focused vt.Style
	// Disabled is used when the selector is disabled or empty
	Disabled vt.Style
}

// DefaultSelectStyle returns the standard selector styles
func DefaultSelectStyle() SelectStyle {
	return SelectStyle{Focused: vt.Style{Reverse: true}, Disabled: vt.Style{Dim: true}}
}

// Select is a compact selector with semantic keyboard and raw pointer selection
//
// Keyboard handling declares standard activation and selection actions.
// Left-button press remains raw so keyboard rebinding does not remove it.
type Select[Message any] struct {
	id          tui.NodeID
	options     []string
	selected    int
	enabled     bool
	placeholder string
	style       SelectStyle
	onSelect    func(int) Message
}

// NewSelect returns an enabled selector using application-owned selection state
//
// A nil onSelect function creates a disabled selector
func NewSelect[Message any](id tui.NodeID, options []string, selected int, onSelect func(int) Message) Select[Message] {
	return Select[Message]{
		id: id, options: append([]string(nil), options...), selected: selected,
		enabled: onSelect != nil, placeholder: "No options", style: DefaultSelectStyle(), onSelect: onSelect,
	}
}

// Enabled sets whether the selector can receive focus and change selection
func (s Select[Message]) Enabled(enabled bool) Select[Message] {
	s.enabled = enabled && s.onSelect != nil
	return s
}

// Placeholder sets the text displayed when there are no options
func (s Select[Message]) Placeholder(placeholder string) Select[Message] {
	s.placeholder = placeholder
	return s
}

// Style replaces the selector styles
func (s Select[Message]) Style(style SelectStyle) Select[Message] {
	s.style = style
	return s
}

// ActionDescriptors returns the ordered semantic actions declared by this selector
//
// The order is activate, previous, next, first, and last. Every descriptor is
// disabled-pass-through when the selector is disabled or empty.
func (s Select[Message]) ActionDescriptors() []tui.ActionDescriptor {
	descriptors := selectActionDescriptors(s.enabled && len(s.options) > 0)
	return append([]tui.ActionDescriptor(nil), descriptors[:]...)
}

// Node builds the public semantic node for this selector
func (s Select[Message]) Node() tui.Node[Message] {
	selected, hasSelection := navigateSelection(len(s.options), s.selected, navigationNormalize)
	label := s.placeholder
	if hasSelection {
		label = s.options[selected]
	}
	content := "< " + label + " >"
	descriptors := selectActionDescriptors(s.enabled && hasSelection)
	if !s.enabled || !hasSelection {
		actions := make([]tui.Action[Message], len(descriptors))
		for index, descriptor := range descriptors {
			actions[index] = tui.NewAction[Message](descriptor, nil)
		}
		return tui.StyledText[Message](content, s.style.Disabled).
			WithID(s.id).
			OnActions(s.id, actions)
	}
	actions := make([]tui.Action[Message], len(descriptors))
	for index, descriptor := range descriptors {
		actions[index] = newSelectAction(
			descriptor, selectAction(index), len(s.options), selected, s.id, s.onSelect,
		)
	}
	return tui.StyledText[Message](content, s.style.Normal).
		Focusable(s.id).
		WithFocusedStyle(s.style.Focused).
		OnActions(s.id, actions).
		OnEvent(s.id, func(event vt.Event) tui.EventResult[Message] {
			if !isPointerActivationEvent(event) {
				return tui.IgnoreResult[Message]()
			}
			return selectActionResult(selectActivate, len(s.options), selected, s.id, s.onSelect)
		})
}

type selectAction uint8

const (
	selectActivate selectAction = iota
	selectPrevious
	selectNext
	selectFirst
	selectLast
)

var selectNavigationActionDescriptors = [4]tui.ActionDescriptor{
	tui.NewActionDescriptor(
		SelectionPreviousActionID,
		selectionPreviousActionLabel,
		[]tui.KeyBinding{
			repeatableActionBinding(vt.KeyLeft),
			repeatableActionBinding(vt.KeyUp),
		},
	),
	tui.NewActionDescriptor(
		SelectionNextActionID,
		selectionNextActionLabel,
		[]tui.KeyBinding{
			repeatableActionBinding(vt.KeyRight),
			repeatableActionBinding(vt.KeyDown),
		},
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

func selectActionDescriptors(enabled bool) [5]tui.ActionDescriptor {
	availability := tui.ActionEnabled
	if !enabled {
		availability = tui.ActionDisabledPassThrough
	}
	return [5]tui.ActionDescriptor{
		ActivateActionDescriptor().WithAvailability(availability),
		selectNavigationActionDescriptors[0].WithAvailability(availability),
		selectNavigationActionDescriptors[1].WithAvailability(availability),
		selectNavigationActionDescriptors[2].WithAvailability(availability),
		selectNavigationActionDescriptors[3].WithAvailability(availability),
	}
}

func newSelectAction[Message any](
	descriptor tui.ActionDescriptor,
	action selectAction,
	count int,
	selected int,
	id tui.NodeID,
	onSelect func(int) Message,
) tui.Action[Message] {
	return tui.NewAction(descriptor, func(tui.ActionEvent) tui.EventResult[Message] {
		return selectActionResult(action, count, selected, id, onSelect)
	})
}

func selectActionResult[Message any](
	action selectAction,
	count int,
	selected int,
	id tui.NodeID,
	onSelect func(int) Message,
) tui.EventResult[Message] {
	next := selected
	switch action {
	case selectActivate:
		next = (selected + 1) % count
	case selectPrevious:
		next, _ = navigateSelection(count, selected, navigationUp)
	case selectNext:
		next, _ = navigateSelection(count, selected, navigationDown)
	case selectFirst:
		next, _ = navigateSelection(count, selected, navigationHome)
	case selectLast:
		next, _ = navigateSelection(count, selected, navigationEnd)
	}
	result := tui.ConsumeResult[Message]().Focus(id)
	if next != selected {
		result = result.Emit(onSelect(next))
	}
	return result
}
