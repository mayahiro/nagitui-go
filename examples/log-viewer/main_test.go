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
			msg, ok := action.Message()
			if !ok || msg.kind != togglePauseMessage {
				t.Fatalf("message = %+v, %t, want pause toggle", msg, ok)
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
	msg, ok := action.Message()
	if action.Kind() != tui.EventMessage || !ok || msg.kind != quitMessage {
		t.Fatalf("action = %+v, message = %+v, %t", action, msg, ok)
	}
}

func TestLogBufferRetainsNewestValuesInOrder(t *testing.T) {
	buffer := newLogBuffer(3)
	for _, value := range []string{"a", "b", "c", "d", "e"} {
		buffer.append(value)
	}
	if buffer.len() != 3 {
		t.Fatalf("buffer length = %d, want 3", buffer.len())
	}
	for index, want := range []string{"c", "d", "e"} {
		if got := buffer.at(index); got != want {
			t.Fatalf("buffer[%d] = %q, want %q", index, got, want)
		}
	}
}

func TestSubscriptionTopologyFollowsApplicationState(t *testing.T) {
	app := newLogViewer()
	app.Init()
	if got := app.Subscriptions().String(); got != "Subscription::Batch(2)" {
		t.Fatalf("live subscriptions = %s", got)
	}
	app.paused = true
	if got := app.Subscriptions().String(); got != "Subscription::Batch(1)" {
		t.Fatalf("paused subscriptions = %s", got)
	}
	app.exiting = true
	if subscription := app.Subscriptions(); !subscription.IsNone() {
		t.Fatalf("exiting subscriptions = %s", subscription.String())
	}
}
