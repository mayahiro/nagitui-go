package tui

import (
	"testing"

	"github.com/mayahiro/nagi-go/vt"
)

type virtualFlowRuntimeEntry struct {
	key    NodeID
	label  string
	height uint32
}

type virtualFlowRuntimeApp struct {
	entries   []virtualFlowRuntimeEntry
	items     VirtualFlowItems
	update    VirtualFlowUpdate
	stick     bool
	buildKeys []NodeID
}

func newVirtualFlowRuntimeApp(
	entries []virtualFlowRuntimeEntry,
	stick bool,
) *virtualFlowRuntimeApp {
	return &virtualFlowRuntimeApp{
		entries: append([]virtualFlowRuntimeEntry(nil), entries...),
		items:   virtualFlowRuntimeItems(entries),
		update:  ResetVirtualFlowUpdate(1),
		stick:   stick,
	}
}

func (*virtualFlowRuntimeApp) Init() Effect[struct{}] { return NoneEffect[struct{}]() }

func (*virtualFlowRuntimeApp) Subscriptions() Subscription[struct{}] {
	return NoneSubscription[struct{}]()
}

func (*virtualFlowRuntimeApp) Update(struct{}) Effect[struct{}] {
	return NoneEffect[struct{}]()
}

func (a *virtualFlowRuntimeApp) View(ViewContext) Node[struct{}] {
	entries := append([]virtualFlowRuntimeEntry(nil), a.entries...)
	source := NewVirtualFlowSource(a.items, func(context VirtualFlowItemContext) Node[struct{}] {
		a.buildKeys = append(a.buildKeys, context.Key)
		for _, entry := range entries {
			if entry.key == context.Key {
				return virtualFlowRuntimeItemNode(entry)
			}
		}
		panic("built virtual flow item is missing")
	}).EstimatedHeight(func(context VirtualFlowItemContext) uint32 {
		for _, entry := range entries {
			if entry.key == context.Key {
				return entry.height
			}
		}
		panic("estimated virtual flow item is missing")
	}).Update(a.update)
	options := DefaultVirtualFlowOptions[struct{}]()
	options.Overscan = 1
	options.StickToEnd = a.stick
	return VirtualFlowWithOptions("virtual-flow", source, options)
}

func (a *virtualFlowRuntimeApp) replaceEntries(
	entries []virtualFlowRuntimeEntry,
	update VirtualFlowUpdate,
) {
	a.entries = append([]virtualFlowRuntimeEntry(nil), entries...)
	a.items = virtualFlowRuntimeItems(entries)
	a.update = update
}

func virtualFlowRuntimeItems(entries []virtualFlowRuntimeEntry) VirtualFlowItems {
	items := make([]VirtualFlowItem, len(entries))
	for index, entry := range entries {
		items[index] = NewVirtualFlowItem(entry.key)
	}
	order, err := NewVirtualFlowItems(items)
	if err != nil {
		panic(err)
	}
	return order
}

func virtualFlowRuntimeItemNode(entry virtualFlowRuntimeEntry) Node[struct{}] {
	rows := make([]Node[struct{}], entry.height)
	for index := range rows {
		rows[index] = Text[struct{}](entry.label)
	}
	return Column(rows...).WithID(NodeID("item-" + entry.key.String()))
}

func TestVirtualFlowMeasuresOnceAndPreservesPrependAnchor(t *testing.T) {
	app := newVirtualFlowRuntimeApp([]virtualFlowRuntimeEntry{
		{key: "a", label: "A", height: 2},
		{key: "b", label: "B", height: 3},
		{key: "c", label: "C", height: 1},
		{key: "d", label: "D", height: 2},
	}, false)
	runtime, err := NewRuntimeWithClock[struct{}](
		app,
		NewRuntimeConfig(Size{Width: 4, Height: 3}),
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
	assertNodeCell(t, initial.Surface(), 0, 0, "A")
	assertNodeCell(t, initial.Surface(), 0, 2, "B")
	assertVirtualFlowBuildKeys(t, app.buildKeys, []NodeID{"a", "b"})
	state, ok := runtime.Interaction().VirtualFlowState("virtual-flow")
	if !ok || state.VisibleStart != 0 || state.VisibleEnd != 2 {
		t.Fatalf("initial virtual flow state = %+v, present = %t", state, ok)
	}

	app.buildKeys = nil
	if !runtime.SetScrollOffset("virtual-flow", ScrollOffset{Y: 2}) {
		t.Fatal("SetScrollOffset returned false")
	}
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	assertVirtualFlowBuildKeys(t, app.buildKeys, []NodeID{"a", "b", "c"})

	app.buildKeys = nil
	app.replaceEntries([]virtualFlowRuntimeEntry{
		{key: "x", label: "X", height: 4},
		{key: "a", label: "A", height: 2},
		{key: "b", label: "B", height: 3},
		{key: "c", label: "C", height: 1},
		{key: "d", label: "D", height: 2},
	}, ChangedVirtualFlowUpdate(2, 1, 0, 1))
	runtime.RequestFrame()
	prepended, err := runtime.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}

	assertNodeCell(t, prepended.Surface(), 0, 0, "B")
	state, ok = runtime.Interaction().VirtualFlowState("virtual-flow")
	if !ok || state.Scroll.Offset != (ScrollOffset{Y: 6}) ||
		!state.HasAnchor || state.Anchor.Key != "b" {
		t.Fatalf("prepended virtual flow state = %+v, present = %t", state, ok)
	}
	assertVirtualFlowBuildKeys(t, app.buildKeys, []NodeID{"a", "b", "c"})
}

func TestVirtualFlowFollowsStreamingTailGrowth(t *testing.T) {
	app := newVirtualFlowRuntimeApp([]virtualFlowRuntimeEntry{
		{key: "a", label: "A", height: 2},
		{key: "b", label: "B", height: 2},
		{key: "c", label: "C", height: 1},
	}, true)
	runtime, err := NewRuntimeWithClock[struct{}](
		app,
		NewRuntimeConfig(Size{Width: 4, Height: 3}),
		NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}

	app.replaceEntries([]virtualFlowRuntimeEntry{
		{key: "a", label: "A", height: 2},
		{key: "b", label: "B", height: 2},
		{key: "c", label: "C2", height: 4},
	}, ChangedVirtualFlowUpdate(2, 1, 2, 3))
	runtime.RequestFrame()
	streamed, err := runtime.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}

	assertNodeCell(t, streamed.Surface(), 0, 0, "C")
	assertNodeCell(t, streamed.Surface(), 1, 0, "2")
	state, ok := runtime.Interaction().VirtualFlowState("virtual-flow")
	if !ok || state.Scroll.Offset != (ScrollOffset{Y: 5}) ||
		state.Scroll.Maximum != (ScrollOffset{Y: 5}) || !state.Scroll.AtEnd {
		t.Fatalf("streaming virtual flow state = %+v, present = %t", state, ok)
	}
}

type virtualFlowScrollMessage struct {
	state ScrollState
}

type virtualFlowScrollApp struct {
	items    VirtualFlowItems
	observed []ScrollState
}

func (*virtualFlowScrollApp) Init() Effect[virtualFlowScrollMessage] {
	return NoneEffect[virtualFlowScrollMessage]()
}

func (*virtualFlowScrollApp) Subscriptions() Subscription[virtualFlowScrollMessage] {
	return NoneSubscription[virtualFlowScrollMessage]()
}

func (a *virtualFlowScrollApp) Update(message virtualFlowScrollMessage) Effect[virtualFlowScrollMessage] {
	a.observed = append(a.observed, message.state)
	return NoneEffect[virtualFlowScrollMessage]()
}

func (a *virtualFlowScrollApp) View(ViewContext) Node[virtualFlowScrollMessage] {
	source := NewVirtualFlowSource(a.items, func(context VirtualFlowItemContext) Node[virtualFlowScrollMessage] {
		return Text[virtualFlowScrollMessage](context.Key.String())
	})
	options := DefaultVirtualFlowOptions[virtualFlowScrollMessage]()
	options.OnScroll = func(state ScrollState) virtualFlowScrollMessage {
		return virtualFlowScrollMessage{state: state}
	}
	return VirtualFlowWithOptions("virtual-flow", source, options)
}

func TestVirtualFlowRoutesCoreScrollActionsAndUserCallback(t *testing.T) {
	items, err := NewVirtualFlowItems([]VirtualFlowItem{
		NewVirtualFlowItem("a"), NewVirtualFlowItem("b"), NewVirtualFlowItem("c"),
		NewVirtualFlowItem("d"), NewVirtualFlowItem("e"),
	})
	if err != nil {
		t.Fatal(err)
	}
	app := &virtualFlowScrollApp{items: items}
	runtime, err := NewRuntimeWithClock[virtualFlowScrollMessage](
		app,
		NewRuntimeConfig(Size{Width: 4, Height: 2}),
		NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.RequestFocus("virtual-flow"); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}

	dispatched, err := runtime.DispatchEvent(vt.Event{
		Kind: vt.EventKey,
		Key:  vt.KeyEvent{Code: vt.KeyPageDown},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !dispatched.Consumed() {
		t.Fatal("PageDown was not consumed")
	}
	if _, err := runtime.ProcessPending(); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}

	if len(app.observed) != 1 || app.observed[0].Offset != (ScrollOffset{Y: 2}) {
		t.Fatalf("observed scroll states = %+v", app.observed)
	}
	state, ok := runtime.Interaction().VirtualFlowState("virtual-flow")
	if !ok || state.Scroll.Offset != (ScrollOffset{Y: 2}) {
		t.Fatalf("virtual flow state = %+v, present = %t", state, ok)
	}
}

func TestVirtualFlowRetainedMemoryStabilizes(t *testing.T) {
	uiRuntime, err := NewRuntimeWithClock[struct{}](
		newVirtualFlowBenchmarkApp(),
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

	renderVirtualFlowFrames(t, uiRuntime, 1, 256)
	baseline := heapAllocationAfterGC()
	renderVirtualFlowFrames(t, uiRuntime, 10_000, 256)
	retained := heapAllocationAfterGC()
	const tolerance = uint64(1 << 20)
	if retained > baseline+tolerance {
		t.Fatalf("retained heap grew from %d to %d bytes", baseline, retained)
	}
	if _, ok := uiRuntime.Interaction().VirtualFlowState("virtual-flow"); !ok {
		t.Fatal("virtual flow state was retired while active")
	}
}

func renderVirtualFlowFrames(
	t *testing.T,
	uiRuntime *Runtime[struct{}],
	start, count uint32,
) {
	t.Helper()
	for offset := start; offset < start+count; offset++ {
		if !uiRuntime.SetScrollOffset("virtual-flow", ScrollOffset{Y: offset}) {
			t.Fatal("SetScrollOffset returned false")
		}
		if frame, err := uiRuntime.RenderIfDirty(); err != nil || frame == nil {
			t.Fatalf("virtual flow frame %d = %v, %v", offset, frame, err)
		}
	}
}

func assertVirtualFlowBuildKeys(t *testing.T, actual, expected []NodeID) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Fatalf("build keys = %v, want %v", actual, expected)
	}
	for index := range actual {
		if actual[index] != expected[index] {
			t.Fatalf("build keys = %v, want %v", actual, expected)
		}
	}
}
