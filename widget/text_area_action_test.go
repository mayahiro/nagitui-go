package widget

import (
	"errors"
	"slices"
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

type textAreaActionMessage struct {
	kind  string
	state TextAreaState
}

type textAreaActionApp struct {
	state    TextAreaState
	handlers string
	enabled  bool
	keyMap   tui.KeyMap
	messages []textAreaActionMessage
}

func (*textAreaActionApp) Init() tui.Effect[textAreaActionMessage] {
	return tui.NoneEffect[textAreaActionMessage]()
}

func (a *textAreaActionApp) Update(message textAreaActionMessage) tui.Effect[textAreaActionMessage] {
	if message.kind == "change" {
		a.state = message.state
	}
	a.messages = append(a.messages, message)
	return tui.NoneEffect[textAreaActionMessage]()
}

func (*textAreaActionApp) Subscriptions() tui.Subscription[textAreaActionMessage] {
	return tui.NoneSubscription[textAreaActionMessage]()
}

func (a *textAreaActionApp) View(tui.ViewContext) tui.Node[textAreaActionMessage] {
	area := NewTextArea(
		tui.NewNodeID("area"),
		a.state,
		func(state TextAreaState) textAreaActionMessage {
			return textAreaActionMessage{kind: "change", state: state}
		},
	).Enabled(a.enabled)
	if textAreaHasActionHandler(a.handlers, "undo") {
		area = area.OnUndo(func() textAreaActionMessage {
			return textAreaActionMessage{kind: "undo"}
		})
	}
	if textAreaHasActionHandler(a.handlers, "redo") {
		area = area.OnRedo(func() textAreaActionMessage {
			return textAreaActionMessage{kind: "redo"}
		})
	}
	return tui.Padding(area.Node(), tui.UniformInsets(0)).
		WithKeyScope(tui.NewKeyScope(tui.NewNodeID("scope"), a.keyMap))
}

func TestTextAreaActionsMatchSharedFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t,
		"widgets/text-area-action.txt",
		"widget-text-area-action",
		"initial", "cursor", "anchor", "handlers", "mode", "enabled", "event",
		"message", "expected", "expected-cursor", "expected-anchor", "consumed", "focus",
	)
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			handlers := record.Field("handlers")
			enabled := fixtureBool(t, record.Field("enabled"))
			keyMap := textAreaActionKeyMap(t, record.Field("mode"))
			app := &textAreaActionApp{
				state: textAreaActionFixtureState(
					t, record.Text("initial"), record.Field("cursor"), record.Field("anchor"),
				),
				handlers: handlers,
				enabled:  enabled,
				keyMap:   keyMap,
			}
			runtime, err := tui.NewRuntimeWithClock(
				app,
				tui.NewRuntimeConfig(tui.Size{Width: 24, Height: 6}),
				tui.NewVirtualClock(),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer runtime.Close()
			if _, err := runtime.RenderIfDirty(); err != nil {
				t.Fatal(err)
			}

			var resolved tui.ResolvedActions
			if enabled {
				focused, err := runtime.RequestFocus(tui.NewNodeID("area"))
				if err != nil || !focused {
					t.Fatalf("RequestFocus = %t, %v", focused, err)
				}
				groups, err := runtime.ActiveActionGroups()
				if err != nil {
					t.Fatal(err)
				}
				if len(groups) != 1 || groups[0].Owner() != tui.NewNodeID("area") {
					t.Fatalf("active action groups = %#v", groups)
				}
				resolved = groups[0]
			} else {
				focused, err := runtime.RequestFocus(tui.NewNodeID("area"))
				if err != nil || focused {
					t.Fatalf("RequestFocus = %t, %v", focused, err)
				}
				resolved, err = tui.ResolveActions(
					tui.NewNodeID("area"),
					textAreaActionDescriptorsForTest(enabled, handlers),
					[]tui.KeyScope{tui.NewKeyScope(tui.NewNodeID("scope"), keyMap)},
				)
				if err != nil {
					t.Fatal(err)
				}
			}
			assertTextAreaActions(t, resolved, enabled, handlers, record.Field("mode"))

			dispatch, err := runtime.DispatchEvent(textAreaActionEvent(t, record.Field("event")))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runtime.ProcessPending(); err != nil {
				t.Fatal(err)
			}

			actualMessages := make([]string, len(app.messages))
			for index, message := range app.messages {
				actualMessages[index] = message.kind
			}
			expectedMessages := textAreaActionFixtureMessages(record.Field("message"))
			if !slices.Equal(actualMessages, expectedMessages) {
				t.Errorf("messages = %#v, want %#v", actualMessages, expectedMessages)
			}
			expectedState := textAreaActionFixtureState(
				t, record.Text("expected"), record.Field("expected-cursor"), record.Field("expected-anchor"),
			)
			if app.state != expectedState {
				t.Errorf("state = %#v, want %#v", app.state, expectedState)
			}
			if dispatch.Messages() != len(expectedMessages) {
				t.Errorf("dispatched messages = %d, want %d", dispatch.Messages(), len(expectedMessages))
			}
			if dispatch.Consumed() != fixtureBool(t, record.Field("consumed")) {
				t.Errorf("consumed = %t", dispatch.Consumed())
			}
			focused, hasFocus := runtime.Interaction().Focused()
			expectsFocus := record.Field("focus") == "area"
			if hasFocus != expectsFocus || expectsFocus && focused != tui.NewNodeID("area") {
				t.Errorf("focus = %q, %t, want area = %t", focused, hasFocus, expectsFocus)
			}
		})
	}
}

func TestTextAreaSameOwnerConflictIsReportedBeforeEditing(t *testing.T) {
	binding := []tui.KeyBinding{
		tui.NewKeyBinding(tui.NewCharacterKeyStroke('x', vt.Modifiers{})),
	}
	keyMap, err := tui.NewKeyMap().Rebind(tui.TextCursorLeftActionID, binding)
	if err != nil {
		t.Fatal(err)
	}
	keyMap, err = keyMap.Rebind(tui.TextCursorRightActionID, binding)
	if err != nil {
		t.Fatal(err)
	}
	app := &textAreaActionApp{
		state: NewTextAreaState("ab", 1), handlers: "both", enabled: true, keyMap: keyMap,
	}
	runtime, err := tui.NewRuntimeWithClock(
		app,
		tui.NewRuntimeConfig(tui.Size{Width: 12, Height: 2}),
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
	if conflict.Kind() != tui.ConflictAmbiguousBinding || conflict.Owner() != tui.NewNodeID("area") {
		t.Fatalf("conflict = kind %d owner %s", conflict.Kind(), conflict.Owner())
	}
	if expected := []tui.ActionID{tui.TextCursorLeftActionID, tui.TextCursorRightActionID}; !slices.Equal(conflict.Actions(), expected) {
		t.Fatalf("conflict actions = %#v, want %#v", conflict.Actions(), expected)
	}
	if len(app.messages) != 0 || app.state != NewTextAreaState("ab", 1) {
		t.Fatalf("conflicting action changed app = %#v %#v", app.messages, app.state)
	}
}

func TestTextAreaHistoryAvailabilityControlsConflicts(t *testing.T) {
	keyMap, err := tui.NewKeyMap().Rebind(
		tui.TextUndoActionID,
		[]tui.KeyBinding{tui.NewKeyBinding(tui.NewKeyStroke(vt.KeyLeft, vt.Modifiers{}))},
	)
	if err != nil {
		t.Fatal(err)
	}
	availableApp := &textAreaActionApp{
		state: NewTextAreaState("ab", 1), handlers: "redo", enabled: true, keyMap: keyMap,
	}
	availableRuntime, err := tui.NewRuntimeWithClock(
		availableApp,
		tui.NewRuntimeConfig(tui.Size{Width: 12, Height: 2}),
		tui.NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer availableRuntime.Close()
	if _, err := availableRuntime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if focused, err := availableRuntime.RequestFocus(tui.NewNodeID("area")); err != nil || !focused {
		t.Fatalf("RequestFocus = %t, %v", focused, err)
	}
	if _, err := availableRuntime.DispatchEvent(textAreaActionEvent(t, "left")); err != nil {
		t.Fatal(err)
	}
	if _, err := availableRuntime.ProcessPending(); err != nil {
		t.Fatal(err)
	}
	if availableApp.state != NewTextAreaState("ab", 0) {
		t.Fatalf("state = %#v, want cursor 0", availableApp.state)
	}

	conflictingApp := &textAreaActionApp{
		state: NewTextAreaState("ab", 1), handlers: "both", enabled: true, keyMap: keyMap,
	}
	conflictingRuntime, err := tui.NewRuntimeWithClock(
		conflictingApp,
		tui.NewRuntimeConfig(tui.Size{Width: 12, Height: 2}),
		tui.NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conflictingRuntime.Close()
	frame, err := conflictingRuntime.RenderIfDirty()
	if frame != nil {
		t.Fatal("conflicting frame was published")
	}
	var conflict *tui.BindingConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("RenderIfDirty error = %v, want BindingConflictError", err)
	}
	expected := []tui.ActionID{tui.TextCursorLeftActionID, tui.TextUndoActionID}
	if !slices.Equal(conflict.Actions(), expected) {
		t.Fatalf("conflict actions = %#v, want %#v", conflict.Actions(), expected)
	}
}

func textAreaActionDescriptorsForTest(enabled bool, handlers string) []tui.ActionDescriptor {
	area := NewTextArea(
		tui.NewNodeID("area"),
		TextAreaState{},
		func(state TextAreaState) textAreaActionMessage {
			return textAreaActionMessage{kind: "change", state: state}
		},
	).Enabled(enabled)
	if textAreaHasActionHandler(handlers, "undo") {
		area = area.OnUndo(func() textAreaActionMessage { return textAreaActionMessage{kind: "undo"} })
	}
	if textAreaHasActionHandler(handlers, "redo") {
		area = area.OnRedo(func() textAreaActionMessage { return textAreaActionMessage{kind: "redo"} })
	}
	return area.ActionDescriptors()
}

func textAreaActionKeyMap(t *testing.T, mode string) tui.KeyMap {
	t.Helper()
	keyMap := tui.NewKeyMap()
	var err error
	switch mode {
	case "default":
		return keyMap
	case "line-break-x":
		keyMap, err = keyMap.Rebind(
			tui.TextInsertLineBreakActionID,
			[]tui.KeyBinding{tui.NewKeyBinding(tui.NewCharacterKeyStroke('x', vt.Modifiers{}))},
		)
	case "unbind-delete-forward":
		keyMap, err = keyMap.Rebind(tui.TextDeleteForwardActionID, nil)
	default:
		t.Fatalf("unknown TextArea action mode %q", mode)
	}
	if err != nil {
		t.Fatal(err)
	}
	return keyMap
}

func assertTextAreaActions(
	t *testing.T,
	resolved tui.ResolvedActions,
	enabled bool,
	handlers string,
	mode string,
) {
	t.Helper()
	expectedIDs := []tui.ActionID{
		tui.TextCursorLeftActionID,
		tui.TextCursorRightActionID,
		tui.TextCursorUpActionID,
		tui.TextCursorDownActionID,
		tui.TextCursorLineStartActionID,
		tui.TextCursorLineEndActionID,
		tui.TextSelectionExtendLeftActionID,
		tui.TextSelectionExtendRightActionID,
		tui.TextSelectionExtendUpActionID,
		tui.TextSelectionExtendDownActionID,
		tui.TextSelectionExtendLineStartActionID,
		tui.TextSelectionExtendLineEndActionID,
		tui.TextSelectAllActionID,
		tui.TextDeleteBackwardActionID,
		tui.TextDeleteForwardActionID,
		tui.TextInsertLineBreakActionID,
		tui.TextUndoActionID,
		tui.TextRedoActionID,
	}
	expectedLabels := []string{
		"Move cursor left",
		"Move cursor right",
		"Move cursor up",
		"Move cursor down",
		"Move to line start",
		"Move to line end",
		"Extend selection left",
		"Extend selection right",
		"Extend selection up",
		"Extend selection down",
		"Extend selection to line start",
		"Extend selection to line end",
		"Select all",
		"Delete backward",
		"Delete forward",
		"Insert line break",
		"Undo",
		"Redo",
	}
	expectedKeys := [][]string{
		{"Left"},
		{"Right"},
		{"Up"},
		{"Down"},
		{"Home"},
		{"End"},
		{"Shift+Left"},
		{"Shift+Right"},
		{"Shift+Up"},
		{"Shift+Down"},
		{"Shift+Home"},
		{"Shift+End"},
		{"Ctrl+a"},
		{"Backspace"},
		{"Delete"},
		{"Enter"},
		{"Ctrl+z"},
		{"Ctrl+y", "Ctrl+Shift+z"},
	}
	if mode == "line-break-x" {
		expectedKeys[15] = []string{"x"}
	} else if mode == "unbind-delete-forward" {
		expectedKeys[14] = nil
	}
	actions := resolved.Actions()
	if len(actions) != len(expectedIDs) {
		t.Fatalf("actions = %d, want %d", len(actions), len(expectedIDs))
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
			t.Errorf("action %s keys = %#v, want %#v", action.ID(), keys, expectedKeys[index])
		}
		expectedAvailable := enabled
		if index == 16 {
			expectedAvailable = expectedAvailable && textAreaHasActionHandler(handlers, "undo")
		} else if index == 17 {
			expectedAvailable = expectedAvailable && textAreaHasActionHandler(handlers, "redo")
		}
		if actual := action.Availability() == tui.ActionEnabled; actual != expectedAvailable {
			t.Errorf("action %s available = %t, want %t", action.ID(), actual, expectedAvailable)
		}
	}
}

func textAreaActionEvent(t *testing.T, value string) vt.Event {
	t.Helper()
	shift := vt.Modifiers{Shift: true}
	control := vt.Modifiers{Control: true}
	controlShift := vt.Modifiers{Control: true, Shift: true}
	switch value {
	case "text-X":
		return vt.Event{Kind: vt.EventText, Text: "X"}
	case "text-x":
		return vt.Event{Kind: vt.EventText, Text: "x"}
	case "paste-X-newline-Y":
		return vt.Event{Kind: vt.EventPaste, Text: "X\nY"}
	case "paste-x":
		return vt.Event{Kind: vt.EventPaste, Text: "x"}
	case "enter":
		return keyEvent(vt.KeyEnter, 0, vt.Modifiers{}, vt.KeyPress)
	case "repeat-enter":
		return keyEvent(vt.KeyEnter, 0, vt.Modifiers{}, vt.KeyRepeat)
	case "shift-enter":
		return keyEvent(vt.KeyEnter, 0, shift, vt.KeyPress)
	case "control-enter":
		return keyEvent(vt.KeyEnter, 0, control, vt.KeyPress)
	case "left":
		return keyEvent(vt.KeyLeft, 0, vt.Modifiers{}, vt.KeyPress)
	case "right":
		return keyEvent(vt.KeyRight, 0, vt.Modifiers{}, vt.KeyPress)
	case "up":
		return keyEvent(vt.KeyUp, 0, vt.Modifiers{}, vt.KeyPress)
	case "down":
		return keyEvent(vt.KeyDown, 0, vt.Modifiers{}, vt.KeyPress)
	case "home":
		return keyEvent(vt.KeyHome, 0, vt.Modifiers{}, vt.KeyPress)
	case "end":
		return keyEvent(vt.KeyEnd, 0, vt.Modifiers{}, vt.KeyPress)
	case "shift-left":
		return keyEvent(vt.KeyLeft, 0, shift, vt.KeyPress)
	case "shift-up":
		return keyEvent(vt.KeyUp, 0, shift, vt.KeyPress)
	case "shift-home":
		return keyEvent(vt.KeyHome, 0, shift, vt.KeyPress)
	case "shift-end":
		return keyEvent(vt.KeyEnd, 0, shift, vt.KeyPress)
	case "control-a":
		return keyEvent(vt.KeyCharacter, 'a', control, vt.KeyPress)
	case "backspace":
		return keyEvent(vt.KeyBackspace, 0, vt.Modifiers{}, vt.KeyPress)
	case "delete":
		return keyEvent(vt.KeyDelete, 0, vt.Modifiers{}, vt.KeyPress)
	case "release-left":
		return keyEvent(vt.KeyLeft, 0, vt.Modifiers{}, vt.KeyRelease)
	case "control-z":
		return keyEvent(vt.KeyCharacter, 'z', control, vt.KeyPress)
	case "control-y":
		return keyEvent(vt.KeyCharacter, 'y', control, vt.KeyPress)
	case "control-shift-z":
		return keyEvent(vt.KeyCharacter, 'z', controlShift, vt.KeyPress)
	default:
		t.Fatalf("unknown TextArea action event %q", value)
		return vt.Event{}
	}
}

func textAreaActionFixtureState(t *testing.T, value, cursor, anchor string) TextAreaState {
	t.Helper()
	state := NewTextAreaState(value, fixtureInt(t, cursor))
	if anchor != "-" {
		state = state.Select(fixtureInt(t, anchor))
	}
	return state
}

func textAreaHasActionHandler(handlers, expected string) bool {
	return handlers == "both" || handlers == expected
}

func textAreaActionFixtureMessages(value string) []string {
	if value == "-" {
		return nil
	}
	return []string{value}
}
