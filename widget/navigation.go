package widget

import "github.com/mayahiro/nagi-go/vt"

type navigation uint8

const (
	navigationNormalize navigation = iota
	navigationUp
	navigationDown
	navigationHome
	navigationEnd
)

func normalizeSelection(count, selected int) (int, bool) {
	if count <= 0 {
		return 0, false
	}
	return min(max(selected, 0), count-1), true
}

func navigateSelection(count, selected int, action navigation) (int, bool) {
	selected, ok := normalizeSelection(count, selected)
	if !ok {
		return 0, false
	}
	switch action {
	case navigationNormalize:
	case navigationUp:
		selected = max(selected-1, 0)
	case navigationDown:
		selected = min(selected+1, count-1)
	case navigationHome:
		selected = 0
	case navigationEnd:
		selected = count - 1
	default:
		return 0, false
	}
	return selected, true
}

func navigationEvent(event vt.Event, count, selected int) (int, bool) {
	if event.Kind != vt.EventKey || event.Key.Action == vt.KeyRelease {
		return 0, false
	}
	modifiers := event.Key.Modifiers
	if modifiers.Alt || modifiers.Control || modifiers.Meta {
		return 0, false
	}
	action := navigationNormalize
	switch event.Key.Code {
	case vt.KeyUp:
		action = navigationUp
	case vt.KeyDown:
		action = navigationDown
	case vt.KeyHome:
		action = navigationHome
	case vt.KeyEnd:
		action = navigationEnd
	default:
		return 0, false
	}
	return navigateSelection(count, selected, action)
}
