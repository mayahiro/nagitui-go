package surface

import (
	"errors"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
)

// MaxSurfaceCells is the maximum number of cells accepted by a constructor
//
// The limit keeps construction failure deterministic across Rust and Go and is
// substantially larger than ordinary terminal dimensions.
const MaxSurfaceCells uint64 = 1_048_576

// ErrSurfaceTooLarge indicates that requested dimensions exceed
// MaxSurfaceCells
var ErrSurfaceTooLarge = errors.New("surface: dimensions are too large")

// Cursor is a visible surface cursor position
type Cursor struct {
	// X is the horizontal cell coordinate
	X uint32
	// Y is the vertical cell coordinate
	Y uint32
}

// ChangedRun is a half-open changed row interval
type ChangedRun struct {
	// Row is the vertical coordinate
	Row uint32
	// Start is the inclusive horizontal coordinate
	Start uint32
	// End is the exclusive horizontal coordinate
	End uint32
}

// Surface is a fixed-size grid of normalized terminal cells
type Surface struct {
	width, height uint32
	cells         []Cell
	cursor        Cursor
	hasCursor     bool
}

// New returns an opaque surface filled with default-style blanks
func New(width, height uint32) (*Surface, error) {
	return newSurface(width, height, false)
}

// NewTransparent returns a surface filled with default-style transparent cells
func NewTransparent(width, height uint32) (*Surface, error) {
	return newSurface(width, height, true)
}

func newSurface(width, height uint32, transparent bool) (*Surface, error) {
	count := uint64(width) * uint64(height)
	if count > MaxSurfaceCells || uint64(width) > MaxSurfaceCells || uint64(height) > MaxSurfaceCells {
		return nil, ErrSurfaceTooLarge
	}
	cells := make([]Cell, int(count))
	if transparent {
		for index := range cells {
			cells[index] = TransparentCell(vt.Style{})
		}
	}
	return &Surface{width: width, height: height, cells: cells}, nil
}

// Width returns the surface width
func (s *Surface) Width() uint32 {
	return s.width
}

// Height returns the surface height
func (s *Surface) Height() uint32 {
	return s.height
}

// Empty reports whether either dimension is zero
func (s *Surface) Empty() bool {
	return s.width == 0 || s.height == 0
}

// Cell returns the cell at signed coordinates
func (s *Surface) Cell(x, y int32) (Cell, bool) {
	if x < 0 || y < 0 || uint32(x) >= s.width || uint32(y) >= s.height {
		return Cell{}, false
	}
	return s.cells[s.index(int(x), int(y))], true
}

// Cursor returns the visible cursor and whether one is set
func (s *Surface) Cursor() (Cursor, bool) {
	return s.cursor, s.hasCursor
}

// SetCursor sets a visible cursor
//
// An out-of-bounds position hides the cursor and returns false.
func (s *Surface) SetCursor(cursor Cursor) bool {
	if cursor.X >= s.width || cursor.Y >= s.height {
		s.HideCursor()
		return false
	}
	s.cursor = cursor
	s.hasCursor = true
	return true
}

// HideCursor removes the visible cursor
func (s *Surface) HideCursor() {
	s.cursor = Cursor{}
	s.hasCursor = false
}

// Clear replaces every cell with a default-style opaque blank and hides the
// cursor
func (s *Surface) Clear() {
	s.ClearWithStyle(vt.Style{})
}

// ClearWithStyle replaces every cell with an opaque blank using style and
// hides the cursor
func (s *Surface) ClearWithStyle(style vt.Style) {
	cell := BlankCell(style)
	for index := range s.cells {
		s.cells[index] = cell
	}
	s.HideCursor()
}

// Fill fills a clipped rectangle with opaque blanks using style
func (s *Surface) Fill(x, y int32, width, height uint32, style vt.Style) {
	s.fillCell(x, y, width, height, BlankCell(style))
}

// FillTransparent fills a clipped rectangle with style-only transparent cells
func (s *Surface) FillTransparent(x, y int32, width, height uint32, style vt.Style) {
	s.fillCell(x, y, width, height, TransparentCell(style))
}

func (s *Surface) fillCell(x, y int32, width, height uint32, cell Cell) {
	startX := max(int64(x), 0)
	startY := max(int64(y), 0)
	endX := min(int64(x)+int64(width), int64(s.width))
	endY := min(int64(y)+int64(height), int64(s.height))
	if startX >= endX || startY >= endY {
		return
	}
	for row := startY; row < endY; row++ {
		for column := startX; column < endX; column++ {
			s.placeCell(int(column), int(row), cell)
		}
	}
}

// Write writes text left-to-right on one row using complete grapheme clusters
//
// Drawing is clipped to the surface. A wide grapheme that would be only partly
// visible is skipped as one indivisible unit.
func (s *Surface) Write(x, y int32, content string, style vt.Style, profile celltext.WidthProfile) {
	if y < 0 || uint32(y) >= s.height {
		return
	}
	column := int64(x)
	graphemes := celltext.IterateGraphemes(content)
	for grapheme, ok := graphemes.Next(); ok; grapheme, ok = graphemes.Next() {
		if column >= int64(s.width) {
			break
		}
		cell := cellFromCluster(grapheme.Text, style, profile)
		end := column + int64(cell.Span().Cells())
		if column >= 0 && end <= int64(s.width) {
			s.placeCell(int(column), int(y), cell)
		}
		column = end
	}
}

// SetStyle replaces the style of a cell's complete grapheme unit
//
// It returns false when the coordinate is outside the surface.
func (s *Surface) SetStyle(x, y int32, style vt.Style) bool {
	if x < 0 || y < 0 || uint32(x) >= s.width || uint32(y) >= s.height {
		return false
	}
	start, end, ok := s.clusterBounds(int(x), int(y))
	if !ok {
		return false
	}
	for index := start; index < end; index++ {
		s.cells[index].style = style
	}
	return true
}

// Composite composites source at a signed destination offset
//
// Opaque cells replace destination content. Transparent cells preserve content
// and merge their non-default style over the complete destination grapheme
// unit. Partly clipped wide graphemes are skipped.
func (s *Surface) Composite(source *Surface, offsetX, offsetY int32) {
	if source == s {
		source = source.Clone()
	}
	for sourceY := 0; sourceY < int(source.height); sourceY++ {
		targetY := int64(offsetY) + int64(sourceY)
		if targetY < 0 || targetY >= int64(s.height) {
			continue
		}
		for sourceX := 0; sourceX < int(source.width); sourceX++ {
			sourceCell := source.cells[source.index(sourceX, sourceY)]
			if sourceCell.Continuation() {
				continue
			}
			targetX := int64(offsetX) + int64(sourceX)
			targetEnd := targetX + int64(sourceCell.Span().Cells())
			if targetX < 0 || targetEnd > int64(s.width) {
				continue
			}
			if sourceCell.Opacity() == Transparent {
				s.mergeStyleAt(int(targetX), int(targetY), sourceCell.Style())
			} else {
				s.placeCell(int(targetX), int(targetY), sourceCell)
			}
		}
	}
	if source.hasCursor {
		x := int64(offsetX) + int64(source.cursor.X)
		y := int64(offsetY) + int64(source.cursor.Y)
		if x >= 0 && x < int64(s.width) && y >= 0 && y < int64(s.height) {
			s.cursor = Cursor{X: uint32(x), Y: uint32(y)}
			s.hasCursor = true
		} else {
			s.HideCursor()
		}
	}
}

// ChangedRuns returns row-contiguous changed intervals relative to previous
//
// Run boundaries are expanded across wide grapheme units in either surface.
// When dimensions differ, every row in this surface is changed.
func (s *Surface) ChangedRuns(previous *Surface) []ChangedRun {
	if s.width != previous.width || s.height != previous.height {
		if s.width == 0 {
			return nil
		}
		runs := make([]ChangedRun, s.height)
		for row := range s.height {
			runs[row] = ChangedRun{Row: row, End: s.width}
		}
		return runs
	}

	width := int(s.width)
	var runs []ChangedRun
	changed := make([]bool, width)
	for row := 0; row < int(s.height); row++ {
		for column := range width {
			index := s.index(column, row)
			changed[column] = s.cells[index] != previous.cells[index]
		}
		for {
			expanded := false
			for column := range width {
				if !changed[column] {
					continue
				}
				expanded = s.markCluster(row, column, changed) || expanded
				expanded = previous.markCluster(row, column, changed) || expanded
			}
			if !expanded {
				break
			}
		}
		for column := 0; column < width; {
			if !changed[column] {
				column++
				continue
			}
			start := column
			for column < width && changed[column] {
				column++
			}
			runs = append(runs, ChangedRun{Row: uint32(row), Start: uint32(start), End: uint32(column)})
		}
	}
	return runs
}

// Snapshot returns the canonical language-independent snapshot
func (s *Surface) Snapshot() string {
	return snapshot(s)
}

// Clone returns an independent copy of the surface
func (s *Surface) Clone() *Surface {
	cells := make([]Cell, len(s.cells))
	copy(cells, s.cells)
	return &Surface{
		width:     s.width,
		height:    s.height,
		cells:     cells,
		cursor:    s.cursor,
		hasCursor: s.hasCursor,
	}
}

func (s *Surface) index(x, y int) int {
	return y*int(s.width) + x
}

func (s *Surface) placeCell(x, y int, cell Cell) bool {
	span := cell.Span().Cells()
	if cell.Continuation() || x < 0 || y < 0 || x >= int(s.width) || y >= int(s.height) {
		return false
	}
	if span == 2 && x+1 >= int(s.width) {
		return false
	}
	s.eraseClusterAt(x, y)
	if span == 2 {
		s.eraseClusterAt(x+1, y)
	}
	index := s.index(x, y)
	s.cells[index] = cell
	if span == 2 {
		s.cells[index+1] = continuationCell(cell)
	}
	return true
}

func (s *Surface) eraseClusterAt(x, y int) {
	index := s.index(x, y)
	cell := s.cells[index]
	leadingX := x
	if cell.Continuation() {
		if x == 0 {
			return
		}
		leadingX--
	} else if cell.Span() != SpanTwo {
		return
	}
	if leadingX+1 >= int(s.width) {
		return
	}
	leadingIndex := s.index(leadingX, y)
	blank := BlankCell(s.cells[leadingIndex].Style())
	s.cells[leadingIndex] = blank
	s.cells[leadingIndex+1] = blank
}

func (s *Surface) mergeStyleAt(x, y int, overlay vt.Style) {
	start, end, ok := s.clusterBounds(x, y)
	if !ok {
		return
	}
	for index := start; index < end; index++ {
		s.cells[index].style = s.cells[index].Style().Merge(overlay)
	}
}

func (s *Surface) clusterBounds(x, y int) (start, end int, ok bool) {
	if x < 0 || y < 0 || x >= int(s.width) || y >= int(s.height) {
		return 0, 0, false
	}
	index := s.index(x, y)
	cell := s.cells[index]
	if cell.Continuation() {
		if x == 0 {
			return 0, 0, false
		}
		return index - 1, index + 1, true
	}
	if cell.Span() == SpanTwo && x+1 < int(s.width) {
		return index, index + 2, true
	}
	return index, index + 1, true
}

func (s *Surface) markCluster(row, column int, changed []bool) bool {
	cell := s.cells[s.index(column, row)]
	start, end := column, column+1
	if cell.Continuation() {
		start = max(column-1, 0)
	} else if cell.Span() == SpanTwo {
		end = min(column+2, int(s.width))
	}
	expanded := false
	for index := start; index < end; index++ {
		if !changed[index] {
			changed[index] = true
			expanded = true
		}
	}
	return expanded
}
