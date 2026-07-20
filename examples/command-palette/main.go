package main

import (
	"fmt"
	"os"
	"strings"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

var commands = []string{
	"Open file",
	"Save file",
	"Close editor",
	"Toggle sidebar",
	"Show shortcuts",
}

type messageKind uint8

const (
	insertMessage messageKind = iota
	backspaceMessage
	moveUpMessage
	moveDownMessage
)

type message struct {
	kind messageKind
	text string
}

type commandPalette struct {
	query    string
	selected int
}

func (*commandPalette) Init() tui.Effect[message] {
	return tui.NoneEffect[message]()
}

func (a *commandPalette) Update(msg message) tui.Effect[message] {
	switch msg.kind {
	case insertMessage:
		a.query += msg.text
		a.clampSelection()
	case backspaceMessage:
		if boundary, ok := celltext.PreviousGraphemeBoundary(a.query, len(a.query)); ok {
			a.query = a.query[:boundary]
			a.clampSelection()
		}
	case moveUpMessage:
		a.selected = max(a.selected-1, 0)
	case moveDownMessage:
		a.selected = min(a.selected+1, max(len(a.filtered())-1, 0))
	}
	return tui.NoneEffect[message]()
}

func (*commandPalette) Subscriptions() tui.Subscription[message] {
	return tui.NoneSubscription[message]()
}

func (a *commandPalette) View(_ tui.ViewContext) tui.Node[message] {
	filtered := a.filtered()
	items := make([]tui.Node[message], 0, len(filtered))
	for index, command := range filtered {
		marker := "  "
		if index == a.selected {
			marker = "> "
		}
		items = append(items, tui.StyledText[message](
			marker+command,
			vt.Style{Reverse: index == a.selected},
		).WithLength(tui.Fixed(1)))
	}
	if len(items) == 0 {
		items = append(items, tui.Text[message]("  No matching commands"))
	}

	return tui.Border(
		tui.Column(
			tui.StyledText[message]("Command Palette", vt.Style{Bold: true}).WithLength(tui.Fixed(1)),
			tui.Text[message]("> "+a.query).WithLength(tui.Fixed(1)),
			tui.Column(items...).WithLength(tui.Flex(1)),
			tui.Text[message]("Type to filter, arrows to move, Enter/Esc to close").WithLength(tui.Fixed(1)),
		),
		vt.Style{},
	)
}

func (a *commandPalette) filtered() []string {
	query := strings.ToLower(a.query)
	filtered := make([]string, 0, len(commands))
	for _, command := range commands {
		if strings.Contains(strings.ToLower(command), query) {
			filtered = append(filtered, command)
		}
	}
	return filtered
}

func (a *commandPalette) clampSelection() {
	a.selected = min(a.selected, max(len(a.filtered())-1, 0))
}

func mapEvent(event vt.Event) tui.EventAction[message] {
	switch {
	case event.Kind == vt.EventText:
		return tui.MessageAction(message{kind: insertMessage, text: event.Text})
	case event.Kind == vt.EventPaste:
		return tui.MessageAction(message{kind: insertMessage, text: event.Text})
	case event.Kind == vt.EventKey && event.Key.Code == vt.KeyBackspace:
		return tui.MessageAction(message{kind: backspaceMessage})
	case event.Kind == vt.EventKey && event.Key.Code == vt.KeyUp:
		return tui.MessageAction(message{kind: moveUpMessage})
	case event.Kind == vt.EventKey && event.Key.Code == vt.KeyDown:
		return tui.MessageAction(message{kind: moveDownMessage})
	case event.Kind == vt.EventKey && (event.Key.Code == vt.KeyEnter || event.Key.Code == vt.KeyEscape):
		return tui.ExitAction[message]()
	case event.Kind == vt.EventKey && event.Key.Code == vt.KeyCharacter && event.Key.Character == 'c' && event.Key.Modifiers.Control:
		return tui.ExitAction[message]()
	default:
		return tui.IgnoreAction[message]()
	}
}

func run() error {
	return tui.RunTerminal[message](&commandPalette{}, tui.DefaultTerminalOptions(), mapEvent)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
