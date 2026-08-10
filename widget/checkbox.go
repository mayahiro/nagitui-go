package widget

import (
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

// CheckboxStyle contains the visual styles used by a Checkbox
type CheckboxStyle struct {
	// Normal is used while the checkbox is enabled and unfocused
	Normal vt.Style
	// Focused is merged over the checkbox while it owns focus
	Focused vt.Style
	// Disabled is used while the checkbox is disabled
	Disabled vt.Style
}

// DefaultCheckboxStyle returns the standard checkbox styles
func DefaultCheckboxStyle() CheckboxStyle {
	return CheckboxStyle{
		Focused:  vt.Style{Reverse: true},
		Disabled: vt.Style{Dim: true},
	}
}

// Checkbox is a controlled Boolean input that emits its requested opposite value
//
// Keyboard activation declares ActivateActionID. Left-button press remains a
// raw pointer event so keyboard rebinding does not remove it.
type Checkbox[Message any] struct {
	id       tui.NodeID
	label    string
	checked  bool
	enabled  bool
	style    CheckboxStyle
	onChange func(bool) Message
}

// NewCheckbox returns an enabled checkbox using application-owned checked state
//
// A nil onChange function creates a disabled checkbox
func NewCheckbox[Message any](id tui.NodeID, label string, checked bool, onChange func(bool) Message) Checkbox[Message] {
	return Checkbox[Message]{
		id: id, label: label, checked: checked, enabled: onChange != nil,
		style: DefaultCheckboxStyle(), onChange: onChange,
	}
}

// Enabled sets whether the checkbox can receive focus and change value
func (c Checkbox[Message]) Enabled(enabled bool) Checkbox[Message] {
	c.enabled = enabled && c.onChange != nil
	return c
}

// Style replaces the checkbox styles
func (c Checkbox[Message]) Style(style CheckboxStyle) Checkbox[Message] {
	c.style = style
	return c
}

// ActionDescriptor returns the semantic activation descriptor declared by this checkbox
func (c Checkbox[Message]) ActionDescriptor() tui.ActionDescriptor {
	availability := tui.ActionEnabled
	if !c.enabled {
		availability = tui.ActionDisabledPassThrough
	}
	return ActivateActionDescriptor().WithAvailability(availability)
}

// Node builds the public semantic node for this checkbox
func (c Checkbox[Message]) Node() tui.Node[Message] {
	marker := " "
	if c.checked {
		marker = "x"
	}
	content := "[" + marker + "] " + c.label
	descriptor := c.ActionDescriptor()
	if !c.enabled {
		return tui.StyledText[Message](content, c.style.Disabled).
			WithID(c.id).
			OnActions(c.id, []tui.Action[Message]{tui.NewAction[Message](descriptor, nil)})
	}
	return tui.StyledText[Message](content, c.style.Normal).
		Focusable(c.id).
		WithFocusedStyle(c.style.Focused).
		OnActions(c.id, []tui.Action[Message]{
			tui.NewAction(descriptor, func(tui.ActionEvent) tui.EventResult[Message] {
				return tui.MessageResult(c.onChange(!c.checked)).Focus(c.id)
			}),
		}).
		OnEvent(c.id, func(event vt.Event) tui.EventResult[Message] {
			if isPointerActivationEvent(event) {
				return tui.MessageResult(c.onChange(!c.checked)).Focus(c.id)
			}
			return tui.IgnoreResult[Message]()
		})
}
