package tui

import (
	"context"
	"fmt"
	"math"
	"sync/atomic"
	"time"
)

// SubscriptionDiagnostics contains lifecycle, backpressure, and failure
// counters
type SubscriptionDiagnostics struct {
	starts             uint64
	stops              uint64
	blockedSends       uint64
	latestReplacements uint64
	batchFlushes       uint64
	discardedMessages  uint64
	producerPanics     uint64
}

// Starts returns the number of started subscription generations
func (d SubscriptionDiagnostics) Starts() uint64 { return d.starts }

// Stops returns the number of stopped subscription generations
func (d SubscriptionDiagnostics) Stops() uint64 { return d.stops }

// BlockedSends returns Stream sends that encountered a full inbox
func (d SubscriptionDiagnostics) BlockedSends() uint64 { return d.blockedSends }

// LatestReplacements returns pending Latest values replaced before delivery
func (d SubscriptionDiagnostics) LatestReplacements() uint64 { return d.latestReplacements }

// BatchFlushes returns count- or time-triggered Batch releases
func (d SubscriptionDiagnostics) BatchFlushes() uint64 { return d.batchFlushes }

// DiscardedMessages returns values discarded when a generation stopped
func (d SubscriptionDiagnostics) DiscardedMessages() uint64 { return d.discardedMessages }

// ProducerPanics returns producer or Every factory panics isolated by the
// supervisor
func (d SubscriptionDiagnostics) ProducerPanics() uint64 { return d.producerPanics }

// DuplicateSubscriptionKeyError indicates that one declaration reused a key
type DuplicateSubscriptionKeyError struct {
	// Key is the duplicated application-defined identity
	Key SubscriptionKey
}

// Error returns the duplicate identity diagnostic
func (e *DuplicateSubscriptionKeyError) Error() string {
	return "duplicate SubscriptionKey " + e.Key.String()
}

type subscriptionTag struct {
	key        SubscriptionKey
	generation uint64
}

type subscriptionMessage[Message any] struct {
	tag     subscriptionTag
	message Message
}

type subscriptionReconciliation struct {
	stopped []subscriptionTag
}

type subscriptionFingerprint struct {
	kind     subscriptionProducerKind
	interval time.Duration
	policy   DeliveryPolicy
}

type activeSubscription[Message any] struct {
	tag              subscriptionTag
	fingerprint      subscriptionFingerprint
	policy           DeliveryPolicy
	inbox            *subscriptionInbox[Message]
	kind             subscriptionProducerKind
	interval         time.Duration
	nextDue          Timestamp
	hasNextDue       bool
	factory          func() Message
	cancel           context.CancelFunc
	finished         *atomic.Bool
	batchDeadline    Timestamp
	hasBatchDeadline bool
	batchReady       bool
}

type subscriptionSupervisor[Message any] struct {
	parent           context.Context
	inboxCapacity    int
	wake             runtimeWake
	active           map[SubscriptionKey]*activeSubscription[Message]
	order            []SubscriptionKey
	generations      map[SubscriptionKey]uint64
	sequence         atomic.Uint64
	atomicDiagnostic subscriptionAtomicDiagnostics
	starts           uint64
	stops            uint64
	batchFlushes     uint64
	notices          *runtimeNoticeQueue
}

func newSubscriptionSupervisor[Message any](inboxCapacity int) *subscriptionSupervisor[Message] {
	return newSubscriptionSupervisorContext[Message](context.Background(), inboxCapacity)
}

func newSubscriptionSupervisorContext[Message any](parent context.Context, inboxCapacity int) *subscriptionSupervisor[Message] {
	return &subscriptionSupervisor[Message]{
		parent:        parent,
		inboxCapacity: inboxCapacity,
		active:        make(map[SubscriptionKey]*activeSubscription[Message]),
		generations:   make(map[SubscriptionKey]uint64),
	}
}

func (s *subscriptionSupervisor[Message]) reconcile(
	subscription Subscription[Message],
	now Timestamp,
) (subscriptionReconciliation, error) {
	var sources []*subscriptionSource[Message]
	flattenSubscriptions(subscription, &sources)
	keys := make(map[SubscriptionKey]struct{}, len(sources))
	for _, source := range sources {
		if _, duplicate := keys[source.key]; duplicate {
			return subscriptionReconciliation{}, &DuplicateSubscriptionKeyError{Key: source.key}
		}
		keys[source.key] = struct{}{}
	}

	reconciliation := subscriptionReconciliation{}
	nextOrder := make([]SubscriptionKey, 0, len(sources))
	for _, source := range sources {
		fingerprint := subscriptionSourceFingerprint(source)
		current, exists := s.active[source.key]
		if !exists || current.fingerprint != fingerprint {
			if exists {
				reconciliation.stopped = append(reconciliation.stopped, s.stopActive(current))
				delete(s.active, source.key)
			}
			s.active[source.key] = s.startSource(source, fingerprint, now)
		}
		nextOrder = append(nextOrder, source.key)
	}
	for _, key := range s.order {
		if _, retained := keys[key]; retained {
			continue
		}
		if active, exists := s.active[key]; exists {
			reconciliation.stopped = append(reconciliation.stopped, s.stopActive(active))
			delete(s.active, key)
		}
	}
	s.order = nextOrder
	return reconciliation, nil
}

func (s *subscriptionSupervisor[Message]) poll(now Timestamp) {
	for _, key := range s.order {
		active, exists := s.active[key]
		if !exists {
			continue
		}
		pollEverySubscription(active, now)
		updateSubscriptionBatch(active, now, &s.batchFlushes)
	}
}

func (s *subscriptionSupervisor[Message]) takeReady(maximum int) []subscriptionMessage[Message] {
	ready := make([]subscriptionMessage[Message], 0, min(maximum, 64))
	for len(ready) < maximum {
		var selectedKey SubscriptionKey
		var selectedSequence uint64
		selected := false
		for _, key := range s.order {
			active, exists := s.active[key]
			if !exists || !subscriptionSourceReady(active) {
				continue
			}
			sequence, ok := active.inbox.frontSequence()
			if ok && (!selected || sequence < selectedSequence) {
				selected = true
				selectedKey = key
				selectedSequence = sequence
			}
		}
		if !selected {
			break
		}
		active, exists := s.active[selectedKey]
		if !exists {
			continue
		}
		envelope, ok := active.inbox.popIfSequence(selectedSequence)
		if !ok {
			continue
		}
		ready = append(ready, subscriptionMessage[Message]{tag: active.tag, message: envelope.message})
		if active.inbox.len() == 0 {
			active.hasBatchDeadline = false
			active.batchReady = false
		}
	}
	return ready
}

func (s *subscriptionSupervisor[Message]) activeSubscriptions() int { return len(s.active) }

func (s *subscriptionSupervisor[Message]) runningStreams() int {
	running := 0
	for _, active := range s.active {
		if active.kind == subscriptionStream && !active.finished.Load() {
			running++
		}
	}
	return running
}

func (s *subscriptionSupervisor[Message]) pendingMessages() int {
	pending := 0
	for _, active := range s.active {
		pending += active.inbox.len()
	}
	return pending
}

func (s *subscriptionSupervisor[Message]) isActive(key SubscriptionKey) bool {
	_, active := s.active[key]
	return active
}

func (s *subscriptionSupervisor[Message]) generation(key SubscriptionKey) uint64 {
	return s.generations[key]
}

func (s *subscriptionSupervisor[Message]) diagnostics() SubscriptionDiagnostics {
	return SubscriptionDiagnostics{
		starts:             s.starts,
		stops:              s.stops,
		blockedSends:       s.atomicDiagnostic.blockedSends.Load(),
		latestReplacements: s.atomicDiagnostic.latestReplacements.Load(),
		batchFlushes:       s.batchFlushes,
		discardedMessages:  s.atomicDiagnostic.discardedMessages.Load(),
		producerPanics:     s.atomicDiagnostic.producerPanics.Load(),
	}
}

func (s *subscriptionSupervisor[Message]) noteDiscarded(count int) {
	s.atomicDiagnostic.discardedMessages.Add(uint64(count))
}

func (s *subscriptionSupervisor[Message]) timeUntilDeadline(now Timestamp) (time.Duration, bool) {
	var earliest Timestamp
	found := false
	for _, active := range s.active {
		if subscriptionSourceReady(active) {
			return 0, true
		}
		if active.kind == subscriptionEvery && active.hasNextDue && (!found || active.nextDue < earliest) {
			earliest = active.nextDue
			found = true
		}
		if active.hasBatchDeadline && !active.batchReady && active.inbox.len() > 0 && (!found || active.batchDeadline < earliest) {
			earliest = active.batchDeadline
			found = true
		}
	}
	if !found || earliest <= now {
		return 0, found
	}
	return time.Duration(earliest - now), true
}

func (s *subscriptionSupervisor[Message]) close() {
	for _, key := range s.order {
		if active, exists := s.active[key]; exists {
			s.stopActive(active)
			delete(s.active, key)
		}
	}
	s.order = nil
}

func (s *subscriptionSupervisor[Message]) startSource(
	source *subscriptionSource[Message],
	fingerprint subscriptionFingerprint,
	now Timestamp,
) *activeSubscription[Message] {
	generation := s.generations[source.key] + 1
	if generation == 0 {
		generation = math.MaxUint64
	}
	s.generations[source.key] = generation
	s.starts++
	inbox := newSubscriptionInbox[Message](
		s.inboxCapacity,
		source.policy,
		&s.sequence,
		&s.atomicDiagnostic,
		s.wake,
	)
	active := &activeSubscription[Message]{
		tag:         subscriptionTag{key: source.key, generation: generation},
		fingerprint: fingerprint,
		policy:      source.policy,
		inbox:       inbox,
		kind:        source.kind,
	}
	switch source.kind {
	case subscriptionEvery:
		active.interval = source.interval
		active.factory = source.factory
		active.nextDue, active.hasNextDue = subscriptionTimestampAfter(now, source.interval)
	case subscriptionStream:
		ctx, cancel := context.WithCancel(s.parent)
		active.cancel = cancel
		active.finished = &atomic.Bool{}
		finished := active.finished
		stream := source.stream
		sink := SubscriptionSink[Message]{inbox: inbox}
		diagnostics := &s.atomicDiagnostic
		notices := s.notices
		wake := s.wake
		tag := active.tag
		go func() {
			panicked := false
			defer func() {
				if recover() != nil {
					panicked = true
					diagnostics.producerPanics.Add(1)
				}
				finished.Store(true)
				switch {
				case panicked:
					notices.push(subscriptionRuntimeNotice(
						RuntimeNoticeSubscriptionStreamPanicked,
						tag.key,
						tag.generation,
					))
				case ctx.Err() == nil && !sink.Closed():
					notices.push(subscriptionRuntimeNotice(
						RuntimeNoticeSubscriptionStreamCompleted,
						tag.key,
						tag.generation,
					))
				}
				wake.notify()
			}()
			stream(ctx, sink)
		}()
	}
	return active
}

func (s *subscriptionSupervisor[Message]) stopActive(active *activeSubscription[Message]) subscriptionTag {
	if active.cancel != nil {
		active.cancel()
	}
	active.inbox.stop()
	s.stops++
	return active.tag
}

func flattenSubscriptions[Message any](
	subscription Subscription[Message],
	sources *[]*subscriptionSource[Message],
) {
	switch subscription.kind {
	case subscriptionNone:
	case subscriptionBatch:
		for _, child := range subscription.subscriptions {
			flattenSubscriptions(child, sources)
		}
	case subscriptionSourceKind:
		*sources = append(*sources, subscription.source)
	}
}

func subscriptionSourceFingerprint[Message any](source *subscriptionSource[Message]) subscriptionFingerprint {
	return subscriptionFingerprint{kind: source.kind, interval: source.interval, policy: source.policy}
}

func pollEverySubscription[Message any](active *activeSubscription[Message], now Timestamp) {
	if active.kind != subscriptionEvery || !active.hasNextDue || active.nextDue > now {
		return
	}
	if active.policy.kind == deliveryLatest {
		active.inbox.pushRuntime(active.factory)
		active.nextDue, active.hasNextDue = subscriptionTimestampAfterNow(active.nextDue, active.interval, now)
		return
	}
	for active.hasNextDue && active.nextDue <= now {
		if !active.inbox.pushRuntime(active.factory) {
			break
		}
		active.nextDue, active.hasNextDue = subscriptionTimestampAfter(active.nextDue, active.interval)
	}
}

func updateSubscriptionBatch[Message any](active *activeSubscription[Message], now Timestamp, flushes *uint64) {
	if active.policy.kind != deliveryBatch {
		return
	}
	if active.inbox.len() == 0 {
		active.hasBatchDeadline = false
		active.batchReady = false
		return
	}
	if !active.hasBatchDeadline {
		active.batchDeadline, active.hasBatchDeadline = subscriptionTimestampAfter(now, active.policy.maximumDelay)
		if !active.hasBatchDeadline {
			active.batchDeadline = math.MaxUint64
			active.hasBatchDeadline = true
		}
	}
	if !active.batchReady && (active.inbox.len() >= active.policy.maximumMessages || active.batchDeadline <= now) {
		active.batchReady = true
		(*flushes)++
	}
}

func subscriptionSourceReady[Message any](active *activeSubscription[Message]) bool {
	return active.inbox.len() > 0 && (active.policy.kind != deliveryBatch || active.batchReady)
}

func subscriptionTimestampAfter(timestamp Timestamp, duration time.Duration) (Timestamp, bool) {
	if duration < 0 {
		return 0, false
	}
	delta := uint64(duration)
	if math.MaxUint64-uint64(timestamp) < delta {
		return 0, false
	}
	return timestamp + Timestamp(delta), true
}

func subscriptionTimestampAfterNow(due Timestamp, interval time.Duration, now Timestamp) (Timestamp, bool) {
	steps := (uint64(now-due) / uint64(interval)) + 1
	if steps > math.MaxUint64/uint64(interval) {
		return 0, false
	}
	delta := steps * uint64(interval)
	if math.MaxUint64-uint64(due) < delta {
		return 0, false
	}
	return due + Timestamp(delta), true
}

func (s subscriptionTag) String() string {
	return fmt.Sprintf("%s:%d", s.key, s.generation)
}
