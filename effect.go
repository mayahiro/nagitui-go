package tui

import (
	"context"
	"time"
)

// TaskKey is a stable key for replacement and cancellation of one latest task
type TaskKey string

// NewTaskKey returns a task key from an application-defined stable value
func NewTaskKey(value string) TaskKey {
	return TaskKey(value)
}

// String returns the application-defined key
func (k TaskKey) String() string {
	return string(k)
}

// ScopeID is a stable key grouping tasks for explicit cancellation
type ScopeID string

// NewScopeID returns a scope ID from an application-defined stable value
func NewScopeID(value string) ScopeID {
	return ScopeID(value)
}

// String returns the application-defined key
func (s ScopeID) String() string {
	return string(s)
}

// Task is one goroutine task producing an application message
type Task[Message any] func(context.Context) Message

type effectKind uint8

const (
	effectNone effectKind = iota
	effectExit
	effectFocus
	effectScrollTo
	effectRun
	effectLatest
	effectCancel
	effectScoped
	effectCancelScope
	effectAfter
	effectBatch
	effectSequence
)

// Effect is declarative follow-up work produced by App.Init or App.Update
type Effect[Message any] struct {
	kind          effectKind
	task          Task[Message]
	key           TaskKey
	scope         ScopeID
	delay         time.Duration
	message       Message
	id            NodeID
	offset        ScrollOffset
	effect        *Effect[Message]
	effects       []Effect[Message]
	withoutRedraw bool
}

// NoneEffect returns an effect that performs no work
func NoneEffect[Message any]() Effect[Message] {
	return Effect[Message]{}
}

// ExitEffect requests normal application exit after the final dirty frame is rendered
func ExitEffect[Message any]() Effect[Message] {
	return Effect[Message]{kind: effectExit}
}

// FocusEffect requests focus for a focusable node in the next application view
func FocusEffect[Message any](id NodeID) Effect[Message] {
	return Effect[Message]{kind: effectFocus, id: id}
}

// ScrollToEffect requests a ScrollViewport offset in the next application view
func ScrollToEffect[Message any](id NodeID, offset ScrollOffset) Effect[Message] {
	return Effect[Message]{kind: effectScrollTo, id: id, offset: offset}
}

// RunEffect runs one task on a supervised goroutine
//
// It panics when task is nil.
func RunEffect[Message any](task Task[Message]) Effect[Message] {
	if task == nil {
		panic("nagi-tui: nil effect task")
	}
	return Effect[Message]{kind: effectRun, task: task}
}

// LatestEffect replaces the current task for key and suppresses stale results
//
// It panics when task is nil.
func LatestEffect[Message any](key TaskKey, task Task[Message]) Effect[Message] {
	if task == nil {
		panic("nagi-tui: nil effect task")
	}
	return Effect[Message]{kind: effectLatest, key: key, task: task}
}

// CancelEffect cancels the current latest task for key
func CancelEffect[Message any](key TaskKey) Effect[Message] {
	return Effect[Message]{kind: effectCancel, key: key}
}

// ScopedEffect associates every task and timer in effect with a scope
func ScopedEffect[Message any](scope ScopeID, effect Effect[Message]) Effect[Message] {
	return Effect[Message]{kind: effectScoped, scope: scope, effect: &effect}
}

// CancelScopeEffect cancels current tasks and timers in scope
func CancelScopeEffect[Message any](scope ScopeID) Effect[Message] {
	return Effect[Message]{kind: effectCancelScope, scope: scope}
}

// AfterEffect emits message after delay according to the runtime clock
//
// A negative delay is treated as zero.
func AfterEffect[Message any](delay time.Duration, message Message) Effect[Message] {
	if delay < 0 {
		delay = 0
	}
	return Effect[Message]{kind: effectAfter, delay: delay, message: message}
}

// BatchEffects starts child effects independently and completes when all finish
func BatchEffects[Message any](effects ...Effect[Message]) Effect[Message] {
	return Effect[Message]{kind: effectBatch, effects: append([]Effect[Message](nil), effects...)}
}

// SequenceEffects runs child effects in order
func SequenceEffects[Message any](effects ...Effect[Message]) Effect[Message] {
	return Effect[Message]{kind: effectSequence, effects: append([]Effect[Message](nil), effects...)}
}

// WithoutRedraw declares that the Update returning this effect did not change
// state observed by View
//
// Follow-up work is still scheduled and Subscriptions are still reconciled.
// Synchronous UI commands and an already-dirty runtime still produce a frame.
func (e Effect[Message]) WithoutRedraw() Effect[Message] {
	e.withoutRedraw = true
	return e
}

// IsNone reports whether this effect performs no work
func (e Effect[Message]) IsNone() bool {
	return e.kind == effectNone
}
