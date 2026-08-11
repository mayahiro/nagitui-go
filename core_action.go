package tui

import "github.com/mayahiro/nagi-go/vt"

type coreAction uint8

const (
	coreFocusNext coreAction = iota
	coreFocusPrevious
	coreScrollPageUp
	coreScrollPageDown
	coreScrollStart
	coreScrollEnd
)

var (
	coreFocusActions       = []coreAction{coreFocusNext, coreFocusPrevious}
	coreScrollActions      = []coreAction{coreScrollPageUp, coreScrollPageDown, coreScrollStart, coreScrollEnd}
	coreFocusScrollActions = []coreAction{
		coreFocusNext,
		coreFocusPrevious,
		coreScrollPageUp,
		coreScrollPageDown,
		coreScrollStart,
		coreScrollEnd,
	}
	coreFocusDescriptors                    = makeCoreActionDescriptors(true, ScrollAxisBoth, false)
	coreScrollVerticalDescriptors           = makeCoreActionDescriptors(false, ScrollAxisVertical, true)
	coreScrollHorizontalDescriptors         = makeCoreActionDescriptors(false, ScrollAxisHorizontal, true)
	coreFocusScrollVerticalDescriptors      = makeCoreActionDescriptors(true, ScrollAxisVertical, true)
	coreFocusScrollHorizontalDescriptors    = makeCoreActionDescriptors(true, ScrollAxisHorizontal, true)
	coreFocusDefaultActions                 = makeCoreResolvedActions(coreFocusDescriptors)
	coreScrollVerticalDefaultActions        = makeCoreResolvedActions(coreScrollVerticalDescriptors)
	coreScrollHorizontalDefaultActions      = makeCoreResolvedActions(coreScrollHorizontalDescriptors)
	coreFocusScrollVerticalDefaultActions   = makeCoreResolvedActions(coreFocusScrollVerticalDescriptors)
	coreFocusScrollHorizontalDefaultActions = makeCoreResolvedActions(coreFocusScrollHorizontalDescriptors)
)

type coreActionGroup struct {
	actions        []coreAction
	descriptors    []ActionDescriptor
	defaultActions []ResolvedAction
}

func coreActionGroupFor(includesFocus bool, scrollAxis ScrollAxis, hasScroll bool) (coreActionGroup, bool) {
	switch {
	case !includesFocus && !hasScroll:
		return coreActionGroup{}, false
	case includesFocus && !hasScroll:
		return coreActionGroup{coreFocusActions, coreFocusDescriptors, coreFocusDefaultActions}, true
	case !includesFocus && scrollAxis == ScrollAxisHorizontal:
		return coreActionGroup{coreScrollActions, coreScrollHorizontalDescriptors, coreScrollHorizontalDefaultActions}, true
	case !includesFocus:
		return coreActionGroup{coreScrollActions, coreScrollVerticalDescriptors, coreScrollVerticalDefaultActions}, true
	case scrollAxis == ScrollAxisHorizontal:
		return coreActionGroup{coreFocusScrollActions, coreFocusScrollHorizontalDescriptors, coreFocusScrollHorizontalDefaultActions}, true
	default:
		return coreActionGroup{coreFocusScrollActions, coreFocusScrollVerticalDescriptors, coreFocusScrollVerticalDefaultActions}, true
	}
}

func resolveCoreActionGroup(owner NodeID, group coreActionGroup, scopes []KeyScope) (ResolvedActions, error) {
	for _, scope := range scopes {
		for _, descriptor := range group.descriptors {
			if _, ok := scope.keyMap.bindingsView(descriptor.id); ok {
				return ResolveActions(owner, group.descriptors, scopes)
			}
		}
	}
	var scopePath []NodeID
	if len(scopes) > 0 {
		scopePath = make([]NodeID, len(scopes))
		for index, scope := range scopes {
			scopePath[index] = scope.id
		}
	}
	return ResolvedActions{owner: owner, scopePath: scopePath, actions: group.defaultActions}, nil
}

func defaultFocusAction(event vt.Event) (coreAction, bool) {
	for index, descriptor := range coreFocusDescriptors {
		for _, binding := range descriptor.defaultBindings {
			if binding.Matches(event) {
				return coreFocusActions[index], true
			}
		}
	}
	return 0, false
}

func makeCoreActionDescriptors(includesFocus bool, scrollAxis ScrollAxis, hasScroll bool) []ActionDescriptor {
	capacity := 4
	if includesFocus {
		capacity += 2
	}
	descriptors := make([]ActionDescriptor, 0, capacity)
	if includesFocus {
		descriptors = append(descriptors,
			coreActionDescriptor(FocusNextActionID, "Focus next", vt.KeyTab, vt.Modifiers{}),
			coreActionDescriptor(FocusPreviousActionID, "Focus previous", vt.KeyTab, vt.Modifiers{Shift: true}),
		)
	}
	if hasScroll {
		pageAvailability := ActionDisabledPassThrough
		if scrollAxis.allowsVertical() {
			pageAvailability = ActionEnabled
		}
		descriptors = append(descriptors,
			coreActionDescriptor(ScrollPageUpActionID, "Scroll page up", vt.KeyPageUp, vt.Modifiers{}).
				WithAvailability(pageAvailability),
			coreActionDescriptor(ScrollPageDownActionID, "Scroll page down", vt.KeyPageDown, vt.Modifiers{}).
				WithAvailability(pageAvailability),
			coreActionDescriptor(ScrollStartActionID, "Scroll to start", vt.KeyHome, vt.Modifiers{}),
			coreActionDescriptor(ScrollEndActionID, "Scroll to end", vt.KeyEnd, vt.Modifiers{}),
		)
	}
	return descriptors
}

func makeCoreResolvedActions(descriptors []ActionDescriptor) []ResolvedAction {
	actions := make([]ResolvedAction, len(descriptors))
	for index, descriptor := range descriptors {
		actions[index] = ResolvedAction{
			id:           descriptor.id,
			label:        descriptor.label,
			bindings:     descriptor.defaultBindings,
			availability: descriptor.availability,
			helpVisible:  descriptor.helpVisible,
		}
	}
	return actions
}

func coreActionDescriptor(id ActionID, label string, code vt.KeyCode, modifiers vt.Modifiers) ActionDescriptor {
	return NewActionDescriptor(id, label, []KeyBinding{
		NewKeyBinding(NewKeyStroke(code, modifiers)).WithRepeatPolicy(RepeatAllow),
	})
}
