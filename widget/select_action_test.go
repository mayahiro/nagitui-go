package widget

import (
	"strconv"
	"strings"
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

type selectActionApp struct {
	count    int
	selected int
	enabled  bool
	keyMap   tui.KeyMap
	messages []int
}

func (*selectActionApp) Init() tui.Effect[int] {
	return tui.NoneEffect[int]()
}

func (a *selectActionApp) Update(message int) tui.Effect[int] {
	a.messages = append(a.messages, message)
	return tui.NoneEffect[int]()
}

func (*selectActionApp) Subscriptions() tui.Subscription[int] {
	return tui.NoneSubscription[int]()
}

func (a *selectActionApp) View(tui.ViewContext) tui.Node[int] {
	options := make([]string, a.count)
	for index := range options {
		options[index] = "Option " + strconv.Itoa(index)
	}
	selectNode := NewSelect(tui.NewNodeID("select"), options, a.selected, func(index int) int {
		return index
	}).Enabled(a.enabled).Node()
	return tui.Padding(selectNode, tui.UniformInsets(0)).
		WithKeyScope(tui.NewKeyScope(tui.NewNodeID("root"), a.keyMap))
}

func TestSelectActionsMatchSharedFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t,
		"widgets/select-action.txt",
		"widget-select-action",
		"count", "selected", "mode", "enabled", "event", "message", "consumed", "keys", "available",
	)
	for _, record := range records {
		count := fixtureInt(t, record.Field("count"))
		selected := fixtureInt(t, record.Field("selected"))
		enabled := fixtureBool(t, record.Field("enabled"))
		keyMap := selectActionKeyMap(t, record.Field("mode"))
		app := &selectActionApp{
			count: count, selected: selected, enabled: enabled, keyMap: keyMap,
		}
		runtime, err := tui.NewRuntimeWithClock(
			app,
			tui.NewRuntimeConfig(tui.Size{Width: 24, Height: 2}),
			tui.NewVirtualClock(),
		)
		if err != nil {
			t.Fatalf("case %s: %v", record.ID, err)
		}
		if _, err := runtime.RenderIfDirty(); err != nil {
			runtime.Close()
			t.Fatalf("case %s: %v", record.ID, err)
		}

		interactive := enabled && count != 0
		var resolved tui.ResolvedActions
		if interactive {
			focused, err := runtime.RequestFocus(tui.NewNodeID("select"))
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
				if group.Owner() == tui.NewNodeID("select") {
					resolved = group
					break
				}
			}
		} else {
			focused, err := runtime.RequestFocus(tui.NewNodeID("select"))
			if err != nil || focused {
				runtime.Close()
				t.Fatalf("case %s: disabled focus = %t, %v", record.ID, focused, err)
			}
			descriptors := selectDescriptors(count, selected, enabled)
			resolved, err = tui.ResolveActions(
				tui.NewNodeID("select"),
				descriptors,
				[]tui.KeyScope{tui.NewKeyScope(tui.NewNodeID("root"), keyMap)},
			)
			if err != nil {
				runtime.Close()
				t.Fatalf("case %s: %v", record.ID, err)
			}
		}

		expectedIDs := []tui.ActionID{
			ActivateActionID,
			SelectionPreviousActionID,
			SelectionNextActionID,
			SelectionFirstActionID,
			SelectionLastActionID,
		}
		expectedLabels := []string{"Activate", "Previous", "Next", "First", "Last"}
		expectedKeys := selectFixtureKeyGroups(record.Field("keys"))
		actions := resolved.Actions()
		if len(actions) != len(expectedIDs) || len(expectedKeys) != len(expectedIDs) {
			runtime.Close()
			t.Fatalf("case %s: actions = %d, key groups = %d", record.ID, len(actions), len(expectedKeys))
		}
		for index, action := range actions {
			if action.ID() != expectedIDs[index] || action.Label() != expectedLabels[index] {
				runtime.Close()
				t.Fatalf("case %s: action %d = %q %q", record.ID, index, action.ID(), action.Label())
			}
			bindings := action.Bindings()
			keys := make([]string, len(bindings))
			for bindingIndex, binding := range bindings {
				keys[bindingIndex] = binding.Stroke().Notation()
			}
			if strings.Join(keys, ",") != strings.Join(expectedKeys[index], ",") {
				runtime.Close()
				t.Fatalf("case %s action %s: keys = %#v", record.ID, expectedIDs[index], keys)
			}
			if available := action.Availability() == tui.ActionEnabled; available != fixtureBool(t, record.Field("available")) {
				runtime.Close()
				t.Fatalf("case %s action %s: available = %t", record.ID, expectedIDs[index], available)
			}
		}

		pointerEvent := record.Field("event") == "mouse-left-press"
		if pointerEvent && interactive {
			runtime.ClearFocus()
		}
		dispatch, err := runtime.DispatchEvent(selectActionEvent(t, record.Field("event")))
		if err != nil {
			runtime.Close()
			t.Fatalf("case %s: %v", record.ID, err)
		}
		if _, err := runtime.ProcessPending(); err != nil {
			runtime.Close()
			t.Fatalf("case %s: %v", record.ID, err)
		}
		expectedMessages := selectFixtureIntList(t, record.Field("message"))
		if strings.Join(intStrings(app.messages), ",") != strings.Join(intStrings(expectedMessages), ",") {
			t.Errorf("case %s: messages = %#v, want %#v", record.ID, app.messages, expectedMessages)
		}
		if dispatch.Messages() != len(expectedMessages) {
			t.Errorf("case %s: dispatched messages = %d", record.ID, dispatch.Messages())
		}
		if consumed := dispatch.Consumed(); consumed != fixtureBool(t, record.Field("consumed")) {
			t.Errorf("case %s: consumed = %t", record.ID, consumed)
		}
		if pointerEvent {
			focused, hasFocus := runtime.Interaction().Focused()
			if actual := hasFocus && focused == tui.NewNodeID("select"); actual != interactive {
				t.Errorf("case %s: pointer focus = %t %q", record.ID, hasFocus, focused)
			}
		}
		runtime.Close()
	}
}

func selectDescriptors(count, selected int, enabled bool) []tui.ActionDescriptor {
	options := make([]string, count)
	return NewSelect(tui.NewNodeID("select"), options, selected, func(index int) int {
		return index
	}).Enabled(enabled).ActionDescriptors()
}

func selectActionKeyMap(t *testing.T, mode string) tui.KeyMap {
	t.Helper()
	keyMap := tui.NewKeyMap()
	var (
		resolved tui.KeyMap
		err      error
	)
	switch mode {
	case "default":
		return keyMap
	case "activate-x":
		resolved, err = keyMap.Rebind(ActivateActionID, []tui.KeyBinding{
			tui.NewKeyBinding(tui.NewCharacterKeyStroke('x', vt.Modifiers{})),
		})
	case "previous-k":
		resolved, err = keyMap.Rebind(SelectionPreviousActionID, []tui.KeyBinding{
			tui.NewKeyBinding(tui.NewCharacterKeyStroke('k', vt.Modifiers{})),
		})
	case "unbind-next":
		resolved, err = keyMap.Rebind(SelectionNextActionID, nil)
	default:
		t.Fatalf("unknown Select action mode %q", mode)
	}
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func selectActionEvent(t *testing.T, value string) vt.Event {
	t.Helper()
	keyboard := func(code vt.KeyCode, character rune, modifiers vt.Modifiers, action vt.KeyAction) vt.Event {
		return keyEvent(code, character, modifiers, action)
	}
	switch value {
	case "enter":
		return keyboard(vt.KeyEnter, 0, vt.Modifiers{}, vt.KeyPress)
	case "space-text":
		return vt.Event{Kind: vt.EventText, Text: " "}
	case "left":
		return keyboard(vt.KeyLeft, 0, vt.Modifiers{}, vt.KeyPress)
	case "up":
		return keyboard(vt.KeyUp, 0, vt.Modifiers{}, vt.KeyPress)
	case "right":
		return keyboard(vt.KeyRight, 0, vt.Modifiers{}, vt.KeyPress)
	case "down":
		return keyboard(vt.KeyDown, 0, vt.Modifiers{}, vt.KeyPress)
	case "home":
		return keyboard(vt.KeyHome, 0, vt.Modifiers{}, vt.KeyPress)
	case "end":
		return keyboard(vt.KeyEnd, 0, vt.Modifiers{}, vt.KeyPress)
	case "repeat-down":
		return keyboard(vt.KeyDown, 0, vt.Modifiers{}, vt.KeyRepeat)
	case "release-down":
		return keyboard(vt.KeyDown, 0, vt.Modifiers{}, vt.KeyRelease)
	case "shift-down":
		return keyboard(vt.KeyDown, 0, vt.Modifiers{Shift: true}, vt.KeyPress)
	case "control-enter":
		return keyboard(vt.KeyEnter, 0, vt.Modifiers{Control: true}, vt.KeyPress)
	case "x":
		return keyboard(vt.KeyCharacter, 'x', vt.Modifiers{}, vt.KeyPress)
	case "k":
		return keyboard(vt.KeyCharacter, 'k', vt.Modifiers{}, vt.KeyPress)
	case "mouse-left-press":
		return mouseEvent(vt.MousePress, vt.MouseLeft)
	default:
		t.Fatalf("unknown Select action event %q", value)
		return vt.Event{}
	}
}

func selectFixtureKeyGroups(value string) [][]string {
	groups := strings.Split(value, "|")
	result := make([][]string, len(groups))
	for index, group := range groups {
		if group != "-" {
			result[index] = strings.Split(group, ",")
		}
	}
	return result
}

func selectFixtureIntList(t *testing.T, value string) []int {
	t.Helper()
	if value == "-" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]int, len(parts))
	for index, part := range parts {
		result[index] = fixtureInt(t, part)
	}
	return result
}

func intStrings(values []int) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = strconv.Itoa(value)
	}
	return result
}
