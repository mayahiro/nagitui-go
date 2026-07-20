package main

import (
	"fmt"
	"os"
	"time"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/widget"
)

const spinnerInterval = 80 * time.Millisecond

type messageKind uint8

const (
	selectMessage messageKind = iota
	advanceMessage
	tickMessage
	openModalMessage
	closeModalMessage
)

type message struct {
	kind     messageKind
	selected int
}

type gallery struct {
	selected int
	progress uint64
	tick     uint64
	modal    bool
}

func (*gallery) Init() tui.Effect[message] {
	return tui.NoneEffect[message]()
}

func (a *gallery) Update(received message) tui.Effect[message] {
	switch received.kind {
	case selectMessage:
		a.selected = received.selected
	case advanceMessage:
		a.progress = (a.progress + 1) % 11
	case tickMessage:
		a.tick++
	case openModalMessage:
		a.modal = true
	case closeModalMessage:
		a.modal = false
	}
	return tui.NoneEffect[message]()
}

func (*gallery) Subscriptions() tui.Subscription[message] {
	return tui.EverySubscription(
		tui.NewSubscriptionKey("gallery-spinner"),
		spinnerInterval,
		tui.LatestDelivery(),
		func() message { return message{kind: tickMessage} },
	)
}

func (a *gallery) View(_ tui.ViewContext) tui.Node[message] {
	content := tui.Border(
		tui.Column(
			tui.StyledText[message]("Standard Widget Gallery", vt.Style{Bold: true}).WithLength(tui.Fixed(1)),
			widget.NewList(
				tui.NewNodeID("gallery-list"),
				[]widget.ListItem{
					widget.NewListItem(tui.NewNodeID("list-alpha"), "Alpha"),
					widget.NewListItem(tui.NewNodeID("list-beta"), "Beta"),
					widget.NewListItem(tui.NewNodeID("list-gamma"), "Gamma"),
				},
				a.selected,
				func(selected int) message { return message{kind: selectMessage, selected: selected} },
			).Node(),
			widget.NewProgress[message](a.progress, 10, 20).Node().WithLength(tui.Fixed(1)),
			widget.NewSpinner[message](a.tick).Label("Clock-driven spinner").Node().WithLength(tui.Fixed(1)),
			tui.Row(
				widget.NewButton(tui.NewNodeID("advance"), "Advance", func() message {
					return message{kind: advanceMessage}
				}).Node(),
				tui.Text[message](" "),
				widget.NewButton(tui.NewNodeID("open-modal"), "Open modal", func() message {
					return message{kind: openModalMessage}
				}).Node(),
			).WithLength(tui.Fixed(1)),
			tui.Text[message]("Tab/Shift-Tab focus, arrows select, Enter/Space activate, q exits").WithLength(tui.Fixed(1)),
		),
		vt.Style{},
	)
	if !a.modal {
		return content
	}
	return tui.Stack(
		content,
		widget.NewModal(
			tui.NewNodeID("gallery-modal"),
			tui.Column(
				tui.Text[message]("This panel owns focus and routing"),
				widget.NewButton(tui.NewNodeID("close-modal"), "Close", func() message {
					return message{kind: closeModalMessage}
				}).Node(),
			),
		).Title("Modal").OnEscape(func() message {
			return message{kind: closeModalMessage}
		}).Node(),
	)
}

func mapEvent(event vt.Event) tui.EventAction[message] {
	switch {
	case event.Kind == vt.EventText && event.Text == "q":
		return tui.ExitAction[message]()
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
	return tui.RunTerminal[message](&gallery{}, options, mapEvent)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
