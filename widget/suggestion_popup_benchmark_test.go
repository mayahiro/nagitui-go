package widget

import (
	"strconv"
	"testing"

	"github.com/mayahiro/nagitui-go"
)

var benchmarkSuggestionPopupNode tui.Node[struct{}]

func BenchmarkSuggestionPopupCandidates(b *testing.B) {
	for _, count := range []int{8, 100_000} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			candidateIDs := make([]SuggestionID, count)
			for index := range candidateIDs {
				candidateIDs[index] = NewSuggestionID("candidate-" + strconv.Itoa(index))
			}
			items, err := NewSuggestionItems(candidateIDs)
			if err != nil {
				b.Fatal(err)
			}
			selected, _ := items.Item(count / 2)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchmarkSuggestionPopupNode = NewSuggestionPopup(
					tui.NewNodeID("popup"),
					tui.Text[struct{}]("base").WithID(tui.NewNodeID("anchor")),
					tui.NewNodeID("anchor"),
					tui.NewNodeID("anchor"),
					items,
					selected,
					true,
					func(context SuggestionRowContext) tui.Node[struct{}] {
						return tui.Text[struct{}](context.ID().String())
					},
					func(SuggestionID) struct{} { return struct{}{} },
					func(SuggestionID) struct{} { return struct{}{} },
					func() struct{} { return struct{}{} },
				).VisibleRows(8).Node()
			}
		})
	}
}
