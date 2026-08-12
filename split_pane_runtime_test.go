package tui

import "testing"

type lazySplitPaneApp struct {
	builds int
}

func (*lazySplitPaneApp) Init() Effect[struct{}] { return NoneEffect[struct{}]() }

func (*lazySplitPaneApp) Update(struct{}) Effect[struct{}] { return NoneEffect[struct{}]() }

func (*lazySplitPaneApp) Subscriptions() Subscription[struct{}] {
	return NoneSubscription[struct{}]()
}

func (a *lazySplitPaneApp) View(ViewContext) Node[struct{}] {
	secondary := VirtualScrollViewport(
		NewNodeID("secondary"), Size{Width: 20, Height: 4},
		func(VirtualViewport) VirtualFragment[struct{}] {
			a.builds++
			return NewVirtualFragment(ScrollOffset{}, Text[struct{}]("secondary"))
		},
	)
	options := DefaultSplitPaneOptions()
	options.PrimaryMinimum = 5
	options.SecondaryMinimum = 5
	return SplitPane(
		Text[struct{}]("primary").Focusable(NewNodeID("primary")),
		secondary,
		options,
	)
}

func TestSplitPaneCollapsedPaneIsOmittedFromSemanticsAndLazyPreparation(t *testing.T) {
	app := &lazySplitPaneApp{}
	runtime, err := NewRuntimeWithClock(
		app, NewRuntimeConfig(Size{Width: 8, Height: 3}), NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if app.builds != 0 {
		t.Fatalf("collapsed builds = %d, want 0", app.builds)
	}
	if focused, err := runtime.RequestFocus(NewNodeID("secondary")); err != nil || focused {
		t.Fatalf("hidden focus = %t, %v", focused, err)
	}

	runtime.Resize(Size{Width: 12, Height: 3})
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if app.builds != 1 {
		t.Fatalf("expanded builds = %d, want 1", app.builds)
	}
	if focused, err := runtime.RequestFocus(NewNodeID("secondary")); err != nil || !focused {
		t.Fatalf("visible focus = %t, %v", focused, err)
	}
}

type renderSplitPaneApp struct{}

func (*renderSplitPaneApp) Init() Effect[struct{}] { return NoneEffect[struct{}]() }

func (*renderSplitPaneApp) Update(struct{}) Effect[struct{}] { return NoneEffect[struct{}]() }

func (*renderSplitPaneApp) Subscriptions() Subscription[struct{}] {
	return NoneSubscription[struct{}]()
}

func (*renderSplitPaneApp) View(ViewContext) Node[struct{}] {
	return SplitPane(
		Text[struct{}]("A"), Text[struct{}]("B"), DefaultSplitPaneOptions(),
	)
}

func TestSplitPaneDividerUsesOneCellBetweenRenderedPanes(t *testing.T) {
	runtime, err := NewRuntimeWithClock(
		&renderSplitPaneApp{}, NewRuntimeConfig(Size{Width: 5, Height: 1}), NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	frame, err := runtime.RenderIfDirty()
	if err != nil || frame == nil {
		t.Fatalf("render = %v, %v", frame, err)
	}
	for _, expected := range []struct {
		x       int32
		content string
	}{{0, "A"}, {2, "│"}, {3, "B"}} {
		cell, ok := frame.Surface().Cell(expected.x, 0)
		if !ok || cell.Content() != expected.content {
			t.Fatalf("cell %d = %#v, %t, want %q", expected.x, cell, ok, expected.content)
		}
	}
}

func TestSplitPaneResolutionDoesNotAllocate(t *testing.T) {
	options := DefaultSplitPaneOptions()
	rect := Rect{Width: 120, Height: 40}
	_ = resolveSplitPaneLayout(rect, options)
	if allocations := testing.AllocsPerRun(1000, func() {
		_ = resolveSplitPaneLayout(rect, options)
	}); allocations != 0 {
		t.Fatalf("allocations = %f, want 0", allocations)
	}
}
