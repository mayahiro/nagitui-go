package main

import (
	"fmt"
	"os"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/widget"
)

type message struct {
	kind     string
	index    int
	value    bool
	text     string
	textArea widget.TextAreaState
}

type gallery struct {
	page         int
	feature      bool
	mode         int
	theme        int
	notes        widget.TextAreaState
	row          int
	tree         int
	treeExpanded bool
	query        string
	command      int
	lastAction   string
}

func newGallery() *gallery {
	return &gallery{
		feature: true, treeExpanded: true,
		notes:      widget.NewTextAreaStateAtEnd("Multiline notes\nremain application state"),
		lastAction: "None",
	}
}

func (*gallery) Init() tui.Effect[message] {
	return tui.NoneEffect[message]()
}

func (a *gallery) Update(received message) tui.Effect[message] {
	switch received.kind {
	case "page":
		a.page = received.index
	case "feature":
		a.feature = received.value
	case "mode":
		a.mode = received.index
	case "theme":
		a.theme = received.index
	case "notes":
		a.notes = received.textArea
	case "row":
		a.row = received.index
	case "tree":
		a.tree = received.index
	case "toggle-tree":
		if received.index == 0 {
			a.treeExpanded = received.value
		}
	case "query":
		a.query = received.text
	case "command":
		a.command = received.index
	case "activate":
		actions := []string{"Open file", "Save file", "Toggle sidebar"}
		if received.index >= 0 && received.index < len(actions) {
			a.lastAction = actions[received.index]
		} else {
			a.lastAction = "Unknown"
		}
	}
	return tui.NoneEffect[message]()
}

func (*gallery) Subscriptions() tui.Subscription[message] {
	return tui.NoneSubscription[message]()
}

func (a *gallery) View(_ tui.ViewContext) tui.Node[message] {
	tabs := widget.NewTabs(
		tui.NewNodeID("gallery-tabs"),
		[]widget.TabItem{
			widget.NewTabItem(tui.NewNodeID("page-inputs"), "Inputs"),
			widget.NewTabItem(tui.NewNodeID("page-data"), "Data"),
			widget.NewTabItem(tui.NewNodeID("page-commands"), "Commands"),
		},
		a.page,
		func(index int) message { return message{kind: "page", index: index} },
	).Node().WithLength(tui.Fixed(1))
	page := a.inputsPage()
	if a.page == 1 {
		page = a.dataPage()
	} else if a.page >= 2 {
		page = a.commandsPage()
	}
	return tui.Border(
		tui.Column(
			tui.StyledText[message]("Extended Widget Gallery", vt.Style{Bold: true}).WithLength(tui.Fixed(1)),
			tabs,
			page.WithLength(tui.Flex(1)),
			tui.Text[message]("Tab changes focus, arrows navigate, Enter/Space activate, Esc exits").WithLength(tui.Fixed(1)),
		),
		vt.Style{},
	)
}

func (a *gallery) inputsPage() tui.Node[message] {
	return tui.Column(
		widget.NewCheckbox(tui.NewNodeID("feature"), "Enable feature", a.feature, func(value bool) message {
			return message{kind: "feature", value: value}
		}).Node(),
		tui.Row(
			widget.NewRadio(tui.NewNodeID("mode-safe"), "Safe", a.mode == 0, func() message {
				return message{kind: "mode", index: 0}
			}).Node(),
			tui.Text[message]("  "),
			widget.NewRadio(tui.NewNodeID("mode-fast"), "Fast", a.mode == 1, func() message {
				return message{kind: "mode", index: 1}
			}).Node(),
		),
		widget.NewSelect(tui.NewNodeID("theme"), []string{"System", "Light", "Dark"}, a.theme, func(index int) message {
			return message{kind: "theme", index: index}
		}).Node(),
		tui.Text[message]("TextArea:"),
		tui.Border(
			widget.NewTextArea(tui.NewNodeID("notes"), a.notes, func(state widget.TextAreaState) message {
				return message{kind: "notes", textArea: state}
			}).Placeholder("Enter notes").Node(),
			vt.Style{},
		),
	)
}

func (a *gallery) dataPage() tui.Node[message] {
	table := widget.NewTable(
		tui.NewNodeID("process-table"),
		[]widget.TableColumn{
			widget.NewTableColumn("Process", tui.Flex(1)),
			widget.NewTableColumn("State", tui.Fixed(8)),
			widget.NewTableColumn("CPU", tui.Fixed(6)),
		},
		[]widget.TableRow{
			widget.NewTableRow(tui.NewNodeID("process-api"), []string{"api", "Ready", "12%"}),
			widget.NewTableRow(tui.NewNodeID("process-worker"), []string{"worker", "Busy", "48%"}),
			widget.NewTableRow(tui.NewNodeID("process-index"), []string{"indexer", "Idle", "2%"}),
		},
		a.row,
		func(index int) message { return message{kind: "row", index: index} },
	).Node()
	tree := widget.NewTree(
		tui.NewNodeID("file-tree"),
		[]widget.TreeItem{
			widget.NewTreeBranch(tui.NewNodeID("tree-src"), "src", 0, a.treeExpanded),
			widget.NewTreeLeaf(tui.NewNodeID("tree-main"), "main.go", 1),
			widget.NewTreeLeaf(tui.NewNodeID("tree-lib"), "library.go", 1),
			widget.NewTreeLeaf(tui.NewNodeID("tree-readme"), "README.md", 0),
		},
		a.tree,
		func(index int) message { return message{kind: "tree", index: index} },
	).OnToggle(func(index int, expanded bool) message {
		return message{kind: "toggle-tree", index: index, value: expanded}
	}).Node()
	offset := uint64(max(a.row, 0)) * 35
	return tui.Column(
		table,
		tui.Text[message]("Tree:"),
		tree,
		tui.Row(
			tui.Text[message]("Viewport: "),
			widget.NewScrollbar[message](100, 30, offset, 24).Orientation(widget.ScrollbarHorizontal).Node(),
		),
	)
}

func (a *gallery) commandsPage() tui.Node[message] {
	return tui.Column(
		widget.NewCommandPalette(
			tui.NewNodeID("command-palette"), tui.NewNodeID("command-query"), a.query,
			[]widget.Command{
				widget.NewCommand(tui.NewNodeID("command-open"), "Open file").WithKeywords("read"),
				widget.NewCommand(tui.NewNodeID("command-save"), "Save file").WithKeywords("write"),
				widget.NewCommand(tui.NewNodeID("command-sidebar"), "Toggle sidebar").WithKeywords("panel"),
			},
			a.command,
			func(query string) message { return message{kind: "query", text: query} },
			func(index int) message { return message{kind: "command", index: index} },
			func(index int) message { return message{kind: "activate", index: index} },
		).Title("Command Palette").Node(),
		tui.Text[message]("Last action: "+a.lastAction),
	)
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
	return tui.RunTerminal[message](newGallery(), options, mapEvent)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
