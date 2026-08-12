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
	confirmModalMessage
	closeModalMessage
	toggleModalDetailsMessage
)

type message struct {
	kind     messageKind
	selected int
	value    bool
}

type gallery struct {
	selected     int
	progress     uint64
	tick         uint64
	modal        bool
	modalDetails bool
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
		a.modalDetails = false
	case confirmModalMessage:
		a.progress = (a.progress + 1) % 11
		a.modal = false
	case closeModalMessage:
		a.modal = false
	case toggleModalDetailsMessage:
		a.modalDetails = received.value
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

func (a *gallery) View(viewContext tui.ViewContext) tui.Node[message] {
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
				widget.NewButton(tui.NewNodeID("open-modal"), "Open confirm", func() message {
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
	details := widget.NewDisclosure(
		tui.NewNodeID("confirm-details"),
		tui.Text[message]("What changes?"),
		a.modalDetails,
		func(expanded bool) message {
			return message{kind: toggleModalDetailsMessage, value: expanded}
		},
	).Body(func() tui.Node[message] {
		return tui.Text[message]("Confirming advances progress by one step")
	})
	dialog := widget.NewConfirmDialog(
		tui.NewNodeID("gallery-dialog"),
		tui.Text[message]("Advance the progress indicator?"),
		widget.NewDialogAction(tui.NewNodeID("confirm-advance"), "Advance", func() message {
			return message{kind: confirmModalMessage}
		}),
		widget.NewDialogAction(tui.NewNodeID("cancel-advance"), "Cancel", func() message {
			return message{kind: closeModalMessage}
		}),
		widget.ConfirmDialogDefaultCancel(),
	).Title(tui.StyledText[message]("Confirm action", vt.Style{Bold: true})).
		Details(details).
		WidthProfile(viewContext.WidthProfile).
		ActionWrapWidth(max(viewContext.Size.Width, 5) - 4).
		Node()
	return tui.Stack(content, dialog)
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
