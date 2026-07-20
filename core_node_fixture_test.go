package tui

import (
	"errors"
	"github.com/mayahiro/nagi-go/vt"
	"testing"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagitui-go/internal/conformance"
	"github.com/mayahiro/nagitui-go/surface"
)

type coreNodeFixtureApp struct {
	caseID string
}

func (*coreNodeFixtureApp) Init() Effect[string]         { return NoneEffect[string]() }
func (*coreNodeFixtureApp) Update(string) Effect[string] { return NoneEffect[string]() }
func (*coreNodeFixtureApp) Subscriptions() Subscription[string] {
	return NoneSubscription[string]()
}
func (a *coreNodeFixtureApp) View(_ ViewContext) Node[string] { return coreNodeFixtureView(a.caseID) }

func TestCoreNodeRenderFixtures(t *testing.T) {
	records, err := conformance.Load(
		"runtime/core-nodes.txt",
		"core-node-render",
		"width",
		"height",
		"expected",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		runtime, err := NewRuntimeWithClock[string](
			&coreNodeFixtureApp{caseID: record.ID},
			NewRuntimeConfig(Size{
				Width:  runtimeFixtureNumber(t, record.Field("width")),
				Height: runtimeFixtureNumber(t, record.Field("height")),
			}),
			NewVirtualClock(),
		)
		if err != nil {
			t.Fatal(err)
		}
		frame, err := runtime.RenderIfDirty()
		if err != nil {
			t.Fatal(err)
		}
		if actual, expected := frame.Surface().Snapshot(), record.Text("expected"); actual != expected {
			t.Fatalf("case %s snapshot mismatch\ngot:\n%s\nwant:\n%s", record.ID, actual, expected)
		}
	}
}

func coreNodeFixtureView(caseID string) Node[string] {
	switch caseID {
	case "rich-text":
		return Paragraph[string]([]TextSpan{
			NewTextSpan("Hel", vt.Style{Bold: true}),
			NewTextSpan("lo world", vt.Style{Italic: true}),
		}, DefaultParagraphOptions())
	case "paragraph-center":
		return Paragraph[string](
			[]TextSpan{NewTextSpan("A日", vt.Style{Underline: true})},
			ParagraphOptions{Wrap: WrapHard, Alignment: AlignCenter},
		)
	case "surface-node":
		source, _ := surface.NewTransparent(3, 1)
		source.Write(0, 0, "日A", vt.Style{Bold: true}, celltext.ModernWidth())
		source.FillTransparent(2, 0, 1, 1, vt.Style{Underline: true})
		source.SetCursor(surface.Cursor{X: 2, Y: 0})
		return Stack(Text[string]("xyz"), SurfaceNode[string](source))
	case "panel-layout":
		options := DefaultPanelOptions()
		options.Border = BorderDouble
		options.Style.Background = vt.Style{Background: vt.IndexedColor(4)}
		return PanelWithOptions(
			Column(
				Row(Text[string]("A"), Gap[string](2), Text[string]("B")),
				Spacer[string](1, 1),
				Text[string]("C"),
			),
			"Panel",
			options,
		)
	default:
		panic("unknown core node fixture " + caseID)
	}
}
