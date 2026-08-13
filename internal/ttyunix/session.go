//go:build linux || darwin

package ttyunix

import (
	"context"
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
	// InlineHeight selects a main-screen viewport when positive
	InlineHeight uint16
	// CursorQueryTimeout bounds inline cursor-position discovery
	CursorQueryTimeout time.Duration
}

const maxCursorQueryInputBytes = 65_536

type inlineRegion struct {
	columns       uint16
	terminalRows  uint16
	originY       uint16
	height        uint16
	cursorOffsetY uint16
}

type inlinePlacement struct {
	originY          uint16
	height           uint16
	linesAfterCursor uint16
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

// Session owns raw mode, terminal I/O, resize signaling, and configured
// viewport lifecycle for one process terminal.
type Session struct {
	backend          terminalBackend
	inputFD          int
	outputFD         int
	originalState    terminalState
	hasOriginalState bool
	watcher          resizeWatcher
	wake             *wakePipe
	mouseTracking    vt.MouseTracking
	hasMouseTracking bool
	inlineHeight     uint16
	cursorQueryLimit time.Duration
	inlineRegion     *inlineRegion
	rawModeActive    bool
	lifecycleStarted bool
	initialResize    bool
	ownsProcess      bool
	pendingInput     []byte
	pendingOffset    int
	outputBuffer     []byte
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
		rawModeActive:    true,
		inlineHeight:     options.InlineHeight,
		cursorQueryLimit: options.CursorQueryTimeout,
		initialResize:    true,
	}
	if options.MouseTracking != nil {
		session.mouseTracking = *options.MouseTracking
		session.hasMouseTracking = true
	}
	if err := session.activateLifecycle(); err != nil {
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
	if s.pendingOffset < len(s.pendingInput) {
		read := copy(buffer, s.pendingInput[s.pendingOffset:])
		s.pendingOffset += read
		if s.pendingOffset == len(s.pendingInput) {
			s.pendingInput = s.pendingInput[:0]
			s.pendingOffset = 0
		}
		return read, nil
	}
	return s.readBackend(buffer)
}

func (s *Session) readBackend(buffer []byte) (int, error) {
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
	s.outputBuffer = vt.AppendEncoded(s.outputBuffer[:0], operations, capabilities)
	return s.Write(s.outputBuffer)
}

// WriteViewportOperations writes one rendered frame in viewport-local
// coordinates and parks a hidden cursor at the viewport origin
func (s *Session) WriteViewportOperations(
	operations []vt.TerminalOp,
	cursorY uint32,
	hasCursor bool,
	capabilities vt.Capabilities,
) error {
	originY := uint32(0)
	if s.inlineRegion != nil {
		originY = uint32(s.inlineRegion.originY)
	}
	s.outputBuffer = vt.AppendEncodedAt(s.outputBuffer[:0], operations, capabilities, 0, originY)
	if s.inlineRegion != nil && !hasCursor {
		s.outputBuffer = vt.AppendEncodedAt(
			s.outputBuffer,
			[]vt.TerminalOp{vt.MoveTo(0, 0)},
			capabilities,
			0,
			originY,
		)
	}
	if err := s.Write(s.outputBuffer); err != nil {
		return err
	}
	if s.inlineRegion != nil {
		offset := uint16(0)
		if hasCursor {
			if cursorY > uint32(^uint16(0)) {
				offset = ^uint16(0)
			} else {
				offset = uint16(cursorY)
			}
		}
		s.inlineRegion.cursorOffsetY = min(offset, s.inlineRegion.height-1)
	}
	return nil
}

// Wait blocks until terminal input, a runtime notification, or an optional
// deadline is ready. It reports whether terminal input can be read.
func (s *Session) Wait(timeout time.Duration, hasTimeout bool) (bool, error) {
	if s.pendingOffset < len(s.pendingInput) {
		return true, nil
	}
	return s.waitBackend(timeout, hasTimeout)
}

func (s *Session) waitBackend(timeout time.Duration, hasTimeout bool) (bool, error) {
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

// ViewportSize returns the local layout size owned by the terminal runner
func (s *Session) ViewportSize() (columns, rows uint16, err error) {
	if s.inlineRegion != nil {
		return s.inlineRegion.columns, s.inlineRegion.height, nil
	}
	return s.Size()
}

// RefreshViewport recomputes inline placement after a terminal resize
func (s *Session) RefreshViewport() (columns, rows uint16, err error) {
	if s.inlineHeight == 0 {
		return s.Size()
	}
	cursorOffset := uint16(0)
	if s.inlineRegion != nil {
		cursorOffset = s.inlineRegion.cursorOffsetY
	}
	if err := s.establishInlineRegion(cursorOffset); err != nil {
		return 0, 0, err
	}
	region := *s.inlineRegion
	if err := s.WriteOperations([]vt.TerminalOp{
		vt.MoveTo(0, uint32(region.originY)),
		vt.EraseDisplay(vt.EraseAfter),
	}, vt.BaselineCapabilities()); err != nil {
		return 0, 0, err
	}
	return region.columns, region.height, nil
}

// LocalizeEvent converts terminal mouse coordinates into viewport coordinates
// and rejects mouse events outside an inline viewport
func (s *Session) LocalizeEvent(event vt.Event) (vt.Event, bool) {
	if s.inlineRegion == nil || event.Kind != vt.EventMouse {
		return event, true
	}
	region := s.inlineRegion
	originY := uint32(region.originY)
	endY := originY + uint32(region.height)
	if event.Mouse.X >= uint32(region.columns) || event.Mouse.Y < originY || event.Mouse.Y >= endY {
		return vt.Event{}, false
	}
	event.Mouse.Y -= originY
	return event, true
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

// Suspend restores the original terminal mode and leaves the configured
// viewport while retaining resize signaling and enough state to Resume
func (s *Session) Suspend() error {
	return s.deactivate("suspend terminal mode")
}

// Resume re-enters raw mode and the configured terminal viewport
func (s *Session) Resume() error {
	if s.lifecycleStarted && s.rawModeActive {
		return nil
	}
	if !s.hasOriginalState {
		return errors.New("resume terminal session: terminal session is closed")
	}
	if !s.rawModeActive {
		rawState := s.originalState
		makeRaw(&rawState)
		if err := s.backend.setState(s.inputFD, &rawState); err != nil {
			return fmt.Errorf("resume terminal raw mode: %w", err)
		}
		s.rawModeActive = true
	}

	if err := s.activateLifecycle(); err != nil {
		_ = s.deactivate("restore terminal mode")
		return err
	}
	return nil
}

func (s *Session) activateLifecycle() error {
	s.lifecycleStarted = true
	operations := make([]vt.TerminalOp, 0, 6)
	if s.inlineHeight == 0 {
		operations = append(operations, vt.EnterAlternateScreen())
	}
	operations = append(operations, vt.HideCursor(), vt.EnableBracketedPaste())
	if s.hasMouseTracking {
		operations = append(operations, vt.EnableMouse(s.mouseTracking))
	}
	operations = append(operations, vt.EnableFocus())
	if err := s.WriteOperations(operations, vt.BaselineCapabilities()); err != nil {
		return err
	}
	if s.inlineHeight != 0 {
		if err := s.establishInlineRegion(0); err != nil {
			return err
		}
	}
	return nil
}

func (s *Session) establishInlineRegion(cursorOffset uint16) error {
	columns, terminalRows, err := s.Size()
	if err != nil {
		return err
	}
	_, cursorY, err := s.queryCursorPosition()
	if err != nil {
		return err
	}
	cursorY = min(cursorY, terminalRows-1)
	placement := computeInlinePlacement(terminalRows, s.inlineHeight, cursorY, cursorOffset)

	s.outputBuffer = vt.AppendEncoded(
		s.outputBuffer[:0],
		[]vt.TerminalOp{vt.MoveTo(0, uint32(cursorY))},
		vt.BaselineCapabilities(),
	)
	for range placement.linesAfterCursor {
		s.outputBuffer = vt.AppendEncoded(
			s.outputBuffer,
			[]vt.TerminalOp{vt.NextLine()},
			vt.BaselineCapabilities(),
		)
	}
	if err := s.Write(s.outputBuffer); err != nil {
		return err
	}
	s.inlineRegion = &inlineRegion{
		columns:       columns,
		terminalRows:  terminalRows,
		originY:       placement.originY,
		height:        placement.height,
		cursorOffsetY: placement.height - 1,
	}
	return nil
}

func computeInlinePlacement(
	terminalRows, requestedHeight, cursorY, cursorOffset uint16,
) inlinePlacement {
	height := min(requestedHeight, terminalRows)
	cursorOffset = min(cursorOffset, height-1)
	linesAfterCursor := height - cursorOffset - 1
	availableLines := terminalRows - cursorY - 1
	missingLines := uint16(0)
	if linesAfterCursor > availableLines {
		missingLines = linesAfterCursor - availableLines
	}
	originY := cursorY
	if missingLines > originY {
		originY = 0
	} else {
		originY -= missingLines
	}
	if cursorOffset > originY {
		originY = 0
	} else {
		originY -= cursorOffset
	}
	return inlinePlacement{originY, height, linesAfterCursor}
}

func (s *Session) queryCursorPosition() (x, y uint16, err error) {
	s.compactPendingInput()
	if err := s.WriteOperations([]vt.TerminalOp{vt.RequestCursorPosition()}, vt.BaselineCapabilities()); err != nil {
		return 0, 0, err
	}
	started := time.Now()
	var input [8_192]byte
	for {
		if x, y, ok := takeCursorPositionReport(&s.pendingInput); ok {
			return x, y, nil
		}
		remaining := s.cursorQueryLimit - time.Since(started)
		if remaining <= 0 {
			return 0, 0, fmt.Errorf("query terminal cursor position: %w", context.DeadlineExceeded)
		}
		readable, waitErr := s.waitBackend(remaining, true)
		if waitErr != nil {
			return 0, 0, waitErr
		}
		if !readable {
			continue
		}
		read, readErr := s.readBackend(input[:])
		if readErr != nil {
			return 0, 0, readErr
		}
		if read == 0 {
			return 0, 0, fmt.Errorf("query terminal cursor position: %w", io.ErrUnexpectedEOF)
		}
		if len(s.pendingInput)+read > maxCursorQueryInputBytes {
			return 0, 0, errors.New("query terminal cursor position: terminal input exceeded the cursor-query limit")
		}
		s.pendingInput = append(s.pendingInput, input[:read]...)
	}
}

func (s *Session) compactPendingInput() {
	if s.pendingOffset == 0 {
		return
	}
	copy(s.pendingInput, s.pendingInput[s.pendingOffset:])
	s.pendingInput = s.pendingInput[:len(s.pendingInput)-s.pendingOffset]
	s.pendingOffset = 0
}

func (s *Session) deactivate(modeOperation string) error {
	var firstErr error
	if s.lifecycleStarted {
		operations := []vt.TerminalOp{
			vt.DisableMouse(),
			vt.DisableFocus(),
			vt.DisableBracketedPaste(),
			vt.ResetStyle(),
		}
		if s.inlineHeight != 0 {
			operations = s.appendInlineFinishOperations(operations)
			operations = append(operations, vt.ShowCursor())
		} else {
			operations = append(operations, vt.ShowCursor(), vt.LeaveAlternateScreen())
		}
		if err := s.WriteOperations(operations, vt.BaselineCapabilities()); err != nil {
			firstErr = err
		} else {
			s.lifecycleStarted = false
			s.inlineRegion = nil
		}
	}
	if s.rawModeActive && s.hasOriginalState {
		if err := s.backend.setState(s.inputFD, &s.originalState); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", modeOperation, err)
			}
		} else {
			s.rawModeActive = false
		}
	}
	return firstErr
}

func (s *Session) appendInlineFinishOperations(operations []vt.TerminalOp) []vt.TerminalOp {
	if s.inlineRegion == nil {
		return operations
	}
	region := s.inlineRegion
	after := region.originY + region.height
	if after < region.terminalRows {
		return append(operations, vt.MoveTo(0, uint32(after)))
	}
	return append(
		operations,
		vt.MoveTo(0, uint32(region.terminalRows-1)),
		vt.NextLine(),
	)
}

// Close performs best-effort terminal restoration and returns the first error.
// It is safe to call more than once.
func (s *Session) Close() error {
	firstErr := s.deactivate("restore terminal mode")
	s.lifecycleStarted = false
	s.rawModeActive = false
	s.hasOriginalState = false
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

func takeCursorPositionReport(input *[]byte) (x, y uint16, ok bool) {
	buffer := *input
	for start := 0; start+1 < len(buffer); start++ {
		if buffer[start] != '\x1B' || buffer[start+1] != '[' {
			continue
		}
		index := start + 2
		rowStart := index
		for index < len(buffer) && buffer[index] >= '0' && buffer[index] <= '9' {
			index++
		}
		if index == rowStart || index >= len(buffer) || buffer[index] != ';' {
			continue
		}
		row, valid := parseCursorCoordinate(buffer[rowStart:index])
		if !valid {
			continue
		}
		index++
		columnStart := index
		for index < len(buffer) && buffer[index] >= '0' && buffer[index] <= '9' {
			index++
		}
		if index == columnStart || index >= len(buffer) || buffer[index] != 'R' {
			continue
		}
		column, valid := parseCursorCoordinate(buffer[columnStart:index])
		if !valid {
			continue
		}
		copy(buffer[start:], buffer[index+1:])
		*input = buffer[:len(buffer)-(index+1-start)]
		return column - 1, row - 1, true
	}
	return 0, 0, false
}

func parseCursorCoordinate(input []byte) (uint16, bool) {
	value := uint32(0)
	for _, digit := range input {
		next := uint32(digit - '0')
		if value > (uint32(^uint16(0))-next)/10 {
			return 0, false
		}
		value = value*10 + next
		if value > uint32(^uint16(0)) {
			return 0, false
		}
	}
	return uint16(value), value != 0
}
