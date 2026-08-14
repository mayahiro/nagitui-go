package tui

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/mayahiro/nagi-go/content"
	"github.com/mayahiro/nagi-go/vt"
)

const benchmarkRows = 100_000
const pointerBenchmarkBytes = 100_000

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

type pointerTextHitBenchmarkApp struct {
	document string
}

func (*pointerTextHitBenchmarkApp) Init() Effect[struct{}] {
	return NoneEffect[struct{}]()
}

func (*pointerTextHitBenchmarkApp) Update(struct{}) Effect[struct{}] {
	return NoneEffect[struct{}]()
}

func (*pointerTextHitBenchmarkApp) Subscriptions() Subscription[struct{}] {
	return NoneSubscription[struct{}]()
}

func (a *pointerTextHitBenchmarkApp) View(ViewContext) Node[struct{}] {
	options := DefaultParagraphOptions()
	options.Wrap = WrapNone
	return Paragraph[struct{}]([]TextSpan{
		NewTextSpan(a.document, vt.Style{}),
	}, options).OnPointerEvent("text", func(context PointerEventContext) EventResult[struct{}] {
		if context.Event().Kind == vt.MousePress {
			return ConsumeResult[struct{}]().CapturePointer("text")
		}
		return ConsumeResult[struct{}]()
	})
}

func pointerTextHitEvent(kind vt.MouseKind) vt.Event {
	return vt.Event{Kind: vt.EventMouse, Mouse: vt.MouseEvent{
		Kind: kind, Button: vt.MouseLeft, X: pointerBenchmarkBytes - 1,
	}}
}

func newPointerTextHitBenchmarkRuntime(tb testing.TB) *Runtime[struct{}] {
	tb.Helper()
	runtime, err := NewRuntime[struct{}](
		&pointerTextHitBenchmarkApp{document: strings.Repeat("x", pointerBenchmarkBytes)},
		Size{Width: 80, Height: 1},
	)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(runtime.Close)
	if _, err := runtime.RenderIfDirty(); err != nil {
		tb.Fatal(err)
	}
	if _, err := runtime.DispatchEvent(pointerTextHitEvent(vt.MousePress)); err != nil {
		tb.Fatal(err)
	}
	return runtime
}

func TestWarmedLongParagraphPointerDispatchDoesNotAllocate(t *testing.T) {
	runtime := newPointerTextHitBenchmarkRuntime(t)
	movement := pointerTextHitEvent(vt.MouseMove)
	var dispatchError error
	allocations := testing.AllocsPerRun(1_000, func() {
		_, dispatchError = runtime.DispatchEvent(movement)
	})
	if dispatchError != nil {
		t.Fatal(dispatchError)
	}
	if allocations != 0 {
		t.Fatalf("pointer dispatch allocations = %f, want 0", allocations)
	}
}

func BenchmarkPointerTextHit100K(b *testing.B) {
	runtime := newPointerTextHitBenchmarkRuntime(b)
	movement := pointerTextHitEvent(vt.MouseMove)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := runtime.DispatchEvent(movement); err != nil {
			b.Fatal(err)
		}
	}
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
var benchmarkViewportOutput []byte

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

func BenchmarkViewportOriginEncoding(b *testing.B) {
	operations := []vt.TerminalOp{
		vt.HideCursor(),
		vt.MoveTo(0, 0),
		vt.WriteText("status"),
		vt.MoveTo(0, 1),
		vt.WriteText("ready"),
		vt.MoveTo(0, 0),
	}
	output := vt.EncodeAt(operations, vt.BaselineCapabilities(), 0, 17)
	output = output[:0]
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		output = vt.AppendEncodedAt(
			output[:0],
			operations,
			vt.BaselineCapabilities(),
			0,
			17,
		)
		benchmarkViewportOutput = output
	}
}

type terminalTaskBenchmarkMessage uint8

const (
	terminalTaskBenchmarkStart terminalTaskBenchmarkMessage = iota
	terminalTaskBenchmarkReturned
)

type terminalTaskBenchmarkApp struct{}

func (*terminalTaskBenchmarkApp) Init() Effect[terminalTaskBenchmarkMessage] {
	return NoneEffect[terminalTaskBenchmarkMessage]()
}

func (*terminalTaskBenchmarkApp) Update(message terminalTaskBenchmarkMessage) Effect[terminalTaskBenchmarkMessage] {
	if message == terminalTaskBenchmarkStart {
		return SuspendTerminalEffect(func(context.Context) terminalTaskBenchmarkMessage {
			return terminalTaskBenchmarkReturned
		})
	}
	return NoneEffect[terminalTaskBenchmarkMessage]()
}

func (*terminalTaskBenchmarkApp) Subscriptions() Subscription[terminalTaskBenchmarkMessage] {
	return NoneSubscription[terminalTaskBenchmarkMessage]()
}

func (*terminalTaskBenchmarkApp) View(ViewContext) Node[terminalTaskBenchmarkMessage] {
	return Text[terminalTaskBenchmarkMessage]("")
}

func BenchmarkTerminalTaskRoundTrip(b *testing.B) {
	runtime, err := NewRuntime[terminalTaskBenchmarkMessage](
		&terminalTaskBenchmarkApp{},
		Size{Width: 1, Height: 1},
	)
	if err != nil {
		b.Fatal(err)
	}
	defer runtime.Close()
	roundTrip := func() {
		if err := runtime.Enqueue(terminalTaskBenchmarkStart); err != nil {
			b.Fatal(err)
		}
		if _, err := runtime.ProcessPending(); err != nil {
			b.Fatal(err)
		}
		if !runtime.RunTerminalTask() {
			b.Fatal("terminal task did not run")
		}
		if _, err := runtime.ProcessPending(); err != nil {
			b.Fatal(err)
		}
	}
	roundTrip()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		roundTrip()
	}
}

func TestTerminalTaskRoundTripAllocationBound(t *testing.T) {
	runtime, err := NewRuntime[terminalTaskBenchmarkMessage](
		&terminalTaskBenchmarkApp{},
		Size{Width: 1, Height: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	roundTrip := func() {
		if err := runtime.Enqueue(terminalTaskBenchmarkStart); err != nil {
			panic(err)
		}
		if _, err := runtime.ProcessPending(); err != nil {
			panic(err)
		}
		if !runtime.RunTerminalTask() {
			panic("terminal task did not run")
		}
		if _, err := runtime.ProcessPending(); err != nil {
			panic(err)
		}
	}
	roundTrip()
	if allocations := testing.AllocsPerRun(1_000, roundTrip); allocations > 4 {
		t.Fatalf("allocations = %.0f, want at most 4", allocations)
	}
}
