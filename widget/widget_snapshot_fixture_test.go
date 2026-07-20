package widget

import (
	"errors"
	"testing"

	"github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/internal/conformance"
)

type widgetSnapshotApp struct {
	caseID string
}

func (*widgetSnapshotApp) Init() tui.Effect[int]      { return tui.NoneEffect[int]() }
func (*widgetSnapshotApp) Update(int) tui.Effect[int] { return tui.NoneEffect[int]() }
func (*widgetSnapshotApp) Subscriptions() tui.Subscription[int] {
	return tui.NoneSubscription[int]()
}
func (a *widgetSnapshotApp) View(_ tui.ViewContext) tui.Node[int] {
	return widgetSnapshotView(a.caseID)
}

func TestWidgetRuntimeSurfaceSnapshots(t *testing.T) {
	records, err := conformance.Load(
		"widgets/runtime-snapshots.txt", "widget-runtime-snapshot", "width", "height", "expected",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		runtime, err := tui.NewRuntimeWithClock(
			&widgetSnapshotApp{caseID: record.ID},
			tui.NewRuntimeConfig(tui.Size{
				Width: uint32(fixtureInt(t, record.Field("width"))), Height: uint32(fixtureInt(t, record.Field("height"))),
			}),
			tui.NewVirtualClock(),
		)
		if err != nil {
			t.Fatal(err)
		}
		frame, err := runtime.RenderIfDirty()
		runtime.Close()
		if err != nil {
			t.Fatal(err)
		}
		if actual, expected := frame.Surface().Snapshot(), record.Text("expected"); actual != expected {
			t.Errorf("case %s snapshot mismatch\ngot:\n%s\nwant:\n%s", record.ID, actual, expected)
		}
	}
}

func widgetSnapshotView(caseID string) tui.Node[int] {
	switch caseID {
	case "sparkline":
		return NewSparkline[int]([]uint64{0, 1, 2, 3}, 4).Bounds(0, 3).Node()
	case "bar-chart":
		return NewBarChart[int]([]BarChartBar{NewBarChartBar("A", 5)}, 4).Maximum(10).ShowValues(false).Node()
	case "chart":
		return NewChart[int]([]ChartSeries{NewChartSeries("s", []ChartPoint{{X: 0, Y: 0}, {X: 2, Y: 2}})}, 5, 3).
			Bounds(0, 2, 0, 2).Node()
	case "help":
		return NewHelp[int]([]HelpBinding{NewHelpBinding("q", "Quit"), NewHelpBinding("?", "Help")}).
			Separator(" | ").Node()
	case "paginator":
		return NewPaginator[int]("pages", 2, 5, func(page int) int { return page }).Node()
	case "text-area":
		return NewTextArea[int](
			"area", NewTextAreaStateWithSelection("A日BC", 4, 1).WithHorizontalOffset(1),
			func(TextAreaState) int { return 0 },
		).Node()
	case "table":
		return NewTable[int](
			"table",
			[]TableColumn{NewTableColumn("A", tui.Fixed(3)), NewTableColumn("B", tui.Fixed(3))},
			[]TableRow{NewTableRow("row", []string{"x", "y"})},
			0,
			func(index int) int { return index },
		).ColumnAlignment(0, tui.AlignEnd).ColumnAlignment(1, tui.AlignCenter).Node()
	case "tree":
		items := make([]TreeItem, 5)
		for index := range items {
			items[index] = NewTreeLeaf(tui.NodeID(rune('a'+index)), "Item"+string(rune('0'+index)), 0)
		}
		return NewTree[int]("tree", items, 3, func(index int) int { return index }).Viewport(2).Node()
	case "file-picker":
		return NewFilePicker[int](
			"files",
			[]FilePickerEntry{
				NewFilePickerFile("a", "A", "a"),
				NewFilePickerDirectory("b", "B", "b"),
				NewFilePickerFile("hidden", "Hidden", ".hidden").WithHidden(true),
				NewFilePickerFile("c", "C", "c"),
			},
			3,
			func(index int) int { return index },
		).Viewport(2).Node()
	default:
		panic("unknown widget snapshot case " + caseID)
	}
}
