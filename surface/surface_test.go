package surface

import (
	"errors"
	"reflect"
	"testing"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
)

func TestSurfaceDimensions(t *testing.T) {
	if _, err := New(0, ^uint32(0)); !errors.Is(err, ErrSurfaceTooLarge) {
		t.Fatalf("New(0, max) error = %v, want ErrSurfaceTooLarge", err)
	}
	if _, err := New(^uint32(0), ^uint32(0)); !errors.Is(err, ErrSurfaceTooLarge) {
		t.Fatalf("New(max, max) error = %v, want ErrSurfaceTooLarge", err)
	}
	if _, err := New(1024, 1024); err != nil {
		t.Fatalf("New(1024, 1024) error = %v", err)
	}
}

func TestOverwriteContinuationRepairsWideUnit(t *testing.T) {
	surface, err := New(3, 1)
	if err != nil {
		t.Fatal(err)
	}
	surface.Write(0, 0, "日", vt.Style{}, celltext.ModernWidth())
	surface.Write(1, 0, "X", vt.Style{}, celltext.ModernWidth())

	first, _ := surface.Cell(0, 0)
	second, _ := surface.Cell(1, 0)
	if first.Content() != " " || first.Span() != SpanOne {
		t.Errorf("first cell = %#v, want a narrow blank", first)
	}
	if second.Content() != "X" || second.Continuation() {
		t.Errorf("second cell = %#v, want leading X", second)
	}
}

func TestChangedRunExpandsOverUnchangedContinuation(t *testing.T) {
	previous, _ := New(2, 1)
	previous.Write(0, 0, "日", vt.Style{}, celltext.ModernWidth())
	current, _ := New(2, 1)
	current.Write(0, 0, "本", vt.Style{}, celltext.ModernWidth())

	got := current.ChangedRuns(previous)
	want := []ChangedRun{{Row: 0, Start: 0, End: 2}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ChangedRuns() = %v, want %v", got, want)
	}
}

func TestCellValidationAndZeroWidthFallback(t *testing.T) {
	if _, err := NewCell("ab", vt.Style{}, celltext.ModernWidth()); !errors.Is(err, ErrExpectedSingleGrapheme) {
		t.Fatalf("NewCell(ab) error = %v, want ErrExpectedSingleGrapheme", err)
	}
	cell, err := NewCell("\u0301", vt.Style{}, celltext.ModernWidth())
	if err != nil {
		t.Fatal(err)
	}
	if cell.Content() != "\uFFFD" || cell.Span() != SpanOne {
		t.Fatalf("zero-width cell = %#v, want one-cell replacement", cell)
	}
}
