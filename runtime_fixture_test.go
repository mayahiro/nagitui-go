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
