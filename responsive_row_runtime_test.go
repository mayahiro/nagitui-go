package tui

import "testing"

type responsiveRowRuntimeApp struct {
	builds int
}

func (*responsiveRowRuntimeApp) Init() Effect[struct{}] { return NoneEffect[struct{}]() }

func (*responsiveRowRuntimeApp) Update(struct{}) Effect[struct{}] { return NoneEffect[struct{}]() }

func (*responsiveRowRuntimeApp) Subscriptions() Subscription[struct{}] {
	return NoneSubscription[struct{}]()
}

func (a *responsiveRowRuntimeApp) View(ViewContext) Node[struct{}] {
	low := VirtualScrollViewport(
		"low", Size{Width: 10, Height: 1},
		func(VirtualViewport) VirtualFragment[struct{}] {
			a.builds++
			return NewVirtualFragment(ScrollOffset{}, Text[struct{}]("low"))
		},
	)
	return ResponsiveRow(
		[]ResponsiveRowItem[struct{}]{
			NewResponsiveRowItem(Text[struct{}]("high!").Focusable("high")).Priority(10),
			NewResponsiveRowItem(low),
		},
		ResponsiveRowOptions{Gap: 1},
	)
}

func TestResponsiveRowHiddenItemIsOmittedFromSemanticsAndLazyPreparation(t *testing.T) {
	app := &responsiveRowRuntimeApp{}
	runtime, err := NewRuntimeWithClock(
		app, NewRuntimeConfig(Size{Width: 5, Height: 1}), NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if app.builds != 0 {
		t.Fatalf("hidden builds = %d, want 0", app.builds)
	}
	if focused, err := runtime.RequestFocus("low"); err != nil || focused {
		t.Fatalf("hidden focus = %t, %v", focused, err)
	}

	runtime.Resize(Size{Width: 16, Height: 1})
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if app.builds != 1 {
		t.Fatalf("visible builds = %d, want 1", app.builds)
	}
	if focused, err := runtime.RequestFocus("low"); err != nil || !focused {
		t.Fatalf("visible focus = %t, %v", focused, err)
	}
}

type overlayRuntimeApp struct{}

func (*overlayRuntimeApp) Init() Effect[struct{}] { return NoneEffect[struct{}]() }

func (*overlayRuntimeApp) Update(struct{}) Effect[struct{}] { return NoneEffect[struct{}]() }

func (*overlayRuntimeApp) Subscriptions() Subscription[struct{}] {
	return NoneSubscription[struct{}]()
}

func (*overlayRuntimeApp) View(ViewContext) Node[struct{}] {
	return Column(
		Overlay(Text[struct{}]("A"), Text[struct{}]("layer\nlayer\nlayer")),
		Text[struct{}]("B"),
	)
}

func TestOverlayUsesOnlyBaseIntrinsicMeasurement(t *testing.T) {
	runtime, err := NewRuntimeWithClock(
		&overlayRuntimeApp{}, NewRuntimeConfig(Size{Width: 8, Height: 4}), NewVirtualClock(),
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
	}{{0, 0, "l"}, {0, 1, "B"}} {
		cell, ok := frame.Surface().Cell(expected.x, expected.y)
		if !ok || cell.Content() != expected.content {
			t.Fatalf("cell %d,%d = %#v, %t, want %q", expected.x, expected.y, cell, ok, expected.content)
		}
	}
}
