package widget

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

type tabsActionApp struct {
	count    int
	selected int
	enabled  bool
	keyMap   tui.KeyMap
	messages []int
}

func (*tabsActionApp) Init() tui.Effect[int] {
	return tui.NoneEffect[int]()
}

func (a *tabsActionApp) Update(message int) tui.Effect[int] {
	a.messages = append(a.messages, message)
	return tui.NoneEffect[int]()
}

func (*tabsActionApp) Subscriptions() tui.Subscription[int] {
	return tui.NoneSubscription[int]()
}

func (a *tabsActionApp) View(tui.ViewContext) tui.Node[int] {
	tabs := fixtureTabs(a.count, a.selected, a.enabled).Node()
	return tui.Padding(tabs, tui.UniformInsets(0)).
		WithKeyScope(tui.NewKeyScope(tui.NewNodeID("scope"), a.keyMap))
}

func TestTabsActionsMatchSharedFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t,
		"widgets/tabs-action.txt",
		"widget-tabs-action",
		"count", "selected", "focus", "mode", "enabled", "event", "message", "consumed",
		"focus-after", "item-keys", "root-keys", "available",
	)
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			count := fixtureInt(t, record.Field("count"))
			selected := fixtureInt(t, record.Field("selected"))
			enabled := fixtureBool(t, record.Field("enabled"))
			keyMap := tabsActionKeyMap(t, record.Field("mode"))
			app := &tabsActionApp{
				count: count, selected: selected, enabled: enabled, keyMap: keyMap,
			}
			runtime, err := tui.NewRuntimeWithClock(
				app,
				tui.NewRuntimeConfig(tui.Size{Width: 24, Height: 2}),
				tui.NewVirtualClock(),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer runtime.Close()
			if _, err := runtime.RenderIfDirty(); err != nil {
				t.Fatal(err)
			}

			interactive := enabled && count != 0
			var itemActions, rootActions tui.ResolvedActions
			if interactive {
				inspectionIndex, hasFocus := tabsFixtureOptionalInt(t, record.Field("focus"))
				if !hasFocus {
					inspectionIndex = min(max(selected, 0), count-1)
				}
				focused, err := runtime.RequestFocus(tabID(inspectionIndex))
				if err != nil || !focused {
					t.Fatalf("inspection focus = %t, %v", focused, err)
				}
				groups, err := runtime.ActiveActionGroups()
				if err != nil {
					t.Fatal(err)
				}
				groups = nodeDeclaredActionGroups(groups)
				if len(groups) != 2 {
					t.Fatalf("active action groups = %d, want 2", len(groups))
				}
				if groups[0].Owner() != tabID(inspectionIndex) || groups[1].Owner() != tui.NewNodeID("tabs") {
					t.Fatalf("action owners = %q, %q", groups[0].Owner(), groups[1].Owner())
				}
				itemActions, rootActions = groups[0], groups[1]
			} else {
				focused, err := runtime.RequestFocus(tabID(0))
				if err != nil || focused {
					t.Fatalf("disabled focus = %t, %v", focused, err)
				}
				tabs := fixtureTabs(count, selected, enabled)
				scopes := []tui.KeyScope{tui.NewKeyScope(tui.NewNodeID("scope"), keyMap)}
				itemActions, err = tui.ResolveActions(
					tabID(0),
					[]tui.ActionDescriptor{tabs.ItemActionDescriptor()},
					scopes,
				)
				if err != nil {
					t.Fatal(err)
				}
				rootActions, err = tui.ResolveActions(
					tui.NewNodeID("tabs"),
					tabs.NavigationActionDescriptors(),
					scopes,
				)
				if err != nil {
					t.Fatal(err)
				}
			}

			available := fixtureBool(t, record.Field("available"))
			assertTabsActionGroup(
				t,
				itemActions,
				[]tui.ActionID{ActivateActionID},
				[]string{"Activate"},
				tabsFixtureKeyGroups(record.Field("item-keys")),
				available,
			)
			assertTabsActionGroup(
				t,
				rootActions,
				[]tui.ActionID{
					SelectionPreviousActionID,
					SelectionNextActionID,
					SelectionFirstActionID,
					SelectionLastActionID,
				},
				[]string{"Previous", "Next", "First", "Last"},
				tabsFixtureKeyGroups(record.Field("root-keys")),
				available,
			)

			focus, hasFocus := tabsFixtureOptionalInt(t, record.Field("focus"))
			if interactive && hasFocus {
				focused, err := runtime.RequestFocus(tabID(focus))
				if err != nil || !focused {
					t.Fatalf("event focus = %t, %v", focused, err)
				}
			} else {
				runtime.ClearFocus()
			}

			dispatch, err := runtime.DispatchEvent(tabsActionEvent(t, record.Field("event")))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runtime.ProcessPending(); err != nil {
				t.Fatal(err)
			}
			expectedMessages := tabsFixtureIntList(t, record.Field("message"))
			if !reflect.DeepEqual(app.messages, expectedMessages) {
				t.Errorf("messages = %#v, want %#v", app.messages, expectedMessages)
			}
			if dispatch.Messages() != len(expectedMessages) {
				t.Errorf("dispatched messages = %d, want %d", dispatch.Messages(), len(expectedMessages))
			}
			if dispatch.Consumed() != fixtureBool(t, record.Field("consumed")) {
				t.Errorf("consumed = %t", dispatch.Consumed())
			}
			expectedFocus, expectsFocus := tabsFixtureOptionalInt(t, record.Field("focus-after"))
			actualFocus, hasActualFocus := runtime.Interaction().Focused()
			if hasActualFocus != expectsFocus || expectsFocus && actualFocus != tabID(expectedFocus) {
				t.Errorf("focus = %q, %t, want tab-%d, %t", actualFocus, hasActualFocus, expectedFocus, expectsFocus)
			}
		})
	}
}

func fixtureTabs(count, selected int, enabled bool) Tabs[int] {
	items := make([]TabItem, count)
	for index := range items {
		items[index] = NewTabItem(tabID(index), string(rune('A'+index)))
	}
	return NewTabs(tui.NewNodeID("tabs"), items, selected, func(index int) int {
		return index
	}).Enabled(enabled)
}

func tabID(index int) tui.NodeID {
	return tui.NewNodeID("tab-" + strconv.Itoa(index))
}

func tabsActionKeyMap(t *testing.T, mode string) tui.KeyMap {
	t.Helper()
	characterBinding := func(character rune) []tui.KeyBinding {
		return []tui.KeyBinding{
			tui.NewKeyBinding(tui.NewCharacterKeyStroke(character, vt.Modifiers{})),
		}
	}
	keyMap := tui.NewKeyMap()
	var err error
	switch mode {
	case "default":
		return keyMap
	case "activate-x":
		keyMap, err = keyMap.Rebind(ActivateActionID, characterBinding('x'))
	case "previous-k":
		keyMap, err = keyMap.Rebind(SelectionPreviousActionID, characterBinding('k'))
	case "unbind-next":
		keyMap, err = keyMap.Rebind(SelectionNextActionID, nil)
	case "shadow-x":
		keyMap, err = keyMap.Rebind(ActivateActionID, characterBinding('x'))
		if err == nil {
			keyMap, err = keyMap.Rebind(SelectionPreviousActionID, characterBinding('x'))
		}
	default:
		t.Fatalf("unknown Tabs action mode %q", mode)
	}
	if err != nil {
		t.Fatal(err)
	}
	return keyMap
}

func tabsActionEvent(t *testing.T, value string) vt.Event {
	t.Helper()
	keyboard := func(code vt.KeyCode, character rune, modifiers vt.Modifiers, action vt.KeyAction) vt.Event {
		return keyEvent(code, character, modifiers, action)
	}
	switch value {
	case "enter":
		return keyboard(vt.KeyEnter, 0, vt.Modifiers{}, vt.KeyPress)
	case "space-text":
		return vt.Event{Kind: vt.EventText, Text: " "}
	case "repeat-enter":
		return keyboard(vt.KeyEnter, 0, vt.Modifiers{}, vt.KeyRepeat)
	case "release-enter":
		return keyboard(vt.KeyEnter, 0, vt.Modifiers{}, vt.KeyRelease)
	case "control-enter":
		return keyboard(vt.KeyEnter, 0, vt.Modifiers{Control: true}, vt.KeyPress)
	case "left":
		return keyboard(vt.KeyLeft, 0, vt.Modifiers{}, vt.KeyPress)
	case "right":
		return keyboard(vt.KeyRight, 0, vt.Modifiers{}, vt.KeyPress)
	case "home":
		return keyboard(vt.KeyHome, 0, vt.Modifiers{}, vt.KeyPress)
	case "end":
		return keyboard(vt.KeyEnd, 0, vt.Modifiers{}, vt.KeyPress)
	case "repeat-right":
		return keyboard(vt.KeyRight, 0, vt.Modifiers{}, vt.KeyRepeat)
	case "shift-right":
		return keyboard(vt.KeyRight, 0, vt.Modifiers{Shift: true}, vt.KeyPress)
	case "x":
		return keyboard(vt.KeyCharacter, 'x', vt.Modifiers{}, vt.KeyPress)
	case "k":
		return keyboard(vt.KeyCharacter, 'k', vt.Modifiers{}, vt.KeyPress)
	default:
		const pointerPrefix = "mouse-left-press-"
		if strings.HasPrefix(value, pointerPrefix) {
			index := fixtureInt(t, strings.TrimPrefix(value, pointerPrefix))
			return vt.Event{
				Kind: vt.EventMouse,
				Mouse: vt.MouseEvent{
					Kind: vt.MousePress, Button: vt.MouseLeft, X: uint32(index * 3),
				},
			}
		}
		t.Fatalf("unknown Tabs action event %q", value)
		return vt.Event{}
	}
}

func assertTabsActionGroup(
	t *testing.T,
	resolved tui.ResolvedActions,
	expectedIDs []tui.ActionID,
	expectedLabels []string,
	expectedKeys [][]string,
	available bool,
) {
	t.Helper()
	actions := resolved.Actions()
	if len(actions) != len(expectedIDs) || len(expectedKeys) != len(expectedIDs) {
		t.Fatalf("actions = %d, key groups = %d, want %d", len(actions), len(expectedKeys), len(expectedIDs))
	}
	for index, action := range actions {
		if action.ID() != expectedIDs[index] || action.Label() != expectedLabels[index] {
			t.Errorf("action %d = %q %q", index, action.ID(), action.Label())
		}
		bindings := action.Bindings()
		keys := make([]string, len(bindings))
		for bindingIndex, binding := range bindings {
			keys[bindingIndex] = binding.Stroke().Notation()
		}
		if strings.Join(keys, ",") != strings.Join(expectedKeys[index], ",") {
			t.Errorf("action %s keys = %#v, want %#v", expectedIDs[index], keys, expectedKeys[index])
		}
		if actual := action.Availability() == tui.ActionEnabled; actual != available {
			t.Errorf("action %s available = %t, want %t", expectedIDs[index], actual, available)
		}
	}
}

func tabsFixtureOptionalInt(t *testing.T, value string) (int, bool) {
	t.Helper()
	if value == "none" {
		return 0, false
	}
	return fixtureInt(t, value), true
}

func tabsFixtureIntList(t *testing.T, value string) []int {
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

func tabsFixtureKeyGroups(value string) [][]string {
	groups := strings.Split(value, "|")
	result := make([][]string, len(groups))
	for index, group := range groups {
		if group != "-" {
			result[index] = strings.Split(group, ",")
		}
	}
	return result
}
