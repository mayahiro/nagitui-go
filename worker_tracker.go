package tui

import (
	"context"
	"sync"
)

type workerTracker struct {
	mutex   sync.Mutex
	running int
	zero    chan struct{}
}

func (t *workerTracker) start() {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.running == 0 {
		t.zero = make(chan struct{})
	}
	t.running++
}

func (t *workerTracker) finish() {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.running == 0 {
		return
	}
	t.running--
	if t.running == 0 {
		close(t.zero)
		t.zero = nil
	}
}

func (t *workerTracker) wait(ctx context.Context) error {
	t.mutex.Lock()
	if t.running == 0 {
		t.mutex.Unlock()
		return nil
	}
	zero := t.zero
	t.mutex.Unlock()

	select {
	case <-zero:
		return nil
	case <-ctx.Done():
		if cause := context.Cause(ctx); cause != nil {
			return cause
		}
		return ctx.Err()
	}
}
