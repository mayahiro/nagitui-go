package widget

import (
	"slices"
	"strings"
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/internal/conformance"
)

type composerFixtureMessage struct {
	kind  string
	state ComposerState
}

type composerFixtureLimit struct {
	unit    string
	maximum int
}

type composerFixtureApp struct {
	state         ComposerState
	history       []string
	wrap          int
	hasWrap       bool
	minRows       int
	maxRows       int
	limit         composerFixtureLimit
	overflow      ComposerOverflowPolicy
	enabled       bool
	submitEnabled bool
	keyMap        tui.KeyMap
	validation    bool
	messages      []composerFixtureMessage
}

func (*composerFixtureApp) Init() tui.Effect[composerFixtureMessage] {
	return tui.NoneEffect[composerFixtureMessage]()
}

func (a *composerFixtureApp) Update(message composerFixtureMessage) tui.Effect[composerFixtureMessage] {
	if message.kind == "change" {
		a.state = message.state
	}
	a.messages = append(a.messages, message)
	return tui.NoneEffect[composerFixtureMessage]()
}

func (*composerFixtureApp) Subscriptions() tui.Subscription[composerFixtureMessage] {
	return tui.NoneSubscription[composerFixtureMessage]()
}

func (a *composerFixtureApp) View(tui.ViewContext) tui.Node[composerFixtureMessage] {
	return tui.Padding(a.composer().Node(), tui.UniformInsets(0)).
		WithKeyScope(tui.NewKeyScope(tui.NewNodeID("scope"), a.keyMap))
}

func (a *composerFixtureApp) composer() Composer[composerFixtureMessage] {
	composer := NewComposer(
		tui.NewNodeID("composer"),
		tui.NewNodeID("composer-viewport"),
		tui.NewNodeID("composer-caret"),
		a.state,
		func(state ComposerState) composerFixtureMessage {
			return composerFixtureMessage{kind: "change", state: state}
		},
		func() composerFixtureMessage { return composerFixtureMessage{kind: "submit"} },
	).Enabled(a.enabled).
		SubmitEnabled(a.submitEnabled).
		Rows(a.minRows, a.maxRows).
		History(a.history)
	if a.hasWrap {
		composer = composer.SoftWrap(a.wrap)
	}
	switch a.limit.unit {
	case "":
	case "bytes":
		composer = composer.MaximumUTF8Bytes(a.limit.maximum, a.overflow)
	case "graphemes":
		composer = composer.MaximumGraphemes(a.limit.maximum, a.overflow)
	default:
		panic("invalid Composer fixture limit")
	}
	if a.validation {
		composer = composer.Validation(tui.Text[composerFixtureMessage]("!"))
	}
	return composer
}

func TestComposerMatchesSharedFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t,
		"widgets/composer.txt",
		"widget-composer",
		"initial", "cursor", "selection", "history", "wrap", "min-rows", "max-rows",
		"limit", "overflow", "enabled", "submit-enabled", "scope", "validation", "events",
		"expected", "expected-cursor", "expected-selection", "expected-history", "expected-draft",
		"expected-draft-cursor", "expected-messages", "expected-rows", "expected-offset",
		"expected-maximum", "expected-validation-row", "expected-submit", "expected-previous",
		"expected-next", "expected-up", "expected-down", "expected-consumed",
	)
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			app := &composerFixtureApp{
				state: composerFixtureState(
					t, record.Text("initial"), record.Field("cursor"), record.Field("selection"),
				),
				history:       composerFixtureHistory(record),
				minRows:       fixtureInt(t, record.Field("min-rows")),
				maxRows:       fixtureInt(t, record.Field("max-rows")),
				limit:         composerFixtureLengthLimit(t, record.Field("limit")),
				overflow:      composerFixtureOverflow(t, record.Field("overflow")),
				enabled:       fixtureBool(t, record.Field("enabled")),
				submitEnabled: fixtureBool(t, record.Field("submit-enabled")),
				keyMap:        composerFixtureKeyMap(t, record.Field("scope")),
				validation:    record.Field("validation") == "error",
			}
			if record.Field("wrap") != "-" {
				app.wrap = fixtureInt(t, record.Field("wrap"))
				app.hasWrap = true
			}
			runtime, err := tui.NewRuntimeWithClock(
				app,
				tui.NewRuntimeConfig(tui.Size{Width: 16, Height: 10}),
				tui.NewVirtualClock(),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer runtime.Close()
			frame, err := runtime.RenderIfDirty()
			if err != nil || frame == nil {
				t.Fatalf("initial RenderIfDirty = %v, %v", frame, err)
			}
			focused, err := runtime.RequestFocus(tui.NewNodeID("composer"))
			if err != nil {
				t.Fatal(err)
			}
			if focused != app.enabled {
				t.Fatalf("RequestFocus = %t, want %t", focused, app.enabled)
			}
			if rendered, err := runtime.RenderIfDirty(); err != nil {
				t.Fatal(err)
			} else if rendered != nil {
				frame = rendered
			}

			var lastConsumed *bool
			if record.Field("events") != "-" {
				for _, value := range strings.Split(record.Field("events"), ",") {
					dispatch, err := runtime.DispatchEvent(composerFixtureEvent(t, value))
					if err != nil {
						t.Fatal(err)
					}
					consumed := dispatch.Consumed()
					lastConsumed = &consumed
					if _, err := runtime.ProcessPending(); err != nil {
						t.Fatal(err)
					}
					if rendered, err := runtime.RenderIfDirty(); err != nil {
						t.Fatal(err)
					} else if rendered != nil {
						frame = rendered
					}
				}
			}

			assertComposerFixtureState(t, app.state, record)
			actualMessages := make([]string, len(app.messages))
			for index, message := range app.messages {
				actualMessages[index] = message.kind
			}
			expectedMessages := composerFixtureNames(record.Field("expected-messages"))
			if !slices.Equal(actualMessages, expectedMessages) {
				t.Errorf("messages = %#v, want %#v", actualMessages, expectedMessages)
			}
			if actual, expected := app.composer().VisibleRows(), uint32(fixtureInt(t, record.Field("expected-rows"))); actual != expected {
				t.Errorf("visible rows = %d, want %d", actual, expected)
			}
			scroll, ok := runtime.Interaction().ScrollState(tui.NewNodeID("composer-viewport"))
			if !ok {
				t.Fatal("Composer viewport has no scroll state")
			}
			if actual, expected := scroll.Offset.Y, uint32(fixtureInt(t, record.Field("expected-offset"))); actual != expected {
				t.Errorf("scroll offset = %d, want %d", actual, expected)
			}
			if actual, expected := scroll.Maximum.Y, uint32(fixtureInt(t, record.Field("expected-maximum"))); actual != expected {
				t.Errorf("scroll maximum = %d, want %d", actual, expected)
			}
			if value := record.Field("expected-validation-row"); value != "-" {
				row := int32(fixtureInt(t, value))
				cell, ok := frame.Surface().Cell(0, row)
				if !ok || cell.Content() != "!" {
					t.Errorf("validation cell at row %d = %#v, %t", row, cell, ok)
				}
			}
			descriptors := app.composer().ActionDescriptors()
			assertComposerAvailability(t, descriptors, ComposerSubmitActionID, record.Field("expected-submit"))
			assertComposerAvailability(t, descriptors, HistoryPreviousActionID, record.Field("expected-previous"))
			assertComposerAvailability(t, descriptors, HistoryNextActionID, record.Field("expected-next"))
			assertComposerAvailability(t, descriptors, tui.TextCursorUpActionID, record.Field("expected-up"))
			assertComposerAvailability(t, descriptors, tui.TextCursorDownActionID, record.Field("expected-down"))
			if expected := record.Field("expected-consumed"); expected != "-" {
				if lastConsumed == nil || *lastConsumed != fixtureBool(t, expected) {
					t.Errorf("last consumed = %v, want %s", lastConsumed, expected)
				}
			}
		})
	}
}

func TestComposerDescriptorDefaultsAreOrderedAndHaveLegacyNewlineFallback(t *testing.T) {
	descriptors := NewComposer(
		tui.NewNodeID("composer"),
		tui.NewNodeID("viewport"),
		tui.NewNodeID("caret"),
		NewComposerState(TextAreaState{}),
		func(state ComposerState) composerFixtureMessage {
			return composerFixtureMessage{kind: "change", state: state}
		},
		func() composerFixtureMessage { return composerFixtureMessage{kind: "submit"} },
	).ActionDescriptors()
	wantLeading := []tui.ActionID{ComposerSubmitActionID, HistoryPreviousActionID, HistoryNextActionID}
	for index, expected := range wantLeading {
		if actual := descriptors[index].ID(); actual != expected {
			t.Errorf("descriptor %d = %q, want %q", index, actual, expected)
		}
	}
	if actual := descriptors[0].DefaultBindings()[0].RepeatPolicy(); actual != tui.RepeatInitialOnly {
		t.Errorf("submit repeat policy = %d, want initial-only", actual)
	}
	for _, descriptor := range descriptors[1:3] {
		if actual := descriptor.DefaultBindings()[0].RepeatPolicy(); actual != tui.RepeatAllow {
			t.Errorf("action %s repeat policy = %d, want allow", descriptor.ID(), actual)
		}
	}
	var lineBreak tui.ActionDescriptor
	found := false
	for _, descriptor := range descriptors {
		if descriptor.ID() == tui.TextInsertLineBreakActionID {
			lineBreak = descriptor
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Composer has no line break action")
	}
	bindings := lineBreak.DefaultBindings()
	notations := make([]string, len(bindings))
	for index, binding := range bindings {
		notations[index] = binding.Stroke().Notation()
	}
	if expected := []string{"Shift+Enter", "Alt+Enter", "Ctrl+o"}; !slices.Equal(notations, expected) {
		t.Errorf("line break bindings = %#v, want %#v", notations, expected)
	}
	for _, binding := range bindings {
		if actual := binding.RepeatPolicy(); actual != tui.RepeatAllow {
			t.Errorf("line break repeat policy = %d, want allow", actual)
		}
	}
	if actual := bindings[2].Support(); actual != tui.BindingSupported {
		t.Errorf("Control-O support = %d, want supported", actual)
	}
}

func TestComposerUnknownOverflowPolicyRejectsInsertion(t *testing.T) {
	inserted, accepted := composerInsertion(
		composerLengthLimit{
			maximum:  1,
			unit:     composerLengthUTF8Bytes,
			overflow: ComposerOverflowPolicy(255),
		},
		NewTextAreaStateAtEnd("a"),
		"b",
	)
	if accepted || inserted != "" {
		t.Fatalf("unknown overflow = %q, %t, want rejection", inserted, accepted)
	}
}

func composerFixtureState(t *testing.T, value, cursor, selection string) ComposerState {
	t.Helper()
	cursorOffset := fixtureInt(t, cursor)
	textArea := NewTextAreaState(value, cursorOffset)
	if selection != "-" {
		start, end := composerFixtureRange(t, selection)
		anchor := start
		if cursorOffset == start {
			anchor = end
		}
		textArea = textArea.Select(anchor)
	}
	return NewComposerState(textArea)
}

func assertComposerFixtureState(t *testing.T, state ComposerState, record conformance.Record) {
	t.Helper()
	textArea := state.TextArea()
	if actual, expected := textArea.Value(), record.Text("expected"); actual != expected {
		t.Errorf("value = %q, want %q", actual, expected)
	}
	if actual, expected := textArea.Cursor(), fixtureInt(t, record.Field("expected-cursor")); actual != expected {
		t.Errorf("cursor = %d, want %d", actual, expected)
	}
	actualStart, actualEnd, actualSelection := textArea.Selection()
	if expected := record.Field("expected-selection"); expected == "-" {
		if actualSelection {
			t.Errorf("selection = %d:%d, want none", actualStart, actualEnd)
		}
	} else {
		expectedStart, expectedEnd := composerFixtureRange(t, expected)
		if !actualSelection || actualStart != expectedStart || actualEnd != expectedEnd {
			t.Errorf("selection = %d:%d, %t, want %d:%d", actualStart, actualEnd, actualSelection, expectedStart, expectedEnd)
		}
	}
	actualHistory, hasHistory := state.HistoryIndex()
	if expected := record.Field("expected-history"); expected == "-" {
		if hasHistory {
			t.Errorf("history index = %d, want none", actualHistory)
		}
	} else if expectedIndex := fixtureInt(t, expected); !hasHistory || actualHistory != expectedIndex {
		t.Errorf("history index = %d, %t, want %d", actualHistory, hasHistory, expectedIndex)
	}
	draft, hasDraft := state.Draft()
	if expected := record.Field("expected-draft"); expected == "-" {
		if hasDraft {
			t.Errorf("draft = %#v, want none", draft)
		}
	} else {
		if !hasDraft {
			t.Fatal("Composer state has no draft")
		}
		if actual, expectedValue := draft.Value(), record.Text("expected-draft"); actual != expectedValue {
			t.Errorf("draft value = %q, want %q", actual, expectedValue)
		}
		if actual, expectedCursor := draft.Cursor(), fixtureInt(t, record.Field("expected-draft-cursor")); actual != expectedCursor {
			t.Errorf("draft cursor = %d, want %d", actual, expectedCursor)
		}
	}
}

func composerFixtureHistory(record conformance.Record) []string {
	if record.Field("history") == "-" {
		return nil
	}
	return strings.Split(record.Text("history"), "/")
}

func composerFixtureLengthLimit(t *testing.T, value string) composerFixtureLimit {
	t.Helper()
	if value == "-" {
		return composerFixtureLimit{}
	}
	unit, maximum, ok := strings.Cut(value, ":")
	if !ok || unit != "bytes" && unit != "graphemes" {
		t.Fatalf("invalid Composer limit %q", value)
	}
	return composerFixtureLimit{unit: unit, maximum: fixtureInt(t, maximum)}
}

func composerFixtureOverflow(t *testing.T, value string) ComposerOverflowPolicy {
	t.Helper()
	switch value {
	case "reject":
		return ComposerOverflowReject
	case "truncate":
		return ComposerOverflowTruncate
	default:
		t.Fatalf("invalid Composer overflow %q", value)
		return ComposerOverflowReject
	}
}

func composerFixtureKeyMap(t *testing.T, value string) tui.KeyMap {
	t.Helper()
	keyMap := tui.NewKeyMap()
	if value == "-" {
		return keyMap
	}
	if value != "swap" {
		t.Fatalf("invalid Composer scope %q", value)
	}
	var err error
	keyMap, err = keyMap.Rebind(
		ComposerSubmitActionID,
		[]tui.KeyBinding{tui.NewKeyBinding(tui.NewKeyStroke(vt.KeyEnter, vt.Modifiers{Control: true}))},
	)
	if err != nil {
		t.Fatal(err)
	}
	keyMap, err = keyMap.Rebind(
		tui.TextInsertLineBreakActionID,
		[]tui.KeyBinding{tui.NewKeyBinding(tui.NewKeyStroke(vt.KeyEnter, vt.Modifiers{}))},
	)
	if err != nil {
		t.Fatal(err)
	}
	return keyMap
}

func composerFixtureEvent(t *testing.T, value string) vt.Event {
	t.Helper()
	switch value {
	case "enter":
		return keyEvent(vt.KeyEnter, 0, vt.Modifiers{}, vt.KeyPress)
	case "repeat-enter":
		return keyEvent(vt.KeyEnter, 0, vt.Modifiers{}, vt.KeyRepeat)
	case "shift-enter":
		return keyEvent(vt.KeyEnter, 0, vt.Modifiers{Shift: true}, vt.KeyPress)
	case "alt-enter":
		return keyEvent(vt.KeyEnter, 0, vt.Modifiers{Alt: true}, vt.KeyPress)
	case "control-enter":
		return keyEvent(vt.KeyEnter, 0, vt.Modifiers{Control: true}, vt.KeyPress)
	case "control-o":
		return keyEvent(vt.KeyCharacter, 'o', vt.Modifiers{Control: true}, vt.KeyPress)
	case "up":
		return keyEvent(vt.KeyUp, 0, vt.Modifiers{}, vt.KeyPress)
	case "down":
		return keyEvent(vt.KeyDown, 0, vt.Modifiers{}, vt.KeyPress)
	case "backspace":
		return keyEvent(vt.KeyBackspace, 0, vt.Modifiers{}, vt.KeyPress)
	case "text-x":
		return vt.Event{Kind: vt.EventText, Text: "x"}
	case "paste-xy":
		return vt.Event{Kind: vt.EventPaste, Text: "xy"}
	case "paste-lines":
		return vt.Event{Kind: vt.EventPaste, Text: "x\ny"}
	case "paste-combining":
		return vt.Event{Kind: vt.EventPaste, Text: "e\u0301x"}
	default:
		t.Fatalf("invalid Composer event %q", value)
		return vt.Event{}
	}
}

func assertComposerAvailability(
	t *testing.T,
	descriptors []tui.ActionDescriptor,
	id tui.ActionID,
	expected string,
) {
	t.Helper()
	availability := tui.ActionEnabled
	switch expected {
	case "enabled":
	case "pass":
		availability = tui.ActionDisabledPassThrough
	case "consume":
		availability = tui.ActionDisabledConsume
	default:
		t.Fatalf("invalid Composer availability %q", expected)
	}
	for _, descriptor := range descriptors {
		if descriptor.ID() == id {
			if descriptor.Availability() != availability {
				t.Errorf("action %s availability = %d, want %d", id, descriptor.Availability(), availability)
			}
			return
		}
	}
	t.Errorf("Composer has no action %s", id)
}

func composerFixtureRange(t *testing.T, value string) (int, int) {
	t.Helper()
	start, end, ok := strings.Cut(value, ":")
	if !ok {
		t.Fatalf("invalid Composer range %q", value)
	}
	return fixtureInt(t, start), fixtureInt(t, end)
}

func composerFixtureNames(value string) []string {
	if value == "-" {
		return nil
	}
	return strings.Split(value, ",")
}
