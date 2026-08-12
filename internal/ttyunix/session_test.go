//go:build linux || darwin

package ttyunix

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mayahiro/nagi-go/vt"
)

type fakeResizeWatcher struct {
	pending bool
	closed  bool
}

func (watcher *fakeResizeWatcher) changed() bool {
	pending := watcher.pending
	watcher.pending = false
	return pending
}

func (watcher *fakeResizeWatcher) close() {
	watcher.closed = true
}

type fakeBackend struct {
	calls       []string
	writes      [][]byte
	writeCount  int
	failWriteAt int
	setCount    int
	failSetAt   int
	watcher     *fakeResizeWatcher
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{failWriteAt: -1, failSetAt: -1, watcher: &fakeResizeWatcher{}}
}

func (backend *fakeBackend) getState(fd int) (terminalState, error) {
	backend.calls = append(backend.calls, "get:"+string(rune('0'+fd)))
	state := terminalState{}
	state.Lflag = 7
	return state, nil
}

func (backend *fakeBackend) setState(fd int, state *terminalState) error {
	backend.calls = append(backend.calls, fmtStateCall(fd, state.Lflag))
	call := backend.setCount
	backend.setCount++
	if backend.failSetAt == call {
		return errors.New("injected set-state failure")
	}
	return nil
}

func fmtStateCall(fd int, flags any) string {
	return fmt.Sprintf("set:%d:%d", fd, flags)
}

func (backend *fakeBackend) startResizeWatcher(_ func()) (resizeWatcher, error) {
	backend.calls = append(backend.calls, "signal:on")
	return backend.watcher, nil
}

func (backend *fakeBackend) read(_ int, buffer []byte) (int, error) {
	return copy(buffer, []byte("input")), nil
}

func (backend *fakeBackend) write(_ int, buffer []byte) (int, error) {
	call := backend.writeCount
	backend.writeCount++
	if backend.failWriteAt == call {
		return 0, errors.New("injected write failure")
	}
	backend.writes = append(backend.writes, append([]byte(nil), buffer...))
	return len(buffer), nil
}

func (backend *fakeBackend) wait(_, _ int, _ time.Duration, _ bool) (waitResult, error) {
	return waitResult{input: true}, nil
}

func (backend *fakeBackend) size(_ int) (uint16, uint16, error) {
	return 80, 24, nil
}

func TestNormalCloseRestoresTerminalState(t *testing.T) {
	backend := newFakeBackend()
	session, err := startSession(backend, 0, 1, Options{})
	if err != nil {
		t.Fatal(err)
	}
	wantStart := vt.Encode([]vt.TerminalOp{
		vt.EnterAlternateScreen(),
		vt.HideCursor(),
		vt.EnableBracketedPaste(),
		vt.EnableFocus(),
	}, vt.BaselineCapabilities())
	if len(backend.writes) != 1 || !bytes.Equal(backend.writes[0], wantStart) {
		t.Fatalf("startup write = %q, want %q", backend.writes, wantStart)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	if !backend.watcher.closed {
		t.Fatal("resize watcher was not closed")
	}
	if len(backend.writes) != 2 {
		t.Fatalf("write count = %d, want 2", len(backend.writes))
	}
	if got := backend.calls[len(backend.calls)-1]; got != "set:0:7" {
		t.Fatalf("last backend call = %q, want restore", got)
	}
}

func TestErrorExitRestoresTerminalState(t *testing.T) {
	backend := newFakeBackend()
	session, err := startSession(backend, 0, 1, Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("application failure")
	if got := runSession(session, func(*Session) error { return want }); !errors.Is(got, want) {
		t.Fatalf("Run error = %v, want %v", got, want)
	}
	if !backend.watcher.closed {
		t.Fatal("resize watcher was not closed")
	}
}

func TestPanicExitRestoresTerminalState(t *testing.T) {
	backend := newFakeBackend()
	session, err := startSession(backend, 0, 1, Options{})
	if err != nil {
		t.Fatal(err)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("panic was not propagated")
			}
		}()
		_ = runSession(session, func(*Session) error { panic("injected panic") })
	}()
	if !backend.watcher.closed {
		t.Fatal("resize watcher was not closed")
	}
}

func TestLifecycleWriteFailureStillRestoresTerminalState(t *testing.T) {
	backend := newFakeBackend()
	backend.failWriteAt = 0
	if _, err := startSession(backend, 0, 1, Options{}); err == nil {
		t.Fatal("startSession unexpectedly succeeded")
	}
	if !backend.watcher.closed {
		t.Fatal("resize watcher was not closed")
	}
	if got := backend.calls[len(backend.calls)-1]; got != "set:0:7" {
		t.Fatalf("last backend call = %q, want restore", got)
	}
}

func TestIOResizeAndSizeAreForwarded(t *testing.T) {
	backend := newFakeBackend()
	session, err := startSession(backend, 0, 1, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	buffer := make([]byte, 8)
	if read, err := session.Read(buffer); err != nil || read != 5 || string(buffer[:read]) != "input" {
		t.Fatalf("Read = %d, %q, %v", read, buffer[:read], err)
	}
	if ready, err := session.Wait(0, true); err != nil || !ready {
		t.Fatalf("Wait = %t, %v", ready, err)
	}
	if columns, rows, err := session.Size(); err != nil || columns != 80 || rows != 24 {
		t.Fatalf("Size = %d, %d, %v", columns, rows, err)
	}
	if !session.TakeResize() || session.TakeResize() {
		t.Fatal("initial resize notification was not one-shot")
	}
	backend.watcher.pending = true
	if !session.TakeResize() || session.TakeResize() {
		t.Fatal("signal resize notification was not one-shot")
	}
}

func TestConfiguredMouseTrackingIsEnabledAndRestored(t *testing.T) {
	backend := newFakeBackend()
	tracking := vt.MouseTrackingPress
	session, err := startSession(backend, 0, 1, Options{MouseTracking: &tracking})
	if err != nil {
		t.Fatal(err)
	}
	wantStart := vt.Encode([]vt.TerminalOp{
		vt.EnterAlternateScreen(),
		vt.HideCursor(),
		vt.EnableBracketedPaste(),
		vt.EnableMouse(vt.MouseTrackingPress),
		vt.EnableFocus(),
	}, vt.BaselineCapabilities())
	if len(backend.writes) != 1 || !bytes.Equal(backend.writes[0], wantStart) {
		t.Fatalf("startup write = %q, want %q", backend.writes, wantStart)
	}

	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	wantRestore := vt.Encode([]vt.TerminalOp{
		vt.DisableMouse(),
		vt.DisableFocus(),
		vt.DisableBracketedPaste(),
		vt.ResetStyle(),
		vt.ShowCursor(),
		vt.LeaveAlternateScreen(),
	}, vt.BaselineCapabilities())
	if len(backend.writes) != 2 || !bytes.Equal(backend.writes[1], wantRestore) {
		t.Fatalf("restoration write = %q, want %q", backend.writes, wantRestore)
	}
}

func TestSuspendAndResumeRestoreConfiguredTerminalLifecycle(t *testing.T) {
	backend := newFakeBackend()
	tracking := vt.MouseTrackingPress
	session, err := startSession(backend, 0, 1, Options{MouseTracking: &tracking})
	if err != nil {
		t.Fatal(err)
	}

	if err := session.Suspend(); err != nil {
		t.Fatal(err)
	}
	writesAfterSuspend := len(backend.writes)
	if err := session.Suspend(); err != nil {
		t.Fatal(err)
	}
	if len(backend.writes) != writesAfterSuspend {
		t.Fatalf("second Suspend wrote %d batches", len(backend.writes)-writesAfterSuspend)
	}

	if err := session.Resume(); err != nil {
		t.Fatal(err)
	}
	writesAfterResume := len(backend.writes)
	if err := session.Resume(); err != nil {
		t.Fatal(err)
	}
	if len(backend.writes) != writesAfterResume {
		t.Fatalf("second Resume wrote %d batches", len(backend.writes)-writesAfterResume)
	}

	wantEnter := vt.Encode([]vt.TerminalOp{
		vt.EnterAlternateScreen(),
		vt.HideCursor(),
		vt.EnableBracketedPaste(),
		vt.EnableMouse(vt.MouseTrackingPress),
		vt.EnableFocus(),
	}, vt.BaselineCapabilities())
	wantLeave := vt.Encode([]vt.TerminalOp{
		vt.DisableMouse(),
		vt.DisableFocus(),
		vt.DisableBracketedPaste(),
		vt.ResetStyle(),
		vt.ShowCursor(),
		vt.LeaveAlternateScreen(),
	}, vt.BaselineCapabilities())
	if len(backend.writes) != 3 ||
		!bytes.Equal(backend.writes[0], wantEnter) ||
		!bytes.Equal(backend.writes[1], wantLeave) ||
		!bytes.Equal(backend.writes[2], wantEnter) {
		t.Fatalf("lifecycle writes = %q", backend.writes)
	}
	if backend.setCount != 3 {
		t.Fatalf("set-state calls = %d, want initial raw, suspend restore, and resumed raw", backend.setCount)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestFailedResumeOutputRollsBackToOriginalMode(t *testing.T) {
	backend := newFakeBackend()
	session, err := startSession(backend, 0, 1, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Suspend(); err != nil {
		t.Fatal(err)
	}
	backend.failWriteAt = 2

	if err := session.Resume(); err == nil || !strings.Contains(err.Error(), "write terminal output") {
		t.Fatalf("Resume error = %v", err)
	}
	if got := backend.calls[len(backend.calls)-1]; got != "set:0:7" {
		t.Fatalf("last backend call = %q, want original mode", got)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCloseRetriesModeRestorationAfterFailedSuspend(t *testing.T) {
	backend := newFakeBackend()
	session, err := startSession(backend, 0, 1, Options{})
	if err != nil {
		t.Fatal(err)
	}
	backend.failSetAt = 1

	if err := session.Suspend(); err == nil || !strings.Contains(err.Error(), "suspend terminal mode") {
		t.Fatalf("Suspend error = %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	if got := backend.calls[len(backend.calls)-1]; got != "set:0:7" {
		t.Fatalf("last backend call = %q, want retried restore", got)
	}
}

func TestCloseRetriesScreenCleanupAfterFailedSuspendWrite(t *testing.T) {
	backend := newFakeBackend()
	session, err := startSession(backend, 0, 1, Options{})
	if err != nil {
		t.Fatal(err)
	}
	backend.failWriteAt = 1

	if err := session.Suspend(); err == nil || !strings.Contains(err.Error(), "write terminal output") {
		t.Fatalf("Suspend error = %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	if len(backend.writes) != 2 {
		t.Fatalf("successful lifecycle writes = %d, want startup and retried cleanup", len(backend.writes))
	}
	if !backend.watcher.closed {
		t.Fatal("resize watcher was not closed")
	}
}

func TestMakeRawMatchesCfmakerawContract(t *testing.T) {
	state := terminalState{}
	state.Iflag = ^state.Iflag
	state.Oflag = ^state.Oflag
	state.Cflag = syscall.PARENB
	state.Lflag = ^state.Lflag
	makeRaw(&state)
	if state.Iflag&(syscall.BRKINT|syscall.ICRNL|syscall.IXON) != 0 {
		t.Fatalf("raw input flags remain set: %#x", state.Iflag)
	}
	if state.Oflag&syscall.OPOST != 0 {
		t.Fatalf("raw output processing remains set: %#x", state.Oflag)
	}
	if state.Lflag&(syscall.ECHO|syscall.ICANON|syscall.ISIG|syscall.IEXTEN) != 0 {
		t.Fatalf("raw local flags remain set: %#x", state.Lflag)
	}
	if state.Cflag&syscall.CSIZE != syscall.CS8 || state.Cflag&syscall.PARENB != 0 {
		t.Fatalf("raw character flags = %#x", state.Cflag)
	}
	if state.Cc[syscall.VMIN] != 1 || state.Cc[syscall.VTIME] != 0 {
		t.Fatalf("raw read thresholds = %d, %d", state.Cc[syscall.VMIN], state.Cc[syscall.VTIME])
	}
}

func TestWriteRejectsZeroProgress(t *testing.T) {
	backend := newFakeBackend()
	session := &Session{backend: zeroWriteBackend{backend}, outputFD: 1}
	if err := session.Write([]byte("x")); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("Write error = %v, want io.ErrShortWrite", err)
	}
}

type zeroWriteBackend struct{ *fakeBackend }

func (zeroWriteBackend) write(_ int, _ []byte) (int, error) { return 0, nil }
