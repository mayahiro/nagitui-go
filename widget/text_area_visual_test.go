package widget

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/internal/conformance"
)

type visualTextAreaApp struct {
	state    TextAreaState
	wrap     int
	hasWrap  bool
	boundary TextAreaBoundaryNavigation
	height   uint32
}

func (*visualTextAreaApp) Init() tui.Effect[TextAreaState] {
	return tui.NoneEffect[TextAreaState]()
}

func (a *visualTextAreaApp) Update(state TextAreaState) tui.Effect[TextAreaState] {
	a.state = state
	return tui.NoneEffect[TextAreaState]()
}

func (*visualTextAreaApp) Subscriptions() tui.Subscription[TextAreaState] {
	return tui.NoneSubscription[TextAreaState]()
}

func (a *visualTextAreaApp) View(tui.ViewContext) tui.Node[TextAreaState] {
	area := NewTextArea(tui.NewNodeID("area"), a.state, func(state TextAreaState) TextAreaState {
		return state
	}).BoundaryNavigation(a.boundary).Viewport(
		tui.NewNodeID("area-viewport"),
		tui.NewNodeID("area-caret"),
		tui.Fixed(a.height),
	)
	if a.hasWrap {
		area = area.SoftWrap(a.wrap)
	}
	return area.Node()
}

func TestTextAreaVisualFixtures(t *testing.T) {
	records := loadTextAreaVisualFixtures(t)
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			wrap, hasWrap := optionalFixtureInt(t, record.Field("wrap"))
			state := NewTextAreaState(
				record.Text("initial"),
				fixtureInt(t, record.Field("cursor")),
			)
			if record.Field("events") != "-" {
				for _, event := range strings.Split(record.Field("events"), ",") {
					state = textAreaStateForAction(
						state,
						visualTextAreaAction(t, event),
						max(wrap, 1),
						hasWrap,
						celltext.ModernWidth(),
					)
				}
			}

			if state.Cursor() != fixtureInt(t, record.Field("expected-cursor")) {
				t.Errorf("cursor = %d", state.Cursor())
			}
			expectedPreferred, hasExpectedPreferred := optionalFixtureInt(t, record.Field("expected-preferred"))
			if preferred, ok := state.PreferredColumn(); preferred != expectedPreferred || ok != hasExpectedPreferred {
				t.Errorf("preferred column = %d, %t, want %d, %t", preferred, ok, expectedPreferred, hasExpectedPreferred)
			}
			assertVisualTextAreaSelection(t, state, record.Field("expected-selection"))

			lines := textAreaVisualLineRanges(state.value, max(wrap, 1), hasWrap, celltext.ModernWidth())
			expectedLines := fixtureTextAreaRanges(t, record.Field("expected-ranges"))
			if !slices.Equal(lines, expectedLines) {
				t.Errorf("ranges = %#v, want %#v", lines, expectedLines)
			}
			if line := textAreaVisualLineIndex(lines, state.cursor); line != fixtureInt(t, record.Field("expected-cursor-line")) {
				t.Errorf("cursor line = %d", line)
			}

			boundary := visualTextAreaBoundary(t, record.Field("boundary"))
			descriptors := textAreaActionDescriptors(
				true, false, false, boundary, state, max(wrap, 1), hasWrap, celltext.ModernWidth(),
			)
			assertTextAreaDescriptorAvailability(t, descriptors[2], record.Field("expected-up"))
			assertTextAreaDescriptorAvailability(t, descriptors[3], record.Field("expected-down"))
		})
	}
}

func TestTextAreaVisualViewportFixtures(t *testing.T) {
	records := loadTextAreaVisualFixtures(t)
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			wrap, hasWrap := optionalFixtureInt(t, record.Field("wrap"))
			height := uint32(fixtureInt(t, record.Field("height")))
			app := &visualTextAreaApp{
				state: NewTextAreaState(
					record.Text("initial"),
					fixtureInt(t, record.Field("cursor")),
				),
				wrap: wrap, hasWrap: hasWrap,
				boundary: visualTextAreaBoundary(t, record.Field("boundary")),
				height:   height,
			}
			runtime, err := tui.NewRuntimeWithClock(
				app,
				tui.NewRuntimeConfig(tui.Size{Width: 12, Height: height}),
				tui.NewVirtualClock(),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer runtime.Close()
			if _, err := runtime.RenderIfDirty(); err != nil {
				t.Fatal(err)
			}
			focused, err := runtime.RequestFocus(tui.NewNodeID("area"))
			if err != nil || !focused {
				t.Fatalf("RequestFocus = %t, %v", focused, err)
			}
			if _, err := runtime.RenderIfDirty(); err != nil {
				t.Fatal(err)
			}

			var lastConsumed *bool
			if record.Field("events") != "-" {
				for _, event := range strings.Split(record.Field("events"), ",") {
					dispatch, err := runtime.DispatchEvent(visualTextAreaEvent(t, event))
					if err != nil {
						t.Fatal(err)
					}
					consumed := dispatch.Consumed()
					lastConsumed = &consumed
					if _, err := runtime.ProcessPending(); err != nil {
						t.Fatal(err)
					}
					if _, err := runtime.RenderIfDirty(); err != nil {
						t.Fatal(err)
					}
				}
			}

			if app.state.Cursor() != fixtureInt(t, record.Field("expected-cursor")) {
				t.Errorf("cursor = %d", app.state.Cursor())
			}
			expectedPreferred, hasExpectedPreferred := optionalFixtureInt(t, record.Field("expected-preferred"))
			if preferred, ok := app.state.PreferredColumn(); preferred != expectedPreferred || ok != hasExpectedPreferred {
				t.Errorf("preferred column = %d, %t, want %d, %t", preferred, ok, expectedPreferred, hasExpectedPreferred)
			}
			assertVisualTextAreaSelection(t, app.state, record.Field("expected-selection"))
			if offset := runtime.Interaction().ScrollOffset(tui.NewNodeID("area-viewport")); offset.Y != uint32(fixtureInt(t, record.Field("expected-offset"))) {
				t.Errorf("offset = %+v", offset)
			}
			if record.Field("expected-consumed") != "-" {
				expected := record.Field("expected-consumed") == "true"
				if lastConsumed == nil || *lastConsumed != expected {
					t.Errorf("last consumed = %v, want %t", lastConsumed, expected)
				}
			}

			groups, err := runtime.ActiveActionGroups()
			if err != nil {
				t.Fatal(err)
			}
			var areaActions []tui.ResolvedAction
			for _, group := range groups {
				if group.Owner() == tui.NewNodeID("area") {
					areaActions = group.Actions()
					break
				}
			}
			if areaActions == nil {
				t.Fatal("TextArea action group was not active")
			}
			assertResolvedTextAreaAvailability(t, areaActions, tui.TextCursorUpActionID, record.Field("expected-up"))
			assertResolvedTextAreaAvailability(t, areaActions, tui.TextCursorDownActionID, record.Field("expected-down"))
		})
	}
}

func loadTextAreaVisualFixtures(t *testing.T) []conformance.Record {
	t.Helper()
	return loadWidgetFixtures(
		t,
		"widgets/text-area-visual.txt",
		"widget-text-area-visual",
		"initial", "cursor", "wrap", "boundary", "events", "height",
		"expected-cursor", "expected-preferred", "expected-selection",
		"expected-ranges", "expected-cursor-line", "expected-offset",
		"expected-up", "expected-down", "expected-consumed",
	)
}

func visualTextAreaAction(t *testing.T, value string) textAreaSemanticAction {
	t.Helper()
	switch value {
	case "up":
		return textAreaCursorUp
	case "down":
		return textAreaCursorDown
	case "left":
		return textAreaCursorLeft
	case "right":
		return textAreaCursorRight
	case "shift-up":
		return textAreaSelectionExtendUp
	case "shift-down":
		return textAreaSelectionExtendDown
	default:
		t.Fatalf("invalid visual TextArea action %q", value)
		return 0
	}
}

func visualTextAreaEvent(t *testing.T, value string) vt.Event {
	t.Helper()
	shift := vt.Modifiers{Shift: true}
	switch value {
	case "up":
		return keyEvent(vt.KeyUp, 0, vt.Modifiers{}, vt.KeyPress)
	case "down":
		return keyEvent(vt.KeyDown, 0, vt.Modifiers{}, vt.KeyPress)
	case "left":
		return keyEvent(vt.KeyLeft, 0, vt.Modifiers{}, vt.KeyPress)
	case "right":
		return keyEvent(vt.KeyRight, 0, vt.Modifiers{}, vt.KeyPress)
	case "shift-up":
		return keyEvent(vt.KeyUp, 0, shift, vt.KeyPress)
	case "shift-down":
		return keyEvent(vt.KeyDown, 0, shift, vt.KeyPress)
	default:
		t.Fatalf("invalid visual TextArea event %q", value)
		return vt.Event{}
	}
}

func visualTextAreaBoundary(t *testing.T, value string) TextAreaBoundaryNavigation {
	t.Helper()
	switch value {
	case "consume":
		return TextAreaBoundaryConsume
	case "bubble":
		return TextAreaBoundaryBubble
	default:
		t.Fatalf("invalid boundary %q", value)
		return 0
	}
}

func assertTextAreaDescriptorAvailability(t *testing.T, descriptor tui.ActionDescriptor, expected string) {
	t.Helper()
	if descriptor.Availability() != visualTextAreaAvailability(t, expected) {
		t.Errorf("availability = %v, want %s", descriptor.Availability(), expected)
	}
}

func assertResolvedTextAreaAvailability(
	t *testing.T,
	actions []tui.ResolvedAction,
	id tui.ActionID,
	expected string,
) {
	t.Helper()
	for _, action := range actions {
		if action.ID() == id {
			if action.Availability() != visualTextAreaAvailability(t, expected) {
				t.Errorf("action %s availability = %v, want %s", id, action.Availability(), expected)
			}
			return
		}
	}
	t.Errorf("action %s was not resolved", id)
}

func visualTextAreaAvailability(t *testing.T, value string) tui.ActionAvailability {
	t.Helper()
	switch value {
	case "enabled":
		return tui.ActionEnabled
	case "pass":
		return tui.ActionDisabledPassThrough
	default:
		t.Fatalf("invalid availability %q", value)
		return 0
	}
}

func assertVisualTextAreaSelection(t *testing.T, state TextAreaState, expected string) {
	t.Helper()
	start, end, ok := state.Selection()
	if expected == "-" {
		if ok {
			t.Errorf("selection = %d:%d", start, end)
		}
		return
	}
	expectedRange := fixtureTextAreaRange(t, expected)
	if !ok || start != expectedRange.start || end != expectedRange.end {
		t.Errorf("selection = %d:%d, %t, want %s", start, end, ok, expected)
	}
}

func fixtureTextAreaRanges(t *testing.T, value string) []textAreaRange {
	t.Helper()
	parts := strings.Split(value, ",")
	ranges := make([]textAreaRange, len(parts))
	for index, part := range parts {
		ranges[index] = fixtureTextAreaRange(t, part)
	}
	return ranges
}

func fixtureTextAreaRange(t *testing.T, value string) textAreaRange {
	t.Helper()
	start, end, ok := strings.Cut(value, ":")
	if !ok {
		t.Fatalf("invalid range %q", value)
	}
	return textAreaRange{start: fixtureInt(t, start), end: fixtureInt(t, end)}
}

func optionalFixtureInt(t *testing.T, value string) (int, bool) {
	t.Helper()
	if value == "-" {
		return 0, false
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		t.Fatalf("invalid integer %q: %v", value, err)
	}
	return parsed, true
}
