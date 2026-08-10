package widget

import (
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

// ActivateActionID is the stable Action ID shared by standard activation widgets
const ActivateActionID tui.ActionID = "nagi.activate"

// SelectionPreviousActionID is the stable Action ID for selecting the previous item
const SelectionPreviousActionID tui.ActionID = "nagi.selection.previous"

// SelectionNextActionID is the stable Action ID for selecting the next item
const SelectionNextActionID tui.ActionID = "nagi.selection.next"

// SelectionFirstActionID is the stable Action ID for selecting the first item
const SelectionFirstActionID tui.ActionID = "nagi.selection.first"

// SelectionLastActionID is the stable Action ID for selecting the last item
const SelectionLastActionID tui.ActionID = "nagi.selection.last"

// CollapseActionID is the stable Action ID for collapsing the current disclosure target
const CollapseActionID tui.ActionID = "nagi.collapse"

// ExpandActionID is the stable Action ID for expanding the current disclosure target
const ExpandActionID tui.ActionID = "nagi.expand"

const (
	selectionPreviousActionLabel = "Previous"
	selectionNextActionLabel     = "Next"
	selectionFirstActionLabel    = "First"
	selectionLastActionLabel     = "Last"
)

var activateActionDescriptor = tui.NewActionDescriptor(
	ActivateActionID,
	"Activate",
	[]tui.KeyBinding{
		tui.NewKeyBinding(tui.NewKeyStroke(vt.KeyEnter, vt.Modifiers{})).
			WithRepeatPolicy(tui.RepeatAllow),
		tui.NewKeyBinding(tui.NewCharacterKeyStroke(' ', vt.Modifiers{})).
			WithRepeatPolicy(tui.RepeatAllow),
	},
)

var defaultVerticalCollectionActionDescriptors = [4]tui.ActionDescriptor{
	tui.NewActionDescriptor(
		SelectionPreviousActionID,
		selectionPreviousActionLabel,
		[]tui.KeyBinding{repeatableActionBinding(vt.KeyUp)},
	),
	tui.NewActionDescriptor(
		SelectionNextActionID,
		selectionNextActionLabel,
		[]tui.KeyBinding{repeatableActionBinding(vt.KeyDown)},
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

type collectionAction uint8

const (
	collectionActivate collectionAction = iota
	collectionPrevious
	collectionNext
	collectionFirst
	collectionLast
)

// ActivateActionDescriptor returns the enabled standard activation descriptor
//
// Enter and unmodified Space are ordered fallback bindings. Explicit repeat
// events remain enabled to preserve standard activation-widget behavior.
func ActivateActionDescriptor() tui.ActionDescriptor {
	return activateActionDescriptor
}

func repeatableActionBinding(code vt.KeyCode) tui.KeyBinding {
	return tui.NewKeyBinding(tui.NewKeyStroke(code, vt.Modifiers{})).WithRepeatPolicy(tui.RepeatAllow)
}

func verticalCollectionActionDescriptors(enabled bool) [5]tui.ActionDescriptor {
	availability := tui.ActionEnabled
	if !enabled {
		availability = tui.ActionDisabledPassThrough
	}
	return [5]tui.ActionDescriptor{
		ActivateActionDescriptor().WithAvailability(availability),
		defaultVerticalCollectionActionDescriptors[0].WithAvailability(availability),
		defaultVerticalCollectionActionDescriptors[1].WithAvailability(availability),
		defaultVerticalCollectionActionDescriptors[2].WithAvailability(availability),
		defaultVerticalCollectionActionDescriptors[3].WithAvailability(availability),
	}
}

func collectionNavigation(action collectionAction) (navigation, bool) {
	switch action {
	case collectionPrevious:
		return navigationUp, true
	case collectionNext:
		return navigationDown, true
	case collectionFirst:
		return navigationHome, true
	case collectionLast:
		return navigationEnd, true
	default:
		return navigationNormalize, false
	}
}

func disabledCollectionActions[Message any](descriptors [5]tui.ActionDescriptor) []tui.Action[Message] {
	actions := make([]tui.Action[Message], len(descriptors))
	for index, descriptor := range descriptors {
		actions[index] = tui.NewAction[Message](descriptor, nil)
	}
	return actions
}
