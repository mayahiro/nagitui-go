package widget

import (
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

// ModalStyle contains the visual styles used by a Modal
type ModalStyle struct {
	// Border is used by the panel border
	Border vt.Style
	// Title is used by a non-empty title
	Title vt.Style
}

// DefaultModalStyle returns the standard modal styles
func DefaultModalStyle() ModalStyle {
	return ModalStyle{}
}

// Modal is a centered panel that restricts routing and focus to its subtree
type Modal[Message any] struct {
	id       tui.NodeID
	child    tui.Node[Message]
	title    string
	style    ModalStyle
	onEscape func() Message
}

// NewModal returns an untitled modal panel
func NewModal[Message any](id tui.NodeID, child tui.Node[Message]) Modal[Message] {
	return Modal[Message]{id: id, child: child, style: DefaultModalStyle()}
}

// Title sets the text rendered above modal content
func (m Modal[Message]) Title(title string) Modal[Message] {
	m.title = title
	return m
}

// Style replaces the modal styles
func (m Modal[Message]) Style(style ModalStyle) Modal[Message] {
	m.style = style
	return m
}

// OnEscape sets the message handler used by the semantic dismissal action
//
// A nil function removes the dismissal handler
func (m Modal[Message]) OnEscape(handler func() Message) Modal[Message] {
	m.onEscape = handler
	return m
}

// ActionDescriptor returns the semantic dismissal descriptor declared by the modal root
func (m Modal[Message]) ActionDescriptor() tui.ActionDescriptor {
	availability := tui.ActionEnabled
	if m.onEscape == nil {
		availability = tui.ActionDisabledPassThrough
	}
	return DismissActionDescriptor().WithAvailability(availability)
}

// Node builds the public semantic node for this modal
func (m Modal[Message]) Node() tui.Node[Message] {
	descriptor := m.ActionDescriptor()
	content := m.child
	if m.title != "" {
		content = tui.Column(
			tui.StyledText[Message](m.title, m.style.Title),
			m.child,
		)
	}
	panel := tui.Border(content, m.style.Border)
	centered := tui.Align(panel, tui.AlignCenter, tui.AlignMiddle)
	modal := tui.Modal(m.id, centered)
	var handler func(tui.ActionEvent) tui.EventResult[Message]
	if m.onEscape != nil {
		handler = func(tui.ActionEvent) tui.EventResult[Message] {
			return tui.MessageResult(m.onEscape())
		}
	}
	return modal.OnActions(m.id, []tui.Action[Message]{tui.NewAction(descriptor, handler)})
}
