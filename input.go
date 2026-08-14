package tui

import (
	"time"

	"github.com/mayahiro/nagi-go/vt"
)

// EventActionKind identifies an application-level terminal event decision
type EventActionKind uint8

const (
	// EventIgnore consumes an event without changing application state
	EventIgnore EventActionKind = iota
	// EventMessage adds a message to the application queue
	EventMessage
	// EventExit finishes the event loop normally
	EventExit
)

// EventAction is the application-level decision for one normalized event
type EventAction[Message any] struct {
	kind    EventActionKind
	message Message
}

// MessageAction returns an action that adds message to the application queue
func MessageAction[Message any](message Message) EventAction[Message] {
	return EventAction[Message]{kind: EventMessage, message: message}
}

// ExitAction returns an action that finishes the event loop normally
func ExitAction[Message any]() EventAction[Message] {
	return EventAction[Message]{kind: EventExit}
}

// IgnoreAction returns an action that consumes an event without state changes
func IgnoreAction[Message any]() EventAction[Message] {
	return EventAction[Message]{kind: EventIgnore}
}

// Kind returns the event decision variant
func (a EventAction[Message]) Kind() EventActionKind {
	return a.kind
}

// Message returns the queued message for EventMessage
func (a EventAction[Message]) Message() (Message, bool) {
	return a.message, a.kind == EventMessage
}

// TimedInputDecoder is a VT input decoder whose ambiguous lone-ESC timeout
// uses an injected clock
type TimedInputDecoder struct {
	decoder        *vt.Decoder
	clock          Clock
	escapeTimeout  time.Duration
	escapeDeadline Timestamp
	hasDeadline    bool
	kittyKeyboard  bool
}

// NewTimedInputDecoder returns a timed decoder
//
// It panics when clock is nil. A negative escape timeout is treated as zero.
func NewTimedInputDecoder(clock Clock, escapeTimeout time.Duration) *TimedInputDecoder {
	if clock == nil {
		panic("nagi-tui: nil input clock")
	}
	if escapeTimeout < 0 {
		escapeTimeout = 0
	}
	return &TimedInputDecoder{
		decoder:       vt.NewDecoder(),
		clock:         clock,
		escapeTimeout: escapeTimeout,
	}
}

// SetKittyKeyboardMode selects Kitty semantics for otherwise ambiguous
// function-key sequences
func (d *TimedInputDecoder) SetKittyKeyboardMode(enabled bool) {
	d.kittyKeyboard = enabled
	d.decoder.SetKittyKeyboardMode(enabled)
}

// Feed consumes one arbitrary input byte chunk
func (d *TimedInputDecoder) Feed(input []byte) []vt.Event {
	events := d.decoder.Feed(input)
	d.updateEscapeDeadline()
	return events
}

// Poll resolves a lone ESC when its deadline has elapsed
func (d *TimedInputDecoder) Poll() []vt.Event {
	if !d.hasDeadline || d.clock.Now() < d.escapeDeadline {
		return nil
	}
	d.hasDeadline = false
	return d.decoder.FlushPending()
}

// Flush resolves all currently incomplete input immediately
func (d *TimedInputDecoder) Flush() []vt.Event {
	d.hasDeadline = false
	return d.decoder.FlushPending()
}

// Reset discards incomplete terminal input without emitting an Event
func (d *TimedInputDecoder) Reset() {
	d.decoder = vt.NewDecoder()
	d.decoder.SetKittyKeyboardMode(d.kittyKeyboard)
	d.hasDeadline = false
}

// HasPending reports whether incomplete input is buffered
func (d *TimedInputDecoder) HasPending() bool {
	return d.decoder.HasPending()
}

// TimeUntilDeadline returns time until a lone-ESC deadline
func (d *TimedInputDecoder) TimeUntilDeadline() (time.Duration, bool) {
	if !d.hasDeadline {
		return 0, false
	}
	now := d.clock.Now()
	if now >= d.escapeDeadline {
		return 0, true
	}
	difference := d.escapeDeadline - now
	if difference > Timestamp(^uint64(0)>>1) {
		return time.Duration(^uint64(0) >> 1), true
	}
	return time.Duration(difference), true
}

func (d *TimedInputDecoder) updateEscapeDeadline() {
	d.hasDeadline = d.decoder.HasPendingEscape()
	if d.hasDeadline {
		d.escapeDeadline = d.clock.Now().Add(d.escapeTimeout)
	}
}
