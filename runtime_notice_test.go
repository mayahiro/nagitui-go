package tui

import (
	"context"
	"reflect"
	"runtime"
	"testing"
)

type runtimeNoticeMessage struct{}

type effectPanicNoticeApp struct{}

func (*effectPanicNoticeApp) Init() Effect[runtimeNoticeMessage] {
	return LatestEffect("load", func(context.Context) runtimeNoticeMessage {
		panic("effect failure")
	})
}

func (*effectPanicNoticeApp) Update(runtimeNoticeMessage) Effect[runtimeNoticeMessage] {
	return NoneEffect[runtimeNoticeMessage]()
}

func (*effectPanicNoticeApp) View(ViewContext) Node[runtimeNoticeMessage] {
	return Text[runtimeNoticeMessage]("")
}

func (*effectPanicNoticeApp) Subscriptions() Subscription[runtimeNoticeMessage] {
	return NoneSubscription[runtimeNoticeMessage]()
}

type streamNoticeApp struct {
	panics bool
}

func (*streamNoticeApp) Init() Effect[runtimeNoticeMessage] {
	return NoneEffect[runtimeNoticeMessage]()
}

func (*streamNoticeApp) Update(runtimeNoticeMessage) Effect[runtimeNoticeMessage] {
	return NoneEffect[runtimeNoticeMessage]()
}

func (*streamNoticeApp) View(ViewContext) Node[runtimeNoticeMessage] {
	return Text[runtimeNoticeMessage]("")
}

func (a *streamNoticeApp) Subscriptions() Subscription[runtimeNoticeMessage] {
	return StreamSubscription("events", ReliableDelivery(), func(context.Context, SubscriptionSink[runtimeNoticeMessage]) {
		if a.panics {
			panic("stream failure")
		}
	})
}

func waitForRuntimeNotice[Message any](t *testing.T, runtimeUnderTest *Runtime[Message], pollEffects bool) RuntimeNotice {
	t.Helper()
	for range 10_000 {
		if pollEffects {
			runtimeUnderTest.PollEffects()
		}
		if runtimeUnderTest.PendingRuntimeNotices() > 0 {
			notices := runtimeUnderTest.DrainRuntimeNotices()
			return notices[0]
		}
		runtime.Gosched()
	}
	t.Fatal("runtime notice was not produced")
	return RuntimeNotice{}
}

func TestRuntimeReportsEffectPanicWithLatestIdentity(t *testing.T) {
	runtimeUnderTest, err := NewRuntimeWithClock[runtimeNoticeMessage](
		&effectPanicNoticeApp{},
		NewRuntimeConfig(Size{Width: 1, Height: 1}),
		NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtimeUnderTest.Close()

	notice := waitForRuntimeNotice(t, runtimeUnderTest, true)
	if notice.Kind() != RuntimeNoticeEffectPanicked {
		t.Fatalf("kind = %d", notice.Kind())
	}
	key, generation, ok := notice.Task()
	if !ok || key != "load" || generation != 1 {
		t.Fatalf("task = %q, %d, %t", key, generation, ok)
	}
	if runtimeUnderTest.EffectDiagnostics().TaskPanics() != 1 {
		t.Fatal("effect panic counter was not updated")
	}
}

func TestRuntimeReportsSubscriptionStreamCompletionAndPanic(t *testing.T) {
	for _, test := range []struct {
		name   string
		panics bool
		kind   RuntimeNoticeKind
	}{
		{name: "completed", kind: RuntimeNoticeSubscriptionStreamCompleted},
		{name: "panicked", panics: true, kind: RuntimeNoticeSubscriptionStreamPanicked},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtimeUnderTest, err := NewRuntimeWithClock[runtimeNoticeMessage](
				&streamNoticeApp{panics: test.panics},
				NewRuntimeConfig(Size{Width: 1, Height: 1}),
				NewVirtualClock(),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer runtimeUnderTest.Close()

			notice := waitForRuntimeNotice(t, runtimeUnderTest, false)
			if notice.Kind() != test.kind {
				t.Fatalf("kind = %d, want %d", notice.Kind(), test.kind)
			}
			key, generation, ok := notice.Subscription()
			if !ok || key != "events" || generation != 1 {
				t.Fatalf("subscription = %q, %d, %t", key, generation, ok)
			}
		})
	}
}

func TestRuntimeNoticeQueueIsBoundedAndPreservesOldest(t *testing.T) {
	queue := newRuntimeNoticeQueue(1)
	queue.push(RuntimeNotice{kind: RuntimeNoticeEffectPanicked})
	queue.push(RuntimeNotice{kind: RuntimeNoticeSubscriptionStreamCompleted})

	notices := queue.drain()
	if len(notices) != 1 || notices[0].Kind() != RuntimeNoticeEffectPanicked {
		t.Fatalf("notices = %+v", notices)
	}
	if queue.diagnostics().Dropped() != 1 {
		t.Fatalf("dropped = %d", queue.diagnostics().Dropped())
	}
}

func TestCancelledSubscriptionStreamDoesNotReportCompletion(t *testing.T) {
	queue := newRuntimeNoticeQueue(4)
	supervisor := newSubscriptionSupervisor[runtimeNoticeMessage](1)
	supervisor.notices = queue
	started := make(chan struct{})
	if _, err := supervisor.reconcile(
		StreamSubscription("events", ReliableDelivery(), func(ctx context.Context, _ SubscriptionSink[runtimeNoticeMessage]) {
			close(started)
			<-ctx.Done()
		}),
		0,
	); err != nil {
		t.Fatal(err)
	}
	<-started
	finished := supervisor.active["events"].finished
	supervisor.close()
	for range 10_000 {
		if finished.Load() {
			break
		}
		runtime.Gosched()
	}
	if !finished.Load() {
		t.Fatal("cancelled stream did not return")
	}
	if queue.pending() != 0 {
		t.Fatalf("cancelled stream notices = %+v", queue.drain())
	}
}

func TestRuntimeNoticeHandlerDrainsTerminalFacadeQueue(t *testing.T) {
	runtimeUnderTest, err := NewRuntimeWithClock[runtimeMessage](
		&counterApp{},
		NewRuntimeConfig(Size{Width: 1, Height: 1}),
		NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtimeUnderTest.Close()
	runtimeUnderTest.notices.push(RuntimeNotice{kind: RuntimeNoticeEffectPanicked})

	var observed []RuntimeNoticeKind
	handleRuntimeNotices(runtimeUnderTest, func(notice RuntimeNotice) {
		observed = append(observed, notice.Kind())
	})
	if len(observed) != 1 || observed[0] != RuntimeNoticeEffectPanicked {
		t.Fatalf("observed = %v", observed)
	}
	if runtimeUnderTest.PendingRuntimeNotices() != 0 {
		t.Fatal("handled notice remained queued")
	}
}

func TestRuntimeNoticeMapperUpdatesApplicationWithoutSubscription(t *testing.T) {
	app := &counterApp{}
	runtimeUnderTest, err := NewRuntimeWithClock[runtimeMessage](
		app,
		NewRuntimeConfig(Size{Width: 1, Height: 1}),
		NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtimeUnderTest.Close()
	runtimeUnderTest.notices.push(RuntimeNotice{kind: RuntimeNoticeEffectPanicked})

	err = mapRuntimeNotices(runtimeUnderTest, func(notice RuntimeNotice) (runtimeMessage, bool) {
		return runtimeMessage{add: 2}, notice.Kind() == RuntimeNoticeEffectPanicked
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(app.updates, []uint32{2}) {
		t.Fatalf("updates = %v, want [2]", app.updates)
	}
	if runtimeUnderTest.PendingRuntimeNotices() != 0 {
		t.Fatal("mapped notice remained queued")
	}
}

func TestRuntimeRejectsZeroNoticeCapacity(t *testing.T) {
	config := NewRuntimeConfig(Size{Width: 1, Height: 1})
	config.RuntimeNoticeCapacity = 0
	if _, err := NewRuntimeWithClock[runtimeMessage](&counterApp{}, config, NewVirtualClock()); err != ErrZeroRuntimeNoticeCapacity {
		t.Fatalf("error = %v, want ErrZeroRuntimeNoticeCapacity", err)
	}
}
