package widget

import (
	"strconv"
	"strings"
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

type disclosureActionApp struct {
	expanded   bool
	enabled    bool
	body       bool
	keyMap     tui.KeyMap
	outer      bool
	bodyBuilds int
	messages   []string
}

func (*disclosureActionApp) Init() tui.Effect[string] { return tui.NoneEffect[string]() }
func (*disclosureActionApp) Subscriptions() tui.Subscription[string] {
	return tui.NoneSubscription[string]()
}
func (a *disclosureActionApp) Update(message string) tui.Effect[string] {
	if message == "true" || message == "false" {
		a.expanded = message == "true"
	}
	a.messages = append(a.messages, message)
	return tui.NoneEffect[string]()
}
func (a *disclosureActionApp) View(tui.ViewContext) tui.Node[string] {
	disclosure := NewDisclosure(
		tui.NewNodeID("disclosure"),
		tui.Text[string]("Summary"),
		a.expanded,
		func(expanded bool) string { return strconv.FormatBool(expanded) },
	).Enabled(a.enabled)
	if a.body {
		disclosure = disclosure.Body(func() tui.Node[string] {
			a.bodyBuilds++
			return tui.Text[string]("Details").Focusable(tui.NewNodeID("body"))
		})
	}
	scoped := tui.Padding(disclosure.Node(), tui.UniformInsets(0)).
		WithKeyScope(tui.NewKeyScope(tui.NewNodeID("scope"), a.keyMap))
	if a.outer {
		return tui.Padding(scoped, tui.UniformInsets(0)).OnEvent(
			tui.NewNodeID("root"),
			func(vt.Event) tui.EventResult[string] { return tui.MessageResult("raw") },
		)
	}
	return scoped
}

func TestDisclosureActionsMatchSharedFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t,
		"widgets/disclosure.txt",
		"widget-disclosure",
		"expanded", "enabled", "body", "event", "mode", "outer", "message", "consumed",
		"focus", "expected-expanded", "builds", "marker", "availability",
	)
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			expanded := fixtureBool(t, record.Field("expanded"))
			enabled := fixtureBool(t, record.Field("enabled"))
			descriptors := NewDisclosure(
				tui.NewNodeID("disclosure"),
				tui.Text[string]("Summary"),
				expanded,
				func(next bool) string { return strconv.FormatBool(next) },
			).Enabled(enabled).ActionDescriptors()
			expectedIDs := []tui.ActionID{ActivateActionID, CollapseActionID, ExpandActionID}
			expectedAvailability := disclosureFixtureStrings(record.Field("availability"))
			for index, descriptor := range descriptors {
				if descriptor.ID() != expectedIDs[index] {
					t.Fatalf("action %d ID = %q, want %q", index, descriptor.ID(), expectedIDs[index])
				}
				if actual := disclosureFixtureAvailability(descriptor.Availability()); actual != expectedAvailability[index] {
					t.Fatalf("action %d availability = %q, want %q", index, actual, expectedAvailability[index])
				}
			}

			app := &disclosureActionApp{
				expanded: expanded,
				enabled:  enabled,
				body:     fixtureBool(t, record.Field("body")),
				keyMap:   disclosureFixtureKeyMap(t, record.Field("mode")),
				outer:    record.Field("outer") == "raw",
			}
			runtime, err := tui.NewRuntimeWithClock(
				app,
				tui.NewRuntimeConfig(tui.Size{Width: 20, Height: 3}),
				tui.NewVirtualClock(),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer runtime.Close()
			frame, err := runtime.RenderIfDirty()
			if err != nil {
				t.Fatal(err)
			}
			var dispatch *tui.EventDispatch

			switch event := record.Field("event"); event {
			case "none":
			case "external-collapse":
				focused, err := runtime.RequestFocus(tui.NewNodeID("body"))
				if err != nil || !focused {
					t.Fatalf("body focus = %t, %v", focused, err)
				}
				app.expanded = false
				runtime.RequestFrame()
				frame, err = runtime.RenderIfDirty()
				if err != nil {
					t.Fatal(err)
				}
			default:
				if event != "pointer" && enabled {
					focused, err := runtime.RequestFocus(tui.NewNodeID("disclosure"))
					if err != nil || !focused {
						t.Fatalf("disclosure focus = %t, %v", focused, err)
					}
				}
				eventDispatch, err := runtime.DispatchEvent(disclosureFixtureEvent(event))
				if err != nil {
					t.Fatal(err)
				}
				dispatch = &eventDispatch
				if _, err := runtime.ProcessPending(); err != nil {
					t.Fatal(err)
				}
				if rendered, err := runtime.RenderIfDirty(); err != nil {
					t.Fatal(err)
				} else if rendered != nil {
					frame = rendered
				}
			}

			expectedMessages := disclosureFixtureStrings(record.Field("message"))
			if strings.Join(app.messages, ",") != strings.Join(expectedMessages, ",") {
				t.Errorf("messages = %#v, want %#v", app.messages, expectedMessages)
			}
			disclosureAssertDispatch(
				t, dispatch, len(expectedMessages), fixtureBool(t, record.Field("consumed")),
			)
			if app.expanded != fixtureBool(t, record.Field("expected-expanded")) {
				t.Errorf("expanded = %t", app.expanded)
			}
			expectedBuilds, err := strconv.Atoi(record.Field("builds"))
			if err != nil {
				t.Fatal(err)
			}
			if app.bodyBuilds != expectedBuilds {
				t.Errorf("body builds = %d, want %d", app.bodyBuilds, expectedBuilds)
			}
			marker, ok := frame.Surface().Cell(0, 0)
			if !ok || marker.Content() != record.Field("marker") {
				t.Errorf("marker = %q, %t, want %q", marker.Content(), ok, record.Field("marker"))
			}
			focus, hasFocus := runtime.Interaction().Focused()
			expectedFocus := record.Field("focus")
			if expectedFocus == "none" {
				if hasFocus {
					t.Errorf("focus = %q, want none", focus)
				}
			} else if !hasFocus || focus.String() != expectedFocus {
				t.Errorf("focus = %q, %t, want %q", focus, hasFocus, expectedFocus)
			}
		})
	}
}

type nestedDisclosureApp struct {
	outer bool
	inner bool
}

func (*nestedDisclosureApp) Init() tui.Effect[[2]bool] { return tui.NoneEffect[[2]bool]() }
func (*nestedDisclosureApp) Subscriptions() tui.Subscription[[2]bool] {
	return tui.NoneSubscription[[2]bool]()
}
func (a *nestedDisclosureApp) Update(state [2]bool) tui.Effect[[2]bool] {
	a.outer, a.inner = state[0], state[1]
	return tui.NoneEffect[[2]bool]()
}
func (a *nestedDisclosureApp) View(tui.ViewContext) tui.Node[[2]bool] {
	outer, inner := a.outer, a.inner
	return NewDisclosure(
		tui.NewNodeID("outer"),
		tui.Text[[2]bool]("Outer"),
		outer,
		func(expanded bool) [2]bool { return [2]bool{expanded, inner} },
	).Body(func() tui.Node[[2]bool] {
		return NewDisclosure(
			tui.NewNodeID("inner"),
			tui.Text[[2]bool]("Inner"),
			inner,
			func(expanded bool) [2]bool { return [2]bool{outer, expanded} },
		).Body(func() tui.Node[[2]bool] {
			return tui.Text[[2]bool]("Nested details").Focusable(tui.NewNodeID("inner-body"))
		}).Node()
	}).Node()
}

func TestNestedDisclosureUsesNearestFocusFallback(t *testing.T) {
	app := &nestedDisclosureApp{outer: true, inner: true}
	runtime, err := tui.NewRuntimeWithClock(
		app,
		tui.NewRuntimeConfig(tui.Size{Width: 20, Height: 4}),
		tui.NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if focused, err := runtime.RequestFocus(tui.NewNodeID("inner-body")); err != nil || !focused {
		t.Fatalf("inner body focus = %t, %v", focused, err)
	}

	app.inner = false
	runtime.RequestFrame()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if focus, ok := runtime.Interaction().Focused(); !ok || focus != tui.NewNodeID("inner") {
		t.Fatalf("inner fallback focus = %q, %t", focus, ok)
	}

	app.outer = false
	runtime.RequestFrame()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if focus, ok := runtime.Interaction().Focused(); !ok || focus != tui.NewNodeID("outer") {
		t.Fatalf("outer fallback focus = %q, %t", focus, ok)
	}
}

func disclosureFixtureKeyMap(t *testing.T, mode string) tui.KeyMap {
	t.Helper()
	switch mode {
	case "default":
		return tui.NewKeyMap()
	case "rebind":
		keyMap, err := tui.NewKeyMap().Rebind(
			ActivateActionID,
			[]tui.KeyBinding{tui.NewKeyBinding(tui.NewCharacterKeyStroke('x', vt.Modifiers{}))},
		)
		if err != nil {
			t.Fatal(err)
		}
		return keyMap
	case "unbind":
		keyMap, err := tui.NewKeyMap().Rebind(ActivateActionID, nil)
		if err != nil {
			t.Fatal(err)
		}
		return keyMap
	default:
		t.Fatalf("unknown Disclosure key mode %q", mode)
		return tui.NewKeyMap()
	}
}

func disclosureFixtureEvent(value string) vt.Event {
	key := func(code vt.KeyCode, character rune) vt.Event {
		return vt.Event{Kind: vt.EventKey, Key: vt.KeyEvent{
			Code: code, Character: character, Action: vt.KeyPress, Protocol: vt.KeyProtocolLegacy,
		}}
	}
	switch value {
	case "enter":
		return key(vt.KeyEnter, 0)
	case "space":
		return key(vt.KeyCharacter, ' ')
	case "left":
		return key(vt.KeyLeft, 0)
	case "right":
		return key(vt.KeyRight, 0)
	case "x":
		return key(vt.KeyCharacter, 'x')
	case "pointer":
		return vt.Event{Kind: vt.EventMouse, Mouse: vt.MouseEvent{
			Kind: vt.MousePress, Button: vt.MouseLeft, X: 0, Y: 0,
		}}
	default:
		panic("unknown Disclosure event " + value)
	}
}

func disclosureAssertDispatch(t *testing.T, dispatch *tui.EventDispatch, messages int, consumed bool) {
	t.Helper()
	if dispatch == nil {
		if messages != 0 || consumed {
			t.Fatalf("missing dispatch with messages = %d, consumed = %t", messages, consumed)
		}
		return
	}
	if dispatch.Messages() != messages || dispatch.Consumed() != consumed {
		t.Fatalf("dispatch messages = %d, consumed = %t", dispatch.Messages(), dispatch.Consumed())
	}
}

func disclosureFixtureAvailability(value tui.ActionAvailability) string {
	switch value {
	case tui.ActionEnabled:
		return "enabled"
	case tui.ActionDisabledPassThrough:
		return "pass"
	case tui.ActionDisabledConsume:
		return "consume"
	default:
		panic("invalid action availability")
	}
}

func disclosureFixtureStrings(value string) []string {
	if value == "-" {
		return nil
	}
	return strings.Split(value, ",")
}
