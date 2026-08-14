package tui

import (
	"bytes"
	"errors"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go/internal/conformance"
)

type fixtureEcho struct {
	text string
}

type fixtureSchedulingApp struct {
	messages []string
}

func (*fixtureSchedulingApp) Init() Effect[string] { return NoneEffect[string]() }
func (a *fixtureSchedulingApp) Update(message string) Effect[string] {
	a.messages = append(a.messages, message)
	return NoneEffect[string]()
}
func (*fixtureSchedulingApp) Subscriptions() Subscription[string] {
	return NoneSubscription[string]()
}
func (a *fixtureSchedulingApp) View(ViewContext) Node[string] {
	return Text[string](strings.Join(a.messages, ","))
}

func (*fixtureEcho) Init() Effect[string] {
	return NoneEffect[string]()
}

func (a *fixtureEcho) Update(message string) Effect[string] {
	a.text += message
	return NoneEffect[string]()
}

func (*fixtureEcho) Subscriptions() Subscription[string] {
	return NoneSubscription[string]()
}

func (a *fixtureEcho) View(_ ViewContext) Node[string] {
	return Border(Text[string](a.text), vt.Style{})
}

func TestRuntimeRoundtripFixtures(t *testing.T) {
	records, err := conformance.Load(
		"runtime/roundtrip.txt",
		"runtime-roundtrip",
		"width",
		"height",
		"input",
		"expected",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			width := runtimeFixtureNumber(t, record.Field("width"))
			height := runtimeFixtureNumber(t, record.Field("height"))
			input := record.Bytes("input")
			runtime, err := NewRuntimeWithClock[string](
				&fixtureEcho{},
				NewRuntimeConfig(Size{Width: width, Height: height}),
				NewVirtualClock(),
			)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runtime.RenderIfDirty(); err != nil {
				t.Fatal(err)
			}
			decoder := NewTimedInputDecoder(NewVirtualClock(), 25*time.Millisecond)
			for _, event := range decoder.Feed(input) {
				if event.Kind == vt.EventText {
					if err := runtime.Enqueue(event.Text); err != nil {
						t.Fatal(err)
					}
				}
			}
			frame, err := runtime.Step()
			if err != nil {
				t.Fatal(err)
			}
			if actual, expected := frame.Surface().Snapshot(), record.Text("expected"); actual != expected {
				t.Fatalf("snapshot mismatch\ngot:\n%s\nwant:\n%s", actual, expected)
			}
			output := vt.Encode(frame.Operations(), vt.BaselineCapabilities())
			if !bytes.Contains(output, input) {
				t.Fatalf("input %q did not reach VT output %q", input, output)
			}
		})
	}
}

func TestRuntimeBoundedSchedulingFixtures(t *testing.T) {
	records, err := conformance.Load(
		"runtime/scheduling.txt",
		"runtime-scheduling",
		"maximum", "messages", "expected-cycle", "expected-remaining", "expected-final",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			config := NewRuntimeConfig(Size{Width: 8, Height: 1})
			config.MaxUpdatesPerCycle = int(runtimeFixtureNumber(t, record.Field("maximum")))
			app := &fixtureSchedulingApp{}
			runtime, err := NewRuntimeWithClock[string](app, config, NewVirtualClock())
			if err != nil {
				t.Fatal(err)
			}
			defer runtime.Close()
			for _, message := range runtimeFixtureList(record.Field("messages")) {
				if err := runtime.Enqueue(message); err != nil {
					t.Fatal(err)
				}
			}

			if _, err := runtime.ProcessPending(); err != nil {
				t.Fatal(err)
			}
			if expected := runtimeFixtureList(record.Field("expected-cycle")); !slices.Equal(app.messages, expected) {
				t.Fatalf("cycle = %v, want %v", app.messages, expected)
			}
			if expected := int(runtimeFixtureNumber(t, record.Field("expected-remaining"))); runtime.QueuedMessages() != expected {
				t.Fatalf("remaining = %d, want %d", runtime.QueuedMessages(), expected)
			}

			if _, err := runtime.ProcessQueued(); err != nil {
				t.Fatal(err)
			}
			if expected := runtimeFixtureList(record.Field("expected-final")); !slices.Equal(app.messages, expected) {
				t.Fatalf("final = %v, want %v", app.messages, expected)
			}
		})
	}
}

type fixtureTextInput struct {
	value string
}

func (*fixtureTextInput) Init() Effect[string] { return NoneEffect[string]() }
func (*fixtureTextInput) Subscriptions() Subscription[string] {
	return NoneSubscription[string]()
}
func (a *fixtureTextInput) Update(value string) Effect[string] {
	a.value = value
	return NoneEffect[string]()
}
func (a *fixtureTextInput) View(_ ViewContext) Node[string] {
	return TextInput("input", a.value, func(value string) string { return value })
}

func TestTextInputRuntimeFixtures(t *testing.T) {
	records, err := conformance.Load(
		"interaction/text-input-runtime.txt",
		"text-input-runtime",
		"width",
		"height",
		"input",
		"expected",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			runtime, err := NewRuntimeWithClock[string](
				&fixtureTextInput{},
				NewRuntimeConfig(Size{
					Width:  runtimeFixtureNumber(t, record.Field("width")),
					Height: runtimeFixtureNumber(t, record.Field("height")),
				}),
				NewVirtualClock(),
			)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runtime.RenderIfDirty(); err != nil {
				t.Fatal(err)
			}
			if focused, err := runtime.RequestFocus("input"); err != nil || !focused {
				t.Fatalf("RequestFocus = %t, %v", focused, err)
			}
			decoder := NewTimedInputDecoder(NewVirtualClock(), 25*time.Millisecond)
			for _, event := range decoder.Feed(record.Bytes("input")) {
				if _, err := runtime.DispatchEvent(event); err != nil {
					t.Fatal(err)
				}
			}
			frame, err := runtime.Step()
			if err != nil {
				t.Fatal(err)
			}
			if actual, expected := frame.Surface().Snapshot(), record.Text("expected"); actual != expected {
				t.Fatalf("snapshot mismatch\ngot:\n%s\nwant:\n%s", actual, expected)
			}
		})
	}
}

type fixtureModalFocusApp struct {
	view   string
	first  ModalFocusOptions
	second ModalFocusOptions
}

func (*fixtureModalFocusApp) Init() Effect[string] { return NoneEffect[string]() }
func (*fixtureModalFocusApp) Update(string) Effect[string] {
	return NoneEffect[string]()
}
func (*fixtureModalFocusApp) Subscriptions() Subscription[string] {
	return NoneSubscription[string]()
}
func (a *fixtureModalFocusApp) View(ViewContext) Node[string] {
	background := fixtureModalFocusBackground()
	switch a.view {
	case "base":
		return background
	case "a":
		return Stack(background, fixtureModalFocusNode("a", a.first, false))
	case "a-empty":
		return Stack(background, fixtureModalFocusNode("a", a.first, true))
	case "b":
		return Stack(background, fixtureModalFocusNode("b", a.second, false))
	case "a+b":
		return Stack(
			background,
			fixtureModalFocusNode("a", a.first, false),
			fixtureModalFocusNode("b", a.second, false),
		)
	case "a>b":
		return Stack(
			background,
			ModalWithFocus(
				"modal-a",
				Column(
					fixtureModalFocusContent("a"),
					fixtureModalFocusNode("b", a.second, false),
				),
				a.first,
			),
		)
	default:
		panic("unknown modal focus fixture view " + a.view)
	}
}

func TestModalFocusLifecycleFixtures(t *testing.T) {
	records, err := conformance.Load(
		"interaction/modal-focus-lifecycle.txt",
		"modal-focus-lifecycle",
		"views", "focus", "a-initial", "a-return", "b-initial", "b-return", "expected",
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
			views := runtimeFixtureList(record.Field("views"))
			expected := runtimeFixtureList(record.Field("expected"))
			if len(views) != len(expected) {
				t.Fatalf("views = %d, expected = %d", len(views), len(expected))
			}
			app := &fixtureModalFocusApp{
				view: views[0],
				first: fixtureModalFocusOptions(
					record.Field("a-initial"), record.Field("a-return"),
				),
				second: fixtureModalFocusOptions(
					record.Field("b-initial"), record.Field("b-return"),
				),
			}
			runtime, err := NewRuntimeWithClock(
				app,
				NewRuntimeConfig(Size{Width: 30, Height: 8}),
				NewVirtualClock(),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer runtime.Close()
			if _, err := runtime.RenderIfDirty(); err != nil {
				t.Fatal(err)
			}
			if focus := record.Field("focus"); focus != "none" {
				focused, err := runtime.RequestFocus(NodeID(focus))
				if err != nil || !focused {
					t.Fatalf("initial focus = %t, %v", focused, err)
				}
			}
			assertFixtureModalFocus(t, runtime.Interaction(), expected[0], 0)

			for step := 1; step < len(views); step++ {
				app.view = views[step]
				runtime.RequestFrame()
				if _, err := runtime.RenderIfDirty(); err != nil {
					t.Fatal(err)
				}
				assertFixtureModalFocus(t, runtime.Interaction(), expected[step], step)
			}
		})
	}
}

func fixtureModalFocusBackground() Node[string] {
	return Column(
		Text[string]("background first").Focusable("background-first"),
		Text[string]("opener").Focusable("opener"),
		Text[string]("background target").Focusable("background-target"),
	)
}

func fixtureModalFocusContent(prefix string) Node[string] {
	return Column(
		Text[string](prefix+" first").Focusable(NodeID(prefix+"-first")),
		Text[string](prefix+" target").Focusable(NodeID(prefix+"-target")),
	)
}

func fixtureModalFocusNode(prefix string, focus ModalFocusOptions, empty bool) Node[string] {
	child := Text[string](prefix + " empty")
	if !empty {
		child = fixtureModalFocusContent(prefix)
	}
	return ModalWithFocus(NodeID("modal-"+prefix), child, focus)
}

func fixtureModalFocusOptions(initial, returnFocus string) ModalFocusOptions {
	options := DefaultModalFocusOptions()
	switch {
	case initial == "first":
		options.Initial = ModalInitialFocusFirst()
	case initial == "none":
		options.Initial = ModalInitialFocusNone()
	case strings.HasPrefix(initial, "target/"):
		options.Initial = ModalInitialFocusTarget(NodeID(strings.TrimPrefix(initial, "target/")))
	default:
		panic("invalid modal initial focus " + initial)
	}
	switch {
	case returnFocus == "previous":
		options.ReturnFocus = ModalReturnFocusPrevious()
	case returnFocus == "none":
		options.ReturnFocus = ModalReturnFocusNone()
	case strings.HasPrefix(returnFocus, "target/"):
		options.ReturnFocus = ModalReturnFocusTarget(NodeID(strings.TrimPrefix(returnFocus, "target/")))
	default:
		panic("invalid modal return focus " + returnFocus)
	}
	return options
}

func assertFixtureModalFocus(t *testing.T, interaction *InteractionState, expected string, step int) {
	t.Helper()
	actual, hasFocus := interaction.Focused()
	if expected == "none" {
		if hasFocus {
			t.Fatalf("step %d focus = %q, want none", step, actual)
		}
		return
	}
	if !hasFocus || actual != NodeID(expected) {
		t.Fatalf("step %d focus = %q, %t, want %q", step, actual, hasFocus, expected)
	}
}

type fixtureKeyRouting struct {
	scenario string
	updates  []string
}

func (*fixtureKeyRouting) Init() Effect[string] { return NoneEffect[string]() }
func (*fixtureKeyRouting) Subscriptions() Subscription[string] {
	return NoneSubscription[string]()
}
func (a *fixtureKeyRouting) Update(message string) Effect[string] {
	a.updates = append(a.updates, message)
	return NoneEffect[string]()
}

func (a *fixtureKeyRouting) View(_ ViewContext) Node[string] {
	switch a.scenario {
	case "child-precedence":
		child := Text[string]("child").
			Focusable("child").
			OnEvent("child", func(vt.Event) EventResult[string] {
				return MessageResult("unexpected-raw")
			}).
			OnActions("child", []Action[string]{fixtureMessageAction("app.child", 'x', "child-action")})
		return Padding(child, Insets{}).
			OnEvent("root", func(vt.Event) EventResult[string] {
				return MessageResult("unexpected-root")
			}).
			OnActions("root", []Action[string]{fixtureMessageAction("app.root", 'x', "root-action")})
	case "ignored-action-routing":
		child := Text[string]("child").
			Focusable("child").
			OnEvent("child", func(vt.Event) EventResult[string] {
				return IgnoreResult[string]().Emit("child-raw")
			}).
			OnActions("child", []Action[string]{fixtureIgnoredAction("app.child", 'x', "child-action")})
		return Padding(child, Insets{}).
			OnEvent("root", func(vt.Event) EventResult[string] {
				return MessageResult("unexpected-root")
			}).
			OnActions("root", []Action[string]{fixtureMessageAction("app.root", 'x', "root-action")})
	case "stop-action-propagation":
		outerMap, err := NewKeyMap().Rebind("app.child", []KeyBinding{fixtureKeyBinding('y')})
		if err != nil {
			panic(err)
		}
		child := Text[string]("child").
			Focusable("child").
			OnActions("child", []Action[string]{fixtureMessageAction("app.child", 'x', "unexpected-child-action")})
		scope := Padding(child, Insets{}).
			WithKeyScope(NewKeyScope("scope", NewKeyMap()).WithPropagation(KeyScopeStopAtScope)).
			OnEvent("scope", func(vt.Event) EventResult[string] {
				return IgnoreResult[string]().Emit("scope-raw")
			})
		return Padding(scope, Insets{}).
			WithKeyScope(NewKeyScope("root", outerMap)).
			OnEvent("root", func(vt.Event) EventResult[string] {
				return MessageResult("root-raw")
			}).
			OnActions("root", []Action[string]{fixtureMessageAction("app.root", 'x', "unexpected-action")})
	case "scope-rebind":
		keyMap, err := NewKeyMap().Rebind("app.child", []KeyBinding{fixtureKeyBinding('y')})
		if err != nil {
			panic(err)
		}
		child := Text[string]("child").
			Focusable("child").
			OnActions("child", []Action[string]{fixtureRoutedAction("app.child", 'x', 'y', "child-action")})
		return Padding(child, Insets{}).WithKeyScope(NewKeyScope("scope", keyMap))
	case "disabled-consume":
		action := NewAction(
			fixtureActionDescriptor("app.child", 'x').WithAvailability(ActionDisabledConsume),
			func(ActionEvent) EventResult[string] { return MessageResult("unexpected-action") },
		)
		return Text[string]("child").
			Focusable("child").
			OnEvent("child", func(vt.Event) EventResult[string] {
				return MessageResult("unexpected-raw")
			}).
			OnActions("child", []Action[string]{action})
	case "paste-bypasses-actions":
		return Text[string]("child").
			Focusable("child").
			OnEvent("child", func(vt.Event) EventResult[string] {
				return MessageResult("child-raw")
			}).
			OnActions("child", []Action[string]{fixtureMessageAction("app.child", 'x', "unexpected-action")})
	case "text-input-local-action":
		return TextInput("input", "", func(string) string { return "input-change" }).
			OnActions("input", []Action[string]{fixtureMessageAction("app.input", 'x', "input-action")})
	case "text-input-core-before-ancestor":
		return Padding(
			TextInput("input", "", func(string) string { return "input-change" }),
			Insets{},
		).OnActions("root", []Action[string]{fixtureMessageAction("app.root", 'x', "unexpected-action")})
	default:
		panic("unknown key routing scenario " + a.scenario)
	}
}

func TestScopedKeyRoutingFixtures(t *testing.T) {
	records, err := conformance.Load(
		"interaction/key-routing-runtime.txt",
		"key-routing-runtime",
		"event", "expected", "consumed", "groups",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			app := &fixtureKeyRouting{scenario: record.ID}
			runtime, err := NewRuntimeWithClock[string](
				app,
				NewRuntimeConfig(Size{Width: 20, Height: 3}),
				NewVirtualClock(),
			)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(runtime.Close)
			if _, err := runtime.RenderIfDirty(); err != nil {
				t.Fatal(err)
			}
			target := NodeID("child")
			if strings.HasPrefix(record.ID, "text-input") {
				target = "input"
			}
			if focused, err := runtime.RequestFocus(target); err != nil || !focused {
				t.Fatalf("RequestFocus = %t, %v", focused, err)
			}
			groups, err := runtime.ActiveActionGroups()
			if err != nil {
				t.Fatal(err)
			}
			owners := make([]string, len(groups))
			for index := range groups {
				owners[index] = groups[index].Owner().String()
			}
			if expected := runtimeFixtureList(record.Field("groups")); !slices.Equal(owners, expected) {
				t.Fatalf("groups = %v, want %v", owners, expected)
			}
			if record.ID == "stop-action-propagation" {
				if scopes := groups[0].ScopePath(); !reflect.DeepEqual(scopes, []NodeID{"root", "scope"}) {
					t.Fatalf("scope path = %v", scopes)
				}
				bindings := groups[0].Actions()[0].Bindings()
				if !reflect.DeepEqual(bindings, []KeyBinding{fixtureKeyBinding('y')}) {
					t.Fatalf("effective bindings = %v", bindings)
				}
			}
			dispatch, err := runtime.DispatchEvent(fixtureKeyRoutingEvent(t, record.Field("event")))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runtime.ProcessPending(); err != nil {
				t.Fatal(err)
			}
			if want := record.Field("consumed") == "true"; dispatch.Consumed() != want {
				t.Fatalf("Consumed = %t, want %t", dispatch.Consumed(), want)
			}
			if expected := runtimeFixtureList(record.Field("expected")); !slices.Equal(app.updates, expected) {
				t.Fatalf("updates = %v, want %v", app.updates, expected)
			}
		})
	}
}

type fixtureCoreNavigation struct {
	scenario string
	updates  []string
}

func (*fixtureCoreNavigation) Init() Effect[string] { return NoneEffect[string]() }
func (*fixtureCoreNavigation) Subscriptions() Subscription[string] {
	return NoneSubscription[string]()
}
func (a *fixtureCoreNavigation) Update(message string) Effect[string] {
	a.updates = append(a.updates, message)
	return NoneEffect[string]()
}
func (a *fixtureCoreNavigation) View(ViewContext) Node[string] {
	if strings.HasPrefix(a.scenario, "focus-") {
		return fixtureFocusNavigationView(a.scenario)
	}
	return fixtureScrollNavigationView(a.scenario)
}

func fixtureFocusNavigationView(scenario string) Node[string] {
	first := Text[string]("a").Focusable("a")
	switch scenario {
	case "focus-unbind-raw":
		first = first.OnEvent("a", func(vt.Event) EventResult[string] {
			return MessageResult("raw")
		})
	case "focus-declared-shadow":
		first = first.OnActions("a", []Action[string]{fixtureCoreAction(vt.KeyTab, false)})
	case "focus-declared-ignore":
		first = first.OnActions("a", []Action[string]{fixtureCoreAction(vt.KeyTab, true)})
	}
	content := Column(first, Text[string]("b").Focusable("b"))
	switch scenario {
	case "focus-rebind":
		keyMap, err := NewKeyMap().Rebind(FocusNextActionID, []KeyBinding{fixtureKeyBinding('x')})
		if err != nil {
			panic(err)
		}
		return content.WithKeyScope(NewKeyScope("root", keyMap))
	case "focus-unbind-raw":
		keyMap, err := NewKeyMap().Rebind(FocusNextActionID, nil)
		if err != nil {
			panic(err)
		}
		return content.WithKeyScope(NewKeyScope("root", keyMap))
	case "focus-modal-no-focus":
		focus := DefaultModalFocusOptions()
		focus.Initial = ModalInitialFocusNone()
		return Padding(ModalWithFocus("modal", content, focus), Insets{})
	default:
		return content
	}
}

func fixtureScrollNavigationView(scenario string) Node[string] {
	if scenario == "scroll-stop-boundary" || scenario == "scroll-wheel-through-stop" {
		child := Text[string]("child").Focusable("child")
		scope := Padding(child, Insets{}).
			WithKeyScope(NewKeyScope("scope", NewKeyMap()).WithPropagation(KeyScopeStopAtScope)).
			WithLength(Fixed(2))
		content := Column(
			scope,
			Text[string]("o0\no1\no2\no3").WithLength(Fixed(4)),
		)
		return ScrollViewportWithOptions(
			"outer",
			content,
			fixtureCoreScrollOptions(ScrollAxisVertical, "outer-scroll"),
		).OnEvent("outer", func(vt.Event) EventResult[string] {
			return MessageResult("outer-raw")
		})
	}

	innerAxis := ScrollAxisVertical
	if scenario == "scroll-horizontal-pass" {
		innerAxis = ScrollAxisHorizontal
	}
	innerContent := Text[string]("i0\ni1\ni2\ni3\ni4\ni5")
	if innerAxis == ScrollAxisHorizontal {
		innerContent = Text[string]("abcdefghijklmnop")
	}
	inner := ScrollViewportWithOptions(
		"inner",
		innerContent,
		fixtureCoreScrollOptions(innerAxis, "inner-scroll"),
	).WithLength(Fixed(2))
	switch scenario {
	case "scroll-unbind-raw", "scroll-modified-raw":
		inner = inner.OnEvent("inner", func(vt.Event) EventResult[string] {
			return MessageResult("raw")
		})
	case "scroll-declared-shadow":
		inner = inner.OnActions("inner", []Action[string]{fixtureCoreAction(vt.KeyPageDown, false)})
	case "scroll-declared-ignore":
		inner = inner.OnActions("inner", []Action[string]{fixtureCoreAction(vt.KeyPageDown, true)})
	}
	content := Column(
		inner,
		Text[string]("o0\no1\no2\no3").WithLength(Fixed(4)),
	)
	outer := ScrollViewportWithOptions(
		"outer",
		content,
		fixtureCoreScrollOptions(ScrollAxisVertical, "outer-scroll"),
	)
	switch scenario {
	case "scroll-rebind":
		keyMap, err := NewKeyMap().Rebind(ScrollPageDownActionID, []KeyBinding{fixtureKeyBinding('x')})
		if err != nil {
			panic(err)
		}
		outer = outer.WithKeyScope(NewKeyScope("outer", keyMap))
	case "scroll-unbind-raw":
		keyMap, err := NewKeyMap().Rebind(ScrollPageDownActionID, nil)
		if err != nil {
			panic(err)
		}
		outer = outer.WithKeyScope(NewKeyScope("outer", keyMap))
	}
	return outer
}

func fixtureCoreScrollOptions(axis ScrollAxis, message string) ScrollViewportOptions[string] {
	return ScrollViewportOptions[string]{
		Axis: axis,
		OnScroll: func(ScrollState) string {
			return message
		},
	}
}

type fixtureRevealRuntime struct {
	lines       uint32
	viewport    uint32
	priorReveal NodeID
	hasPrior    bool
	reveal      NodeID
	hasReveal   bool
	focus       NodeID
	hasFocus    bool
	ensureFocus bool
	stickToEnd  bool
	updates     []string
}

func (*fixtureRevealRuntime) Init() Effect[string] { return NoneEffect[string]() }
func (a *fixtureRevealRuntime) Update(message string) Effect[string] {
	a.updates = append(a.updates, message)
	return NoneEffect[string]()
}
func (*fixtureRevealRuntime) Subscriptions() Subscription[string] {
	return NoneSubscription[string]()
}
func (a *fixtureRevealRuntime) View(ViewContext) Node[string] {
	rows := make([]Node[string], a.lines)
	for index := range a.lines {
		id := NodeID("row-" + strconv.FormatUint(uint64(index), 10))
		row := Text[string](strconv.FormatUint(uint64(index), 10)).WithID(id)
		if a.hasFocus && a.focus == id {
			row = row.Focusable(id)
		}
		rows[index] = row.WithLength(Fixed(1))
	}
	viewport := ScrollViewportWithOptions(
		"viewport",
		Column(rows...),
		ScrollViewportOptions[string]{
			Axis:                 ScrollAxisVertical,
			StickToEnd:           a.stickToEnd,
			EnsureFocusedVisible: a.ensureFocus,
			OnScroll:             func(ScrollState) string { return "user-scroll" },
		},
	)
	if a.hasPrior {
		viewport = viewport.RevealDescendant(a.priorReveal)
	}
	if a.hasReveal {
		viewport = viewport.RevealDescendant(a.reveal)
	}
	return Column(
		viewport.WithLength(Fixed(a.viewport)),
		Text[string]("outside").WithID("outside").WithLength(Fixed(1)),
	)
}

func TestExplicitRevealTargetFixtures(t *testing.T) {
	records, err := conformance.Load(
		"interaction/reveal-runtime.txt",
		"interaction-reveal-runtime",
		"lines",
		"viewport",
		"prior-reveal",
		"reveal",
		"focus",
		"ensure-focus",
		"stick-end",
		"expected-offset",
		"expected-top",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			viewport := runtimeFixtureNumber(t, record.Field("viewport"))
			app := &fixtureRevealRuntime{
				lines:       runtimeFixtureNumber(t, record.Field("lines")),
				viewport:    viewport,
				ensureFocus: record.Field("ensure-focus") == "true",
				stickToEnd:  record.Field("stick-end") == "true",
			}
			app.priorReveal, app.hasPrior = runtimeFixtureOptionalNodeID(record.Field("prior-reveal"))
			app.reveal, app.hasReveal = runtimeFixtureOptionalNodeID(record.Field("reveal"))
			app.focus, app.hasFocus = runtimeFixtureOptionalNodeID(record.Field("focus"))
			runtime, err := NewRuntimeWithClock[string](
				app,
				NewRuntimeConfig(Size{Width: 8, Height: viewport + 1}),
				NewVirtualClock(),
			)
			if err != nil {
				t.Fatal(err)
			}
			frame, err := runtime.RenderIfDirty()
			if err != nil {
				t.Fatal(err)
			}
			if app.hasFocus {
				if focused, err := runtime.RequestFocus(app.focus); err != nil || !focused {
					t.Fatalf("RequestFocus = %t, %v", focused, err)
				}
				frame, err = runtime.RenderIfDirty()
				if err != nil {
					t.Fatal(err)
				}
			}
			expectedOffset := runtimeFixtureNumber(t, record.Field("expected-offset"))
			if offset := runtime.Interaction().ScrollOffset("viewport"); offset.Y != expectedOffset {
				t.Fatalf("offset = %+v, want Y %d", offset, expectedOffset)
			}
			assertNodeCell(t, frame.Surface(), 0, 0, record.Field("expected-top"))
			if runtime.QueuedMessages() != 0 || len(app.updates) != 0 {
				t.Fatalf("automatic reveal emitted user-scroll message: queued=%d updates=%v", runtime.QueuedMessages(), app.updates)
			}
		})
	}
}

func runtimeFixtureOptionalNodeID(value string) (NodeID, bool) {
	if value == "-" {
		return "", false
	}
	return NodeID(value), true
}

func fixtureCoreAction(code vt.KeyCode, ignored bool) Action[string] {
	descriptor := NewActionDescriptor(
		"app.declared",
		"Declared",
		[]KeyBinding{NewKeyBinding(NewKeyStroke(code, vt.Modifiers{}))},
	)
	return NewAction(descriptor, func(ActionEvent) EventResult[string] {
		if ignored {
			return IgnoreResult[string]().Emit("declared")
		}
		return MessageResult("declared")
	})
}

func TestCoreNavigationActionsMatchSharedFixtures(t *testing.T) {
	records, err := conformance.Load(
		"interaction/core-navigation-runtime.txt",
		"core-navigation-runtime",
		"event",
		"focus",
		"expected-focus",
		"inner",
		"outer",
		"expected-inner",
		"expected-outer",
		"messages",
		"consumed",
		"groups",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			app := &fixtureCoreNavigation{scenario: record.ID}
			runtime, err := NewRuntimeWithClock[string](
				app,
				NewRuntimeConfig(Size{Width: 8, Height: 3}),
				NewVirtualClock(),
			)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(runtime.Close)
			if _, err := runtime.RenderIfDirty(); err != nil {
				t.Fatal(err)
			}
			if focus := record.Field("focus"); focus != "none" {
				if focused, err := runtime.RequestFocus(NodeID(focus)); err != nil || !focused {
					t.Fatalf("RequestFocus = %t, %v", focused, err)
				}
			}
			for _, field := range []string{"inner", "outer"} {
				if offset, ok := fixtureOptionalRuntimeNumber(t, record.Field(field)); ok {
					if !runtime.SetScrollOffset(NodeID(field), ScrollOffset{Y: offset}) {
						t.Fatalf("SetScrollOffset(%s) failed", field)
					}
				}
			}
			if _, err := runtime.RenderIfDirty(); err != nil {
				t.Fatal(err)
			}

			groups, err := runtime.ActiveActionGroups()
			if err != nil {
				t.Fatal(err)
			}
			owners := make([]string, len(groups))
			for index := range groups {
				owners[index] = groups[index].Owner().String()
			}
			if expected := runtimeFixtureList(record.Field("groups")); !slices.Equal(owners, expected) {
				t.Fatalf("groups = %v, want %v", owners, expected)
			}

			dispatch, err := runtime.DispatchEvent(fixtureCoreNavigationEvent(t, record.Field("event")))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runtime.ProcessPending(); err != nil {
				t.Fatal(err)
			}
			if want := record.Field("consumed") == "true"; dispatch.Consumed() != want {
				t.Fatalf("Consumed = %t, want %t", dispatch.Consumed(), want)
			}
			focused, hasFocus := runtime.Interaction().Focused()
			actualFocus := "none"
			if hasFocus {
				actualFocus = focused.String()
			}
			if actualFocus != record.Field("expected-focus") {
				t.Fatalf("focus = %s, want %s", actualFocus, record.Field("expected-focus"))
			}
			for _, field := range []string{"inner", "outer"} {
				expectedField := "expected-" + field
				if expected, ok := fixtureOptionalRuntimeNumber(t, record.Field(expectedField)); ok {
					if actual := runtime.Interaction().ScrollOffset(NodeID(field)).Y; actual != expected {
						t.Fatalf("%s offset = %d, want %d", field, actual, expected)
					}
				}
			}
			if expected := runtimeFixtureList(record.Field("messages")); !slices.Equal(app.updates, expected) {
				t.Fatalf("updates = %v, want %v", app.updates, expected)
			}
		})
	}
}

func fixtureCoreNavigationEvent(t *testing.T, value string) vt.Event {
	t.Helper()
	switch value {
	case "tab":
		return fixtureNamedKeyEvent(vt.KeyTab, vt.Modifiers{})
	case "shift-tab":
		return fixtureNamedKeyEvent(vt.KeyTab, vt.Modifiers{Shift: true})
	case "repeat-tab":
		return fixtureKeyEventWithAction(vt.KeyTab, vt.Modifiers{}, vt.KeyRepeat)
	case "release-tab":
		return fixtureKeyEventWithAction(vt.KeyTab, vt.Modifiers{}, vt.KeyRelease)
	case "ctrl-tab":
		return fixtureNamedKeyEvent(vt.KeyTab, vt.Modifiers{Control: true})
	case "page-up":
		return fixtureNamedKeyEvent(vt.KeyPageUp, vt.Modifiers{})
	case "page-down":
		return fixtureNamedKeyEvent(vt.KeyPageDown, vt.Modifiers{})
	case "repeat-page-down":
		return fixtureKeyEventWithAction(vt.KeyPageDown, vt.Modifiers{}, vt.KeyRepeat)
	case "unknown-page-down":
		return fixtureKeyEventWithAction(vt.KeyPageDown, vt.Modifiers{}, vt.KeyActionUnknown)
	case "release-page-down":
		return fixtureKeyEventWithAction(vt.KeyPageDown, vt.Modifiers{}, vt.KeyRelease)
	case "home":
		return fixtureNamedKeyEvent(vt.KeyHome, vt.Modifiers{})
	case "end":
		return fixtureNamedKeyEvent(vt.KeyEnd, vt.Modifiers{})
	case "ctrl-page-down":
		return fixtureNamedKeyEvent(vt.KeyPageDown, vt.Modifiers{Control: true})
	case "key/x":
		return vt.Event{Kind: vt.EventKey, Key: vt.KeyEvent{
			Code: vt.KeyCharacter, Character: 'x', Action: vt.KeyPress,
			Text: "x", HasText: true, Protocol: vt.KeyProtocolLegacy,
		}}
	case "wheel-down":
		return vt.Event{Kind: vt.EventMouse, Mouse: vt.MouseEvent{
			Kind: vt.MouseScroll, Button: vt.MouseWheelDown,
		}}
	default:
		t.Fatalf("unknown core navigation event %q", value)
		return vt.Event{}
	}
}

func fixtureNamedKeyEvent(code vt.KeyCode, modifiers vt.Modifiers) vt.Event {
	return fixtureKeyEventWithAction(code, modifiers, vt.KeyPress)
}

func fixtureKeyEventWithAction(code vt.KeyCode, modifiers vt.Modifiers, action vt.KeyAction) vt.Event {
	return vt.Event{Kind: vt.EventKey, Key: vt.KeyEvent{
		Code: code, Modifiers: modifiers, Action: action, Protocol: vt.KeyProtocolLegacy,
	}}
}

func fixtureOptionalRuntimeNumber(t *testing.T, value string) (uint32, bool) {
	t.Helper()
	if value == "-" {
		return 0, false
	}
	return runtimeFixtureNumber(t, value), true
}

type fixtureConflictingCoreActions struct{}

func (*fixtureConflictingCoreActions) Init() Effect[struct{}] { return NoneEffect[struct{}]() }
func (*fixtureConflictingCoreActions) Update(struct{}) Effect[struct{}] {
	return NoneEffect[struct{}]()
}
func (*fixtureConflictingCoreActions) Subscriptions() Subscription[struct{}] {
	return NoneSubscription[struct{}]()
}
func (*fixtureConflictingCoreActions) View(ViewContext) Node[struct{}] {
	keyMap, err := NewKeyMap().
		Rebind(FocusNextActionID, []KeyBinding{fixtureKeyBinding('x')})
	if err != nil {
		panic(err)
	}
	keyMap, err = keyMap.Rebind(ScrollPageDownActionID, []KeyBinding{fixtureKeyBinding('x')})
	if err != nil {
		panic(err)
	}
	return ScrollViewport("viewport", Text[struct{}]("a\nb\nc")).
		WithKeyScope(NewKeyScope("viewport", keyMap))
}

func TestRuntimeRejectsSameOwnerCoreActionConflicts(t *testing.T) {
	runtime, err := NewRuntimeWithClock[struct{}](
		&fixtureConflictingCoreActions{},
		NewRuntimeConfig(Size{Width: 8, Height: 1}),
		NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	frame, err := runtime.RenderIfDirty()
	if frame != nil {
		t.Fatal("conflicting frame was published")
	}
	var conflict *BindingConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("RenderIfDirty error = %v, want BindingConflictError", err)
	}
	if conflict.Kind() != ConflictAmbiguousBinding || conflict.Owner() != "viewport" {
		t.Fatalf("conflict = kind %d owner %s", conflict.Kind(), conflict.Owner())
	}
	if actions := conflict.Actions(); !reflect.DeepEqual(
		actions,
		[]ActionID{FocusNextActionID, ScrollPageDownActionID},
	) {
		t.Fatalf("conflict actions = %v", actions)
	}
}

type fixtureFutureFocusConflict struct{}

func (*fixtureFutureFocusConflict) Init() Effect[struct{}] { return NoneEffect[struct{}]() }
func (*fixtureFutureFocusConflict) Update(struct{}) Effect[struct{}] {
	return NoneEffect[struct{}]()
}
func (*fixtureFutureFocusConflict) Subscriptions() Subscription[struct{}] {
	return NoneSubscription[struct{}]()
}
func (*fixtureFutureFocusConflict) View(ViewContext) Node[struct{}] {
	keyMap, err := NewKeyMap().
		Rebind(FocusNextActionID, []KeyBinding{fixtureKeyBinding('x')})
	if err != nil {
		panic(err)
	}
	keyMap, err = keyMap.Rebind(FocusPreviousActionID, []KeyBinding{fixtureKeyBinding('x')})
	if err != nil {
		panic(err)
	}
	return Column(
		Text[struct{}]("a").Focusable("a"),
		Text[struct{}]("b").
			Focusable("b").
			WithKeyScope(NewKeyScope("b", keyMap)).
			OnEvent("b", func(vt.Event) EventResult[struct{}] {
				return MessageResult(struct{}{})
			}),
	)
}

func TestNewlyFocusedCoreRouteIsResolvedBeforeNextHandler(t *testing.T) {
	runtime, err := NewRuntimeWithClock[struct{}](
		&fixtureFutureFocusConflict{},
		NewRuntimeConfig(Size{Width: 8, Height: 2}),
		NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if focused, err := runtime.RequestFocus("a"); err != nil || !focused {
		t.Fatalf("RequestFocus = %t, %v", focused, err)
	}
	if _, err := runtime.DispatchEvent(fixtureNamedKeyEvent(vt.KeyTab, vt.Modifiers{})); err != nil {
		t.Fatal(err)
	}
	if focused, ok := runtime.Interaction().Focused(); !ok || focused != "b" {
		t.Fatalf("focus = %s, %t", focused, ok)
	}

	_, err = runtime.DispatchEvent(vt.Event{Kind: vt.EventKey, Key: vt.KeyEvent{
		Code: vt.KeyCharacter, Character: 'z', Action: vt.KeyPress,
	}})
	var conflict *BindingConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("DispatchEvent error = %v, want BindingConflictError", err)
	}
	if conflict.Owner() != "b" {
		t.Fatalf("conflict owner = %s", conflict.Owner())
	}
	if runtime.QueuedMessages() != 0 {
		t.Fatalf("queued messages = %d, want 0", runtime.QueuedMessages())
	}
}

type fixtureConflictingActions struct{}

func (*fixtureConflictingActions) Init() Effect[struct{}] { return NoneEffect[struct{}]() }
func (*fixtureConflictingActions) Update(struct{}) Effect[struct{}] {
	return NoneEffect[struct{}]()
}
func (*fixtureConflictingActions) Subscriptions() Subscription[struct{}] {
	return NoneSubscription[struct{}]()
}
func (*fixtureConflictingActions) View(ViewContext) Node[struct{}] {
	return Text[struct{}]("conflict").OnActions("owner", []Action[struct{}]{
		NewAction(fixtureActionDescriptor("app.first", 'x'), func(ActionEvent) EventResult[struct{}] {
			return ConsumeResult[struct{}]()
		}),
		NewAction(fixtureActionDescriptor("app.second", 'x'), func(ActionEvent) EventResult[struct{}] {
			return ConsumeResult[struct{}]()
		}),
	})
}

func TestRuntimeRejectsActionConflictsBeforePublishingFrame(t *testing.T) {
	runtime, err := NewRuntimeWithClock[struct{}](
		&fixtureConflictingActions{},
		NewRuntimeConfig(Size{Width: 20, Height: 1}),
		NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	frame, err := runtime.RenderIfDirty()
	if frame != nil {
		t.Fatal("conflicting frame was published")
	}
	var conflict *BindingConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("RenderIfDirty error = %v, want BindingConflictError", err)
	}
	if conflict.Kind() != ConflictAmbiguousBinding || conflict.Owner() != "owner" {
		t.Fatalf("conflict = kind %d owner %s", conflict.Kind(), conflict.Owner())
	}
	if actions := conflict.Actions(); !reflect.DeepEqual(actions, []ActionID{"app.first", "app.second"}) {
		t.Fatalf("conflict actions = %v", actions)
	}
}

type fixtureRouteConflictingActions struct{}

func (*fixtureRouteConflictingActions) Init() Effect[struct{}] { return NoneEffect[struct{}]() }
func (*fixtureRouteConflictingActions) Update(struct{}) Effect[struct{}] {
	return NoneEffect[struct{}]()
}
func (*fixtureRouteConflictingActions) Subscriptions() Subscription[struct{}] {
	return NoneSubscription[struct{}]()
}
func (*fixtureRouteConflictingActions) View(ViewContext) Node[struct{}] {
	keyMap, err := NewKeyMap().Rebind("app.second", []KeyBinding{fixtureKeyBinding('x')})
	if err != nil {
		panic(err)
	}
	child := Text[struct{}]("child").
		Focusable("child").
		WithKeyScope(NewKeyScope("child", keyMap)).
		OnEvent("child", func(vt.Event) EventResult[struct{}] { return MessageResult(struct{}{}) })
	return Padding(child, Insets{}).
		OnEvent("root", func(vt.Event) EventResult[struct{}] { return MessageResult(struct{}{}) }).
		OnActions("root", []Action[struct{}]{
			NewAction(fixtureActionDescriptor("app.first", 'x'), func(ActionEvent) EventResult[struct{}] {
				return ConsumeResult[struct{}]()
			}),
			NewAction(fixtureActionDescriptor("app.second", 'y'), func(ActionEvent) EventResult[struct{}] {
				return ConsumeResult[struct{}]()
			}),
		})
}

func TestRuntimeRejectsRouteSpecificConflictBeforeAnyHandler(t *testing.T) {
	runtime, err := NewRuntimeWithClock[struct{}](
		&fixtureRouteConflictingActions{},
		NewRuntimeConfig(Size{Width: 20, Height: 1}),
		NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if focused, err := runtime.RequestFocus("child"); err != nil || !focused {
		t.Fatalf("RequestFocus = %t, %v", focused, err)
	}

	_, err = runtime.DispatchEvent(fixtureKeyRoutingEvent(t, "key/z"))
	var conflict *BindingConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("DispatchEvent error = %v, want BindingConflictError", err)
	}
	if conflict.Owner() != "root" || !reflect.DeepEqual(conflict.ScopePath(), []NodeID{"child"}) {
		t.Fatalf("conflict owner = %s, scopes = %v", conflict.Owner(), conflict.ScopePath())
	}
	if runtime.QueuedMessages() != 0 {
		t.Fatalf("queued messages = %d, want 0", runtime.QueuedMessages())
	}
}

func fixtureActionDescriptor(id ActionID, character rune) ActionDescriptor {
	return NewActionDescriptor(id, id.String(), []KeyBinding{fixtureKeyBinding(character)})
}

func fixtureKeyBinding(character rune) KeyBinding {
	return NewKeyBinding(NewCharacterKeyStroke(character, vt.Modifiers{}))
}

func fixtureMessageAction(id ActionID, character rune, message string) Action[string] {
	return fixtureRoutedAction(id, character, character, message)
}

func fixtureRoutedAction(id ActionID, defaultCharacter, eventCharacter rune, message string) Action[string] {
	return NewAction(fixtureActionDescriptor(id, defaultCharacter), func(event ActionEvent) EventResult[string] {
		if event.Action() != id || event.Stroke() != NewCharacterKeyStroke(eventCharacter, vt.Modifiers{}) {
			panic("action event does not match resolved action")
		}
		return MessageResult(message)
	})
}

func fixtureIgnoredAction(id ActionID, character rune, message string) Action[string] {
	return NewAction(fixtureActionDescriptor(id, character), func(event ActionEvent) EventResult[string] {
		if event.Action() != id || event.Stroke() != NewCharacterKeyStroke(character, vt.Modifiers{}) {
			panic("action event does not match resolved action")
		}
		return IgnoreResult[string]().Emit(message)
	})
}

func fixtureKeyRoutingEvent(t *testing.T, value string) vt.Event {
	t.Helper()
	parts := strings.SplitN(value, "/", 2)
	if len(parts) != 2 {
		t.Fatalf("invalid fixture event %q", value)
	}
	character, _ := utf8.DecodeRuneInString(parts[1])
	switch parts[0] {
	case "key":
		return vt.Event{Kind: vt.EventKey, Key: vt.KeyEvent{
			Code: vt.KeyCharacter, Character: character, Action: vt.KeyPress,
			Text: string(character), HasText: true, Protocol: vt.KeyProtocolLegacy,
		}}
	case "text":
		return vt.Event{Kind: vt.EventText, Text: string(character)}
	case "paste":
		return vt.Event{Kind: vt.EventPaste, Text: string(character)}
	default:
		t.Fatalf("unknown fixture event kind %q", parts[0])
		return vt.Event{}
	}
}

func runtimeFixtureList(value string) []string {
	if value == "-" {
		return []string{}
	}
	return strings.Split(value, ",")
}

func runtimeFixtureNumber(t *testing.T, value string) uint32 {
	t.Helper()
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		t.Fatalf("parse %q: %v", value, err)
	}
	return uint32(parsed)
}
