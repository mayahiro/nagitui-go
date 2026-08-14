package widget

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	tui "github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/internal/conformance"
)

type statusBarFixtureApp struct {
	labels     []string
	placements []tui.ResponsiveRowPlacement
	priorities []StatusBarPriority
	gap        uint32
}

func (*statusBarFixtureApp) Init() tui.Effect[struct{}] {
	return tui.NoneEffect[struct{}]()
}

func (*statusBarFixtureApp) Update(struct{}) tui.Effect[struct{}] {
	return tui.NoneEffect[struct{}]()
}

func (*statusBarFixtureApp) Subscriptions() tui.Subscription[struct{}] {
	return tui.NoneSubscription[struct{}]()
}

func (a *statusBarFixtureApp) View(tui.ViewContext) tui.Node[struct{}] {
	slots := make([]StatusBarSlot[struct{}], len(a.labels))
	for index, label := range a.labels {
		slots[index] = NewStatusBarSlot(tui.Text[struct{}](label)).
			Placement(a.placements[index]).
			Priority(a.priorities[index])
	}
	return NewStatusBar(slots).Gap(a.gap).Node()
}

func TestStatusBarMatchesSharedFixtures(t *testing.T) {
	records, err := conformance.Load(
		"widgets/status-bar.txt",
		"widget-status-bar",
		"width", "gap", "labels", "placements", "priorities", "expected",
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
			labels := strings.Split(record.Field("labels"), ",")
			placementFields := strings.Split(record.Field("placements"), ",")
			priorityFields := strings.Split(record.Field("priorities"), ",")
			placements := make([]tui.ResponsiveRowPlacement, len(placementFields))
			priorities := make([]StatusBarPriority, len(priorityFields))
			for index := range placementFields {
				placements[index] = fixtureStatusBarPlacement(t, placementFields[index])
				priorities[index] = fixtureStatusBarPriority(t, priorityFields[index])
			}
			width := fixtureStatusBarUint(t, record.Field("width"))
			runtime, err := tui.NewRuntimeWithClock(
				&statusBarFixtureApp{
					labels: labels, placements: placements, priorities: priorities,
					gap: fixtureStatusBarUint(t, record.Field("gap")),
				},
				tui.NewRuntimeConfig(tui.Size{Width: width, Height: 1}),
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
			if actual, expected := widgetRowText(frame.Surface(), 0), record.Text("expected"); actual != expected {
				t.Fatalf("row = %q, want %q", actual, expected)
			}
		})
	}
}

type statusBarOneRowApp struct{}

func (*statusBarOneRowApp) Init() tui.Effect[struct{}] {
	return tui.NoneEffect[struct{}]()
}

func (*statusBarOneRowApp) Update(struct{}) tui.Effect[struct{}] {
	return tui.NoneEffect[struct{}]()
}

func (*statusBarOneRowApp) Subscriptions() tui.Subscription[struct{}] {
	return tui.NoneSubscription[struct{}]()
}

func (*statusBarOneRowApp) View(tui.ViewContext) tui.Node[struct{}] {
	return NewStatusBar([]StatusBarSlot[struct{}]{
		NewStatusBarSlot(tui.Text[struct{}]("first\nsecond")),
	}).Node()
}

func TestStatusBarIsOneRowEvenAsTheRoot(t *testing.T) {
	runtime, err := tui.NewRuntimeWithClock(
		&statusBarOneRowApp{}, tui.NewRuntimeConfig(tui.Size{Width: 8, Height: 3}), tui.NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	frame, err := runtime.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	first, _ := frame.Surface().Cell(0, 0)
	second, _ := frame.Surface().Cell(0, 1)
	if first.Content() != "f" || second.Content() != " " {
		t.Fatalf("first rows = %q, %q, want f and blank", first.Content(), second.Content())
	}
}

func TestToastVisibleBodySelectionMatchesSharedFixtures(t *testing.T) {
	records, err := conformance.Load(
		"widgets/toast.txt", "widget-toast", "count", "limit", "built",
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
			var built []int
			count := int(fixtureStatusBarUint(t, record.Field("count")))
			toasts := make([]Toast[struct{}], count)
			for index := range toasts {
				index := index
				toasts[index] = NewToast(tui.NodeID("toast-"+strconv.Itoa(index)), func() tui.Node[struct{}] {
					built = append(built, index)
					return tui.Text[struct{}](strconv.Itoa(index))
				})
			}
			_ = NewToastRegion(tui.Text[struct{}]("base"), toasts).
				VisibleLimit(int(fixtureStatusBarUint(t, record.Field("limit")))).
				Node()
			expected := fixtureToastIndices(t, record.Field("built"))
			if len(built) != len(expected) {
				t.Fatalf("built = %v, want %v", built, expected)
			}
			for index := range built {
				if built[index] != expected[index] {
					t.Fatalf("built = %v, want %v", built, expected)
				}
			}
		})
	}
}

func TestStatusBarAndToastUnknownEnumsNormalize(t *testing.T) {
	slot := NewStatusBarSlot(tui.Text[struct{}]("x")).Priority(StatusBarPriority(255))
	if slot.ConfiguredPriority() != StatusBarNormal {
		t.Fatalf("priority = %v, want normal", slot.ConfiguredPriority())
	}
	toast := NewToast[struct{}]("toast", nil).Tone(ToastTone(255))
	if toast.ConfiguredTone() != ToastNeutral {
		t.Fatalf("tone = %v, want neutral", toast.ConfiguredTone())
	}
	region := NewToastRegion(tui.Text[struct{}]("base"), []Toast[struct{}]{toast}).
		Placement(ToastPlacement(255))
	_ = region.Node()
}

type toastExpiryMessage struct {
	show       bool
	generation uint64
}

type toastExpiryApp struct {
	current  uint64
	hasToast bool
}

func (*toastExpiryApp) Init() tui.Effect[toastExpiryMessage] {
	return tui.NoneEffect[toastExpiryMessage]()
}

func (a *toastExpiryApp) Update(message toastExpiryMessage) tui.Effect[toastExpiryMessage] {
	if message.show {
		a.current = message.generation
		a.hasToast = true
		return tui.AfterEffect(10*time.Millisecond, toastExpiryMessage{generation: message.generation})
	}
	if a.hasToast && a.current == message.generation {
		a.hasToast = false
	}
	return tui.NoneEffect[toastExpiryMessage]()
}

func (*toastExpiryApp) Subscriptions() tui.Subscription[toastExpiryMessage] {
	return tui.NoneSubscription[toastExpiryMessage]()
}

func (a *toastExpiryApp) View(tui.ViewContext) tui.Node[toastExpiryMessage] {
	var toasts []Toast[toastExpiryMessage]
	if a.hasToast {
		generation := a.current
		toasts = append(toasts, NewToast("toast", func() tui.Node[toastExpiryMessage] {
			return tui.Text[toastExpiryMessage](strconv.FormatUint(generation, 10))
		}))
	}
	return NewToastRegion(tui.Text[toastExpiryMessage]("base"), toasts).Node()
}

func TestToastApplicationGenerationIgnoresStaleVirtualTimeout(t *testing.T) {
	clock := tui.NewVirtualClock()
	app := &toastExpiryApp{}
	runtime, err := tui.NewRuntimeWithClock(
		app, tui.NewRuntimeConfig(tui.Size{Width: 8, Height: 3}), clock,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()

	if err := runtime.Enqueue(toastExpiryMessage{show: true, generation: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ProcessPending(); err != nil {
		t.Fatal(err)
	}
	clock.Advance(5 * time.Millisecond)
	if err := runtime.Enqueue(toastExpiryMessage{show: true, generation: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ProcessPending(); err != nil {
		t.Fatal(err)
	}
	clock.Advance(5 * time.Millisecond)
	if _, err := runtime.ProcessPending(); err != nil {
		t.Fatal(err)
	}
	if !app.hasToast || app.current != 2 {
		t.Fatalf("after stale timeout: current=%d visible=%t, want 2 true", app.current, app.hasToast)
	}

	clock.Advance(5 * time.Millisecond)
	if _, err := runtime.ProcessPending(); err != nil {
		t.Fatal(err)
	}
	if app.hasToast {
		t.Fatal("current timeout did not remove Toast")
	}
}

type toastPlacementApp struct{}

func (*toastPlacementApp) Init() tui.Effect[struct{}] { return tui.NoneEffect[struct{}]() }

func (*toastPlacementApp) Update(struct{}) tui.Effect[struct{}] {
	return tui.NoneEffect[struct{}]()
}

func (*toastPlacementApp) Subscriptions() tui.Subscription[struct{}] {
	return tui.NoneSubscription[struct{}]()
}

func (*toastPlacementApp) View(tui.ViewContext) tui.Node[struct{}] {
	toast := NewToast("toast", func() tui.Node[struct{}] { return tui.Text[struct{}]("X") })
	return NewToastRegion(tui.Text[struct{}]("base"), []Toast[struct{}]{toast}).
		Placement(ToastBottomEnd).
		Node()
}

func TestToastBottomEndPlacementUsesBaseRectangle(t *testing.T) {
	runtime, err := tui.NewRuntimeWithClock(
		&toastPlacementApp{}, tui.NewRuntimeConfig(tui.Size{Width: 8, Height: 5}), tui.NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	frame, err := runtime.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []struct {
		x, y    int32
		content string
	}{{5, 2, "┌"}, {6, 3, "X"}, {7, 4, "┘"}} {
		cell, ok := frame.Surface().Cell(expected.x, expected.y)
		if !ok || cell.Content() != expected.content {
			t.Fatalf("cell %d,%d = %#v, %t, want %q", expected.x, expected.y, cell, ok, expected.content)
		}
	}
}

func fixtureStatusBarPlacement(t *testing.T, value string) tui.ResponsiveRowPlacement {
	t.Helper()
	switch value {
	case "start":
		return tui.ResponsiveRowStart
	case "center":
		return tui.ResponsiveRowCenter
	case "end":
		return tui.ResponsiveRowEnd
	default:
		t.Fatalf("invalid placement %q", value)
		return 0
	}
}

func fixtureStatusBarPriority(t *testing.T, value string) StatusBarPriority {
	t.Helper()
	switch value {
	case "low":
		return StatusBarLow
	case "normal":
		return StatusBarNormal
	case "high":
		return StatusBarHigh
	case "critical":
		return StatusBarCritical
	default:
		t.Fatalf("invalid priority %q", value)
		return 0
	}
}

func fixtureStatusBarUint(t *testing.T, value string) uint32 {
	t.Helper()
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		t.Fatal(err)
	}
	return uint32(parsed)
}

func fixtureToastIndices(t *testing.T, value string) []int {
	t.Helper()
	if value == "-" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]int, len(parts))
	for index, part := range parts {
		parsed, err := strconv.Atoi(part)
		if err != nil {
			t.Fatal(err)
		}
		result[index] = parsed
	}
	return result
}
