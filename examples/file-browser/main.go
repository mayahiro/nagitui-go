package main

import (
	"fmt"
	"os"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/widget"
)

type messageKind uint8

const (
	selectMessage messageKind = iota
	openMessage
	backMessage
	hiddenMessage
)

type message struct {
	kind  messageKind
	index int
	value bool
}

type fileBrowser struct {
	cwd        string
	selected   int
	showHidden bool
	status     string
}

func newFileBrowser() *fileBrowser {
	return &fileBrowser{cwd: "/", status: "Select an entry"}
}

func (*fileBrowser) Init() tui.Effect[message] {
	return tui.NoneEffect[message]()
}

func (a *fileBrowser) Update(received message) tui.Effect[message] {
	switch received.kind {
	case selectMessage:
		a.selected = received.index
	case openMessage:
		entries := entriesFor(a.cwd)
		if received.index < 0 || received.index >= len(entries) {
			break
		}
		entry := entries[received.index]
		if entry.Directory {
			a.cwd = entry.Path
			a.selected = 0
			a.status = "Entered " + entry.Path
		} else {
			a.status = "Opened " + entry.Path
		}
	case backMessage:
		if a.cwd == "/" {
			a.status = "Already at root"
			break
		}
		a.cwd = parentDirectory(a.cwd)
		a.selected = 0
		a.status = "Returned to " + a.cwd
	case hiddenMessage:
		a.showHidden = received.value
		a.selected = 0
	}
	return tui.NoneEffect[message]()
}

func (*fileBrowser) Subscriptions() tui.Subscription[message] {
	return tui.NoneSubscription[message]()
}

func (a *fileBrowser) View(_ tui.ViewContext) tui.Node[message] {
	entries := entriesFor(a.cwd)
	picker := widget.NewFilePicker(
		tui.NewNodeID("files"),
		entries,
		a.selected,
		func(index int) message { return message{kind: selectMessage, index: index} },
	).OnOpen(func(index int) message {
		return message{kind: openMessage, index: index}
	}).OnBack(func() message {
		return message{kind: backMessage}
	}).ShowHidden(a.showHidden).
		Viewport(10).
		Node()

	name, kind, path := "None", "-", "-"
	if a.selected >= 0 && a.selected < len(entries) {
		entry := entries[a.selected]
		name, path = entry.Name, entry.Path
		kind = "file"
		if entry.Directory {
			kind = "directory"
		}
	}
	details := tui.Column(
		tui.RichText[message](
			tui.NewTextSpan("Name: ", vt.Style{Bold: true}),
			tui.NewTextSpan(name, vt.Style{}),
		),
		tui.Text[message]("Kind: "+kind),
		tui.Text[message]("Path: "+path),
		tui.Gap[message](1),
		tui.Text[message](a.status),
	)

	return tui.Column(
		tui.StyledText[message]("File browser  "+a.cwd, vt.Style{Bold: true}).WithLength(tui.Fixed(1)),
		widget.NewCheckbox(
			tui.NewNodeID("show-hidden"),
			"Show hidden entries",
			a.showHidden,
			func(value bool) message { return message{kind: hiddenMessage, value: value} },
		).Node().WithLength(tui.Fixed(1)),
		tui.Row(
			tui.Panel(picker, "Entries").WithLength(tui.Fixed(36)),
			tui.Panel(details, "Details").WithLength(tui.Flex(1)),
		).WithLength(tui.Flex(1)),
		widget.NewHelp[message]([]widget.HelpBinding{
			widget.NewHelpBinding("Up/Down", "select"),
			widget.NewHelpBinding("Enter", "open"),
			widget.NewHelpBinding("Left/Backspace", "parent"),
			widget.NewHelpBinding("Tab", "focus"),
			widget.NewHelpBinding("Esc", "exit"),
		}).Node().WithLength(tui.Fixed(1)),
	)
}

func entriesFor(directory string) []widget.FilePickerEntry {
	switch directory {
	case "/src":
		return []widget.FilePickerEntry{
			widget.NewFilePickerDirectory(tui.NewNodeID("src-widget"), "widget", "/src/widget"),
			widget.NewFilePickerFile(tui.NewNodeID("src-main"), "main.go", "/src/main.go"),
			widget.NewFilePickerFile(tui.NewNodeID("src-runtime"), "runtime.go", "/src/runtime.go"),
		}
	case "/src/widget":
		return []widget.FilePickerEntry{
			widget.NewFilePickerFile(tui.NewNodeID("widget-list"), "list.go", "/src/widget/list.go"),
			widget.NewFilePickerFile(tui.NewNodeID("widget-table"), "table.go", "/src/widget/table.go"),
			widget.NewFilePickerFile(tui.NewNodeID("widget-tree"), "tree.go", "/src/widget/tree.go"),
		}
	case "/docs":
		return []widget.FilePickerEntry{
			widget.NewFilePickerFile(tui.NewNodeID("docs-api"), "API.md", "/docs/API.md"),
			widget.NewFilePickerFile(tui.NewNodeID("docs-authoring"), "WIDGET_AUTHORING.md", "/docs/WIDGET_AUTHORING.md"),
		}
	default:
		return []widget.FilePickerEntry{
			widget.NewFilePickerDirectory(tui.NewNodeID("root-src"), "src", "/src"),
			widget.NewFilePickerDirectory(tui.NewNodeID("root-docs"), "docs", "/docs"),
			widget.NewFilePickerFile(tui.NewNodeID("root-readme"), "README.md", "/README.md"),
			widget.NewFilePickerFile(tui.NewNodeID("root-env"), ".env", "/.env").WithHidden(true),
		}
	}
}

func parentDirectory(directory string) string {
	switch directory {
	case "/src/widget":
		return "/src"
	default:
		return "/"
	}
}

func mapEvent(event vt.Event) tui.EventAction[message] {
	switch {
	case event.Kind == vt.EventKey && event.Key.Code == vt.KeyEscape:
		return tui.ExitAction[message]()
	case event.Kind == vt.EventKey && event.Key.Code == vt.KeyCharacter && event.Key.Character == 'c' && event.Key.Modifiers.Control:
		return tui.ExitAction[message]()
	default:
		return tui.IgnoreAction[message]()
	}
}

func run() error {
	options := tui.DefaultTerminalOptions()
	options.FocusFirst = true
	mouseTracking := vt.MouseTrackingPress
	options.MouseTracking = &mouseTracking
	return tui.RunTerminal[message](newFileBrowser(), options, mapEvent)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
