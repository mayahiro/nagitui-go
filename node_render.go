package tui

import (
	"github.com/mayahiro/nagi-go/vt"
	"math"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagitui-go/surface"
)

func (n Node[Message]) render(target *surface.Surface, rect, clip Rect, interaction *InteractionState) {
	switch n.kind {
	case nodeText:
		renderText(target, rect, clip, n.content, n.style)
	case nodeRichText:
		renderRichText(target, rect, clip, n.spans, n.paragraph)
	case nodeSurface:
		if n.payload != nil {
			renderSurfaceNode(target, rect, clip, n.payload.surface)
		}
	case nodeSpacer, nodeGap:
	case nodeTextInput:
		renderTextInput(target, rect, clip, n.id, n.content, n.placeholder, n.style, n.placeholderStyle, interaction)
	case nodeRow:
		renderLinear(target, rect, clip, n.children, true, interaction)
	case nodeColumn:
		renderLinear(target, rect, clip, n.children, false, interaction)
	case nodeStack:
		for _, child := range n.children {
			child.render(target, rect, clip, interaction)
		}
	case nodePadding:
		childRect := insetRect(rect, n.insets.Left, n.insets.Top, n.insets.Right, n.insets.Bottom)
		n.child.render(target, childRect, clip, interaction)
	case nodeBorder:
		renderBorder(target, rect, clip, n.style)
		n.child.render(target, insetRect(rect, 1, 1, 1, 1), clip, interaction)
	case nodeAlign:
		n.child.render(target, alignedChildRect(rect, n.child, n.horizontal, n.vertical), clip, interaction)
	case nodeClip:
		n.child.render(target, rect, clip.Intersection(rect), interaction)
	case nodeScrollViewport:
		childRect := scrollChildRect(rect, n.child, interaction.ScrollOffset(n.id), n.scroll.Axis)
		n.child.render(target, childRect, clip.Intersection(rect), interaction)
	case nodeVirtualScrollViewport:
		if fragment, ok := ensureVirtualFragment(
			n.payload.virtualSize,
			n.scroll.Axis,
			n.payload.virtualBuilder,
			&n.payload.virtualCache,
			rect,
			interaction.ScrollOffset(n.id),
		); ok {
			fragment.fragment.Node.render(
				target,
				virtualFragmentRect(rect, fragment),
				clip.Intersection(rect),
				interaction,
			)
		}
	case nodeModal:
		n.child.render(target, rect, clip, interaction)
	case nodePanel:
		renderPanel(target, rect, clip, n.child, n.title, n.panel, interaction)
	default:
		panic("nagi-tui: invalid node kind")
	}
	if focused, ok := interaction.Focused(); ok && n.hasID && focused == n.id && n.hasFocusedStyle {
		overlay := rect.Intersection(clip)
		mergeNodeStyle(target, overlay, n.focusedStyle)
	}
}

func mergeNodeStyle(target *surface.Surface, rect Rect, overlay vt.Style) {
	for yOffset := range rect.Height {
		y := saturatingCoordinate(rect.Y, yOffset)
		for xOffset := range rect.Width {
			x := saturatingCoordinate(rect.X, xOffset)
			cell, ok := target.Cell(x, y)
			if !ok {
				continue
			}
			target.SetStyle(x, y, cell.Style().Merge(overlay))
		}
	}
}

func renderLinear[Message any](target *surface.Surface, rect, clip Rect, children []Node[Message], horizontal bool, interaction *InteractionState) {
	for index, childRect := range linearRects(children, rect, horizontal) {
		children[index].render(target, childRect, clip, interaction)
	}
}

func linearRects[Message any](children []Node[Message], rect Rect, horizontal bool) []Rect {
	available := rect.Height
	if horizontal {
		available = rect.Width
	}
	tracks := make([]layoutTrack, len(children))
	for index, child := range children {
		measured := child.measure(boundedConstraints(rect.Size()))
		desired := measured.Height
		if horizontal {
			desired = measured.Width
		}
		if child.kind == nodeGap {
			desired = child.gap
		}
		tracks[index] = layoutTrack{length: child.length, desired: desired}
	}
	allocations := allocate(available, tracks)
	rects := make([]Rect, len(children))
	var offset uint32
	for index := range children {
		var childRect Rect
		if horizontal {
			childRect = horizontalRect(rect, offset, allocations[index])
		} else {
			childRect = verticalRect(rect, offset, allocations[index])
		}
		rects[index] = childRect
		offset = saturatingAdd32(offset, allocations[index])
	}
	return rects
}

func renderText(target *surface.Surface, rect, clip Rect, content string, style vt.Style) {
	if rect.Empty() {
		return
	}
	lines := celltext.Wrap(content, int(rect.Width), celltext.ModernWidth())
	for lineIndex, line := range lines {
		if uint32(lineIndex) >= rect.Height {
			break
		}
		y := saturatingCoordinate(rect.Y, uint32(lineIndex))
		x := int64(rect.X)
		for _, grapheme := range celltext.Graphemes(line) {
			span := int64(max(celltext.GraphemeWidth(grapheme.Text, celltext.ModernWidth()), 1))
			end := x + span
			if containsRenderUnit(clip, x, int64(y), end) {
				target.Write(clampInt64ToInt32(x), y, grapheme.Text, style, celltext.ModernWidth())
			}
			x = end
		}
	}
}

func renderTextInput(
	target *surface.Surface,
	rect, clip Rect,
	id NodeID,
	value, placeholder string,
	style, placeholderStyle vt.Style,
	interaction *InteractionState,
) {
	if rect.Empty() {
		return
	}
	state, ok := interaction.TextInput(id)
	cursor := len(value)
	if ok {
		cursor = state.Cursor()
	}
	focused, hasFocus := interaction.Focused()
	isFocused := hasFocus && focused == id
	content := value
	contentStyle := style
	if value == "" {
		content = placeholder
		contentStyle = placeholderStyle
	}
	cursorCell := 0
	if value != "" {
		cursorCell, _ = celltext.CellAtByte(value, cursor, celltext.ModernWidth())
	}
	visibleWidth := int(rect.Width)
	requestedStart := max(cursorCell-(visibleWidth-1), 0)
	startCell := requestedStart
	startByte := 0
	for {
		if offset, exact := celltext.ByteAtCell(content, startCell, celltext.ModernWidth()); exact {
			startByte = offset
			break
		}
		if startCell == 0 {
			break
		}
		startCell--
	}
	renderSingleLine(target, rect, clip, content[startByte:], contentStyle)
	if isFocused {
		relative := min(max(cursorCell-startCell, 0), visibleWidth-1)
		point := Point{X: saturatingCoordinate(rect.X, uint32(relative)), Y: rect.Y}
		if clip.Contains(point) && point.X >= 0 && point.Y >= 0 {
			target.SetCursor(surface.Cursor{X: uint32(point.X), Y: uint32(point.Y)})
		}
	}
}

func renderSingleLine(target *surface.Surface, rect, clip Rect, content string, style vt.Style) {
	x := int64(rect.X)
	right := int64(rect.X) + int64(rect.Width)
	for _, grapheme := range celltext.Graphemes(content) {
		span := int64(max(celltext.GraphemeWidth(grapheme.Text, celltext.ModernWidth()), 1))
		end := x + span
		if end > right {
			break
		}
		if containsRenderUnit(clip, x, int64(rect.Y), end) {
			target.Write(clampInt64ToInt32(x), rect.Y, grapheme.Text, style, celltext.ModernWidth())
		}
		x = end
	}
}

func scrollChildRect[Message any](viewport Rect, child *Node[Message], requested ScrollOffset, axis ScrollAxis) Rect {
	content := child.measure(scrollConstraints(viewport, axis))
	width := max(content.Width, viewport.Width)
	height := max(content.Height, viewport.Height)
	offset := clampScroll(width, height, viewport.Width, viewport.Height, requested)
	return Rect{
		X:      clampInt64ToInt32(int64(viewport.X) - int64(offset.X)),
		Y:      clampInt64ToInt32(int64(viewport.Y) - int64(offset.Y)),
		Width:  width,
		Height: height,
	}
}

func ensureVirtualFragment[Message any](
	declaredContentSize Size,
	axis ScrollAxis,
	builder func(VirtualViewport) VirtualFragment[Message],
	cache *virtualCacheState[Message],
	viewport Rect,
	requested ScrollOffset,
) (*virtualCacheState[Message], bool) {
	if viewport.Empty() || virtualContentEmpty(declaredContentSize, axis) {
		var zero VirtualFragment[Message]
		cache.valid = false
		cache.fragment = zero
		return cache, false
	}
	contentSize := resolvedVirtualContentSize(declaredContentSize, viewport.Size(), axis)
	requested = normalizeScrollOffset(axis, requested)
	offset := clampScroll(
		contentSize.Width,
		contentSize.Height,
		viewport.Width,
		viewport.Height,
		requested,
	)
	request := VirtualViewport{
		Offset: offset,
		Size: Size{
			Width:  min(viewport.Width, contentSize.Width-offset.X),
			Height: min(viewport.Height, contentSize.Height-offset.Y),
		},
		ContentSize: contentSize,
	}
	if !cache.valid || cache.request != request {
		cache.request = request
		cache.fragment = builder(request)
		cache.valid = true
	}
	return cache, true
}

func resolvedVirtualContentSize(declared, viewport Size, _ ScrollAxis) Size {
	return Size{
		Width:  max(declared.Width, viewport.Width),
		Height: max(declared.Height, viewport.Height),
	}
}

func virtualContentEmpty(content Size, axis ScrollAxis) bool {
	switch axis {
	case ScrollAxisBoth:
		return content.Width == 0 || content.Height == 0
	case ScrollAxisVertical:
		return content.Height == 0
	case ScrollAxisHorizontal:
		return content.Width == 0
	default:
		panic("nagi-tui: invalid scroll axis")
	}
}

func virtualFragmentRect[Message any](viewport Rect, cached *virtualCacheState[Message]) Rect {
	request := cached.request
	origin := ScrollOffset{
		X: min(cached.fragment.Origin.X, request.ContentSize.Width),
		Y: min(cached.fragment.Origin.Y, request.ContentSize.Height),
	}
	remaining := Size{
		Width:  request.ContentSize.Width - origin.X,
		Height: request.ContentSize.Height - origin.Y,
	}
	measured := cached.fragment.Node.measure(boundedConstraints(remaining))
	visibleEnd := ScrollOffset{
		X: saturatingAdd32(request.Offset.X, request.Size.Width),
		Y: saturatingAdd32(request.Offset.Y, request.Size.Height),
	}
	coverage := Size{
		Width:  visibleEnd.X - min(visibleEnd.X, origin.X),
		Height: visibleEnd.Y - min(visibleEnd.Y, origin.Y),
	}
	return Rect{
		X:      clampInt64ToInt32(int64(viewport.X) + int64(origin.X) - int64(request.Offset.X)),
		Y:      clampInt64ToInt32(int64(viewport.Y) + int64(origin.Y) - int64(request.Offset.Y)),
		Width:  min(max(measured.Width, coverage.Width), remaining.Width),
		Height: min(max(measured.Height, coverage.Height), remaining.Height),
	}
}

func scrollConstraints(viewport Rect, axis ScrollAxis) layoutConstraints {
	constraints := layoutConstraints{}
	if !axis.allowsHorizontal() {
		constraints.width = layoutLimit{value: viewport.Width, bounded: true}
	}
	if !axis.allowsVertical() {
		constraints.height = layoutLimit{value: viewport.Height, bounded: true}
	}
	return constraints
}

func renderBorder(target *surface.Surface, rect, clip Rect, style vt.Style) {
	renderBorderWithGlyphs(target, rect, clip, style, glyphsForBorder(BorderSingle))
}

func renderBorderWithGlyphs(target *surface.Surface, rect, clip Rect, style vt.Style, glyphs borderGlyphs) {
	if rect.Empty() {
		return
	}
	right := int64(rect.X) + int64(rect.Width) - 1
	bottom := int64(rect.Y) + int64(rect.Height) - 1
	for x := int64(rect.X); x <= right; x++ {
		writeBorderCell(target, clip, x, int64(rect.Y), glyphs.horizontal, style)
		if bottom != int64(rect.Y) {
			writeBorderCell(target, clip, x, bottom, glyphs.horizontal, style)
		}
	}
	for y := int64(rect.Y); y <= bottom; y++ {
		writeBorderCell(target, clip, int64(rect.X), y, glyphs.vertical, style)
		if right != int64(rect.X) {
			writeBorderCell(target, clip, right, y, glyphs.vertical, style)
		}
	}
	writeBorderCell(target, clip, int64(rect.X), int64(rect.Y), glyphs.topLeft, style)
	if right != int64(rect.X) {
		writeBorderCell(target, clip, right, int64(rect.Y), glyphs.topRight, style)
	}
	if bottom != int64(rect.Y) {
		writeBorderCell(target, clip, int64(rect.X), bottom, glyphs.bottomLeft, style)
		if right != int64(rect.X) {
			writeBorderCell(target, clip, right, bottom, glyphs.bottomRight, style)
		}
	}
}

func writeBorderCell(target *surface.Surface, clip Rect, x, y int64, content string, style vt.Style) {
	if containsRenderUnit(clip, x, y, x+1) {
		target.Write(clampInt64ToInt32(x), clampInt64ToInt32(y), content, style, celltext.ModernWidth())
	}
}

func containsRenderUnit(clip Rect, startX, y, endX int64) bool {
	return startX >= int64(clip.X) &&
		endX <= int64(clip.X)+int64(clip.Width) &&
		y >= int64(clip.Y) &&
		y < int64(clip.Y)+int64(clip.Height)
}

func horizontalAlignmentOffset(available, desired uint32, alignment HorizontalAlignment) uint32 {
	remaining := available - min(available, desired)
	switch alignment {
	case AlignStart:
		return 0
	case AlignCenter:
		return remaining / 2
	case AlignEnd:
		return remaining
	default:
		panic("nagi-tui: invalid horizontal alignment")
	}
}

func verticalAlignmentOffset(available, desired uint32, alignment VerticalAlignment) uint32 {
	remaining := available - min(available, desired)
	switch alignment {
	case AlignTop:
		return 0
	case AlignMiddle:
		return remaining / 2
	case AlignBottom:
		return remaining
	default:
		panic("nagi-tui: invalid vertical alignment")
	}
}

func alignedChildRect[Message any](rect Rect, child *Node[Message], horizontal HorizontalAlignment, vertical VerticalAlignment) Rect {
	desired := child.measure(boundedConstraints(rect.Size()))
	width := min(desired.Width, rect.Width)
	height := min(desired.Height, rect.Height)
	return Rect{
		X:      saturatingCoordinate(rect.X, horizontalAlignmentOffset(rect.Width, width, horizontal)),
		Y:      saturatingCoordinate(rect.Y, verticalAlignmentOffset(rect.Height, height, vertical)),
		Width:  width,
		Height: height,
	}
}

func clampInt64ToInt32(value int64) int32 {
	return int32(min(max(value, int64(math.MinInt32)), int64(math.MaxInt32)))
}
