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

type suggestionFixtureMessage struct {
	kind  string
	state ComposerState
	id    SuggestionID
}

type suggestionFixtureApp struct {
	composer    ComposerState
	candidates  SuggestionItems
	selected    SuggestionID
	hasSelected bool
	status      SuggestionPopupStatus
	enabled     bool
	open        bool
	messages    []string
}

func (*suggestionFixtureApp) Init() tui.Effect[suggestionFixtureMessage] {
	return tui.NoneEffect[suggestionFixtureMessage]()
}

func (a *suggestionFixtureApp) Update(message suggestionFixtureMessage) tui.Effect[suggestionFixtureMessage] {
	switch message.kind {
	case "change":
		a.composer = message.state
		a.messages = append(a.messages, "change:"+message.state.TextArea().Value())
	case "select":
		a.selected, a.hasSelected = message.id, true
		a.messages = append(a.messages, "select:"+message.id.String())
	case "accept":
		a.messages = append(a.messages, "accept:"+message.id.String())
	case "dismiss":
		a.open = false
		a.messages = append(a.messages, "dismiss")
	case "submit":
		a.messages = append(a.messages, "submit")
	}
	return tui.NoneEffect[suggestionFixtureMessage]()
}

func (*suggestionFixtureApp) Subscriptions() tui.Subscription[suggestionFixtureMessage] {
	return tui.NoneSubscription[suggestionFixtureMessage]()
}

func (a *suggestionFixtureApp) View(tui.ViewContext) tui.Node[suggestionFixtureMessage] {
	composer := NewComposer(
		tui.NewNodeID("composer"),
		tui.NewNodeID("composer-viewport"),
		tui.NewNodeID("composer-caret"),
		a.composer,
		func(state ComposerState) suggestionFixtureMessage {
			return suggestionFixtureMessage{kind: "change", state: state}
		},
		func() suggestionFixtureMessage { return suggestionFixtureMessage{kind: "submit"} },
	).Rows(1, 1).Node()
	return NewSuggestionPopup(
		tui.NewNodeID("suggestions"),
		composer,
		tui.NewNodeID("composer-caret"),
		tui.NewNodeID("composer"),
		a.candidates,
		a.selected,
		a.hasSelected,
		func(context SuggestionRowContext) tui.Node[suggestionFixtureMessage] {
			style := vt.Style{}
			if context.Selected() {
				style.Reverse = true
			}
			return tui.StyledText[suggestionFixtureMessage](context.ID().String(), style)
		},
		func(id SuggestionID) suggestionFixtureMessage {
			return suggestionFixtureMessage{kind: "select", id: id}
		},
		func(id SuggestionID) suggestionFixtureMessage {
			return suggestionFixtureMessage{kind: "accept", id: id}
		},
		func() suggestionFixtureMessage { return suggestionFixtureMessage{kind: "dismiss"} },
	).Status(a.status).Enabled(a.enabled).Open(a.open).VisibleRows(3).Node()
}

func (a *suggestionFixtureApp) normalizedSelection() (SuggestionID, bool) {
	if a.candidates.Empty() {
		return SuggestionID{}, false
	}
	if a.hasSelected {
		for _, candidate := range a.candidates.items() {
			if candidate == a.selected {
				return candidate, true
			}
		}
	}
	first, _ := a.candidates.Item(0)
	return first, true
}

func TestSuggestionPopupMatchesSharedFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t,
		"widgets/suggestion-popup.txt",
		"widget-suggestion-popup",
		"candidates", "selected", "status", "enabled", "event", "expected-selected",
		"expected-value", "expected-open", "expected-messages", "consumed",
	)
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			selected, hasSelected := suggestionFixtureOptionalID(record.Field("selected"))
			candidates, err := NewSuggestionItems(suggestionFixtureIDs(record.Field("candidates")))
			if err != nil {
				t.Fatal(err)
			}
			app := &suggestionFixtureApp{
				composer:    NewComposerStateAtEnd(""),
				candidates:  candidates,
				selected:    selected,
				hasSelected: hasSelected,
				status:      suggestionFixtureStatus(t, record.Field("status")),
				enabled:     fixtureBool(t, record.Field("enabled")),
				open:        true,
			}
			runtime, err := tui.NewRuntimeWithClock(
				app,
				tui.NewRuntimeConfig(tui.Size{Width: 20, Height: 8}),
				tui.NewVirtualClock(),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer runtime.Close()
			if frame, err := runtime.RenderIfDirty(); err != nil || frame == nil {
				t.Fatalf("initial render = %v, %v", frame, err)
			}
			focused, err := runtime.RequestFocus(tui.NewNodeID("composer"))
			if err != nil || !focused {
				t.Fatalf("focus Composer = %t, %v", focused, err)
			}
			if _, err := runtime.RenderIfDirty(); err != nil {
				t.Fatal(err)
			}

			var consumed *bool
			if record.Field("event") != "-" {
				dispatch, err := runtime.DispatchEvent(suggestionFixtureEvent(t, record.Field("event")))
				if err != nil {
					t.Fatal(err)
				}
				value := dispatch.Consumed()
				consumed = &value
				if _, err := runtime.ProcessPending(); err != nil {
					t.Fatal(err)
				}
				if _, err := runtime.RenderIfDirty(); err != nil {
					t.Fatal(err)
				}
			}

			actualSelected, hasActualSelected := app.normalizedSelection()
			expectedSelected, hasExpectedSelected := suggestionFixtureOptionalID(record.Field("expected-selected"))
			if actualSelected != expectedSelected || hasActualSelected != hasExpectedSelected {
				t.Errorf("selected = %q, %t, want %q, %t", actualSelected, hasActualSelected, expectedSelected, hasExpectedSelected)
			}
			if actual := app.composer.TextArea().Value(); actual != record.Field("expected-value") {
				t.Errorf("value = %q, want %q", actual, record.Field("expected-value"))
			}
			if expected := fixtureBool(t, record.Field("expected-open")); app.open != expected {
				t.Errorf("open = %t, want %t", app.open, expected)
			}
			if expected := suggestionFixtureStrings(record.Field("expected-messages")); !slices.Equal(app.messages, expected) {
				t.Errorf("messages = %#v, want %#v", app.messages, expected)
			}
			focus, hasFocus := runtime.Interaction().Focused()
			if !hasFocus || focus != tui.NewNodeID("composer") {
				t.Errorf("focus = %q, %t, want Composer", focus, hasFocus)
			}
			if expected := record.Field("consumed"); expected != "-" {
				if consumed == nil || *consumed != fixtureBool(t, expected) {
					t.Errorf("consumed = %v, want %s", consumed, expected)
				}
			}
		})
	}
}

func suggestionFixtureIDs(value string) []SuggestionID {
	values := suggestionFixtureStrings(value)
	result := make([]SuggestionID, len(values))
	for index, value := range values {
		result[index] = NewSuggestionID(value)
	}
	return result
}

func suggestionFixtureOptionalID(value string) (SuggestionID, bool) {
	if value == "-" {
		return SuggestionID{}, false
	}
	return NewSuggestionID(value), true
}

func suggestionFixtureStrings(value string) []string {
	if value == "-" {
		return nil
	}
	return strings.Split(value, ",")
}

func suggestionFixtureStatus(t *testing.T, value string) SuggestionPopupStatus {
	t.Helper()
	switch value {
	case "ready":
		return SuggestionPopupReady
	case "loading":
		return SuggestionPopupLoading
	default:
		t.Fatalf("invalid SuggestionPopup status %q", value)
		return 0
	}
}

func suggestionFixtureEvent(t *testing.T, value string) vt.Event {
	t.Helper()
	key := func(code vt.KeyCode, action vt.KeyAction) vt.Event {
		return keyEvent(code, 0, vt.Modifiers{}, action)
	}
	switch value {
	case "down":
		return key(vt.KeyDown, vt.KeyPress)
	case "repeat-down":
		return key(vt.KeyDown, vt.KeyRepeat)
	case "up":
		return key(vt.KeyUp, vt.KeyPress)
	case "enter":
		return key(vt.KeyEnter, vt.KeyPress)
	case "repeat-enter":
		return key(vt.KeyEnter, vt.KeyRepeat)
	case "escape":
		return key(vt.KeyEscape, vt.KeyPress)
	case "repeat-escape":
		return key(vt.KeyEscape, vt.KeyRepeat)
	case "text-X":
		return vt.Event{Kind: vt.EventText, Text: "X"}
	case "mouse-second":
		return vt.Event{Kind: vt.EventMouse, Mouse: vt.MouseEvent{
			Kind: vt.MousePress, Button: vt.MouseLeft, X: 1, Y: 3,
		}}
	default:
		t.Fatalf("invalid SuggestionPopup event %q", value)
		return vt.Event{}
	}
}

func TestSuggestionIDNormalizesInvalidUTF8(t *testing.T) {
	id := NewSuggestionID(string([]byte{0xff, 'A'}))
	if id.String() != "\uFFFDA" {
		t.Fatalf("SuggestionID = %q, want replacement plus A", id.String())
	}
}

func TestSuggestionPopupBoundsRowConstructionAndSkipsNotices(t *testing.T) {
	candidates := make([]SuggestionID, 100)
	for index := range candidates {
		candidates[index] = NewSuggestionID("candidate-" + strconv.Itoa(index))
	}
	builds := 0
	newPopup := func(candidates []SuggestionID, status SuggestionPopupStatus) SuggestionPopup[struct{}] {
		selected := SuggestionID{}
		if len(candidates) != 0 {
			selected = candidates[min(50, len(candidates)-1)]
		}
		items, err := NewSuggestionItems(candidates)
		if err != nil {
			t.Fatal(err)
		}
		return NewSuggestionPopup(
			tui.NewNodeID("popup"),
			tui.Text[struct{}]("base").WithID(tui.NewNodeID("anchor")),
			tui.NewNodeID("anchor"),
			tui.NewNodeID("anchor"),
			items,
			selected,
			len(candidates) != 0,
			func(SuggestionRowContext) tui.Node[struct{}] {
				builds++
				return tui.Text[struct{}]("row")
			},
			func(SuggestionID) struct{} { return struct{}{} },
			func(SuggestionID) struct{} { return struct{}{} },
			func() struct{} { return struct{}{} },
		).Status(status).VisibleRows(3)
	}
	newPopup(candidates, SuggestionPopupReady).Node()
	if builds != 3 {
		t.Fatalf("row builds = %d, want 3", builds)
	}
	newPopup([]SuggestionID{NewSuggestionID("candidate")}, SuggestionPopupLoading).Node()
	newPopup(nil, SuggestionPopupReady).Node()
	if builds != 3 {
		t.Fatalf("notice row builds = %d, want unchanged 3", builds)
	}
}

func TestSuggestionPopupDescriptorsHaveStableOrderAndRepeatPolicy(t *testing.T) {
	items, err := NewSuggestionItems([]SuggestionID{NewSuggestionID("candidate")})
	if err != nil {
		t.Fatal(err)
	}
	popup := NewSuggestionPopup(
		tui.NewNodeID("popup"),
		tui.Text[struct{}]("base").WithID(tui.NewNodeID("anchor")),
		tui.NewNodeID("anchor"),
		tui.NewNodeID("anchor"),
		items,
		SuggestionID{},
		false,
		func(SuggestionRowContext) tui.Node[struct{}] { return tui.Text[struct{}]("row") },
		func(SuggestionID) struct{} { return struct{}{} },
		func(SuggestionID) struct{} { return struct{}{} },
		func() struct{} { return struct{}{} },
	)
	descriptors := popup.ActionDescriptors()
	expectedIDs := []tui.ActionID{
		SuggestionAcceptActionID,
		SelectionPreviousActionID,
		SelectionNextActionID,
		SuggestionDismissActionID,
	}
	expectedRepeat := []tui.RepeatPolicy{
		tui.RepeatInitialOnly,
		tui.RepeatAllow,
		tui.RepeatAllow,
		tui.RepeatInitialOnly,
	}
	for index, descriptor := range descriptors {
		if descriptor.ID() != expectedIDs[index] {
			t.Errorf("descriptor %d ID = %q, want %q", index, descriptor.ID(), expectedIDs[index])
		}
		if descriptor.Availability() != tui.ActionEnabled {
			t.Errorf("descriptor %d availability = %d", index, descriptor.Availability())
		}
		if actual := descriptor.DefaultBindings()[0].RepeatPolicy(); actual != expectedRepeat[index] {
			t.Errorf("descriptor %d repeat = %d, want %d", index, actual, expectedRepeat[index])
		}
	}
}

func TestSuggestionItemsRejectDuplicatesAndShareStorage(t *testing.T) {
	input := []SuggestionID{NewSuggestionID("a"), NewSuggestionID("b")}
	items, err := NewSuggestionItems(input)
	if err != nil {
		t.Fatal(err)
	}
	clone := items
	if items.inner != clone.inner {
		t.Fatal("SuggestionItems copy does not share storage")
	}
	input[1] = NewSuggestionID("changed")
	second, ok := items.Item(1)
	if !ok || second.String() != "b" {
		t.Fatalf("immutable item = %q, %t, want b", second.String(), ok)
	}
	copyItems := items.Items()
	copyItems[0] = NewSuggestionID("changed")
	first, _ := items.Item(0)
	if first.String() != "a" {
		t.Fatalf("accessor mutation changed item to %q", first.String())
	}
	_, err = NewSuggestionItems([]SuggestionID{NewSuggestionID("same"), NewSuggestionID("same")})
	var duplicate *DuplicateSuggestionIDError
	if !errors.As(err, &duplicate) || duplicate.ID.String() != "same" {
		t.Fatalf("duplicate error = %#v, %v", duplicate, err)
	}
}

func TestSuggestionRowIDsDistinguishPopupAndCandidateBoundaries(t *testing.T) {
	first := suggestionRowNodeID(
		tui.NewNodeID("x"),
		NewSuggestionID("y:suggestion-row:1:z"),
	)
	second := suggestionRowNodeID(
		tui.NewNodeID("x:suggestion-row:20:y"),
		NewSuggestionID("z"),
	)
	if first == second {
		t.Fatalf("derived row IDs collide at %q", first)
	}
}
