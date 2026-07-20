//go:build linux || darwin

package ttyunix

import (
	"syscall"
	"testing"
	"time"
)

func TestWakePipeCoalescesNotifications(t *testing.T) {
	pipe, err := newWakePipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pipe.close()

	for range 1_000 {
		pipe.notify()
	}
	if !pipe.pending.Load() {
		t.Fatal("wake notification was not retained")
	}
	if err := pipe.acknowledge(); err != nil {
		t.Fatal(err)
	}
	if pipe.pending.Load() {
		t.Fatal("wake notification remained pending after acknowledgement")
	}

	pipe.notify()
	ready, err := (unixBackend{}).wait(pipe.readFD, pipe.readFD, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if !ready.wake {
		t.Fatal("wake pipe was not rearmed after acknowledgement")
	}
}

func TestWakePipeIgnoresLateNotificationAfterClose(t *testing.T) {
	pipe, err := newWakePipe()
	if err != nil {
		t.Fatal(err)
	}
	pipe.close()

	replacement, err := newWakePipe()
	if err != nil {
		t.Fatal(err)
	}
	defer replacement.close()
	pipe.notify()

	ready, err := (unixBackend{}).wait(replacement.readFD, replacement.readFD, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if ready.wake {
		t.Fatal("late notification reached a replacement descriptor")
	}
}

func TestWaitWithoutDeadlineReturnsForRuntimeWake(t *testing.T) {
	pipe, err := newWakePipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pipe.close()
	input := []int{-1, -1}
	if err := syscall.Pipe(input); err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(input[0])
	defer syscall.Close(input[1])

	result := make(chan error, 1)
	started := make(chan struct{})
	go func() {
		close(started)
		ready, err := (unixBackend{}).wait(input[0], pipe.readFD, 0, false)
		if err == nil && (!ready.wake || ready.input) {
			err = syscall.EINVAL
		}
		result <- err
	}()
	<-started
	pipe.notify()

	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("wait did not return for runtime wake-up")
	}
}
