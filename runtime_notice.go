package tui

import "sync"

// RuntimeNoticeKind identifies an asynchronous lifecycle event that does not
// produce an application Message
type RuntimeNoticeKind uint8

const (
	// RuntimeNoticeEffectPanicked reports a recovered Effect task panic
	RuntimeNoticeEffectPanicked RuntimeNoticeKind = iota
	// RuntimeNoticeEffectSpawnFailed reports failure to start an Effect worker
	//
	// Go runtimes currently cannot emit this notice, but the kind is shared with
	// runtimes whose worker primitive can fail during creation.
	RuntimeNoticeEffectSpawnFailed
	// RuntimeNoticeSubscriptionStreamCompleted reports a Stream that returned
	// while its subscription generation was still active
	RuntimeNoticeSubscriptionStreamCompleted
	// RuntimeNoticeSubscriptionStreamPanicked reports a recovered Stream panic
	RuntimeNoticeSubscriptionStreamPanicked
	// RuntimeNoticeSubscriptionSpawnFailed reports failure to start a Stream worker
	//
	// Go runtimes currently cannot emit this notice, but the kind is shared with
	// runtimes whose worker primitive can fail during creation.
	RuntimeNoticeSubscriptionSpawnFailed
)

// RuntimeNotice describes one recovered failure or unexpected asynchronous
// lifecycle transition
//
// Panic payloads are intentionally omitted. A keyed Effect includes its
// TaskKey and generation; a Stream notice includes its SubscriptionKey and
// generation.
type RuntimeNotice struct {
	kind            RuntimeNoticeKind
	taskKey         TaskKey
	subscriptionKey SubscriptionKey
	generation      uint64
	hasTask         bool
	hasSubscription bool
}

// Kind returns the lifecycle event kind
func (n RuntimeNotice) Kind() RuntimeNoticeKind { return n.kind }

// Task returns keyed Effect identity when the notice belongs to a LatestEffect
func (n RuntimeNotice) Task() (TaskKey, uint64, bool) {
	return n.taskKey, n.generation, n.hasTask
}

// Subscription returns Stream identity when the notice belongs to a Subscription
func (n RuntimeNotice) Subscription() (SubscriptionKey, uint64, bool) {
	return n.subscriptionKey, n.generation, n.hasSubscription
}

// RuntimeNoticeDiagnostics contains bounded notice queue counters
type RuntimeNoticeDiagnostics struct {
	dropped uint64
}

// Dropped returns notices discarded because the bounded queue was full
func (d RuntimeNoticeDiagnostics) Dropped() uint64 { return d.dropped }

type runtimeNoticeQueue struct {
	mutex    sync.Mutex
	capacity int
	items    []RuntimeNotice
	dropped  uint64
}

func newRuntimeNoticeQueue(capacity int) *runtimeNoticeQueue {
	return &runtimeNoticeQueue{capacity: capacity, items: make([]RuntimeNotice, 0, min(capacity, 16))}
}

func (q *runtimeNoticeQueue) push(notice RuntimeNotice) {
	if q == nil {
		return
	}
	q.mutex.Lock()
	defer q.mutex.Unlock()
	if len(q.items) >= q.capacity {
		q.dropped = saturatingAdd64(q.dropped, 1)
		return
	}
	q.items = append(q.items, notice)
}

func (q *runtimeNoticeQueue) drain() []RuntimeNotice {
	if q == nil {
		return nil
	}
	q.mutex.Lock()
	defer q.mutex.Unlock()
	notices := append([]RuntimeNotice(nil), q.items...)
	clear(q.items)
	q.items = q.items[:0]
	return notices
}

func (q *runtimeNoticeQueue) pending() int {
	if q == nil {
		return 0
	}
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return len(q.items)
}

func (q *runtimeNoticeQueue) diagnostics() RuntimeNoticeDiagnostics {
	if q == nil {
		return RuntimeNoticeDiagnostics{}
	}
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return RuntimeNoticeDiagnostics{dropped: q.dropped}
}

func effectRuntimeNotice(kind RuntimeNoticeKind, key TaskKey, generation uint64, keyed bool) RuntimeNotice {
	return RuntimeNotice{kind: kind, taskKey: key, generation: generation, hasTask: keyed}
}

func subscriptionRuntimeNotice(kind RuntimeNoticeKind, key SubscriptionKey, generation uint64) RuntimeNotice {
	return RuntimeNotice{
		kind:            kind,
		subscriptionKey: key,
		generation:      generation,
		hasSubscription: true,
	}
}
