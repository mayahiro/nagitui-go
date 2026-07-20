package tuitest

import (
	"context"
	"sync"

	"github.com/mayahiro/nagitui-go"
)

type manualSubscriptionState[Message any] struct {
	mu             sync.Mutex
	changed        *sync.Cond
	nextSourceID   uint64
	activeSourceID uint64
	hasActive      bool
	sink           tui.SubscriptionSink[Message]
	starts         uint64
	stops          uint64
}

// ManualSubscription is a deterministic handle that declares, observes, and
// feeds one Stream source
type ManualSubscription[Message any] struct {
	state *manualSubscriptionState[Message]
}

// NewManualSubscription returns a deterministic manual Stream source handle
func NewManualSubscription[Message any]() *ManualSubscription[Message] {
	state := &manualSubscriptionState[Message]{}
	state.changed = sync.NewCond(&state.mu)
	return &ManualSubscription[Message]{state: state}
}

// Subscription returns a Stream declaration backed by this handle
func (m *ManualSubscription[Message]) Subscription(
	key tui.SubscriptionKey,
	policy tui.DeliveryPolicy,
) tui.Subscription[Message] {
	m.state.mu.Lock()
	m.state.nextSourceID++
	sourceID := m.state.nextSourceID
	m.state.mu.Unlock()
	return tui.StreamSubscription(key, policy, func(ctx context.Context, sink tui.SubscriptionSink[Message]) {
		m.state.mu.Lock()
		m.state.activeSourceID = sourceID
		m.state.hasActive = true
		m.state.sink = sink
		m.state.starts++
		m.state.changed.Broadcast()
		m.state.mu.Unlock()

		select {
		case <-ctx.Done():
		case <-sink.Done():
		}

		m.state.mu.Lock()
		if m.state.hasActive && m.state.activeSourceID == sourceID {
			m.state.hasActive = false
			var zero tui.SubscriptionSink[Message]
			m.state.sink = zero
		}
		m.state.stops++
		m.state.changed.Broadcast()
		m.state.mu.Unlock()
	})
}

// Send submits one value through the current generation
func (m *ManualSubscription[Message]) Send(message Message) bool {
	m.state.mu.Lock()
	if !m.state.hasActive {
		m.state.mu.Unlock()
		return false
	}
	sink := m.state.sink
	m.state.mu.Unlock()
	return sink.Send(message)
}

// WaitStarted blocks until at least one generation starts
func (m *ManualSubscription[Message]) WaitStarted() {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	for m.state.starts == 0 {
		m.state.changed.Wait()
	}
}

// WaitStopped blocks until at least one generation stops
func (m *ManualSubscription[Message]) WaitStopped() {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	for m.state.stops == 0 {
		m.state.changed.Wait()
	}
}

// Active reports whether a generation is currently active
func (m *ManualSubscription[Message]) Active() bool {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	return m.state.hasActive
}

// Starts returns the number of observed generation starts
func (m *ManualSubscription[Message]) Starts() uint64 {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	return m.state.starts
}

// Stops returns the number of observed generation stops
func (m *ManualSubscription[Message]) Stops() uint64 {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	return m.state.stops
}
