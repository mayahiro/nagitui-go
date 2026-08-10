package widget

import (
	"strings"
	"testing"

	"github.com/mayahiro/nagitui-go"
)

type choiceActionApp struct {
	widget   string
	state    bool
	enabled  bool
	keyMap   tui.KeyMap
	messages []string
}

func (*choiceActionApp) Init() tui.Effect[string] {
	return tui.NoneEffect[string]()
}

func (a *choiceActionApp) Update(message string) tui.Effect[string] {
	a.messages = append(a.messages, message)
	return tui.NoneEffect[string]()
}

func (*choiceActionApp) Subscriptions() tui.Subscription[string] {
	return tui.NoneSubscription[string]()
}

func (a *choiceActionApp) View(tui.ViewContext) tui.Node[string] {
	var choice tui.Node[string]
	switch a.widget {
	case "checkbox":
		choice = NewCheckbox(tui.NewNodeID("choice"), "Choice", a.state, func(value bool) string {
			if value {
				return "true"
			}
			return "false"
		}).Enabled(a.enabled).Node()
	case "radio":
		choice = NewRadio(tui.NewNodeID("choice"), "Choice", a.state, func() string {
			return "select"
		}).Enabled(a.enabled).Node()
	default:
		panic("unknown choice widget " + a.widget)
	}
	return tui.Padding(choice, tui.UniformInsets(0)).
		WithKeyScope(tui.NewKeyScope(tui.NewNodeID("root"), a.keyMap))
}

func TestChoiceActionsMatchSharedFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t,
		"widgets/choice-action.txt",
		"widget-choice-action",
		"widget", "state", "mode", "enabled", "event", "message", "consumed", "keys", "available",
	)
	for _, record := range records {
		enabled := fixtureBool(t, record.Field("enabled"))
		state := fixtureBool(t, record.Field("state"))
		keyMap := activationActionKeyMap(t, record.Field("mode"))
		app := &choiceActionApp{
			widget: record.Field("widget"), state: state, enabled: enabled, keyMap: keyMap,
		}
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

		var resolved tui.ResolvedActions
		if enabled {
			focused, err := runtime.RequestFocus(tui.NewNodeID("choice"))
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
				if group.Owner() == tui.NewNodeID("choice") {
					resolved = group
					break
				}
			}
		} else {
			focused, err := runtime.RequestFocus(tui.NewNodeID("choice"))
			if err != nil || focused {
				runtime.Close()
				t.Fatalf("case %s: disabled focus = %t, %v", record.ID, focused, err)
			}
			descriptor := choiceActionDescriptor(record.Field("widget"), state)
			resolved, err = tui.ResolveActions(
				tui.NewNodeID("choice"),
				[]tui.ActionDescriptor{descriptor},
				[]tui.KeyScope{tui.NewKeyScope(tui.NewNodeID("root"), keyMap)},
			)
			if err != nil {
				runtime.Close()
				t.Fatalf("case %s: %v", record.ID, err)
			}
		}
		actions := resolved.Actions()
		if len(actions) != 1 || actions[0].ID() != ActivateActionID {
			runtime.Close()
			t.Fatalf("case %s: actions = %#v", record.ID, actions)
		}
		bindings := actions[0].Bindings()
		keys := make([]string, len(bindings))
		for index, binding := range bindings {
			keys[index] = binding.Stroke().Notation()
		}
		if actual := fixtureList(keys); actual != record.Field("keys") {
			runtime.Close()
			t.Fatalf("case %s: keys = %q, want %q", record.ID, actual, record.Field("keys"))
		}
		if actual := actions[0].Availability() == tui.ActionEnabled; actual != fixtureBool(t, record.Field("available")) {
			runtime.Close()
			t.Fatalf("case %s: available = %t", record.ID, actual)
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
		expectedMessages := fixtureStringList(record.Field("message"))
		if strings.Join(app.messages, ",") != strings.Join(expectedMessages, ",") {
			t.Errorf("case %s: messages = %#v, want %#v", record.ID, app.messages, expectedMessages)
		}
		if dispatch.Messages() != len(expectedMessages) {
			t.Errorf("case %s: dispatched messages = %d", record.ID, dispatch.Messages())
		}
		if actual := dispatch.Consumed(); actual != fixtureBool(t, record.Field("consumed")) {
			t.Errorf("case %s: consumed = %t", record.ID, actual)
		}
		runtime.Close()
	}
}

func choiceActionDescriptor(widget string, state bool) tui.ActionDescriptor {
	switch widget {
	case "checkbox":
		return NewCheckbox(tui.NewNodeID("choice"), "Choice", state, func(bool) string {
			return "change"
		}).Enabled(false).ActionDescriptor()
	case "radio":
		return NewRadio(tui.NewNodeID("choice"), "Choice", state, func() string {
			return "select"
		}).Enabled(false).ActionDescriptor()
	default:
		panic("unknown choice widget " + widget)
	}
}

func fixtureStringList(value string) []string {
	if value == "-" {
		return nil
	}
	return strings.Split(value, ",")
}

func fixtureList(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ",")
}
