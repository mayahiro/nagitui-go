package widget

import (
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

// ButtonStyle contains the visual styles used by a Button
type ButtonStyle struct {
	// Normal is used while the button is enabled and unfocused
	Normal vt.Style
	// Focused is merged over the button while it owns focus
	Focused vt.Style
	// Disabled is used while the button is disabled
	Disabled vt.Style
}

// DefaultButtonStyle returns the standard button styles
func DefaultButtonStyle() ButtonStyle {
	return ButtonStyle{
		Normal:   vt.Style{Bold: true},
		Focused:  vt.Style{Reverse: true},
		Disabled: vt.Style{Dim: true},
	}
}

// Button is a focusable command that emits one message when activated
type Button[Message any] struct {
	id         tui.NodeID
	label      string
	enabled    bool
	style      ButtonStyle
	onActivate func() Message
}

// NewButton returns an enabled button with default styles
//
// A nil onActivate function creates a disabled button
func NewButton[Message any](id tui.NodeID, label string, onActivate func() Message) Button[Message] {
	return Button[Message]{
		id:         id,
		label:      label,
		enabled:    onActivate != nil,
		style:      DefaultButtonStyle(),
		onActivate: onActivate,
	}
}

// Enabled sets whether the button can receive focus and activate
func (b Button[Message]) Enabled(enabled bool) Button[Message] {
	b.enabled = enabled && b.onActivate != nil
	return b
}

// Style replaces the button styles
func (b Button[Message]) Style(style ButtonStyle) Button[Message] {
	b.style = style
	return b
}

// Node builds the public semantic node for this button
func (b Button[Message]) Node() tui.Node[Message] {
	content := "[ " + b.label + " ]"
	if !b.enabled {
		return tui.StyledText[Message](content, b.style.Disabled).WithID(b.id)
	}
	return tui.StyledText[Message](content, b.style.Normal).
		Focusable(b.id).
		WithFocusedStyle(b.style.Focused).
		OnEvent(b.id, func(event vt.Event) tui.EventResult[Message] {
			if isActivationEvent(event) {
				return tui.MessageResult(b.onActivate()).Focus(b.id)
			}
			return tui.IgnoreResult[Message]()
		})
}
