package widget

import "github.com/mayahiro/nagi-go/vt"

func isActivationEvent(event vt.Event) bool {
	switch event.Kind {
	case vt.EventKey:
		if event.Key.Action == vt.KeyRelease {
			return false
		}
		switch event.Key.Code {
		case vt.KeyEnter:
			return true
		case vt.KeyCharacter:
			modifiers := event.Key.Modifiers
			return event.Key.Character == ' ' && !modifiers.Shift && !modifiers.Alt && !modifiers.Control && !modifiers.Meta
		}
	case vt.EventText:
		return event.Text == " "
	case vt.EventMouse:
		return event.Mouse.Kind == vt.MousePress && event.Mouse.Button == vt.MouseLeft
	}
	return false
}
