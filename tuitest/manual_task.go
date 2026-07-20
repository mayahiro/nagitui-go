package tuitest

import (
	"context"
	"errors"
	"sync"

	"github.com/mayahiro/nagitui-go"
)

var (
	// ErrManualTaskCompleted indicates that a manual result was already supplied
	ErrManualTaskCompleted = errors.New("manual task already completed")
)

// ManualTask observes and completes one deterministic effect task
type ManualTask[Message any] struct {
	mu        sync.Mutex
	context   context.Context
	started   chan struct{}
	result    chan Message
	completed bool
}

// NewManualTask returns a task and handle whose result is supplied manually
//
// The task intentionally remains alive after cancellation until Complete is
// called, allowing stale-result tests.
func NewManualTask[Message any]() (tui.Task[Message], *ManualTask[Message]) {
	manual := &ManualTask[Message]{
		started: make(chan struct{}),
		result:  make(chan Message, 1),
	}
	task := func(ctx context.Context) Message {
		manual.mu.Lock()
		manual.context = ctx
		close(manual.started)
		manual.mu.Unlock()
		return <-manual.result
	}
	return task, manual
}

// WaitStarted blocks until the runtime starts the task
func (m *ManualTask[Message]) WaitStarted() {
	<-m.started
}

// Started reports whether the runtime started the task
func (m *ManualTask[Message]) Started() bool {
	select {
	case <-m.started:
		return true
	default:
		return false
	}
}

// Cancelled reports whether the runtime requested cooperative cancellation
func (m *ManualTask[Message]) Cancelled() bool {
	if !m.Started() {
		return false
	}
	m.mu.Lock()
	ctx := m.context
	m.mu.Unlock()
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// Complete supplies the task result and releases its worker
func (m *ManualTask[Message]) Complete(message Message) error {
	m.mu.Lock()
	if m.completed {
		m.mu.Unlock()
		return ErrManualTaskCompleted
	}
	m.completed = true
	m.mu.Unlock()
	m.result <- message
	return nil
}
