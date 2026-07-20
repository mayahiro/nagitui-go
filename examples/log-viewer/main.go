package main

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

const (
	logInterval       = 5 * time.Millisecond
	frameInterval     = (time.Second + 59) / 60
	uptimeInterval    = time.Second
	maximumBatchLines = 64
	maxLogLines       = 1_000
)

type messageKind uint8

const (
	processOutputMessage messageKind = iota
	uptimeMessage
	togglePauseMessage
	quitMessage
)

type message struct {
	kind          messageKind
	sequence      uint64
	uptimeSeconds uint64
}

type logBuffer struct {
	values []string
	start  int
	limit  int
}

func newLogBuffer(limit int) logBuffer {
	return logBuffer{values: make([]string, 0, limit), limit: limit}
}

func (b *logBuffer) append(value string) {
	if b.limit == 0 {
		return
	}
	if len(b.values) < b.limit {
		b.values = append(b.values, value)
		return
	}
	b.values[b.start] = value
	b.start = (b.start + 1) % b.limit
}

func (b logBuffer) len() int {
	return len(b.values)
}

func (b logBuffer) at(index int) string {
	return b.values[(b.start+index)%len(b.values)]
}

type logViewer struct {
	paused    bool
	exiting   bool
	startedAt time.Time
	uptime    uint64
	sequence  atomic.Uint64
	lines     logBuffer
}

func newLogViewer() *logViewer {
	return &logViewer{lines: newLogBuffer(maxLogLines)}
}

func (a *logViewer) Init() tui.Effect[message] {
	a.startedAt = time.Now()
	return tui.NoneEffect[message]()
}

func (a *logViewer) Update(msg message) tui.Effect[message] {
	switch msg.kind {
	case processOutputMessage:
		a.lines.append(formatLogLine(msg.sequence))
	case uptimeMessage:
		a.uptime = msg.uptimeSeconds
	case togglePauseMessage:
		a.paused = !a.paused
	case quitMessage:
		a.exiting = true
		return tui.ExitEffect[message]()
	}
	return tui.NoneEffect[message]()
}

func (a *logViewer) Subscriptions() tui.Subscription[message] {
	if a.exiting {
		return tui.NoneSubscription[message]()
	}
	startedAt := a.startedAt
	subscriptions := []tui.Subscription[message]{
		tui.EverySubscription("uptime", uptimeInterval, tui.LatestDelivery(), func() message {
			return message{
				kind:          uptimeMessage,
				uptimeSeconds: uint64(time.Since(startedAt) / time.Second),
			}
		}),
	}
	if !a.paused {
		subscriptions = append(subscriptions, processOutputSubscription(&a.sequence))
	}
	return tui.BatchSubscriptions(subscriptions...)
}

func processOutputSubscription(sequence *atomic.Uint64) tui.Subscription[message] {
	return tui.StreamSubscription(
		"process-output",
		tui.BatchDelivery(maximumBatchLines, frameInterval),
		func(ctx context.Context, sink tui.SubscriptionSink[message]) {
			// A real adapter blocks on process stdout. The timer only keeps this
			// example self-contained while Nagi owns cancellation and wake-up.
			ticker := time.NewTicker(logInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if !sink.Send(message{kind: processOutputMessage, sequence: sequence.Add(1)}) {
						return
					}
				}
			}
		},
	)
}

func (a *logViewer) View(viewContext tui.ViewContext) tui.Node[message] {
	status := "LIVE"
	if a.exiting {
		status = "STOPPED"
	} else if a.paused {
		status = "PAUSED"
	}
	lines := a.lines
	contentHeight := uint32(max(lines.len(), 1))
	help := "Space/P pause, PageUp/PageDown/Home/End or wheel scroll, Q/Esc quit"
	if viewContext.Size.Width < 60 {
		help = "P pause  Q quit"
	}
	return tui.Border(
		tui.Column(
			tui.StyledText[message]("Event-driven Process Monitor", vt.Style{Bold: true}).WithLength(tui.Fixed(1)),
			tui.Text[message](fmt.Sprintf(
				"Status: %s  Uptime: %s  Buffered: %d",
				status,
				formatUptime(a.uptime),
				lines.len(),
			)).WithLength(tui.Fixed(1)),
			tui.VirtualScrollViewportWithOptions(
				"log-scroll",
				tui.Size{Height: contentHeight},
				tui.ScrollViewportOptions[message]{
					Axis:       tui.ScrollAxisVertical,
					StickToEnd: true,
				},
				func(viewport tui.VirtualViewport) tui.VirtualFragment[message] {
					if lines.len() == 0 {
						return tui.NewVirtualFragment(
							tui.ScrollOffset{},
							tui.Column(tui.Text[message]("Waiting for process output...").WithLength(tui.Fixed(1))),
						)
					}
					start := int(viewport.Offset.Y)
					end := min(start+int(viewport.Size.Height), lines.len())
					visible := make([]tui.Node[message], 0, end-start)
					for index := start; index < end; index++ {
						visible = append(visible, tui.ANSIText[message](lines.at(index), tui.ANSITextOptions{
							Paragraph: tui.ParagraphOptions{Wrap: tui.WrapNone, Alignment: tui.AlignStart},
						}).WithLength(tui.Fixed(1)))
					}
					return tui.NewVirtualFragment(
						tui.ScrollOffset{Y: uint32(start)},
						tui.Column(visible...),
					)
				},
			).WithLength(tui.Flex(1)),
			tui.Text[message](help).WithLength(tui.Fixed(1)),
		),
		vt.Style{},
	)
}

func formatLogLine(sequence uint64) string {
	level := "\x1b[32mINFO\x1b[0m"
	if sequence%10 == 0 {
		level = "\x1b[31;1mERROR\x1b[0m"
	}
	return fmt.Sprintf("%06d %s simulated process output", sequence, level)
}

func formatUptime(seconds uint64) string {
	return fmt.Sprintf("%02d:%02d:%02d", seconds/3_600, seconds/60%60, seconds%60)
}

func mapEvent(event vt.Event) tui.EventAction[message] {
	switch {
	case event.Kind == vt.EventKey && event.Key.Code == vt.KeyEscape:
		return tui.MessageAction(message{kind: quitMessage})
	case event.Kind == vt.EventKey && event.Key.Code == vt.KeyCharacter && event.Key.Character == 'c' && event.Key.Modifiers.Control:
		return tui.MessageAction(message{kind: quitMessage})
	case event.Kind == vt.EventText && (event.Text == "q" || event.Text == "Q"):
		return tui.MessageAction(message{kind: quitMessage})
	case isPauseToggle(event):
		return tui.MessageAction(message{kind: togglePauseMessage})
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
	return tui.RunTerminal[message](newLogViewer(), options, mapEvent)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
