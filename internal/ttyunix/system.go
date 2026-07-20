//go:build linux || darwin

package ttyunix

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

type unixBackend struct{}

type windowSize struct {
	rows, columns    uint16
	xPixels, yPixels uint16
}

type signalResizeWatcher struct {
	channel  chan os.Signal
	stop     chan struct{}
	finished chan struct{}
	pending  atomic.Bool
	once     sync.Once
}

type waitResult struct {
	input bool
	wake  bool
}

func (unixBackend) startResizeWatcher(notify func()) (resizeWatcher, error) {
	watcher := &signalResizeWatcher{
		channel:  make(chan os.Signal, 1),
		stop:     make(chan struct{}),
		finished: make(chan struct{}),
	}
	signal.Notify(watcher.channel, syscall.SIGWINCH)
	go func() {
		defer close(watcher.finished)
		for {
			select {
			case <-watcher.channel:
				watcher.pending.Store(true)
				notify()
			case <-watcher.stop:
				return
			}
		}
	}()
	return watcher, nil
}

func (watcher *signalResizeWatcher) changed() bool {
	return watcher.pending.Swap(false)
}

func (watcher *signalResizeWatcher) close() {
	watcher.once.Do(func() {
		signal.Stop(watcher.channel)
		close(watcher.stop)
		<-watcher.finished
	})
}

func (unixBackend) read(fd int, buffer []byte) (int, error) {
	return syscall.Read(fd, buffer)
}

func (unixBackend) write(fd int, buffer []byte) (int, error) {
	return syscall.Write(fd, buffer)
}

func (unixBackend) size(fd int) (columns, rows uint16, err error) {
	size := windowSize{}
	_, _, callErr := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(&size)),
	)
	if callErr != 0 {
		return 0, 0, callErr
	}
	if size.columns == 0 || size.rows == 0 {
		return 0, 0, errors.New("terminal reported a zero-sized window")
	}
	return size.columns, size.rows, nil
}

func selectTimeout(timeout time.Duration, hasTimeout bool) *syscall.Timeval {
	if !hasTimeout {
		return nil
	}
	timeval := syscall.NsecToTimeval(timeout.Nanoseconds())
	return &timeval
}

func validateSelectFD(fd int, capacity int) error {
	if fd < 0 || fd >= capacity {
		return fmt.Errorf("terminal descriptor %d exceeds select capacity %d", fd, capacity)
	}
	return nil
}
