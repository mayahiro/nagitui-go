package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/widget"
)

type message struct {
	kind        string
	index       int
	value       bool
	text        string
	textArea    widget.TextAreaState
	composer    widget.ComposerState
	selectable  widget.SelectableTextState
	copyRequest widget.TextCopyRequest
}

type gallery struct {
	page              int
	feature           bool
	mode              int
	theme             int
	notes             widget.TextAreaState
	composer          widget.ComposerState
	history           []string
	row               int
	tree              int
	treeExpanded      bool
	detailsOpen       bool
	selectableContent widget.SelectableTextContent
	selectableState   widget.SelectableTextState
	query             string
	command           int
	lastAction        string
	dialogOpen        bool
}

func newGallery() *gallery {
	return &gallery{
		feature: true, treeExpanded: true,
		notes:    widget.NewTextAreaStateAtEnd("Multiline notes\nremain application state"),
		composer: widget.NewComposerStateAtEnd("Draft message"),
		history:  []string{"Earlier message"},
		selectableContent: widget.NewSelectableTextContent([]tui.TextSpan{
			tui.NewTextSpan("Selectable", vt.Style{Bold: true}),
			tui.NewTextSpan(" text keeps application-owned selection.", vt.Style{}),
		}),
		selectableState: widget.NewSelectableTextStateWithSelection(10, 0),
		lastAction:      "None",
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
	case "composer":
		a.composer = received.composer
	case "submit-composer":
		value := a.composer.TextArea().Value()
		if strings.TrimSpace(value) != "" {
			if len(a.history) == 8 {
				copy(a.history, a.history[1:])
				a.history = a.history[:7]
			}
			a.history = append(a.history, value)
			a.composer = widget.NewComposerStateAtEnd("")
			a.lastAction = "Submitted: " + strings.ReplaceAll(value, "\n", " / ")
		}
	case "row":
		a.row = received.index
	case "tree":
		a.tree = received.index
	case "toggle-tree":
		if received.index == 0 {
			a.treeExpanded = received.value
		}
	case "toggle-details":
		a.detailsOpen = received.value
	case "selectable":
		a.selectableState = received.selectable
	case "copy-text":
		kind := "selection"
		if received.copyRequest.Kind == widget.TextCopyDocument {
			kind = "document"
		}
		a.lastAction = "Copied " + kind + ": " + strings.ReplaceAll(received.copyRequest.Text, "\n", " / ")
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
	case "open-dialog":
		a.dialogOpen = true
	case "dialog-choice":
		actions := []string{"Open from dialog", "Save from dialog"}
		if received.index >= 0 && received.index < len(actions) {
			a.lastAction = actions[received.index]
		} else {
			a.lastAction = "Unknown dialog action"
		}
		a.dialogOpen = false
	case "close-dialog":
		a.dialogOpen = false
	}
	return tui.NoneEffect[message]()
}

func (*gallery) Subscriptions() tui.Subscription[message] {
	return tui.NoneSubscription[message]()
}

func (a *gallery) View(viewContext tui.ViewContext) tui.Node[message] {
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
	page := a.inputsPage(viewContext)
	if a.page == 1 {
		page = a.dataPage()
	} else if a.page >= 2 {
		page = a.commandsPage()
	}
	content := tui.Border(
		tui.Column(
			tui.StyledText[message]("Extended Widget Gallery", vt.Style{Bold: true}).WithLength(tui.Fixed(1)),
			tabs,
			page.WithLength(tui.Flex(1)),
			tui.Text[message]("Tab changes focus, arrows navigate, Enter/Space activate, Esc exits").WithLength(tui.Fixed(1)),
		),
		vt.Style{},
	)
	if !a.dialogOpen {
		return content
	}
	dialog := widget.NewDialog(
		tui.NewNodeID("choice-dialog"),
		tui.Text[message]("Choose one application-defined command"),
		[]widget.DialogAction[message]{
			widget.NewDialogAction(tui.NewNodeID("dialog-open"), "Open", func() message {
				return message{kind: "dialog-choice", index: 0}
			}),
			widget.NewDialogAction(tui.NewNodeID("dialog-save"), "Save", func() message {
				return message{kind: "dialog-choice", index: 1}
			}),
			widget.NewDialogAction(tui.NewNodeID("dialog-cancel"), "Cancel", func() message {
				return message{kind: "close-dialog"}
			}),
		},
	).Title(tui.StyledText[message]("Generic dialog", vt.Style{Bold: true})).
		DefaultAction(tui.NewNodeID("dialog-cancel")).
		CancelAction(tui.NewNodeID("dialog-cancel")).
		WidthProfile(viewContext.WidthProfile).
		ActionWrapWidth(max(viewContext.Size.Width, 5) - 4).
		Node()
	return tui.Stack(content, dialog)
}

func (a *gallery) inputsPage(viewContext tui.ViewContext) tui.Node[message] {
	composerWidth := max(int(viewContext.Size.Width)-4, 1)
	composerValid := strings.TrimSpace(a.composer.TextArea().Value()) != ""
	composer := widget.NewComposer(
		tui.NewNodeID("composer"),
		tui.NewNodeID("composer-viewport"),
		tui.NewNodeID("composer-caret"),
		a.composer,
		func(state widget.ComposerState) message {
			return message{kind: "composer", composer: state}
		},
		func() message { return message{kind: "submit-composer"} },
	).Placeholder("Enter a message").
		WidthProfile(viewContext.WidthProfile).
		SoftWrap(composerWidth).
		Rows(1, 3).
		History(a.history).
		MaximumGraphemes(240, widget.ComposerOverflowTruncate).
		SubmitEnabled(composerValid)
	if !composerValid {
		composer = composer.Validation(tui.Text[message]("A message is required"))
	}
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
			}).Placeholder("Enter notes").WidthProfile(viewContext.WidthProfile).Node(),
			vt.Style{},
		),
		tui.Text[message]("Composer: Enter submits, Shift-Enter inserts a line"),
		tui.Border(composer.Node(), vt.Style{}),
	)
}

func (a *gallery) dataPage() tui.Node[message] {
	details := widget.NewDisclosure(
		tui.NewNodeID("process-details"),
		tui.Text[message]("Process details"),
		a.detailsOpen,
		func(expanded bool) message { return message{kind: "toggle-details", value: expanded} },
	).Body(func() tui.Node[message] {
		return tui.Text[message]("Metrics are application-owned detail content")
	}).Node()
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
		details,
		table,
		tui.Text[message]("Tree:"),
		tree,
		tui.Row(
			tui.Text[message]("Viewport: "),
			widget.NewScrollbar[message](100, 30, offset, 24).Orientation(widget.ScrollbarHorizontal).Node(),
		),
		tui.Text[message]("SelectableText: Shift-arrows select, Ctrl-C copies, Ctrl-Shift-C copies all"),
		tui.Border(
			widget.NewSelectableText(
				tui.NewNodeID("selectable-text"),
				a.selectableContent,
				a.selectableState,
				func(state widget.SelectableTextState) message {
					return message{kind: "selectable", selectable: state}
				},
			).OnCopy(func(request widget.TextCopyRequest) message {
				return message{kind: "copy-text", copyRequest: request}
			}).Node(),
			vt.Style{},
		),
		tui.Text[message]("Last action: "+a.lastAction),
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
		widget.NewButton(tui.NewNodeID("open-dialog"), "Open generic dialog", func() message {
			return message{kind: "open-dialog"}
		}).Node(),
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
