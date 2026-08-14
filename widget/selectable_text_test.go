package widget

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

type selectableTextMessage struct {
	kind    string
	state   SelectableTextState
	request TextCopyRequest
	offset  tui.ScrollOffset
}

type selectableTextFixtureApp struct {
	content  SelectableTextContent
	state    SelectableTextState
	enabled  bool
	copy     bool
	keyMap   tui.KeyMap
	messages []selectableTextMessage
}

type selectableTextPointerApp struct {
	content  SelectableTextContent
	state    SelectableTextState
	enabled  bool
	options  tui.ParagraphOptions
	scroll   string
	messages []selectableTextMessage
}

func (*selectableTextPointerApp) Init() tui.Effect[selectableTextMessage] {
	return tui.NoneEffect[selectableTextMessage]()
}

func (*selectableTextPointerApp) Subscriptions() tui.Subscription[selectableTextMessage] {
	return tui.NoneSubscription[selectableTextMessage]()
}

func (a *selectableTextPointerApp) Update(message selectableTextMessage) tui.Effect[selectableTextMessage] {
	if message.kind == "change" {
		a.state = message.state
	}
	a.messages = append(a.messages, message)
	return tui.NoneEffect[selectableTextMessage]()
}

func (a *selectableTextPointerApp) View(tui.ViewContext) tui.Node[selectableTextMessage] {
	text := NewSelectableText(
		tui.NewNodeID("text"),
		a.content,
		a.state,
		func(state SelectableTextState) selectableTextMessage {
			return selectableTextMessage{kind: "change", state: state}
		},
	).Enabled(a.enabled).ParagraphOptions(a.options).Node()
	if a.scroll == "none" {
		return text
	}
	options := tui.DefaultScrollViewportOptions[selectableTextMessage]()
	switch a.scroll {
	case "vertical":
		options.Axis = tui.ScrollAxisVertical
	case "horizontal":
		options.Axis = tui.ScrollAxisHorizontal
	default:
		panic("invalid pointer scroll axis " + a.scroll)
	}
	options.OnScroll = func(state tui.ScrollState) selectableTextMessage {
		return selectableTextMessage{kind: "scroll", offset: state.Offset}
	}
	return tui.ScrollViewportWithOptions(tui.NewNodeID("scroll"), text, options)
}

func TestSelectableTextPointerSelectionMatchesSharedFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t,
		"widgets/selectable-text-pointer.txt",
		"widget-selectable-text-pointer",
		"content", "width", "height", "wrap", "alignment", "profile", "scroll",
		"enabled", "cursor", "anchor", "events", "expected-cursor", "expected-anchor",
		"messages", "consumed", "capture", "focus", "offset",
	)
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			content := NewPlainSelectableTextContent(record.Text("content"))
			state := content.NormalizeState(selectableTextFixtureState(
				t, record.Field("cursor"), record.Field("anchor"),
			))
			config := tui.NewRuntimeConfig(tui.Size{
				Width:  uint32(fixtureInt(t, record.Field("width"))),
				Height: uint32(fixtureInt(t, record.Field("height"))),
			})
			switch record.Field("profile") {
			case "modern":
				config.WidthProfile = celltext.ModernWidth()
			case "cjk":
				config.WidthProfile = celltext.CJKWidth()
			default:
				t.Fatalf("invalid pointer WidthProfile %q", record.Field("profile"))
			}
			options := tui.DefaultParagraphOptions()
			switch record.Field("wrap") {
			case "word":
				options.Wrap = tui.WrapWord
			case "hard":
				options.Wrap = tui.WrapHard
			case "none":
				options.Wrap = tui.WrapNone
			default:
				t.Fatalf("invalid pointer WrapMode %q", record.Field("wrap"))
			}
			switch record.Field("alignment") {
			case "start":
				options.Alignment = tui.AlignStart
			case "center":
				options.Alignment = tui.AlignCenter
			case "end":
				options.Alignment = tui.AlignEnd
			default:
				t.Fatalf("invalid pointer alignment %q", record.Field("alignment"))
			}
			app := &selectableTextPointerApp{
				content: content, state: state,
				enabled: fixtureBool(t, record.Field("enabled")),
				options: options, scroll: record.Field("scroll"),
			}
			runtime, err := tui.NewRuntimeWithClock(app, config, tui.NewVirtualClock())
			if err != nil {
				t.Fatal(err)
			}
			defer runtime.Close()
			if _, err := runtime.RenderIfDirty(); err != nil {
				t.Fatal(err)
			}
			var consumed []bool
			for _, event := range strings.Split(record.Field("events"), ",") {
				dispatch, err := runtime.DispatchEvent(selectableTextPointerFixtureEvent(t, event))
				if err != nil {
					t.Fatal(err)
				}
				consumed = append(consumed, dispatch.Consumed())
				if _, err := runtime.ProcessPending(); err != nil {
					t.Fatal(err)
				}
				if _, err := runtime.RenderIfDirty(); err != nil {
					t.Fatal(err)
				}
			}
			expectedState := selectableTextFixtureState(
				t, record.Field("expected-cursor"), record.Field("expected-anchor"),
			)
			if app.state != expectedState {
				t.Errorf("state = %#v, want %#v", app.state, expectedState)
			}
			if actual := selectableTextPointerMessages(app.messages); actual != record.Field("messages") {
				t.Errorf("messages = %q, want %q", actual, record.Field("messages"))
			}
			expectedConsumed := strings.Split(record.Field("consumed"), ",")
			if len(consumed) != len(expectedConsumed) {
				t.Fatalf("consumed = %#v, want %q", consumed, record.Field("consumed"))
			}
			for index := range consumed {
				if consumed[index] != fixtureBool(t, expectedConsumed[index]) {
					t.Errorf("consumed[%d] = %t, want %s", index, consumed[index], expectedConsumed[index])
				}
			}
			capture := "none"
			if id, ok := runtime.Interaction().PointerCapture(); ok {
				capture = id.String()
			}
			if capture != record.Field("capture") {
				t.Errorf("capture = %q, want %q", capture, record.Field("capture"))
			}
			focus := "none"
			if id, ok := runtime.Interaction().Focused(); ok {
				focus = id.String()
			}
			if focus != record.Field("focus") {
				t.Errorf("focus = %q, want %q", focus, record.Field("focus"))
			}
			expectedOffset := selectableTextFixtureScrollOffset(t, record.Field("offset"))
			if offset := runtime.Interaction().ScrollOffset(tui.NewNodeID("scroll")); offset != expectedOffset {
				t.Errorf("scroll offset = %#v, want %#v", offset, expectedOffset)
			}
		})
	}
}

func (*selectableTextFixtureApp) Init() tui.Effect[selectableTextMessage] {
	return tui.NoneEffect[selectableTextMessage]()
}

func (*selectableTextFixtureApp) Subscriptions() tui.Subscription[selectableTextMessage] {
	return tui.NoneSubscription[selectableTextMessage]()
}

func (a *selectableTextFixtureApp) Update(message selectableTextMessage) tui.Effect[selectableTextMessage] {
	if message.kind == "change" {
		a.state = message.state
	}
	a.messages = append(a.messages, message)
	return tui.NoneEffect[selectableTextMessage]()
}

func (a *selectableTextFixtureApp) View(tui.ViewContext) tui.Node[selectableTextMessage] {
	text := NewSelectableText(
		tui.NewNodeID("text"),
		a.content,
		a.state,
		func(state SelectableTextState) selectableTextMessage {
			return selectableTextMessage{kind: "change", state: state}
		},
	).Enabled(a.enabled)
	if a.copy {
		text = text.OnCopy(func(request TextCopyRequest) selectableTextMessage {
			return selectableTextMessage{kind: "copy", request: request}
		})
	}
	return tui.Padding(text.Node(), tui.UniformInsets(0)).
		WithKeyScope(tui.NewKeyScope(tui.NewNodeID("scope"), a.keyMap))
}

func TestSelectableTextActionsMatchSharedFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t,
		"widgets/selectable-text.txt",
		"widget-selectable-text",
		"content", "cursor", "anchor", "enabled", "copy", "hidden", "mode", "event",
		"expected-cursor", "expected-anchor", "message", "copy-kind", "copy-range",
		"copy-text", "consumed", "focus", "availability",
	)
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			content := selectableTextFixtureContent(
				record.Text("content"), fixtureBool(t, record.Field("hidden")),
			)
			state := content.NormalizeState(selectableTextFixtureState(
				t, record.Field("cursor"), record.Field("anchor"),
			))
			enabled := fixtureBool(t, record.Field("enabled"))
			copyHandler := fixtureBool(t, record.Field("copy"))

			if record.Field("availability") != "-" {
				text := NewSelectableText(
					tui.NewNodeID("text"), content, state,
					func(state SelectableTextState) selectableTextMessage {
						return selectableTextMessage{kind: "change", state: state}
					},
				).Enabled(enabled)
				if copyHandler {
					text = text.OnCopy(func(request TextCopyRequest) selectableTextMessage {
						return selectableTextMessage{kind: "copy", request: request}
					})
				}
				actionID := selectableTextFixtureActionID(t, record.Field("event"))
				var availability tui.ActionAvailability
				found := false
				for _, descriptor := range text.ActionDescriptors() {
					if descriptor.ID() == actionID {
						availability = descriptor.Availability()
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("action %q is missing", actionID)
				}
				if actual := selectableTextFixtureAvailability(t, availability); actual != record.Field("availability") {
					t.Errorf("availability = %q, want %q", actual, record.Field("availability"))
				}
			}

			app := &selectableTextFixtureApp{
				content: content,
				state:   state,
				enabled: enabled,
				copy:    copyHandler,
				keyMap:  selectableTextFixtureKeyMap(t, record.Field("mode")),
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

			focused, err := runtime.RequestFocus(tui.NewNodeID("text"))
			if err != nil || focused != enabled {
				t.Fatalf("RequestFocus = %t, %v, want %t", focused, err, enabled)
			}

			var dispatch *tui.EventDispatch
			if record.Field("event") != "none" {
				result, err := runtime.DispatchEvent(selectableTextFixtureEvent(t, record.Field("event")))
				if err != nil {
					t.Fatal(err)
				}
				dispatch = &result
				if _, err := runtime.ProcessPending(); err != nil {
					t.Fatal(err)
				}
			}

			expectedState := selectableTextFixtureState(
				t, record.Field("expected-cursor"), record.Field("expected-anchor"),
			)
			if app.state != expectedState {
				t.Errorf("state = %#v, want %#v", app.state, expectedState)
			}
			selectableTextAssertMessages(t, app.messages, record.Field("message"),
				record.Field("copy-kind"), record.Field("copy-range"), record.Text("copy-text"))
			selectableTextAssertDispatch(t, dispatch, record.Field("consumed"))
			focus, hasFocus := runtime.Interaction().Focused()
			expectsFocus := record.Field("focus") == "text"
			if hasFocus != expectsFocus || expectsFocus && focus != tui.NewNodeID("text") {
				t.Errorf("focus = %q, %t, want text = %t", focus, hasFocus, expectsFocus)
			}
		})
	}
}

func TestSelectableTextDeclaresTheOrderedActionContract(t *testing.T) {
	descriptors := NewSelectableText(
		tui.NewNodeID("text"),
		NewPlainSelectableTextContent("text"),
		NewSelectableTextStateWithSelection(4, 0),
		func(state SelectableTextState) SelectableTextState { return state },
	).OnCopy(func(TextCopyRequest) SelectableTextState { return SelectableTextState{} }).ActionDescriptors()
	expectedIDs := []tui.ActionID{
		tui.TextCursorLeftActionID,
		tui.TextCursorRightActionID,
		tui.TextCursorWordLeftActionID,
		tui.TextCursorWordRightActionID,
		tui.TextCursorLineStartActionID,
		tui.TextCursorLineEndActionID,
		tui.TextCursorDocumentStartActionID,
		tui.TextCursorDocumentEndActionID,
		tui.TextSelectionExtendLeftActionID,
		tui.TextSelectionExtendRightActionID,
		tui.TextSelectionExtendWordLeftActionID,
		tui.TextSelectionExtendWordRightActionID,
		tui.TextSelectionExtendLineStartActionID,
		tui.TextSelectionExtendLineEndActionID,
		tui.TextSelectionExtendDocumentStartActionID,
		tui.TextSelectionExtendDocumentEndActionID,
		tui.TextSelectAllActionID,
		tui.TextCopySelectionActionID,
		tui.TextCopyDocumentActionID,
	}
	expectedBindings := []string{
		"Left", "Right", "Ctrl+Left", "Ctrl+Right", "Home", "End", "Ctrl+Home", "Ctrl+End",
		"Shift+Left", "Shift+Right", "Ctrl+Shift+Left", "Ctrl+Shift+Right", "Shift+Home",
		"Shift+End", "Ctrl+Shift+Home", "Ctrl+Shift+End", "Ctrl+a", "Ctrl+c", "Ctrl+Shift+c",
	}
	if len(descriptors) != len(expectedIDs) {
		t.Fatalf("descriptors = %d, want %d", len(descriptors), len(expectedIDs))
	}
	for index, descriptor := range descriptors {
		if descriptor.ID() != expectedIDs[index] {
			t.Errorf("action %d = %q, want %q", index, descriptor.ID(), expectedIDs[index])
		}
		bindings := descriptor.DefaultBindings()
		if len(bindings) != 1 || bindings[0].Stroke().Notation() != expectedBindings[index] {
			t.Errorf("action %q bindings = %#v", descriptor.ID(), bindings)
			continue
		}
		expectedRepeat := tui.RepeatAllow
		if index >= len(descriptors)-2 {
			expectedRepeat = tui.RepeatInitialOnly
		}
		if bindings[0].RepeatPolicy() != expectedRepeat {
			t.Errorf("action %q repeat = %v, want %v", descriptor.ID(), bindings[0].RepeatPolicy(), expectedRepeat)
		}
	}
}

type selectableTextVisualApp struct {
	enabled bool
}

func (*selectableTextVisualApp) Init() tui.Effect[SelectableTextState] {
	return tui.NoneEffect[SelectableTextState]()
}

func (*selectableTextVisualApp) Subscriptions() tui.Subscription[SelectableTextState] {
	return tui.NoneSubscription[SelectableTextState]()
}

func (*selectableTextVisualApp) Update(SelectableTextState) tui.Effect[SelectableTextState] {
	return tui.NoneEffect[SelectableTextState]()
}

func (a *selectableTextVisualApp) View(tui.ViewContext) tui.Node[SelectableTextState] {
	return NewSelectableText(
		tui.NewNodeID("visual"),
		NewSelectableTextContent([]tui.TextSpan{
			tui.NewTextSpan("ab", vt.Style{Bold: true}),
			tui.NewTextSpan("cd", vt.Style{Foreground: vt.IndexedColor(2)}),
		}),
		NewSelectableTextStateWithSelection(3, 1),
		func(state SelectableTextState) SelectableTextState { return state },
	).Enabled(a.enabled).Node()
}

func TestSelectableTextMergesBaseSelectionFocusAndDisabledStyles(t *testing.T) {
	enabled, err := tui.NewRuntimeWithClock(
		&selectableTextVisualApp{enabled: true},
		tui.NewRuntimeConfig(tui.Size{Width: 4, Height: 1}),
		tui.NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer enabled.Close()
	initial, err := enabled.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	styles := selectableTextSurfaceStyles(t, initial, 4)
	if !styles[0].Bold || styles[0].Reverse || !styles[1].Reverse || !styles[2].Reverse {
		t.Errorf("initial styles = %#v", styles)
	}
	if styles[3].Foreground != vt.IndexedColor(2) {
		t.Errorf("final foreground = %+v", styles[3].Foreground)
	}

	if focused, err := enabled.RequestFocus(tui.NewNodeID("visual")); err != nil || !focused {
		t.Fatalf("RequestFocus = %t, %v", focused, err)
	}
	focused, err := enabled.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	for index, style := range selectableTextSurfaceStyles(t, focused, 4) {
		if !style.Underline {
			t.Errorf("focused style %d = %+v", index, style)
		}
	}

	disabled, err := tui.NewRuntimeWithClock(
		&selectableTextVisualApp{enabled: false},
		tui.NewRuntimeConfig(tui.Size{Width: 4, Height: 1}),
		tui.NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer disabled.Close()
	disabledFrame, err := disabled.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	if focused, err := disabled.RequestFocus(tui.NewNodeID("visual")); err != nil || focused {
		t.Fatalf("disabled RequestFocus = %t, %v", focused, err)
	}
	styles = selectableTextSurfaceStyles(t, disabledFrame, 4)
	for index, style := range styles {
		if !style.Dim {
			t.Errorf("disabled style %d = %+v", index, style)
		}
	}
	if !styles[1].Reverse {
		t.Errorf("disabled selection style = %+v", styles[1])
	}
}

func TestSelectableTextReportsSameOwnerBindingConflictsBeforeInput(t *testing.T) {
	keyMap, err := tui.NewKeyMap().Rebind(tui.TextCopyDocumentActionID, []tui.KeyBinding{
		tui.NewKeyBinding(tui.NewCharacterKeyStroke('c', vt.Modifiers{Control: true})),
	})
	if err != nil {
		t.Fatal(err)
	}
	initial := NewSelectableTextStateWithSelection(3, 0)
	app := &selectableTextFixtureApp{
		content: NewPlainSelectableTextContent("text"),
		state:   initial,
		enabled: true,
		copy:    true,
		keyMap:  keyMap,
	}
	runtime, err := tui.NewRuntimeWithClock(
		app,
		tui.NewRuntimeConfig(tui.Size{Width: 8, Height: 1}),
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
	if conflict.Kind() != tui.ConflictAmbiguousBinding || conflict.Owner() != tui.NewNodeID("text") {
		t.Fatalf("conflict = %#v", conflict)
	}
	if !slices.Equal(conflict.Actions(), []tui.ActionID{
		tui.TextCopySelectionActionID,
		tui.TextCopyDocumentActionID,
	}) {
		t.Fatalf("conflict actions = %#v", conflict.Actions())
	}
	if app.state != initial || len(app.messages) != 0 {
		t.Fatalf("app changed before conflict: %#v", app)
	}
}

func selectableTextSurfaceStyles(t *testing.T, frame *tui.Frame, width int32) []vt.Style {
	t.Helper()
	styles := make([]vt.Style, width)
	for x := int32(0); x < width; x++ {
		cell, ok := frame.Surface().Cell(x, 0)
		if !ok {
			t.Fatalf("cell %d is missing", x)
		}
		styles[x] = cell.Style()
	}
	return styles
}

func selectableTextFixtureContent(text string, hidden bool) SelectableTextContent {
	if hidden {
		return NewSelectableTextContent([]tui.TextSpan{
			tui.NewTextSpan(text, vt.Style{Hidden: true}),
		})
	}
	return NewPlainSelectableTextContent(text)
}

func selectableTextFixtureState(t *testing.T, cursor, anchor string) SelectableTextState {
	t.Helper()
	state := NewSelectableTextState(fixtureInt(t, cursor))
	if anchor != "-" {
		state = state.Select(fixtureInt(t, anchor))
	}
	return state
}

func selectableTextFixtureKeyMap(t *testing.T, mode string) tui.KeyMap {
	t.Helper()
	keyMap := tui.NewKeyMap()
	var err error
	switch mode {
	case "default":
		return keyMap
	case "selection-alt":
		keyMap, err = keyMap.Rebind(tui.TextCopySelectionActionID, []tui.KeyBinding{
			tui.NewKeyBinding(tui.NewCharacterKeyStroke('c', vt.Modifiers{Alt: true})),
		})
	case "selection-unbound":
		keyMap, err = keyMap.Rebind(tui.TextCopySelectionActionID, nil)
	default:
		t.Fatalf("invalid SelectableText key map mode %q", mode)
	}
	if err != nil {
		t.Fatal(err)
	}
	return keyMap
}

func selectableTextFixtureEvent(t *testing.T, value string) vt.Event {
	t.Helper()
	shift := vt.Modifiers{Shift: true}
	control := vt.Modifiers{Control: true}
	alt := vt.Modifiers{Alt: true}
	controlShift := vt.Modifiers{Control: true, Shift: true}
	switch value {
	case "left":
		return keyEvent(vt.KeyLeft, 0, vt.Modifiers{}, vt.KeyPress)
	case "right":
		return keyEvent(vt.KeyRight, 0, vt.Modifiers{}, vt.KeyPress)
	case "control-left":
		return keyEvent(vt.KeyLeft, 0, control, vt.KeyPress)
	case "control-right":
		return keyEvent(vt.KeyRight, 0, control, vt.KeyPress)
	case "home":
		return keyEvent(vt.KeyHome, 0, vt.Modifiers{}, vt.KeyPress)
	case "end":
		return keyEvent(vt.KeyEnd, 0, vt.Modifiers{}, vt.KeyPress)
	case "control-home":
		return keyEvent(vt.KeyHome, 0, control, vt.KeyPress)
	case "control-end":
		return keyEvent(vt.KeyEnd, 0, control, vt.KeyPress)
	case "shift-left":
		return keyEvent(vt.KeyLeft, 0, shift, vt.KeyPress)
	case "shift-right":
		return keyEvent(vt.KeyRight, 0, shift, vt.KeyPress)
	case "control-shift-left":
		return keyEvent(vt.KeyLeft, 0, controlShift, vt.KeyPress)
	case "control-shift-right":
		return keyEvent(vt.KeyRight, 0, controlShift, vt.KeyPress)
	case "shift-home":
		return keyEvent(vt.KeyHome, 0, shift, vt.KeyPress)
	case "shift-end":
		return keyEvent(vt.KeyEnd, 0, shift, vt.KeyPress)
	case "control-shift-home":
		return keyEvent(vt.KeyHome, 0, controlShift, vt.KeyPress)
	case "control-shift-end":
		return keyEvent(vt.KeyEnd, 0, controlShift, vt.KeyPress)
	case "control-a":
		return keyEvent(vt.KeyCharacter, 'a', control, vt.KeyPress)
	case "control-c":
		return keyEvent(vt.KeyCharacter, 'c', control, vt.KeyPress)
	case "control-shift-c":
		return keyEvent(vt.KeyCharacter, 'c', controlShift, vt.KeyPress)
	case "alt-c":
		return keyEvent(vt.KeyCharacter, 'c', alt, vt.KeyPress)
	case "repeat-left":
		return keyEvent(vt.KeyLeft, 0, vt.Modifiers{}, vt.KeyRepeat)
	case "repeat-control-c":
		return keyEvent(vt.KeyCharacter, 'c', control, vt.KeyRepeat)
	default:
		t.Fatalf("invalid SelectableText event %q", value)
		return vt.Event{}
	}
}

func selectableTextPointerFixtureEvent(t *testing.T, value string) vt.Event {
	t.Helper()
	kind, coordinates, ok := strings.Cut(value, "@")
	if !ok {
		t.Fatalf("invalid pointer event %q", value)
	}
	xText, yText, ok := strings.Cut(coordinates, ":")
	if !ok {
		t.Fatalf("invalid pointer coordinates %q", coordinates)
	}
	x, err := strconv.ParseUint(xText, 10, 32)
	if err != nil {
		t.Fatalf("invalid pointer x %q: %v", xText, err)
	}
	y, err := strconv.ParseUint(yText, 10, 32)
	if err != nil {
		t.Fatalf("invalid pointer y %q: %v", yText, err)
	}
	event := vt.MouseEvent{X: uint32(x), Y: uint32(y)}
	switch kind {
	case "press":
		event.Kind, event.Button = vt.MousePress, vt.MouseLeft
	case "shift-press":
		event.Kind, event.Button = vt.MousePress, vt.MouseLeft
		event.Modifiers.Shift = true
	case "right-press":
		event.Kind, event.Button = vt.MousePress, vt.MouseRight
	case "move":
		event.Kind, event.Button = vt.MouseMove, vt.MouseLeft
	case "release":
		event.Kind, event.Button = vt.MouseRelease, vt.MouseLeft
	default:
		t.Fatalf("invalid pointer event kind %q", kind)
	}
	return vt.Event{Kind: vt.EventMouse, Mouse: event}
}

func selectableTextPointerMessages(messages []selectableTextMessage) string {
	if len(messages) == 0 {
		return "-"
	}
	values := make([]string, len(messages))
	for index, message := range messages {
		switch message.kind {
		case "change":
			anchor := "-"
			if value, ok := message.state.SelectionAnchor(); ok {
				anchor = strconv.Itoa(value)
			}
			values[index] = "change:" + strconv.Itoa(message.state.Cursor()) + ":" + anchor
		case "scroll":
			values[index] = "scroll:" + strconv.FormatUint(uint64(message.offset.X), 10) +
				":" + strconv.FormatUint(uint64(message.offset.Y), 10)
		default:
			panic("unexpected message in pointer fixture: " + message.kind)
		}
	}
	return strings.Join(values, ",")
}

func selectableTextFixtureScrollOffset(t *testing.T, value string) tui.ScrollOffset {
	t.Helper()
	x, y, ok := strings.Cut(value, ":")
	if !ok {
		t.Fatalf("invalid scroll offset %q", value)
	}
	return tui.ScrollOffset{
		X: uint32(fixtureInt(t, x)),
		Y: uint32(fixtureInt(t, y)),
	}
}

func selectableTextFixtureActionID(t *testing.T, event string) tui.ActionID {
	t.Helper()
	switch event {
	case "left", "repeat-left":
		return tui.TextCursorLeftActionID
	case "right":
		return tui.TextCursorRightActionID
	case "control-left":
		return tui.TextCursorWordLeftActionID
	case "control-right":
		return tui.TextCursorWordRightActionID
	case "home":
		return tui.TextCursorLineStartActionID
	case "end":
		return tui.TextCursorLineEndActionID
	case "control-home":
		return tui.TextCursorDocumentStartActionID
	case "control-end":
		return tui.TextCursorDocumentEndActionID
	case "shift-left":
		return tui.TextSelectionExtendLeftActionID
	case "shift-right":
		return tui.TextSelectionExtendRightActionID
	case "control-shift-left":
		return tui.TextSelectionExtendWordLeftActionID
	case "control-shift-right":
		return tui.TextSelectionExtendWordRightActionID
	case "shift-home":
		return tui.TextSelectionExtendLineStartActionID
	case "shift-end":
		return tui.TextSelectionExtendLineEndActionID
	case "control-shift-home":
		return tui.TextSelectionExtendDocumentStartActionID
	case "control-shift-end":
		return tui.TextSelectionExtendDocumentEndActionID
	case "control-a":
		return tui.TextSelectAllActionID
	case "control-c", "repeat-control-c", "alt-c":
		return tui.TextCopySelectionActionID
	case "control-shift-c":
		return tui.TextCopyDocumentActionID
	default:
		t.Fatalf("invalid SelectableText availability event %q", event)
		return ""
	}
}

func selectableTextAssertMessages(
	t *testing.T,
	messages []selectableTextMessage,
	expected, copyKind, copyRange, copyText string,
) {
	t.Helper()
	switch expected {
	case "-":
		if len(messages) != 0 {
			t.Errorf("messages = %#v, want none", messages)
		}
	case "change":
		if len(messages) != 1 || messages[0].kind != "change" {
			t.Errorf("messages = %#v, want change", messages)
		}
	case "copy":
		if len(messages) != 1 || messages[0].kind != "copy" {
			t.Fatalf("messages = %#v, want copy", messages)
		}
		request := messages[0].request
		expectedKind := TextCopySelection
		if copyKind == "document" {
			expectedKind = TextCopyDocument
		} else if copyKind != "selection" {
			t.Fatalf("invalid SelectableText copy kind %q", copyKind)
		}
		start, end := selectableTextFixtureRange(t, copyRange)
		expectedRequest := TextCopyRequest{
			Source: tui.NewNodeID("text"), Kind: expectedKind, Text: copyText, Start: start, End: end,
		}
		if request != expectedRequest {
			t.Errorf("copy request = %#v, want %#v", request, expectedRequest)
		}
	default:
		t.Fatalf("invalid SelectableText message %q", expected)
	}
}

func selectableTextAssertDispatch(t *testing.T, dispatch *tui.EventDispatch, consumed string) {
	t.Helper()
	if dispatch == nil {
		if consumed != "-" {
			t.Errorf("missing dispatch, want consumed %q", consumed)
		}
		return
	}
	if consumed == "-" {
		t.Fatal("unexpected dispatch")
	}
	if dispatch.Consumed() != fixtureBool(t, consumed) {
		t.Errorf("consumed = %t, want %s", dispatch.Consumed(), consumed)
	}
}

func selectableTextFixtureAvailability(t *testing.T, availability tui.ActionAvailability) string {
	t.Helper()
	switch availability {
	case tui.ActionEnabled:
		return "enabled"
	case tui.ActionDisabledPassThrough:
		return "pass"
	case tui.ActionDisabledConsume:
		return "consume"
	default:
		t.Fatalf("invalid SelectableText availability %v", availability)
		return ""
	}
}

func selectableTextFixtureRange(t *testing.T, value string) (int, int) {
	t.Helper()
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		t.Fatalf("invalid SelectableText range %q", value)
	}
	return fixtureInt(t, parts[0]), fixtureInt(t, parts[1])
}

func TestSelectableTextContentSpansReturnsAnIndependentSlice(t *testing.T) {
	content := NewSelectableTextContent([]tui.TextSpan{
		tui.NewTextSpan("a", vt.Style{Bold: true}),
		tui.NewTextSpan("b", vt.Style{Underline: true}),
	})
	clone := content
	if clone.inner != content.inner {
		t.Fatal("content copy did not share immutable storage")
	}
	spans := content.Spans()
	spans[0] = tui.NewTextSpan("changed", vt.Style{})
	if content.Text() != "ab" || content.Spans()[0].Text != "a" {
		t.Fatalf("content was mutated through Spans: %#v", content.Spans())
	}
	if !slices.Equal(content.Spans(), []tui.TextSpan{
		tui.NewTextSpan("a", vt.Style{Bold: true}),
		tui.NewTextSpan("b", vt.Style{Underline: true}),
	}) {
		t.Fatalf("spans = %#v", content.Spans())
	}
}

func TestSelectableTextContentNormalizesInvalidUTF8BeforeOffsets(t *testing.T) {
	content := NewSelectableTextContent([]tui.TextSpan{
		tui.NewTextSpan("a\xffb", vt.Style{}),
	})
	if content.Text() != "a\uFFFDb" {
		t.Fatalf("text = %q, want replacement text", content.Text())
	}
	state := content.NormalizeState(NewSelectableTextState(2))
	if state.Cursor() != 1 {
		t.Fatalf("cursor = %d, want preceding grapheme boundary 1", state.Cursor())
	}
}
