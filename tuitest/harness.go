package tuitest

import (
	"time"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/surface"
)

// Harness is a deterministic application driver with virtual input and time
type Harness[Message any] struct {
	runtime       *tui.Runtime[Message]
	decoder       *tui.TimedInputDecoder
	clock         *tui.VirtualClock
	mapEvent      func(vt.Event) tui.EventAction[Message]
	frames        []tui.Frame
	messages      []Message
	exitRequested bool
}

// New returns a harness with default runtime settings and a 25 ms ESC timeout
func New[Message any](
	app tui.App[Message],
	size tui.Size,
	mapEvent func(vt.Event) tui.EventAction[Message],
) (*Harness[Message], error) {
	return NewWithConfig(app, tui.NewRuntimeConfig(size), 25*time.Millisecond, mapEvent)
}

// NewWithConfig returns a harness with explicit settings and ESC timeout
func NewWithConfig[Message any](
	app tui.App[Message],
	config tui.RuntimeConfig,
	escapeTimeout time.Duration,
	mapEvent func(vt.Event) tui.EventAction[Message],
) (*Harness[Message], error) {
	clock := tui.NewVirtualClock()
	runtime, err := tui.NewRuntimeWithClock(app, config, clock)
	if err != nil {
		return nil, err
	}
	harness := &Harness[Message]{
		runtime:  runtime,
		decoder:  tui.NewTimedInputDecoder(clock, escapeTimeout),
		clock:    clock,
		mapEvent: mapEvent,
	}
	if err := harness.captureFrame(); err != nil {
		runtime.Close()
		return nil, err
	}
	return harness, nil
}

// Close cooperatively cancels active effect tasks and timers
func (h *Harness[Message]) Close() {
	h.runtime.Close()
}

// App returns the application instance owned by the runtime
func (h *Harness[Message]) App() tui.App[Message] {
	return h.runtime.App()
}

// Interaction returns runtime-owned Interaction State for assertions
func (h *Harness[Message]) Interaction() *tui.InteractionState {
	return h.runtime.Interaction()
}

// RequestFocus requests focus for a focusable ID in the current semantic tree
func (h *Harness[Message]) RequestFocus(id tui.NodeID) (bool, error) {
	return h.runtime.RequestFocus(id)
}

// ClearFocus releases node focus
func (h *Harness[Message]) ClearFocus() {
	h.runtime.ClearFocus()
}

// SetTextCursor sets a TextInput UTF-8 byte cursor at a grapheme boundary
func (h *Harness[Message]) SetTextCursor(id tui.NodeID, cursor int) bool {
	return h.runtime.SetTextCursor(id, cursor)
}

// SetScrollOffset requests an offset that is clamped during layout
func (h *Harness[Message]) SetScrollOffset(id tui.NodeID, offset tui.ScrollOffset) bool {
	return h.runtime.SetScrollOffset(id, offset)
}

// ScrollState returns resolved ScrollViewport state for assertions
func (h *Harness[Message]) ScrollState(id tui.NodeID) (tui.ScrollState, bool) {
	return h.runtime.Interaction().ScrollState(id)
}

// ActiveActionGroups returns resolved action groups on the active target-to-root route
func (h *Harness[Message]) ActiveActionGroups() ([]tui.ResolvedActions, error) {
	return h.runtime.ActiveActionGroups()
}

// ActiveTasks returns supervised tasks that have not fully finished
func (h *Harness[Message]) ActiveTasks() int {
	return h.runtime.ActiveTasks()
}

// RunningTasks returns tasks occupying worker slots
func (h *Harness[Message]) RunningTasks() int {
	return h.runtime.RunningTasks()
}

// PendingTasks returns tasks waiting for a worker slot
func (h *Harness[Message]) PendingTasks() int {
	return h.runtime.PendingTasks()
}

// PendingEffectMessages returns completed messages waiting for queue capacity
func (h *Harness[Message]) PendingEffectMessages() int {
	return h.runtime.PendingEffectMessages()
}

// EffectDiagnostics returns supervised effect diagnostic counters
func (h *Harness[Message]) EffectDiagnostics() tui.EffectDiagnostics {
	return h.runtime.EffectDiagnostics()
}

// TaskGeneration returns the latest generation started for one task key
func (h *Harness[Message]) TaskGeneration(key tui.TaskKey) uint64 {
	return h.runtime.TaskGeneration(key)
}

// ActiveSubscriptions returns the number of currently declared sources
func (h *Harness[Message]) ActiveSubscriptions() int {
	return h.runtime.ActiveSubscriptions()
}

// RunningSubscriptionStreams returns Stream producers that have not returned
func (h *Harness[Message]) RunningSubscriptionStreams() int {
	return h.runtime.RunningSubscriptionStreams()
}

// PendingSubscriptionMessages returns values waiting before the app queue
func (h *Harness[Message]) PendingSubscriptionMessages() int {
	return h.runtime.PendingSubscriptionMessages()
}

// SubscriptionActive reports whether key is currently declared
func (h *Harness[Message]) SubscriptionActive(key tui.SubscriptionKey) bool {
	return h.runtime.SubscriptionActive(key)
}

// SubscriptionGeneration returns the latest started generation for key
func (h *Harness[Message]) SubscriptionGeneration(key tui.SubscriptionKey) uint64 {
	return h.runtime.SubscriptionGeneration(key)
}

// SubscriptionDiagnostics returns lifecycle and backpressure counters
func (h *Harness[Message]) SubscriptionDiagnostics() tui.SubscriptionDiagnostics {
	return h.runtime.SubscriptionDiagnostics()
}

// Send injects one application message and completes one coalesced step
func (h *Harness[Message]) Send(message Message) error {
	if err := h.runtime.Enqueue(message); err != nil {
		return err
	}
	return h.Step()
}

// Input injects raw terminal bytes and completes one coalesced step
func (h *Harness[Message]) Input(input []byte) error {
	if err := h.dispatch(h.decoder.Feed(input)); err != nil {
		return err
	}
	return h.Step()
}

// FlushInput resolves incomplete input and completes one coalesced step
func (h *Harness[Message]) FlushInput() error {
	if err := h.dispatch(h.decoder.Flush()); err != nil {
		return err
	}
	return h.Step()
}

// Advance moves virtual time, resolves due input deadlines, and steps once
func (h *Harness[Message]) Advance(duration time.Duration) error {
	h.clock.Advance(duration)
	if err := h.dispatch(h.decoder.Poll()); err != nil {
		return err
	}
	return h.Step()
}

// Resize changes virtual terminal size and renders one coalesced frame
func (h *Harness[Message]) Resize(size tui.Size) error {
	h.runtime.Resize(size)
	return h.Step()
}

// Step processes queued messages and captures at most one rendered frame
func (h *Harness[Message]) Step() error {
	if _, err := h.runtime.ProcessPendingWith(func(message Message) {
		h.messages = append(h.messages, message)
	}); err != nil {
		return err
	}
	return h.captureFrame()
}

// Frames returns copies of every captured frame, including the initial frame
func (h *Harness[Message]) Frames() []tui.Frame {
	return append([]tui.Frame(nil), h.frames...)
}

// LatestFrame returns the most recently captured frame
func (h *Harness[Message]) LatestFrame() tui.Frame {
	return h.frames[len(h.frames)-1]
}

// LatestSurface returns a copy of the most recently rendered surface
func (h *Harness[Message]) LatestSurface() *surface.Surface {
	return h.LatestFrame().Surface()
}

// MessageHistory returns messages delivered in injection order
func (h *Harness[Message]) MessageHistory() []Message {
	return append([]Message(nil), h.messages...)
}

// ExitRequested reports whether the application or event mapper requested normal exit
func (h *Harness[Message]) ExitRequested() bool {
	return h.exitRequested || h.runtime.ExitRequested()
}

func (h *Harness[Message]) dispatch(events []vt.Event) error {
	for _, event := range events {
		dispatch, err := h.runtime.DispatchEvent(event)
		if err != nil {
			return err
		}
		if dispatch.Consumed() {
			continue
		}
		action := h.mapEvent(event)
		switch action.Kind() {
		case tui.EventIgnore:
		case tui.EventMessage:
			message, _ := action.Message()
			if err := h.runtime.Enqueue(message); err != nil {
				return err
			}
		case tui.EventExit:
			h.exitRequested = true
			return nil
		}
	}
	return nil
}

func (h *Harness[Message]) captureFrame() error {
	frame, err := h.runtime.RenderIfDirty()
	if err != nil {
		return err
	}
	if frame != nil {
		h.frames = append(h.frames, *frame)
	}
	return nil
}
