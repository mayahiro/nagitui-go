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

// Select is a compact selector that exposes one application-owned option at a time
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

// Node builds the public semantic node for this selector
func (s Select[Message]) Node() tui.Node[Message] {
	selected, hasSelection := navigateSelection(len(s.options), s.selected, navigationNormalize)
	label := s.placeholder
	if hasSelection {
		label = s.options[selected]
	}
	content := "< " + label + " >"
	if !s.enabled || !hasSelection {
		return tui.StyledText[Message](content, s.style.Disabled).WithID(s.id)
	}
	return tui.StyledText[Message](content, s.style.Normal).
		Focusable(s.id).
		WithFocusedStyle(s.style.Focused).
		OnEvent(s.id, func(event vt.Event) tui.EventResult[Message] {
			next, handled := selectEvent(event, len(s.options), selected)
			if !handled {
				return tui.IgnoreResult[Message]()
			}
			result := tui.ConsumeResult[Message]().Focus(s.id)
			if next != selected {
				result = result.Emit(s.onSelect(next))
			}
			return result
		})
}

func selectEvent(event vt.Event, count, selected int) (int, bool) {
	if isActivationEvent(event) {
		return (selected + 1) % count, true
	}
	if event.Kind != vt.EventKey || event.Key.Action == vt.KeyRelease {
		return 0, false
	}
	modifiers := event.Key.Modifiers
	if modifiers.Alt || modifiers.Control || modifiers.Meta {
		return 0, false
	}
	action := navigationNormalize
	switch event.Key.Code {
	case vt.KeyLeft, vt.KeyUp:
		action = navigationUp
	case vt.KeyRight, vt.KeyDown:
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
