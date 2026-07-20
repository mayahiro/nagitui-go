package tui

import (
	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go/surface"
)

func renderSurfaceNode(target *surface.Surface, rect, clip Rect, source *surface.Surface) {
	if source == nil || rect.Empty() {
		return
	}
	visibleWidth := min(rect.Width, source.Width())
	visibleHeight := min(rect.Height, source.Height())
	for sourceY := range visibleHeight {
		targetY := saturatingCoordinate(rect.Y, sourceY)
		for sourceX := uint32(0); sourceX < visibleWidth; sourceX++ {
			cell, ok := source.Cell(int32(sourceX), int32(sourceY))
			if !ok || cell.Continuation() {
				continue
			}
			span := uint32(cell.Span().Cells())
			if sourceX+span > visibleWidth {
				continue
			}
			targetX := saturatingCoordinate(rect.X, sourceX)
			end := int64(targetX) + int64(span)
			if !containsRenderUnit(clip, int64(targetX), int64(targetY), end) {
				continue
			}
			if cell.Opacity() == surface.Transparent {
				if destination, ok := target.Cell(targetX, targetY); ok {
					target.SetStyle(targetX, targetY, destination.Style().Merge(cell.Style()))
				}
				continue
			}
			target.Write(targetX, targetY, cell.Content(), cell.Style(), celltext.ModernWidth())
		}
	}
	if cursor, ok := source.Cursor(); ok {
		point := Point{X: saturatingCoordinate(rect.X, cursor.X), Y: saturatingCoordinate(rect.Y, cursor.Y)}
		if cursor.X < visibleWidth && cursor.Y < visibleHeight && point.X >= 0 && point.Y >= 0 && clip.Contains(point) {
			target.SetCursor(surface.Cursor{X: uint32(point.X), Y: uint32(point.Y)})
		} else {
			target.HideCursor()
		}
	}
}

func renderPanel[Message any](
	target *surface.Surface,
	rect, clip Rect,
	child *Node[Message],
	title string,
	options PanelOptions,
	interaction *InteractionState,
) {
	if rect.Empty() {
		return
	}
	background := rect.Intersection(clip)
	target.Fill(background.X, background.Y, background.Width, background.Height, options.Style.Background)
	renderBorderWithGlyphs(target, rect, clip, options.Style.Border, glyphsForBorder(options.Border))
	renderPanelTitle(target, rect, clip, title, options.Style.Title)
	insets := panelContentInsets(options)
	child.render(target, insetRect(rect, insets.Left, insets.Top, insets.Right, insets.Bottom), clip, interaction)
}

func renderPanelTitle(target *surface.Surface, rect, clip Rect, title string, style vt.Style) {
	if title == "" || rect.Width < 4 {
		return
	}
	content := " " + celltext.Truncate(title, int(rect.Width-4), celltext.ModernWidth()) + " "
	x := saturatingCoordinate(rect.X, 1)
	renderSingleLine(target, Rect{X: x, Y: rect.Y, Width: rect.Width - 2, Height: 1}, clip, content, style)
}
