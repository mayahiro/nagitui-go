package tui

import (
	"context"
	"errors"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go/surface"
)

type runtimeMessage struct {
	add               uint32
	withoutRedraw     bool
	exitWithoutRedraw bool
}

type counterApp struct {
	value   uint32
	updates []uint32
}

func (a *counterApp) Init() Effect[runtimeMessage] {
	a.value = 1
	return NoneEffect[runtimeMessage]()
}

func (a *counterApp) Update(message runtimeMessage) Effect[runtimeMessage] {
	if message.exitWithoutRedraw {
		return ExitEffect[runtimeMessage]().WithoutRedraw()
	}
	if message.withoutRedraw {
		a.updates = append(a.updates, message.add)
		return NoneEffect[runtimeMessage]().WithoutRedraw()
	}
	a.value += message.add
	a.updates = append(a.updates, message.add)
	return NoneEffect[runtimeMessage]()
}

func TestEffectWithoutRedrawSkipsUnchangedFrame(t *testing.T) {
	runtime, err := NewRuntime[runtimeMessage](&counterApp{}, Size{Width: 3, Height: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Enqueue(runtimeMessage{withoutRedraw: true}); err != nil {
		t.Fatal(err)
	}
	if processed, err := runtime.ProcessPending(); err != nil || processed != 1 {
		t.Fatalf("ProcessPending = %d, %v", processed, err)
	}
	if frame, err := runtime.RenderIfDirty(); err != nil || frame != nil {
		t.Fatalf("RenderIfDirty = %v, %v, want nil frame", frame, err)
	}
}

func TestEffectWithoutRedrawDoesNotSuppressSynchronousCommandFrame(t *testing.T) {
	runtime, err := NewRuntime[runtimeMessage](&counterApp{}, Size{Width: 3, Height: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Enqueue(runtimeMessage{exitWithoutRedraw: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ProcessPending(); err != nil {
		t.Fatal(err)
	}
	if frame, err := runtime.RenderIfDirty(); err != nil || frame == nil {
		t.Fatalf("RenderIfDirty = %v, %v, want command frame", frame, err)
	}
}

func TestTerminalRenderingReusesReleasedSurfaceStorage(t *testing.T) {
	runtime, err := NewRuntime[runtimeMessage](&counterApp{}, Size{Width: 3, Height: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	if _, err := runtime.terminalOperationsIfDirty(); err != nil {
		t.Fatal(err)
	}
	first := runtime.previousSurface
	runtime.RequestFrame()
	if _, err := runtime.terminalOperationsIfDirty(); err != nil {
		t.Fatal(err)
	}
	runtime.RequestFrame()
	if _, err := runtime.terminalOperationsIfDirty(); err != nil {
		t.Fatal(err)
	}
	if runtime.previousSurface != first {
		t.Fatal("terminal rendering did not reuse the released surface")
	}
}

func (*counterApp) Subscriptions() Subscription[runtimeMessage] {
	return NoneSubscription[runtimeMessage]()
}

func (a *counterApp) View(_ ViewContext) Node[runtimeMessage] {
	return Text[runtimeMessage](string(rune('0' + a.value)))
}

func TestRuntimeProcessesFIFOAndCoalescesRendering(t *testing.T) {
	clock := NewVirtualClock()
	app := &counterApp{}
	runtime, err := NewRuntimeWithClock[runtimeMessage](
		app,
		NewRuntimeConfig(Size{Width: 3, Height: 1}),
		clock,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Enqueue(runtimeMessage{add: 2}); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Enqueue(runtimeMessage{add: 3}); err != nil {
		t.Fatal(err)
	}
	clock.Advance(7 * time.Millisecond)

	frame, err := runtime.Step()
	if err != nil {
		t.Fatal(err)
	}

	if len(app.updates) != 2 || app.updates[0] != 2 || app.updates[1] != 3 {
		t.Fatalf("update order = %v, want [2 3]", app.updates)
	}
	assertNodeCell(t, frame.Surface(), 0, 0, "6")
	if frame.Timestamp().Nanoseconds() != 7_000_000 {
		t.Fatalf("frame timestamp = %d", frame.Timestamp().Nanoseconds())
	}
	again, err := runtime.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	if again != nil {
		t.Fatal("unchanged runtime rendered another frame")
	}
}

type dispatchAllocationApp struct {
	withAction bool
}

func (*dispatchAllocationApp) Init() Effect[struct{}] { return NoneEffect[struct{}]() }
func (*dispatchAllocationApp) Update(struct{}) Effect[struct{}] {
	return NoneEffect[struct{}]()
}
func (*dispatchAllocationApp) Subscriptions() Subscription[struct{}] {
	return NoneSubscription[struct{}]()
}
func (a *dispatchAllocationApp) View(ViewContext) Node[struct{}] {
	node := Text[struct{}]("target").Focusable("target")
	if !a.withAction {
		return node
	}
	descriptor := NewActionDescriptor(
		"app.action",
		"Action",
		[]KeyBinding{NewKeyBinding(NewCharacterKeyStroke('x', vt.Modifiers{}))},
	)
	return node.OnActions("target", []Action[struct{}]{
		NewAction(descriptor, func(ActionEvent) EventResult[struct{}] {
			return IgnoreResult[struct{}]()
		}),
	})
}

func TestDispatchRouteCacheHasNoPerEventActionResolutionAllocation(t *testing.T) {
	event := vt.Event{Kind: vt.EventKey, Key: vt.KeyEvent{
		Code: vt.KeyCharacter, Character: 'x', Action: vt.KeyPress,
	}}
	for _, withAction := range []bool{false, true} {
		name := "cached-core-action"
		if withAction {
			name = "cached-action"
		}
		t.Run(name, func(t *testing.T) {
			runtime, err := NewRuntime[struct{}](&dispatchAllocationApp{withAction: withAction}, Size{Width: 8, Height: 1})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(runtime.Close)
			if _, err := runtime.RenderIfDirty(); err != nil {
				t.Fatal(err)
			}
			if focused, err := runtime.RequestFocus("target"); err != nil || !focused {
				t.Fatalf("RequestFocus = %t, %v", focused, err)
			}
			if _, err := runtime.ActiveActionGroups(); err != nil {
				t.Fatal(err)
			}
			var dispatchErr error
			allocations := testing.AllocsPerRun(1_000, func() {
				_, dispatchErr = runtime.DispatchEvent(event)
			})
			if dispatchErr != nil {
				t.Fatal(dispatchErr)
			}
			if allocations > 1 {
				t.Fatalf("DispatchEvent allocations = %f, want at most route storage", allocations)
			}
		})
	}
}

func TestRuntimeFrameRateAndUrgentCoalescingMatchSharedFixtures(t *testing.T) {
	records := loadSubscriptionFixtures(
		t,
		"subscriptions/frame-coalescing.txt",
		"runtime-frame-coalescing",
		"interval", "actions", "updates", "frames",
	)
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			clock := NewVirtualClock()
			config := NewRuntimeConfig(Size{Width: 3, Height: 1})
			config.MinimumFrameInterval = time.Duration(subscriptionFixtureInt(t, record.Field("interval"))) * time.Millisecond
			app := &counterApp{}
			runtime, err := NewRuntimeWithClock[runtimeMessage](app, config, clock)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(runtime.Close)
			initial, err := runtime.RenderIfDirty()
			if err != nil {
				t.Fatal(err)
			}
			frames := []uint64{initial.Timestamp().Nanoseconds() / 1_000_000}
			for _, action := range strings.Split(record.Field("actions"), ";") {
				kind, value, _ := strings.Cut(action, ":")
				switch kind {
				case "enqueue":
					if err := runtime.Enqueue(runtimeMessage{add: uint32(subscriptionFixtureInt(t, value))}); err != nil {
						t.Fatal(err)
					}
				case "advance":
					clock.Advance(time.Duration(subscriptionFixtureInt(t, value)) * time.Millisecond)
				case "urgent":
					runtime.RequestFrame()
				case "step":
					frame, err := runtime.Step()
					if err != nil {
						t.Fatal(err)
					}
					if frame != nil {
						frames = append(frames, frame.Timestamp().Nanoseconds()/1_000_000)
					}
				default:
					t.Fatalf("unknown action %q", action)
				}
			}
			updates := make([]uint64, len(app.updates))
			for index, update := range app.updates {
				updates[index] = uint64(update)
			}
			if expected := runtimeFixtureUintList(t, record.Field("updates")); !reflect.DeepEqual(updates, expected) {
				t.Fatalf("updates = %v, want %v", updates, expected)
			}
			if expected := runtimeFixtureUintList(t, record.Field("frames")); !reflect.DeepEqual(frames, expected) {
				t.Fatalf("frames = %v, want %v", frames, expected)
			}
		})
	}
}

type runtimeSubscriptionMessage struct {
	toggle bool
	tick   uint64
}

type runtimeSubscriptionApp struct {
	running bool
	counter atomic.Uint64
	values  []uint64
}

func (*runtimeSubscriptionApp) Init() Effect[runtimeSubscriptionMessage] {
	return NoneEffect[runtimeSubscriptionMessage]()
}

func (a *runtimeSubscriptionApp) Update(message runtimeSubscriptionMessage) Effect[runtimeSubscriptionMessage] {
	if message.toggle {
		a.running = !a.running
		return NoneEffect[runtimeSubscriptionMessage]().WithoutRedraw()
	} else {
		a.values = append(a.values, message.tick)
	}
	return NoneEffect[runtimeSubscriptionMessage]()
}

func (a *runtimeSubscriptionApp) Subscriptions() Subscription[runtimeSubscriptionMessage] {
	if !a.running {
		return NoneSubscription[runtimeSubscriptionMessage]()
	}
	return EverySubscription("ticks", 10*time.Millisecond, LatestDelivery(), func() runtimeSubscriptionMessage {
		return runtimeSubscriptionMessage{tick: a.counter.Add(1)}
	})
}

func (a *runtimeSubscriptionApp) View(_ ViewContext) Node[runtimeSubscriptionMessage] {
	return Text[runtimeSubscriptionMessage](strconv.Itoa(len(a.values)))
}

func TestRuntimePauseDiscardsQueuedSubscriptionAndResumeRestartsGeneration(t *testing.T) {
	clock := NewVirtualClock()
	app := &runtimeSubscriptionApp{running: true}
	runtime, err := NewRuntimeWithClock[runtimeSubscriptionMessage](
		app,
		NewRuntimeConfig(Size{Width: 2, Height: 1}),
		clock,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	key := SubscriptionKey("ticks")
	if generation := runtime.SubscriptionGeneration(key); generation != 1 {
		t.Fatalf("generation = %d, want 1", generation)
	}

	clock.Advance(10 * time.Millisecond)
	if err := runtime.Enqueue(runtimeSubscriptionMessage{toggle: true}); err != nil {
		t.Fatal(err)
	}
	if delivered := runtime.PollSubscriptions(); delivered != 1 {
		t.Fatalf("delivered = %d, want 1", delivered)
	}
	if _, err := runtime.ProcessPending(); err != nil {
		t.Fatal(err)
	}
	if len(app.values) != 0 || runtime.SubscriptionActive(key) {
		t.Fatalf("paused values = %v, active = %t", app.values, runtime.SubscriptionActive(key))
	}
	if discarded := runtime.SubscriptionDiagnostics().DiscardedMessages(); discarded != 1 {
		t.Fatalf("discarded = %d, want 1", discarded)
	}

	clock.Advance(10 * time.Millisecond)
	if _, err := runtime.ProcessPending(); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Enqueue(runtimeSubscriptionMessage{toggle: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ProcessPending(); err != nil {
		t.Fatal(err)
	}
	if !runtime.SubscriptionActive(key) || runtime.SubscriptionGeneration(key) != 2 {
		t.Fatalf("resumed active = %t, generation = %d", runtime.SubscriptionActive(key), runtime.SubscriptionGeneration(key))
	}
	clock.Advance(10 * time.Millisecond)
	if _, err := runtime.ProcessPending(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(app.values, []uint64{2}) {
		t.Fatalf("values = %v, want [2]", app.values)
	}
}

func runtimeFixtureUintList(t *testing.T, value string) []uint64 {
	t.Helper()
	if value == "-" || value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	values := make([]uint64, len(parts))
	for index, part := range parts {
		parsed, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			t.Fatalf("invalid fixture integer %q", part)
		}
		values[index] = parsed
	}
	return values
}

func TestRuntimeEnforcesQueueCapacityBeforeUpdate(t *testing.T) {
	config := NewRuntimeConfig(Size{Width: 1, Height: 1})
	config.QueueCapacity = 1
	runtime, err := NewRuntimeWithClock[runtimeMessage](&counterApp{}, config, NewVirtualClock())
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Enqueue(runtimeMessage{add: 1}); err != nil {
		t.Fatal(err)
	}

	if err := runtime.Enqueue(runtimeMessage{add: 2}); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("second enqueue error = %v, want ErrQueueFull", err)
	}
	if runtime.QueuedMessages() != 1 {
		t.Fatalf("queued messages = %d, want 1", runtime.QueuedMessages())
	}
}

type asyncMessage struct {
	kind  string
	value string
}

type asyncApp struct {
	old     Task[asyncMessage]
	current Task[asyncMessage]
	results []string
}

func (*asyncApp) Init() Effect[asyncMessage] { return NoneEffect[asyncMessage]() }
func (*asyncApp) Subscriptions() Subscription[asyncMessage] {
	return NoneSubscription[asyncMessage]()
}
func (a *asyncApp) Update(message asyncMessage) Effect[asyncMessage] {
	switch message.kind {
	case "start-old":
		task := a.old
		a.old = nil
		return LatestEffect("search", task)
	case "start-new":
		task := a.current
		a.current = nil
		return LatestEffect("search", task)
	case "result":
		a.results = append(a.results, message.value)
	}
	return NoneEffect[asyncMessage]()
}
func (a *asyncApp) View(_ ViewContext) Node[asyncMessage] {
	return Text[asyncMessage](strings.Join(a.results, ","))
}

type runtimeAsyncControl struct {
	started  chan context.Context
	complete chan asyncMessage
	returned chan struct{}
}

func runtimeAsyncTask() (Task[asyncMessage], *runtimeAsyncControl) {
	control := &runtimeAsyncControl{
		started:  make(chan context.Context, 1),
		complete: make(chan asyncMessage, 1),
		returned: make(chan struct{}, 1),
	}
	return func(ctx context.Context) asyncMessage {
		control.started <- ctx
		message := <-control.complete
		control.returned <- struct{}{}
		return message
	}, control
}

func waitRuntimeAsyncStarted(t *testing.T, control *runtimeAsyncControl) context.Context {
	t.Helper()
	for range 10_000 {
		select {
		case ctx := <-control.started:
			return ctx
		default:
			runtime.Gosched()
		}
	}
	t.Fatal("runtime async task did not start")
	return nil
}

func completeRuntimeAsyncTask(control *runtimeAsyncControl, message asyncMessage) {
	control.complete <- message
	<-control.returned
}

func TestRuntimeNeverDeliversStaleLatestResultsToApp(t *testing.T) {
	oldTask, old := runtimeAsyncTask()
	newTask, current := runtimeAsyncTask()
	app := &asyncApp{old: oldTask, current: newTask}
	runtimeUnderTest, err := NewRuntimeWithClock[asyncMessage](app, NewRuntimeConfig(Size{Width: 8, Height: 1}), NewVirtualClock())
	if err != nil {
		t.Fatal(err)
	}
	defer runtimeUnderTest.Close()

	if err := runtimeUnderTest.Enqueue(asyncMessage{kind: "start-old"}); err != nil {
		t.Fatal(err)
	}
	if _, err := runtimeUnderTest.ProcessPending(); err != nil {
		t.Fatal(err)
	}
	oldContext := waitRuntimeAsyncStarted(t, old)
	if err := runtimeUnderTest.Enqueue(asyncMessage{kind: "start-new"}); err != nil {
		t.Fatal(err)
	}
	if _, err := runtimeUnderTest.ProcessPending(); err != nil {
		t.Fatal(err)
	}
	_ = waitRuntimeAsyncStarted(t, current)
	select {
	case <-oldContext.Done():
	default:
		t.Fatal("old latest task did not receive cancellation")
	}

	completeRuntimeAsyncTask(old, asyncMessage{kind: "result", value: "old"})
	completeRuntimeAsyncTask(current, asyncMessage{kind: "result", value: "new"})
	for range 10_000 {
		runtimeUnderTest.PollEffects()
		if runtimeUnderTest.ActiveTasks() == 0 {
			break
		}
		runtime.Gosched()
	}
	if runtimeUnderTest.ActiveTasks() != 0 {
		t.Fatal("effect tasks did not finish")
	}
	if _, err := runtimeUnderTest.ProcessPending(); err != nil {
		t.Fatal(err)
	}

	if len(app.results) != 1 || app.results[0] != "new" {
		t.Fatalf("results = %v, want [new]", app.results)
	}
	if generation := runtimeUnderTest.TaskGeneration("search"); generation != 2 {
		t.Fatalf("generation = %d, want 2", generation)
	}
	if stale := runtimeUnderTest.EffectDiagnostics().StaleResults(); stale != 1 {
		t.Fatalf("stale results = %d, want 1", stale)
	}
}

type inputMessage struct {
	value string
}

type inputApp struct {
	value string
}

func (*inputApp) Init() Effect[inputMessage] { return NoneEffect[inputMessage]() }
func (*inputApp) Subscriptions() Subscription[inputMessage] {
	return NoneSubscription[inputMessage]()
}
func (a *inputApp) Update(message inputMessage) Effect[inputMessage] {
	a.value = message.value
	return NoneEffect[inputMessage]()
}
func (a *inputApp) View(_ ViewContext) Node[inputMessage] {
	return TextInput(NodeID("input"), a.value, func(value string) inputMessage {
		return inputMessage{value: value}
	})
}

func TestRuntimeTextInputRoutesUnicodeAndRendersCursor(t *testing.T) {
	app := &inputApp{}
	runtime, err := NewRuntimeWithClock[inputMessage](app, NewRuntimeConfig(Size{Width: 5, Height: 1}), NewVirtualClock())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if focused, err := runtime.RequestFocus(NodeID("input")); err != nil || !focused {
		t.Fatalf("RequestFocus = %t, %v", focused, err)
	}
	dispatch, err := runtime.DispatchEvent(vt.Event{Kind: vt.EventText, Text: "日"})
	if err != nil {
		t.Fatal(err)
	}
	if !dispatch.Consumed() || dispatch.Messages() != 1 {
		t.Fatalf("dispatch = %+v", dispatch)
	}
	frame, err := runtime.Step()
	if err != nil {
		t.Fatal(err)
	}
	if app.value != "日" {
		t.Fatalf("value = %q", app.value)
	}
	state, ok := runtime.Interaction().TextInput(NodeID("input"))
	if !ok || state.Cursor() != 3 {
		t.Fatalf("TextInput state = %+v, %t", state, ok)
	}
	if cursor, ok := frame.Surface().Cursor(); !ok || cursor != (surface.Cursor{X: 2}) {
		t.Fatalf("cursor = %+v, %t", cursor, ok)
	}

	_, err = runtime.DispatchEvent(vt.Event{Kind: vt.EventKey, Key: vt.KeyEvent{
		Code: vt.KeyBackspace, Action: vt.KeyActionUnknown, Protocol: vt.KeyProtocolLegacy,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Step(); err != nil {
		t.Fatal(err)
	}
	if app.value != "" {
		t.Fatalf("value after Backspace = %q", app.value)
	}
}

type focusMessage struct {
	visit string
}

type focusApp struct {
	ids      []string
	visits   []string
	handlers bool
}

func (*focusApp) Init() Effect[focusMessage] { return NoneEffect[focusMessage]() }
func (*focusApp) Subscriptions() Subscription[focusMessage] {
	return NoneSubscription[focusMessage]()
}
func (a *focusApp) Update(message focusMessage) Effect[focusMessage] {
	a.visits = append(a.visits, message.visit)
	return NoneEffect[focusMessage]()
}
func (a *focusApp) View(_ ViewContext) Node[focusMessage] {
	if a.handlers {
		input := Text[focusMessage]("input").Focusable("input").OnEvent("input", func(vt.Event) EventResult[focusMessage] {
			return IgnoreResult[focusMessage]().Emit(focusMessage{visit: "input"})
		})
		panel := Padding(input, Insets{}).OnEvent("panel", func(vt.Event) EventResult[focusMessage] {
			return MessageResult(focusMessage{visit: "panel"})
		})
		return Padding(panel, Insets{}).OnEvent("root", func(vt.Event) EventResult[focusMessage] {
			return MessageResult(focusMessage{visit: "root"})
		})
	}
	children := make([]Node[focusMessage], len(a.ids))
	for index, id := range a.ids {
		children[index] = Text[focusMessage](id).Focusable(NodeID(id))
	}
	return Column(children...)
}

func TestRuntimeFocusFirstSelectsFirstFocusableNode(t *testing.T) {
	runtime, err := NewRuntimeWithClock[focusMessage](
		&focusApp{ids: []string{"a", "b"}},
		NewRuntimeConfig(Size{Width: 8, Height: 2}),
		NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	focused, err := runtime.focusFirst()
	if err != nil {
		t.Fatal(err)
	}
	if !focused {
		t.Fatal("focusFirst returned false")
	}
	if id, ok := runtime.Interaction().Focused(); !ok || id != "a" {
		t.Fatalf("focus = %q, %t, want a", id, ok)
	}
}

func TestRuntimeFocusFallbackAndAncestorRouting(t *testing.T) {
	app := &focusApp{ids: []string{"a", "b", "c"}}
	runtime, err := NewRuntimeWithClock[focusMessage](app, NewRuntimeConfig(Size{Width: 8, Height: 3}), NewVirtualClock())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.RequestFocus("b"); err != nil {
		t.Fatal(err)
	}
	app.ids = []string{"a", "c"}
	runtime.RequestFrame()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if focused, ok := runtime.Interaction().Focused(); !ok || focused != "c" {
		t.Fatalf("focus = %q, %t", focused, ok)
	}

	app.handlers = true
	runtime.RequestFrame()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.RequestFocus("input"); err != nil {
		t.Fatal(err)
	}
	dispatch, err := runtime.DispatchEvent(vt.Event{Kind: vt.EventKey, Key: vt.KeyEvent{Code: vt.KeyRight}})
	if err != nil {
		t.Fatal(err)
	}
	if !dispatch.Consumed() {
		t.Fatal("event was not consumed")
	}
	if _, err := runtime.Step(); err != nil {
		t.Fatal(err)
	}
	if len(app.visits) != 2 || app.visits[0] != "input" || app.visits[1] != "panel" {
		t.Fatalf("visits = %v", app.visits)
	}
}

type modalApp struct {
	visits []string
}

func (*modalApp) Init() Effect[focusMessage] { return NoneEffect[focusMessage]() }
func (*modalApp) Subscriptions() Subscription[focusMessage] {
	return NoneSubscription[focusMessage]()
}
func (a *modalApp) Update(message focusMessage) Effect[focusMessage] {
	a.visits = append(a.visits, message.visit)
	return NoneEffect[focusMessage]()
}
func (*modalApp) View(_ ViewContext) Node[focusMessage] {
	background := Text[focusMessage]("background").Focusable("background").OnEvent("background", func(vt.Event) EventResult[focusMessage] {
		return IgnoreResult[focusMessage]().Emit(focusMessage{visit: "background"})
	})
	input := Text[focusMessage]("input").Focusable("input").OnEvent("input", func(vt.Event) EventResult[focusMessage] {
		return IgnoreResult[focusMessage]().Emit(focusMessage{visit: "input"})
	})
	modal := Modal("modal", input).OnEvent("modal", func(vt.Event) EventResult[focusMessage] {
		return IgnoreResult[focusMessage]().Emit(focusMessage{visit: "modal"})
	})
	return Stack(background, modal).OnEvent("root", func(vt.Event) EventResult[focusMessage] {
		return MessageResult(focusMessage{visit: "root"})
	})
}

func TestRuntimeModalRestrictsFocusAndRoutesThroughModalAncestors(t *testing.T) {
	app := &modalApp{}
	runtime, err := NewRuntimeWithClock[focusMessage](app, NewRuntimeConfig(Size{Width: 12, Height: 1}), NewVirtualClock())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if focused, err := runtime.RequestFocus("background"); err != nil || focused {
		t.Fatalf("RequestFocus(background) = %t, %v", focused, err)
	}
	tab, err := runtime.DispatchEvent(vt.Event{Kind: vt.EventKey, Key: vt.KeyEvent{Code: vt.KeyTab}})
	if err != nil {
		t.Fatal(err)
	}
	if !tab.Consumed() {
		t.Fatal("Tab was not consumed")
	}
	if focused, ok := runtime.Interaction().Focused(); !ok || focused != "input" {
		t.Fatalf("focus = %q, %t", focused, ok)
	}

	routed, err := runtime.DispatchEvent(vt.Event{Kind: vt.EventKey, Key: vt.KeyEvent{Code: vt.KeyRight}})
	if err != nil {
		t.Fatal(err)
	}
	if !routed.Consumed() || routed.Messages() != 3 {
		t.Fatalf("dispatch = %+v", routed)
	}
	if _, err := runtime.Step(); err != nil {
		t.Fatal(err)
	}
	if len(app.visits) != 3 || app.visits[0] != "input" || app.visits[1] != "modal" || app.visits[2] != "root" {
		t.Fatalf("visits = %v", app.visits)
	}
}

type scrollApp struct{}

func (*scrollApp) Init() Effect[focusMessage] { return NoneEffect[focusMessage]() }
func (*scrollApp) Subscriptions() Subscription[focusMessage] {
	return NoneSubscription[focusMessage]()
}
func (*scrollApp) Update(focusMessage) Effect[focusMessage] { return NoneEffect[focusMessage]() }
func (*scrollApp) View(_ ViewContext) Node[focusMessage] {
	return ScrollViewport("scroll", Column(
		Text[focusMessage]("A").WithLength(Fixed(1)),
		Text[focusMessage]("B").WithLength(Fixed(1)),
		Text[focusMessage]("C").WithLength(Fixed(1)),
	))
}

func TestRuntimeScrollViewportClampsAndClips(t *testing.T) {
	runtime, err := NewRuntimeWithClock[focusMessage](&scrollApp{}, NewRuntimeConfig(Size{Width: 3, Height: 2}), NewVirtualClock())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if !runtime.SetScrollOffset("scroll", ScrollOffset{Y: 9}) {
		t.Fatal("SetScrollOffset returned false")
	}
	frame, err := runtime.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	assertNodeCell(t, frame.Surface(), 0, 0, "B")
	assertNodeCell(t, frame.Surface(), 0, 1, "C")
	if offset := runtime.Interaction().ScrollOffset("scroll"); offset != (ScrollOffset{Y: 1}) {
		t.Fatalf("offset = %+v", offset)
	}
}

type virtualScrollApp struct {
	builds atomic.Int64
	rows   atomic.Int64
}

func (*virtualScrollApp) Init() Effect[struct{}] { return NoneEffect[struct{}]() }
func (*virtualScrollApp) Subscriptions() Subscription[struct{}] {
	return NoneSubscription[struct{}]()
}
func (*virtualScrollApp) Update(struct{}) Effect[struct{}] { return NoneEffect[struct{}]() }
func (a *virtualScrollApp) View(_ ViewContext) Node[struct{}] {
	return VirtualScrollViewportWithOptions(
		"virtual-scroll",
		Size{Width: 3, Height: 1_000_000},
		ScrollViewportOptions[struct{}]{Axis: ScrollAxisVertical},
		func(viewport VirtualViewport) VirtualFragment[struct{}] {
			a.builds.Add(1)
			a.rows.Add(int64(viewport.Size.Height))
			rows := make([]Node[struct{}], viewport.Size.Height)
			for index := range rows {
				row := viewport.Offset.Y + uint32(index)
				rows[index] = Text[struct{}](strconv.Itoa(int(row % 10))).
					WithID(NewNodeID("row-" + strconv.FormatUint(uint64(row), 10)))
			}
			return NewVirtualFragment(ScrollOffset{Y: viewport.Offset.Y}, Column(rows...))
		},
	)
}

func TestVirtualScrollViewportBoundsConstructionToVisibleRows(t *testing.T) {
	app := &virtualScrollApp{}
	runtime, err := NewRuntimeWithClock[struct{}](
		app,
		NewRuntimeConfig(Size{Width: 3, Height: 2}),
		NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()

	first, err := runtime.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	if app.builds.Load() != 1 || app.rows.Load() != 2 {
		t.Fatalf("initial construction = %d builds, %d rows", app.builds.Load(), app.rows.Load())
	}
	if len(runtime.treeIndex.records) != 3 {
		t.Fatalf("initial tree records = %d", len(runtime.treeIndex.records))
	}
	firstSurface := first.Surface()
	assertNodeCell(t, firstSurface, 0, 0, "0")
	assertNodeCell(t, firstSurface, 0, 1, "1")

	if !runtime.SetScrollOffset("virtual-scroll", ScrollOffset{Y: ^uint32(0)}) {
		t.Fatal("SetScrollOffset returned false")
	}
	last, err := runtime.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	if app.builds.Load() != 2 || app.rows.Load() != 4 {
		t.Fatalf("total construction = %d builds, %d rows", app.builds.Load(), app.rows.Load())
	}
	if len(runtime.treeIndex.records) != 3 {
		t.Fatalf("final tree records = %d", len(runtime.treeIndex.records))
	}
	lastSurface := last.Surface()
	assertNodeCell(t, lastSurface, 0, 0, "8")
	assertNodeCell(t, lastSurface, 0, 1, "9")
	state, ok := runtime.Interaction().ScrollState("virtual-scroll")
	if !ok || state.Maximum != (ScrollOffset{Y: 999_998}) {
		t.Fatalf("scroll state = %+v, present = %t", state, ok)
	}

	for offset := range uint32(256) {
		if !runtime.SetScrollOffset("virtual-scroll", ScrollOffset{Y: offset}) {
			t.Fatal("SetScrollOffset returned false during sustained scrolling")
		}
		if frame, err := runtime.RenderIfDirty(); err != nil || frame == nil {
			t.Fatalf("sustained frame %d = %v, %v", offset, frame, err)
		}
		if len(runtime.treeIndex.records) != 3 {
			t.Fatalf("sustained tree records = %d", len(runtime.treeIndex.records))
		}
	}
	if app.builds.Load() != 258 || app.rows.Load() != 516 {
		t.Fatalf("sustained construction = %d builds, %d rows", app.builds.Load(), app.rows.Load())
	}
}

type growingVirtualScrollApp struct {
	contentHeight uint32
	builds        atomic.Int64
	rows          atomic.Int64
}

func (*growingVirtualScrollApp) Init() Effect[struct{}] { return NoneEffect[struct{}]() }
func (*growingVirtualScrollApp) Subscriptions() Subscription[struct{}] {
	return NoneSubscription[struct{}]()
}
func (*growingVirtualScrollApp) Update(struct{}) Effect[struct{}] {
	return NoneEffect[struct{}]()
}
func (a *growingVirtualScrollApp) View(ViewContext) Node[struct{}] {
	return VirtualScrollViewportWithOptions(
		"growing-virtual-scroll",
		Size{Width: 3, Height: a.contentHeight},
		ScrollViewportOptions[struct{}]{Axis: ScrollAxisVertical, StickToEnd: true},
		func(viewport VirtualViewport) VirtualFragment[struct{}] {
			a.builds.Add(1)
			a.rows.Add(int64(viewport.Size.Height))
			rows := make([]Node[struct{}], viewport.Size.Height)
			for index := range rows {
				row := viewport.Offset.Y + uint32(index)
				rows[index] = Text[struct{}](strconv.FormatUint(uint64(row), 10))
			}
			return NewVirtualFragment(ScrollOffset{Y: viewport.Offset.Y}, Column(rows...))
		},
	)
}

func TestVirtualScrollViewportBuildsOnceWhenFollowingGrowingEnd(t *testing.T) {
	app := &growingVirtualScrollApp{contentHeight: 4}
	runtime, err := NewRuntimeWithClock[struct{}](
		app,
		NewRuntimeConfig(Size{Width: 3, Height: 2}),
		NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()

	initial, err := runtime.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	if app.builds.Load() != 1 || app.rows.Load() != 2 {
		t.Fatalf("initial construction = %d builds, %d rows", app.builds.Load(), app.rows.Load())
	}
	assertNodeCell(t, initial.Surface(), 0, 0, "2")
	assertNodeCell(t, initial.Surface(), 0, 1, "3")

	app.contentHeight = 5
	runtime.RequestFrame()
	grown, err := runtime.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	if app.builds.Load() != 2 || app.rows.Load() != 4 {
		t.Fatalf("growth construction = %d builds, %d rows", app.builds.Load(), app.rows.Load())
	}
	assertNodeCell(t, grown.Surface(), 0, 0, "3")
	assertNodeCell(t, grown.Surface(), 0, 1, "4")
	state, ok := runtime.Interaction().ScrollState("growing-virtual-scroll")
	if !ok || state.Offset != (ScrollOffset{Y: 3}) || state.Maximum != (ScrollOffset{Y: 3}) || !state.AtEnd {
		t.Fatalf("grown scroll state = %+v, present = %t", state, ok)
	}

	if !runtime.SetScrollOffset("growing-virtual-scroll", ScrollOffset{Y: 1}) {
		t.Fatal("SetScrollOffset returned false")
	}
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	app.contentHeight = 6
	runtime.RequestFrame()
	away, err := runtime.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	if app.builds.Load() != 4 || app.rows.Load() != 8 {
		t.Fatalf("away construction = %d builds, %d rows", app.builds.Load(), app.rows.Load())
	}
	assertNodeCell(t, away.Surface(), 0, 0, "1")
	assertNodeCell(t, away.Surface(), 0, 1, "2")
	state, ok = runtime.Interaction().ScrollState("growing-virtual-scroll")
	if !ok || state.Offset != (ScrollOffset{Y: 1}) || state.Maximum != (ScrollOffset{Y: 4}) || state.AtEnd {
		t.Fatalf("away scroll state = %+v, present = %t", state, ok)
	}
}

type overscannedScrollApp struct{}

func (*overscannedScrollApp) Init() Effect[struct{}] { return NoneEffect[struct{}]() }
func (*overscannedScrollApp) Subscriptions() Subscription[struct{}] {
	return NoneSubscription[struct{}]()
}
func (*overscannedScrollApp) Update(struct{}) Effect[struct{}] { return NoneEffect[struct{}]() }
func (*overscannedScrollApp) View(ViewContext) Node[struct{}] {
	return VirtualScrollViewportWithOptions(
		"overscanned-scroll",
		Size{Width: 3, Height: 6},
		ScrollViewportOptions[struct{}]{Axis: ScrollAxisVertical},
		func(viewport VirtualViewport) VirtualFragment[struct{}] {
			start := viewport.Offset.Y
			if start > 0 {
				start--
			}
			end := min(
				saturatingAdd32(saturatingAdd32(viewport.Offset.Y, viewport.Size.Height), 1),
				viewport.ContentSize.Height,
			)
			rows := make([]Node[struct{}], 0, end-start)
			for row := start; row < end; row++ {
				rows = append(rows, Text[struct{}](strconv.FormatUint(uint64(row), 10)).
					WithID(NewNodeID("overscan-row-"+strconv.FormatUint(uint64(row), 10))))
			}
			return NewVirtualFragment(ScrollOffset{Y: start}, Column(rows...))
		},
	)
}

func TestVirtualScrollViewportPositionsBoundedOverscan(t *testing.T) {
	runtime, err := NewRuntimeWithClock[struct{}](
		&overscannedScrollApp{},
		NewRuntimeConfig(Size{Width: 3, Height: 2}),
		NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if !runtime.SetScrollOffset("overscanned-scroll", ScrollOffset{Y: 2}) {
		t.Fatal("SetScrollOffset returned false")
	}

	frame, err := runtime.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	rendered := frame.Surface()
	assertNodeCell(t, rendered, 0, 0, "2")
	assertNodeCell(t, rendered, 0, 1, "3")
	if len(runtime.treeIndex.records) != 5 {
		t.Fatalf("tree records = %d, want 5", len(runtime.treeIndex.records))
	}
}

func TestVirtualScrollViewportRetainedMemoryStabilizes(t *testing.T) {
	app := &virtualScrollApp{}
	uiRuntime, err := NewRuntimeWithClock[struct{}](
		app,
		NewRuntimeConfig(Size{Width: 80, Height: 24}),
		NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer uiRuntime.Close()
	if _, err := uiRuntime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}

	renderVirtualFrames(t, uiRuntime, 1, 512)
	baseline := heapAllocationAfterGC()
	renderVirtualFrames(t, uiRuntime, 10_000, 512)
	retained := heapAllocationAfterGC()
	const tolerance = uint64(1 << 20)
	if retained > baseline+tolerance {
		t.Fatalf("retained heap grew from %d to %d bytes", baseline, retained)
	}
	runtime.KeepAlive(uiRuntime)
}

func renderVirtualFrames(t *testing.T, uiRuntime *Runtime[struct{}], start, count uint32) {
	t.Helper()
	for offset := start; offset < start+count; offset++ {
		if !uiRuntime.SetScrollOffset("virtual-scroll", ScrollOffset{Y: offset}) {
			t.Fatal("SetScrollOffset returned false")
		}
		if frame, err := uiRuntime.RenderIfDirty(); err != nil || frame == nil {
			t.Fatalf("virtual frame %d = %v, %v", offset, frame, err)
		}
	}
}

func heapAllocationAfterGC() uint64 {
	runtime.GC()
	var statistics runtime.MemStats
	runtime.ReadMemStats(&statistics)
	return statistics.HeapAlloc
}

type lifecycleMessage uint8

const lifecycleStop lifecycleMessage = iota

type lifecycleApp struct {
	stopped bool
}

func (*lifecycleApp) Init() Effect[lifecycleMessage] {
	return NoneEffect[lifecycleMessage]()
}

func (*lifecycleApp) Subscriptions() Subscription[lifecycleMessage] {
	return NoneSubscription[lifecycleMessage]()
}

func (a *lifecycleApp) Update(message lifecycleMessage) Effect[lifecycleMessage] {
	if message == lifecycleStop {
		a.stopped = true
	}
	return ExitEffect[lifecycleMessage]()
}

func (a *lifecycleApp) View(context ViewContext) Node[lifecycleMessage] {
	state := "running"
	if a.stopped {
		state = "stopped"
	}
	return Text[lifecycleMessage](state + ":" + strconv.FormatUint(uint64(context.Size.Width), 10))
}

func TestExitEffectPreservesFinalViewAndViewReceivesSize(t *testing.T) {
	runtime, err := NewRuntimeWithClock[lifecycleMessage](
		&lifecycleApp{},
		NewRuntimeConfig(Size{Width: 12, Height: 1}),
		NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := runtime.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	assertNodeCell(t, initial.Surface(), 0, 0, "r")
	if runtime.ExitRequested() {
		t.Fatal("ExitRequested = true before update")
	}

	if err := runtime.Enqueue(lifecycleStop); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ProcessPending(); err != nil {
		t.Fatal(err)
	}
	if !runtime.ExitRequested() {
		t.Fatal("ExitRequested = false after ExitEffect")
	}
	finalFrame, err := runtime.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	assertNodeCell(t, finalFrame.Surface(), 0, 0, "s")

	runtime.Resize(Size{Width: 9, Height: 1})
	resized, err := runtime.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	assertNodeCell(t, resized.Surface(), 8, 0, "9")
}

type commandMessage uint8

const commandApply commandMessage = iota

type commandApp struct{}

func (*commandApp) Init() Effect[commandMessage] {
	return NoneEffect[commandMessage]()
}

func (*commandApp) Subscriptions() Subscription[commandMessage] {
	return NoneSubscription[commandMessage]()
}

func (*commandApp) Update(commandMessage) Effect[commandMessage] {
	return BatchEffects(
		FocusEffect[commandMessage]("target"),
		ScrollToEffect[commandMessage]("scroll", ScrollOffset{Y: ^uint32(0)}),
	)
}

func (*commandApp) View(ViewContext) Node[commandMessage] {
	return ScrollViewport("scroll", Column(
		Text[commandMessage]("A").WithLength(Fixed(1)),
		Text[commandMessage]("B").Focusable("target").WithLength(Fixed(1)),
		Text[commandMessage]("C").WithLength(Fixed(1)),
	))
}

func TestFocusAndScrollEffectsApplySynchronouslyToNextView(t *testing.T) {
	runtime, err := NewRuntimeWithClock[commandMessage](
		&commandApp{},
		NewRuntimeConfig(Size{Width: 3, Height: 2}),
		NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Enqueue(commandApply); err != nil {
		t.Fatal(err)
	}
	frame, err := runtime.Step()
	if err != nil {
		t.Fatal(err)
	}

	if focused, ok := runtime.Interaction().Focused(); !ok || focused != "target" {
		t.Fatalf("focus = %q, %t", focused, ok)
	}
	if offset := runtime.Interaction().ScrollOffset("scroll"); offset != (ScrollOffset{Y: 1}) {
		t.Fatalf("offset = %+v", offset)
	}
	assertNodeCell(t, frame.Surface(), 0, 0, "B")
}

type advancedScrollMessage struct {
	state ScrollState
}

type advancedScrollApp struct {
	lines                uint32
	stickToEnd           bool
	ensureFocusedVisible bool
	observed             []ScrollState
}

func (*advancedScrollApp) Init() Effect[advancedScrollMessage] {
	return NoneEffect[advancedScrollMessage]()
}

func (*advancedScrollApp) Subscriptions() Subscription[advancedScrollMessage] {
	return NoneSubscription[advancedScrollMessage]()
}

func (a *advancedScrollApp) Update(message advancedScrollMessage) Effect[advancedScrollMessage] {
	a.observed = append(a.observed, message.state)
	return NoneEffect[advancedScrollMessage]()
}

func (a *advancedScrollApp) View(ViewContext) Node[advancedScrollMessage] {
	rows := make([]Node[advancedScrollMessage], a.lines)
	for index := range a.lines {
		row := Text[advancedScrollMessage](strconv.FormatUint(uint64(index), 10))
		if index+1 == a.lines {
			row = row.Focusable("target")
		}
		rows[index] = row.WithLength(Fixed(1))
	}
	viewport := ScrollViewportWithOptions(
		NodeID("scroll"),
		Column(rows...),
		ScrollViewportOptions[advancedScrollMessage]{
			Axis:                 ScrollAxisVertical,
			StickToEnd:           a.stickToEnd,
			EnsureFocusedVisible: a.ensureFocusedVisible,
			OnScroll: func(state ScrollState) advancedScrollMessage {
				return advancedScrollMessage{state: state}
			},
		},
	).WithLength(Fixed(2))
	return Column(
		viewport,
		Text[advancedScrollMessage]("footer").WithLength(Flex(1)),
	)
}

func TestScrollStateUsesViewportPageAndStickToEndResumesAtEnd(t *testing.T) {
	app := &advancedScrollApp{lines: 6, stickToEnd: true}
	runtime, err := NewRuntimeWithClock[advancedScrollMessage](
		app,
		NewRuntimeConfig(Size{Width: 8, Height: 5}),
		NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	state, ok := runtime.Interaction().ScrollState("scroll")
	if !ok || state != (ScrollState{
		Offset: ScrollOffset{Y: 4}, Maximum: ScrollOffset{Y: 4}, AtEnd: true,
	}) {
		t.Fatalf("initial state = %+v, %t", state, ok)
	}
	if _, err := runtime.RequestFocus("target"); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}

	dispatch, err := runtime.DispatchEvent(vt.Event{Kind: vt.EventKey, Key: vt.KeyEvent{Code: vt.KeyPageUp}})
	if err != nil {
		t.Fatal(err)
	}
	if !dispatch.Consumed() {
		t.Fatal("PageUp was not consumed")
	}
	if _, err := runtime.ProcessPending(); err != nil {
		t.Fatal(err)
	}
	if offset := runtime.Interaction().ScrollOffset("scroll"); offset != (ScrollOffset{Y: 2}) {
		t.Fatalf("offset after PageUp = %+v", offset)
	}
	if len(app.observed) != 1 || app.observed[0].AtEnd {
		t.Fatalf("observed = %+v", app.observed)
	}

	app.lines = 7
	runtime.RequestFrame()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	state, _ = runtime.Interaction().ScrollState("scroll")
	if state.Offset != (ScrollOffset{Y: 2}) || state.Maximum != (ScrollOffset{Y: 5}) {
		t.Fatalf("state after growth away from end = %+v", state)
	}

	if _, err := runtime.DispatchEvent(vt.Event{Kind: vt.EventKey, Key: vt.KeyEvent{Code: vt.KeyEnd}}); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ProcessPending(); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	app.lines = 8
	runtime.RequestFrame()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	state, _ = runtime.Interaction().ScrollState("scroll")
	if state.Offset != (ScrollOffset{Y: 6}) || state.Maximum != (ScrollOffset{Y: 6}) || !state.AtEnd {
		t.Fatalf("state after resumed following = %+v", state)
	}
}

func TestFocusedDescendantIsRevealedWhenRequested(t *testing.T) {
	runtime, err := NewRuntimeWithClock[advancedScrollMessage](
		&advancedScrollApp{lines: 5, ensureFocusedVisible: true},
		NewRuntimeConfig(Size{Width: 8, Height: 4}),
		NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.RequestFocus("target"); err != nil {
		t.Fatal(err)
	}
	frame, err := runtime.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	if offset := runtime.Interaction().ScrollOffset("scroll"); offset != (ScrollOffset{Y: 3}) {
		t.Fatalf("offset = %+v", offset)
	}
	assertNodeCell(t, frame.Surface(), 0, 1, "4")
}
