// Command terminal-suspend demonstrates an application-owned child process
// running while the Nagi full-screen terminal session is suspended
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/mayahiro/nagi-go/vt"
	tui "github.com/mayahiro/nagitui-go"
)

type message struct {
	openShell bool
	returned  string
	quit      bool
}

type terminalSuspendApp struct {
	status  string
	exiting bool
}

func (*terminalSuspendApp) Init() tui.Effect[message] {
	return tui.NoneEffect[message]()
}

func (a *terminalSuspendApp) Update(msg message) tui.Effect[message] {
	switch {
	case msg.quit:
		a.exiting = true
		return tui.ExitEffect[message]()
	case msg.openShell:
		a.status = "Opening the application-owned shell"
		return tui.SuspendTerminalEffect(func(ctx context.Context) message {
			shell := os.Getenv("SHELL")
			if shell == "" {
				shell = "/bin/sh"
			}
			err := exec.CommandContext(ctx, shell).Run()
			if err != nil {
				return message{returned: "Shell failed: " + err.Error()}
			}
			return message{returned: "Shell returned successfully"}
		})
	case msg.returned != "":
		a.status = msg.returned
	}
	return tui.NoneEffect[message]()
}

func (*terminalSuspendApp) Subscriptions() tui.Subscription[message] {
	return tui.NoneSubscription[message]()
}

func (a *terminalSuspendApp) View(tui.ViewContext) tui.Node[message] {
	status := a.status
	if status == "" {
		status = "Shell has not run"
	}
	lifecycle := "Running"
	if a.exiting {
		lifecycle = "Stopping"
	}
	return tui.Panel(
		tui.Column(
			tui.Text[message]("Status: "+status),
			tui.Text[message]("Lifecycle: "+lifecycle),
			tui.Text[message]("Press S to open $SHELL, then exit the shell to resume"),
			tui.Text[message]("Press Escape to exit"),
		),
		"Terminal suspend / resume",
	)
}

func main() {
	err := tui.RunTerminal(
		&terminalSuspendApp{},
		tui.DefaultTerminalOptions(),
		func(event vt.Event) tui.EventAction[message] {
			switch {
			case event.Kind == vt.EventText && (event.Text == "s" || event.Text == "S"):
				return tui.MessageAction(message{openShell: true})
			case event.Kind == vt.EventKey && event.Key.Code == vt.KeyEscape:
				return tui.MessageAction(message{quit: true})
			default:
				return tui.IgnoreAction[message]()
			}
		},
	)
	if err != nil {
		log.Fatal(fmt.Errorf("run terminal suspend example: %w", err))
	}
}
