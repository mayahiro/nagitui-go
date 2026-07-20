package widget

import (
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

// RadioStyle contains the visual styles used by a Radio
type RadioStyle struct {
	// Normal is used while the radio is enabled and unfocused
	Normal vt.Style
	// Focused is merged over the radio while it owns focus
	Focused vt.Style
	// Disabled is used while the radio is disabled
	Disabled vt.Style
}

// DefaultRadioStyle returns the standard radio styles
func DefaultRadioStyle() RadioStyle {
	return RadioStyle{
		Focused:  vt.Style{Reverse: true},
		Disabled: vt.Style{Dim: true},
	}
}

// Radio is one controlled choice in an application-owned radio group
type Radio[Message any] struct {
	id       tui.NodeID
	label    string
	selected bool
	enabled  bool
	style    RadioStyle
	onSelect func() Message
}

// NewRadio returns an enabled radio using application-owned selection state
//
// A nil onSelect function creates a disabled radio
func NewRadio[Message any](id tui.NodeID, label string, selected bool, onSelect func() Message) Radio[Message] {
	return Radio[Message]{
		id: id, label: label, selected: selected, enabled: onSelect != nil,
		style: DefaultRadioStyle(), onSelect: onSelect,
	}
}

// Enabled sets whether the radio can receive focus and select itself
func (r Radio[Message]) Enabled(enabled bool) Radio[Message] {
	r.enabled = enabled && r.onSelect != nil
	return r
}

// Style replaces the radio styles
func (r Radio[Message]) Style(style RadioStyle) Radio[Message] {
	r.style = style
	return r
}

// Node builds the public semantic node for this radio
func (r Radio[Message]) Node() tui.Node[Message] {
	marker := " "
	if r.selected {
		marker = "o"
	}
	content := "(" + marker + ") " + r.label
	if !r.enabled {
		return tui.StyledText[Message](content, r.style.Disabled).WithID(r.id)
	}
	return tui.StyledText[Message](content, r.style.Normal).
		Focusable(r.id).
		WithFocusedStyle(r.style.Focused).
		OnEvent(r.id, func(event vt.Event) tui.EventResult[Message] {
			if !isActivationEvent(event) {
				return tui.IgnoreResult[Message]()
			}
			result := tui.ConsumeResult[Message]().Focus(r.id)
			if !r.selected {
				result = result.Emit(r.onSelect())
			}
			return result
		})
}
