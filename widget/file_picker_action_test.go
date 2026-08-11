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

type filePickerFixtureMessage struct {
	kind  string
	index int
}

type filePickerActionApp struct {
	count      int
	hidden     []int
	showHidden bool
	selected   int
	viewport   int
	handlers   string
	enabled    bool
	keyMap     tui.KeyMap
	messages   []filePickerFixtureMessage
}

func (*filePickerActionApp) Init() tui.Effect[filePickerFixtureMessage] {
	return tui.NoneEffect[filePickerFixtureMessage]()
}

func (a *filePickerActionApp) Update(message filePickerFixtureMessage) tui.Effect[filePickerFixtureMessage] {
	if message.kind == "select" {
		a.selected = message.index
	}
	a.messages = append(a.messages, message)
	return tui.NoneEffect[filePickerFixtureMessage]()
}

func (*filePickerActionApp) Subscriptions() tui.Subscription[filePickerFixtureMessage] {
	return tui.NoneSubscription[filePickerFixtureMessage]()
}

func (a *filePickerActionApp) View(tui.ViewContext) tui.Node[filePickerFixtureMessage] {
	picker := fixtureFilePicker(
		a.count,
		a.hidden,
		a.showHidden,
		a.selected,
		a.viewport,
		a.handlers,
		a.enabled,
	).Node()
	return tui.Padding(picker, tui.UniformInsets(0)).
		WithKeyScope(tui.NewKeyScope(tui.NewNodeID("scope"), a.keyMap))
}

func TestFilePickerActionsMatchSharedFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t,
		"widgets/file-picker-action.txt",
		"widget-file-picker-action",
		"count", "hidden", "show-hidden", "selected", "viewport", "focus", "mode",
		"enabled", "handlers", "event", "messages", "consumed", "focus-after",
		"selected-after", "keys", "available",
	)
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			count := fixtureInt(t, record.Field("count"))
			hidden := filePickerFixtureInts(t, record.Field("hidden"))
			showHidden := fixtureBool(t, record.Field("show-hidden"))
			selected := fixtureInt(t, record.Field("selected"))
			viewport := fixtureInt(t, record.Field("viewport"))
			handlers := record.Field("handlers")
			enabled := fixtureBool(t, record.Field("enabled"))
			keyMap := filePickerActionKeyMap(t, record.Field("mode"))
			picker := fixtureFilePicker(
				count,
				hidden,
				showHidden,
				selected,
				viewport,
				handlers,
				enabled,
			)
			resolved, err := tui.ResolveActions(
				tui.NewNodeID("files"),
				picker.ActionDescriptors(),
				[]tui.KeyScope{tui.NewKeyScope(tui.NewNodeID("scope"), keyMap)},
			)
			if err != nil {
				t.Fatal(err)
			}
			assertFilePickerActionGroup(
				t,
				resolved,
				filePickerFixtureKeyGroups(record.Field("keys")),
				filePickerFixtureBools(t, record.Field("available")),
			)

			app := &filePickerActionApp{
				count: count, hidden: hidden, showHidden: showHidden, selected: selected,
				viewport: viewport, handlers: handlers, enabled: enabled, keyMap: keyMap,
			}
			runtime, err := tui.NewRuntimeWithClock(
				app,
				tui.NewRuntimeConfig(tui.Size{Width: 40, Height: 20}),
				tui.NewVirtualClock(),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer runtime.Close()
			if _, err := runtime.RenderIfDirty(); err != nil {
				t.Fatal(err)
			}
			if record.Field("focus") == "files" {
				focused, err := runtime.RequestFocus(tui.NewNodeID("files"))
				if err != nil || !focused {
					t.Fatalf("focus = %t, %v", focused, err)
				}
				groups, err := runtime.ActiveActionGroups()
				if err != nil {
					t.Fatal(err)
				}
				if len(groups) != 1 || groups[0].Owner() != tui.NewNodeID("files") {
					t.Fatalf("action groups = %#v", groups)
				}
			}

			event := filePickerActionEvent(
				t,
				record.Field("event"),
				count,
				hidden,
				showHidden,
				selected,
				viewport,
			)
			dispatch, err := runtime.DispatchEvent(event)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runtime.ProcessPending(); err != nil {
				t.Fatal(err)
			}
			expectedMessages := filePickerFixtureMessages(t, record.Field("messages"))
			if !slices.Equal(app.messages, expectedMessages) {
				t.Errorf("messages = %#v, want %#v", app.messages, expectedMessages)
			}
			if dispatch.Messages() != len(expectedMessages) {
				t.Errorf("dispatched messages = %d, want %d", dispatch.Messages(), len(expectedMessages))
			}
			if dispatch.Consumed() != fixtureBool(t, record.Field("consumed")) {
				t.Errorf("consumed = %t", dispatch.Consumed())
			}
			expectedFocus, expectsFocus := filePickerFixtureFocus(record.Field("focus-after"))
			actualFocus, hasFocus := runtime.Interaction().Focused()
			if hasFocus != expectsFocus || expectsFocus && actualFocus != expectedFocus {
				t.Errorf("focus = %q, %t, want %q, %t", actualFocus, hasFocus, expectedFocus, expectsFocus)
			}
			if expected := fixtureInt(t, record.Field("selected-after")); app.selected != expected {
				t.Errorf("selected = %d, want %d", app.selected, expected)
			}
		})
	}
}

func TestFilePickerRootConflictIsReportedBeforeDispatch(t *testing.T) {
	binding := []tui.KeyBinding{
		tui.NewKeyBinding(tui.NewCharacterKeyStroke('x', vt.Modifiers{})),
	}
	keyMap, err := tui.NewKeyMap().Rebind(SelectionPreviousActionID, binding)
	if err != nil {
		t.Fatal(err)
	}
	keyMap, err = keyMap.Rebind(SelectionPreviousPageActionID, binding)
	if err != nil {
		t.Fatal(err)
	}
	app := &filePickerActionApp{
		count: 5, selected: 2, viewport: 3, handlers: "both", enabled: true, keyMap: keyMap,
	}
	runtime, err := tui.NewRuntimeWithClock(
		app,
		tui.NewRuntimeConfig(tui.Size{Width: 40, Height: 10}),
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
	if conflict.Kind() != tui.ConflictAmbiguousBinding || conflict.Owner() != tui.NewNodeID("files") {
		t.Fatalf("conflict = kind %d owner %s", conflict.Kind(), conflict.Owner())
	}
	expected := []tui.ActionID{SelectionPreviousActionID, SelectionPreviousPageActionID}
	if !slices.Equal(conflict.Actions(), expected) {
		t.Fatalf("conflict actions = %#v, want %#v", conflict.Actions(), expected)
	}
	if len(app.messages) != 0 {
		t.Fatalf("conflicting action emitted messages = %#v", app.messages)
	}
}

func fixtureFilePicker(
	count int,
	hidden []int,
	showHidden bool,
	selected int,
	viewport int,
	handlers string,
	enabled bool,
) FilePicker[filePickerFixtureMessage] {
	picker := NewFilePicker(
		tui.NewNodeID("files"),
		filePickerFixtureEntries(count, hidden),
		selected,
		func(index int) filePickerFixtureMessage {
			return filePickerFixtureMessage{kind: "select", index: index}
		},
	).ShowHidden(showHidden).Viewport(viewport).Enabled(enabled)
	if handlers == "both" || handlers == "open" {
		picker = picker.OnOpen(func(index int) filePickerFixtureMessage {
			return filePickerFixtureMessage{kind: "open", index: index}
		})
	}
	if handlers == "both" || handlers == "back" {
		picker = picker.OnBack(func() filePickerFixtureMessage {
			return filePickerFixtureMessage{kind: "back"}
		})
	}
	return picker
}

func filePickerFixtureEntries(count int, hidden []int) []FilePickerEntry {
	entries := make([]FilePickerEntry, count)
	for index := range entries {
		id := tui.NewNodeID("entry-" + strconv.Itoa(index))
		name := "Entry " + strconv.Itoa(index)
		path := strconv.Itoa(index)
		if index%2 == 0 {
			entries[index] = NewFilePickerFile(id, name, path)
		} else {
			entries[index] = NewFilePickerDirectory(id, name, path)
		}
		entries[index] = entries[index].WithHidden(slices.Contains(hidden, index))
	}
	return entries
}

func filePickerActionKeyMap(t *testing.T, mode string) tui.KeyMap {
	t.Helper()
	character := func(value rune) []tui.KeyBinding {
		return []tui.KeyBinding{
			tui.NewKeyBinding(tui.NewCharacterKeyStroke(value, vt.Modifiers{})),
		}
	}
	keyMap := tui.NewKeyMap()
	var (
		resolved tui.KeyMap
		err      error
	)
	switch mode {
	case "default":
		return keyMap
	case "previous-k":
		resolved, err = keyMap.Rebind(SelectionPreviousActionID, character('k'))
	case "activate-o":
		resolved, err = keyMap.Rebind(ActivateActionID, character('o'))
	case "previous-page-u":
		resolved, err = keyMap.Rebind(SelectionPreviousPageActionID, character('u'))
	case "back-b":
		resolved, err = keyMap.Rebind(NavigationBackActionID, character('b'))
	case "unbind-next":
		resolved, err = keyMap.Rebind(SelectionNextActionID, nil)
	case "unbind-activate":
		resolved, err = keyMap.Rebind(ActivateActionID, nil)
	default:
		t.Fatalf("unknown FilePicker action mode %q", mode)
	}
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func filePickerActionEvent(
	t *testing.T,
	value string,
	count int,
	hidden []int,
	showHidden bool,
	selected int,
	viewport int,
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
	case "right":
		return keyboard(vt.KeyRight, 0, vt.Modifiers{}, vt.KeyPress)
	case "up":
		return keyboard(vt.KeyUp, 0, vt.Modifiers{}, vt.KeyPress)
	case "down":
		return keyboard(vt.KeyDown, 0, vt.Modifiers{}, vt.KeyPress)
	case "home":
		return keyboard(vt.KeyHome, 0, vt.Modifiers{}, vt.KeyPress)
	case "end":
		return keyboard(vt.KeyEnd, 0, vt.Modifiers{}, vt.KeyPress)
	case "page-up":
		return keyboard(vt.KeyPageUp, 0, vt.Modifiers{}, vt.KeyPress)
	case "page-down":
		return keyboard(vt.KeyPageDown, 0, vt.Modifiers{}, vt.KeyPress)
	case "left":
		return keyboard(vt.KeyLeft, 0, vt.Modifiers{}, vt.KeyPress)
	case "backspace":
		return keyboard(vt.KeyBackspace, 0, vt.Modifiers{}, vt.KeyPress)
	case "repeat-down":
		return keyboard(vt.KeyDown, 0, vt.Modifiers{}, vt.KeyRepeat)
	case "unknown-down":
		return keyboard(vt.KeyDown, 0, vt.Modifiers{}, vt.KeyActionUnknown)
	case "repeat-enter":
		return keyboard(vt.KeyEnter, 0, vt.Modifiers{}, vt.KeyRepeat)
	case "release-down":
		return keyboard(vt.KeyDown, 0, vt.Modifiers{}, vt.KeyRelease)
	case "shift-down":
		return keyboard(vt.KeyDown, 0, vt.Modifiers{Shift: true}, vt.KeyPress)
	case "control-enter":
		return keyboard(vt.KeyEnter, 0, vt.Modifiers{Control: true}, vt.KeyPress)
	case "k":
		return keyboard(vt.KeyCharacter, 'k', vt.Modifiers{}, vt.KeyPress)
	case "o":
		return keyboard(vt.KeyCharacter, 'o', vt.Modifiers{}, vt.KeyPress)
	case "u":
		return keyboard(vt.KeyCharacter, 'u', vt.Modifiers{}, vt.KeyPress)
	case "b":
		return keyboard(vt.KeyCharacter, 'b', vt.Modifiers{}, vt.KeyPress)
	default:
		if strings.HasPrefix(value, "mouse-") {
			kind, button, candidate := filePickerPointerEvent(t, value)
			y := filePickerPointerY(count, hidden, showHidden, selected, viewport, candidate)
			return vt.Event{
				Kind: vt.EventMouse,
				Mouse: vt.MouseEvent{
					Kind: kind, Button: button, X: 0, Y: y,
				},
			}
		}
		t.Fatalf("unknown FilePicker action event %q", value)
		return vt.Event{}
	}
}

func filePickerPointerEvent(t *testing.T, value string) (vt.MouseKind, vt.MouseButton, int) {
	t.Helper()
	types := []struct {
		prefix string
		kind   vt.MouseKind
		button vt.MouseButton
	}{
		{prefix: "mouse-left-press-", kind: vt.MousePress, button: vt.MouseLeft},
		{prefix: "mouse-right-press-", kind: vt.MousePress, button: vt.MouseRight},
		{prefix: "mouse-left-release-", kind: vt.MouseRelease, button: vt.MouseLeft},
	}
	for _, eventType := range types {
		if candidate, ok := strings.CutPrefix(value, eventType.prefix); ok {
			return eventType.kind, eventType.button, fixtureInt(t, candidate)
		}
	}
	t.Fatalf("invalid FilePicker pointer event %q", value)
	return 0, 0, 0
}

func filePickerPointerY(
	count int,
	hidden []int,
	showHidden bool,
	selected int,
	viewport int,
	candidate int,
) uint32 {
	visible := filePickerVisibleIndices(filePickerFixtureEntries(count, hidden), showHidden)
	position := slices.Index(visible, candidate)
	if position < 0 {
		return 0
	}
	selectedPosition, ok := normalizedListSelection(visible, selected)
	if !ok {
		return 0
	}
	start := 0
	if viewport > 0 {
		start, _ = treeViewportRange(len(visible), selectedPosition, viewport)
	}
	return uint32(max(position-start, 0))
}

func assertFilePickerActionGroup(
	t *testing.T,
	resolved tui.ResolvedActions,
	expectedKeys [][]string,
	expectedAvailable []bool,
) {
	t.Helper()
	expectedIDs := []tui.ActionID{
		ActivateActionID,
		SelectionPreviousActionID,
		SelectionNextActionID,
		SelectionFirstActionID,
		SelectionLastActionID,
		SelectionPreviousPageActionID,
		SelectionNextPageActionID,
		NavigationBackActionID,
	}
	expectedLabels := []string{
		"Activate", "Previous", "Next", "First", "Last", "Previous page", "Next page", "Back",
	}
	if resolved.Owner() != tui.NewNodeID("files") {
		t.Fatalf("owner = %q", resolved.Owner())
	}
	actions := resolved.Actions()
	if len(actions) != len(expectedIDs) || len(expectedKeys) != len(expectedIDs) || len(expectedAvailable) != len(expectedIDs) {
		t.Fatalf(
			"actions = %d, key groups = %d, availability = %d",
			len(actions), len(expectedKeys), len(expectedAvailable),
		)
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
		expectedAvailability := tui.ActionDisabledPassThrough
		if expectedAvailable[index] {
			expectedAvailability = tui.ActionEnabled
		}
		if action.Availability() != expectedAvailability {
			t.Errorf(
				"action %s availability = %d, want %d",
				expectedIDs[index], action.Availability(), expectedAvailability,
			)
		}
	}
}

func filePickerFixtureMessages(t *testing.T, value string) []filePickerFixtureMessage {
	t.Helper()
	if value == "-" {
		return nil
	}
	parts := strings.Split(value, ",")
	messages := make([]filePickerFixtureMessage, len(parts))
	for index, part := range parts {
		if part == "back" {
			messages[index] = filePickerFixtureMessage{kind: "back"}
			continue
		}
		kind, rawIndex, ok := strings.Cut(part, ":")
		if !ok || kind != "select" && kind != "open" {
			t.Fatalf("invalid FilePicker fixture message %q", part)
		}
		messages[index] = filePickerFixtureMessage{kind: kind, index: fixtureInt(t, rawIndex)}
	}
	return messages
}

func filePickerFixtureInts(t *testing.T, value string) []int {
	t.Helper()
	if value == "-" {
		return nil
	}
	parts := strings.Split(value, ",")
	values := make([]int, len(parts))
	for index, part := range parts {
		values[index] = fixtureInt(t, part)
	}
	return values
}

func filePickerFixtureBools(t *testing.T, value string) []bool {
	t.Helper()
	parts := strings.Split(value, ",")
	values := make([]bool, len(parts))
	for index, part := range parts {
		values[index] = fixtureBool(t, part)
	}
	return values
}

func filePickerFixtureKeyGroups(value string) [][]string {
	groups := strings.Split(value, "|")
	output := make([][]string, len(groups))
	for index, group := range groups {
		if group != "-" {
			output[index] = strings.Split(group, ",")
		}
	}
	return output
}

func filePickerFixtureFocus(value string) (tui.NodeID, bool) {
	if value == "none" {
		return "", false
	}
	return tui.NewNodeID(value), true
}
