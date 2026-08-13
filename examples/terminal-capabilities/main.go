// Command terminal-capabilities inspects detected terminal capabilities
package main

import (
	"fmt"
	"log"

	"github.com/mayahiro/nagi-go/vt"
	tui "github.com/mayahiro/nagitui-go"
)

type message struct {
	input string
	quit  bool
}

type capabilityDemo struct {
	lastInput string
	exiting   bool
}

func (*capabilityDemo) Init() tui.Effect[message] {
	return tui.NoneEffect[message]()
}

func (a *capabilityDemo) Update(msg message) tui.Effect[message] {
	if msg.quit {
		a.exiting = true
		return tui.ExitEffect[message]()
	}
	if msg.input != "" {
		a.lastInput = msg.input
	}
	return tui.NoneEffect[message]()
}

func (*capabilityDemo) Subscriptions() tui.Subscription[message] {
	return tui.NoneSubscription[message]()
}

func (a *capabilityDemo) View(context tui.ViewContext) tui.Node[message] {
	profile := context.TerminalCapabilities
	status := "Running"
	if a.exiting {
		status = "Stopping"
	}
	lastInput := a.lastInput
	if lastInput == "" {
		lastInput = "None"
	}
	return tui.Panel(
		tui.Column(
			tui.Text[message]("Color: "+colorLevelName(profile.ColorLevel())),
			tui.Text[message](fmt.Sprintf("NO_COLOR preference: %t", profile.PrefersNoColor())),
			tui.Text[message]("Hyperlinks: "+featureSupportName(profile.Hyperlinks())),
			tui.Text[message]("Clipboard: "+featureSupportName(profile.Clipboard())),
			tui.Text[message]("Extended keyboard: "+featureSupportName(profile.ExtendedKeyboard())),
			tui.Text[message]("Keyboard protocol: "+keyboardProtocolName(profile.KeyboardProtocol())),
			tui.Text[message]("Last input: "+lastInput),
			tui.Text[message]("Status: "+status),
			tui.Text[message]("Press Enter or Shift+Enter, Escape to exit"),
		),
		"Terminal capabilities",
	)
}

func colorLevelName(level tui.TerminalColorLevel) string {
	switch level {
	case tui.TerminalColorMonochrome:
		return "Monochrome"
	case tui.TerminalColorANSI16:
		return "ANSI16"
	case tui.TerminalColorIndexed256:
		return "Indexed256"
	case tui.TerminalColorTrueColor:
		return "TrueColor"
	default:
		return "Unknown"
	}
}

func featureSupportName(support tui.TerminalFeatureSupport) string {
	switch support {
	case tui.TerminalFeatureUnsupported:
		return "Unsupported"
	case tui.TerminalFeatureSupported:
		return "Supported"
	default:
		return "Unknown"
	}
}

func keyboardProtocolName(protocol tui.TerminalKeyboardProtocol) string {
	if protocol == tui.TerminalKeyboardKitty {
		return "Kitty"
	}
	return "Legacy"
}

func main() {
	options := tui.DefaultTerminalOptions()
	options.CapabilityDetection = tui.TerminalCapabilityDetectionEnabled
	err := tui.RunTerminal(
		&capabilityDemo{},
		options,
		func(event vt.Event) tui.EventAction[message] {
			if event.Kind != vt.EventKey || event.Key.Action == vt.KeyRelease {
				return tui.IgnoreAction[message]()
			}
			switch {
			case event.Key.Code == vt.KeyEnter && event.Key.Modifiers.Shift:
				return tui.MessageAction(message{input: "Shift+Enter via " + keyboardProtocolNameFromVT(event.Key.Protocol)})
			case event.Key.Code == vt.KeyEnter:
				return tui.MessageAction(message{input: "Enter via " + keyboardProtocolNameFromVT(event.Key.Protocol)})
			case event.Key.Code == vt.KeyEscape:
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

func keyboardProtocolNameFromVT(protocol vt.KeyProtocol) string {
	if protocol == vt.KeyProtocolKitty {
		return "Kitty"
	}
	return "Legacy"
}
