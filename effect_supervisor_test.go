package tui

import (
	"context"
	"errors"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mayahiro/nagitui-go/internal/conformance"
)

type effectTaskControl struct {
	started   chan context.Context
	complete  chan string
	cancelled chan bool
	returned  chan struct{}
}

func controlledEffectTask() (Task[string], *effectTaskControl) {
	control := &effectTaskControl{
		started:   make(chan context.Context, 1),
		complete:  make(chan string, 1),
		cancelled: make(chan bool, 1),
		returned:  make(chan struct{}, 1),
	}
	return func(ctx context.Context) string {
		control.started <- ctx
		message := <-control.complete
		select {
		case <-ctx.Done():
			control.cancelled <- true
		default:
			control.cancelled <- false
		}
		control.returned <- struct{}{}
		return message
	}, control
}

func waitEffectTaskStarted(t *testing.T, control *effectTaskControl) context.Context {
	t.Helper()
	for range 10_000 {
		select {
		case ctx := <-control.started:
			return ctx
		default:
			runtime.Gosched()
		}
	}
	t.Fatal("controlled effect task did not start")
	return nil
}

func completeEffectTask(t *testing.T, control *effectTaskControl, message string) bool {
	t.Helper()
	control.complete <- message
	<-control.returned
	return <-control.cancelled
}

func TestEffectTaskConstructorsRejectNil(t *testing.T) {
	tests := []struct {
		name      string
		construct func()
	}{
		{name: "run", construct: func() { RunEffect[string](nil) }},
		{name: "latest", construct: func() { LatestEffect[string]("search", nil) }},
		{name: "suspend-terminal", construct: func() { SuspendTerminalEffect[string](nil) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("constructor accepted a nil task")
				}
			}()
			test.construct()
		})
	}
}

func TestAfterEffectTreatsNegativeDelayAsZero(t *testing.T) {
	supervisor := newEffectSupervisor[string](1)
	defer supervisor.close()
	supervisor.schedule(AfterEffect(-time.Second, "ready"), 0)
	supervisor.poll(0)
	if messages := supervisor.takeReady(1); len(messages) != 1 || messages[0] != "ready" {
		t.Fatalf("messages = %v, want [ready]", messages)
	}
}

func TestEffectCompletionNotifiesRuntimeWake(t *testing.T) {
	notified := make(chan struct{}, 1)
	supervisor := newEffectSupervisor[string](1)
	supervisor.wake = func() {
		select {
		case notified <- struct{}{}:
		default:
		}
	}
	defer supervisor.close()
	task, control := controlledEffectTask()
	supervisor.schedule(RunEffect(task), 0)
	waitEffectTaskStarted(t, control)
	completeEffectTask(t, control, "done")

	select {
	case <-notified:
	case <-time.After(time.Second):
		t.Fatal("effect completion did not notify runtime wake")
	}
	supervisor.poll(0)
	if messages := supervisor.takeReady(1); len(messages) != 1 || messages[0] != "done" {
		t.Fatalf("messages = %v, want [done]", messages)
	}
}

func pollEffectSupervisorUntil(
	t *testing.T,
	supervisor *effectSupervisor[string],
	predicate func(*effectSupervisor[string]) bool,
) {
	t.Helper()
	for range 10_000 {
		supervisor.poll(0)
		if predicate(supervisor) {
			return
		}
		runtime.Gosched()
	}
	t.Fatal("effect supervisor did not reach expected state")
}

func TestLatestEffectCancelsOldTaskAndSuppressesStaleResult(t *testing.T) {
	supervisor := newEffectSupervisor[string](2)
	defer supervisor.close()
	oldTask, old := controlledEffectTask()
	newTask, current := controlledEffectTask()
	supervisor.schedule(LatestEffect(TaskKey("search"), oldTask), 0)
	oldContext := waitEffectTaskStarted(t, old)

	supervisor.schedule(LatestEffect(TaskKey("search"), newTask), 0)
	_ = waitEffectTaskStarted(t, current)
	select {
	case <-oldContext.Done():
	default:
		t.Fatal("old latest task did not receive cancellation")
	}
	if generation := supervisor.generation("search"); generation != 2 {
		t.Fatalf("generation = %d, want 2", generation)
	}

	completeEffectTask(t, old, "old")
	completeEffectTask(t, current, "new")
	pollEffectSupervisorUntil(t, supervisor, func(supervisor *effectSupervisor[string]) bool {
		return supervisor.activeTasks() == 0
	})
	if messages := supervisor.takeReady(100); len(messages) != 1 || messages[0] != "new" {
		t.Fatalf("messages = %v, want [new]", messages)
	}
	if supervisor.diagnostics.Cancellations() != 1 || supervisor.diagnostics.StaleResults() != 1 {
		t.Fatalf("diagnostics = %+v", supervisor.diagnostics)
	}
}

func TestScopeCancellationPropagatesWithoutAffectingOtherScopes(t *testing.T) {
	supervisor := newEffectSupervisor[string](2)
	defer supervisor.close()
	cancelledTask, cancelled := controlledEffectTask()
	retainedTask, retained := controlledEffectTask()
	supervisor.schedule(ScopedEffect(ScopeID("screen-a"), RunEffect(cancelledTask)), 0)
	supervisor.schedule(ScopedEffect(ScopeID("screen-b"), RunEffect(retainedTask)), 0)
	cancelledContext := waitEffectTaskStarted(t, cancelled)
	retainedContext := waitEffectTaskStarted(t, retained)

	supervisor.schedule(CancelScopeEffect[string]("screen-a"), 0)
	select {
	case <-cancelledContext.Done():
	default:
		t.Fatal("scoped task did not receive cancellation")
	}
	select {
	case <-retainedContext.Done():
		t.Fatal("unrelated scope was cancelled")
	default:
	}
	completeEffectTask(t, cancelled, "cancelled")
	completeEffectTask(t, retained, "retained")
	pollEffectSupervisorUntil(t, supervisor, func(supervisor *effectSupervisor[string]) bool {
		return supervisor.activeTasks() == 0
	})
	if messages := supervisor.takeReady(100); len(messages) != 1 || messages[0] != "retained" {
		t.Fatalf("messages = %v, want [retained]", messages)
	}
}

func TestVirtualEffectTimersPreserveBatchAndSequenceSemantics(t *testing.T) {
	supervisor := newEffectSupervisor[string](1)
	defer supervisor.close()
	supervisor.schedule(BatchEffects(
		AfterEffect(10*time.Millisecond, "late"),
		AfterEffect(5*time.Millisecond, "early"),
	), 0)
	if deadline, ok := supervisor.timeUntilDeadline(0); !ok || deadline != 5*time.Millisecond {
		t.Fatalf("deadline = %s, %t", deadline, ok)
	}
	supervisor.poll(Timestamp(4_999_999))
	if messages := supervisor.takeReady(100); len(messages) != 0 {
		t.Fatalf("early messages = %v", messages)
	}
	supervisor.poll(Timestamp(10_000_000))
	if messages := supervisor.takeReady(100); len(messages) != 2 || messages[0] != "early" || messages[1] != "late" {
		t.Fatalf("batch messages = %v", messages)
	}

	supervisor.schedule(SequenceEffects(
		AfterEffect(5*time.Millisecond, "first"),
		AfterEffect(0, "second"),
	), Timestamp(10_000_000))
	supervisor.poll(Timestamp(15_000_000))
	if messages := supervisor.takeReady(100); len(messages) != 2 || messages[0] != "first" || messages[1] != "second" {
		t.Fatalf("sequence messages = %v", messages)
	}
}

func TestEffectTaskLimitDefersBatchWorkers(t *testing.T) {
	supervisor := newEffectSupervisor[string](1)
	defer supervisor.close()
	firstTask, first := controlledEffectTask()
	secondTask, second := controlledEffectTask()
	supervisor.schedule(BatchEffects(RunEffect(firstTask), RunEffect(secondTask)), 0)
	_ = waitEffectTaskStarted(t, first)
	select {
	case <-second.started:
		t.Fatal("second task started before a worker slot was available")
	default:
	}
	if supervisor.runningTasks() != 1 || supervisor.pendingTasks() != 1 {
		t.Fatalf("running = %d, pending = %d", supervisor.runningTasks(), supervisor.pendingTasks())
	}

	completeEffectTask(t, first, "first")
	pollEffectSupervisorUntil(t, supervisor, func(supervisor *effectSupervisor[string]) bool {
		return supervisor.pendingTasks() == 0
	})
	_ = waitEffectTaskStarted(t, second)
	completeEffectTask(t, second, "second")
	pollEffectSupervisorUntil(t, supervisor, func(supervisor *effectSupervisor[string]) bool {
		return supervisor.activeTasks() == 0
	})
	if messages := supervisor.takeReady(100); len(messages) != 2 || messages[0] != "first" || messages[1] != "second" {
		t.Fatalf("messages = %v", messages)
	}
}

func TestEffectTaskPanicBecomesDiagnosticAndDoesNotStallSequence(t *testing.T) {
	supervisor := newEffectSupervisor[string](1)
	defer supervisor.close()
	supervisor.schedule(SequenceEffects(
		RunEffect(func(context.Context) string { panic("task failure") }),
		AfterEffect(0, "recovered"),
	), 0)
	pollEffectSupervisorUntil(t, supervisor, func(supervisor *effectSupervisor[string]) bool {
		return supervisor.diagnostics.TaskPanics() == 1
	})
	supervisor.poll(0)
	if messages := supervisor.takeReady(100); len(messages) != 1 || messages[0] != "recovered" {
		t.Fatalf("messages = %v, want [recovered]", messages)
	}
}

func TestTerminalSuspensionMatchesSharedFixtures(t *testing.T) {
	records, err := conformance.Load(
		"effects/terminal-suspend.txt",
		"effect-terminal-suspend",
		"mode",
		"pending",
		"expected",
		"panics",
		"cancellations",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			supervisor := newEffectSupervisor[string](1)
			defer supervisor.close()
			switch record.Field("mode") {
			case "sequence":
				supervisor.schedule(SequenceEffects(
					SuspendTerminalEffect(func(context.Context) string { return "first" }),
					SuspendTerminalEffect(func(context.Context) string { return "second" }),
				), 0)
			case "batch":
				supervisor.schedule(BatchEffects(
					SuspendTerminalEffect(func(context.Context) string { return "first" }),
					SuspendTerminalEffect(func(context.Context) string { return "second" }),
				), 0)
			case "panic-sequence":
				supervisor.schedule(SequenceEffects(
					SuspendTerminalEffect(func(context.Context) string { panic("terminal task failure") }),
					AfterEffect(0, "recovered"),
				), 0)
			case "scoped-cancel":
				supervisor.schedule(ScopedEffect(
					"external",
					SuspendTerminalEffect(func(context.Context) string { return "cancelled" }),
				), 0)
				supervisor.schedule(CancelScopeEffect[string]("external"), 0)
			default:
				t.Fatalf("invalid terminal effect mode %q", record.Field("mode"))
			}

			pending := []string{strconv.Itoa(supervisor.pendingTerminalTasks())}
			for supervisor.runTerminalTask(0) {
				pending = append(pending, strconv.Itoa(supervisor.pendingTerminalTasks()))
				supervisor.poll(0)
			}
			supervisor.poll(0)

			effectAssertList(t, "pending", pending, strings.Split(record.Field("pending"), ","))
			effectAssertList(t, "messages", supervisor.takeReady(1<<30), effectFixtureList(record.Field("expected")))
			if got, want := supervisor.diagnostics.TaskPanics(), effectFixtureNumber(t, record.Field("panics")); got != want {
				t.Fatalf("task panics = %d, want %d", got, want)
			}
			if got, want := supervisor.diagnostics.Cancellations(), effectFixtureNumber(t, record.Field("cancellations")); got != want {
				t.Fatalf("cancellations = %d, want %d", got, want)
			}
		})
	}
}

func TestRepeatedTerminalTaskCancellationDoesNotRetainQueueEntries(t *testing.T) {
	supervisor := newEffectSupervisor[struct{}](1)
	defer supervisor.close()
	for range 10_000 {
		supervisor.schedule(ScopedEffect(
			"external",
			SuspendTerminalEffect(func(context.Context) struct{} { return struct{}{} }),
		), 0)
		supervisor.schedule(CancelScopeEffect[struct{}]("external"), 0)
	}

	if len(supervisor.terminalTasks) != 0 || len(supervisor.pendingTerminal) != 0 || supervisor.pendingTerminalTasks() != 0 {
		t.Fatalf("terminal tasks = %d, queue = %d", len(supervisor.terminalTasks), len(supervisor.pendingTerminal))
	}
}

func TestLatestEffectGenerationsMatchSharedFixtures(t *testing.T) {
	runControlledEffectFixtures(
		t,
		"effects/generation.txt",
		"effect-generation",
		[]string{"actions", "expected", "cancelled", "generations", "stale"},
		true,
	)
}

func TestEffectScopeCancellationMatchesSharedFixtures(t *testing.T) {
	runControlledEffectFixtures(
		t,
		"effects/cancellation.txt",
		"effect-cancellation",
		[]string{"actions", "expected", "cancelled", "stale"},
		false,
	)
}

func TestEffectVirtualTimeMatchesSharedFixtures(t *testing.T) {
	records, err := conformance.Load(
		"effects/virtual-time.txt",
		"effect-virtual-time",
		"mode",
		"effects",
		"advances",
		"checkpoints",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			clock := NewVirtualClock()
			effects := make([]Effect[string], 0)
			for _, item := range strings.Split(record.Field("effects"), ",") {
				delay, message, ok := strings.Cut(item, ":")
				if !ok {
					t.Fatalf("invalid timer %q", item)
				}
				effects = append(effects, AfterEffect(time.Duration(effectFixtureNumber(t, delay))*time.Millisecond, message))
			}
			var effect Effect[string]
			switch record.Field("mode") {
			case "batch":
				effect = BatchEffects(effects...)
			case "sequence":
				effect = SequenceEffects(effects...)
			default:
				t.Fatalf("invalid mode %q", record.Field("mode"))
			}
			supervisor := newEffectSupervisor[string](1)
			defer supervisor.close()
			supervisor.schedule(effect, clock.Now())
			advances := strings.Split(record.Field("advances"), ",")
			checkpoints := strings.Split(record.Field("checkpoints"), "|")
			if len(advances) != len(checkpoints) {
				t.Fatalf("%d advances, %d checkpoints", len(advances), len(checkpoints))
			}
			var delivered []string
			for index, advance := range advances {
				clock.Advance(time.Duration(effectFixtureNumber(t, advance)) * time.Millisecond)
				supervisor.poll(clock.Now())
				delivered = append(delivered, supervisor.takeReady(1<<30)...)
				effectAssertList(t, "checkpoint", delivered, effectFixtureList(checkpoints[index]))
			}
		})
	}
}

type fixtureEffectControl struct {
	control   *effectTaskControl
	cancelled bool
}

func runControlledEffectFixtures(
	t *testing.T,
	path, suite string,
	fields []string,
	checkGenerations bool,
) {
	t.Helper()
	records, err := conformance.Load(path, suite, fields...)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			supervisor := newEffectSupervisor[string](16)
			defer supervisor.close()
			controls := make(map[string]*fixtureEffectControl)
			for _, action := range strings.Split(record.Field("actions"), ";") {
				parts := strings.Split(action, ":")
				switch {
				case len(parts) == 3 && parts[0] == "latest":
					task, control := controlledEffectTask()
					supervisor.schedule(LatestEffect(TaskKey(parts[1]), task), 0)
					_ = waitEffectTaskStarted(t, control)
					controls[parts[2]] = &fixtureEffectControl{control: control}
				case len(parts) == 3 && parts[0] == "scoped":
					task, control := controlledEffectTask()
					supervisor.schedule(ScopedEffect(ScopeID(parts[1]), RunEffect(task)), 0)
					_ = waitEffectTaskStarted(t, control)
					controls[parts[2]] = &fixtureEffectControl{control: control}
				case len(parts) == 3 && parts[0] == "complete":
					active := supervisor.activeTasks()
					control := controls[parts[1]]
					control.cancelled = completeEffectTask(t, control.control, parts[2])
					pollEffectSupervisorUntil(t, supervisor, func(supervisor *effectSupervisor[string]) bool {
						return supervisor.activeTasks() < active
					})
				case len(parts) == 2 && parts[0] == "cancel":
					supervisor.schedule(CancelEffect[string](TaskKey(parts[1])), 0)
				case len(parts) == 2 && parts[0] == "cancel-scope":
					supervisor.schedule(CancelScopeEffect[string](ScopeID(parts[1])), 0)
				default:
					t.Fatalf("invalid action %q", action)
				}
			}
			effectAssertList(t, "messages", supervisor.takeReady(1<<30), effectFixtureList(record.Field("expected")))
			var cancelled []string
			for name, control := range controls {
				if control.cancelled {
					cancelled = append(cancelled, name)
				}
			}
			sort.Strings(cancelled)
			effectAssertList(t, "cancelled", cancelled, effectFixtureList(record.Field("cancelled")))
			if expected := effectFixtureNumber(t, record.Field("stale")); supervisor.diagnostics.StaleResults() != expected {
				t.Fatalf("stale results = %d, want %d", supervisor.diagnostics.StaleResults(), expected)
			}
			if checkGenerations {
				for _, item := range strings.Split(record.Field("generations"), ",") {
					key, generation, ok := strings.Cut(item, ":")
					if !ok {
						t.Fatalf("invalid generation %q", item)
					}
					if actual, expected := supervisor.generation(TaskKey(key)), effectFixtureNumber(t, generation); actual != expected {
						t.Fatalf("generation %s = %d, want %d", key, actual, expected)
					}
				}
			}
		})
	}
}

func effectFixtureList(value string) []string {
	if value == "" || value == "-" {
		return nil
	}
	return strings.Split(value, ",")
}

func effectFixtureNumber(t *testing.T, value string) uint64 {
	t.Helper()
	number, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		t.Fatalf("parse %q: %v", value, err)
	}
	return number
}

func effectAssertList(t *testing.T, name string, actual, expected []string) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Fatalf("%s = %v, want %v", name, actual, expected)
	}
	for index := range actual {
		if actual[index] != expected[index] {
			t.Fatalf("%s = %v, want %v", name, actual, expected)
		}
	}
}
