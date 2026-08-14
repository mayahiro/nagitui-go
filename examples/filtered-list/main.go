package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/widget"
)

const pageSize = 6

var packages = []string{
	"calendar", "chart", "command palette", "file picker", "help",
	"list", "modal", "paginator", "progress", "scrollbar",
	"select", "sparkline", "table", "text area", "tree",
}

type messageKind uint8

const (
	queryMessage messageKind = iota
	selectMessage
	pageMessage
)

type message struct {
	kind  messageKind
	text  string
	index int
}

type filteredList struct {
	query    string
	selected int
	page     int
}

func (*filteredList) Init() tui.Effect[message] {
	return tui.NoneEffect[message]()
}

func (a *filteredList) Update(received message) tui.Effect[message] {
	switch received.kind {
	case queryMessage:
		a.query = received.text
		a.page = 0
		matches := matchingIndices(a.query)
		if len(matches) > 0 {
			a.selected = matches[0]
		}
	case selectMessage:
		a.selected = received.index
	case pageMessage:
		a.page = received.index
		matches := matchingIndices(a.query)
		start := a.page * pageSize
		if start < len(matches) {
			a.selected = matches[start]
		}
	}
	return tui.NoneEffect[message]()
}

func (*filteredList) Subscriptions() tui.Subscription[message] {
	return tui.NoneSubscription[message]()
}

func (a *filteredList) View(context tui.ViewContext) tui.Node[message] {
	items := make([]widget.ListItem, 0, len(packages))
	for index, name := range packages {
		items = append(items, widget.NewListItem(
			tui.NewNodeID(fmt.Sprintf("package-%d", index)),
			name,
		))
	}
	matches := matchingIndices(a.query)
	totalPages := (len(matches) + pageSize - 1) / pageSize
	results := tui.Text[message]("No matching widgets").WithLength(tui.Fixed(pageSize))
	if len(matches) > 0 {
		results = widget.NewList(
			tui.NewNodeID("results"),
			items,
			a.selected,
			func(index int) message { return message{kind: selectMessage, index: index} },
		).Filter(a.query).
			Paginate(a.page, pageSize).
			Viewport(tui.NewNodeID("results-viewport"), tui.Fixed(pageSize)).
			Node()
	}

	selection := "None"
	if a.selected >= 0 && a.selected < len(packages) && len(matches) > 0 {
		selection = packages[a.selected]
	}
	filter := tui.Row(
		tui.Text[message]("Filter: ").WithLength(tui.Fixed(8)),
		tui.StyledTextInput(
			tui.NewNodeID("query"),
			a.query,
			"type to filter",
			vt.Style{},
			vt.Style{Dim: true},
			func(value string) message { return message{kind: queryMessage, text: value} },
		).WithLength(tui.Flex(1)),
	)

	return tui.Panel(
		tui.Column(
			filter.WithLength(tui.Fixed(1)),
			tui.Gap[message](1),
			results.WithLength(tui.Fixed(pageSize)),
			tui.Row(
				widget.NewPaginator(
					tui.NewNodeID("pages"),
					a.page,
					totalPages,
					func(index int) message { return message{kind: pageMessage, index: index} },
				).Node(),
				tui.Text[message](fmt.Sprintf("  %d matches  selected: %s", len(matches), selection)),
			).WithLength(tui.Fixed(1)),
			widget.NewHelp[message]([]widget.HelpBinding{
				widget.NewHelpBinding("Tab", "focus"),
				widget.NewHelpBinding("Up/Down", "select"),
				widget.NewHelpBinding("Left/Right", "page"),
				widget.NewHelpBinding("Esc", "exit"),
			}).WidthProfile(context.WidthProfile).Node().WithLength(tui.Fixed(1)),
		),
		"Filtered widget catalog",
	)
}

func matchingIndices(query string) []int {
	query = strings.ToLower(query)
	matches := make([]int, 0, len(packages))
	for index, name := range packages {
		if strings.Contains(strings.ToLower(name), query) {
			matches = append(matches, index)
		}
	}
	return matches
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
	return tui.RunTerminal[message](&filteredList{}, options, mapEvent)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
