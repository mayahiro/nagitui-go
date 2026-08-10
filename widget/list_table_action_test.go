package widget

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

type collectionActionApp struct {
	widget   string
	layout   string
	view     string
	count    int
	selected int
	enabled  bool
	keyMap   tui.KeyMap
	messages []int
}

func (*collectionActionApp) Init() tui.Effect[int] {
	return tui.NoneEffect[int]()
}

func (a *collectionActionApp) Update(message int) tui.Effect[int] {
	a.messages = append(a.messages, message)
	return tui.NoneEffect[int]()
}

func (*collectionActionApp) Subscriptions() tui.Subscription[int] {
	return tui.NoneSubscription[int]()
}

func (a *collectionActionApp) View(tui.ViewContext) tui.Node[int] {
	node := collectionNode(a.widget, a.layout, a.view, a.count, a.selected, a.enabled)
	return tui.Padding(node, tui.UniformInsets(0)).
		WithKeyScope(tui.NewKeyScope(tui.NewNodeID("scope"), a.keyMap))
}

func TestListAndTableActionsMatchSharedFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t,
		"widgets/list-table-action.txt",
		"widget-list-table-action",
		"widget", "layout", "view", "count", "selected", "mode", "enabled", "event",
		"message", "consumed", "focus", "keys", "available",
	)
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			widget := record.Field("widget")
			layout := record.Field("layout")
			view := record.Field("view")
			count := fixtureInt(t, record.Field("count"))
			selected := fixtureInt(t, record.Field("selected"))
			enabled := fixtureBool(t, record.Field("enabled"))
			available := fixtureBool(t, record.Field("available"))
			keyMap := collectionActionKeyMap(t, record.Field("mode"))
			app := &collectionActionApp{
				widget: widget, layout: layout, view: view, count: count,
				selected: selected, enabled: enabled, keyMap: keyMap,
			}
			runtime, err := tui.NewRuntimeWithClock(
				app,
				tui.NewRuntimeConfig(tui.Size{Width: 30, Height: 6}),
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
				if len(groups) != 1 || groups[0].Owner() != tui.NewNodeID("root") {
					t.Fatalf("active action groups = %#v", groups)
				}
				resolved = groups[0]
			} else {
				resolved, err = tui.ResolveActions(
					tui.NewNodeID("root"),
					collectionDescriptors(widget, layout, view, count, selected, enabled),
					[]tui.KeyScope{tui.NewKeyScope(tui.NewNodeID("scope"), keyMap)},
				)
				if err != nil {
					t.Fatal(err)
				}
			}
			assertCollectionActions(
				t,
				resolved,
				collectionFixtureKeyGroups(record.Field("keys")),
				available,
			)

			if strings.HasPrefix(record.Field("event"), "pointer-") {
				runtime.ClearFocus()
			}
			dispatch, err := runtime.DispatchEvent(collectionActionEvent(
				t, widget, view, count, record.Field("event"),
			))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runtime.ProcessPending(); err != nil {
				t.Fatal(err)
			}

			expectedMessages := collectionFixtureIntList(t, record.Field("message"))
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

func collectionNode(widget, layout, view string, count, selected int, enabled bool) tui.Node[int] {
	switch widget {
	case "list":
		return configuredList(layout, view, count, selected, enabled).Node()
	case "table":
		return configuredTable(layout, count, selected, enabled).Node()
	default:
		panic("unknown collection widget " + widget)
	}
}

func collectionDescriptors(widget, layout, view string, count, selected int, enabled bool) []tui.ActionDescriptor {
	switch widget {
	case "list":
		return configuredList(layout, view, count, selected, enabled).ActionDescriptors()
	case "table":
		return configuredTable(layout, count, selected, enabled).ActionDescriptors()
	default:
		panic("unknown collection widget " + widget)
	}
}

func configuredList(layout, view string, count, selected int, enabled bool) List[int] {
	items := make([]ListItem, count)
	for index := range items {
		items[index] = NewListItem(tui.NewNodeID("item-"+strconv.Itoa(index)), collectionFixtureLabel(index))
	}
	list := NewList(tui.NewNodeID("root"), items, selected, func(index int) int {
		return index
	}).Enabled(enabled)
	switch view {
	case "all":
	case "filter-p":
		list = list.Filter("p")
	case "filter-z":
		list = list.Filter("z")
	case "window-1-2":
		list = list.Window(1, 2)
	case "window-empty":
		list = list.Window(0, 0)
	default:
		panic("unknown List view " + view)
	}
	switch layout {
	case "eager":
		return list
	case "virtual":
		return list.Viewport(tui.NewNodeID("viewport"), tui.Fixed(2))
	default:
		panic("unknown collection layout " + layout)
	}
}

func configuredTable(layout string, count, selected int, enabled bool) Table[int] {
	rows := make([]TableRow, count)
	for index := range rows {
		rows[index] = NewTableRow(
			tui.NewNodeID("row-"+strconv.Itoa(index)),
			[]string{collectionFixtureLabel(index)},
		)
	}
	table := NewTable(
		tui.NewNodeID("root"),
		[]TableColumn{NewTableColumn("Value", tui.Fixed(8))},
		rows,
		selected,
		func(index int) int { return index },
	).Enabled(enabled)
	switch layout {
	case "eager":
		return table
	case "virtual":
		return table.Viewport(tui.NewNodeID("viewport"), tui.Fixed(2))
	default:
		panic("unknown collection layout " + layout)
	}
}

func collectionFixtureLabel(index int) string {
	labels := []string{"Alpha", "Beta", "Alpine", "Gamma"}
	if index < len(labels) {
		return labels[index]
	}
	return "Item " + strconv.Itoa(index)
}

func collectionActionKeyMap(t *testing.T, mode string) tui.KeyMap {
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
	default:
		t.Fatalf("unknown collection action mode %q", mode)
	}
	if err != nil {
		t.Fatal(err)
	}
	return keyMap
}

func collectionActionEvent(t *testing.T, widget, view string, count int, value string) vt.Event {
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
	case "up":
		return keyboard(vt.KeyUp, 0, vt.Modifiers{}, vt.KeyPress)
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
	default:
		const pointerPrefix = "pointer-"
		if strings.HasPrefix(value, pointerPrefix) {
			original := fixtureInt(t, strings.TrimPrefix(value, pointerPrefix))
			position := original
			if widget == "list" {
				position = collectionVisiblePosition(view, count, original)
			}
			if widget == "table" {
				position++
			}
			return vt.Event{
				Kind: vt.EventMouse,
				Mouse: vt.MouseEvent{
					Kind: vt.MousePress, Button: vt.MouseLeft, Y: uint32(position),
				},
			}
		}
		t.Fatalf("unknown collection action event %q", value)
		return vt.Event{}
	}
}

func collectionVisiblePosition(view string, count, original int) int {
	visible := collectionFixtureVisibleIndices(view, count)
	for position, index := range visible {
		if index == original {
			return position
		}
	}
	return 0
}

func collectionFixtureVisibleIndices(view string, count int) []int {
	switch view {
	case "all":
		result := make([]int, count)
		for index := range result {
			result[index] = index
		}
		return result
	case "filter-p":
		var result []int
		for _, index := range []int{0, 2} {
			if index < count {
				result = append(result, index)
			}
		}
		return result
	case "window-1-2":
		var result []int
		for index := 1; index < min(count, 3); index++ {
			result = append(result, index)
		}
		return result
	case "filter-z", "window-empty":
		return nil
	default:
		panic("unknown List view " + view)
	}
}

func assertCollectionActions(
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
	}
	expectedLabels := []string{"Activate", "Previous", "Next", "First", "Last"}
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

func collectionFixtureIntList(t *testing.T, value string) []int {
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

func collectionFixtureKeyGroups(value string) [][]string {
	groups := strings.Split(value, "|")
	result := make([][]string, len(groups))
	for index, group := range groups {
		if group != "-" {
			result[index] = strings.Split(group, ",")
		}
	}
	return result
}
