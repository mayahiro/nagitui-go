package widget

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

type treeActionMessage struct {
	kind     string
	index    int
	expanded bool
}

type treeActionApp struct {
	layout   string
	model    string
	selected int
	toggle   bool
	enabled  bool
	keyMap   tui.KeyMap
	messages []treeActionMessage
}

func (*treeActionApp) Init() tui.Effect[treeActionMessage] {
	return tui.NoneEffect[treeActionMessage]()
}

func (a *treeActionApp) Update(message treeActionMessage) tui.Effect[treeActionMessage] {
	a.messages = append(a.messages, message)
	return tui.NoneEffect[treeActionMessage]()
}

func (*treeActionApp) Subscriptions() tui.Subscription[treeActionMessage] {
	return tui.NoneSubscription[treeActionMessage]()
}

func (a *treeActionApp) View(tui.ViewContext) tui.Node[treeActionMessage] {
	node := configuredActionTree(a.layout, a.model, a.selected, a.toggle, a.enabled).Node()
	return tui.Padding(node, tui.UniformInsets(0)).
		WithKeyScope(tui.NewKeyScope(tui.NewNodeID("scope"), a.keyMap))
}

func TestTreeActionsMatchSharedFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t,
		"widgets/tree-action.txt",
		"widget-tree-action",
		"layout", "model", "selected", "toggle", "mode", "enabled", "event",
		"message", "consumed", "focus", "keys", "available",
	)
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			layout := record.Field("layout")
			model := record.Field("model")
			selected := fixtureInt(t, record.Field("selected"))
			toggle := fixtureBool(t, record.Field("toggle"))
			enabled := fixtureBool(t, record.Field("enabled"))
			available := fixtureBool(t, record.Field("available"))
			keyMap := treeActionKeyMap(t, record.Field("mode"))
			app := &treeActionApp{
				layout: layout, model: model, selected: selected, toggle: toggle,
				enabled: enabled, keyMap: keyMap,
			}
			runtime, err := tui.NewRuntimeWithClock(
				app,
				tui.NewRuntimeConfig(tui.Size{Width: 30, Height: 8}),
				tui.NewVirtualClock(),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer runtime.Close()
			if _, err := runtime.RenderIfDirty(); err != nil {
				t.Fatal(err)
			}

			focused, err := runtime.RequestFocus(tui.NewNodeID("root"))
			if err != nil || focused != available {
				t.Fatalf("focus = %t, %v, want %t", focused, err, available)
			}
			var resolved tui.ResolvedActions
			if available {
				groups, err := runtime.ActiveActionGroups()
				if err != nil {
					t.Fatal(err)
				}
				groups = nodeDeclaredActionGroups(groups)
				if len(groups) != 1 || groups[0].Owner() != tui.NewNodeID("root") {
					t.Fatalf("active action groups = %#v", groups)
				}
				resolved = groups[0]
			} else {
				resolved, err = tui.ResolveActions(
					tui.NewNodeID("root"),
					configuredActionTree(layout, model, selected, toggle, enabled).ActionDescriptors(),
					[]tui.KeyScope{tui.NewKeyScope(tui.NewNodeID("scope"), keyMap)},
				)
				if err != nil {
					t.Fatal(err)
				}
			}
			assertTreeActions(
				t,
				resolved,
				treeFixtureKeyGroups(record.Field("keys")),
				available,
			)

			eventName := record.Field("event")
			if strings.HasPrefix(eventName, "pointer-") {
				runtime.ClearFocus()
			}
			dispatch, err := runtime.DispatchEvent(treeActionEvent(
				t, layout, model, selected, eventName,
			))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runtime.ProcessPending(); err != nil {
				t.Fatal(err)
			}

			expectedMessages := treeFixtureMessages(t, record.Field("message"))
			if !reflect.DeepEqual(app.messages, expectedMessages) {
				t.Errorf("messages = %#v, want %#v", app.messages, expectedMessages)
			}
			if dispatch.Messages() != len(expectedMessages) {
				t.Errorf("dispatched messages = %d, want %d", dispatch.Messages(), len(expectedMessages))
			}
			if dispatch.Consumed() != fixtureBool(t, record.Field("consumed")) {
				t.Errorf("consumed = %t", dispatch.Consumed())
			}
			actualFocus, hasFocus := runtime.Interaction().Focused()
			expectsFocus := record.Field("focus") == "root"
			if hasFocus != expectsFocus || expectsFocus && actualFocus != tui.NewNodeID("root") {
				t.Errorf("focus = %q, %t, want root = %t", actualFocus, hasFocus, expectsFocus)
			}
		})
	}
}

func configuredActionTree(
	layout string,
	model string,
	selected int,
	toggle bool,
	enabled bool,
) Tree[treeActionMessage] {
	tree := NewTree(
		tui.NewNodeID("root"),
		treeActionItems(model),
		selected,
		func(index int) treeActionMessage {
			return treeActionMessage{kind: "select", index: index}
		},
	).Enabled(enabled)
	if toggle {
		tree = tree.OnToggle(func(index int, expanded bool) treeActionMessage {
			return treeActionMessage{kind: "toggle", index: index, expanded: expanded}
		})
	}
	switch layout {
	case "full":
		return tree
	case "viewport":
		return tree.Viewport(2)
	default:
		panic("unknown Tree layout " + layout)
	}
}

func treeActionItems(model string) []TreeItem {
	switch model {
	case "nested":
		return nestedActionTree(true)
	case "nested-group-collapsed":
		return nestedActionTree(false)
	case "root-collapsed":
		return []TreeItem{
			NewTreeBranch("item-0", "Root", 0, false),
			NewTreeBranch("item-1", "Group", 1, true),
			NewTreeLeaf("item-2", "Leaf", 2),
			NewTreeLeaf("item-3", "Sibling", 1),
			NewTreeBranch("item-4", "Peer", 0, false),
			NewTreeLeaf("item-5", "Hidden", 1),
			NewTreeLeaf("item-6", "Tail", 0),
		}
	case "flat":
		items := make([]TreeItem, 4)
		for index := range items {
			items[index] = NewTreeLeaf(
				tui.NewNodeID("item-"+strconv.Itoa(index)),
				"Item "+strconv.Itoa(index),
				0,
			)
		}
		return items
	case "empty":
		return nil
	default:
		panic("unknown Tree model " + model)
	}
}

func nestedActionTree(groupExpanded bool) []TreeItem {
	return []TreeItem{
		NewTreeBranch("item-0", "Root", 0, true),
		NewTreeBranch("item-1", "Group", 1, groupExpanded),
		NewTreeLeaf("item-2", "Leaf", 2),
		NewTreeLeaf("item-3", "Sibling", 1),
		NewTreeBranch("item-4", "Peer", 0, false),
		NewTreeLeaf("item-5", "Hidden", 1),
		NewTreeLeaf("item-6", "Tail", 0),
	}
}

func treeActionKeyMap(t *testing.T, mode string) tui.KeyMap {
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
	case "collapse-h":
		keyMap, err = keyMap.Rebind(CollapseActionID, characterBinding('h'))
	case "expand-l":
		keyMap, err = keyMap.Rebind(ExpandActionID, characterBinding('l'))
	case "unbind-next":
		keyMap, err = keyMap.Rebind(SelectionNextActionID, nil)
	default:
		t.Fatalf("unknown Tree action mode %q", mode)
	}
	if err != nil {
		t.Fatal(err)
	}
	return keyMap
}

func treeActionEvent(
	t *testing.T,
	layout string,
	model string,
	selected int,
	value string,
) vt.Event {
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
	case "control-enter":
		return keyboard(vt.KeyEnter, 0, vt.Modifiers{Control: true}, vt.KeyPress)
	case "up":
		return keyboard(vt.KeyUp, 0, vt.Modifiers{}, vt.KeyPress)
	case "down":
		return keyboard(vt.KeyDown, 0, vt.Modifiers{}, vt.KeyPress)
	case "home":
		return keyboard(vt.KeyHome, 0, vt.Modifiers{}, vt.KeyPress)
	case "end":
		return keyboard(vt.KeyEnd, 0, vt.Modifiers{}, vt.KeyPress)
	case "left":
		return keyboard(vt.KeyLeft, 0, vt.Modifiers{}, vt.KeyPress)
	case "right":
		return keyboard(vt.KeyRight, 0, vt.Modifiers{}, vt.KeyPress)
	case "repeat-down":
		return keyboard(vt.KeyDown, 0, vt.Modifiers{}, vt.KeyRepeat)
	case "repeat-right":
		return keyboard(vt.KeyRight, 0, vt.Modifiers{}, vt.KeyRepeat)
	case "release-down":
		return keyboard(vt.KeyDown, 0, vt.Modifiers{}, vt.KeyRelease)
	case "release-right":
		return keyboard(vt.KeyRight, 0, vt.Modifiers{}, vt.KeyRelease)
	case "shift-down":
		return keyboard(vt.KeyDown, 0, vt.Modifiers{Shift: true}, vt.KeyPress)
	case "shift-right":
		return keyboard(vt.KeyRight, 0, vt.Modifiers{Shift: true}, vt.KeyPress)
	case "x":
		return keyboard(vt.KeyCharacter, 'x', vt.Modifiers{}, vt.KeyPress)
	case "h":
		return keyboard(vt.KeyCharacter, 'h', vt.Modifiers{}, vt.KeyPress)
	case "l":
		return keyboard(vt.KeyCharacter, 'l', vt.Modifiers{}, vt.KeyPress)
	default:
		const pointerPrefix = "pointer-"
		if strings.HasPrefix(value, pointerPrefix) {
			original := fixtureInt(t, strings.TrimPrefix(value, pointerPrefix))
			visible := treeActionVisibleIndices(model)
			position := -1
			for index, candidate := range visible {
				if candidate == original {
					position = index
					break
				}
			}
			if position < 0 {
				t.Fatalf("pointer target %d is not visible", original)
			}
			selectedPosition := 0
			for index, candidate := range visible {
				if candidate > max(selected, 0) {
					break
				}
				selectedPosition = index
			}
			start := 0
			if layout == "viewport" {
				start = treeActionViewportStart(len(visible), selectedPosition, 2)
				if position < start || position >= start+2 {
					t.Fatalf("pointer target %d is outside viewport", original)
				}
			}
			return vt.Event{
				Kind: vt.EventMouse,
				Mouse: vt.MouseEvent{
					Kind: vt.MousePress, Button: vt.MouseLeft,
					X: 0, Y: uint32(position - start),
				},
			}
		}
		t.Fatalf("unknown Tree action event %q", value)
		return vt.Event{}
	}
}

func treeActionVisibleIndices(model string) []int {
	switch model {
	case "nested":
		return []int{0, 1, 2, 3, 4, 6}
	case "nested-group-collapsed":
		return []int{0, 1, 3, 4, 6}
	case "root-collapsed":
		return []int{0, 4, 6}
	case "flat":
		return []int{0, 1, 2, 3}
	case "empty":
		return nil
	default:
		panic("unknown Tree model " + model)
	}
}

func treeActionViewportStart(count, selected, height int) int {
	height = min(height, count)
	return min(max(selected-height/2, 0), count-height)
}

func assertTreeActions(
	t *testing.T,
	resolved tui.ResolvedActions,
	expectedKeys [][]string,
	available bool,
) {
	t.Helper()
	expectedIDs := []tui.ActionID{
		ActivateActionID,
		SelectionPreviousActionID,
		SelectionNextActionID,
		SelectionFirstActionID,
		SelectionLastActionID,
		CollapseActionID,
		ExpandActionID,
	}
	expectedLabels := []string{
		"Activate", "Previous", "Next", "First", "Last", "Collapse", "Expand",
	}
	actions := resolved.Actions()
	if len(actions) != len(expectedIDs) || len(expectedKeys) != len(expectedIDs) {
		t.Fatalf("actions = %d, keys = %d", len(actions), len(expectedKeys))
	}
	for index, action := range actions {
		if action.ID() != expectedIDs[index] || action.Label() != expectedLabels[index] {
			t.Errorf("action %d = %q %q", index, action.ID(), action.Label())
		}
		actualKeys := make([]string, len(action.Bindings()))
		for bindingIndex, binding := range action.Bindings() {
			actualKeys[bindingIndex] = binding.Stroke().Notation()
		}
		if strings.Join(actualKeys, ",") != strings.Join(expectedKeys[index], ",") {
			t.Errorf("action %s keys = %#v, want %#v", action.ID(), actualKeys, expectedKeys[index])
		}
		if actual := action.Availability() == tui.ActionEnabled; actual != available {
			t.Errorf("action %s available = %t, want %t", action.ID(), actual, available)
		}
	}
}

func treeFixtureMessages(t *testing.T, value string) []treeActionMessage {
	t.Helper()
	if value == "-" {
		return nil
	}
	messages := make([]treeActionMessage, 0, strings.Count(value, ",")+1)
	for _, message := range strings.Split(value, ",") {
		fields := strings.Split(message, ":")
		switch {
		case len(fields) == 2 && fields[0] == "select":
			messages = append(messages, treeActionMessage{
				kind: "select", index: fixtureInt(t, fields[1]),
			})
		case len(fields) == 3 && fields[0] == "toggle":
			messages = append(messages, treeActionMessage{
				kind: "toggle", index: fixtureInt(t, fields[1]), expanded: fixtureBool(t, fields[2]),
			})
		default:
			t.Fatalf("invalid Tree fixture message %q", message)
		}
	}
	return messages
}

func treeFixtureKeyGroups(value string) [][]string {
	groups := strings.Split(value, "|")
	output := make([][]string, len(groups))
	for index, group := range groups {
		if group != "-" {
			output[index] = strings.Split(group, ",")
		}
	}
	return output
}
