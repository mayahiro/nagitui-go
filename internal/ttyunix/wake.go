//go:build linux || darwin

package ttyunix

import (
	"errors"
	"sync"
	"sync/atomic"
	"syscall"
)

type wakePipe struct {
	mu      sync.Mutex
	readFD  int
	writeFD int
	closed  bool
	pending atomic.Bool
}

func newWakePipe() (*wakePipe, error) {
	descriptors := []int{-1, -1}
	if err := syscall.Pipe(descriptors); err != nil {
		return nil, err
	}
	for _, descriptor := range descriptors {
		syscall.CloseOnExec(descriptor)
		if err := syscall.SetNonblock(descriptor, true); err != nil {
			_ = syscall.Close(descriptors[0])
			_ = syscall.Close(descriptors[1])
			return nil, err
		}
	}
	return &wakePipe{readFD: descriptors[0], writeFD: descriptors[1]}, nil
}

func (pipe *wakePipe) notify() {
	if pipe == nil || pipe.pending.Load() {
		return
	}
	pipe.mu.Lock()
	defer pipe.mu.Unlock()
	if pipe.closed || pipe.pending.Swap(true) {
		return
	}
	byte := [1]byte{1}
	for {
		_, err := syscall.Write(pipe.writeFD, byte[:])
		switch {
		case err == nil:
			return
		case errors.Is(err, syscall.EINTR):
			continue
		case errors.Is(err, syscall.EAGAIN):
			return
		default:
			pipe.pending.Store(false)
			return
		}
	}
}

func (pipe *wakePipe) acknowledge() error {
	pipe.mu.Lock()
	defer pipe.mu.Unlock()
	if pipe.closed {
		return nil
	}
	var buffer [64]byte
	for {
		_, err := syscall.Read(pipe.readFD, buffer[:])
		switch {
		case err == nil:
			continue
		case errors.Is(err, syscall.EINTR):
			continue
		case errors.Is(err, syscall.EAGAIN):
			// Work notified before this boundary is observed by the runtime pass
			// immediately after Wait returns. Later work writes a new byte.
			pipe.pending.Store(false)
			return nil
		default:
			pipe.pending.Store(false)
			return err
		}
	}
}

func (pipe *wakePipe) close() {
	if pipe == nil {
		return
	}
	pipe.mu.Lock()
	defer pipe.mu.Unlock()
	if pipe.closed {
		return
	}
	pipe.closed = true
	pipe.pending.Store(false)
	_ = syscall.Close(pipe.readFD)
	_ = syscall.Close(pipe.writeFD)
}
