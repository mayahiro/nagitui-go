package widget

import (
	"fmt"

	"github.com/mayahiro/nagi-go/vt"
	tui "github.com/mayahiro/nagitui-go"
)

// SplitPaneRatioScale is the complete basis-point range used by SplitPaneState
const SplitPaneRatioScale uint16 = 10_000

// SplitPaneState is the controlled primary-pane share for a SplitPane
type SplitPaneState struct {
	ratio uint16
}

// NewSplitPaneState returns state from a primary-pane share in basis points
//
// Values above 10,000 are clamped to 10,000
func NewSplitPaneState(ratio uint16) SplitPaneState {
	return SplitPaneState{ratio: min(ratio, SplitPaneRatioScale)}
}

// DefaultSplitPaneState returns an equal pane share
func DefaultSplitPaneState() SplitPaneState {
	return NewSplitPaneState(5_000)
}

// Ratio returns the controlled primary-pane share in basis points
func (s SplitPaneState) Ratio() uint16 {
	return s.ratio
}

func (s SplitPaneState) moved(towardEnd bool, step uint16) SplitPaneState {
	if towardEnd {
		return NewSplitPaneState(uint16(min(uint32(s.ratio)+uint32(step), uint32(SplitPaneRatioScale))))
	}
	return NewSplitPaneState(s.ratio - min(s.ratio, step))
}

// SplitPaneStyle contains visual styles used by a SplitPane
type SplitPaneStyle struct {
	// Divider is used by the one-Cell divider
	Divider vt.Style
}

// DefaultSplitPaneStyle returns the standard split-pane styles
func DefaultSplitPaneStyle() SplitPaneStyle {
	return SplitPaneStyle{}
}

// SplitPane is a controlled responsive two-pane layout
//
// The Core split-pane node owns cell allocation and automatic collapse. This
// widget adds semantic focus movement, controlled keyboard resizing, and
// pointer dragging without assigning application meaning to either pane
type SplitPane[Message any] struct {
	id                               tui.NodeID
	primary, secondary               tui.Node[Message]
	state                            SplitPaneState
	axis                             tui.SplitPaneAxis
	primaryMinimum, secondaryMinimum uint32
	collapse                         tui.SplitPaneCollapse
	style                            SplitPaneStyle
	resizeStep                       uint16
	primaryFocus, secondaryFocus     tui.NodeID
	hasFocusTargets                  bool
	onResize                         func(SplitPaneState) Message
}

// NewSplitPane returns a horizontal controlled split that collapses its secondary pane
func NewSplitPane[Message any](
	id tui.NodeID,
	primary, secondary tui.Node[Message],
	state SplitPaneState,
) SplitPane[Message] {
	return SplitPane[Message]{
		id: id, primary: primary, secondary: secondary, state: state,
		axis: tui.SplitPaneHorizontal, primaryMinimum: 1, secondaryMinimum: 1,
		collapse: tui.SplitPaneCollapseSecondary, style: DefaultSplitPaneStyle(),
		resizeStep: 500,
	}
}

// Axis sets the main-axis direction
func (s SplitPane[Message]) Axis(axis tui.SplitPaneAxis) SplitPane[Message] {
	s.axis = axis
	return s
}

// Minimums sets expanded minima, each normalized to at least one Cell
func (s SplitPane[Message]) Minimums(primary, secondary uint32) SplitPane[Message] {
	s.primaryMinimum = primary
	s.secondaryMinimum = secondary
	return s
}

// Collapse sets which pane is omitted when both minima and the divider do not fit
func (s SplitPane[Message]) Collapse(collapse tui.SplitPaneCollapse) SplitPane[Message] {
	s.collapse = collapse
	return s
}

// Style replaces the divider style
func (s SplitPane[Message]) Style(style SplitPaneStyle) SplitPane[Message] {
	s.style = style
	return s
}

// ResizeStep sets the keyboard resize increment in basis points
//
// Zero disables state movement at either ratio boundary but the bound actions
// remain consumed while a resize handler exists
func (s SplitPane[Message]) ResizeStep(step uint16) SplitPane[Message] {
	s.resizeStep = step
	return s
}

// FocusTargets sets stable targets used for F6 pane traversal and collapse fallback
func (s SplitPane[Message]) FocusTargets(primary, secondary tui.NodeID) SplitPane[Message] {
	s.primaryFocus = primary
	s.secondaryFocus = secondary
	s.hasFocusTargets = true
	return s
}

// OnResize sets the application message mapper for keyboard and pointer resizing
//
// A nil handler disables resizing
func (s SplitPane[Message]) OnResize(handler func(SplitPaneState) Message) SplitPane[Message] {
	s.onResize = handler
	return s
}

// ActionDescriptors returns focus-previous, focus-next, resize-previous, and resize-next descriptors
func (s SplitPane[Message]) ActionDescriptors() [4]tui.ActionDescriptor {
	return splitPaneActionDescriptors(s.axis, s.hasFocusTargets, s.onResize != nil)
}

// Node builds the public semantic node for this split pane
func (s SplitPane[Message]) Node() tui.Node[Message] {
	descriptors := s.ActionDescriptors()
	primaryOwner := splitPaneOwnerID(s.id, "primary")
	secondaryOwner := splitPaneOwnerID(s.id, "secondary")
	primary := paneFocusNode(
		s.primary, primaryOwner, s.secondaryFocus, s.hasFocusTargets, descriptors,
	)
	secondary := paneFocusNode(
		s.secondary, secondaryOwner, s.primaryFocus, s.hasFocusTargets, descriptors,
	)
	options := tui.SplitPaneOptions{
		Axis: s.axis, Ratio: s.state.Ratio(),
		PrimaryMinimum: s.primaryMinimum, SecondaryMinimum: s.secondaryMinimum,
		Collapse: s.collapse, DividerStyle: s.style.Divider,
	}
	node := tui.SplitPane(primary, secondary, options).WithID(s.id)
	node = node.OnActions(s.id, resizeActions(
		descriptors[2], descriptors[3], s.state, s.resizeStep, s.onResize,
	))
	if s.onResize != nil {
		id := s.id
		state := s.state
		axis := s.axis
		primaryMinimum := s.primaryMinimum
		secondaryMinimum := s.secondaryMinimum
		onResize := s.onResize
		node = node.OnPointerEvent(s.id, func(context tui.PointerEventContext) tui.EventResult[Message] {
			return splitPanePointerEvent(
				id, context, state, axis, primaryMinimum, secondaryMinimum, onResize,
			)
		})
	}
	return node
}

func paneFocusNode[Message any](
	node tui.Node[Message],
	owner, target tui.NodeID,
	hasTarget bool,
	descriptors [4]tui.ActionDescriptor,
) tui.Node[Message] {
	node = tui.Padding(node, tui.UniformInsets(0)).WithID(owner).OnActions(owner, []tui.Action[Message]{
		tui.NewAction(descriptors[0], func(tui.ActionEvent) tui.EventResult[Message] {
			if !hasTarget {
				return tui.IgnoreResult[Message]()
			}
			return tui.ConsumeResult[Message]().Focus(target)
		}),
		tui.NewAction(descriptors[1], func(tui.ActionEvent) tui.EventResult[Message] {
			if !hasTarget {
				return tui.IgnoreResult[Message]()
			}
			return tui.ConsumeResult[Message]().Focus(target)
		}),
	})
	if hasTarget {
		node = node.FocusFallback(target)
	}
	return node
}

func resizeActions[Message any](
	previous, next tui.ActionDescriptor,
	state SplitPaneState,
	step uint16,
	onResize func(SplitPaneState) Message,
) []tui.Action[Message] {
	result := func(towardEnd bool) tui.EventResult[Message] {
		if onResize == nil {
			return tui.IgnoreResult[Message]()
		}
		next := state.moved(towardEnd, step)
		if next == state {
			return tui.ConsumeResult[Message]()
		}
		return tui.MessageResult(onResize(next))
	}
	return []tui.Action[Message]{
		tui.NewAction(previous, func(tui.ActionEvent) tui.EventResult[Message] { return result(false) }),
		tui.NewAction(next, func(tui.ActionEvent) tui.EventResult[Message] { return result(true) }),
	}
}

func splitPanePointerEvent[Message any](
	id tui.NodeID,
	context tui.PointerEventContext,
	state SplitPaneState,
	axis tui.SplitPaneAxis,
	primaryMinimum, secondaryMinimum uint32,
	onResize func(SplitPaneState) Message,
) tui.EventResult[Message] {
	event := context.Event()
	main := context.Bounds().Width
	position := context.LocalPosition().X
	if axis == tui.SplitPaneVertical {
		main = context.Bounds().Height
		position = context.LocalPosition().Y
	}
	switch {
	case event.Kind == vt.MousePress && event.Button == vt.MouseLeft:
		divider, ok := splitPaneDividerPosition(main, state, primaryMinimum, secondaryMinimum)
		if ok && position >= 0 && uint32(position) == divider {
			return tui.ConsumeResult[Message]().CapturePointer(id)
		}
		return tui.IgnoreResult[Message]()
	case event.Kind == vt.MouseMove && context.IsCaptured():
		return splitPanePointerResizeResult(position, main, state, onResize, false)
	case event.Kind == vt.MouseRelease && context.IsCaptured():
		return splitPanePointerResizeResult(position, main, state, onResize, true)
	default:
		return tui.IgnoreResult[Message]()
	}
}

func splitPanePointerResizeResult[Message any](
	position int32,
	main uint32,
	state SplitPaneState,
	onResize func(SplitPaneState) Message,
	release bool,
) tui.EventResult[Message] {
	next, ok := splitPanePointerRatio(position, main)
	result := tui.ConsumeResult[Message]()
	if ok && next != state {
		result = tui.MessageResult(onResize(next))
	}
	if release {
		result = result.ReleasePointer()
	}
	return result
}

func splitPaneDividerPosition(
	main uint32,
	state SplitPaneState,
	primaryMinimum, secondaryMinimum uint32,
) (uint32, bool) {
	primaryMinimum = max(primaryMinimum, 1)
	secondaryMinimum = max(secondaryMinimum, 1)
	required := uint64(primaryMinimum) + 1 + uint64(secondaryMinimum)
	if uint64(main) < required {
		return 0, false
	}
	usable := main - 1
	ideal := uint32(uint64(usable) * uint64(state.Ratio()) / uint64(SplitPaneRatioScale))
	return min(max(ideal, primaryMinimum), usable-secondaryMinimum), true
}

func splitPanePointerRatio(position int32, main uint32) (SplitPaneState, bool) {
	if main <= 1 {
		return SplitPaneState{}, false
	}
	usable := main - 1
	position = max(position, 0)
	cell := min(uint32(position), usable)
	ratio := (uint64(cell)*uint64(SplitPaneRatioScale) + uint64(usable) - 1) / uint64(usable)
	return NewSplitPaneState(uint16(ratio)), true
}

func splitPaneOwnerID(root tui.NodeID, pane string) tui.NodeID {
	return tui.NewNodeID(fmt.Sprintf("%s:split-pane:%s", root, pane))
}

var paneFocusDescriptors = [2]tui.ActionDescriptor{
	tui.NewActionDescriptor(
		PaneFocusPreviousActionID,
		"Focus previous pane",
		[]tui.KeyBinding{tui.NewKeyBinding(tui.NewFunctionKeyStroke(6, vt.Modifiers{Shift: true}))},
	),
	tui.NewActionDescriptor(
		PaneFocusNextActionID,
		"Focus next pane",
		[]tui.KeyBinding{tui.NewKeyBinding(tui.NewFunctionKeyStroke(6, vt.Modifiers{}))},
	),
}

var horizontalPaneResizeDescriptors = paneResizeDescriptors(vt.KeyLeft, vt.KeyRight)
var verticalPaneResizeDescriptors = paneResizeDescriptors(vt.KeyUp, vt.KeyDown)

func paneResizeDescriptors(previous, next vt.KeyCode) [2]tui.ActionDescriptor {
	modifiers := vt.Modifiers{Alt: true}
	return [2]tui.ActionDescriptor{
		tui.NewActionDescriptor(
			PaneResizePreviousActionID,
			"Resize pane toward start",
			[]tui.KeyBinding{tui.NewKeyBinding(tui.NewKeyStroke(previous, modifiers)).WithRepeatPolicy(tui.RepeatAllow)},
		),
		tui.NewActionDescriptor(
			PaneResizeNextActionID,
			"Resize pane toward end",
			[]tui.KeyBinding{tui.NewKeyBinding(tui.NewKeyStroke(next, modifiers)).WithRepeatPolicy(tui.RepeatAllow)},
		),
	}
}

func splitPaneActionDescriptors(axis tui.SplitPaneAxis, focusEnabled, resizeEnabled bool) [4]tui.ActionDescriptor {
	focusAvailability := tui.ActionDisabledPassThrough
	if focusEnabled {
		focusAvailability = tui.ActionEnabled
	}
	resizeAvailability := tui.ActionDisabledPassThrough
	if resizeEnabled {
		resizeAvailability = tui.ActionEnabled
	}
	resize := horizontalPaneResizeDescriptors
	if axis == tui.SplitPaneVertical {
		resize = verticalPaneResizeDescriptors
	}
	return [4]tui.ActionDescriptor{
		paneFocusDescriptors[0].WithAvailability(focusAvailability),
		paneFocusDescriptors[1].WithAvailability(focusAvailability),
		resize[0].WithAvailability(resizeAvailability),
		resize[1].WithAvailability(resizeAvailability),
	}
}
