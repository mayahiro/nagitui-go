package tui

import (
	"math"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go/surface"
)

func (n Node[Message]) render(target *surface.Surface, rect, clip Rect, interaction *InteractionState, profile celltext.WidthProfile) {
	switch n.kind {
	case nodeText:
		renderText(target, rect, clip, n.content, n.style, profile)
	case nodeRichText:
		renderRichText(target, rect, clip, n.spans, n.paragraph, n.richTextCache, profile)
	case nodeSurface:
		if n.payload != nil {
			renderSurfaceNode(target, rect, clip, n.payload.surface, profile)
		}
	case nodeSpacer, nodeGap:
	case nodeCursorAnchor:
		renderCursorAnchor(target, rect, clip, n.cursorOwner, interaction)
	case nodeTextInput:
		renderTextInput(target, rect, clip, n.id, n.content, n.placeholder, n.style, n.placeholderStyle, interaction, profile)
	case nodeRow:
		renderLinear(target, rect, clip, n.children, n.resolvedLinearLayout(rect, true, profile), interaction, profile)
	case nodeColumn:
		renderLinear(target, rect, clip, n.children, n.resolvedLinearLayout(rect, false, profile), interaction, profile)
	case nodeResponsiveRow:
		responsive := n.payload.responsive
		layout := responsive.resolvedLayout(rect, profile)
		for index, itemRect := range layout.slice() {
			if itemRect.visible {
				responsive.items[index].node.render(target, itemRect.rect, clip, interaction, profile)
			}
		}
	case nodeSplitPane:
		renderSplitPane(target, rect, clip, n.children, n.split, interaction, profile)
	case nodeStack:
		for _, child := range n.children {
			child.render(target, rect, clip, interaction, profile)
		}
	case nodeOverlay:
		n.children[0].render(target, rect, clip, interaction, profile)
		n.children[1].render(target, rect, clip, interaction, profile)
	case nodeAnchoredOverlay:
		anchored := n.payload.anchored
		anchored.base.render(target, rect, clip, interaction, profile)
		if overlayRect, ok := anchoredOverlayRect(
			anchored, rect, clip, interaction, profile,
		); ok {
			anchored.overlay.render(
				target, overlayRect, clip.Intersection(rect), interaction, profile,
			)
		}
	case nodePadding:
		childRect := insetRect(rect, n.insets.Left, n.insets.Top, n.insets.Right, n.insets.Bottom)
		n.child.render(target, childRect, clip, interaction, profile)
	case nodeBorder:
		renderBorder(target, rect, clip, n.style, profile)
		n.child.render(target, insetRect(rect, 1, 1, 1, 1), clip, interaction, profile)
	case nodeAlign:
		n.child.render(target, alignedChildRect(rect, n.child, n.horizontal, n.vertical, profile), clip, interaction, profile)
	case nodeClip:
		n.child.render(target, rect, clip.Intersection(rect), interaction, profile)
	case nodeScrollViewport:
		childRect := scrollChildRect(rect, n.child, interaction.ScrollOffset(n.id), n.scroll.Axis, profile)
		n.child.render(target, childRect, clip.Intersection(rect), interaction, profile)
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
				virtualFragmentRect(rect, fragment, profile),
				clip.Intersection(rect),
				interaction,
				profile,
			)
		}
	case nodeVirtualFlow:
		if n.payload != nil && n.payload.virtualFlow != nil && n.payload.virtualFlow.cache.valid {
			frame := &n.payload.virtualFlow.cache
			for index := range frame.items {
				item := &frame.items[index]
				item.node.render(
					target,
					virtualFlowItemRect(rect, frame.offset, item.origin, item.height),
					clip.Intersection(rect),
					interaction,
					profile,
				)
			}
		}
	case nodeModal:
		n.child.render(target, rect, clip, interaction, profile)
	case nodePanel:
		renderPanel(target, rect, clip, n.child, n.title, n.panel, interaction, profile)
	default:
		panic("nagi-tui: invalid node kind")
	}
	if focused, ok := interaction.Focused(); ok && n.hasID && focused == n.id && n.hasFocusedStyle {
		overlay := rect.Intersection(clip)
		mergeNodeStyle(target, overlay, n.focusedStyle)
	}
}

func renderSplitPane[Message any](
	target *surface.Surface,
	rect, clip Rect,
	panes []Node[Message],
	options SplitPaneOptions,
	interaction *InteractionState,
	profile celltext.WidthProfile,
) {
	layout := resolveSplitPaneLayout(rect, options)
	if !layout.hasCollapsed || layout.collapsed != SplitPaneCollapsePrimary {
		panes[0].render(target, layout.primary, clip, interaction, profile)
	}
	if layout.hasDivider {
		glyphs := profileAwareBorderGlyphs(glyphsForBorder(BorderSingle), profile)
		if normalizedSplitPaneAxis(options.Axis) == SplitPaneVertical {
			end := int64(layout.divider.X) + int64(layout.divider.Width)
			for x := int64(layout.divider.X); x < end; x++ {
				writeBorderCell(target, clip, x, int64(layout.divider.Y), glyphs.horizontal, options.DividerStyle, profile)
			}
		} else {
			end := int64(layout.divider.Y) + int64(layout.divider.Height)
			for y := int64(layout.divider.Y); y < end; y++ {
				writeBorderCell(target, clip, int64(layout.divider.X), y, glyphs.vertical, options.DividerStyle, profile)
			}
		}
	}
	if !layout.hasCollapsed || layout.collapsed != SplitPaneCollapseSecondary {
		panes[1].render(target, layout.secondary, clip, interaction, profile)
	}
}

func renderCursorAnchor(
	target *surface.Surface,
	rect, clip Rect,
	focusOwner NodeID,
	interaction *InteractionState,
) {
	focused, hasFocus := interaction.Focused()
	if !hasFocus || focused != focusOwner || rect.X < clip.X || rect.Y < clip.Y {
		return
	}
	x := int64(rect.X)
	right := int64(clip.X) + int64(clip.Width)
	if x == right && clip.Width > 0 {
		x--
	}
	point := Point{X: clampInt64ToInt32(x), Y: rect.Y}
	if !clip.Contains(point) || point.X < 0 || point.Y < 0 {
		return
	}
	target.SetCursor(surface.Cursor{X: uint32(point.X), Y: uint32(point.Y)})
}

func anchoredOverlayRect[Message any](
	anchored *anchoredOverlayPayload[Message],
	rect, clip Rect,
	interaction *InteractionState,
	profile celltext.WidthProfile,
) (Rect, bool) {
	if anchored.cache.valid && anchored.cache.rect == rect && anchored.cache.clip == clip {
		return anchored.cache.overlay, anchored.cache.hasOverlay
	}
	boundary := rect.Intersection(clip)
	anchorRect, anchorClip, found := anchored.base.findNodeGeometry(
		anchored.anchor, rect, clip, interaction, profile,
	)
	overlayRect, ok := Rect{}, false
	if found {
		overlayRect, ok = resolveAnchoredOverlayRect(
			anchorRect,
			anchorClip,
			boundary,
			&anchored.overlay,
			anchored.options,
			profile,
		)
	}
	anchored.cache = anchoredOverlayFrame{
		valid: true, rect: rect, clip: clip, overlay: overlayRect, hasOverlay: ok,
	}
	return overlayRect, ok
}

func resolveAnchoredOverlayRect[Message any](
	anchor, anchorClip, boundary Rect,
	overlay *Node[Message],
	options AnchoredOverlayOptions,
	profile celltext.WidthProfile,
) (Rect, bool) {
	if !anchoredOverlayAnchorVisible(anchor, anchorClip, boundary) {
		return Rect{}, false
	}
	widthLimit := boundary.Width
	if options.MaximumWidth != 0 {
		widthLimit = min(widthLimit, options.MaximumWidth)
	}
	heightLimit := boundary.Height
	if options.MaximumHeight != 0 {
		heightLimit = min(heightLimit, options.MaximumHeight)
	}
	desired := overlay.measure(boundedConstraints(Size{Width: widthLimit, Height: heightLimit}), profile)
	if desired.Empty() {
		return Rect{}, false
	}

	side := options.Side
	if side != AnchoredOverlayAbove {
		side = AnchoredOverlayBelow
	}
	fallback := options.Fallback
	if fallback != AnchoredOverlayClip {
		fallback = AnchoredOverlayFlip
	}
	boundaryTop := int64(boundary.Y)
	boundaryBottom := boundaryTop + int64(boundary.Height)
	anchorTop := int64(anchor.Y)
	anchorBottom := anchorTop + int64(anchor.Height)
	gap := int64(options.Gap)
	belowStart := anchorBottom + gap
	aboveEnd := anchorTop - gap
	below := overlayExtent(belowStart, boundaryBottom)
	above := overlayExtent(boundaryTop, aboveEnd)
	preferred, opposite := below, above
	if side == AnchoredOverlayAbove {
		preferred, opposite = above, below
	}
	if fallback == AnchoredOverlayFlip && desired.Height > preferred && opposite > preferred {
		if side == AnchoredOverlayBelow {
			side = AnchoredOverlayAbove
		} else {
			side = AnchoredOverlayBelow
		}
	}
	availableHeight := below
	if side == AnchoredOverlayAbove {
		availableHeight = above
	}
	height := min(desired.Height, availableHeight)
	width := min(desired.Width, boundary.Width)
	if width == 0 || height == 0 {
		return Rect{}, false
	}

	alignment := options.Alignment
	if alignment != AlignCenter && alignment != AlignEnd {
		alignment = AlignStart
	}
	anchorLeft := int64(anchor.X)
	anchorRight := anchorLeft + int64(anchor.Width)
	candidateX := anchorLeft
	switch alignment {
	case AlignCenter:
		candidateX = anchorLeft + int64(anchor.Width)/2 - int64(width)/2
	case AlignEnd:
		candidateX = anchorRight - int64(width)
	}
	boundaryLeft := int64(boundary.X)
	boundaryRight := boundaryLeft + int64(boundary.Width)
	maximumX := boundaryRight - int64(width)
	x := min(max(candidateX, boundaryLeft), maximumX)
	y := max(belowStart, boundaryTop)
	if side == AnchoredOverlayAbove {
		y = max(aboveEnd-int64(height), boundaryTop)
	}
	return Rect{
		X: clampInt64ToInt32(x), Y: clampInt64ToInt32(y), Width: width, Height: height,
	}, true
}

func anchoredOverlayAnchorVisible(anchor, anchorClip, boundary Rect) bool {
	if anchor.Height == 0 || anchorClip.Empty() || boundary.Empty() {
		return false
	}
	anchorTop := int64(anchor.Y)
	anchorBottom := anchorTop + int64(anchor.Height)
	visibleTop := max(anchorTop, int64(anchorClip.Y), int64(boundary.Y))
	visibleBottom := min(
		anchorBottom,
		int64(anchorClip.Y)+int64(anchorClip.Height),
		int64(boundary.Y)+int64(boundary.Height),
	)
	if visibleBottom <= visibleTop {
		return false
	}
	if anchor.Width != 0 {
		return !anchor.Intersection(anchorClip).Intersection(boundary).Empty()
	}
	anchorX := int64(anchor.X)
	visibleLeft := max(int64(anchorClip.X), int64(boundary.X))
	visibleRight := min(
		int64(anchorClip.X)+int64(anchorClip.Width),
		int64(boundary.X)+int64(boundary.Width),
	)
	return anchorX >= visibleLeft && anchorX < visibleRight
}

func overlayExtent(start, end int64) uint32 {
	if end <= start {
		return 0
	}
	return uint32(min(end-start, int64(math.MaxUint32)))
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

func renderLinear[Message any](target *surface.Surface, rect, clip Rect, children []Node[Message], layout *linearLayout, interaction *InteractionState, profile celltext.WidthProfile) {
	var offset uint32
	for index := range children {
		childRect := layout.childRect(rect, layout.horizontal, index, offset)
		children[index].render(target, childRect, clip, interaction, profile)
		offset = saturatingAdd32(offset, layout.allocation(index))
	}
}

const inlineLinearChildren = 32

type linearLayout struct {
	inline     [inlineLinearChildren]uint32
	overflow   []uint32
	horizontal bool
}

type linearLayoutCache struct {
	valid  bool
	rect   Rect
	layout linearLayout
}

func (n Node[Message]) resolvedLinearLayout(rect Rect, horizontal bool, profile celltext.WidthProfile) *linearLayout {
	if n.linearCache == nil {
		layout := resolveLinearLayout(n.children, rect, horizontal, profile)
		return &layout
	}
	if n.linearCache.valid && n.linearCache.rect == rect && n.linearCache.layout.horizontal == horizontal {
		return &n.linearCache.layout
	}
	n.linearCache.valid = true
	n.linearCache.rect = rect
	n.linearCache.layout = resolveLinearLayout(n.children, rect, horizontal, profile)
	return &n.linearCache.layout
}

func resolveLinearLayout[Message any](children []Node[Message], rect Rect, horizontal bool, profile celltext.WidthProfile) linearLayout {
	available := rect.Height
	if horizontal {
		available = rect.Width
	}
	layout := linearLayout{horizontal: horizontal}
	if len(children) <= inlineLinearChildren {
		var tracks [inlineLinearChildren]layoutTrack
		var minimums [inlineLinearChildren]uint32
		fillLinearTracks(children, rect, horizontal, tracks[:len(children)], profile)
		allocateInto(available, tracks[:len(children)], layout.inline[:len(children)], minimums[:len(children)])
		return layout
	}

	tracks := make([]layoutTrack, len(children))
	minimums := make([]uint32, len(children))
	layout.overflow = make([]uint32, len(children))
	fillLinearTracks(children, rect, horizontal, tracks, profile)
	allocateInto(available, tracks, layout.overflow, minimums)
	return layout
}

func fillLinearTracks[Message any](children []Node[Message], rect Rect, horizontal bool, tracks []layoutTrack, profile celltext.WidthProfile) {
	for index, child := range children {
		measured := child.measure(boundedConstraints(rect.Size()), profile)
		desired := measured.Height
		if horizontal {
			desired = measured.Width
		}
		if child.kind == nodeGap {
			desired = child.gap
		}
		tracks[index] = layoutTrack{length: child.length, desired: desired}
	}
}

func (l *linearLayout) allocation(index int) uint32 {
	if l.overflow != nil {
		return l.overflow[index]
	}
	return l.inline[index]
}

func (l *linearLayout) childRect(parent Rect, horizontal bool, index int, offset uint32) Rect {
	allocated := l.allocation(index)
	if horizontal {
		return horizontalRect(parent, offset, allocated)
	}
	return verticalRect(parent, offset, allocated)
}

func renderText(target *surface.Surface, rect, clip Rect, content string, style vt.Style, profile celltext.WidthProfile) {
	if rect.Empty() {
		return
	}
	lines := celltext.IterateWrappedLines(content, int(rect.Width), profile)
	for lineIndex := uint32(0); lineIndex < rect.Height; lineIndex++ {
		line, ok := lines.Next()
		if !ok {
			break
		}
		y := saturatingCoordinate(rect.Y, lineIndex)
		x := int64(rect.X)
		graphemes := celltext.IterateGraphemes(line.Text)
		for grapheme, ok := graphemes.Next(); ok; grapheme, ok = graphemes.Next() {
			span := int64(max(celltext.GraphemeWidth(grapheme.Text, profile), 1))
			end := x + span
			if containsRenderUnit(clip, x, int64(y), end) {
				target.Write(clampInt64ToInt32(x), y, grapheme.Text, style, profile)
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
	profile celltext.WidthProfile,
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
		cursorCell, _ = celltext.CellAtByte(value, cursor, profile)
	}
	visibleWidth := int(rect.Width)
	requestedStart := max(cursorCell-(visibleWidth-1), 0)
	startCell := requestedStart
	startByte := 0
	for {
		if offset, exact := celltext.ByteAtCell(content, startCell, profile); exact {
			startByte = offset
			break
		}
		if startCell == 0 {
			break
		}
		startCell--
	}
	renderSingleLine(target, rect, clip, content[startByte:], contentStyle, profile)
	if isFocused {
		relative := min(max(cursorCell-startCell, 0), visibleWidth-1)
		point := Point{X: saturatingCoordinate(rect.X, uint32(relative)), Y: rect.Y}
		if clip.Contains(point) && point.X >= 0 && point.Y >= 0 {
			target.SetCursor(surface.Cursor{X: uint32(point.X), Y: uint32(point.Y)})
		}
	}
}

func renderSingleLine(target *surface.Surface, rect, clip Rect, content string, style vt.Style, profile celltext.WidthProfile) {
	x := int64(rect.X)
	right := int64(rect.X) + int64(rect.Width)
	graphemes := celltext.IterateGraphemes(content)
	for grapheme, ok := graphemes.Next(); ok; grapheme, ok = graphemes.Next() {
		span := int64(max(celltext.GraphemeWidth(grapheme.Text, profile), 1))
		end := x + span
		if end > right {
			break
		}
		if containsRenderUnit(clip, x, int64(rect.Y), end) {
			target.Write(clampInt64ToInt32(x), rect.Y, grapheme.Text, style, profile)
		}
		x = end
	}
}

func scrollChildRect[Message any](viewport Rect, child *Node[Message], requested ScrollOffset, axis ScrollAxis, profile celltext.WidthProfile) Rect {
	content := child.measure(scrollConstraints(viewport, axis), profile)
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

func virtualFlowIntrinsicSize(constraints layoutConstraints) Size {
	width := uint32(0)
	if constraints.width.bounded {
		width = constraints.width.value
	}
	return Size{Width: width}
}

func prepareVirtualFlowNode[Message any](
	id NodeID,
	flow *virtualFlowNodePayload[Message],
	rect Rect,
	interaction *InteractionState,
	profile celltext.WidthProfile,
) bool {
	type frameSignature struct {
		valid         bool
		rect          Rect
		offset        uint32
		contentHeight uint32
		generation    uint64
		first         int
		end           int
	}
	previousSignature := frameSignature{}
	reusable := make(map[int]Node[Message])
	if flow.cache.valid {
		frame := &flow.cache
		previousSignature = frameSignature{
			valid: true, rect: frame.rect, offset: frame.offset,
			contentHeight: frame.contentHeight, generation: frame.generation,
			first: -1, end: -1,
		}
		if len(frame.items) > 0 {
			previousSignature.first = frame.items[0].index
			previousSignature.end = frame.items[len(frame.items)-1].index + 1
		}
		if frame.rect.Width == rect.Width {
			for _, item := range frame.items {
				reusable[item.index] = item.node
			}
		}
	}

	options := flow.options
	window := prepareVirtualFlow(
		interaction,
		id,
		flow.source,
		rect.Width,
		rect.Height,
		options.Overscan,
		options.StickToEnd,
	)
	for {
		for index := window.builtStart; index < window.builtEnd; index++ {
			if _, exists := reusable[index]; !exists {
				reusable[index] = flow.source.build(index, rect.Width)
			}
		}
		measurements := make([]virtualFlowMeasurement, 0, window.builtEnd-window.builtStart)
		for index := window.builtStart; index < window.builtEnd; index++ {
			_, _, measured, ok := virtualFlowItemLayout(interaction, id, index)
			if !ok || measured {
				continue
			}
			node := reusable[index]
			height := max(node.measure(layoutConstraints{
				width: layoutLimit{value: rect.Width, bounded: true},
			}, profile).Height, 1)
			measurements = append(measurements, virtualFlowMeasurement{index: index, height: height})
		}
		if len(measurements) == 0 {
			break
		}
		next, ok := applyVirtualFlowMeasurements(
			interaction,
			id,
			measurements,
			rect.Height,
			options.Overscan,
			options.StickToEnd,
		)
		if !ok {
			break
		}
		window = next
	}

	items := make([]virtualFlowBuiltItem[Message], 0, window.builtEnd-window.builtStart)
	childChanged := false
	for index := window.builtStart; index < window.builtEnd; index++ {
		node, exists := reusable[index]
		if !exists {
			node = flow.source.build(index, rect.Width)
		}
		origin, height, _, ok := virtualFlowItemLayout(interaction, id, index)
		if !ok {
			continue
		}
		childChanged = node.prepareAt(
			virtualFlowItemRect(rect, window.offset, origin, height),
			interaction,
			profile,
		) || childChanged
		items = append(items, virtualFlowBuiltItem[Message]{
			index: index, origin: origin, height: height, node: node,
		})
	}
	flow.cache = virtualFlowFrame[Message]{
		valid: true, rect: rect, offset: window.offset,
		contentHeight: window.contentHeight, generation: window.generation,
		items: items,
	}
	currentSignature := frameSignature{
		valid: true, rect: rect, offset: window.offset,
		contentHeight: window.contentHeight, generation: window.generation,
		first: -1, end: -1,
	}
	if len(items) > 0 {
		currentSignature.first = items[0].index
		currentSignature.end = items[len(items)-1].index + 1
	}
	return previousSignature != currentSignature || childChanged
}

func virtualFlowItemRect(viewport Rect, offset, origin, height uint32) Rect {
	return Rect{
		X:      viewport.X,
		Y:      clampInt64ToInt32(int64(viewport.Y) + int64(origin) - int64(offset)),
		Width:  viewport.Width,
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

func virtualScrollMaximum(declared Size, viewport Rect, axis ScrollAxis) ScrollOffset {
	content := resolvedVirtualContentSize(declared, viewport.Size(), axis)
	return ScrollOffset{
		X: content.Width - viewport.Width,
		Y: content.Height - viewport.Height,
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

func virtualFragmentRect[Message any](viewport Rect, cached *virtualCacheState[Message], profile celltext.WidthProfile) Rect {
	request := cached.request
	origin := ScrollOffset{
		X: min(cached.fragment.Origin.X, request.ContentSize.Width),
		Y: min(cached.fragment.Origin.Y, request.ContentSize.Height),
	}
	remaining := Size{
		Width:  request.ContentSize.Width - origin.X,
		Height: request.ContentSize.Height - origin.Y,
	}
	measured := cached.fragment.Node.measure(boundedConstraints(remaining), profile)
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

func renderBorder(target *surface.Surface, rect, clip Rect, style vt.Style, profile celltext.WidthProfile) {
	renderBorderWithGlyphs(target, rect, clip, style, profileAwareBorderGlyphs(glyphsForBorder(BorderSingle), profile), profile)
}

func renderBorderWithGlyphs(target *surface.Surface, rect, clip Rect, style vt.Style, glyphs borderGlyphs, profile celltext.WidthProfile) {
	if rect.Empty() {
		return
	}
	right := int64(rect.X) + int64(rect.Width) - 1
	bottom := int64(rect.Y) + int64(rect.Height) - 1
	for x := int64(rect.X); x <= right; x++ {
		writeBorderCell(target, clip, x, int64(rect.Y), glyphs.horizontal, style, profile)
		if bottom != int64(rect.Y) {
			writeBorderCell(target, clip, x, bottom, glyphs.horizontal, style, profile)
		}
	}
	for y := int64(rect.Y); y <= bottom; y++ {
		writeBorderCell(target, clip, int64(rect.X), y, glyphs.vertical, style, profile)
		if right != int64(rect.X) {
			writeBorderCell(target, clip, right, y, glyphs.vertical, style, profile)
		}
	}
	writeBorderCell(target, clip, int64(rect.X), int64(rect.Y), glyphs.topLeft, style, profile)
	if right != int64(rect.X) {
		writeBorderCell(target, clip, right, int64(rect.Y), glyphs.topRight, style, profile)
	}
	if bottom != int64(rect.Y) {
		writeBorderCell(target, clip, int64(rect.X), bottom, glyphs.bottomLeft, style, profile)
		if right != int64(rect.X) {
			writeBorderCell(target, clip, right, bottom, glyphs.bottomRight, style, profile)
		}
	}
}

func writeBorderCell(target *surface.Surface, clip Rect, x, y int64, content string, style vt.Style, profile celltext.WidthProfile) {
	if containsRenderUnit(clip, x, y, x+1) {
		target.Write(clampInt64ToInt32(x), clampInt64ToInt32(y), content, style, profile)
	}
}

func profileAwareBorderGlyphs(glyphs borderGlyphs, profile celltext.WidthProfile) borderGlyphs {
	if celltext.GraphemeWidth(glyphs.horizontal, profile) == 1 &&
		celltext.GraphemeWidth(glyphs.vertical, profile) == 1 &&
		celltext.GraphemeWidth(glyphs.topLeft, profile) == 1 &&
		celltext.GraphemeWidth(glyphs.topRight, profile) == 1 &&
		celltext.GraphemeWidth(glyphs.bottomLeft, profile) == 1 &&
		celltext.GraphemeWidth(glyphs.bottomRight, profile) == 1 {
		return glyphs
	}
	return borderGlyphs{"+", "-", "+", "|", "+", "+"}
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

func alignedChildRect[Message any](rect Rect, child *Node[Message], horizontal HorizontalAlignment, vertical VerticalAlignment, profile celltext.WidthProfile) Rect {
	desired := child.measure(boundedConstraints(rect.Size()), profile)
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
