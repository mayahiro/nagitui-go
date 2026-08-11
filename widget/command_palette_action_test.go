package widget

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

type commandPaletteActionMessage struct {
	kind  string
	text  string
	index int
}

type commandPaletteActionApp struct {
	query    string
	selected int
	enabled  bool
	keyMap   tui.KeyMap
	messages []commandPaletteActionMessage
}

func (*commandPaletteActionApp) Init() tui.Effect[commandPaletteActionMessage] {
	return tui.NoneEffect[commandPaletteActionMessage]()
}

func (a *commandPaletteActionApp) Update(message commandPaletteActionMessage) tui.Effect[commandPaletteActionMessage] {
	switch message.kind {
	case "query":
		a.query = message.text
	case "select":
		a.selected = message.index
	}
	a.messages = append(a.messages, message)
	return tui.NoneEffect[commandPaletteActionMessage]()
}

func (*commandPaletteActionApp) Subscriptions() tui.Subscription[commandPaletteActionMessage] {
	return tui.NoneSubscription[commandPaletteActionMessage]()
}

func (a *commandPaletteActionApp) View(tui.ViewContext) tui.Node[commandPaletteActionMessage] {
	palette := fixtureCommandPalette(a.query, a.selected, a.enabled).Node()
	return tui.Padding(palette, tui.UniformInsets(0)).
		WithKeyScope(tui.NewKeyScope(tui.NewNodeID("scope"), a.keyMap))
}

func TestCommandPaletteActionsMatchSharedFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t,
		"widgets/command-palette-action.txt",
		"widget-command-palette-action",
		"query", "selected", "focus", "mode", "enabled", "event", "messages", "consumed",
		"focus-after", "query-after", "selected-after", "command-keys", "root-keys",
		"available", "owners", "cursor-after",
	)
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			query := commandPaletteFixtureQuery(record.Text("query"))
			selected := fixtureInt(t, record.Field("selected"))
			enabled := fixtureBool(t, record.Field("enabled"))
			keyMap := commandPaletteActionKeyMap(t, record.Field("mode"))
			palette := fixtureCommandPalette(query, selected, enabled)
			scopes := []tui.KeyScope{tui.NewKeyScope(tui.NewNodeID("scope"), keyMap)}
			commandActions, err := tui.ResolveActions(
				tui.NewNodeID("command-inspection"),
				[]tui.ActionDescriptor{palette.CommandActionDescriptor()},
				scopes,
			)
			if err != nil {
				t.Fatal(err)
			}
			rootActions, err := tui.ResolveActions(
				tui.NewNodeID("palette"),
				palette.ActionDescriptors(),
				scopes,
			)
			if err != nil {
				t.Fatal(err)
			}
			available := fixtureBool(t, record.Field("available"))
			assertCommandPaletteActionGroup(
				t,
				commandActions,
				[]tui.ActionID{ActivateActionID},
				[]string{"Activate"},
				[][]string{commandPaletteFixtureStrings(record.Field("command-keys"))},
				available,
			)
			assertCommandPaletteActionGroup(
				t,
				rootActions,
				[]tui.ActionID{
					ActivateActionID,
					SelectionPreviousActionID,
					SelectionNextActionID,
					SelectionFirstActionID,
					SelectionLastActionID,
				},
				[]string{"Activate", "Previous", "Next", "First", "Last"},
				commandPaletteFixtureKeyGroups(record.Field("root-keys")),
				available,
			)

			app := &commandPaletteActionApp{
				query: query, selected: selected, enabled: enabled, keyMap: keyMap,
			}
			runtime, err := tui.NewRuntimeWithClock(
				app,
				tui.NewRuntimeConfig(tui.Size{Width: 32, Height: 8}),
				tui.NewVirtualClock(),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer runtime.Close()
			if _, err := runtime.RenderIfDirty(); err != nil {
				t.Fatal(err)
			}

			if focus, ok := commandPaletteFixtureFocus(record.Field("focus")); ok {
				focused, err := runtime.RequestFocus(focus)
				if err != nil || !focused {
					t.Fatalf("focus %s = %t, %v", focus, focused, err)
				}
			} else {
				runtime.ClearFocus()
			}
			groups, err := runtime.ActiveActionGroups()
			if err != nil {
				t.Fatal(err)
			}
			owners := make([]string, len(groups))
			for index, group := range groups {
				owners[index] = string(group.Owner())
			}
			if expected := commandPaletteFixtureStrings(record.Field("owners")); !slices.Equal(owners, expected) {
				t.Errorf("owners = %#v, want %#v", owners, expected)
			}

			dispatch, err := runtime.DispatchEvent(commandPaletteActionEvent(t, record.Field("event")))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runtime.ProcessPending(); err != nil {
				t.Fatal(err)
			}
			messages := make([]string, len(app.messages))
			for index, message := range app.messages {
				messages[index] = commandPaletteActionMessageName(message)
			}
			expectedMessages := commandPaletteFixtureStrings(record.Text("messages"))
			if !slices.Equal(messages, expectedMessages) {
				t.Errorf("messages = %#v, want %#v", messages, expectedMessages)
			}
			if dispatch.Messages() != len(expectedMessages) {
				t.Errorf("dispatched messages = %d, want %d", dispatch.Messages(), len(expectedMessages))
			}
			if dispatch.Consumed() != fixtureBool(t, record.Field("consumed")) {
				t.Errorf("consumed = %t", dispatch.Consumed())
			}
			expectedFocus, expectsFocus := commandPaletteFixtureFocus(record.Field("focus-after"))
			actualFocus, hasFocus := runtime.Interaction().Focused()
			if hasFocus != expectsFocus || expectsFocus && actualFocus != expectedFocus {
				t.Errorf("focus = %q, %t, want %q, %t", actualFocus, hasFocus, expectedFocus, expectsFocus)
			}
			if expected := commandPaletteFixtureQuery(record.Text("query-after")); app.query != expected {
				t.Errorf("query = %q, want %q", app.query, expected)
			}
			if expected := fixtureInt(t, record.Field("selected-after")); app.selected != expected {
				t.Errorf("selected = %d, want %d", app.selected, expected)
			}
			if value := record.Field("cursor-after"); value != "-" {
				state, ok := runtime.Interaction().TextInput(tui.NewNodeID("query"))
				if !ok || state.Cursor() != fixtureInt(t, value) {
					t.Errorf("query cursor = %d, %t, want %s", state.Cursor(), ok, value)
				}
			}
		})
	}
}

func TestCommandPaletteRootConflictIsReportedBeforeDispatch(t *testing.T) {
	binding := []tui.KeyBinding{
		tui.NewKeyBinding(tui.NewCharacterKeyStroke('k', vt.Modifiers{Control: true})),
	}
	keyMap, err := tui.NewKeyMap().Rebind(ActivateActionID, binding)
	if err != nil {
		t.Fatal(err)
	}
	keyMap, err = keyMap.Rebind(SelectionPreviousActionID, binding)
	if err != nil {
		t.Fatal(err)
	}
	app := &commandPaletteActionApp{enabled: true, keyMap: keyMap}
	runtime, err := tui.NewRuntimeWithClock(
		app,
		tui.NewRuntimeConfig(tui.Size{Width: 32, Height: 8}),
		tui.NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	frame, err := runtime.RenderIfDirty()
	if frame != nil {
		t.Fatal("conflicting frame was published")
	}
	var conflict *tui.BindingConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("RenderIfDirty error = %v, want BindingConflictError", err)
	}
	if conflict.Kind() != tui.ConflictAmbiguousBinding || conflict.Owner() != tui.NewNodeID("palette") {
		t.Fatalf("conflict = kind %d owner %s", conflict.Kind(), conflict.Owner())
	}
	expected := []tui.ActionID{ActivateActionID, SelectionPreviousActionID}
	if !slices.Equal(conflict.Actions(), expected) {
		t.Fatalf("conflict actions = %#v, want %#v", conflict.Actions(), expected)
	}
	if len(app.messages) != 0 {
		t.Fatalf("conflicting action emitted messages = %#v", app.messages)
	}
}

func fixtureCommandPalette(query string, selected int, enabled bool) CommandPalette[commandPaletteActionMessage] {
	return NewCommandPalette(
		tui.NewNodeID("palette"),
		tui.NewNodeID("query"),
		query,
		[]Command{
			NewCommand(tui.NewNodeID("command-0"), "Alpha").WithKeywords("first"),
			NewCommand(tui.NewNodeID("command-1"), "Beta").WithKeywords("second"),
			NewCommand(tui.NewNodeID("command-2"), "Alpine").WithKeywords("peak"),
			NewCommand(tui.NewNodeID("command-3"), "Gamma").WithKeywords("third"),
		},
		selected,
		func(query string) commandPaletteActionMessage {
			return commandPaletteActionMessage{kind: "query", text: query}
		},
		func(index int) commandPaletteActionMessage {
			return commandPaletteActionMessage{kind: "select", index: index}
		},
		func(index int) commandPaletteActionMessage {
			return commandPaletteActionMessage{kind: "activate", index: index}
		},
	).Enabled(enabled)
}

func commandPaletteActionKeyMap(t *testing.T, mode string) tui.KeyMap {
	t.Helper()
	keyMap := tui.NewKeyMap()
	var bindings []tui.KeyBinding
	var action tui.ActionID
	switch mode {
	case "default":
		return keyMap
	case "activate-control-enter":
		action = ActivateActionID
		bindings = []tui.KeyBinding{
			tui.NewKeyBinding(tui.NewKeyStroke(vt.KeyEnter, vt.Modifiers{Control: true})),
		}
	case "activate-x":
		action = ActivateActionID
		bindings = []tui.KeyBinding{
			tui.NewKeyBinding(tui.NewCharacterKeyStroke('x', vt.Modifiers{})),
		}
	case "next-control-j":
		action = SelectionNextActionID
		bindings = []tui.KeyBinding{
			tui.NewKeyBinding(tui.NewCharacterKeyStroke('j', vt.Modifiers{Control: true})),
		}
	case "unbind-next":
		action = SelectionNextActionID
	default:
		t.Fatalf("unknown CommandPalette action mode %q", mode)
	}
	keyMap, err := keyMap.Rebind(action, bindings)
	if err != nil {
		t.Fatal(err)
	}
	return keyMap
}

func commandPaletteActionEvent(t *testing.T, value string) vt.Event {
	t.Helper()
	keyboard := func(code vt.KeyCode, character rune, modifiers vt.Modifiers, action vt.KeyAction) vt.Event {
		return keyEvent(code, character, modifiers, action)
	}
	switch value {
	case "enter":
		return keyboard(vt.KeyEnter, 0, vt.Modifiers{}, vt.KeyPress)
	case "repeat-enter":
		return keyboard(vt.KeyEnter, 0, vt.Modifiers{}, vt.KeyRepeat)
	case "release-enter":
		return keyboard(vt.KeyEnter, 0, vt.Modifiers{}, vt.KeyRelease)
	case "shift-enter":
		return keyboard(vt.KeyEnter, 0, vt.Modifiers{Shift: true}, vt.KeyPress)
	case "control-enter":
		return keyboard(vt.KeyEnter, 0, vt.Modifiers{Control: true}, vt.KeyPress)
	case "up":
		return keyboard(vt.KeyUp, 0, vt.Modifiers{}, vt.KeyPress)
	case "down":
		return keyboard(vt.KeyDown, 0, vt.Modifiers{}, vt.KeyPress)
	case "repeat-down":
		return keyboard(vt.KeyDown, 0, vt.Modifiers{}, vt.KeyRepeat)
	case "shift-down":
		return keyboard(vt.KeyDown, 0, vt.Modifiers{Shift: true}, vt.KeyPress)
	case "home":
		return keyboard(vt.KeyHome, 0, vt.Modifiers{}, vt.KeyPress)
	case "end":
		return keyboard(vt.KeyEnd, 0, vt.Modifiers{}, vt.KeyPress)
	case "control-j":
		return keyboard(vt.KeyCharacter, 'j', vt.Modifiers{Control: true}, vt.KeyPress)
	case "space-text":
		return vt.Event{Kind: vt.EventText, Text: " "}
	case "text-x":
		return vt.Event{Kind: vt.EventText, Text: "x"}
	default:
		const pointerPrefix = "mouse-left-press-"
		if strings.HasPrefix(value, pointerPrefix) {
			position := fixtureInt(t, strings.TrimPrefix(value, pointerPrefix))
			return vt.Event{
				Kind: vt.EventMouse,
				Mouse: vt.MouseEvent{
					Kind: vt.MousePress, Button: vt.MouseLeft, X: 2, Y: uint32(position + 2),
				},
			}
		}
		t.Fatalf("unknown CommandPalette action event %q", value)
		return vt.Event{}
	}
}

func assertCommandPaletteActionGroup(
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
	availability := tui.ActionDisabledPassThrough
	if available {
		availability = tui.ActionEnabled
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
		if !slices.Equal(keys, expectedKeys[index]) {
			t.Errorf("action %s keys = %#v, want %#v", expectedIDs[index], keys, expectedKeys[index])
		}
		if action.Availability() != availability {
			t.Errorf("action %s availability = %d, want %d", expectedIDs[index], action.Availability(), availability)
		}
	}
}

func commandPaletteActionMessageName(message commandPaletteActionMessage) string {
	switch message.kind {
	case "query":
		return "query:" + message.text
	case "select", "activate":
		return message.kind + ":" + strconv.Itoa(message.index)
	default:
		panic("unknown CommandPalette action message")
	}
}

func commandPaletteFixtureFocus(value string) (tui.NodeID, bool) {
	switch {
	case value == "none":
		return tui.NodeID(""), false
	case value == "query", strings.HasPrefix(value, "command-"):
		return tui.NewNodeID(value), true
	default:
		panic("invalid CommandPalette fixture focus " + value)
	}
}

func commandPaletteFixtureQuery(value string) string {
	if value == "-" {
		return ""
	}
	return value
}

func commandPaletteFixtureStrings(value string) []string {
	if value == "-" {
		return nil
	}
	return strings.Split(value, ",")
}

func commandPaletteFixtureKeyGroups(value string) [][]string {
	groups := strings.Split(value, "|")
	result := make([][]string, len(groups))
	for index, group := range groups {
		result[index] = commandPaletteFixtureStrings(group)
	}
	return result
}
