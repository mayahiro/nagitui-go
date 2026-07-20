package tui

import (
	"strconv"
	"testing"
)

const benchmarkRows = 100_000

type eagerViewportBenchmarkApp struct{}

type virtualViewportBenchmarkApp struct{}

type identifiedVirtualViewportBenchmarkApp struct {
	rowIDs []NodeID
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
