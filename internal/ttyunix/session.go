//go:build linux || darwin

package ttyunix

import (
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/mayahiro/nagi-go/vt"
)

// ErrSessionActive indicates that this process already owns a terminal
// session. Nested sessions are intentionally unsupported.
var ErrSessionActive = errors.New("a Nagi TUI terminal session is already active")

var sessionActive atomic.Bool

// Options configures terminal modes owned by a Session
type Options struct {
	// MouseTracking enables one SGR mouse tracking policy when non-nil
	MouseTracking *vt.MouseTracking
}

type resizeWatcher interface {
	changed() bool
	close()
}

type terminalBackend interface {
	getState(fd int) (terminalState, error)
	setState(fd int, state *terminalState) error
	startResizeWatcher(notify func()) (resizeWatcher, error)
	read(fd int, buffer []byte) (int, error)
	write(fd int, buffer []byte) (int, error)
	wait(inputFD, wakeFD int, timeout time.Duration, hasTimeout bool) (waitResult, error)
	size(fd int) (columns, rows uint16, err error)
}

// Session owns raw mode, terminal I/O, resize signaling, and alternate-screen
// lifecycle for one process terminal.
type Session struct {
	backend          terminalBackend
	inputFD          int
	outputFD         int
	originalState    terminalState
	hasOriginalState bool
	watcher          resizeWatcher
	wake             *wakePipe
	lifecycleStarted bool
	initialResize    bool
	ownsProcess      bool
}

// Open validates stdin and stdout, enables raw mode, and enters the alternate
// screen.
func Open(options Options) (*Session, error) {
	if !sessionActive.CompareAndSwap(false, true) {
		return nil, ErrSessionActive
	}
	session, err := startSession(unixBackend{}, 0, 1, options)
	if err != nil {
		sessionActive.Store(false)
		return nil, err
	}
	session.ownsProcess = true
	return session, nil
}

func startSession(backend terminalBackend, inputFD, outputFD int, options Options) (*Session, error) {
	originalState, err := backend.getState(inputFD)
	if err != nil {
		return nil, fmt.Errorf("validate terminal input: %w", err)
	}
	if _, err := backend.getState(outputFD); err != nil {
		return nil, fmt.Errorf("validate terminal output: %w", err)
	}

	rawState := originalState
	makeRaw(&rawState)
	if err := backend.setState(inputFD, &rawState); err != nil {
		return nil, fmt.Errorf("enable terminal raw mode: %w", err)
	}

	wake, err := newWakePipe()
	if err != nil {
		_ = backend.setState(inputFD, &originalState)
		return nil, fmt.Errorf("create runtime wake pipe: %w", err)
	}

	watcher, err := backend.startResizeWatcher(wake.notify)
	if err != nil {
		wake.close()
		_ = backend.setState(inputFD, &originalState)
		return nil, fmt.Errorf("install SIGWINCH watcher: %w", err)
	}

	session := &Session{
		backend:          backend,
		inputFD:          inputFD,
		outputFD:         outputFD,
		originalState:    originalState,
		hasOriginalState: true,
		watcher:          watcher,
		wake:             wake,
		lifecycleStarted: true,
		initialResize:    true,
	}
	operations := []vt.TerminalOp{
		vt.EnterAlternateScreen(),
		vt.HideCursor(),
		vt.EnableBracketedPaste(),
	}
	if options.MouseTracking != nil {
		operations = append(operations, vt.EnableMouse(*options.MouseTracking))
	}
	operations = append(operations, vt.EnableFocus())
	if err := session.Write(vt.Encode(operations, vt.BaselineCapabilities())); err != nil {
		_ = session.Close()
		return nil, err
	}
	return session, nil
}

func makeRaw(state *terminalState) {
	state.Iflag &^= syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.ISTRIP |
		syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON
	state.Oflag &^= syscall.OPOST
	state.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	state.Cflag &^= syscall.CSIZE | syscall.PARENB
	state.Cflag |= syscall.CS8
	state.Cc[syscall.VMIN] = 1
	state.Cc[syscall.VTIME] = 0
}

// Run opens a session, invokes function, and restores the terminal on normal,
// error, and panic exits.
func Run(options Options, function func(*Session) error) (err error) {
	session, err := Open(options)
	if err != nil {
		return err
	}
	return runSession(session, function)
}

func runSession(session *Session, function func(*Session) error) (err error) {
	defer func() {
		if closeErr := session.Close(); err == nil {
			err = closeErr
		}
	}()
	return function(session)
}

// Read reads terminal input, retrying interrupted system calls.
func (s *Session) Read(buffer []byte) (int, error) {
	for {
		read, err := s.backend.read(s.inputFD, buffer)
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil {
			return 0, fmt.Errorf("read terminal input: %w", err)
		}
		return read, nil
	}
}

// Write writes all terminal output bytes, retrying interrupted and partial
// writes.
func (s *Session) Write(buffer []byte) error {
	for len(buffer) > 0 {
		written, err := s.backend.write(s.outputFD, buffer)
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil {
			return fmt.Errorf("write terminal output: %w", err)
		}
		if written <= 0 || written > len(buffer) {
			return fmt.Errorf("write terminal output: %w", io.ErrShortWrite)
		}
		buffer = buffer[written:]
	}
	return nil
}

// WriteOperations encodes and writes typed terminal operations.
func (s *Session) WriteOperations(operations []vt.TerminalOp, capabilities vt.Capabilities) error {
	return s.Write(vt.Encode(operations, capabilities))
}

// Wait blocks until terminal input, a runtime notification, or an optional
// deadline is ready. It reports whether terminal input can be read.
func (s *Session) Wait(timeout time.Duration, hasTimeout bool) (bool, error) {
	ready, err := s.backend.wait(s.inputFD, s.wake.readFD, timeout, hasTimeout)
	if err != nil {
		return false, fmt.Errorf("poll terminal input: %w", err)
	}
	if ready.wake {
		if err := s.wake.acknowledge(); err != nil {
			return false, fmt.Errorf("acknowledge runtime wake-up: %w", err)
		}
	}
	return ready.input, nil
}

// Notify wakes a blocked terminal loop after asynchronous runtime work.
func (s *Session) Notify() {
	if s.wake != nil {
		s.wake.notify()
	}
}

// Size returns terminal columns and rows.
func (s *Session) Size() (columns, rows uint16, err error) {
	columns, rows, err = s.backend.size(s.outputFD)
	if err != nil {
		return 0, 0, fmt.Errorf("read terminal size: %w", err)
	}
	return columns, rows, nil
}

// TakeResize reports and clears a pending resize notification. The first call
// after Open reports true so callers establish an initial layout.
func (s *Session) TakeResize() bool {
	if s.initialResize {
		s.initialResize = false
		return true
	}
	return s.watcher != nil && s.watcher.changed()
}

// Close performs best-effort terminal restoration and returns the first error.
// It is safe to call more than once.
func (s *Session) Close() error {
	var firstErr error
	if s.lifecycleStarted {
		s.lifecycleStarted = false
		operations := []vt.TerminalOp{
			vt.DisableMouse(),
			vt.DisableFocus(),
			vt.DisableBracketedPaste(),
			vt.ResetStyle(),
			vt.ShowCursor(),
			vt.LeaveAlternateScreen(),
		}
		if err := s.Write(vt.Encode(operations, vt.BaselineCapabilities())); err != nil {
			firstErr = err
		}
	}
	if s.hasOriginalState {
		s.hasOriginalState = false
		if err := s.backend.setState(s.inputFD, &s.originalState); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("restore terminal mode: %w", err)
		}
	}
	if s.watcher != nil {
		s.watcher.close()
		s.watcher = nil
	}
	if s.wake != nil {
		s.wake.close()
	}
	if s.ownsProcess {
		s.ownsProcess = false
		sessionActive.Store(false)
	}
	return firstErr
}
