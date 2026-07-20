package tui

import (
	"github.com/mayahiro/nagi-go/vt"
	"testing"

	"github.com/mayahiro/nagitui-go/surface"
)

type nodeTestMessage struct{}

func TestPrimitiveTreeRendersGraphemesAndLayout(t *testing.T) {
	node := Border(
		Column(
			Text[nodeTestMessage]("A日").WithLength(Fixed(1)),
			Align(Text[nodeTestMessage]("B"), AlignEnd, AlignBottom).WithLength(Flex(1)),
		),
		vt.Style{},
	)
	target, err := surface.New(5, 4)
	if err != nil {
		t.Fatal(err)
	}

	node.renderTo(target, NewInteractionState())

	assertNodeCell(t, target, 1, 1, "A")
	assertNodeCell(t, target, 2, 1, "日")
	assertNodeCell(t, target, 3, 2, "B")
	assertNodeCell(t, target, 0, 0, "┌")
	assertNodeCell(t, target, 4, 3, "┘")
}

func TestClipPreventsChildDrawingOutsideItsRect(t *testing.T) {
	node := Clip(Text[nodeTestMessage]("ABCDE"))
	target, err := surface.New(3, 1)
	if err != nil {
		t.Fatal(err)
	}

	node.renderTo(target, NewInteractionState())

	assertNodeCell(t, target, 2, 0, "C")
}

func TestFocusedStyleOverlayPreservesTextAndBaseStyle(t *testing.T) {
	node := StyledText[nodeTestMessage]("A日", vt.Style{Reverse: true}).
		Focusable("item").
		WithFocusedStyle(vt.Style{Underline: true})
	target, err := surface.New(3, 1)
	if err != nil {
		t.Fatal(err)
	}
	interaction := NewInteractionState()
	interaction.focused = "item"
	interaction.hasFocus = true

	node.renderTo(target, interaction)

	assertNodeCell(t, target, 0, 0, "A")
	assertNodeCell(t, target, 1, 0, "日")
	for _, x := range []int32{0, 1, 2} {
		cell, ok := target.Cell(x, 0)
		if !ok || !cell.Style().Reverse || !cell.Style().Underline {
			t.Fatalf("cell %d style = %+v, %t, want reverse and underline", x, cell.Style(), ok)
		}
	}
}

func assertNodeCell(t *testing.T, target *surface.Surface, x, y int32, expected string) {
	t.Helper()
	cell, ok := target.Cell(x, y)
	if !ok {
		t.Fatalf("cell %d,%d is out of bounds", x, y)
	}
	if cell.Content() != expected {
		t.Fatalf("cell %d,%d content = %q, want %q", x, y, cell.Content(), expected)
	}
}
