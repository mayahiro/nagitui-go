package tui

import (
	"context"
	"errors"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mayahiro/nagitui-go/internal/conformance"
)

func TestSubscriptionLifecycleMatchesSharedFixtures(t *testing.T) {
	records := loadSubscriptionFixtures(
		t,
		"subscriptions/lifecycle.txt",
		"subscription-lifecycle",
		"frames", "starts", "stops", "active", "generations",
	)
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			supervisor := newSubscriptionSupervisor[string](8)
			t.Cleanup(supervisor.close)
			var stops []string
			for _, frame := range strings.Split(record.Field("frames"), "|") {
				declaration := NoneSubscription[string]()
				if frame != "-" {
					parts := strings.Split(frame, ",")
					declarations := make([]Subscription[string], 0, len(parts))
					for _, part := range parts {
						declarations = append(declarations, parseSubscriptionDeclaration(t, part))
					}
					declaration = BatchSubscriptions(declarations...)
				}
				reconciliation, err := supervisor.reconcile(declaration, 0)
				if err != nil {
					t.Fatal(err)
				}
				for _, tag := range reconciliation.stopped {
					stops = append(stops, tag.String())
				}
			}
			expectedStarts := subscriptionFixtureList(record.Field("starts"))
			var starts []string
			for key, generation := range supervisor.generations {
				for current := uint64(1); current <= generation; current++ {
					starts = append(starts, string(key)+":"+strconv.FormatUint(current, 10))
				}
			}
			sort.Strings(starts)
			sort.Strings(expectedStarts)
			if !reflect.DeepEqual(starts, expectedStarts) {
				t.Fatalf("starts = %v, want %v", starts, expectedStarts)
			}
			if expected := subscriptionFixtureList(record.Field("stops")); !reflect.DeepEqual(stops, expected) {
				t.Fatalf("stops = %v, want %v", stops, expected)
			}
			active := make([]string, len(supervisor.order))
			for index, key := range supervisor.order {
				active[index] = string(key)
			}
			if expected := subscriptionFixtureList(record.Field("active")); !reflect.DeepEqual(active, expected) {
				t.Fatalf("active = %v, want %v", active, expected)
			}
			var generations []string
			for key, generation := range supervisor.generations {
				generations = append(generations, string(key)+":"+strconv.FormatUint(generation, 10))
			}
			expectedGenerations := subscriptionFixtureList(record.Field("generations"))
			sort.Strings(generations)
			sort.Strings(expectedGenerations)
			if !reflect.DeepEqual(generations, expectedGenerations) {
				t.Fatalf("generations = %v, want %v", generations, expectedGenerations)
			}
		})
	}
}

func TestSubscriptionBackpressureMatchesSharedFixtures(t *testing.T) {
	records := loadSubscriptionFixtures(
		t,
		"subscriptions/backpressure.txt",
		"subscription-backpressure",
		"policy", "capacity", "actions", "checkpoints", "replacements",
	)
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			started := make(chan SubscriptionSink[string], 1)
			supervisor := newSubscriptionSupervisor[string](subscriptionFixtureInt(t, record.Field("capacity")))
			t.Cleanup(supervisor.close)
			_, err := supervisor.reconcile(
				StreamSubscription("source", parseSubscriptionPolicy(t, record.Field("policy")), func(ctx context.Context, sink SubscriptionSink[string]) {
					started <- sink
					<-ctx.Done()
				}),
				0,
			)
			if err != nil {
				t.Fatal(err)
			}
			sink := waitSubscriptionSink(t, started)
			var now Timestamp
			var checkpoints []string
			for _, action := range strings.Split(record.Field("actions"), ";") {
				kind, value, _ := strings.Cut(action, ":")
				switch kind {
				case "send":
					if !sink.Send(value) {
						t.Fatal("subscription sink closed")
					}
				case "advance":
					now = now.Add(time.Duration(subscriptionFixtureInt(t, value)) * time.Millisecond)
				case "poll":
					supervisor.poll(now)
					checkpoints = append(checkpoints, joinSubscriptionMessages(supervisor.takeReady(1<<30)))
				default:
					t.Fatalf("unknown action %q", action)
				}
			}
			if actual := strings.Join(checkpoints, "|"); actual != record.Field("checkpoints") {
				t.Fatalf("checkpoints = %q, want %q", actual, record.Field("checkpoints"))
			}
			if actual, expected := supervisor.diagnostics().LatestReplacements(), uint64(subscriptionFixtureInt(t, record.Field("replacements"))); actual != expected {
				t.Fatalf("replacements = %d, want %d", actual, expected)
			}
		})
	}
}

func TestEverySubscriptionMatchesSharedVirtualTimeFixtures(t *testing.T) {
	records := loadSubscriptionFixtures(
		t,
		"subscriptions/every.txt",
		"subscription-every",
		"interval", "policy", "capacity", "advances", "checkpoints",
	)
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			supervisor := newSubscriptionSupervisor[uint64](subscriptionFixtureInt(t, record.Field("capacity")))
			t.Cleanup(supervisor.close)
			var counter atomic.Uint64
			_, err := supervisor.reconcile(
				EverySubscription(
					"clock",
					time.Duration(subscriptionFixtureInt(t, record.Field("interval")))*time.Millisecond,
					parseSubscriptionPolicy(t, record.Field("policy")),
					func() uint64 { return counter.Add(1) },
				),
				0,
			)
			if err != nil {
				t.Fatal(err)
			}
			var now Timestamp
			var checkpoints []string
			for _, advance := range strings.Split(record.Field("advances"), ",") {
				now = now.Add(time.Duration(subscriptionFixtureInt(t, advance)) * time.Millisecond)
				supervisor.poll(now)
				messages := supervisor.takeReady(1 << 30)
				values := make([]string, len(messages))
				for index, message := range messages {
					values[index] = strconv.FormatUint(message.message, 10)
				}
				if len(values) == 0 {
					values = []string{"-"}
				}
				checkpoints = append(checkpoints, strings.Join(values, ","))
			}
			if actual := strings.Join(checkpoints, "|"); actual != record.Field("checkpoints") {
				t.Fatalf("checkpoints = %q, want %q", actual, record.Field("checkpoints"))
			}
		})
	}
}

func TestReliableSubscriptionBlocksAtCapacityAndWakesOnStop(t *testing.T) {
	started := make(chan SubscriptionSink[string], 1)
	supervisor := newSubscriptionSupervisor[string](1)
	_, err := supervisor.reconcile(
		StreamSubscription("source", ReliableDelivery(), func(ctx context.Context, sink SubscriptionSink[string]) {
			started <- sink
			<-ctx.Done()
		}),
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	sink := waitSubscriptionSink(t, started)
	if !sink.Send("first") {
		t.Fatal("first send failed")
	}
	returned := make(chan bool, 1)
	go func() { returned <- sink.Send("second") }()
	for attempts := 0; attempts < 10_000 && supervisor.diagnostics().BlockedSends() == 0; attempts++ {
		time.Sleep(10 * time.Microsecond)
	}
	select {
	case <-returned:
		t.Fatal("full Reliable inbox did not block")
	default:
	}
	supervisor.poll(0)
	if actual := joinSubscriptionMessages(supervisor.takeReady(1)); actual != "first" {
		t.Fatalf("messages = %q", actual)
	}
	if !<-returned {
		t.Fatal("blocked send failed after capacity became available")
	}
	reconciliation, err := supervisor.reconcile(NoneSubscription[string](), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(reconciliation.stopped) != 1 {
		t.Fatalf("stops = %v", reconciliation.stopped)
	}
}

func TestLatestSubscriptionBurstRemainsBounded(t *testing.T) {
	started := make(chan SubscriptionSink[string], 1)
	supervisor := newSubscriptionSupervisor[string](2)
	t.Cleanup(supervisor.close)
	_, err := supervisor.reconcile(
		StreamSubscription("logs", LatestDelivery(), func(ctx context.Context, sink SubscriptionSink[string]) {
			started <- sink
			<-ctx.Done()
		}),
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	sink := waitSubscriptionSink(t, started)
	for value := 0; value < 10_000; value++ {
		if !sink.Send(strconv.Itoa(value)) {
			t.Fatal("latest sink closed")
		}
	}
	if pending := supervisor.pendingMessages(); pending != 1 {
		t.Fatalf("pending = %d, want 1", pending)
	}
	if replaced := supervisor.diagnostics().LatestReplacements(); replaced != 9_999 {
		t.Fatalf("replacements = %d, want 9999", replaced)
	}
	supervisor.poll(0)
	if actual := joinSubscriptionMessages(supervisor.takeReady(10)); actual != "9999" {
		t.Fatalf("messages = %q, want 9999", actual)
	}
}

func TestSubscriptionSourcesPreserveGlobalArrivalOrder(t *testing.T) {
	firstStarted := make(chan SubscriptionSink[string], 1)
	secondStarted := make(chan SubscriptionSink[string], 1)
	supervisor := newSubscriptionSupervisor[string](4)
	t.Cleanup(supervisor.close)
	_, err := supervisor.reconcile(BatchSubscriptions(
		StreamSubscription("first", ReliableDelivery(), func(ctx context.Context, sink SubscriptionSink[string]) {
			firstStarted <- sink
			<-ctx.Done()
		}),
		StreamSubscription("second", ReliableDelivery(), func(ctx context.Context, sink SubscriptionSink[string]) {
			secondStarted <- sink
			<-ctx.Done()
		}),
	), 0)
	if err != nil {
		t.Fatal(err)
	}
	first := waitSubscriptionSink(t, firstStarted)
	second := waitSubscriptionSink(t, secondStarted)
	first.Send("first-1")
	second.Send("second-1")
	first.Send("first-2")
	supervisor.poll(0)
	if actual := joinSubscriptionMessages(supervisor.takeReady(10)); actual != "first-1,second-1,first-2" {
		t.Fatalf("messages = %q", actual)
	}
}

func TestSubscriptionStreamPanicIsIsolatedAndDiagnosed(t *testing.T) {
	supervisor := newSubscriptionSupervisor[string](2)
	t.Cleanup(supervisor.close)
	_, err := supervisor.reconcile(
		StreamSubscription[string]("panic", ReliableDelivery(), func(context.Context, SubscriptionSink[string]) {
			panic("producer failure")
		}),
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	for attempts := 0; attempts < 10_000 && supervisor.runningStreams() != 0; attempts++ {
		runtime.Gosched()
	}
	if running := supervisor.runningStreams(); running != 0 {
		t.Fatalf("running streams = %d", running)
	}
	if supervisor.activeSubscriptions() != 1 {
		t.Fatalf("active subscriptions = %d", supervisor.activeSubscriptions())
	}
	if panics := supervisor.diagnostics().ProducerPanics(); panics != 1 {
		t.Fatalf("producer panics = %d, want 1", panics)
	}
}

func TestDuplicateSubscriptionKeysAreRejectedBeforeMutation(t *testing.T) {
	supervisor := newSubscriptionSupervisor[string](4)
	_, err := supervisor.reconcile(BatchSubscriptions(
		EverySubscription("duplicate", time.Millisecond, ReliableDelivery(), func() string { return "a" }),
		EverySubscription("duplicate", time.Millisecond, ReliableDelivery(), func() string { return "b" }),
	), 0)
	var duplicate *DuplicateSubscriptionKeyError
	if !errors.As(err, &duplicate) || duplicate.Key != "duplicate" {
		t.Fatalf("error = %v", err)
	}
	if supervisor.activeSubscriptions() != 0 {
		t.Fatalf("active subscriptions = %d", supervisor.activeSubscriptions())
	}
}

func loadSubscriptionFixtures(t *testing.T, path, suite string, fields ...string) []conformance.Record {
	t.Helper()
	records, err := conformance.Load(path, suite, fields...)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	return records
}

func parseSubscriptionDeclaration(t *testing.T, value string) Subscription[string] {
	t.Helper()
	parts := strings.Split(value, ":")
	switch {
	case len(parts) == 4 && parts[0] == "every" && parts[3] == "reliable":
		return EverySubscription(
			SubscriptionKey(parts[1]),
			time.Duration(subscriptionFixtureInt(t, parts[2]))*time.Millisecond,
			ReliableDelivery(),
			func() string { return "" },
		)
	case len(parts) == 3 && parts[0] == "stream" && parts[2] == "reliable":
		return dormantStreamSubscription(parts[1], ReliableDelivery())
	case len(parts) == 3 && parts[0] == "stream" && parts[2] == "latest":
		return dormantStreamSubscription(parts[1], LatestDelivery())
	default:
		t.Fatalf("invalid declaration %q", value)
		return Subscription[string]{}
	}
}

func dormantStreamSubscription(key string, policy DeliveryPolicy) Subscription[string] {
	return StreamSubscription(SubscriptionKey(key), policy, func(ctx context.Context, _ SubscriptionSink[string]) {
		<-ctx.Done()
	})
}

func parseSubscriptionPolicy(t *testing.T, value string) DeliveryPolicy {
	t.Helper()
	parts := strings.Split(value, ":")
	switch {
	case len(parts) == 1 && parts[0] == "reliable":
		return ReliableDelivery()
	case len(parts) == 1 && parts[0] == "latest":
		return LatestDelivery()
	case len(parts) == 3 && parts[0] == "batch":
		return BatchDelivery(
			subscriptionFixtureInt(t, parts[1]),
			time.Duration(subscriptionFixtureInt(t, parts[2]))*time.Millisecond,
		)
	default:
		t.Fatalf("invalid policy %q", value)
		return DeliveryPolicy{}
	}
}

func waitSubscriptionSink[Message any](t *testing.T, started <-chan SubscriptionSink[Message]) SubscriptionSink[Message] {
	t.Helper()
	select {
	case sink := <-started:
		return sink
	case <-time.After(time.Second):
		t.Fatal("subscription stream did not start")
		return SubscriptionSink[Message]{}
	}
}

func joinSubscriptionMessages(messages []subscriptionMessage[string]) string {
	values := make([]string, len(messages))
	for index, message := range messages {
		values[index] = message.message
	}
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ",")
}

func subscriptionFixtureList(value string) []string {
	if value == "-" || value == "" {
		return nil
	}
	return strings.Split(value, ",")
}

func subscriptionFixtureInt(t *testing.T, value string) int {
	t.Helper()
	number, err := strconv.Atoi(value)
	if err != nil {
		t.Fatalf("invalid integer %q", value)
	}
	return number
}
