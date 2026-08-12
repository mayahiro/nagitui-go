package widget

import (
	"github.com/mayahiro/nagi-go/vt"
	tui "github.com/mayahiro/nagitui-go"
)

// DrawerSide identifies the viewport edge from which a Drawer is presented
type DrawerSide uint8

const (
	// DrawerLeft presents the drawer against the left edge
	DrawerLeft DrawerSide = iota
	// DrawerRight presents the drawer against the right edge
	DrawerRight
	// DrawerTop presents the drawer against the top edge
	DrawerTop
	// DrawerBottom presents the drawer against the bottom edge
	DrawerBottom
)

// DrawerStyle contains visual styles used by a Drawer
type DrawerStyle struct {
	// Border is used by the drawer border
	Border vt.Style
}

// DefaultDrawerStyle returns the standard drawer styles
func DefaultDrawerStyle() DrawerStyle {
	return DrawerStyle{}
}

// Drawer is a controlled edge overlay with a lazily constructed body
//
// The base remains present while the drawer is open. Modal drawers restrict
// routing and focus to the body through the public Core modal node. Closed
// drawers do not invoke their body builder or add a semantic subtree
type Drawer[Message any] struct {
	id        tui.NodeID
	base      tui.Node[Message]
	open      bool
	side      DrawerSide
	size      tui.Length
	style     DrawerStyle
	modal     bool
	focus     tui.ModalFocusOptions
	onDismiss func() Message
	body      func() tui.Node[Message]
}

// NewDrawer returns a left-side modal drawer whose body occupies 40 percent
func NewDrawer[Message any](id tui.NodeID, base tui.Node[Message], open bool) Drawer[Message] {
	return Drawer[Message]{
		id: id, base: base, open: open, side: DrawerLeft, size: tui.Percent(40),
		style: DefaultDrawerStyle(), modal: true, focus: tui.DefaultModalFocusOptions(),
	}
}

// Body sets the lazy drawer body builder
//
// The final builder is invoked once by Node only while the controlled drawer is open
func (d Drawer[Message]) Body(builder func() tui.Node[Message]) Drawer[Message] {
	d.body = builder
	return d
}

// Side sets the viewport edge used by the drawer. Unknown values use the left edge.
func (d Drawer[Message]) Side(side DrawerSide) Drawer[Message] {
	d.side = side
	return d
}

// Size sets the drawer main-axis size
func (d Drawer[Message]) Size(size tui.Length) Drawer[Message] {
	d.size = size
	return d
}

// Style replaces the drawer border style
func (d Drawer[Message]) Style(style DrawerStyle) Drawer[Message] {
	d.style = style
	return d
}

// Modal sets whether an open drawer restricts routing and focus to its subtree
func (d Drawer[Message]) Modal(modal bool) Drawer[Message] {
	d.modal = modal
	return d
}

// InitialFocus sets the focus policy used when a modal drawer opens
func (d Drawer[Message]) InitialFocus(focus tui.ModalInitialFocus) Drawer[Message] {
	d.focus.Initial = focus
	return d
}

// ReturnFocus sets the focus policy used when a modal drawer closes
func (d Drawer[Message]) ReturnFocus(focus tui.ModalReturnFocus) Drawer[Message] {
	d.focus.ReturnFocus = focus
	return d
}

// OnDismiss sets the message handler used by the semantic dismissal action
//
// A nil handler disables dismissal
func (d Drawer[Message]) OnDismiss(handler func() Message) Drawer[Message] {
	d.onDismiss = handler
	return d
}

// ActionDescriptor returns the semantic dismissal descriptor declared by an open drawer
func (d Drawer[Message]) ActionDescriptor() tui.ActionDescriptor {
	availability := tui.ActionDisabledPassThrough
	if d.open && d.onDismiss != nil {
		availability = tui.ActionEnabled
	}
	return DismissActionDescriptor().WithAvailability(availability)
}

// Node builds the public semantic node without constructing a closed body
func (d Drawer[Message]) Node() tui.Node[Message] {
	if !d.open {
		return d.base
	}

	descriptor := d.ActionDescriptor()
	content := tui.Column[Message]()
	if d.body != nil {
		content = d.body()
	}
	bordered := tui.Border(content, d.style.Border)
	drawer := bordered.WithID(d.id)
	if d.modal {
		drawer = tui.ModalWithFocus(d.id, bordered, d.focus)
	}
	drawer = drawer.OnActions(d.id, []tui.Action[Message]{
		tui.NewAction(descriptor, func(tui.ActionEvent) tui.EventResult[Message] {
			if d.onDismiss == nil {
				return tui.IgnoreResult[Message]()
			}
			return tui.MessageResult(d.onDismiss())
		}),
	}).WithLength(d.size)

	filler := tui.Spacer[Message](0, 0).WithLength(tui.Flex(1))
	var layer tui.Node[Message]
	switch d.side {
	case DrawerRight:
		layer = tui.Row(filler, drawer)
	case DrawerTop:
		layer = tui.Column(drawer, filler)
	case DrawerBottom:
		layer = tui.Column(filler, drawer)
	default:
		layer = tui.Row(drawer, filler)
	}
	return tui.Overlay(d.base, layer)
}
