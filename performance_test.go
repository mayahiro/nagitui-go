package tui

import (
	"strconv"
	"strings"
	"testing"

	"github.com/mayahiro/nagi-go/content"
	"github.com/mayahiro/nagi-go/vt"
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

func contentProjectionBenchmarkInput() (
	content.Content,
	PresentationSheet,
	ContentProjectionOptions,
) {
	paragraphRole, _ := content.NewRole("paragraph")
	children := make([]content.Content, 24)
	for index := range children {
		inline := content.NewInline([]content.Content{content.NewText("value")})
		paragraph, _ := content.NewParagraph([]content.Content{
			content.NewText(strconv.Itoa(index)),
			inline.Content(),
		}).WithRoles([]content.Role{paragraphRole})
		children[index] = paragraph.Content()
	}
	root := content.NewFlow(children).Content()
	sheet := NewPresentationSheet([]PresentationRule{
		NewPresentationRule(
			AnyPresentationSelector(),
			PresentationDeclaration{}.WithTextStyle(
				TextStyleDeclaration{}.WithDim(SetDeclarationValue(true)),
			),
		),
		NewPresentationRule(
			RolePresentationSelector(paragraphRole),
			PresentationDeclaration{}.WithVisualSeparator(SetDeclarationValue(" ")),
		),
	})
	return root, sheet, ContentProjectionOptions{}
}

func BenchmarkContentProjectionVisibleSubtree(b *testing.B) {
	root, sheet, options := contentProjectionBenchmarkInput()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		node, err := ProjectContent[struct{}](root, sheet, options)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkProjectionNode = node
	}
}

func BenchmarkContentProjectionBoundedFailure100K(b *testing.B) {
	children := make([]content.Content, benchmarkRows)
	for index := range children {
		children[index] = content.NewText("value")
	}
	root := content.NewFlow(children).Content()
	options := ContentProjectionOptions{}.WithLimits(
		ContentProjectionLimits{}.WithMaxContentNodes(128),
	)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_, err := ProjectContent[struct{}](root, PresentationSheet{}, options)
		if err == nil {
			b.Fatal("projection unexpectedly succeeded")
		}
	}
}

var benchmarkProjectionNode Node[struct{}]

var benchmarkClipboardOutput []byte

func benchmarkClipboardEncoding(b *testing.B, text string, reuse bool) {
	b.Helper()
	operations := []vt.TerminalOp{vt.SetClipboard(text)}
	output := vt.Encode(operations, vt.BaselineCapabilities())
	output = output[:0]
	b.SetBytes(int64(len(text)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if reuse {
			output = output[:0]
			output = vt.AppendEncoded(output, operations, vt.BaselineCapabilities())
			benchmarkClipboardOutput = output
		} else {
			benchmarkClipboardOutput = vt.Encode(operations, vt.BaselineCapabilities())
		}
	}
}

func BenchmarkClipboardEncodingSmall(b *testing.B) {
	for _, test := range []struct {
		name  string
		reuse bool
	}{
		{name: "Fresh"},
		{name: "Reused", reuse: true},
	} {
		b.Run(test.name, func(b *testing.B) {
			benchmarkClipboardEncoding(b, "A日", test.reuse)
		})
	}
}

func BenchmarkClipboardEncoding1MiB(b *testing.B) {
	text := strings.Repeat("x", 1<<20)
	for _, test := range []struct {
		name  string
		reuse bool
	}{
		{name: "Fresh"},
		{name: "Reused", reuse: true},
	} {
		b.Run(test.name, func(b *testing.B) {
			benchmarkClipboardEncoding(b, text, test.reuse)
		})
	}
}
