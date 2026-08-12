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
