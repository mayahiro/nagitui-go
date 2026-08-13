package tui

import (
	"testing"
	"time"

	"github.com/mayahiro/nagi-go/vt"
)

func TestTimedInputDecoderUsesVirtualEscapeDeadline(t *testing.T) {
	clock := NewVirtualClock()
	decoder := NewTimedInputDecoder(clock, 25*time.Millisecond)

	if events := decoder.Feed([]byte{0x1B}); len(events) != 0 {
		t.Fatalf("initial events = %v, want none", events)
	}
	clock.Advance(24 * time.Millisecond)
	if events := decoder.Poll(); len(events) != 0 {
		t.Fatalf("early events = %v, want none", events)
	}
	clock.Advance(time.Millisecond)

	events := decoder.Poll()
	if len(events) != 1 || events[0].Kind != vt.EventKey || events[0].Key.Code != vt.KeyEscape {
		t.Fatalf("deadline events = %v, want Escape", events)
	}
}

func TestTimedInputDecoderTreatsNegativeTimeoutAsZero(t *testing.T) {
	decoder := NewTimedInputDecoder(NewVirtualClock(), -time.Second)
	if events := decoder.Feed([]byte{0x1B}); len(events) != 0 {
		t.Fatalf("initial events = %v, want none", events)
	}
	events := decoder.Poll()
	if len(events) != 1 || events[0].Kind != vt.EventKey || events[0].Key.Code != vt.KeyEscape {
		t.Fatalf("deadline events = %v, want Escape", events)
	}
}

func TestTimedInputDecoderResetDiscardsIncompleteInput(t *testing.T) {
	clock := NewVirtualClock()
	decoder := NewTimedInputDecoder(clock, 25*time.Millisecond)
	if events := decoder.Feed([]byte{0x1B}); len(events) != 0 {
		t.Fatalf("initial events = %v, want none", events)
	}

	decoder.Reset()
	clock.Advance(25 * time.Millisecond)

	if events := decoder.Poll(); len(events) != 0 || decoder.HasPending() {
		t.Fatalf("events after reset = %v, pending = %t", events, decoder.HasPending())
	}
}

func TestTimedInputDecoderResetPreservesKittyModifierSemantics(t *testing.T) {
	decoder := NewTimedInputDecoder(NewVirtualClock(), 25*time.Millisecond)
	decoder.SetKittyKeyboardMode(true)
	if events := decoder.Feed([]byte{0x1B}); len(events) != 0 {
		t.Fatalf("initial events = %v, want none", events)
	}

	decoder.Reset()
	events := decoder.Feed([]byte("\x1B[1;9A"))
	if len(events) != 1 || events[0].Kind != vt.EventKey || events[0].Key.Code != vt.KeyUp ||
		!events[0].Key.Modifiers.Super || events[0].Key.Modifiers.Meta ||
		events[0].Key.Action != vt.KeyPress || events[0].Key.Protocol != vt.KeyProtocolKitty {
		t.Fatalf("events after reset = %#v, want Kitty Super+Up press", events)
	}
}

func TestTimedInputDecoderRejectsNilClock(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("constructor accepted a nil clock")
		}
	}()
	NewTimedInputDecoder(nil, 0)
}
