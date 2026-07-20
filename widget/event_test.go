package widget

import (
	"errors"
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go/internal/conformance"
)

func TestActivationFixtures(t *testing.T) {
	records, err := conformance.Load(
		"widgets/activation.txt",
		"widget-activation",
		"widget", "event", "enabled", "activate",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if kind := record.Field("widget"); kind != "button" && kind != "list" &&
			kind != "checkbox" && kind != "radio" && kind != "tabs" &&
			kind != "select" && kind != "table" && kind != "tree" &&
			kind != "command-palette" {
			t.Fatalf("case %s: invalid widget %q", record.ID, kind)
		}
		actual := fixtureBool(t, record.Field("enabled")) && isActivationEvent(fixtureEvent(t, record.Field("event")))
		expected := fixtureBool(t, record.Field("activate"))
		if actual != expected {
			t.Errorf("case %s: activation = %t, want %t", record.ID, actual, expected)
		}
	}
}

func fixtureEvent(t *testing.T, value string) vt.Event {
	t.Helper()
	switch value {
	case "enter":
		return keyEvent(vt.KeyEnter, 0, vt.Modifiers{}, vt.KeyPress)
	case "space":
		return vt.Event{Kind: vt.EventText, Text: " "}
	case "control-space":
		return keyEvent(vt.KeyCharacter, ' ', vt.Modifiers{Control: true}, vt.KeyPress)
	case "key-release":
		return keyEvent(vt.KeyEnter, 0, vt.Modifiers{}, vt.KeyRelease)
	case "mouse-left-press":
		return mouseEvent(vt.MousePress, vt.MouseLeft)
	case "mouse-left-release":
		return mouseEvent(vt.MouseRelease, vt.MouseLeft)
	case "mouse-right-press":
		return mouseEvent(vt.MousePress, vt.MouseRight)
	default:
		t.Fatalf("unknown fixture event %q", value)
		return vt.Event{}
	}
}

func keyEvent(code vt.KeyCode, character rune, modifiers vt.Modifiers, action vt.KeyAction) vt.Event {
	return vt.Event{Kind: vt.EventKey, Key: vt.KeyEvent{
		Code: code, Character: character, Modifiers: modifiers, Action: action,
		Protocol: vt.KeyProtocolLegacy,
	}}
}

func mouseEvent(kind vt.MouseKind, button vt.MouseButton) vt.Event {
	return vt.Event{Kind: vt.EventMouse, Mouse: vt.MouseEvent{Kind: kind, Button: button}}
}

func fixtureBool(t *testing.T, value string) bool {
	t.Helper()
	switch value {
	case "true":
		return true
	case "false":
		return false
	default:
		t.Fatalf("invalid Boolean %q", value)
		return false
	}
}
