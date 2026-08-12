package main

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/widget"
)

const (
	logInterval = 750 * time.Millisecond
	maxLogLines = 200
)

var sources = []string{"all", "api", "worker", "database"}

type logEntry struct {
	sequence uint64
	source   string
	level    string
	text     string
}

type messageKind uint8

const (
	logMessage messageKind = iota
	sourceMessage
	rowMessage
	resizeMessage
	toggleMessage
	toggleDetailsMessage
	dismissDetailsMessage
)

type message struct {
	kind     messageKind
	sequence uint64
	index    int
	panes    widget.SplitPaneState
}

type logViewer struct {
	paused   bool
	source   int
	row      int
	sequence atomic.Uint64
	logs     []logEntry
	panes    widget.SplitPaneState
	details  bool
}

func newLogViewer() *logViewer {
	viewer := &logViewer{panes: widget.NewSplitPaneState(2_500)}
	for sequence := uint64(1); sequence <= 5; sequence++ {
		viewer.logs = append(viewer.logs, generatedLog(sequence))
	}
	viewer.sequence.Store(5)
	return viewer
}

func (*logViewer) Init() tui.Effect[message] {
	return tui.NoneEffect[message]()
}

func (a *logViewer) Update(received message) tui.Effect[message] {
	switch received.kind {
	case logMessage:
		a.logs = append(a.logs, generatedLog(received.sequence))
		if len(a.logs) > maxLogLines {
			copy(a.logs, a.logs[len(a.logs)-maxLogLines:])
			a.logs = a.logs[:maxLogLines]
		}
		a.clampRow()
	case sourceMessage:
		a.source = received.index
		a.row = 0
	case rowMessage:
		a.row = received.index
	case resizeMessage:
		a.panes = received.panes
	case toggleMessage:
		a.paused = !a.paused
	case toggleDetailsMessage:
		a.details = !a.details
	case dismissDetailsMessage:
		a.details = false
	}
	return tui.NoneEffect[message]()
}

func (a *logViewer) Subscriptions() tui.Subscription[message] {
	if a.paused {
		return tui.NoneSubscription[message]()
	}
	return tui.EverySubscription("multi-pane-logs", logInterval, tui.LatestDelivery(), func() message {
		return message{kind: logMessage, sequence: a.sequence.Add(1)}
	})
}

func (a *logViewer) View(context tui.ViewContext) tui.Node[message] {
	sourceItems := make([]widget.ListItem, 0, len(sources))
	for index, source := range sources {
		sourceItems = append(sourceItems, widget.NewListItem(
			tui.NewNodeID(fmt.Sprintf("source-%d", index)),
			source,
		))
	}
	sourceList := widget.NewList(
		tui.NewNodeID("sources"),
		sourceItems,
		a.source,
		func(index int) message { return message{kind: sourceMessage, index: index} },
	).Node()

	visible := a.filteredLogs()
	rows := make([]widget.TableRow, 0, len(visible))
	for _, entry := range visible {
		rows = append(rows, widget.NewTableRow(
			tui.NewNodeID(fmt.Sprintf("log-%d", entry.sequence)),
			[]string{
				fmt.Sprintf("%06d", entry.sequence),
				entry.source,
				entry.level,
				entry.text,
			},
		))
	}
	table := widget.NewTable(
		tui.NewNodeID("logs"),
		[]widget.TableColumn{
			widget.NewTableColumn("Seq", tui.Fixed(8)),
			widget.NewTableColumn("Source", tui.Fixed(10)),
			widget.NewTableColumn("Level", tui.Fixed(8)),
			widget.NewTableColumn("Message", tui.Flex(1)),
		},
		rows,
		a.row,
		func(index int) message { return message{kind: rowMessage, index: index} },
	).ColumnAlignment(0, tui.AlignEnd).
		Viewport(tui.NewNodeID("log-rows"), tui.Fixed(9)).
		Node()

	detail := "No matching log entry"
	if a.row >= 0 && a.row < len(visible) {
		entry := visible[a.row]
		detail = fmt.Sprintf("#%06d [%s] %s: %s", entry.sequence, entry.source, entry.level, entry.text)
	}
	status := "LIVE"
	if a.paused {
		status = "PAUSED"
	}

	panes := widget.NewSplitPane(
		tui.NewNodeID("log-panes"),
		tui.Panel(sourceList, "Sources"),
		tui.Panel(table, "Events"),
		a.panes,
	).Minimums(18, 30).
		Collapse(tui.SplitPaneCollapsePrimary).
		FocusTargets(tui.NewNodeID("sources"), tui.NewNodeID("logs")).
		OnResize(func(state widget.SplitPaneState) message {
			return message{kind: resizeMessage, panes: state}
		}).Node().WithLength(tui.Flex(1))

	base := tui.Column(
		tui.StyledText[message](
			fmt.Sprintf("Multi-pane log viewer  %s  buffered: %d", status, len(a.logs)),
			vt.Style{Bold: true},
		).WithLength(tui.Fixed(1)),
		panes,
		widget.NewHelp[message]([]widget.HelpBinding{
			widget.NewHelpBinding("F6", "pane focus"),
			widget.NewHelpBinding("Alt+Left/Right", "resize"),
			widget.NewHelpBinding("Up/Down", "select"),
			widget.NewHelpBinding("p", "pause"),
			widget.NewHelpBinding("d", "details"),
			widget.NewHelpBinding("Esc", "exit"),
		}).WidthProfile(context.WidthProfile).Node().WithLength(tui.Fixed(1)),
	)
	return widget.NewDrawer(tui.NewNodeID("details-drawer"), base, a.details).
		Side(widget.DrawerBottom).
		Size(tui.Fixed(5)).
		Body(func() tui.Node[message] {
			return tui.Column(
				tui.StyledText[message]("Details", vt.Style{Bold: true}),
				tui.Paragraph[message](
					[]tui.TextSpan{tui.NewTextSpan(detail, vt.Style{})},
					tui.DefaultParagraphOptions(),
				),
			)
		}).
		OnDismiss(func() message { return message{kind: dismissDetailsMessage} }).
		Node()
}

func (a *logViewer) filteredLogs() []logEntry {
	if a.source <= 0 || a.source >= len(sources) {
		return append([]logEntry(nil), a.logs...)
	}
	source := sources[a.source]
	visible := make([]logEntry, 0, len(a.logs))
	for _, entry := range a.logs {
		if entry.source == source {
			visible = append(visible, entry)
		}
	}
	return visible
}

func (a *logViewer) clampRow() {
	a.row = min(a.row, max(len(a.filteredLogs())-1, 0))
}

func generatedLog(sequence uint64) logEntry {
	source := []string{"api", "worker", "database"}[sequence%3]
	level := "INFO"
	text := "request completed"
	if sequence%5 == 0 {
		level = "WARN"
		text = "latency threshold exceeded"
	}
	if sequence%11 == 0 {
		level = "ERROR"
		text = "upstream connection failed"
	}
	return logEntry{sequence: sequence, source: source, level: level, text: text}
}

func mapEvent(event vt.Event) tui.EventAction[message] {
	if isPauseToggle(event) {
		return tui.MessageAction(message{kind: toggleMessage})
	}
	if isDetailsToggle(event) {
		return tui.MessageAction(message{kind: toggleDetailsMessage})
	}
	switch {
	case event.Kind == vt.EventKey && event.Key.Code == vt.KeyEscape:
		return tui.ExitAction[message]()
	case event.Kind == vt.EventKey && event.Key.Code == vt.KeyCharacter && event.Key.Character == 'c' && event.Key.Modifiers.Control:
		return tui.ExitAction[message]()
	default:
		return tui.IgnoreAction[message]()
	}
}

func isDetailsToggle(event vt.Event) bool {
	if event.Kind == vt.EventText {
		return event.Text == "d" || event.Text == "D"
	}
	if event.Kind != vt.EventKey || event.Key.Action == vt.KeyRelease || event.Key.Code != vt.KeyCharacter {
		return false
	}
	modifiers := event.Key.Modifiers
	return !modifiers.Alt && !modifiers.Control && !modifiers.Meta &&
		(event.Key.Character == 'd' || event.Key.Character == 'D')
}

func isPauseToggle(event vt.Event) bool {
	if event.Kind == vt.EventText {
		return event.Text == "p" || event.Text == "P"
	}
	if event.Kind != vt.EventKey || event.Key.Action == vt.KeyRelease || event.Key.Code != vt.KeyCharacter {
		return false
	}
	modifiers := event.Key.Modifiers
	return !modifiers.Alt && !modifiers.Control && !modifiers.Meta &&
		(event.Key.Character == 'p' || event.Key.Character == 'P')
}

func run() error {
	options := tui.DefaultTerminalOptions()
	options.MinimumFrameInterval = 33 * time.Millisecond
	options.FocusFirst = true
	mouseTracking := vt.MouseTrackingButton
	options.MouseTracking = &mouseTracking
	return tui.RunTerminal[message](newLogViewer(), options, mapEvent)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
