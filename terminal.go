package tui

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go/internal/ttyunix"
)

// TerminalOptions contains settings for RunTerminal
type TerminalOptions struct {
	// Capabilities contains optional output encoder capabilities
	Capabilities vt.Capabilities
	// MouseTracking enables SGR mouse reports when non-nil
	//
	// It is disabled by default so terminal text selection remains available.
	MouseTracking *vt.MouseTracking
	// FocusFirst focuses the first focusable node before the initial frame
	FocusFirst bool
	// EscapeTimeout disambiguates a lone ESC from an escape sequence
	EscapeTimeout time.Duration
	// MaximumIdleWait limits waits before checking resize and scheduler state
	MaximumIdleWait time.Duration
	// QueueCapacity is the maximum number of waiting application messages
	QueueCapacity int
	// TaskLimit is the maximum number of effect tasks executing concurrently
	TaskLimit int
	// SubscriptionCapacity is the maximum pending values retained per source
	SubscriptionCapacity int
	// MinimumFrameInterval limits non-urgent rendering; zero disables the limit
	MinimumFrameInterval time.Duration
}

// DefaultTerminalOptions returns conservative terminal loop settings
func DefaultTerminalOptions() TerminalOptions {
	return TerminalOptions{
		Capabilities:         vt.BaselineCapabilities(),
		EscapeTimeout:        25 * time.Millisecond,
		MaximumIdleWait:      50 * time.Millisecond,
		QueueCapacity:        DefaultQueueCapacity,
		TaskLimit:            DefaultTaskLimit,
		SubscriptionCapacity: DefaultSubscriptionCapacity,
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
	return RunTerminalContext(context.Background(), app, options, mapEvent)
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
	if ctx == nil {
		return errors.New("nagi-tui: nil terminal context")
	}
	if mapEvent == nil {
		return errors.New("nagi-tui: nil terminal event mapper")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return ttyunix.Run(ttyunix.Options{MouseTracking: options.MouseTracking}, func(session *ttyunix.Session) error {
		columns, rows, err := session.Size()
		if err != nil {
			return err
		}
		clock := NewSystemClock()
		config := NewRuntimeConfig(Size{Width: uint32(columns), Height: uint32(rows)})
		config.QueueCapacity = options.QueueCapacity
		config.TaskLimit = options.TaskLimit
		config.SubscriptionCapacity = options.SubscriptionCapacity
		config.MinimumFrameInterval = options.MinimumFrameInterval
		runtime, err := NewRuntimeWithClock(app, config, clock)
		if err != nil {
			return err
		}
		defer runtime.Close()
		focusFirst := options.FocusFirst
		decoder := NewTimedInputDecoder(clock, options.EscapeTimeout)
		input := make([]byte, 8_192)

		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			if session.TakeResize() {
				columns, rows, err = session.Size()
				if err != nil {
					return err
				}
				runtime.Resize(Size{Width: uint32(columns), Height: uint32(rows)})
			}
			if _, err := runtime.ProcessPending(); err != nil {
				return err
			}
			if focusFirst {
				focusFirst = false
				if _, err := runtime.focusFirst(); err != nil {
					return err
				}
			}
			if err := writeTerminalFrame(session, runtime, options.Capabilities); err != nil {
				return err
			}
			if runtime.ExitRequested() {
				break
			}

			timeout := options.MaximumIdleWait
			if deadline, ok := decoder.TimeUntilDeadline(); ok {
				timeout = min(timeout, deadline)
			}
			if deadline, ok := runtime.TimeUntilEffectDeadline(); ok {
				timeout = min(timeout, deadline)
			}
			if deadline, ok := runtime.TimeUntilSubscriptionDeadline(); ok {
				timeout = min(timeout, deadline)
			}
			if deadline, ok := runtime.TimeUntilFrameDeadline(); ok {
				timeout = min(timeout, deadline)
			}
			if timeout < 0 {
				timeout = 0
			}
			readable, err := session.WaitReadable(timeout)
			if err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
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
				dispatch, err := runtime.DispatchEvent(event)
				if err != nil {
					return err
				}
				if dispatch.Consumed() {
					continue
				}
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
				if exit {
					break
				}
			}
			if _, err := runtime.ProcessPending(); err != nil {
				return err
			}
			if err := writeTerminalFrame(session, runtime, options.Capabilities); err != nil {
				return err
			}
			if exit || runtime.ExitRequested() {
				break
			}
		}
		return nil
	})
}

func writeTerminalFrame[Message any](session *ttyunix.Session, runtime *Runtime[Message], capabilities vt.Capabilities) error {
	frame, err := runtime.RenderIfDirty()
	if err != nil {
		return err
	}
	if frame == nil {
		return nil
	}
	return session.WriteOperations(frame.Operations(), capabilities)
}
