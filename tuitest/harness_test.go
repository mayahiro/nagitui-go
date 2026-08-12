package tuitest

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

type echoMessage struct {
	text string
}

type echoApp struct {
	text string
}

func (*echoApp) Init() tui.Effect[echoMessage] {
	return tui.NoneEffect[echoMessage]()
}

func (a *echoApp) Update(message echoMessage) tui.Effect[echoMessage] {
	a.text += message.text
	return tui.NoneEffect[echoMessage]()
}

func (*echoApp) Subscriptions() tui.Subscription[echoMessage] {
	return tui.NoneSubscription[echoMessage]()
}

func (a *echoApp) View(_ tui.ViewContext) tui.Node[echoMessage] {
	return tui.Text[echoMessage](a.text)
}

func TestHarnessObservesBytesMessagesFramesAndEscapeTime(t *testing.T) {
	app := &echoApp{}
	harness, err := New[echoMessage](app, tui.Size{Width: 4, Height: 1}, func(event vt.Event) tui.EventAction[echoMessage] {
		switch {
		case event.Kind == vt.EventText:
			return tui.MessageAction(echoMessage{text: event.Text})
		case event.Kind == vt.EventKey && event.Key.Code == vt.KeyEscape:
			return tui.ExitAction[echoMessage]()
		default:
			return tui.IgnoreAction[echoMessage]()
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := harness.Input([]byte("A日")); err != nil {
		t.Fatal(err)
	}
	if err := harness.Input([]byte{0x1B}); err != nil {
		t.Fatal(err)
	}
	if err := harness.Advance(25 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	if app.text != "A日" {
		t.Fatalf("application text = %q", app.text)
	}
	if len(harness.Frames()) != 2 {
		t.Fatalf("frame count = %d, want 2", len(harness.Frames()))
	}
	if len(harness.MessageHistory()) != 2 {
		t.Fatalf("message count = %d, want 2", len(harness.MessageHistory()))
	}
	if !harness.ExitRequested() {
		t.Fatal("Escape did not request exit")
	}
}

type clipboardApp struct{}

func (*clipboardApp) Init() tui.Effect[string] {
	return tui.NoneEffect[string]()
}

func (*clipboardApp) Update(message string) tui.Effect[string] {
	return tui.SetClipboardEffect[string](message).WithoutRedraw()
}

func (*clipboardApp) Subscriptions() tui.Subscription[string] {
	return tui.NoneSubscription[string]()
}

func (*clipboardApp) View(tui.ViewContext) tui.Node[string] {
	return tui.Text[string]("clipboard")
}

func TestHarnessObservesAndTakesPendingClipboardRequest(t *testing.T) {
	harness, err := New[string](
		&clipboardApp{},
		tui.Size{Width: 12, Height: 1},
		func(vt.Event) tui.EventAction[string] { return tui.IgnoreAction[string]() },
	)
	if err != nil {
		t.Fatal(err)
	}
	defer harness.Close()
	if err := harness.Send("copy"); err != nil {
		t.Fatal(err)
	}

	request, ok := harness.PendingClipboardRequest()
	if !ok || request.Text() != "copy" {
		t.Fatalf("pending request = %q, %t", request.Text(), ok)
	}
	request, ok = harness.TakeClipboardRequest()
	if !ok || request.Text() != "copy" {
		t.Fatalf("taken request = %q, %t", request.Text(), ok)
	}
	if _, ok := harness.TakeClipboardRequest(); ok {
		t.Fatal("second take returned a request")
	}
}

type controlledInputMessage struct {
	value     string
	selection int
}

type controlledInputApp struct {
	value     string
	selection int
}

func (*controlledInputApp) Init() tui.Effect[controlledInputMessage] {
	return tui.NoneEffect[controlledInputMessage]()
}

func (a *controlledInputApp) Update(message controlledInputMessage) tui.Effect[controlledInputMessage] {
	a.value = message.value
	a.selection = message.selection
	return tui.NoneEffect[controlledInputMessage]()
}

func (*controlledInputApp) Subscriptions() tui.Subscription[controlledInputMessage] {
	return tui.NoneSubscription[controlledInputMessage]()
}

func (a *controlledInputApp) View(tui.ViewContext) tui.Node[controlledInputMessage] {
	value := a.value
	selection := a.selection
	return tui.Text[controlledInputMessage](value).Focusable("controlled").OnEvent(
		"controlled",
		func(event vt.Event) tui.EventResult[controlledInputMessage] {
			switch {
			case event.Kind == vt.EventText:
				return tui.MessageResult(controlledInputMessage{
					value: value + event.Text, selection: selection,
				})
			case event.Kind == vt.EventKey && event.Key.Code == vt.KeyBackspace:
				next := value
				if len(next) > 0 {
					next = next[:len(next)-1]
				}
				return tui.MessageResult(controlledInputMessage{
					value: next, selection: selection,
				})
			case event.Kind == vt.EventKey && event.Key.Code == vt.KeyDown:
				return tui.MessageResult(controlledInputMessage{
					value: value, selection: selection + 1,
				})
			default:
				return tui.IgnoreResult[controlledInputMessage]()
			}
		},
	)
}

func TestHarnessProcessesControlledInputEventsSequentially(t *testing.T) {
	tests := []struct {
		name              string
		app               *controlledInputApp
		input             []byte
		expectedValue     string
		expectedSelection int
	}{
		{name: "unicode text", app: &controlledInputApp{}, input: []byte("A日"), expectedValue: "A日"},
		{name: "backspace", app: &controlledInputApp{value: "abc"}, input: []byte{0x7f, 0x7f}, expectedValue: "a"},
		{name: "down", app: &controlledInputApp{}, input: []byte("\x1b[B\x1b[B"), expectedSelection: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			harness, err := New[controlledInputMessage](
				test.app,
				tui.Size{Width: 8, Height: 1},
				func(vt.Event) tui.EventAction[controlledInputMessage] {
					return tui.IgnoreAction[controlledInputMessage]()
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			defer harness.Close()
			if focused, err := harness.RequestFocus("controlled"); err != nil || !focused {
				t.Fatalf("RequestFocus = %t, %v", focused, err)
			}
			if err := harness.Input(test.input); err != nil {
				t.Fatal(err)
			}
			if test.app.value != test.expectedValue || test.app.selection != test.expectedSelection {
				t.Fatalf(
					"state = %q, %d, want %q, %d",
					test.app.value,
					test.app.selection,
					test.expectedValue,
					test.expectedSelection,
				)
			}
			if len(harness.Frames()) != 2 {
				t.Fatalf("frame count = %d, want 2", len(harness.Frames()))
			}
		})
	}
}

type actionProjectionApp struct{}

func (*actionProjectionApp) Init() tui.Effect[struct{}] { return tui.NoneEffect[struct{}]() }
func (*actionProjectionApp) Update(struct{}) tui.Effect[struct{}] {
	return tui.NoneEffect[struct{}]()
}
func (*actionProjectionApp) Subscriptions() tui.Subscription[struct{}] {
	return tui.NoneSubscription[struct{}]()
}
func (*actionProjectionApp) View(tui.ViewContext) tui.Node[struct{}] {
	descriptor := tui.NewActionDescriptor(
		"app.action",
		"Action",
		[]tui.KeyBinding{tui.NewKeyBinding(tui.NewCharacterKeyStroke('x', vt.Modifiers{}))},
	)
	return tui.Text[struct{}]("action").Focusable("owner").OnActions(
		"owner",
		[]tui.Action[struct{}]{tui.NewAction(descriptor, func(tui.ActionEvent) tui.EventResult[struct{}] {
			return tui.ConsumeResult[struct{}]()
		})},
	)
}

func TestHarnessObservesActiveActionProjection(t *testing.T) {
	harness, err := New[struct{}](
		&actionProjectionApp{},
		tui.Size{Width: 8, Height: 1},
		func(vt.Event) tui.EventAction[struct{}] { return tui.IgnoreAction[struct{}]() },
	)
	if err != nil {
		t.Fatal(err)
	}
	defer harness.Close()
	if focused, err := harness.RequestFocus("owner"); err != nil || !focused {
		t.Fatalf("RequestFocus = %t, %v", focused, err)
	}

	groups, err := harness.ActiveActionGroups()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 || groups[0].Owner() != "owner" || groups[1].Owner() != "owner" {
		t.Fatalf("groups = %v", groups)
	}
	if actions := groups[0].Actions(); len(actions) != 1 || actions[0].ID() != "app.action" {
		t.Fatalf("actions = %v", actions)
	}
	if actions := groups[1].Actions(); len(actions) != 2 || actions[0].ID() != tui.FocusNextActionID {
		t.Fatalf("core actions = %v", actions)
	}
}

type manualApp struct {
	task   tui.Task[echoMessage]
	values []string
}

func (a *manualApp) Init() tui.Effect[echoMessage] {
	task := a.task
	a.task = nil
	return tui.RunEffect(task)
}

func (a *manualApp) Update(message echoMessage) tui.Effect[echoMessage] {
	a.values = append(a.values, message.text)
	return tui.NoneEffect[echoMessage]()
}

func (*manualApp) Subscriptions() tui.Subscription[echoMessage] {
	return tui.NoneSubscription[echoMessage]()
}

func (a *manualApp) View(_ tui.ViewContext) tui.Node[echoMessage] {
	return tui.Text[echoMessage](strings.Join(a.values, ","))
}

func TestHarnessObservesManualTasksAndSupervision(t *testing.T) {
	task, manual := NewManualTask[echoMessage]()
	app := &manualApp{task: task}
	harness, err := New[echoMessage](app, tui.Size{Width: 8, Height: 1}, func(vt.Event) tui.EventAction[echoMessage] {
		return tui.IgnoreAction[echoMessage]()
	})
	if err != nil {
		t.Fatal(err)
	}
	defer harness.Close()
	manual.WaitStarted()
	if harness.ActiveTasks() != 1 {
		t.Fatalf("active tasks = %d, want 1", harness.ActiveTasks())
	}
	if err := manual.Complete(echoMessage{text: "done"}); err != nil {
		t.Fatal(err)
	}
	for range 10_000 {
		if err := harness.Step(); err != nil {
			t.Fatal(err)
		}
		if harness.ActiveTasks() == 0 {
			break
		}
		runtime.Gosched()
	}
	if len(app.values) != 1 || app.values[0] != "done" {
		t.Fatalf("values = %v, want [done]", app.values)
	}
	if harness.ActiveTasks() != 0 {
		t.Fatalf("active tasks = %d, want 0", harness.ActiveTasks())
	}
}

type subscriptionTestMessage struct {
	value  string
	toggle bool
}

type manualSubscriptionApp struct {
	source  *ManualSubscription[subscriptionTestMessage]
	running bool
	values  []string
}

func (*manualSubscriptionApp) Init() tui.Effect[subscriptionTestMessage] {
	return tui.NoneEffect[subscriptionTestMessage]()
}

func (a *manualSubscriptionApp) Update(message subscriptionTestMessage) tui.Effect[subscriptionTestMessage] {
	if message.toggle {
		a.running = !a.running
	} else {
		a.values = append(a.values, message.value)
	}
	return tui.NoneEffect[subscriptionTestMessage]()
}

func (a *manualSubscriptionApp) Subscriptions() tui.Subscription[subscriptionTestMessage] {
	if !a.running {
		return tui.NoneSubscription[subscriptionTestMessage]()
	}
	return a.source.Subscription("manual", tui.ReliableDelivery())
}

func (a *manualSubscriptionApp) View(_ tui.ViewContext) tui.Node[subscriptionTestMessage] {
	return tui.Text[subscriptionTestMessage](strings.Join(a.values, ","))
}

func TestHarnessObservesManualSubscriptionLifecycleAndMessages(t *testing.T) {
	source := NewManualSubscription[subscriptionTestMessage]()
	app := &manualSubscriptionApp{source: source, running: true}
	harness, err := New[subscriptionTestMessage](
		app,
		tui.Size{Width: 8, Height: 1},
		func(vt.Event) tui.EventAction[subscriptionTestMessage] {
			return tui.IgnoreAction[subscriptionTestMessage]()
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer harness.Close()
	source.WaitStarted()
	if !source.Active() || harness.ActiveSubscriptions() != 1 {
		t.Fatalf("active = %t, subscriptions = %d", source.Active(), harness.ActiveSubscriptions())
	}
	if !source.Send(subscriptionTestMessage{value: "first"}) {
		t.Fatal("manual subscription send failed")
	}
	if err := harness.Step(); err != nil {
		t.Fatal(err)
	}
	if len(app.values) != 1 || app.values[0] != "first" {
		t.Fatalf("values = %v, want [first]", app.values)
	}
	if err := harness.Send(subscriptionTestMessage{toggle: true}); err != nil {
		t.Fatal(err)
	}
	source.WaitStopped()
	if source.Active() || harness.ActiveSubscriptions() != 0 {
		t.Fatalf("active = %t, subscriptions = %d", source.Active(), harness.ActiveSubscriptions())
	}
	if len(harness.MessageHistory()) != 2 {
		t.Fatalf("message history = %d, want 2", len(harness.MessageHistory()))
	}
}

type completedStreamHarnessApp struct{}

func (*completedStreamHarnessApp) Init() tui.Effect[struct{}] {
	return tui.NoneEffect[struct{}]()
}

func (*completedStreamHarnessApp) Update(struct{}) tui.Effect[struct{}] {
	return tui.NoneEffect[struct{}]()
}

func (*completedStreamHarnessApp) Subscriptions() tui.Subscription[struct{}] {
	return tui.StreamSubscription(
		"events",
		tui.ReliableDelivery(),
		func(context.Context, tui.SubscriptionSink[struct{}]) {},
	)
}

func (*completedStreamHarnessApp) View(tui.ViewContext) tui.Node[struct{}] {
	return tui.Text[struct{}]("")
}

func TestHarnessObservesRuntimeNotices(t *testing.T) {
	harness, err := New[struct{}](
		&completedStreamHarnessApp{},
		tui.Size{Width: 1, Height: 1},
		func(vt.Event) tui.EventAction[struct{}] { return tui.IgnoreAction[struct{}]() },
	)
	if err != nil {
		t.Fatal(err)
	}
	defer harness.Close()

	for range 10_000 {
		if harness.PendingRuntimeNotices() > 0 {
			break
		}
		runtime.Gosched()
	}
	notices := harness.DrainRuntimeNotices()
	if len(notices) != 1 || notices[0].Kind() != tui.RuntimeNoticeSubscriptionStreamCompleted {
		t.Fatalf("notices = %+v", notices)
	}
	if harness.RuntimeNoticeDiagnostics().Dropped() != 0 {
		t.Fatalf("dropped notices = %d", harness.RuntimeNoticeDiagnostics().Dropped())
	}
}
