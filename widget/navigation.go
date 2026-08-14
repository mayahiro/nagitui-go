package widget

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
