package tui

import (
	"github.com/mayahiro/nagi-go/vt"
	"testing"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagitui-go/surface"
)

func TestRichTextPreservesSpanStylesAcrossWordWrapping(t *testing.T) {
	first := vt.Style{Bold: true}
	second := vt.Style{Italic: true}
	node := Paragraph[nodeTestMessage]([]TextSpan{
		NewTextSpan("Hel", first),
		NewTextSpan("lo world", second),
	}, DefaultParagraphOptions())
	target := newNodeTestSurface(t, 7, 2)

	node.renderTo(target, NewInteractionState())

	assertNodeCell(t, target, 0, 0, "H")
	assertNodeCell(t, target, 4, 0, "o")
	assertNodeCell(t, target, 0, 1, "w")
	assertNodeStyle(t, target, 0, 0, first)
	assertNodeStyle(t, target, 3, 0, second)
	assertNodeStyle(t, target, 0, 1, second)
}

func TestParagraphAlignmentAndNoWrapRespondToBounds(t *testing.T) {
	centered := Paragraph[nodeTestMessage](
		[]TextSpan{NewTextSpan("A日", vt.Style{})},
		ParagraphOptions{Wrap: WrapHard, Alignment: AlignCenter},
	)
	target := newNodeTestSurface(t, 5, 1)

	centered.renderTo(target, NewInteractionState())

	assertNodeCell(t, target, 1, 0, "A")
	assertNodeCell(t, target, 2, 0, "日")

	unwrapped := Paragraph[nodeTestMessage](
		[]TextSpan{NewTextSpan("ABCDE", vt.Style{})},
		ParagraphOptions{Wrap: WrapNone},
	)
	clipped := newNodeTestSurface(t, 3, 2)
	unwrapped.renderTo(clipped, NewInteractionState())
	assertNodeCell(t, clipped, 2, 0, "C")
	assertNodeCell(t, clipped, 0, 1, " ")
}

func TestSurfaceNodeCapturesAndSafelyCompositesSurface(t *testing.T) {
	source, err := surface.NewTransparent(3, 1)
	if err != nil {
		t.Fatal(err)
	}
	source.Write(0, 0, "日A", vt.Style{Bold: true}, celltext.ModernWidth())
	source.FillTransparent(2, 0, 1, 1, vt.Style{Underline: true})
	source.SetCursor(surface.Cursor{X: 2, Y: 0})
	node := Stack(
		Text[nodeTestMessage]("xyz"),
		SurfaceNode[nodeTestMessage](source),
	)
	source.Clear()
	target := newNodeTestSurface(t, 3, 1)

	node.renderTo(target, NewInteractionState())

	assertNodeCell(t, target, 0, 0, "日")
	assertNodeCell(t, target, 2, 0, "z")
	assertNodeStyle(t, target, 0, 0, vt.Style{Bold: true})
	cell, _ := target.Cell(2, 0)
	if !cell.Style().Underline {
		t.Fatalf("transparent style = %+v, want underline", cell.Style())
	}
	if cursor, ok := target.Cursor(); !ok || cursor != (surface.Cursor{X: 2, Y: 0}) {
		t.Fatalf("cursor = %+v, %t, want 2,0", cursor, ok)
	}
}

func TestPanelRendersTitleBorderPaddingAndBackground(t *testing.T) {
	options := DefaultPanelOptions()
	options.Border = BorderRounded
	options.Style.Background = vt.Style{Background: vt.IndexedColor(4)}
	node := PanelWithOptions(Text[nodeTestMessage]("X"), "Title", options)
	target := newNodeTestSurface(t, 10, 5)

	node.renderTo(target, NewInteractionState())

	assertNodeCell(t, target, 0, 0, "╭")
	assertNodeCell(t, target, 1, 0, " ")
	assertNodeCell(t, target, 2, 0, "T")
	assertNodeCell(t, target, 9, 4, "╯")
	assertNodeCell(t, target, 2, 2, "X")
	cell, _ := target.Cell(5, 2)
	if cell.Style().Background != vt.IndexedColor(4) {
		t.Fatalf("background = %+v, want indexed 4", cell.Style().Background)
	}
}

func TestGapAndSpacerReserveDeterministicLayoutSpace(t *testing.T) {
	node := Column(
		Row(
			Text[nodeTestMessage]("A"),
			Gap[nodeTestMessage](2),
			Text[nodeTestMessage]("B"),
		),
		Spacer[nodeTestMessage](1, 2),
		Text[nodeTestMessage]("C"),
	)
	target := newNodeTestSurface(t, 4, 4)

	node.renderTo(target, NewInteractionState())

	assertNodeCell(t, target, 0, 0, "A")
	assertNodeCell(t, target, 3, 0, "B")
	assertNodeCell(t, target, 0, 3, "C")
}

func newNodeTestSurface(t *testing.T, width, height uint32) *surface.Surface {
	t.Helper()
	target, err := surface.New(width, height)
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func assertNodeStyle(t *testing.T, target *surface.Surface, x, y int32, expected vt.Style) {
	t.Helper()
	cell, ok := target.Cell(x, y)
	if !ok || cell.Style() != expected {
		t.Fatalf("cell %d,%d style = %+v, %t, want %+v", x, y, cell.Style(), ok, expected)
	}
}
