package tui

import (
	"context"
	"errors"
	"fmt"
	"time"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go/internal/ttyunix"
)

const defaultMinimumFrameInterval = (time.Second + 119) / 120

// ErrZeroInlineViewportHeight reports an invalid inline terminal viewport
var ErrZeroInlineViewportHeight = errors.New("nagi-tui: an inline terminal viewport requires a positive height")

type terminalViewportKind uint8

const (
	terminalViewportFullscreen terminalViewportKind = iota
	terminalViewportInline
)

// TerminalViewport is the screen region owned by the standard terminal runner
//
// Its zero value is a full-screen alternate-screen viewport.
type TerminalViewport struct {
	kind         terminalViewportKind
	inlineHeight uint16
}

// NewInlineTerminalViewport returns a main-screen viewport with a positive
// requested row count
//
// The terminal runner clamps this height to the current terminal height.
func NewInlineTerminalViewport(height uint16) (TerminalViewport, error) {
	if height == 0 {
		return TerminalViewport{}, ErrZeroInlineViewportHeight
	}
	return TerminalViewport{kind: terminalViewportInline, inlineHeight: height}, nil
}

// InlineHeight returns the requested row count for an inline viewport
func (v TerminalViewport) InlineHeight() (uint16, bool) {
	return v.inlineHeight, v.kind == terminalViewportInline
}

// TerminalClipboard selects standard terminal handling for clipboard requests
type TerminalClipboard uint8

const (
	// TerminalClipboardDisabled drops clipboard requests without terminal output
	TerminalClipboardDisabled TerminalClipboard = iota
	// TerminalClipboardOSC52 writes requests through direct, write-only OSC 52
	// without detecting support or adding multiplexer wrapping
	TerminalClipboardOSC52
)

// TerminalOptions contains settings for RunTerminal
type TerminalOptions struct {
	// Capabilities contains optional output encoder capabilities
	Capabilities vt.Capabilities
	// CapabilityDetection controls environment inspection and active input query
	CapabilityDetection TerminalCapabilityDetection
	// CapabilityQueryTimeout is the maximum wait for active capability queries
	// Zero performs an immediate query check.
	CapabilityQueryTimeout time.Duration
	// MouseTracking enables SGR mouse reports when non-nil
	//
	// It is disabled by default so terminal text selection remains available.
	MouseTracking *vt.MouseTracking
	// Clipboard selects terminal clipboard output and defaults to disabled
	Clipboard TerminalClipboard
	// Viewport is the terminal screen region owned by the runner
	Viewport TerminalViewport
	// CursorQueryTimeout is the maximum wait for an inline viewport cursor report
	// Zero performs an immediate query check.
	CursorQueryTimeout time.Duration
	// FocusFirst focuses the first focusable node before the initial frame
	FocusFirst bool
	// EscapeTimeout disambiguates a lone ESC from an escape sequence
	EscapeTimeout time.Duration
	// QueueCapacity is the maximum number of waiting application messages
	QueueCapacity int
	// TaskLimit is the maximum number of effect tasks executing concurrently
	TaskLimit int
	// SubscriptionCapacity is the maximum pending values retained per source
	SubscriptionCapacity int
	// RuntimeNoticeCapacity is the maximum retained asynchronous lifecycle notices
	RuntimeNoticeCapacity int
	// MinimumFrameInterval limits non-urgent rendering; the default is 120 FPS
	// and zero disables the limit
	MinimumFrameInterval time.Duration
	// WidthProfile is the terminal cell-width policy used by the complete view
	//
	// A custom override must return stable widths for this Runtime's lifetime.
	WidthProfile celltext.WidthProfile
}

// DefaultTerminalOptions returns event-driven settings with bounded queues and
// non-urgent rendering limited to 120 FPS
func DefaultTerminalOptions() TerminalOptions {
	return TerminalOptions{
		Capabilities:           vt.BaselineCapabilities(),
		CapabilityQueryTimeout: 100 * time.Millisecond,
		CursorQueryTimeout:     100 * time.Millisecond,
		EscapeTimeout:          25 * time.Millisecond,
		QueueCapacity:          DefaultQueueCapacity,
		TaskLimit:              DefaultTaskLimit,
		SubscriptionCapacity:   DefaultSubscriptionCapacity,
		RuntimeNoticeCapacity:  DefaultRuntimeNoticeCapacity,
		MinimumFrameInterval:   defaultMinimumFrameInterval,
		WidthProfile:           celltext.ModernWidth(),
	}
}

// RunTerminal runs an application in the process terminal until the
// application or mapEvent requests exit, or terminal input reaches EOF
//
// The terminal session restores raw mode and screen state on normal, error,
// and panic exits.
func RunTerminal[Message any](
	app App[Message],
	options TerminalOptions,
	mapEvent func(vt.Event) EventAction[Message],
) error {
	return runTerminalContext(context.Background(), app, options, mapEvent, nil)
}

// RunTerminalWithNoticeHandler runs an application and synchronously observes
// recovered failures and unexpected asynchronous lifecycle transitions
func RunTerminalWithNoticeHandler[Message any](
	app App[Message],
	options TerminalOptions,
	mapEvent func(vt.Event) EventAction[Message],
	handleNotice func(RuntimeNotice),
) error {
	if handleNotice == nil {
		return errors.New("nagi-tui: nil runtime notice handler")
	}
	return runTerminalContext(context.Background(), app, options, mapEvent, handleNotice)
}

// RunTerminalContext runs an application until normal exit, terminal EOF, or
// context cancellation
//
// Context cancellation returns ctx.Err(). The terminal session restores raw
// mode and screen state on every return path.
func RunTerminalContext[Message any](
	ctx context.Context,
	app App[Message],
	options TerminalOptions,
	mapEvent func(vt.Event) EventAction[Message],
) error {
	return runTerminalContext(ctx, app, options, mapEvent, nil)
}

// RunTerminalContextWithNoticeHandler is the context-aware terminal loop with
// synchronous RuntimeNotice observation
func RunTerminalContextWithNoticeHandler[Message any](
	ctx context.Context,
	app App[Message],
	options TerminalOptions,
	mapEvent func(vt.Event) EventAction[Message],
	handleNotice func(RuntimeNotice),
) error {
	if handleNotice == nil {
		return errors.New("nagi-tui: nil runtime notice handler")
	}
	return runTerminalContext(ctx, app, options, mapEvent, handleNotice)
}

func runTerminalContext[Message any](
	ctx context.Context,
	app App[Message],
	options TerminalOptions,
	mapEvent func(vt.Event) EventAction[Message],
	handleNotice func(RuntimeNotice),
) error {
	if ctx == nil {
		return errors.New("nagi-tui: nil terminal context")
	}
	if mapEvent == nil {
		return errors.New("nagi-tui: nil terminal event mapper")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if options.Clipboard > TerminalClipboardOSC52 {
		return fmt.Errorf("nagi-tui: invalid terminal clipboard mode %d", options.Clipboard)
	}
	if options.CapabilityDetection > TerminalCapabilityDetectionEnabled {
		return fmt.Errorf("nagi-tui: invalid terminal capability detection mode %d", options.CapabilityDetection)
	}
	if options.CapabilityQueryTimeout < 0 {
		return errors.New("nagi-tui: terminal capability query timeout must not be negative")
	}
	inlineHeight, _ := options.Viewport.InlineHeight()
	return ttyunix.Run(ttyunix.Options{
		MouseTracking:      options.MouseTracking,
		InlineHeight:       inlineHeight,
		CursorQueryTimeout: options.CursorQueryTimeout,
	}, func(session *ttyunix.Session) error {
		terminalCapabilities, err := detectTerminalCapabilities(session, options)
		if err != nil {
			return err
		}
		outputCapabilities := resolvedOutputCapabilities(options, terminalCapabilities)
		columns, rows, err := session.ViewportSize()
		if err != nil {
			return err
		}
		clock := NewSystemClock()
		config := NewRuntimeConfig(Size{Width: uint32(columns), Height: uint32(rows)})
		config.QueueCapacity = options.QueueCapacity
		config.TaskLimit = options.TaskLimit
		config.SubscriptionCapacity = options.SubscriptionCapacity
		config.RuntimeNoticeCapacity = options.RuntimeNoticeCapacity
		config.MinimumFrameInterval = options.MinimumFrameInterval
		config.WidthProfile = options.WidthProfile
		config.TerminalCapabilities = terminalCapabilities
		runtime, err := newRuntimeWithClockAndWakeContext(ctx, app, config, clock, session.Notify)
		if err != nil {
			return err
		}
		defer runtime.Close()
		stopContextWake := context.AfterFunc(ctx, session.Notify)
		defer stopContextWake()
		focusFirst := options.FocusFirst
		decoder := NewTimedInputDecoder(clock, options.EscapeTimeout)
		decoder.SetKittyKeyboardMode(terminalCapabilities.KeyboardProtocol() == TerminalKeyboardKitty)
		input := make([]byte, 8_192)

		if session.TakeResize() {
			columns, rows, err = session.RefreshViewport()
			if err != nil {
				return err
			}
			runtime.Resize(Size{Width: uint32(columns), Height: uint32(rows)})
		}
		if _, err := runtime.ProcessPending(); err != nil {
			return err
		}
		handleRuntimeNotices(runtime, handleNotice)
		if !runtime.ExitRequested() {
			ran, err := runPendingTerminalTasks(ctx, session, runtime, handleNotice)
			if err != nil {
				return err
			}
			if ran {
				decoder.Reset()
			}
		}
		if focusFirst {
			focusFirst = false
			if _, err := runtime.focusFirst(); err != nil {
				return err
			}
		}
		if err := writeTerminalOutput(session, runtime, outputCapabilities, options.Clipboard); err != nil {
			return err
		}
		if runtime.ExitRequested() {
			return nil
		}

		for {
			timeout, hasTimeout := terminalWaitDuration(decoder, runtime)
			readable, err := session.Wait(timeout, hasTimeout)
			if err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if session.TakeResize() {
				columns, rows, err = session.RefreshViewport()
				if err != nil {
					return err
				}
				runtime.Resize(Size{Width: uint32(columns), Height: uint32(rows)})
			}
			var events []vt.Event
			if readable {
				read, err := session.Read(input)
				if err != nil {
					return err
				}
				if read == 0 {
					break
				}
				events = decoder.Feed(input[:read])
			}
			events = append(events, decoder.Poll()...)

			exit := false
			for _, event := range events {
				event, insideViewport := session.LocalizeEvent(event)
				if !insideViewport {
					continue
				}
				dispatch, err := runtime.DispatchEvent(event)
				if err != nil {
					return err
				}
				if !dispatch.Consumed() {
					action := mapEvent(event)
					switch action.Kind() {
					case EventIgnore:
					case EventMessage:
						message, _ := action.Message()
						if err := runtime.Enqueue(message); err != nil {
							return err
						}
					case EventExit:
						exit = true
					default:
						return fmt.Errorf("nagi-tui: invalid event action %d", action.Kind())
					}
				}
				if _, err := runtime.ProcessQueued(); err != nil {
					return err
				}
				if exit || runtime.ExitRequested() {
					break
				}
				ran, err := runPendingTerminalTasks(ctx, session, runtime, handleNotice)
				if err != nil {
					return err
				}
				if ran {
					decoder.Reset()
					break
				}
			}
			if _, err := runtime.ProcessPending(); err != nil {
				return err
			}
			handleRuntimeNotices(runtime, handleNotice)
			if !exit && !runtime.ExitRequested() {
				ran, err := runPendingTerminalTasks(ctx, session, runtime, handleNotice)
				if err != nil {
					return err
				}
				if ran {
					decoder.Reset()
				}
			}
			if err := writeTerminalOutput(session, runtime, outputCapabilities, options.Clipboard); err != nil {
				return err
			}
			if exit || runtime.ExitRequested() {
				break
			}
		}
		return nil
	})
}

func detectTerminalCapabilities(
	session *ttyunix.Session,
	options TerminalOptions,
) (TerminalCapabilityProfile, error) {
	if options.CapabilityDetection == TerminalCapabilityDetectionDisabled {
		return TerminalCapabilityProfile{}, nil
	}
	profile := processEnvironmentProfile()
	support, err := session.EnableExtendedKeyboard(options.CapabilityQueryTimeout)
	if err != nil {
		return TerminalCapabilityProfile{}, err
	}
	switch support {
	case ttyunix.KeyboardSupportUnsupported:
		return profile.WithExtendedKeyboard(TerminalFeatureUnsupported, TerminalKeyboardLegacy), nil
	case ttyunix.KeyboardSupportSupported:
		return profile.WithExtendedKeyboard(TerminalFeatureSupported, TerminalKeyboardKitty), nil
	default:
		return profile.WithExtendedKeyboard(TerminalFeatureUnknown, TerminalKeyboardLegacy), nil
	}
}

func resolvedOutputCapabilities(
	options TerminalOptions,
	profile TerminalCapabilityProfile,
) vt.Capabilities {
	capabilities := options.Capabilities
	if options.CapabilityDetection == TerminalCapabilityDetectionEnabled {
		detected := vt.ColorIndexed256
		switch profile.ColorLevel() {
		case TerminalColorMonochrome:
			detected = vt.ColorMonochrome
		case TerminalColorANSI16:
			detected = vt.ColorANSI16
		case TerminalColorIndexed256:
			detected = vt.ColorIndexed256
		case TerminalColorTrueColor:
			detected = vt.ColorTrueColor
		}
		capabilities.ColorLevel = boundedVTColorLevel(capabilities.ColorLevel, detected)
	}
	return capabilities
}

func boundedVTColorLevel(configured, detected vt.ColorLevel) vt.ColorLevel {
	rank := func(level vt.ColorLevel) int {
		switch level {
		case vt.ColorMonochrome:
			return 0
		case vt.ColorANSI16:
			return 1
		case vt.ColorTrueColor:
			return 3
		default:
			return 2
		}
	}
	if rank(detected) < rank(configured) {
		return detected
	}
	return configured
}

func runPendingTerminalTasks[Message any](
	ctx context.Context,
	session *ttyunix.Session,
	runtime *Runtime[Message],
	handleNotice func(RuntimeNotice),
) (bool, error) {
	ran := false
	for !runtime.ExitRequested() && runtime.PendingTerminalTasks() > 0 {
		if err := ctx.Err(); err != nil {
			return ran, err
		}
		if err := session.Suspend(); err != nil {
			return ran, err
		}
		if !runtime.RunTerminalTask() {
			if err := session.Resume(); err != nil {
				return ran, err
			}
			return ran, errors.New("nagi-tui: pending terminal task disappeared")
		}
		if err := session.Resume(); err != nil {
			return ran, err
		}
		if err := ctx.Err(); err != nil {
			return true, err
		}

		runtime.InvalidateTerminalSurface()
		columns, rows, err := session.ViewportSize()
		if err != nil {
			return true, err
		}
		runtime.Resize(Size{Width: uint32(columns), Height: uint32(rows)})
		if _, err := runtime.ProcessPending(); err != nil {
			return true, err
		}
		handleRuntimeNotices(runtime, handleNotice)
		ran = true
	}
	return ran, nil
}

func handleRuntimeNotices[Message any](runtime *Runtime[Message], handler func(RuntimeNotice)) {
	if handler == nil {
		return
	}
	for _, notice := range runtime.DrainRuntimeNotices() {
		handler(notice)
	}
}

type terminalDeadlineSource interface {
	TimeUntilDeadline() (time.Duration, bool)
}

type runtimeDeadlineSource interface {
	TimeUntilEffectDeadline() (time.Duration, bool)
	TimeUntilSubscriptionDeadline() (time.Duration, bool)
	TimeUntilFrameDeadline() (time.Duration, bool)
}

func terminalWaitDuration(decoder terminalDeadlineSource, runtime runtimeDeadlineSource) (time.Duration, bool) {
	timeout := time.Duration(0)
	hasTimeout := false
	include := func(deadline time.Duration, ok bool) {
		if !ok {
			return
		}
		deadline = max(deadline, 0)
		if !hasTimeout || deadline < timeout {
			timeout = deadline
			hasTimeout = true
		}
	}
	include(decoder.TimeUntilDeadline())
	include(runtime.TimeUntilEffectDeadline())
	include(runtime.TimeUntilSubscriptionDeadline())
	include(runtime.TimeUntilFrameDeadline())
	return timeout, hasTimeout
}

func writeTerminalOutput[Message any](
	session *ttyunix.Session,
	runtime *Runtime[Message],
	capabilities vt.Capabilities,
	clipboard TerminalClipboard,
) error {
	output, err := pendingTerminalOutput(runtime, clipboard)
	if err != nil {
		return err
	}
	if len(output.operations) == 0 {
		return nil
	}
	if output.hasFrame {
		return session.WriteViewportOperations(
			output.operations,
			output.cursorY,
			output.hasCursor,
			capabilities,
		)
	}
	return session.WriteOperations(output.operations, capabilities)
}

type terminalOutput struct {
	operations []vt.TerminalOp
	hasFrame   bool
	hasCursor  bool
	cursorY    uint32
}

func pendingTerminalOutput[Message any](
	runtime *Runtime[Message],
	clipboard TerminalClipboard,
) (terminalOutput, error) {
	frame, err := runtime.renderIfDirty(true)
	if err != nil {
		return terminalOutput{}, err
	}
	output := terminalOutput{}
	if frame != nil {
		output.operations = frame.operations
		output.hasFrame = true
		if cursor, ok := frame.surface.Cursor(); ok {
			output.hasCursor = true
			output.cursorY = cursor.Y
		}
	}
	request, ok := runtime.TakeClipboardRequest()
	if ok && clipboard == TerminalClipboardOSC52 {
		output.operations = append(output.operations, vt.SetClipboard(request.Text()))
	}
	return output, nil
}

func pendingTerminalOutputOperations[Message any](
	runtime *Runtime[Message],
	clipboard TerminalClipboard,
) ([]vt.TerminalOp, error) {
	output, err := pendingTerminalOutput(runtime, clipboard)
	return output.operations, err
}
