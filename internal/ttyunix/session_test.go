//go:build linux || darwin

package ttyunix

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go/internal/conformance"
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
	reads       [][]byte
	failWait    bool
	columns     uint16
	rows        uint16
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{
		failWriteAt: -1,
		failSetAt:   -1,
		watcher:     &fakeResizeWatcher{},
		columns:     80,
		rows:        24,
	}
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
	if len(backend.reads) != 0 {
		input := backend.reads[0]
		backend.reads = backend.reads[1:]
		return copy(buffer, input), nil
	}
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
	if backend.failWait {
		return waitResult{}, errors.New("injected wait failure")
	}
	return waitResult{input: true}, nil
}

func (backend *fakeBackend) size(_ int) (uint16, uint16, error) {
	return backend.columns, backend.rows, nil
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

func TestInlinePlacementsMatchSharedFixtures(t *testing.T) {
	records, err := conformance.Load(
		"tui/terminal-viewport.txt",
		"terminal-viewport",
		"terminal", "requested", "cursor", "offset", "expected",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		terminal := fixtureUint16Pair(t, record.Field("terminal"))
		cursor := fixtureUint16Pair(t, record.Field("cursor"))
		requested := fixtureUint16(t, record.Field("requested"))
		offset := fixtureUint16(t, record.Field("offset"))
		placement := computeInlinePlacement(terminal[1], requested, cursor[1], offset)
		got := fmt.Sprintf(
			"origin:%d;height:%d;lines:%d",
			placement.originY,
			placement.height,
			placement.linesAfterCursor,
		)
		if got != record.Field("expected") {
			t.Errorf("case %s: placement = %s, want %s", record.ID, got, record.Field("expected"))
		}
	}
}

func TestInlineSessionPreservesInputTranslatesFramesAndLeavesOutput(t *testing.T) {
	backend := newFakeBackend()
	backend.reads = [][]byte{[]byte("typed\x1B[23;5Rtail")}
	session, err := startSession(backend, 0, 1, Options{
		MouseTracking:      pointerTo(vt.MouseTrackingPress),
		InlineHeight:       4,
		CursorQueryTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	columns, rows, err := session.ViewportSize()
	if err != nil || columns != 80 || rows != 4 {
		t.Fatalf("ViewportSize = %d, %d, %v", columns, rows, err)
	}
	backend.failWait = true
	if ready, err := session.Wait(time.Second, true); err != nil || !ready {
		t.Fatalf("Wait with preserved input = %t, %v", ready, err)
	}
	input := make([]byte, 16)
	read, err := session.Read(input)
	if err != nil || string(input[:read]) != "typedtail" {
		t.Fatalf("Read = %q, %v", input[:read], err)
	}
	frame := []vt.TerminalOp{vt.MoveTo(1, 2), vt.WriteText("view")}
	if err := session.WriteViewportOperations(frame, 2, true, vt.BaselineCapabilities()); err != nil {
		t.Fatal(err)
	}
	wantFrame := vt.EncodeAt(frame, vt.BaselineCapabilities(), 0, 20)
	if !bytes.Equal(backend.writes[3], wantFrame) {
		t.Fatalf("frame = %q, want %q", backend.writes[3], wantFrame)
	}
	inside := vt.Event{Kind: vt.EventMouse, Mouse: vt.MouseEvent{X: 3, Y: 22}}
	localized, ok := session.LocalizeEvent(inside)
	if !ok || localized.Mouse.Y != 2 {
		t.Fatalf("localized mouse = %#v, %t", localized, ok)
	}
	outside := vt.Event{Kind: vt.EventMouse, Mouse: vt.MouseEvent{X: 3, Y: 19}}
	if _, ok := session.LocalizeEvent(outside); ok {
		t.Fatal("outside mouse was accepted")
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(backend.writes[0], []byte("\x1B[?1049")) {
		t.Fatalf("inline startup entered alternate screen: %q", backend.writes[0])
	}
	if string(backend.writes[1]) != "\x1B[6n" {
		t.Fatalf("query = %q", backend.writes[1])
	}
	if !bytes.HasSuffix(backend.writes[len(backend.writes)-1], []byte("\x1B[24;1H\x1BE\x1B[?25h")) {
		t.Fatalf("finish = %q", backend.writes[len(backend.writes)-1])
	}
}

func TestInlineHiddenCursorIsParkedAtViewportOrigin(t *testing.T) {
	backend := newFakeBackend()
	backend.reads = [][]byte{[]byte("\x1B[6;5R")}
	session, err := startSession(backend, 0, 1, Options{
		InlineHeight:       3,
		CursorQueryTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	operations := []vt.TerminalOp{vt.MoveTo(2, 1), vt.WriteText("view")}

	if err := session.WriteViewportOperations(
		operations,
		0,
		false,
		vt.BaselineCapabilities(),
	); err != nil {
		t.Fatal(err)
	}

	want := vt.EncodeAt(operations, vt.BaselineCapabilities(), 0, 5)
	want = vt.AppendEncodedAt(
		want,
		[]vt.TerminalOp{vt.MoveTo(0, 0)},
		vt.BaselineCapabilities(),
		0,
		5,
	)
	if !bytes.Equal(backend.writes[3], want) {
		t.Fatalf("frame = %q, want %q", backend.writes[3], want)
	}
	if session.inlineRegion.cursorOffsetY != 0 {
		t.Fatalf("cursor offset = %d, want 0", session.inlineRegion.cursorOffsetY)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestInlineSuspendFinalizesThenResumeReservesFreshRegion(t *testing.T) {
	backend := newFakeBackend()
	backend.reads = [][]byte{[]byte("\x1B[6;1R"), []byte("\x1B[12;1R")}
	session, err := startSession(backend, 0, 1, Options{
		InlineHeight:       3,
		CursorQueryTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if session.inlineRegion.originY != 5 {
		t.Fatalf("initial origin = %d, want 5", session.inlineRegion.originY)
	}

	if err := session.Suspend(); err != nil {
		t.Fatal(err)
	}
	if session.inlineRegion != nil {
		t.Fatal("Suspend retained the finalized inline region")
	}
	if err := session.Resume(); err != nil {
		t.Fatal(err)
	}
	if session.inlineRegion.originY != 11 {
		t.Fatalf("resumed origin = %d, want 11", session.inlineRegion.originY)
	}

	if !bytes.HasSuffix(backend.writes[3], []byte("\x1B[9;1H\x1B[?25h")) {
		t.Fatalf("suspend output = %q", backend.writes[3])
	}
	if string(backend.writes[5]) != "\x1B[6n" {
		t.Fatalf("resume query = %q", backend.writes[5])
	}
	for _, write := range backend.writes {
		if bytes.Contains(write, []byte("\x1B[?1049")) {
			t.Fatalf("inline lifecycle entered alternate screen: %q", write)
		}
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestInlineCursorQueryTimeoutRestoresTerminal(t *testing.T) {
	backend := newFakeBackend()
	_, err := startSession(backend, 0, 1, Options{
		InlineHeight:       3,
		CursorQueryTimeout: 0,
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("startSession error = %v, want context deadline", err)
	}
	if got := backend.calls[len(backend.calls)-1]; got != "set:0:7" {
		t.Fatalf("last backend call = %q, want original mode", got)
	}
	if !backend.watcher.closed {
		t.Fatal("resize watcher was not closed")
	}
}

func TestInlineCursorQueryRejectsMoreThanInputLimit(t *testing.T) {
	backend := newFakeBackend()
	for range 9 {
		backend.reads = append(backend.reads, bytes.Repeat([]byte{'x'}, 8_192))
	}
	_, err := startSession(backend, 0, 1, Options{
		InlineHeight:       3,
		CursorQueryTimeout: time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "cursor-query limit") {
		t.Fatalf("startSession error = %v, want cursor-query limit", err)
	}
	if got := backend.calls[len(backend.calls)-1]; got != "set:0:7" {
		t.Fatalf("last backend call = %q, want original mode", got)
	}
	if !backend.watcher.closed {
		t.Fatal("resize watcher was not closed")
	}
}

func TestCursorReportParserSkipsInvalidAndPreservesOtherInput(t *testing.T) {
	input := []byte("a\x1B[999999;1Rb\x1B[3;4Rc")
	x, y, ok := takeCursorPositionReport(&input)
	if !ok || x != 3 || y != 2 {
		t.Fatalf("position = %d, %d, %t", x, y, ok)
	}
	if string(input) != "a\x1B[999999;1Rbc" {
		t.Fatalf("remaining input = %q", input)
	}
}

func TestInlineResizePreservesLogicalCursorOffset(t *testing.T) {
	backend := newFakeBackend()
	backend.reads = [][]byte{[]byte("\x1B[6;5R")}
	session, err := startSession(backend, 0, 1, Options{
		InlineHeight:       5,
		CursorQueryTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.WriteViewportOperations(
		[]vt.TerminalOp{vt.MoveTo(0, 2)},
		2,
		true,
		vt.BaselineCapabilities(),
	); err != nil {
		t.Fatal(err)
	}
	backend.columns = 100
	backend.rows = 30
	backend.reads = append(backend.reads, []byte("\x1B[19;13R"))

	columns, rows, err := session.RefreshViewport()
	if err != nil || columns != 100 || rows != 5 {
		t.Fatalf("RefreshViewport = %d, %d, %v", columns, rows, err)
	}
	if session.inlineRegion.originY != 16 {
		t.Fatalf("originY = %d, want 16", session.inlineRegion.originY)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}

func pointerTo[T any](value T) *T {
	return &value
}

func fixtureUint16Pair(t *testing.T, value string) [2]uint16 {
	t.Helper()
	fields := strings.Split(value, ",")
	if len(fields) != 2 {
		t.Fatalf("invalid pair %q", value)
	}
	return [2]uint16{fixtureUint16(t, fields[0]), fixtureUint16(t, fields[1])}
}

func fixtureUint16(t *testing.T, value string) uint16 {
	t.Helper()
	parsed, err := strconv.ParseUint(value, 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	return uint16(parsed)
}

type zeroWriteBackend struct{ *fakeBackend }

func (zeroWriteBackend) write(_ int, _ []byte) (int, error) { return 0, nil }
