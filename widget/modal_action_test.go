package widget

import (
	"slices"
	"strings"
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

type modalActionApp struct {
	handler     bool
	child       string
	outer       string
	keyMap      tui.KeyMap
	propagation tui.KeyScopePropagation
	messages    []string
}

func (*modalActionApp) Init() tui.Effect[string] {
	return tui.NoneEffect[string]()
}

func (a *modalActionApp) Update(message string) tui.Effect[string] {
	a.messages = append(a.messages, message)
	return tui.NoneEffect[string]()
}

func (*modalActionApp) Subscriptions() tui.Subscription[string] {
	return tui.NoneSubscription[string]()
}

func (a *modalActionApp) View(tui.ViewContext) tui.Node[string] {
	modal := NewModal(tui.NewNodeID("modal"), modalActionChild(a.child))
	if a.handler {
		modal = modal.OnEscape(func() string { return "dismiss" })
	}
	node := modal.Node().WithKeyScope(
		tui.NewKeyScope(tui.NewNodeID("modal"), a.keyMap).WithPropagation(a.propagation),
	)
	root := tui.Padding(node, tui.UniformInsets(0))
	switch a.outer {
	case "none":
		return root
	case "action":
		return root.OnActions(tui.NewNodeID("root"), []tui.Action[string]{
			tui.NewAction(
				modalFixtureActionDescriptor(tui.NewActionID("app.outer"), "Outer"),
				func(tui.ActionEvent) tui.EventResult[string] { return tui.MessageResult("outer-action") },
			),
		})
	case "raw":
		return root.OnEvent(tui.NewNodeID("root"), func(vt.Event) tui.EventResult[string] {
			return tui.MessageResult("outer-raw")
		})
	default:
		panic("unknown Modal outer behavior " + a.outer)
	}
}

func TestModalActionsMatchSharedFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t,
		"widgets/modal-action.txt",
		"widget-modal-action",
		"handler", "focus", "mode", "propagation", "child", "outer", "event",
		"messages", "consumed", "owners", "keys", "available",
	)
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			app := &modalActionApp{
				handler:     fixtureBool(t, record.Field("handler")),
				child:       record.Field("child"),
				outer:       record.Field("outer"),
				keyMap:      modalActionKeyMap(t, record.Field("mode")),
				propagation: modalFixturePropagation(record.Field("propagation")),
			}
			runtime, err := tui.NewRuntimeWithClock(
				app,
				tui.NewRuntimeConfig(tui.Size{Width: 24, Height: 4}),
				tui.NewVirtualClock(),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer runtime.Close()
			if _, err := runtime.RenderIfDirty(); err != nil {
				t.Fatal(err)
			}
			if record.Field("focus") == "child" {
				focused, err := runtime.RequestFocus(tui.NewNodeID("child"))
				if err != nil || !focused {
					t.Fatalf("focus = %t, %v", focused, err)
				}
			}

			groups, err := runtime.ActiveActionGroups()
			if err != nil {
				t.Fatal(err)
			}
			owners := make([]string, len(groups))
			var modalGroup *tui.ResolvedActions
			for index := range groups {
				owners[index] = groups[index].Owner().String()
				if groups[index].Owner() == tui.NewNodeID("modal") {
					modalGroup = &groups[index]
				}
			}
			if expected := modalFixtureStrings(record.Field("owners")); !slices.Equal(owners, expected) {
				t.Fatalf("owners = %#v, want %#v", owners, expected)
			}
			if modalGroup == nil {
				t.Fatal("Modal action group is missing")
			}
			actions := modalGroup.Actions()
			if len(actions) != 1 {
				t.Fatalf("Modal actions = %d, want 1", len(actions))
			}
			dismiss := actions[0]
			if dismiss.ID() != DismissActionID || dismiss.Label() != "Dismiss" {
				t.Fatalf("dismiss action = %q %q", dismiss.ID(), dismiss.Label())
			}
			bindings := dismiss.Bindings()
			keys := make([]string, len(bindings))
			for index, binding := range bindings {
				keys[index] = binding.Stroke().Notation()
			}
			if expected := modalFixtureStrings(record.Field("keys")); !slices.Equal(keys, expected) {
				t.Errorf("keys = %#v, want %#v", keys, expected)
			}
			expectedAvailability := tui.ActionDisabledPassThrough
			if fixtureBool(t, record.Field("available")) {
				expectedAvailability = tui.ActionEnabled
			}
			if dismiss.Availability() != expectedAvailability {
				t.Errorf("availability = %d, want %d", dismiss.Availability(), expectedAvailability)
			}

			dispatch, err := runtime.DispatchEvent(modalActionEvent(record.Field("event")))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runtime.ProcessPending(); err != nil {
				t.Fatal(err)
			}
			expectedMessages := modalFixtureStrings(record.Field("messages"))
			if !slices.Equal(app.messages, expectedMessages) {
				t.Errorf("messages = %#v, want %#v", app.messages, expectedMessages)
			}
			if dispatch.Messages() != len(expectedMessages) {
				t.Errorf("dispatched messages = %d, want %d", dispatch.Messages(), len(expectedMessages))
			}
			if dispatch.Consumed() != fixtureBool(t, record.Field("consumed")) {
				t.Errorf("consumed = %t", dispatch.Consumed())
			}
		})
	}
}

func modalActionChild(behavior string) tui.Node[string] {
	child := tui.Text[string]("child").Focusable(tui.NewNodeID("child"))
	switch behavior {
	case "none":
		return child
	case "action-consume":
		return child.OnActions(tui.NewNodeID("child"), []tui.Action[string]{
			tui.NewAction(
				modalFixtureActionDescriptor(tui.NewActionID("app.child"), "Child"),
				func(tui.ActionEvent) tui.EventResult[string] { return tui.MessageResult("child-action") },
			),
		})
	case "action-ignore":
		return child.OnActions(tui.NewNodeID("child"), []tui.Action[string]{
			tui.NewAction(
				modalFixtureActionDescriptor(tui.NewActionID("app.child"), "Child"),
				func(tui.ActionEvent) tui.EventResult[string] {
					return tui.IgnoreResult[string]().Emit("child-action")
				},
			),
		})
	case "raw-consume":
		return child.OnEvent(tui.NewNodeID("child"), func(vt.Event) tui.EventResult[string] {
			return tui.MessageResult("child-raw")
		})
	case "raw-ignore":
		return child.OnEvent(tui.NewNodeID("child"), func(vt.Event) tui.EventResult[string] {
			return tui.IgnoreResult[string]().Emit("child-raw")
		})
	default:
		panic("unknown Modal child behavior " + behavior)
	}
}

func modalFixtureActionDescriptor(id tui.ActionID, label string) tui.ActionDescriptor {
	return tui.NewActionDescriptor(id, label, []tui.KeyBinding{
		tui.NewKeyBinding(tui.NewKeyStroke(vt.KeyEscape, vt.Modifiers{})),
	})
}

func modalActionKeyMap(t *testing.T, mode string) tui.KeyMap {
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
		resolved, err = keyMap.Rebind(DismissActionID, []tui.KeyBinding{
			tui.NewKeyBinding(tui.NewCharacterKeyStroke('x', vt.Modifiers{})),
		})
	case "unbound":
		resolved, err = keyMap.Rebind(DismissActionID, nil)
	default:
		t.Fatalf("unknown Modal action mode %q", mode)
	}
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func modalFixturePropagation(value string) tui.KeyScopePropagation {
	switch value {
	case "continue":
		return tui.KeyScopeContinue
	case "stop":
		return tui.KeyScopeStopAtScope
	default:
		panic("unknown Modal propagation " + value)
	}
}

func modalActionEvent(value string) vt.Event {
	keyboard := func(modifiers vt.Modifiers, action vt.KeyAction) vt.Event {
		return vt.Event{
			Kind: vt.EventKey,
			Key: vt.KeyEvent{
				Code: vt.KeyEscape, Modifiers: modifiers, Action: action,
			},
		}
	}
	switch value {
	case "escape":
		return keyboard(vt.Modifiers{}, vt.KeyPress)
	case "repeat-escape":
		return keyboard(vt.Modifiers{}, vt.KeyRepeat)
	case "unknown-escape":
		return keyboard(vt.Modifiers{}, vt.KeyActionUnknown)
	case "release-escape":
		return keyboard(vt.Modifiers{}, vt.KeyRelease)
	case "shift-escape":
		return keyboard(vt.Modifiers{Shift: true}, vt.KeyPress)
	case "control-escape":
		return keyboard(vt.Modifiers{Control: true}, vt.KeyPress)
	case "alt-escape":
		return keyboard(vt.Modifiers{Alt: true}, vt.KeyPress)
	case "meta-escape":
		return keyboard(vt.Modifiers{Meta: true}, vt.KeyPress)
	case "x":
		return vt.Event{
			Kind: vt.EventKey,
			Key:  vt.KeyEvent{Code: vt.KeyCharacter, Character: 'x', Action: vt.KeyPress},
		}
	default:
		panic("unknown Modal action event " + value)
	}
}

func modalFixtureStrings(value string) []string {
	if value == "-" {
		return nil
	}
	return strings.Split(value, ",")
}
