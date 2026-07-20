package main

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

const (
	logInterval   = 5 * time.Millisecond
	frameInterval = 16 * time.Millisecond
	maxLogLines   = 1_000
)

type message struct {
	sequence uint64
	toggle   bool
	quit     bool
}

type logViewer struct {
	paused   bool
	exiting  bool
	sequence atomic.Uint64
	lines    []string
}

func (*logViewer) Init() tui.Effect[message] {
	return tui.NoneEffect[message]()
}

func (a *logViewer) Update(msg message) tui.Effect[message] {
	if msg.quit {
		a.exiting = true
		return tui.ExitEffect[message]()
	}
	if msg.toggle {
		a.paused = !a.paused
		return tui.NoneEffect[message]()
	}
	level := "\x1b[32mINFO\x1b[0m"
	if msg.sequence%10 == 0 {
		level = "\x1b[31;1mERROR\x1b[0m"
	}
	a.lines = append(a.lines, fmt.Sprintf("%06d %s simulated log event", msg.sequence, level))
	if len(a.lines) > maxLogLines {
		copy(a.lines, a.lines[len(a.lines)-maxLogLines:])
		a.lines = a.lines[:maxLogLines]
	}
	return tui.NoneEffect[message]()
}

func (a *logViewer) Subscriptions() tui.Subscription[message] {
	if a.paused {
		return tui.NoneSubscription[message]()
	}
	return tui.EverySubscription("live-logs", logInterval, tui.LatestDelivery(), func() message {
		return message{sequence: a.sequence.Add(1)}
	})
}

func (a *logViewer) View(context tui.ViewContext) tui.Node[message] {
	status := "LIVE"
	if a.exiting {
		status = "STOPPED"
	} else if a.paused {
		status = "PAUSED"
	}
	lines := make([]tui.Node[message], 0, max(len(a.lines), 1))
	for _, line := range a.lines {
		lines = append(lines, tui.ANSIText[message](line, tui.ANSITextOptions{
			Paragraph: tui.ParagraphOptions{Wrap: tui.WrapNone, Alignment: tui.AlignStart},
		}).WithLength(tui.Fixed(1)))
	}
	if len(lines) == 0 {
		lines = append(lines, tui.Text[message]("Waiting for logs...").WithLength(tui.Fixed(1)))
	}
	help := "Space/P pause, PageUp/PageDown/Home/End or wheel scroll, Q/Esc quit"
	if context.Size.Width < 60 {
		help = "P pause  Q quit"
	}
	return tui.Border(
		tui.Column(
			tui.StyledText[message]("Log Viewer", vt.Style{Bold: true}).WithLength(tui.Fixed(1)),
			tui.Text[message](fmt.Sprintf("Status: %s  Buffered: %d", status, len(a.lines))).WithLength(tui.Fixed(1)),
			tui.ScrollViewportWithOptions(
				"log-scroll",
				tui.Column(lines...),
				tui.ScrollViewportOptions[message]{
					Axis:       tui.ScrollAxisVertical,
					StickToEnd: true,
				},
			).WithLength(tui.Flex(1)),
			tui.Text[message](help).WithLength(tui.Fixed(1)),
		),
		vt.Style{},
	)
}

func mapEvent(event vt.Event) tui.EventAction[message] {
	switch {
	case event.Kind == vt.EventKey && event.Key.Code == vt.KeyEscape:
		return tui.MessageAction(message{quit: true})
	case event.Kind == vt.EventKey && event.Key.Code == vt.KeyCharacter && event.Key.Character == 'c' && event.Key.Modifiers.Control:
		return tui.MessageAction(message{quit: true})
	case event.Kind == vt.EventText && (event.Text == "q" || event.Text == "Q"):
		return tui.MessageAction(message{quit: true})
	case isPauseToggle(event):
		return tui.MessageAction(message{toggle: true})
	default:
		return tui.IgnoreAction[message]()
	}
}

func isPauseToggle(event vt.Event) bool {
	if event.Kind == vt.EventText {
		return event.Text == " " || event.Text == "p" || event.Text == "P"
	}
	if event.Kind != vt.EventKey || event.Key.Action == vt.KeyRelease || event.Key.Code != vt.KeyCharacter {
		return false
	}
	modifiers := event.Key.Modifiers
	return !modifiers.Alt && !modifiers.Control && !modifiers.Meta &&
		(event.Key.Character == ' ' || event.Key.Character == 'p' || event.Key.Character == 'P')
}

func run() error {
	options := tui.DefaultTerminalOptions()
	options.MinimumFrameInterval = frameInterval
	options.FocusFirst = true
	mouseTracking := vt.MouseTrackingPress
	options.MouseTracking = &mouseTracking
	return tui.RunTerminal[message](&logViewer{}, options, mapEvent)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
