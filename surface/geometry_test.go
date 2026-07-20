package surface

import "testing"

func TestRectEndpointMathDoesNotOverflow(t *testing.T) {
	t.Parallel()

	rect := Rect{X: 1<<31 - 1, Y: 1<<31 - 1, Width: 1<<32 - 1, Height: 1<<32 - 1}
	point := Point{X: 1<<31 - 1, Y: 1<<31 - 1}
	if !rect.Contains(point) {
		t.Fatal("rectangle does not contain its maximum-domain origin")
	}
	if got := rect.Intersection(rect); got != rect {
		t.Fatalf("self intersection = %+v, want %+v", got, rect)
	}
}

func TestRectParts(t *testing.T) {
	t.Parallel()

	rect := Rect{X: -4, Y: 9, Width: 12, Height: 3}
	if got, want := rect.Origin(), (Point{X: -4, Y: 9}); got != want {
		t.Fatalf("Origin() = %+v, want %+v", got, want)
	}
	if got, want := rect.Size(), (Size{Width: 12, Height: 3}); got != want {
		t.Fatalf("Size() = %+v, want %+v", got, want)
	}
}
