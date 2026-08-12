package widget

import (
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	tui "github.com/mayahiro/nagitui-go"
)

type focusSplitPaneApp struct{}

func (*focusSplitPaneApp) Init() tui.Effect[struct{}] { return tui.NoneEffect[struct{}]() }

func (*focusSplitPaneApp) Update(struct{}) tui.Effect[struct{}] {
	return tui.NoneEffect[struct{}]()
}

func (*focusSplitPaneApp) Subscriptions() tui.Subscription[struct{}] {
	return tui.NoneSubscription[struct{}]()
}

func (*focusSplitPaneApp) View(tui.ViewContext) tui.Node[struct{}] {
	return NewSplitPane(
		tui.NewNodeID("split"),
		tui.Text[struct{}]("primary").Focusable(tui.NewNodeID("primary")),
		tui.Text[struct{}]("secondary").Focusable(tui.NewNodeID("secondary")),
		DefaultSplitPaneState(),
	).Minimums(5, 5).
		FocusTargets(tui.NewNodeID("primary"), tui.NewNodeID("secondary")).
		Node()
}

func TestSplitPaneCollapseMovesFocusWithoutRestoringOnExpand(t *testing.T) {
	runtime, err := tui.NewRuntimeWithClock(
		&focusSplitPaneApp{}, tui.NewRuntimeConfig(tui.Size{Width: 12, Height: 2}), tui.NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if focused, err := runtime.RequestFocus(tui.NewNodeID("secondary")); err != nil || !focused {
		t.Fatalf("focus = %t, %v", focused, err)
	}

	runtime.Resize(tui.Size{Width: 8, Height: 2})
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if focused, ok := runtime.Interaction().Focused(); !ok || focused != tui.NewNodeID("primary") {
		t.Fatalf("collapsed focus = %s, %t", focused, ok)
	}
	if focused, err := runtime.RequestFocus(tui.NewNodeID("secondary")); err != nil || focused {
		t.Fatalf("hidden focus = %t, %v", focused, err)
	}

	runtime.Resize(tui.Size{Width: 12, Height: 2})
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if focused, ok := runtime.Interaction().Focused(); !ok || focused != tui.NewNodeID("primary") {
		t.Fatalf("expanded focus = %s, %t", focused, ok)
	}
}

type pointerSplitPaneApp struct {
	state SplitPaneState
}

func (*pointerSplitPaneApp) Init() tui.Effect[SplitPaneState] {
	return tui.NoneEffect[SplitPaneState]()
}

func (a *pointerSplitPaneApp) Update(state SplitPaneState) tui.Effect[SplitPaneState] {
	a.state = state
	return tui.NoneEffect[SplitPaneState]()
}

func (*pointerSplitPaneApp) Subscriptions() tui.Subscription[SplitPaneState] {
	return tui.NoneSubscription[SplitPaneState]()
}

func (a *pointerSplitPaneApp) View(tui.ViewContext) tui.Node[SplitPaneState] {
	return NewSplitPane(
		tui.NewNodeID("split"),
		tui.Text[SplitPaneState]("primary"),
		tui.Text[SplitPaneState]("secondary"),
		a.state,
	).OnResize(func(state SplitPaneState) SplitPaneState { return state }).Node()
}

func TestSplitPanePointerDragCapturesUpdatesAndReleases(t *testing.T) {
	app := &pointerSplitPaneApp{state: DefaultSplitPaneState()}
	runtime, err := tui.NewRuntimeWithClock(
		app, tui.NewRuntimeConfig(tui.Size{Width: 11, Height: 3}), tui.NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}

	press, err := runtime.DispatchEvent(splitPanePointer(vt.MousePress, 5, 1))
	if err != nil || !press.Consumed() {
		t.Fatalf("press = %+v, %v", press, err)
	}
	if capture, ok := runtime.Interaction().PointerCapture(); !ok || capture != tui.NewNodeID("split") {
		t.Fatalf("capture = %s, %t", capture, ok)
	}

	movement, err := runtime.DispatchEvent(splitPanePointer(vt.MouseMove, 8, 1))
	if err != nil || !movement.Consumed() || movement.Messages() != 1 {
		t.Fatalf("movement = %+v, %v", movement, err)
	}
	if _, err := runtime.ProcessPending(); err != nil {
		t.Fatal(err)
	}
	if app.state.Ratio() != 8_000 {
		t.Fatalf("move ratio = %d", app.state.Ratio())
	}
	if capture, ok := runtime.Interaction().PointerCapture(); !ok || capture != tui.NewNodeID("split") {
		t.Fatalf("move capture = %s, %t", capture, ok)
	}

	release, err := runtime.DispatchEvent(splitPanePointer(vt.MouseRelease, 7, 1))
	if err != nil || !release.Consumed() || release.Messages() != 1 {
		t.Fatalf("release = %+v, %v", release, err)
	}
	if _, err := runtime.ProcessPending(); err != nil {
		t.Fatal(err)
	}
	if app.state.Ratio() != 7_000 {
		t.Fatalf("release ratio = %d", app.state.Ratio())
	}
	if capture, ok := runtime.Interaction().PointerCapture(); ok {
		t.Fatalf("released capture = %s", capture)
	}
}

func splitPanePointer(kind vt.MouseKind, x, y uint32) vt.Event {
	return vt.Event{Kind: vt.EventMouse, Mouse: vt.MouseEvent{
		Kind: kind, Button: vt.MouseLeft, X: x, Y: y,
	}}
}

func TestSplitPaneDividerDetectionCollapsesWhenExtremeMinimaDoNotFit(t *testing.T) {
	if _, ok := splitPaneDividerPosition(^uint32(0), DefaultSplitPaneState(), ^uint32(0), 1); ok {
		t.Fatal("extreme minima unexpectedly produced a divider")
	}
}
