package tui

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// SubscriptionKey is a stable identity for one long-lived source
type SubscriptionKey string

// NewSubscriptionKey returns a key from an application-defined stable value
func NewSubscriptionKey(value string) SubscriptionKey {
	return SubscriptionKey(value)
}

// String returns the application-defined key
func (k SubscriptionKey) String() string {
	return string(k)
}

type deliveryKind uint8

const (
	deliveryReliable deliveryKind = iota
	deliveryLatest
	deliveryBatch
)

// DeliveryPolicy defines bounded delivery behavior for one subscription
type DeliveryPolicy struct {
	kind            deliveryKind
	maximumMessages int
	maximumDelay    time.Duration
}

// ReliableDelivery preserves every value in FIFO order and blocks a full
// Stream inbox
func ReliableDelivery() DeliveryPolicy {
	return DeliveryPolicy{kind: deliveryReliable}
}

// LatestDelivery retains only the newest value not yet delivered to Update
func LatestDelivery() DeliveryPolicy {
	return DeliveryPolicy{kind: deliveryLatest}
}

// BatchDelivery releases FIFO values after maximumMessages or maximumDelay
//
// It panics when maximumMessages is not positive or maximumDelay is negative.
func BatchDelivery(maximumMessages int, maximumDelay time.Duration) DeliveryPolicy {
	if maximumMessages <= 0 {
		panic("nagi-tui: batch delivery maximum must be positive")
	}
	if maximumDelay < 0 {
		panic("nagi-tui: batch delivery delay must not be negative")
	}
	return DeliveryPolicy{
		kind:            deliveryBatch,
		maximumMessages: maximumMessages,
		maximumDelay:    maximumDelay,
	}
}

// IsReliable reports whether every value is delivered reliably
func (p DeliveryPolicy) IsReliable() bool {
	return p.kind == deliveryReliable
}

// IsLatest reports whether only the newest pending value is retained
func (p DeliveryPolicy) IsLatest() bool {
	return p.kind == deliveryLatest
}

// BatchLimits returns the count and delay for Batch delivery
func (p DeliveryPolicy) BatchLimits() (int, time.Duration, bool) {
	return p.maximumMessages, p.maximumDelay, p.kind == deliveryBatch
}

// SubscriptionSink is a cancellation-aware sender owned by one Stream
// producer
type SubscriptionSink[Message any] struct {
	inbox *subscriptionInbox[Message]
}

// Send submits a value according to the source delivery policy
//
// Reliable and Batch delivery can block while the bounded inbox is full. It
// returns false after the runtime stops this subscription generation.
func (s SubscriptionSink[Message]) Send(message Message) bool {
	if s.inbox == nil {
		return false
	}
	return s.inbox.send(message)
}

// Done is closed when the runtime stops this subscription generation
func (s SubscriptionSink[Message]) Done() <-chan struct{} {
	if s.inbox == nil {
		closed := make(chan struct{})
		close(closed)
		return closed
	}
	return s.inbox.done
}

// Closed reports whether the runtime stopped this subscription generation
func (s SubscriptionSink[Message]) Closed() bool {
	if s.inbox == nil {
		return true
	}
	return s.inbox.isStopped()
}

type subscriptionKind uint8

const (
	subscriptionNone subscriptionKind = iota
	subscriptionBatch
	subscriptionSourceKind
)

type subscriptionProducerKind uint8

const (
	subscriptionEvery subscriptionProducerKind = iota
	subscriptionStream
)

// SubscriptionStream is one cooperatively cancellable long-lived producer
type SubscriptionStream[Message any] func(context.Context, SubscriptionSink[Message])

type subscriptionSource[Message any] struct {
	key      SubscriptionKey
	policy   DeliveryPolicy
	kind     subscriptionProducerKind
	interval time.Duration
	factory  func() Message
	stream   SubscriptionStream[Message]
}

// Subscription is a declarative set of long-lived application message sources
type Subscription[Message any] struct {
	kind          subscriptionKind
	subscriptions []Subscription[Message]
	source        *subscriptionSource[Message]
}

// NoneSubscription returns an empty subscription set
func NoneSubscription[Message any]() Subscription[Message] {
	return Subscription[Message]{}
}

// BatchSubscriptions combines declarations while preserving declaration order
func BatchSubscriptions[Message any](subscriptions ...Subscription[Message]) Subscription[Message] {
	return Subscription[Message]{
		kind:          subscriptionBatch,
		subscriptions: append([]Subscription[Message](nil), subscriptions...),
	}
}

// EverySubscription returns a runtime-clock interval source
//
// The first value is produced after one full interval. It panics when interval
// is not positive or factory is nil.
func EverySubscription[Message any](
	key SubscriptionKey,
	interval time.Duration,
	policy DeliveryPolicy,
	factory func() Message,
) Subscription[Message] {
	if interval <= 0 {
		panic("nagi-tui: subscription interval must be positive")
	}
	if factory == nil {
		panic("nagi-tui: nil subscription factory")
	}
	return Subscription[Message]{
		kind: subscriptionSourceKind,
		source: &subscriptionSource[Message]{
			key:      key,
			policy:   policy,
			kind:     subscriptionEvery,
			interval: interval,
			factory:  factory,
		},
	}
}

// StreamSubscription returns a cooperatively cancellable long-lived source
//
// It panics when stream is nil.
func StreamSubscription[Message any](
	key SubscriptionKey,
	policy DeliveryPolicy,
	stream SubscriptionStream[Message],
) Subscription[Message] {
	if stream == nil {
		panic("nagi-tui: nil subscription stream")
	}
	return Subscription[Message]{
		kind: subscriptionSourceKind,
		source: &subscriptionSource[Message]{
			key:    key,
			policy: policy,
			kind:   subscriptionStream,
			stream: stream,
		},
	}
}

// IsNone reports whether this declaration contains no sources
func (s Subscription[Message]) IsNone() bool {
	switch s.kind {
	case subscriptionNone:
		return true
	case subscriptionBatch:
		for _, subscription := range s.subscriptions {
			if !subscription.IsNone() {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// String returns a diagnostic description of the subscription shape
func (s Subscription[Message]) String() string {
	switch s.kind {
	case subscriptionNone:
		return "Subscription::None"
	case subscriptionBatch:
		return fmt.Sprintf("Subscription::Batch(%d)", len(s.subscriptions))
	default:
		return fmt.Sprintf("Subscription::Source(%s)", s.source.key)
	}
}

type subscriptionEnvelope[Message any] struct {
	sequence uint64
	message  Message
}

type subscriptionAtomicDiagnostics struct {
	blockedSends       atomic.Uint64
	latestReplacements atomic.Uint64
	discardedMessages  atomic.Uint64
	producerPanics     atomic.Uint64
}

type subscriptionInbox[Message any] struct {
	mu          sync.Mutex
	changed     *sync.Cond
	done        chan struct{}
	stopped     bool
	messages    []subscriptionEnvelope[Message]
	capacity    int
	policy      DeliveryPolicy
	sequence    *atomic.Uint64
	diagnostics *subscriptionAtomicDiagnostics
	wake        runtimeWake
}

func newSubscriptionInbox[Message any](
	capacity int,
	policy DeliveryPolicy,
	sequence *atomic.Uint64,
	diagnostics *subscriptionAtomicDiagnostics,
	wake runtimeWake,
) *subscriptionInbox[Message] {
	inbox := &subscriptionInbox[Message]{
		done:        make(chan struct{}),
		messages:    make([]subscriptionEnvelope[Message], 0, min(capacity, 64)),
		capacity:    capacity,
		policy:      policy,
		sequence:    sequence,
		diagnostics: diagnostics,
		wake:        wake,
	}
	inbox.changed = sync.NewCond(&inbox.mu)
	return inbox
}

func (i *subscriptionInbox[Message]) send(message Message) bool {
	i.mu.Lock()
	if i.stopped {
		i.mu.Unlock()
		return false
	}
	if i.policy.kind == deliveryLatest {
		if len(i.messages) > 0 {
			i.diagnostics.latestReplacements.Add(uint64(len(i.messages)))
			i.messages = i.messages[:0]
		}
	} else if len(i.messages) >= i.capacity {
		i.diagnostics.blockedSends.Add(1)
		for !i.stopped && len(i.messages) >= i.capacity {
			i.changed.Wait()
		}
		if i.stopped {
			i.mu.Unlock()
			return false
		}
	}
	i.messages = append(i.messages, subscriptionEnvelope[Message]{
		sequence: nextSubscriptionSequence(i.sequence),
		message:  message,
	})
	i.changed.Broadcast()
	i.mu.Unlock()
	i.wake.notify()
	return true
}

func (i *subscriptionInbox[Message]) pushRuntime(factory func() Message) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.stopped {
		return false
	}
	if i.policy.kind == deliveryLatest {
		if len(i.messages) > 0 {
			i.diagnostics.latestReplacements.Add(uint64(len(i.messages)))
			i.messages = i.messages[:0]
		}
	} else if len(i.messages) >= i.capacity {
		return false
	}
	message, ok := callSubscriptionFactory(factory, i.diagnostics)
	if !ok {
		return true
	}
	i.messages = append(i.messages, subscriptionEnvelope[Message]{
		sequence: nextSubscriptionSequence(i.sequence),
		message:  message,
	})
	return true
}

func callSubscriptionFactory[Message any](
	factory func() Message,
	diagnostics *subscriptionAtomicDiagnostics,
) (message Message, ok bool) {
	ok = true
	defer func() {
		if recover() != nil {
			diagnostics.producerPanics.Add(1)
			ok = false
		}
	}()
	message = factory()
	return message, ok
}

func (i *subscriptionInbox[Message]) frontSequence() (uint64, bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if len(i.messages) == 0 {
		return 0, false
	}
	return i.messages[0].sequence, true
}

func (i *subscriptionInbox[Message]) popIfSequence(sequence uint64) (subscriptionEnvelope[Message], bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if len(i.messages) == 0 || i.messages[0].sequence != sequence {
		return subscriptionEnvelope[Message]{}, false
	}
	message := i.messages[0]
	var zero subscriptionEnvelope[Message]
	i.messages[0] = zero
	i.messages = i.messages[1:]
	i.changed.Broadcast()
	return message, true
}

func (i *subscriptionInbox[Message]) len() int {
	i.mu.Lock()
	defer i.mu.Unlock()
	return len(i.messages)
}

func (i *subscriptionInbox[Message]) stop() int {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.stopped {
		return 0
	}
	i.stopped = true
	discarded := len(i.messages)
	for index := range i.messages {
		var zero subscriptionEnvelope[Message]
		i.messages[index] = zero
	}
	i.messages = i.messages[:0]
	i.diagnostics.discardedMessages.Add(uint64(discarded))
	close(i.done)
	i.changed.Broadcast()
	return discarded
}

func (i *subscriptionInbox[Message]) isStopped() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.stopped
}

func nextSubscriptionSequence(sequence *atomic.Uint64) uint64 {
	for {
		current := sequence.Load()
		next := current + 1
		if next < current {
			next = ^uint64(0)
		}
		if sequence.CompareAndSwap(current, next) {
			return next
		}
	}
}
