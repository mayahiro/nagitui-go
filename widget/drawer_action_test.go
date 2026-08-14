package widget

import (
	"errors"
	"strconv"
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	tui "github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/internal/conformance"
)

type drawerActionApp struct {
	open, modal, body, dismiss bool
	side                       DrawerSide
	builds                     int
	messages                   []string
}

func (*drawerActionApp) Init() tui.Effect[string] { return tui.NoneEffect[string]() }

func (a *drawerActionApp) Update(message string) tui.Effect[string] {
	if message == "false" {
		a.open = false
	}
	a.messages = append(a.messages, message)
	return tui.NoneEffect[string]()
}

func (*drawerActionApp) Subscriptions() tui.Subscription[string] {
	return tui.NoneSubscription[string]()
}

func (a *drawerActionApp) View(tui.ViewContext) tui.Node[string] {
	base := tui.Text[string]("bbbbbbbbbb\nbbbbbbbbbb\nbbbbbbbbbb\nbbbbbbbbbb\nbbbbbbbbbb").
		Focusable(tui.NewNodeID("base"))
	size := tui.Fixed(4)
	if a.side == DrawerTop || a.side == DrawerBottom {
		size = tui.Fixed(3)
	}
	drawer := NewDrawer(tui.NewNodeID("drawer"), base, a.open).
		Side(a.side).
		Size(size).
		Modal(a.modal)
	if a.body {
		drawer = drawer.Body(func() tui.Node[string] {
			a.builds++
			return tui.Text[string]("D").Focusable(tui.NewNodeID("drawer-focus"))
		})
	}
	if a.dismiss {
		drawer = drawer.OnDismiss(func() string { return "false" })
	}
	return tui.Padding(drawer.Node(), tui.UniformInsets(0)).OnEvent(
		tui.NewNodeID("outer"),
		func(vt.Event) tui.EventResult[string] { return tui.MessageResult("raw") },
	)
}

func TestDrawerBehaviorMatchesSharedFixtures(t *testing.T) {
	records, err := conformance.Load(
		"widgets/drawer.txt",
		"widget-drawer",
		"open", "side", "modal", "body", "dismiss", "event", "start",
		"expected-open", "message", "consumed", "expected-focus", "builds",
		"expected-body", "availability",
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
			open := drawerFixtureBool(t, record.Field("open"))
			modal := drawerFixtureBool(t, record.Field("modal"))
			start := record.Field("start")
			enterFromBase := open && modal && start == "base"
			app := &drawerActionApp{
				open:    open && !enterFromBase,
				side:    drawerFixtureSide(t, record.Field("side")),
				modal:   modal,
				body:    drawerFixtureBool(t, record.Field("body")),
				dismiss: drawerFixtureBool(t, record.Field("dismiss")),
			}
			runtime, err := tui.NewRuntimeWithClock(
				app, tui.NewRuntimeConfig(tui.Size{Width: 10, Height: 5}), tui.NewVirtualClock(),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer runtime.Close()
			frame, err := runtime.RenderIfDirty()
			if err != nil || frame == nil {
				t.Fatalf("initial render = %v, %v", frame, err)
			}
			if start == "base" {
				if focused, err := runtime.RequestFocus(tui.NewNodeID("base")); err != nil || !focused {
					t.Fatalf("base focus = %t, %v", focused, err)
				}
			}
			if enterFromBase {
				app.open = true
				runtime.RequestFrame()
				frame, err = runtime.RenderIfDirty()
				if err != nil || frame == nil {
					t.Fatalf("open render = %v, %v", frame, err)
				}
			} else if start == "drawer-focus" {
				if focused, err := runtime.RequestFocus(tui.NewNodeID("drawer-focus")); err != nil || !focused {
					t.Fatalf("drawer focus = %t, %v", focused, err)
				}
			}

			descriptor := configuredGoDrawer(app).ActionDescriptor()
			if descriptor.ID() != DismissActionID {
				t.Fatalf("action ID = %s", descriptor.ID())
			}
			if actual := splitPaneFixtureAvailability(descriptor.Availability()); actual != record.Field("availability") {
				t.Fatalf("availability = %s", actual)
			}

			var dispatch *tui.EventDispatch
			if record.Field("event") != "none" {
				result, err := runtime.DispatchEvent(drawerEscapeEvent())
				if err != nil {
					t.Fatal(err)
				}
				dispatch = &result
				if _, err := runtime.ProcessPending(); err != nil {
					t.Fatal(err)
				}
				if rendered, err := runtime.RenderIfDirty(); err != nil {
					t.Fatal(err)
				} else if rendered != nil {
					frame = rendered
				}
			}

			expectedMessages := drawerFixtureList(record.Field("message"))
			if !stringSlicesEqual(app.messages, expectedMessages) {
				t.Fatalf("messages = %v, want %v", app.messages, expectedMessages)
			}
			if dispatch != nil && (dispatch.Consumed() != drawerFixtureBool(t, record.Field("consumed")) || dispatch.Messages() != len(expectedMessages)) {
				t.Fatalf("dispatch = consumed %t messages %d", dispatch.Consumed(), dispatch.Messages())
			}
			if app.open != drawerFixtureBool(t, record.Field("expected-open")) {
				t.Fatalf("open = %t", app.open)
			}
			expectedBuilds, err := strconv.Atoi(record.Field("builds"))
			if err != nil || app.builds != expectedBuilds {
				t.Fatalf("builds = %d, want %d, parse %v", app.builds, expectedBuilds, err)
			}
			focused, ok := runtime.Interaction().Focused()
			if !ok || focused.String() != record.Field("expected-focus") {
				t.Fatalf("focus = %s, %t", focused, ok)
			}
			assertGoDrawerBodyPosition(t, frame, record.Field("expected-body"))
		})
	}
}

func TestDrawerUnknownSideUsesLeftEdge(t *testing.T) {
	app := &drawerActionApp{
		open: true, side: DrawerSide(255), modal: false, body: true,
	}
	runtime, err := tui.NewRuntimeWithClock(
		app, tui.NewRuntimeConfig(tui.Size{Width: 10, Height: 5}), tui.NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	frame, err := runtime.RenderIfDirty()
	if err != nil || frame == nil {
		t.Fatalf("render = %v, %v", frame, err)
	}
	assertGoDrawerBodyPosition(t, frame, "1:1")
}

func configuredGoDrawer(app *drawerActionApp) Drawer[string] {
	drawer := NewDrawer(tui.NewNodeID("drawer"), tui.Text[string]("base"), app.open)
	if app.dismiss {
		drawer = drawer.OnDismiss(func() string { return "false" })
	}
	return drawer
}

func assertGoDrawerBodyPosition(t *testing.T, frame *tui.Frame, expected string) {
	t.Helper()
	positions := make([]string, 0, 1)
	for y := uint32(0); y < frame.Surface().Height(); y++ {
		for x := uint32(0); x < frame.Surface().Width(); x++ {
			cell, ok := frame.Surface().Cell(int32(x), int32(y))
			if ok && cell.Content() == "D" {
				positions = append(positions, strconv.Itoa(int(x))+":"+strconv.Itoa(int(y)))
			}
		}
	}
	expectedPositions := []string(nil)
	if expected != "none" {
		expectedPositions = []string{expected}
	}
	if !stringSlicesEqual(positions, expectedPositions) {
		t.Fatalf("body positions = %v, want %v", positions, expectedPositions)
	}
}

func drawerEscapeEvent() vt.Event {
	return vt.Event{Kind: vt.EventKey, Key: vt.KeyEvent{
		Code: vt.KeyEscape, Action: vt.KeyPress, Protocol: vt.KeyProtocolLegacy,
	}}
}

func drawerFixtureSide(t *testing.T, value string) DrawerSide {
	t.Helper()
	switch value {
	case "left":
		return DrawerLeft
	case "right":
		return DrawerRight
	case "top":
		return DrawerTop
	case "bottom":
		return DrawerBottom
	default:
		t.Fatalf("invalid side %q", value)
		return 0
	}
}

func drawerFixtureBool(t *testing.T, value string) bool {
	t.Helper()
	if value == "true" {
		return true
	}
	if value != "false" {
		t.Fatalf("invalid bool %q", value)
	}
	return false
}

func drawerFixtureList(value string) []string {
	if value == "-" {
		return nil
	}
	return []string{value}
}
