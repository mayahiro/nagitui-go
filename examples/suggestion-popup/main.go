package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/widget"
)

const searchDelay = 120 * time.Millisecond

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
	editMessage messageKind = iota
	searchFinishedMessage
	selectMessage
	acceptMessage
	dismissMessage
	submitMessage
)

type message struct {
	kind       messageKind
	composer   widget.ComposerState
	query      string
	candidates widget.SuggestionItems
	err        error
	id         widget.SuggestionID
}

type suggestionExample struct {
	composer    widget.ComposerState
	candidates  widget.SuggestionItems
	selected    widget.SuggestionID
	hasSelected bool
	searching   bool
	popupOpen   bool
	accepted    string
	hasAccepted bool
	searchError string
}

func (a *suggestionExample) Init() tui.Effect[message] {
	a.searching = true
	a.popupOpen = true
	return search("")
}

func (a *suggestionExample) Update(msg message) tui.Effect[message] {
	switch msg.kind {
	case editMessage:
		a.composer = msg.composer
		a.selected, a.hasSelected = widget.SuggestionID{}, false
		a.searching = true
		a.popupOpen = true
		a.searchError = ""
		return search(msg.composer.TextArea().Value())
	case searchFinishedMessage:
		a.candidates = msg.candidates
		a.searching = false
		if msg.err != nil {
			a.searchError = msg.err.Error()
		}
	case selectMessage:
		a.selected, a.hasSelected = msg.id, true
	case acceptMessage:
		a.composer = widget.NewComposerStateAtEnd(msg.id.String())
		a.accepted, a.hasAccepted = msg.id.String(), true
		a.popupOpen = false
	case dismissMessage:
		a.popupOpen = false
	case submitMessage:
		a.accepted, a.hasAccepted = a.composer.TextArea().Value(), true
		a.popupOpen = false
	}
	return tui.NoneEffect[message]()
}

func (*suggestionExample) Subscriptions() tui.Subscription[message] {
	return tui.NoneSubscription[message]()
}

func (a *suggestionExample) View(tui.ViewContext) tui.Node[message] {
	composer := widget.NewComposer(
		tui.NewNodeID("composer"),
		tui.NewNodeID("composer-viewport"),
		tui.NewNodeID("composer-caret"),
		a.composer,
		func(state widget.ComposerState) message {
			return message{kind: editMessage, composer: state}
		},
		func() message { return message{kind: submitMessage} },
	).Rows(1, 3).Node()
	status := widget.SuggestionPopupReady
	if a.searching {
		status = widget.SuggestionPopupLoading
	}
	popup := widget.NewSuggestionPopup(
		tui.NewNodeID("suggestions"),
		composer,
		tui.NewNodeID("composer-caret"),
		tui.NewNodeID("composer"),
		a.candidates,
		a.selected,
		a.hasSelected,
		func(context widget.SuggestionRowContext) tui.Node[message] {
			prefix := "  "
			if context.Selected() {
				prefix = "> "
			}
			return tui.Text[message](prefix + context.ID().String())
		},
		func(id widget.SuggestionID) message { return message{kind: selectMessage, id: id} },
		func(id widget.SuggestionID) message { return message{kind: acceptMessage, id: id} },
		func() message { return message{kind: dismissMessage} },
	).Status(status).Open(a.popupOpen).VisibleRows(5)
	if a.searchError != "" {
		popup = popup.Empty(tui.Text[message]("Candidate error: " + a.searchError))
	}
	suggestions := popup.Node().WithLength(tui.Flex(1))

	accepted := "Accepted: --"
	if a.hasAccepted {
		accepted = "Accepted: " + a.accepted
	}
	return tui.Border(
		tui.Column(
			tui.StyledText[message]("SuggestionPopup", vt.Style{Bold: true}).WithLength(tui.Fixed(1)),
			suggestions,
			tui.Text[message](accepted).WithLength(tui.Fixed(1)),
			tui.Text[message]("Type to search, arrows select, Enter accepts, Esc closes").WithLength(tui.Fixed(1)),
		),
		vt.Style{},
	)
}

func search(query string) tui.Effect[message] {
	return tui.LatestEffect(tui.NewTaskKey("suggestions"), func(ctx context.Context) message {
		timer := time.NewTimer(searchDelay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
		}
		normalized := strings.ToLower(query)
		candidateIDs := make([]widget.SuggestionID, 0, len(items))
		for _, item := range items {
			if strings.Contains(strings.ToLower(item), normalized) {
				candidateIDs = append(candidateIDs, widget.NewSuggestionID(item))
			}
		}
		candidates, err := widget.NewSuggestionItems(candidateIDs)
		return message{kind: searchFinishedMessage, query: query, candidates: candidates, err: err}
	})
}

func mapEvent(event vt.Event) tui.EventAction[message] {
	if event.Kind == vt.EventKey && event.Key.Code == vt.KeyCharacter && event.Key.Character == 'c' && event.Key.Modifiers.Control {
		return tui.ExitAction[message]()
	}
	return tui.IgnoreAction[message]()
}

func run() error {
	options := tui.DefaultTerminalOptions()
	options.FocusFirst = true
	mouseTracking := vt.MouseTrackingPress
	options.MouseTracking = &mouseTracking
	return tui.RunTerminal[message](&suggestionExample{}, options, mapEvent)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
