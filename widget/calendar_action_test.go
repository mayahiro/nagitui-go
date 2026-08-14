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

type calendarActionApp struct {
	year         int
	month        int
	selected     CalendarDate
	showAdjacent bool
	enabled      bool
	keyMap       tui.KeyMap
	messages     []CalendarDate
}

func (*calendarActionApp) Init() tui.Effect[CalendarDate] {
	return tui.NoneEffect[CalendarDate]()
}

func (a *calendarActionApp) Update(message CalendarDate) tui.Effect[CalendarDate] {
	a.selected = message
	a.messages = append(a.messages, message)
	return tui.NoneEffect[CalendarDate]()
}

func (*calendarActionApp) Subscriptions() tui.Subscription[CalendarDate] {
	return tui.NoneSubscription[CalendarDate]()
}

func (a *calendarActionApp) View(tui.ViewContext) tui.Node[CalendarDate] {
	calendar := calendarActionFixture(
		a.year,
		a.month,
		a.selected,
		a.showAdjacent,
		a.enabled,
	).Node()
	return tui.Padding(calendar, tui.UniformInsets(0)).
		WithKeyScope(tui.NewKeyScope(tui.NewNodeID("scope"), a.keyMap))
}

func TestCalendarActionsMatchSharedFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t,
		"widgets/calendar-action.txt",
		"widget-calendar-action",
		"displayed", "selected", "show-adjacent", "focus", "mode", "enabled",
		"event", "messages", "consumed", "focus-after", "selected-after", "keys", "available",
	)
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			year, month := calendarActionFixtureDisplayed(t, record.Field("displayed"))
			selected := calendarActionFixtureDate(t, record.Field("selected"))
			showAdjacent := fixtureBool(t, record.Field("show-adjacent"))
			enabled := fixtureBool(t, record.Field("enabled"))
			keyMap := calendarActionKeyMap(t, record.Field("mode"))
			calendar := calendarActionFixture(year, month, selected, showAdjacent, enabled)
			resolved, err := tui.ResolveActions(
				tui.NewNodeID("calendar"),
				calendar.ActionDescriptors(),
				[]tui.KeyScope{tui.NewKeyScope(tui.NewNodeID("scope"), keyMap)},
			)
			if err != nil {
				t.Fatal(err)
			}
			assertCalendarActionGroup(
				t,
				resolved,
				calendarActionFixtureKeyGroups(record.Field("keys")),
				calendarActionFixtureBools(t, record.Field("available")),
			)

			app := &calendarActionApp{
				year: year, month: month, selected: selected, showAdjacent: showAdjacent,
				enabled: enabled, keyMap: keyMap,
			}
			runtime, err := tui.NewRuntimeWithClock(
				app,
				tui.NewRuntimeConfig(tui.Size{Width: 24, Height: 8}),
				tui.NewVirtualClock(),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer runtime.Close()
			if _, err := runtime.RenderIfDirty(); err != nil {
				t.Fatal(err)
			}
			if record.Field("focus") == "calendar" {
				focused, err := runtime.RequestFocus(tui.NewNodeID("calendar"))
				if err != nil || !focused {
					t.Fatalf("focus = %t, %v", focused, err)
				}
				groups, err := runtime.ActiveActionGroups()
				if err != nil {
					t.Fatal(err)
				}
				groups = nodeDeclaredActionGroups(groups)
				if len(groups) != 1 || groups[0].Owner() != tui.NewNodeID("calendar") {
					t.Fatalf("action groups = %#v", groups)
				}
			}

			dispatch, err := runtime.DispatchEvent(calendarActionEvent(t, record.Field("event")))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runtime.ProcessPending(); err != nil {
				t.Fatal(err)
			}
			expectedMessages := calendarActionFixtureDates(t, record.Field("messages"))
			if !slices.Equal(app.messages, expectedMessages) {
				t.Errorf("messages = %#v, want %#v", app.messages, expectedMessages)
			}
			if dispatch.Messages() != len(expectedMessages) {
				t.Errorf("dispatched messages = %d, want %d", dispatch.Messages(), len(expectedMessages))
			}
			if dispatch.Consumed() != fixtureBool(t, record.Field("consumed")) {
				t.Errorf("consumed = %t", dispatch.Consumed())
			}
			expectedFocus, expectsFocus := calendarActionFixtureFocus(record.Field("focus-after"))
			actualFocus, hasFocus := runtime.Interaction().Focused()
			if hasFocus != expectsFocus || expectsFocus && actualFocus != expectedFocus {
				t.Errorf("focus = %q, %t, want %q, %t", actualFocus, hasFocus, expectedFocus, expectsFocus)
			}
			expectedSelected := calendarActionFixtureDate(t, record.Field("selected-after"))
			if app.selected != expectedSelected {
				t.Errorf("selected = %#v, want %#v", app.selected, expectedSelected)
			}
		})
	}
}

func TestCalendarRootConflictIsReportedBeforeDispatch(t *testing.T) {
	binding := []tui.KeyBinding{
		tui.NewKeyBinding(tui.NewCharacterKeyStroke('x', vt.Modifiers{})),
	}
	keyMap, err := tui.NewKeyMap().Rebind(SelectionPreviousDayActionID, binding)
	if err != nil {
		t.Fatal(err)
	}
	keyMap, err = keyMap.Rebind(SelectionPreviousWeekActionID, binding)
	if err != nil {
		t.Fatal(err)
	}
	app := &calendarActionApp{
		year: 2024, month: 2, selected: NewCalendarDate(2024, 2, 15),
		enabled: true, keyMap: keyMap,
	}
	runtime, err := tui.NewRuntimeWithClock(
		app,
		tui.NewRuntimeConfig(tui.Size{Width: 24, Height: 8}),
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
	if conflict.Kind() != tui.ConflictAmbiguousBinding || conflict.Owner() != tui.NewNodeID("calendar") {
		t.Fatalf("conflict = kind %d owner %s", conflict.Kind(), conflict.Owner())
	}
	expected := []tui.ActionID{SelectionPreviousDayActionID, SelectionPreviousWeekActionID}
	if !slices.Equal(conflict.Actions(), expected) {
		t.Fatalf("conflict actions = %#v, want %#v", conflict.Actions(), expected)
	}
	if len(app.messages) != 0 {
		t.Fatalf("conflicting action emitted messages = %#v", app.messages)
	}
}

func TestCalendarNilOnSelectDisablesActions(t *testing.T) {
	calendar := NewCalendar[CalendarDate](
		tui.NewNodeID("calendar"),
		2024,
		2,
		NewCalendarDate(2024, 2, 15),
		nil,
	)
	for _, descriptor := range calendar.ActionDescriptors() {
		if descriptor.Availability() != tui.ActionDisabledPassThrough {
			t.Fatalf("availability = %d", descriptor.Availability())
		}
	}
}

func calendarActionFixture(
	year int,
	month int,
	selected CalendarDate,
	showAdjacent bool,
	enabled bool,
) Calendar[CalendarDate] {
	return NewCalendar(
		tui.NewNodeID("calendar"),
		year,
		month,
		selected,
		func(date CalendarDate) CalendarDate { return date },
	).ShowAdjacent(showAdjacent).Enabled(enabled)
}

func calendarActionKeyMap(t *testing.T, mode string) tui.KeyMap {
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
	case "previous-day-h":
		resolved, err = keyMap.Rebind(SelectionPreviousDayActionID, character('h'))
	case "next-week-j":
		resolved, err = keyMap.Rebind(SelectionNextWeekActionID, character('j'))
	case "previous-month-u":
		resolved, err = keyMap.Rebind(SelectionPreviousMonthActionID, character('u'))
	case "activate-o":
		resolved, err = keyMap.Rebind(ActivateActionID, character('o'))
	case "unbind-next-day":
		resolved, err = keyMap.Rebind(SelectionNextDayActionID, nil)
	case "unbind-activate":
		resolved, err = keyMap.Rebind(ActivateActionID, nil)
	default:
		t.Fatalf("unknown Calendar action mode %q", mode)
	}
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func calendarActionEvent(t *testing.T, value string) vt.Event {
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
	case "right":
		return keyboard(vt.KeyRight, 0, vt.Modifiers{}, vt.KeyPress)
	case "up":
		return keyboard(vt.KeyUp, 0, vt.Modifiers{}, vt.KeyPress)
	case "down":
		return keyboard(vt.KeyDown, 0, vt.Modifiers{}, vt.KeyPress)
	case "page-up":
		return keyboard(vt.KeyPageUp, 0, vt.Modifiers{}, vt.KeyPress)
	case "page-down":
		return keyboard(vt.KeyPageDown, 0, vt.Modifiers{}, vt.KeyPress)
	case "home":
		return keyboard(vt.KeyHome, 0, vt.Modifiers{}, vt.KeyPress)
	case "end":
		return keyboard(vt.KeyEnd, 0, vt.Modifiers{}, vt.KeyPress)
	case "repeat-right":
		return keyboard(vt.KeyRight, 0, vt.Modifiers{}, vt.KeyRepeat)
	case "unknown-right":
		return keyboard(vt.KeyRight, 0, vt.Modifiers{}, vt.KeyActionUnknown)
	case "repeat-enter":
		return keyboard(vt.KeyEnter, 0, vt.Modifiers{}, vt.KeyRepeat)
	case "release-right":
		return keyboard(vt.KeyRight, 0, vt.Modifiers{}, vt.KeyRelease)
	case "shift-right":
		return keyboard(vt.KeyRight, 0, vt.Modifiers{Shift: true}, vt.KeyPress)
	case "control-enter":
		return keyboard(vt.KeyEnter, 0, vt.Modifiers{Control: true}, vt.KeyPress)
	case "h":
		return keyboard(vt.KeyCharacter, 'h', vt.Modifiers{}, vt.KeyPress)
	case "j":
		return keyboard(vt.KeyCharacter, 'j', vt.Modifiers{}, vt.KeyPress)
	case "u":
		return keyboard(vt.KeyCharacter, 'u', vt.Modifiers{}, vt.KeyPress)
	case "o":
		return keyboard(vt.KeyCharacter, 'o', vt.Modifiers{}, vt.KeyPress)
	default:
		if strings.HasPrefix(value, "mouse-") {
			return calendarActionPointerEvent(t, value)
		}
		t.Fatalf("unknown Calendar action event %q", value)
		return vt.Event{}
	}
}

func calendarActionPointerEvent(t *testing.T, value string) vt.Event {
	t.Helper()
	types := []struct {
		prefix string
		kind   vt.MouseKind
		button vt.MouseButton
	}{
		{prefix: "mouse-left-press:", kind: vt.MousePress, button: vt.MouseLeft},
		{prefix: "mouse-right-press:", kind: vt.MousePress, button: vt.MouseRight},
		{prefix: "mouse-left-release:", kind: vt.MouseRelease, button: vt.MouseLeft},
	}
	for _, eventType := range types {
		if coordinates, ok := strings.CutPrefix(value, eventType.prefix); ok {
			x, y, ok := strings.Cut(coordinates, ",")
			if !ok {
				t.Fatalf("invalid Calendar pointer coordinates %q", coordinates)
			}
			return vt.Event{
				Kind: vt.EventMouse,
				Mouse: vt.MouseEvent{
					Kind: eventType.kind, Button: eventType.button,
					X: calendarActionFixtureUint32(t, x), Y: calendarActionFixtureUint32(t, y),
				},
			}
		}
	}
	t.Fatalf("invalid Calendar pointer event %q", value)
	return vt.Event{}
}

func assertCalendarActionGroup(
	t *testing.T,
	resolved tui.ResolvedActions,
	expectedKeys [][]string,
	expectedAvailable []bool,
) {
	t.Helper()
	expectedIDs := []tui.ActionID{
		ActivateActionID,
		SelectionPreviousDayActionID,
		SelectionNextDayActionID,
		SelectionPreviousWeekActionID,
		SelectionNextWeekActionID,
		SelectionPreviousMonthActionID,
		SelectionNextMonthActionID,
		SelectionFirstDayOfMonthActionID,
		SelectionLastDayOfMonthActionID,
	}
	expectedLabels := []string{
		"Activate",
		"Previous day",
		"Next day",
		"Previous week",
		"Next week",
		"Previous month",
		"Next month",
		"First day of month",
		"Last day of month",
	}
	if resolved.Owner() != tui.NewNodeID("calendar") {
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
		availability := tui.ActionDisabledPassThrough
		if expectedAvailable[index] {
			availability = tui.ActionEnabled
		}
		if action.Availability() != availability {
			t.Errorf("action %s availability = %d, want %d", expectedIDs[index], action.Availability(), availability)
		}
	}
}

func calendarActionFixtureDisplayed(t *testing.T, value string) (int, int) {
	t.Helper()
	year, month, ok := strings.Cut(value, "-")
	if !ok {
		t.Fatalf("invalid Calendar displayed month %q", value)
	}
	return calendarActionFixtureInt(t, year), calendarActionFixtureInt(t, month)
}

func calendarActionFixtureDate(t *testing.T, value string) CalendarDate {
	t.Helper()
	parts := strings.Split(value, "-")
	if len(parts) != 3 {
		t.Fatalf("invalid Calendar date %q", value)
	}
	return NewCalendarDate(
		calendarActionFixtureInt(t, parts[0]),
		calendarActionFixtureInt(t, parts[1]),
		calendarActionFixtureInt(t, parts[2]),
	)
}

func calendarActionFixtureDates(t *testing.T, value string) []CalendarDate {
	t.Helper()
	if value == "-" {
		return nil
	}
	parts := strings.Split(value, ",")
	dates := make([]CalendarDate, len(parts))
	for index, part := range parts {
		dates[index] = calendarActionFixtureDate(t, part)
	}
	return dates
}

func calendarActionFixtureBools(t *testing.T, value string) []bool {
	t.Helper()
	parts := strings.Split(value, ",")
	values := make([]bool, len(parts))
	for index, part := range parts {
		values[index] = fixtureBool(t, part)
	}
	return values
}

func calendarActionFixtureKeyGroups(value string) [][]string {
	groups := strings.Split(value, "|")
	output := make([][]string, len(groups))
	for index, group := range groups {
		if group != "-" {
			output[index] = strings.Split(group, ",")
		}
	}
	return output
}

func calendarActionFixtureFocus(value string) (tui.NodeID, bool) {
	if value == "none" {
		return "", false
	}
	return tui.NewNodeID(value), true
}

func calendarActionFixtureInt(t *testing.T, value string) int {
	t.Helper()
	parsed, err := strconv.Atoi(value)
	if err != nil {
		t.Fatalf("invalid fixture integer %q: %v", value, err)
	}
	return parsed
}

func calendarActionFixtureUint32(t *testing.T, value string) uint32 {
	t.Helper()
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		t.Fatalf("invalid fixture uint32 %q: %v", value, err)
	}
	return uint32(parsed)
}
