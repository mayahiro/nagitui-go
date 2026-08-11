package widget

import (
	"testing"

	tui "github.com/mayahiro/nagitui-go"
)

type emptyVirtualFeedApp struct{}

func (*emptyVirtualFeedApp) Init() tui.Effect[struct{}] {
	return tui.NoneEffect[struct{}]()
}

func (*emptyVirtualFeedApp) Update(struct{}) tui.Effect[struct{}] {
	return tui.NoneEffect[struct{}]()
}

func (*emptyVirtualFeedApp) Subscriptions() tui.Subscription[struct{}] {
	return tui.NoneSubscription[struct{}]()
}

func (*emptyVirtualFeedApp) View(tui.ViewContext) tui.Node[struct{}] {
	items, _ := tui.NewVirtualFlowItems(nil)
	source := tui.NewVirtualFlowSource(items, func(tui.VirtualFlowItemContext) tui.Node[struct{}] {
		return tui.Spacer[struct{}](0, 1)
	})
	return NewVirtualFeed("feed", source).
		Empty(tui.Text[struct{}]("empty")).
		LoadingBefore(tui.Text[struct{}]("before")).
		LoadingAfter(tui.Text[struct{}]("after")).
		UnreadIndicator(tui.Text[struct{}]("unread")).
		Node()
}

func TestVirtualFeedSlotsArePinnedAndEmptyIsCentered(t *testing.T) {
	runtime, err := tui.NewRuntimeWithClock[struct{}](
		&emptyVirtualFeedApp{},
		tui.NewRuntimeConfig(tui.Size{Width: 10, Height: 7}),
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
	assertVirtualFeedCell(t, frame, 0, 0, "b")
	assertVirtualFeedCell(t, frame, 2, 3, "e")
	assertVirtualFeedCell(t, frame, 4, 5, "u")
	assertVirtualFeedCell(t, frame, 0, 6, "a")
	state, ok := runtime.Interaction().VirtualFlowState("feed")
	if !ok || state.ItemCount != 0 {
		t.Fatalf("virtual feed state = %+v, present = %t", state, ok)
	}
}

type followingVirtualFeedApp struct{}

func (*followingVirtualFeedApp) Init() tui.Effect[struct{}] {
	return tui.NoneEffect[struct{}]()
}

func (*followingVirtualFeedApp) Update(struct{}) tui.Effect[struct{}] {
	return tui.NoneEffect[struct{}]()
}

func (*followingVirtualFeedApp) Subscriptions() tui.Subscription[struct{}] {
	return tui.NoneSubscription[struct{}]()
}

func (*followingVirtualFeedApp) View(tui.ViewContext) tui.Node[struct{}] {
	items, _ := tui.NewVirtualFlowItems([]tui.VirtualFlowItem{
		tui.NewVirtualFlowItem("a"),
		tui.NewVirtualFlowItem("b"),
		tui.NewVirtualFlowItem("c"),
	})
	source := tui.NewVirtualFlowSource(items, func(context tui.VirtualFlowItemContext) tui.Node[struct{}] {
		return tui.Column(
			tui.Text[struct{}](context.Key.String()),
			tui.Text[struct{}](context.Key.String()),
		)
	}).EstimatedHeight(func(tui.VirtualFlowItemContext) uint32 { return 2 })
	return NewVirtualFeed("feed", source).Node()
}

func TestVirtualFeedFollowsTheEndByDefault(t *testing.T) {
	runtime, err := tui.NewRuntimeWithClock[struct{}](
		&followingVirtualFeedApp{},
		tui.NewRuntimeConfig(tui.Size{Width: 4, Height: 3}),
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
	state, ok := runtime.Interaction().VirtualFlowState("feed")
	if !ok || state.Scroll.Offset.Y != 3 || !state.Scroll.AtEnd {
		t.Fatalf("virtual feed state = %+v, present = %t", state, ok)
	}
	assertVirtualFeedCell(t, frame, 0, 0, "b")
	assertVirtualFeedCell(t, frame, 0, 1, "c")
}

func assertVirtualFeedCell(
	t *testing.T,
	frame *tui.Frame,
	x, y int32,
	expected string,
) {
	t.Helper()
	cell, ok := frame.Surface().Cell(x, y)
	if !ok || cell.Content() != expected {
		t.Fatalf("cell (%d, %d) = %q, present = %t, want %q", x, y, cell.Content(), ok, expected)
	}
}
