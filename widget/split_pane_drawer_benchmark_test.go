package widget

import (
	"testing"

	tui "github.com/mayahiro/nagitui-go"
)

func BenchmarkSplitPaneConstruction(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		splitPane := NewSplitPane(
			"split",
			tui.Text[struct{}]("primary").WithID("primary"),
			tui.Text[struct{}]("secondary").WithID("secondary"),
			NewSplitPaneState(2_500),
		).
			Minimums(18, 30).
			FocusTargets("primary", "secondary").
			OnResize(func(SplitPaneState) struct{} { return struct{}{} })
		_ = splitPane.Node()
	}
}

func BenchmarkDrawerClosedConstruction(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		drawer := NewDrawer("drawer", tui.Text[struct{}]("base"), false).
			Side(DrawerBottom).
			Size(tui.Fixed(5)).
			Body(func() tui.Node[struct{}] { return tui.Text[struct{}]("details") }).
			OnDismiss(func() struct{} { return struct{}{} })
		_ = drawer.Node()
	}
}

func BenchmarkDrawerOpenConstruction(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		drawer := NewDrawer("drawer", tui.Text[struct{}]("base"), true).
			Side(DrawerBottom).
			Size(tui.Fixed(5)).
			Body(func() tui.Node[struct{}] { return tui.Text[struct{}]("details") }).
			OnDismiss(func() struct{} { return struct{}{} })
		_ = drawer.Node()
	}
}

func BenchmarkStatusBarThreeSlotConstruction(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		bar := NewStatusBar([]StatusBarSlot[struct{}]{
			NewStatusBarSlot(tui.Text[struct{}]("connected")).Priority(StatusBarHigh),
			NewStatusBarSlot(tui.Text[struct{}]("running")).
				Placement(tui.ResponsiveRowCenter).
				Priority(StatusBarCritical),
			NewStatusBarSlot(tui.Text[struct{}]("usage 42%")).Placement(tui.ResponsiveRowEnd),
		})
		_ = bar.Node()
	}
}

func BenchmarkToastRegionEightRecordsThreeVisible(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		toasts := make([]Toast[struct{}], 8)
		for index := range toasts {
			index := index
			toasts[index] = NewToast(tui.NodeID("toast"), func() tui.Node[struct{}] {
				return tui.Text[struct{}](string(rune('0' + index)))
			}).Tone(ToastInfo)
		}
		_ = NewToastRegion(tui.Text[struct{}]("base"), toasts).VisibleLimit(3).Node()
	}
}
