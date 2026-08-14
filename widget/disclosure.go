package widget

import (
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

// DisclosureStyle contains visual styles used by a Disclosure
type DisclosureStyle struct {
	// Marker is used by the disclosure marker while enabled
	Marker vt.Style
	// Focused is merged over the summary header while it owns focus
	Focused vt.Style
	// Disabled is used by the disclosure marker while disabled
	Disabled vt.Style
}

// DefaultDisclosureStyle returns the standard marker and focus styles
func DefaultDisclosureStyle() DisclosureStyle {
	return DisclosureStyle{
		Focused:  vt.Style{Reverse: true},
		Disabled: vt.Style{Dim: true},
	}
}

// Disclosure is a controlled summary and lazily constructed detail subtree
//
// The summary header owns focus, semantic actions, and pointer toggling. The
// supplied summary is therefore display-only. The body builder is called only
// while Expanded is true, and focused body descendants return to the header
// when a later frame collapses the disclosure.
type Disclosure[Message any] struct {
	id       tui.NodeID
	summary  tui.Node[Message]
	expanded bool
	enabled  bool
	style    DisclosureStyle
	onToggle func(bool) Message
	body     func() tui.Node[Message]
}

// NewDisclosure returns a controlled disclosure without a body builder
//
// A nil onToggle function creates a disabled disclosure.
func NewDisclosure[Message any](
	id tui.NodeID,
	summary tui.Node[Message],
	expanded bool,
	onToggle func(bool) Message,
) Disclosure[Message] {
	return Disclosure[Message]{
		id: id, summary: summary, expanded: expanded, enabled: onToggle != nil,
		style: DefaultDisclosureStyle(), onToggle: onToggle,
	}
}

// Body sets the lazy detail builder
//
// Replacing the builder does not construct either body. The final builder is
// invoked once by Node only while the disclosure is expanded.
func (d Disclosure[Message]) Body(builder func() tui.Node[Message]) Disclosure[Message] {
	d.body = builder
	return d
}

// Enabled sets whether the summary can receive focus and request state changes
func (d Disclosure[Message]) Enabled(enabled bool) Disclosure[Message] {
	d.enabled = enabled && d.onToggle != nil
	return d
}

// Style replaces marker and focus styles
func (d Disclosure[Message]) Style(style DisclosureStyle) Disclosure[Message] {
	d.style = style
	return d
}

// ActionDescriptors returns activate, collapse, and expand descriptors in semantic order
func (d Disclosure[Message]) ActionDescriptors() [3]tui.ActionDescriptor {
	return disclosureActionDescriptors(d.enabled, d.expanded)
}

// Node builds the public semantic node without constructing a collapsed body
func (d Disclosure[Message]) Node() tui.Node[Message] {
	descriptors := d.ActionDescriptors()
	marker := "▶ "
	if d.expanded {
		marker = "▼ "
	}
	markerStyle := d.style.Marker
	if !d.enabled {
		markerStyle = d.style.Disabled
	}
	header := tui.Row(
		tui.StyledText[Message](marker, markerStyle),
		d.summary,
	)
	if d.enabled {
		expanded := d.expanded
		header = header.
			Focusable(d.id).
			WithFocusedStyle(d.style.Focused).
			OnActions(d.id, []tui.Action[Message]{
				tui.NewAction(descriptors[0], func(tui.ActionEvent) tui.EventResult[Message] {
					return tui.MessageResult(d.onToggle(!expanded)).Focus(d.id)
				}),
				tui.NewAction(descriptors[1], func(tui.ActionEvent) tui.EventResult[Message] {
					return tui.MessageResult(d.onToggle(false)).Focus(d.id)
				}),
				tui.NewAction(descriptors[2], func(tui.ActionEvent) tui.EventResult[Message] {
					return tui.MessageResult(d.onToggle(true)).Focus(d.id)
				}),
			}).
			OnEvent(d.id, func(event vt.Event) tui.EventResult[Message] {
				if isPointerActivationEvent(event) {
					return tui.MessageResult(d.onToggle(!expanded)).Focus(d.id)
				}
				return tui.IgnoreResult[Message]()
			})
	} else {
		header = header.WithID(d.id).OnActions(d.id, []tui.Action[Message]{
			tui.NewAction[Message](descriptors[0], nil),
			tui.NewAction[Message](descriptors[1], nil),
			tui.NewAction[Message](descriptors[2], nil),
		})
	}

	children := []tui.Node[Message]{header}
	if d.expanded && d.body != nil {
		children = append(children, d.body().FocusFallback(d.id))
	}
	return tui.Column(children...)
}

var defaultDisclosureDirectionActionDescriptors = [2]tui.ActionDescriptor{
	tui.NewActionDescriptor(
		CollapseActionID,
		"Collapse",
		[]tui.KeyBinding{repeatableActionBinding(vt.KeyLeft)},
	),
	tui.NewActionDescriptor(
		ExpandActionID,
		"Expand",
		[]tui.KeyBinding{repeatableActionBinding(vt.KeyRight)},
	),
}

func disclosureActionDescriptors(enabled, expanded bool) [3]tui.ActionDescriptor {
	toggle := tui.ActionDisabledPassThrough
	collapse := tui.ActionDisabledPassThrough
	expand := tui.ActionDisabledPassThrough
	if enabled {
		toggle = tui.ActionEnabled
		if expanded {
			collapse = tui.ActionEnabled
		} else {
			expand = tui.ActionEnabled
		}
	}
	return [3]tui.ActionDescriptor{
		ActivateActionDescriptor().WithAvailability(toggle),
		defaultDisclosureDirectionActionDescriptors[0].WithAvailability(collapse),
		defaultDisclosureDirectionActionDescriptors[1].WithAvailability(expand),
	}
}
