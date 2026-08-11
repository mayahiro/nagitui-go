package tui

import (
	"strconv"
	"testing"
)

const benchmarkRows = 100_000

type eagerViewportBenchmarkApp struct{}

type virtualViewportBenchmarkApp struct{}

type growingVirtualViewportBenchmarkApp struct {
	contentHeight uint32
}

type identifiedVirtualViewportBenchmarkApp struct {
	rowIDs []NodeID
}

type virtualFlowBenchmarkApp struct {
	items VirtualFlowItems
}

func warmViewportBenchmark(b *testing.B, runtime *Runtime[struct{}]) {
	b.Helper()
	for range 2 {
		runtime.RequestFrame()
		if _, err := runtime.RenderIfDirty(); err != nil {
			b.Fatal(err)
		}
	}
}

func (*eagerViewportBenchmarkApp) Init() Effect[struct{}] {
	return NoneEffect[struct{}]()
}

func (*eagerViewportBenchmarkApp) Update(struct{}) Effect[struct{}] {
	return NoneEffect[struct{}]()
}

func (*eagerViewportBenchmarkApp) Subscriptions() Subscription[struct{}] {
	return NoneSubscription[struct{}]()
}

func (*eagerViewportBenchmarkApp) View(ViewContext) Node[struct{}] {
	rows := make([]Node[struct{}], benchmarkRows)
	for index := range rows {
		rows[index] = Text[struct{}]("row")
	}
	return ScrollViewport("viewport", Column(rows...))
}

func BenchmarkScrollViewportEager100K(b *testing.B) {
	runtime, err := NewRuntime[struct{}](&eagerViewportBenchmarkApp{}, Size{Width: 80, Height: 24})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(runtime.Close)
	warmViewportBenchmark(b, runtime)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		runtime.RequestFrame()
		if _, err := runtime.RenderIfDirty(); err != nil {
			b.Fatal(err)
		}
	}
}

func (*virtualViewportBenchmarkApp) Init() Effect[struct{}] {
	return NoneEffect[struct{}]()
}

func (*virtualViewportBenchmarkApp) Update(struct{}) Effect[struct{}] {
	return NoneEffect[struct{}]()
}

func (*virtualViewportBenchmarkApp) Subscriptions() Subscription[struct{}] {
	return NoneSubscription[struct{}]()
}

func (*virtualViewportBenchmarkApp) View(ViewContext) Node[struct{}] {
	return VirtualScrollViewport(
		"viewport",
		Size{Width: 80, Height: benchmarkRows},
		func(viewport VirtualViewport) VirtualFragment[struct{}] {
			rows := make([]Node[struct{}], viewport.Size.Height)
			for index := range rows {
				rows[index] = Text[struct{}]("row")
			}
			return NewVirtualFragment(
				ScrollOffset{Y: viewport.Offset.Y},
				Column(rows...),
			)
		},
	)
}

func BenchmarkScrollViewportVirtual100K(b *testing.B) {
	runtime, err := NewRuntime[struct{}](&virtualViewportBenchmarkApp{}, Size{Width: 80, Height: 24})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(runtime.Close)
	warmViewportBenchmark(b, runtime)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		runtime.RequestFrame()
		if _, err := runtime.RenderIfDirty(); err != nil {
			b.Fatal(err)
		}
	}
}

func (*growingVirtualViewportBenchmarkApp) Init() Effect[struct{}] {
	return NoneEffect[struct{}]()
}

func (*growingVirtualViewportBenchmarkApp) Update(struct{}) Effect[struct{}] {
	return NoneEffect[struct{}]()
}

func (*growingVirtualViewportBenchmarkApp) Subscriptions() Subscription[struct{}] {
	return NoneSubscription[struct{}]()
}

func (a *growingVirtualViewportBenchmarkApp) View(ViewContext) Node[struct{}] {
	return VirtualScrollViewportWithOptions(
		"viewport",
		Size{Width: 80, Height: a.contentHeight},
		ScrollViewportOptions[struct{}]{Axis: ScrollAxisVertical, StickToEnd: true},
		func(viewport VirtualViewport) VirtualFragment[struct{}] {
			rows := make([]Node[struct{}], viewport.Size.Height)
			for index := range rows {
				rows[index] = Text[struct{}]("row")
			}
			return NewVirtualFragment(
				ScrollOffset{Y: viewport.Offset.Y},
				Column(rows...),
			)
		},
	)
}

func BenchmarkScrollViewportVirtualStickToEndGrowth100K(b *testing.B) {
	app := &growingVirtualViewportBenchmarkApp{contentHeight: benchmarkRows}
	runtime, err := NewRuntime[struct{}](app, Size{Width: 80, Height: 24})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(runtime.Close)
	warmViewportBenchmark(b, runtime)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		app.contentHeight++
		runtime.RequestFrame()
		if _, err := runtime.RenderIfDirty(); err != nil {
			b.Fatal(err)
		}
	}
}

func newIdentifiedVirtualViewportBenchmarkApp() *identifiedVirtualViewportBenchmarkApp {
	rowIDs := make([]NodeID, benchmarkRows)
	for index := range rowIDs {
		rowIDs[index] = NodeID("row-" + strconv.Itoa(index))
	}
	return &identifiedVirtualViewportBenchmarkApp{rowIDs: rowIDs}
}

func (*identifiedVirtualViewportBenchmarkApp) Init() Effect[struct{}] {
	return NoneEffect[struct{}]()
}

func (*identifiedVirtualViewportBenchmarkApp) Update(struct{}) Effect[struct{}] {
	return NoneEffect[struct{}]()
}

func (*identifiedVirtualViewportBenchmarkApp) Subscriptions() Subscription[struct{}] {
	return NoneSubscription[struct{}]()
}

func (a *identifiedVirtualViewportBenchmarkApp) View(ViewContext) Node[struct{}] {
	return VirtualScrollViewport(
		"viewport",
		Size{Width: 80, Height: benchmarkRows},
		func(viewport VirtualViewport) VirtualFragment[struct{}] {
			rows := make([]Node[struct{}], viewport.Size.Height)
			start := int(viewport.Offset.Y)
			for index := range rows {
				rows[index] = Text[struct{}]("row").WithID(a.rowIDs[start+index])
			}
			return NewVirtualFragment(
				ScrollOffset{Y: viewport.Offset.Y},
				Column(rows...),
			)
		},
	)
}

func BenchmarkScrollViewportVirtualIdentified100K(b *testing.B) {
	runtime, err := NewRuntime[struct{}](newIdentifiedVirtualViewportBenchmarkApp(), Size{Width: 80, Height: 24})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(runtime.Close)
	warmViewportBenchmark(b, runtime)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		runtime.RequestFrame()
		if _, err := runtime.RenderIfDirty(); err != nil {
			b.Fatal(err)
		}
	}
}

func newVirtualFlowBenchmarkApp() *virtualFlowBenchmarkApp {
	items := make([]VirtualFlowItem, benchmarkRows)
	for index := range items {
		items[index] = NewVirtualFlowItem(NodeID("flow-row-" + strconv.Itoa(index)))
	}
	order, err := NewVirtualFlowItems(items)
	if err != nil {
		panic(err)
	}
	return &virtualFlowBenchmarkApp{items: order}
}

func (*virtualFlowBenchmarkApp) Init() Effect[struct{}] {
	return NoneEffect[struct{}]()
}

func (*virtualFlowBenchmarkApp) Update(struct{}) Effect[struct{}] {
	return NoneEffect[struct{}]()
}

func (*virtualFlowBenchmarkApp) Subscriptions() Subscription[struct{}] {
	return NoneSubscription[struct{}]()
}

func (a *virtualFlowBenchmarkApp) View(ViewContext) Node[struct{}] {
	source := NewVirtualFlowSource(a.items, func(context VirtualFlowItemContext) Node[struct{}] {
		if context.Index%2 == 0 {
			return Column(Text[struct{}]("row"), Text[struct{}]("row"))
		}
		return Text[struct{}]("row")
	}).EstimatedHeight(func(context VirtualFlowItemContext) uint32 {
		if context.Index%2 == 0 {
			return 2
		}
		return 1
	})
	return VirtualFlow("virtual-flow", source)
}

func BenchmarkVirtualFlowVariableHeight100K(b *testing.B) {
	runtime, err := NewRuntime[struct{}](newVirtualFlowBenchmarkApp(), Size{Width: 80, Height: 24})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(runtime.Close)
	warmViewportBenchmark(b, runtime)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		runtime.RequestFrame()
		if _, err := runtime.RenderIfDirty(); err != nil {
			b.Fatal(err)
		}
	}
}
