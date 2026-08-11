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

// SelectionPreviousPageActionID is the stable Action ID for selecting one page toward the beginning
const SelectionPreviousPageActionID tui.ActionID = "nagi.selection.previous-page"

// SelectionNextPageActionID is the stable Action ID for selecting one page toward the end
const SelectionNextPageActionID tui.ActionID = "nagi.selection.next-page"

// SelectionPreviousDayActionID is the stable Action ID for selecting the previous calendar day
const SelectionPreviousDayActionID tui.ActionID = "nagi.selection.previous-day"

// SelectionNextDayActionID is the stable Action ID for selecting the next calendar day
const SelectionNextDayActionID tui.ActionID = "nagi.selection.next-day"

// SelectionPreviousWeekActionID is the stable Action ID for selecting the date one week earlier
const SelectionPreviousWeekActionID tui.ActionID = "nagi.selection.previous-week"

// SelectionNextWeekActionID is the stable Action ID for selecting the date one week later
const SelectionNextWeekActionID tui.ActionID = "nagi.selection.next-week"

// SelectionPreviousMonthActionID is the stable Action ID for selecting the date one month earlier
const SelectionPreviousMonthActionID tui.ActionID = "nagi.selection.previous-month"

// SelectionNextMonthActionID is the stable Action ID for selecting the date one month later
const SelectionNextMonthActionID tui.ActionID = "nagi.selection.next-month"

// SelectionFirstDayOfMonthActionID is the stable Action ID for selecting the first day of the displayed month
const SelectionFirstDayOfMonthActionID tui.ActionID = "nagi.selection.first-day-of-month"

// SelectionLastDayOfMonthActionID is the stable Action ID for selecting the last day of the displayed month
const SelectionLastDayOfMonthActionID tui.ActionID = "nagi.selection.last-day-of-month"

// NavigationBackActionID is the stable Action ID for navigating back from the current location
const NavigationBackActionID tui.ActionID = "nagi.navigation.back"

// CollapseActionID is the stable Action ID for collapsing the current disclosure target
const CollapseActionID tui.ActionID = "nagi.collapse"

// ExpandActionID is the stable Action ID for expanding the current disclosure target
const ExpandActionID tui.ActionID = "nagi.expand"

// DismissActionID is the stable Action ID for dismissing the current transient surface
const DismissActionID tui.ActionID = "nagi.dismiss"

const (
	activateActionLabel                 = "Activate"
	selectionPreviousActionLabel        = "Previous"
	selectionNextActionLabel            = "Next"
	selectionFirstActionLabel           = "First"
	selectionLastActionLabel            = "Last"
	selectionPreviousPageActionLabel    = "Previous page"
	selectionNextPageActionLabel        = "Next page"
	selectionPreviousDayActionLabel     = "Previous day"
	selectionNextDayActionLabel         = "Next day"
	selectionPreviousWeekActionLabel    = "Previous week"
	selectionNextWeekActionLabel        = "Next week"
	selectionPreviousMonthActionLabel   = "Previous month"
	selectionNextMonthActionLabel       = "Next month"
	selectionFirstDayOfMonthActionLabel = "First day of month"
	selectionLastDayOfMonthActionLabel  = "Last day of month"
	navigationBackActionLabel           = "Back"
)

var activateActionDescriptor = tui.NewActionDescriptor(
	ActivateActionID,
	activateActionLabel,
	[]tui.KeyBinding{
		tui.NewKeyBinding(tui.NewKeyStroke(vt.KeyEnter, vt.Modifiers{})).
			WithRepeatPolicy(tui.RepeatAllow),
		tui.NewKeyBinding(tui.NewCharacterKeyStroke(' ', vt.Modifiers{})).
			WithRepeatPolicy(tui.RepeatAllow),
	},
)

var dismissActionDescriptor = tui.NewActionDescriptor(
	DismissActionID,
	"Dismiss",
	[]tui.KeyBinding{repeatableActionBinding(vt.KeyEscape)},
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

// DismissActionDescriptor returns the enabled standard dismissal descriptor
//
// Unmodified Escape is the default binding and explicit repeat events remain
// enabled to preserve standard transient-surface behavior.
func DismissActionDescriptor() tui.ActionDescriptor {
	return dismissActionDescriptor
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
