package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

const searchDelay = 150 * time.Millisecond

var items = []string{
	"Application runtime",
	"Cell surface",
	"Effect supervision",
	"Grapheme-aware text",
	"Interaction state",
	"Unix terminal session",
	"VT codec",
}

type messageKind uint8

const (
	queryChangedMessage messageKind = iota
	searchFinishedMessage
)

type message struct {
	kind    messageKind
	query   string
	results []string
}

type asyncSearch struct {
	query     string
	results   []string
	searching bool
}

func (a *asyncSearch) Init() tui.Effect[message] {
	a.searching = true
	return search("")
}

func (a *asyncSearch) Update(msg message) tui.Effect[message] {
	switch msg.kind {
	case queryChangedMessage:
		a.query = msg.query
		a.searching = true
		return search(msg.query)
	case searchFinishedMessage:
		a.query = msg.query
		a.results = msg.results
		a.searching = false
	}
	return tui.NoneEffect[message]()
}

func (*asyncSearch) Subscriptions() tui.Subscription[message] {
	return tui.NoneSubscription[message]()
}

func (a *asyncSearch) View(_ tui.ViewContext) tui.Node[message] {
	status := "Results"
	if a.searching {
		status = "Searching..."
	} else if len(a.results) == 0 {
		status = "No matches"
	}

	results := make([]tui.Node[message], 0, max(len(a.results), 1))
	for _, result := range a.results {
		results = append(results, tui.Text[message]("  "+result))
	}
	if len(results) == 0 {
		results = append(results, tui.Text[message]("  --"))
	}

	return tui.Border(
		tui.Column(
			tui.StyledText[message]("Async Search", vt.Style{Bold: true}).WithLength(tui.Fixed(1)),
			tui.TextInput(tui.NodeID("search-input"), a.query, func(query string) message {
				return message{kind: queryChangedMessage, query: query}
			}).WithLength(tui.Fixed(1)),
			tui.Text[message](status).WithLength(tui.Fixed(1)),
			tui.Column(results...).WithLength(tui.Flex(1)),
			tui.Text[message]("Type to search, click or Tab to restore focus, Esc to exit").WithLength(tui.Fixed(1)),
		),
		vt.Style{},
	)
}

func search(query string) tui.Effect[message] {
	return tui.LatestEffect(tui.NewTaskKey("search"), func(ctx context.Context) message {
		timer := time.NewTimer(searchDelay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
		}

		normalized := strings.ToLower(query)
		results := make([]string, 0, len(items))
		for _, item := range items {
			if strings.Contains(strings.ToLower(item), normalized) {
				results = append(results, item)
			}
		}
		return message{kind: searchFinishedMessage, query: query, results: results}
	})
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
	return tui.RunTerminal[message](&asyncSearch{}, options, mapEvent)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
