package widget

import (
	"strings"
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

type buttonActionMessage uint8

const buttonActivated buttonActionMessage = iota

type buttonActionApp struct {
	enabled     bool
	keyMap      tui.KeyMap
	activations int
}

func (*buttonActionApp) Init() tui.Effect[buttonActionMessage] {
	return tui.NoneEffect[buttonActionMessage]()
}

func (a *buttonActionApp) Update(buttonActionMessage) tui.Effect[buttonActionMessage] {
	a.activations++
	return tui.NoneEffect[buttonActionMessage]()
}

func (*buttonActionApp) Subscriptions() tui.Subscription[buttonActionMessage] {
	return tui.NoneSubscription[buttonActionMessage]()
}

func (a *buttonActionApp) View(tui.ViewContext) tui.Node[buttonActionMessage] {
	button := NewButton(tui.NewNodeID("button"), "Run", func() buttonActionMessage {
		return buttonActivated
	}).Enabled(a.enabled).Node()
	return tui.Padding(button, tui.UniformInsets(0)).
		WithKeyScope(tui.NewKeyScope(tui.NewNodeID("root"), a.keyMap))
}

type resolvedHelpApp struct {
	actions []tui.ResolvedAction
}

func (*resolvedHelpApp) Init() tui.Effect[struct{}] {
	return tui.NoneEffect[struct{}]()
}

func (*resolvedHelpApp) Update(struct{}) tui.Effect[struct{}] {
	return tui.NoneEffect[struct{}]()
}

func (*resolvedHelpApp) Subscriptions() tui.Subscription[struct{}] {
	return tui.NoneSubscription[struct{}]()
}

func (a *resolvedHelpApp) View(tui.ViewContext) tui.Node[struct{}] {
	return NewHelpFromResolvedActions[struct{}](a.actions).
		Separator("|").
		ShowDisabled(true).
		Node()
}

func TestButtonActionsAndHelpMatchSharedFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t,
		"widgets/button-action.txt",
		"widget-button-action",
		"mode", "enabled", "event", "activate", "consumed", "help", "help-enabled",
	)
	for _, record := range records {
		enabled := fixtureBool(t, record.Field("enabled"))
		keyMap := activationActionKeyMap(t, record.Field("mode"))
		app := &buttonActionApp{enabled: enabled, keyMap: keyMap}
		runtime, err := tui.NewRuntimeWithClock(
			app,
			tui.NewRuntimeConfig(tui.Size{Width: 20, Height: 2}),
			tui.NewVirtualClock(),
		)
		if err != nil {
			t.Fatalf("case %s: %v", record.ID, err)
		}

		if _, err := runtime.RenderIfDirty(); err != nil {
			runtime.Close()
			t.Fatalf("case %s: %v", record.ID, err)
		}
		var actions []tui.ResolvedAction
		if enabled {
			focused, err := runtime.RequestFocus(tui.NewNodeID("button"))
			if err != nil || !focused {
				runtime.Close()
				t.Fatalf("case %s: focus = %t, %v", record.ID, focused, err)
			}
			groups, err := runtime.ActiveActionGroups()
			if err != nil {
				runtime.Close()
				t.Fatalf("case %s: %v", record.ID, err)
			}
			for _, group := range groups {
				if group.Owner() == tui.NewNodeID("button") {
					actions = group.Actions()
					break
				}
			}
			if len(actions) != 1 || actions[0].ID() != ActivateActionID {
				runtime.Close()
				t.Fatalf("case %s: actions = %#v", record.ID, actions)
			}
		} else {
			focused, err := runtime.RequestFocus(tui.NewNodeID("button"))
			if err != nil || focused {
				runtime.Close()
				t.Fatalf("case %s: disabled focus = %t, %v", record.ID, focused, err)
			}
			descriptor := NewButton(
				tui.NewNodeID("button"), "Run", func() buttonActionMessage { return buttonActivated },
			).Enabled(false).ActionDescriptor()
			resolved, err := tui.ResolveActions(
				tui.NewNodeID("button"),
				[]tui.ActionDescriptor{descriptor},
				[]tui.KeyScope{tui.NewKeyScope(tui.NewNodeID("root"), keyMap)},
			)
			if err != nil {
				runtime.Close()
				t.Fatalf("case %s: %v", record.ID, err)
			}
			actions = resolved.Actions()
		}

		dispatch, err := runtime.DispatchEvent(activationActionEvent(t, record.Field("event")))
		if err != nil {
			runtime.Close()
			t.Fatalf("case %s: %v", record.ID, err)
		}
		if _, err := runtime.ProcessPending(); err != nil {
			runtime.Close()
			t.Fatalf("case %s: %v", record.ID, err)
		}
		if actual := app.activations == 1; actual != fixtureBool(t, record.Field("activate")) {
			runtime.Close()
			t.Errorf("case %s: activation = %t", record.ID, actual)
		}
		if actual := dispatch.Consumed(); actual != fixtureBool(t, record.Field("consumed")) {
			runtime.Close()
			t.Errorf("case %s: consumed = %t", record.ID, actual)
		}
		runtime.Close()

		helpRuntime, err := tui.NewRuntimeWithClock(
			&resolvedHelpApp{actions: actions},
			tui.NewRuntimeConfig(tui.Size{Width: 64, Height: 1}),
			tui.NewVirtualClock(),
		)
		if err != nil {
			t.Fatalf("case %s: %v", record.ID, err)
		}
		frame, err := helpRuntime.RenderIfDirty()
		if err != nil {
			helpRuntime.Close()
			t.Fatalf("case %s: %v", record.ID, err)
		}
		actualHelp := strings.TrimRight(widgetRowText(frame.Surface(), 0), " ")
		if actualHelp != record.Field("help") {
			t.Errorf("case %s: help = %q, want %q", record.ID, actualHelp, record.Field("help"))
		}
		if record.Field("help-enabled") != "none" {
			cell, _ := frame.Surface().Cell(0, 0)
			if actual := !cell.Style().Dim; actual != fixtureBool(t, record.Field("help-enabled")) {
				t.Errorf("case %s: Help enabled = %t", record.ID, actual)
			}
		}
		helpRuntime.Close()
	}
}

func activationActionKeyMap(t *testing.T, mode string) tui.KeyMap {
	t.Helper()
	keyMap := tui.NewKeyMap()
	var (
		resolved tui.KeyMap
		err      error
	)
	switch mode {
	case "default":
		return keyMap
	case "rebind-x":
		resolved, err = keyMap.Rebind(ActivateActionID, []tui.KeyBinding{
			tui.NewKeyBinding(tui.NewCharacterKeyStroke('x', vt.Modifiers{})),
		})
	case "unbound":
		resolved, err = keyMap.Rebind(ActivateActionID, nil)
	default:
		t.Fatalf("unknown Button action mode %q", mode)
	}
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func activationActionEvent(t *testing.T, value string) vt.Event {
	t.Helper()
	keyboard := func(code vt.KeyCode, character rune, modifiers vt.Modifiers, action vt.KeyAction) vt.Event {
		return keyEvent(code, character, modifiers, action)
	}
	switch value {
	case "enter":
		return keyboard(vt.KeyEnter, 0, vt.Modifiers{}, vt.KeyPress)
	case "space-text":
		return vt.Event{Kind: vt.EventText, Text: " "}
	case "space-key":
		return keyboard(vt.KeyCharacter, ' ', vt.Modifiers{}, vt.KeyPress)
	case "repeat-enter":
		return keyboard(vt.KeyEnter, 0, vt.Modifiers{}, vt.KeyRepeat)
	case "repeat-space":
		return keyboard(vt.KeyCharacter, ' ', vt.Modifiers{}, vt.KeyRepeat)
	case "release-enter":
		return keyboard(vt.KeyEnter, 0, vt.Modifiers{}, vt.KeyRelease)
	case "control-enter":
		return keyboard(vt.KeyEnter, 0, vt.Modifiers{Control: true}, vt.KeyPress)
	case "control-space":
		return keyboard(vt.KeyCharacter, ' ', vt.Modifiers{Control: true}, vt.KeyPress)
	case "x":
		return keyboard(vt.KeyCharacter, 'x', vt.Modifiers{}, vt.KeyPress)
	case "mouse-left-press":
		return mouseEvent(vt.MousePress, vt.MouseLeft)
	default:
		t.Fatalf("unknown Button action event %q", value)
		return vt.Event{}
	}
}
