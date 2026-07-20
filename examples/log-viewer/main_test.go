package main

import (
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

func TestDecodedPrintableShortcutsTogglePause(t *testing.T) {
	for _, input := range []string{"p", "P", " "} {
		t.Run(input, func(t *testing.T) {
			events := vt.NewDecoder().Feed([]byte(input))
			if len(events) != 1 {
				t.Fatalf("decoded event count = %d, want 1", len(events))
			}
			action := mapEvent(events[0])
			if action.Kind() != tui.EventMessage {
				t.Fatalf("action kind = %d, want EventMessage", action.Kind())
			}
			message, ok := action.Message()
			if !ok || !message.toggle {
				t.Fatalf("message = %+v, %t, want pause toggle", message, ok)
			}
		})
	}
}

func TestModifiedOrReleasedCharacterDoesNotTogglePause(t *testing.T) {
	for _, event := range []vt.Event{
		{Kind: vt.EventKey, Key: vt.KeyEvent{Code: vt.KeyCharacter, Character: 'p', Modifiers: vt.Modifiers{Control: true}}},
		{Kind: vt.EventKey, Key: vt.KeyEvent{Code: vt.KeyCharacter, Character: 'p', Modifiers: vt.Modifiers{Alt: true}}},
		{Kind: vt.EventKey, Key: vt.KeyEvent{Code: vt.KeyCharacter, Character: 'p', Action: vt.KeyRelease}},
	} {
		if action := mapEvent(event); action.Kind() != tui.EventIgnore {
			t.Fatalf("mapEvent(%+v) kind = %d, want EventIgnore", event, action.Kind())
		}
	}
}

func TestQuitShortcutProducesApplicationMessage(t *testing.T) {
	action := mapEvent(vt.Event{Kind: vt.EventText, Text: "q"})
	message, ok := action.Message()
	if action.Kind() != tui.EventMessage || !ok || !message.quit {
		t.Fatalf("action = %+v, message = %+v, %t", action, message, ok)
	}
}
