package widget

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

type paginatorActionApp struct {
	display  PaginatorMode
	page     int
	total    int
	limit    int
	enabled  bool
	keyMap   tui.KeyMap
	messages []int
}

func (*paginatorActionApp) Init() tui.Effect[int] {
	return tui.NoneEffect[int]()
}

func (a *paginatorActionApp) Update(message int) tui.Effect[int] {
	a.page = message
	a.messages = append(a.messages, message)
	return tui.NoneEffect[int]()
}

func (*paginatorActionApp) Subscriptions() tui.Subscription[int] {
	return tui.NoneSubscription[int]()
}

func (a *paginatorActionApp) View(tui.ViewContext) tui.Node[int] {
	paginator := fixturePaginator(a.display, a.page, a.total, a.limit, a.enabled).Node()
	return tui.Padding(paginator, tui.UniformInsets(0)).
		WithKeyScope(tui.NewKeyScope(tui.NewNodeID("scope"), a.keyMap))
}

func TestPaginatorActionsMatchSharedFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t,
		"widgets/paginator-action.txt",
		"widget-paginator-action",
		"display", "total", "page", "limit", "focus", "mode", "enabled", "event",
		"messages", "consumed", "focus-after", "page-after", "keys", "available",
	)
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			display := paginatorFixtureDisplay(record.Field("display"))
			total := fixtureInt(t, record.Field("total"))
			page := fixtureInt(t, record.Field("page"))
			limit := fixtureInt(t, record.Field("limit"))
			enabled := fixtureBool(t, record.Field("enabled"))
			keyMap := paginatorActionKeyMap(t, record.Field("mode"))
			paginator := fixturePaginator(display, page, total, limit, enabled)
			resolved, err := tui.ResolveActions(
				tui.NewNodeID("pages"),
				paginator.ActionDescriptors(),
				[]tui.KeyScope{tui.NewKeyScope(tui.NewNodeID("scope"), keyMap)},
			)
			if err != nil {
				t.Fatal(err)
			}
			assertPaginatorActionGroup(
				t,
				resolved,
				paginatorFixtureKeyGroups(record.Field("keys")),
				fixtureBool(t, record.Field("available")),
			)

			app := &paginatorActionApp{
				display: display, page: page, total: total, limit: limit,
				enabled: enabled, keyMap: keyMap,
			}
			runtime, err := tui.NewRuntimeWithClock(
				app,
				tui.NewRuntimeConfig(tui.Size{Width: 32, Height: 2}),
				tui.NewVirtualClock(),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer runtime.Close()
			if _, err := runtime.RenderIfDirty(); err != nil {
				t.Fatal(err)
			}
			if record.Field("focus") == "pages" {
				focused, err := runtime.RequestFocus(tui.NewNodeID("pages"))
				if err != nil || !focused {
					t.Fatalf("focus = %t, %v", focused, err)
				}
				groups, err := runtime.ActiveActionGroups()
				if err != nil {
					t.Fatal(err)
				}
				groups = nodeDeclaredActionGroups(groups)
				if len(groups) != 1 || groups[0].Owner() != tui.NewNodeID("pages") {
					t.Fatalf("action groups = %#v", groups)
				}
			}

			event := paginatorActionEvent(
				t,
				record.Field("event"),
				display,
				page,
				total,
				limit,
			)
			dispatch, err := runtime.DispatchEvent(event)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runtime.ProcessPending(); err != nil {
				t.Fatal(err)
			}
			expectedMessages := paginatorFixtureInts(t, record.Field("messages"))
			if !slices.Equal(app.messages, expectedMessages) {
				t.Errorf("messages = %#v, want %#v", app.messages, expectedMessages)
			}
			if dispatch.Messages() != len(expectedMessages) {
				t.Errorf("dispatched messages = %d, want %d", dispatch.Messages(), len(expectedMessages))
			}
			if dispatch.Consumed() != fixtureBool(t, record.Field("consumed")) {
				t.Errorf("consumed = %t", dispatch.Consumed())
			}
			expectedFocus, expectsFocus := paginatorFixtureFocus(record.Field("focus-after"))
			actualFocus, hasFocus := runtime.Interaction().Focused()
			if hasFocus != expectsFocus || expectsFocus && actualFocus != expectedFocus {
				t.Errorf("focus = %q, %t, want %q, %t", actualFocus, hasFocus, expectedFocus, expectsFocus)
			}
			if expected := fixtureInt(t, record.Field("page-after")); app.page != expected {
				t.Errorf("page = %d, want %d", app.page, expected)
			}
		})
	}
}

func TestPaginatorRootConflictIsReportedBeforeDispatch(t *testing.T) {
	binding := []tui.KeyBinding{
		tui.NewKeyBinding(tui.NewCharacterKeyStroke('x', vt.Modifiers{})),
	}
	keyMap, err := tui.NewKeyMap().Rebind(SelectionPreviousActionID, binding)
	if err != nil {
		t.Fatal(err)
	}
	keyMap, err = keyMap.Rebind(SelectionNextActionID, binding)
	if err != nil {
		t.Fatal(err)
	}
	app := &paginatorActionApp{
		display: PaginatorDots, page: 2, total: 5, enabled: true, keyMap: keyMap,
	}
	runtime, err := tui.NewRuntimeWithClock(
		app,
		tui.NewRuntimeConfig(tui.Size{Width: 32, Height: 2}),
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
	if conflict.Kind() != tui.ConflictAmbiguousBinding || conflict.Owner() != tui.NewNodeID("pages") {
		t.Fatalf("conflict = kind %d owner %s", conflict.Kind(), conflict.Owner())
	}
	expected := []tui.ActionID{SelectionPreviousActionID, SelectionNextActionID}
	if !slices.Equal(conflict.Actions(), expected) {
		t.Fatalf("conflict actions = %#v, want %#v", conflict.Actions(), expected)
	}
	if len(app.messages) != 0 {
		t.Fatalf("conflicting action emitted messages = %#v", app.messages)
	}
}

func fixturePaginator(display PaginatorMode, page, total, limit int, enabled bool) Paginator[int] {
	return NewPaginator(tui.NewNodeID("pages"), page, total, func(page int) int { return page }).
		Mode(display).
		IndicatorLimit(limit).
		Enabled(enabled)
}

func paginatorActionKeyMap(t *testing.T, mode string) tui.KeyMap {
	t.Helper()
	keyMap := tui.NewKeyMap()
	var (
		resolved tui.KeyMap
		err      error
	)
	switch mode {
	case "default":
		return keyMap
	case "previous-k":
		resolved, err = keyMap.Rebind(SelectionPreviousActionID, []tui.KeyBinding{
			tui.NewKeyBinding(tui.NewCharacterKeyStroke('k', vt.Modifiers{})),
		})
	case "unbind-next":
		resolved, err = keyMap.Rebind(SelectionNextActionID, nil)
	default:
		t.Fatalf("unknown Paginator action mode %q", mode)
	}
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func paginatorActionEvent(
	t *testing.T,
	value string,
	display PaginatorMode,
	page, total, limit int,
) vt.Event {
	t.Helper()
	keyboard := func(code vt.KeyCode, character rune, modifiers vt.Modifiers, action vt.KeyAction) vt.Event {
		return keyEvent(code, character, modifiers, action)
	}
	switch value {
	case "left":
		return keyboard(vt.KeyLeft, 0, vt.Modifiers{}, vt.KeyPress)
	case "up":
		return keyboard(vt.KeyUp, 0, vt.Modifiers{}, vt.KeyPress)
	case "page-up":
		return keyboard(vt.KeyPageUp, 0, vt.Modifiers{}, vt.KeyPress)
	case "right":
		return keyboard(vt.KeyRight, 0, vt.Modifiers{}, vt.KeyPress)
	case "down":
		return keyboard(vt.KeyDown, 0, vt.Modifiers{}, vt.KeyPress)
	case "page-down":
		return keyboard(vt.KeyPageDown, 0, vt.Modifiers{}, vt.KeyPress)
	case "home":
		return keyboard(vt.KeyHome, 0, vt.Modifiers{}, vt.KeyPress)
	case "end":
		return keyboard(vt.KeyEnd, 0, vt.Modifiers{}, vt.KeyPress)
	case "repeat-left":
		return keyboard(vt.KeyLeft, 0, vt.Modifiers{}, vt.KeyRepeat)
	case "unknown-left":
		return keyboard(vt.KeyLeft, 0, vt.Modifiers{}, vt.KeyActionUnknown)
	case "release-left":
		return keyboard(vt.KeyLeft, 0, vt.Modifiers{}, vt.KeyRelease)
	case "shift-left":
		return keyboard(vt.KeyLeft, 0, vt.Modifiers{Shift: true}, vt.KeyPress)
	case "control-left":
		return keyboard(vt.KeyLeft, 0, vt.Modifiers{Control: true}, vt.KeyPress)
	case "k":
		return keyboard(vt.KeyCharacter, 'k', vt.Modifiers{}, vt.KeyPress)
	default:
		if strings.HasPrefix(value, "mouse-") {
			kind, button, candidate := paginatorPointerEvent(t, value)
			x := 0
			if display == PaginatorDots && total > 0 {
				start, _ := paginatorWindow(total, page, limit)
				x = max(candidate-start, 0) * 2
			}
			return vt.Event{
				Kind: vt.EventMouse,
				Mouse: vt.MouseEvent{
					Kind: kind, Button: button, X: uint32(x), Y: 0,
				},
			}
		}
		t.Fatalf("unknown Paginator action event %q", value)
		return vt.Event{}
	}
}

func paginatorPointerEvent(t *testing.T, value string) (vt.MouseKind, vt.MouseButton, int) {
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
	t.Fatalf("invalid Paginator pointer event %q", value)
	return 0, 0, 0
}

func assertPaginatorActionGroup(
	t *testing.T,
	resolved tui.ResolvedActions,
	expectedKeys [][]string,
	available bool,
) {
	t.Helper()
	expectedIDs := []tui.ActionID{
		SelectionPreviousActionID,
		SelectionNextActionID,
		SelectionFirstActionID,
		SelectionLastActionID,
	}
	expectedLabels := []string{"Previous", "Next", "First", "Last"}
	if resolved.Owner() != tui.NewNodeID("pages") {
		t.Fatalf("owner = %q", resolved.Owner())
	}
	actions := resolved.Actions()
	if len(actions) != len(expectedIDs) || len(expectedKeys) != len(expectedIDs) {
		t.Fatalf("actions = %d, key groups = %d", len(actions), len(expectedKeys))
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

func paginatorFixtureDisplay(value string) PaginatorMode {
	switch value {
	case "dots":
		return PaginatorDots
	case "numeric":
		return PaginatorNumeric
	default:
		panic("invalid Paginator display " + value)
	}
}

func paginatorFixtureInts(t *testing.T, value string) []int {
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

func paginatorFixtureKeyGroups(value string) [][]string {
	groups := strings.Split(value, "|")
	result := make([][]string, len(groups))
	for index, group := range groups {
		if group != "-" {
			result[index] = strings.Split(group, ",")
		}
	}
	return result
}

func paginatorFixtureFocus(value string) (tui.NodeID, bool) {
	if value == "none" {
		return tui.NodeID(""), false
	}
	return tui.NewNodeID(value), true
}
