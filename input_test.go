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

func TestTimedInputDecoderRejectsNilClock(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("constructor accepted a nil clock")
		}
	}()
	NewTimedInputDecoder(nil, 0)
}
