// Command inline-terminal demonstrates a bounded main-screen Nagi TUI that
// leaves its final frame in terminal history
package main

import (
	"fmt"
	"log"

	"github.com/mayahiro/nagi-go/vt"
	tui "github.com/mayahiro/nagitui-go"
)

type message uint8

const (
	increment message = iota
	quit
)

type inlineApp struct {
	count   uint64
	exiting bool
}

func (*inlineApp) Init() tui.Effect[message] {
	return tui.NoneEffect[message]()
}

func (a *inlineApp) Update(msg message) tui.Effect[message] {
	switch msg {
	case increment:
		a.count++
	case quit:
		a.exiting = true
		return tui.ExitEffect[message]()
	}
	return tui.NoneEffect[message]()
}

func (*inlineApp) Subscriptions() tui.Subscription[message] {
	return tui.NoneSubscription[message]()
}

func (a *inlineApp) View(context tui.ViewContext) tui.Node[message] {
	status := "Press Escape to stop"
	if a.exiting {
		status = "Stopped; this frame remains in terminal history"
	}
	return tui.Column(
		tui.Text[message](fmt.Sprintf("Inline count: %d", a.count)),
		tui.Text[message]("Press Enter to increment"),
		tui.Text[message](status),
		tui.Text[message](fmt.Sprintf("Viewport: %d x %d", context.Size.Width, context.Size.Height)),
	)
}

func main() {
	viewport, err := tui.NewInlineTerminalViewport(4)
	if err != nil {
		log.Fatal(err)
	}
	options := tui.DefaultTerminalOptions()
	options.Viewport = viewport
	err = tui.RunTerminal(
		&inlineApp{},
		options,
		func(event vt.Event) tui.EventAction[message] {
			switch {
			case event.Kind == vt.EventKey && event.Key.Code == vt.KeyEnter:
				return tui.MessageAction(increment)
			case event.Kind == vt.EventKey && event.Key.Code == vt.KeyEscape:
				return tui.MessageAction(quit)
			default:
				return tui.IgnoreAction[message]()
			}
		},
	)
	if err != nil {
		log.Fatal(fmt.Errorf("run inline terminal example: %w", err))
	}
}
