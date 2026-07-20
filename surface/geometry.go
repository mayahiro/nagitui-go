package surface

// Point is a terminal cell coordinate in the signed 32-bit geometry domain
type Point struct {
	// X is the horizontal coordinate
	X int32
	// Y is the vertical coordinate
	Y int32
}

// Translate returns the point translated by dx and dy
//
// The second result is false when either coordinate would overflow int32
func (p Point) Translate(dx, dy int32) (Point, bool) {
	x, ok := addInt32(p.X, dx)
	if !ok {
		return Point{}, false
	}
	y, ok := addInt32(p.Y, dy)
	if !ok {
		return Point{}, false
	}
	return Point{X: x, Y: y}, true
}

// Size is a non-negative terminal cell size
type Size struct {
	// Width is the number of horizontal cells
	Width uint32
	// Height is the number of vertical cells
	Height uint32
}

// Empty reports whether either dimension is zero
func (s Size) Empty() bool {
	return s.Width == 0 || s.Height == 0
}

// Rect is a half-open terminal cell rectangle
//
// The covered region is [X, X+Width) x [Y, Y+Height). Endpoint calculations
// use a signed 64-bit intermediate domain
type Rect struct {
	// X is the horizontal coordinate of the origin
	X int32
	// Y is the vertical coordinate of the origin
	Y int32
	// Width is the number of horizontal cells
	Width uint32
	// Height is the number of vertical cells
	Height uint32
}

// Origin returns the rectangle origin
func (r Rect) Origin() Point {
	return Point{X: r.X, Y: r.Y}
}

// Size returns the rectangle size
func (r Rect) Size() Size {
	return Size{Width: r.Width, Height: r.Height}
}

// Empty reports whether either dimension is zero
func (r Rect) Empty() bool {
	return r.Width == 0 || r.Height == 0
}

// Contains reports whether the half-open rectangle contains point
func (r Rect) Contains(point Point) bool {
	if r.Empty() {
		return false
	}
	pointX := int64(point.X)
	pointY := int64(point.Y)
	return pointX >= int64(r.X) && pointX < r.right() &&
		pointY >= int64(r.Y) && pointY < r.bottom()
}

// Intersection returns the half-open intersection with other
//
// Disjoint rectangles produce an empty rectangle whose origin is the
// component-wise maximum of the two origins
func (r Rect) Intersection(other Rect) Rect {
	x := max(r.X, other.X)
	y := max(r.Y, other.Y)
	right := min(r.right(), other.right())
	bottom := min(r.bottom(), other.bottom())
	return Rect{
		X:      x,
		Y:      y,
		Width:  intersectionExtent(int64(x), right),
		Height: intersectionExtent(int64(y), bottom),
	}
}

// Translate returns the rectangle translated by dx and dy
//
// The second result is false when either origin coordinate would overflow
// int32. The size is preserved
func (r Rect) Translate(dx, dy int32) (Rect, bool) {
	origin, ok := r.Origin().Translate(dx, dy)
	if !ok {
		return Rect{}, false
	}
	return Rect{X: origin.X, Y: origin.Y, Width: r.Width, Height: r.Height}, true
}

func (r Rect) right() int64 {
	return int64(r.X) + int64(r.Width)
}

func (r Rect) bottom() int64 {
	return int64(r.Y) + int64(r.Height)
}

func addInt32(a, b int32) (int32, bool) {
	sum := int64(a) + int64(b)
	if sum < int64(-1<<31) || sum > int64(1<<31-1) {
		return 0, false
	}
	return int32(sum), true
}

func intersectionExtent(start, end int64) uint32 {
	if end <= start {
		return 0
	}
	return uint32(end - start)
}
