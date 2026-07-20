// Command counter is a minimal stateful Nagi TUI application
package main

import (
	"fmt"
	"log"

	"github.com/mayahiro/nagi-go/vt"
	tui "github.com/mayahiro/nagitui-go"
)

type message struct {
	increment bool
	quit      bool
}

type counter struct {
	count   uint64
	exiting bool
}

func (*counter) Init() tui.Effect[message] {
	return tui.NoneEffect[message]()
}

func (a *counter) Update(msg message) tui.Effect[message] {
	if msg.quit {
		a.exiting = true
		return tui.ExitEffect[message]()
	}
	if msg.increment {
		a.count++
	}
	return tui.NoneEffect[message]()
}

func (*counter) Subscriptions() tui.Subscription[message] {
	return tui.NoneSubscription[message]()
}

func (a *counter) View(_ tui.ViewContext) tui.Node[message] {
	status := "Running"
	if a.exiting {
		status = "Stopping"
	}
	return tui.Panel(
		tui.Column(
			tui.Text[message](fmt.Sprintf("Count: %d", a.count)),
			tui.Text[message]("Status: "+status),
			tui.Text[message]("Press Enter to increment, Escape to exit"),
		),
		"Counter",
	)
}

func main() {
	err := tui.RunTerminal(
		&counter{},
		tui.DefaultTerminalOptions(),
		func(event vt.Event) tui.EventAction[message] {
			switch {
			case event.Kind == vt.EventKey && event.Key.Code == vt.KeyEnter:
				return tui.MessageAction(message{increment: true})
			case event.Kind == vt.EventKey && event.Key.Code == vt.KeyEscape:
				return tui.MessageAction(message{quit: true})
			default:
				return tui.IgnoreAction[message]()
			}
		},
	)
	if err != nil {
		log.Fatal(err)
	}
}
