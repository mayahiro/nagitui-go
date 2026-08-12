package widget

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	tui "github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/internal/conformance"
)

type splitPaneActionApp struct {
	state    SplitPaneState
	axis     tui.SplitPaneAxis
	step     uint16
	focus    bool
	resize   bool
	messages []string
}

func (*splitPaneActionApp) Init() tui.Effect[string] { return tui.NoneEffect[string]() }

func (a *splitPaneActionApp) Update(message string) tui.Effect[string] {
	if value, ok := strings.CutPrefix(message, "ratio:"); ok {
		ratio, _ := strconv.ParseUint(value, 10, 16)
		a.state = NewSplitPaneState(uint16(ratio))
	}
	a.messages = append(a.messages, message)
	return tui.NoneEffect[string]()
}

func (*splitPaneActionApp) Subscriptions() tui.Subscription[string] {
	return tui.NoneSubscription[string]()
}

func (a *splitPaneActionApp) View(tui.ViewContext) tui.Node[string] {
	split := NewSplitPane(
		tui.NewNodeID("split"),
		tui.Text[string]("primary").Focusable(tui.NewNodeID("primary")),
		tui.Text[string]("secondary").Focusable(tui.NewNodeID("secondary")),
		a.state,
	).Axis(a.axis).ResizeStep(a.step)
	if a.focus {
		split = split.FocusTargets(tui.NewNodeID("primary"), tui.NewNodeID("secondary"))
	}
	if a.resize {
		split = split.OnResize(func(state SplitPaneState) string {
			return "ratio:" + strconv.Itoa(int(state.Ratio()))
		})
	}
	return tui.Padding(split.Node(), tui.UniformInsets(0)).OnEvent(
		tui.NewNodeID("outer"),
		func(vt.Event) tui.EventResult[string] { return tui.MessageResult("raw") },
	)
}

func TestSplitPaneActionsMatchSharedFixtures(t *testing.T) {
	records, err := conformance.Load(
		"widgets/split-pane.txt",
		"widget-split-pane",
		"ratio", "axis", "step", "focus", "resize", "event", "start",
		"expected-ratio", "message", "consumed", "expected-focus", "availability",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			state := NewSplitPaneState(splitPaneFixtureU16(t, record.Field("ratio")))
			axis := splitPaneFixtureWidgetAxis(t, record.Field("axis"))
			focus := splitPaneFixtureBool(t, record.Field("focus"))
			resize := splitPaneFixtureBool(t, record.Field("resize"))
			split := NewSplitPane(
				tui.NewNodeID("split"), tui.Text[string]("primary"), tui.Text[string]("secondary"), state,
			).Axis(axis)
			if focus {
				split = split.FocusTargets(tui.NewNodeID("primary"), tui.NewNodeID("secondary"))
			}
			if resize {
				split = split.OnResize(func(next SplitPaneState) string {
					return "ratio:" + strconv.Itoa(int(next.Ratio()))
				})
			}
			descriptors := split.ActionDescriptors()
			actualIDs := make([]string, len(descriptors))
			actualAvailability := make([]string, len(descriptors))
			for index, descriptor := range descriptors {
				actualIDs[index] = descriptor.ID().String()
				actualAvailability[index] = splitPaneFixtureAvailability(descriptor.Availability())
			}
			expectedIDs := []string{
				PaneFocusPreviousActionID.String(), PaneFocusNextActionID.String(),
				PaneResizePreviousActionID.String(), PaneResizeNextActionID.String(),
			}
			if !stringSlicesEqual(actualIDs, expectedIDs) {
				t.Fatalf("action IDs = %v, want %v", actualIDs, expectedIDs)
			}
			if expected := splitPaneFixtureList(record.Field("availability")); !stringSlicesEqual(actualAvailability, expected) {
				t.Fatalf("availability = %v, want %v", actualAvailability, expected)
			}

			app := &splitPaneActionApp{
				state: state, axis: axis, step: splitPaneFixtureU16(t, record.Field("step")),
				focus: focus, resize: resize,
			}
			runtime, err := tui.NewRuntimeWithClock(
				app, tui.NewRuntimeConfig(tui.Size{Width: 30, Height: 6}), tui.NewVirtualClock(),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer runtime.Close()
			if _, err := runtime.RenderIfDirty(); err != nil {
				t.Fatal(err)
			}
			if focused, err := runtime.RequestFocus(tui.NewNodeID(record.Field("start"))); err != nil || !focused {
				t.Fatalf("initial focus = %t, %v", focused, err)
			}

			var dispatch *tui.EventDispatch
			if record.Field("event") != "none" {
				result, err := runtime.DispatchEvent(splitPaneFixtureEvent(t, record.Field("event")))
				if err != nil {
					t.Fatal(err)
				}
				dispatch = &result
				if _, err := runtime.ProcessPending(); err != nil {
					t.Fatal(err)
				}
			}
			expectedMessages := splitPaneFixtureList(record.Field("message"))
			if !stringSlicesEqual(app.messages, expectedMessages) {
				t.Fatalf("messages = %v, want %v", app.messages, expectedMessages)
			}
			if dispatch != nil {
				if dispatch.Consumed() != splitPaneFixtureBool(t, record.Field("consumed")) || dispatch.Messages() != len(expectedMessages) {
					t.Fatalf("dispatch = consumed %t messages %d", dispatch.Consumed(), dispatch.Messages())
				}
			}
			if app.state.Ratio() != splitPaneFixtureU16(t, record.Field("expected-ratio")) {
				t.Fatalf("ratio = %d", app.state.Ratio())
			}
			focused, ok := runtime.Interaction().Focused()
			if !ok || focused.String() != record.Field("expected-focus") {
				t.Fatalf("focus = %s, %t", focused, ok)
			}
		})
	}
}

func splitPaneFixtureEvent(t *testing.T, value string) vt.Event {
	t.Helper()
	code, modifiers, action := vt.KeyUnknown, (vt.Modifiers{}), vt.KeyPress
	switch value {
	case "f6":
		code = vt.KeyFunction
	case "shift-f6":
		code, modifiers = vt.KeyFunction, vt.Modifiers{Shift: true}
	case "alt-left":
		code, modifiers = vt.KeyLeft, vt.Modifiers{Alt: true}
	case "alt-right":
		code, modifiers = vt.KeyRight, vt.Modifiers{Alt: true}
	case "repeat-alt-right":
		code, modifiers, action = vt.KeyRight, vt.Modifiers{Alt: true}, vt.KeyRepeat
	case "alt-up":
		code, modifiers = vt.KeyUp, vt.Modifiers{Alt: true}
	case "alt-down":
		code, modifiers = vt.KeyDown, vt.Modifiers{Alt: true}
	default:
		t.Fatalf("unknown event %q", value)
	}
	key := vt.KeyEvent{Code: code, Modifiers: modifiers, Action: action, Protocol: vt.KeyProtocolLegacy}
	if code == vt.KeyFunction {
		key.Function = 6
	}
	return vt.Event{Kind: vt.EventKey, Key: key}
}

func splitPaneFixtureWidgetAxis(t *testing.T, value string) tui.SplitPaneAxis {
	t.Helper()
	if value == "vertical" {
		return tui.SplitPaneVertical
	}
	if value != "horizontal" {
		t.Fatalf("invalid axis %q", value)
	}
	return tui.SplitPaneHorizontal
}

func splitPaneFixtureAvailability(value tui.ActionAvailability) string {
	switch value {
	case tui.ActionEnabled:
		return "enabled"
	case tui.ActionDisabledPassThrough:
		return "pass"
	case tui.ActionDisabledConsume:
		return "consume"
	default:
		return "invalid"
	}
}

func splitPaneFixtureBool(t *testing.T, value string) bool {
	t.Helper()
	if value == "true" {
		return true
	}
	if value != "false" {
		t.Fatalf("invalid bool %q", value)
	}
	return false
}

func splitPaneFixtureU16(t *testing.T, value string) uint16 {
	t.Helper()
	parsed, err := strconv.ParseUint(value, 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	return uint16(parsed)
}

func splitPaneFixtureList(value string) []string {
	if value == "-" {
		return nil
	}
	return strings.Split(value, ",")
}

func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
