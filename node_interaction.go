package tui

import (
	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
)

func (n *Node[Message]) buildTreeIndex(
	size Size,
	interaction *InteractionState,
	index *treeIndex,
	actions *actionIndex[Message],
	profile celltext.WidthProfile,
) error {
	bounds := Rect{Width: size.Width, Height: size.Height}
	index.reset()
	actions.reset()
	return n.buildIndex(bounds, bounds, "", false, true, "", false, interaction, index, actions, profile)
}

func (n *Node[Message]) prepareInteraction(size Size, interaction *InteractionState, profile celltext.WidthProfile) bool {
	return n.prepareAt(Rect{Width: size.Width, Height: size.Height}, interaction, profile)
}

func (n *Node[Message]) prepareVirtualFlows(size Size, interaction *InteractionState, profile celltext.WidthProfile) {
	n.prepareVirtualFlowsAt(Rect{Width: size.Width, Height: size.Height}, interaction, profile)
}

func (n *Node[Message]) handleEvent(id NodeID, event vt.Event) (EventResult[Message], bool) {
	node := n.find(id)
	if node == nil || node.handler == nil {
		return EventResult[Message]{}, false
	}
	return node.handler(event), true
}

func (n *Node[Message]) handlePointerEvent(
	id NodeID,
	context PointerEventContext,
) (EventResult[Message], bool) {
	node := n.find(id)
	if node == nil || node.pointerHandler == nil {
		return EventResult[Message]{}, false
	}
	if node.kind == nodeRichText {
		layout := node.richTextCache.resolveTextHit(
			node.spans,
			context.bounds.Width,
			node.paragraph.Wrap,
			context.widthProfile,
		)
		context.textHit = paragraphTextHit(
			layout,
			context.bounds.Width,
			node.paragraph.Alignment,
			context.localPosition,
		)
		context.hasTextHit = true
	}
	return node.pointerHandler(context), true
}

func (n *Node[Message]) textInputMessage(id NodeID, value string) (Message, bool) {
	node := n.find(id)
	if node == nil || node.kind != nodeTextInput || node.onChange == nil {
		var zero Message
		return zero, false
	}
	return node.onChange(value), true
}

func (n *Node[Message]) scrollOptions(id NodeID) (ScrollViewportOptions[Message], bool) {
	node := n.find(id)
	if node == nil {
		return ScrollViewportOptions[Message]{}, false
	}
	if node.kind == nodeVirtualFlow {
		options := node.payload.virtualFlow.options
		return ScrollViewportOptions[Message]{
			Axis:                 ScrollAxisVertical,
			EnsureFocusedVisible: options.EnsureFocusedVisible,
			OnScroll:             options.OnScroll,
		}, true
	}
	if node.kind != nodeScrollViewport && node.kind != nodeVirtualScrollViewport {
		return ScrollViewportOptions[Message]{}, false
	}
	return node.scroll, true
}

func (n *Node[Message]) scrollMessage(id NodeID, state ScrollState) (Message, bool) {
	options, ok := n.scrollOptions(id)
	if !ok || options.OnScroll == nil {
		var zero Message
		return zero, false
	}
	return options.OnScroll(state), true
}

func (n *Node[Message]) find(id NodeID) *Node[Message] {
	if n.hasID && n.id == id {
		return n
	}
	switch n.kind {
	case nodeRow, nodeColumn, nodeStack:
		for index := range n.children {
			if found := n.children[index].find(id); found != nil {
				return found
			}
		}
	case nodePadding, nodeBorder, nodeAlign, nodeClip, nodeScrollViewport, nodeModal, nodePanel:
		return n.child.find(id)
	case nodeVirtualScrollViewport:
		if n.payload != nil && n.payload.virtualCache.valid {
			return n.payload.virtualCache.fragment.Node.find(id)
		}
	case nodeVirtualFlow:
		if n.payload != nil && n.payload.virtualFlow != nil && n.payload.virtualFlow.cache.valid {
			for index := range n.payload.virtualFlow.cache.items {
				if found := n.payload.virtualFlow.cache.items[index].node.find(id); found != nil {
					return found
				}
			}
		}
	}
	return nil
}

func (n *Node[Message]) buildIndex(
	rect, clip Rect,
	parent NodeID,
	hasParent, root bool,
	focusFallback NodeID,
	hasFocusFallback bool,
	interaction *InteractionState,
	tree *treeIndex,
	actions *actionIndex[Message],
	profile celltext.WidthProfile,
) error {
	if n.keyInteraction != nil && n.keyInteraction.hasFocusFallback {
		focusFallback = n.keyInteraction.focusFallback
		hasFocusFallback = true
	}
	childParent := parent
	hasChildParent := hasParent
	if n.hasID {
		kind := interactiveGeneric
		switch n.kind {
		case nodeTextInput:
			kind = interactiveTextInput
		case nodeScrollViewport:
			kind = scrollInteractiveKind(n.scroll.Axis)
		case nodeVirtualScrollViewport:
			kind = scrollInteractiveKind(n.scroll.Axis)
		case nodeVirtualFlow:
			kind = scrollInteractiveKind(ScrollAxisVertical)
		case nodeModal:
			kind = interactiveModal
		}
		if err := tree.register(nodeRecord{
			id:              n.id,
			parent:          parent,
			hasParent:       hasParent,
			rect:            rect,
			clip:            clip,
			focusable:       n.focusable,
			hasHandler:      n.handler != nil || n.pointerHandler != nil,
			blocksUnhandled: n.blocksUnhandled,
			kind:            kind,
		}, root); err != nil {
			return err
		}
		if hasFocusFallback {
			tree.registerFocusFallback(n.id, focusFallback)
		}
		if n.kind == nodeModal {
			focus := DefaultModalFocusOptions()
			if n.keyInteraction != nil && n.keyInteraction.hasModalFocus {
				focus = n.keyInteraction.modalFocus
			}
			tree.setActiveModalFocus(n.id, focus)
		}
		actions.register(n.id, n.keyInteraction)
		if kind.isScrollViewport() && n.keyInteraction != nil && n.keyInteraction.hasRevealTarget {
			tree.revealTargets = append(tree.revealTargets, revealTarget{
				viewport: n.id,
				target:   n.keyInteraction.revealTarget,
			})
		}
		childParent = n.id
		hasChildParent = true
	}

	switch n.kind {
	case nodeText, nodeRichText, nodeSurface, nodeSpacer, nodeGap, nodeCursorAnchor, nodeTextInput:
		return nil
	case nodeRow, nodeColumn:
		horizontal := n.kind == nodeRow
		layout := n.resolvedLinearLayout(rect, horizontal, profile)
		var offset uint32
		for childIndex := range n.children {
			childRect := layout.childRect(rect, horizontal, childIndex, offset)
			if err := n.children[childIndex].buildIndex(childRect, clip, childParent, hasChildParent, false, focusFallback, hasFocusFallback, interaction, tree, actions, profile); err != nil {
				return err
			}
			offset = saturatingAdd32(offset, layout.allocation(childIndex))
		}
	case nodeStack:
		for child := range n.children {
			if err := n.children[child].buildIndex(rect, clip, childParent, hasChildParent, false, focusFallback, hasFocusFallback, interaction, tree, actions, profile); err != nil {
				return err
			}
		}
	case nodePadding:
		childRect := insetRect(rect, n.insets.Left, n.insets.Top, n.insets.Right, n.insets.Bottom)
		return n.child.buildIndex(childRect, clip, childParent, hasChildParent, false, focusFallback, hasFocusFallback, interaction, tree, actions, profile)
	case nodeBorder:
		return n.child.buildIndex(insetRect(rect, 1, 1, 1, 1), clip, childParent, hasChildParent, false, focusFallback, hasFocusFallback, interaction, tree, actions, profile)
	case nodeAlign:
		return n.child.buildIndex(alignedChildRect(rect, n.child, n.horizontal, n.vertical, profile), clip, childParent, hasChildParent, false, focusFallback, hasFocusFallback, interaction, tree, actions, profile)
	case nodeClip:
		return n.child.buildIndex(rect, clip.Intersection(rect), childParent, hasChildParent, false, focusFallback, hasFocusFallback, interaction, tree, actions, profile)
	case nodeScrollViewport:
		childRect := scrollChildRect(rect, n.child, interaction.ScrollOffset(n.id), n.scroll.Axis, profile)
		return n.child.buildIndex(childRect, clip.Intersection(rect), childParent, hasChildParent, false, focusFallback, hasFocusFallback, interaction, tree, actions, profile)
	case nodeVirtualScrollViewport:
		state := interaction.previewScroll(
			n.id,
			virtualScrollMaximum(n.payload.virtualSize, rect, n.scroll.Axis),
			n.scroll.Axis,
			n.scroll.StickToEnd,
		)
		fragment, ok := ensureVirtualFragment(
			n.payload.virtualSize,
			n.scroll.Axis,
			n.payload.virtualBuilder,
			&n.payload.virtualCache,
			rect,
			state.Offset,
		)
		if !ok {
			return nil
		}
		return fragment.fragment.Node.buildIndex(
			virtualFragmentRect(rect, fragment, profile),
			clip.Intersection(rect),
			childParent,
			hasChildParent,
			false,
			focusFallback,
			hasFocusFallback,
			interaction,
			tree,
			actions,
			profile,
		)
	case nodeVirtualFlow:
		if n.payload == nil || n.payload.virtualFlow == nil || !n.payload.virtualFlow.cache.valid {
			return nil
		}
		frame := &n.payload.virtualFlow.cache
		for index := range frame.items {
			item := &frame.items[index]
			if err := item.node.buildIndex(
				virtualFlowItemRect(rect, frame.offset, item.origin, item.height),
				clip.Intersection(rect),
				childParent,
				hasChildParent,
				false,
				focusFallback,
				hasFocusFallback,
				interaction,
				tree,
				actions,
				profile,
			); err != nil {
				return err
			}
		}
		return nil
	case nodeModal:
		return n.child.buildIndex(rect, clip, childParent, hasChildParent, false, focusFallback, hasFocusFallback, interaction, tree, actions, profile)
	case nodePanel:
		insets := panelContentInsets(n.panel)
		childRect := insetRect(rect, insets.Left, insets.Top, insets.Right, insets.Bottom)
		return n.child.buildIndex(childRect, clip, childParent, hasChildParent, false, focusFallback, hasFocusFallback, interaction, tree, actions, profile)
	}
	return nil
}

func (n *Node[Message]) prepareVirtualFlowsAt(rect Rect, interaction *InteractionState, profile celltext.WidthProfile) {
	switch n.kind {
	case nodeVirtualFlow:
		prepareVirtualFlowNode(n.id, n.payload.virtualFlow, rect, interaction, profile)
	case nodeRow, nodeColumn:
		horizontal := n.kind == nodeRow
		layout := n.resolvedLinearLayout(rect, horizontal, profile)
		var offset uint32
		for index := range n.children {
			childRect := layout.childRect(rect, horizontal, index, offset)
			n.children[index].prepareVirtualFlowsAt(childRect, interaction, profile)
			offset = saturatingAdd32(offset, layout.allocation(index))
		}
	case nodeStack:
		for index := range n.children {
			n.children[index].prepareVirtualFlowsAt(rect, interaction, profile)
		}
	case nodePadding:
		n.child.prepareVirtualFlowsAt(
			insetRect(rect, n.insets.Left, n.insets.Top, n.insets.Right, n.insets.Bottom),
			interaction,
			profile,
		)
	case nodeBorder:
		n.child.prepareVirtualFlowsAt(insetRect(rect, 1, 1, 1, 1), interaction, profile)
	case nodeAlign:
		n.child.prepareVirtualFlowsAt(
			alignedChildRect(rect, n.child, n.horizontal, n.vertical, profile),
			interaction,
			profile,
		)
	case nodeClip, nodeModal:
		n.child.prepareVirtualFlowsAt(rect, interaction, profile)
	case nodePanel:
		insets := panelContentInsets(n.panel)
		n.child.prepareVirtualFlowsAt(
			insetRect(rect, insets.Left, insets.Top, insets.Right, insets.Bottom),
			interaction,
			profile,
		)
	case nodeScrollViewport:
		n.child.prepareVirtualFlowsAt(
			scrollChildRect(rect, n.child, interaction.ScrollOffset(n.id), n.scroll.Axis, profile),
			interaction,
			profile,
		)
	case nodeVirtualScrollViewport:
		state := interaction.previewScroll(
			n.id,
			virtualScrollMaximum(n.payload.virtualSize, rect, n.scroll.Axis),
			n.scroll.Axis,
			n.scroll.StickToEnd,
		)
		if fragment, ok := ensureVirtualFragment(
			n.payload.virtualSize,
			n.scroll.Axis,
			n.payload.virtualBuilder,
			&n.payload.virtualCache,
			rect,
			state.Offset,
		); ok {
			fragment.fragment.Node.prepareVirtualFlowsAt(
				virtualFragmentRect(rect, fragment, profile),
				interaction,
				profile,
			)
		}
	}
}

func (n *Node[Message]) prepareAt(rect Rect, interaction *InteractionState, profile celltext.WidthProfile) bool {
	switch n.kind {
	case nodeTextInput:
		interaction.ensureTextInput(n.id, n.content)
		return false
	case nodeScrollViewport:
		previous := interaction.ScrollOffset(n.id)
		content := n.child.measure(scrollConstraints(rect, n.scroll.Axis), profile)
		width := max(content.Width, rect.Width)
		height := max(content.Height, rect.Height)
		state := interaction.prepareScroll(
			n.id,
			ScrollOffset{X: width - rect.Width, Y: height - rect.Height},
			n.scroll.Axis,
			n.scroll.StickToEnd,
		)
		childChanged := n.child.prepareAt(scrollChildRect(rect, n.child, state.Offset, n.scroll.Axis, profile), interaction, profile)
		return state.Offset != previous || childChanged
	case nodeVirtualScrollViewport:
		previousRequest := n.payload.virtualCache.request
		wasValid := n.payload.virtualCache.valid
		state := interaction.prepareScroll(
			n.id,
			virtualScrollMaximum(n.payload.virtualSize, rect, n.scroll.Axis),
			n.scroll.Axis,
			n.scroll.StickToEnd,
		)
		childChanged := false
		if fragment, ok := ensureVirtualFragment(
			n.payload.virtualSize,
			n.scroll.Axis,
			n.payload.virtualBuilder,
			&n.payload.virtualCache,
			rect,
			state.Offset,
		); ok {
			childChanged = fragment.fragment.Node.prepareAt(virtualFragmentRect(rect, fragment, profile), interaction, profile)
		}
		cacheChanged := wasValid != n.payload.virtualCache.valid ||
			n.payload.virtualCache.valid && previousRequest != n.payload.virtualCache.request
		return cacheChanged || childChanged
	case nodeVirtualFlow:
		return prepareVirtualFlowNode(n.id, n.payload.virtualFlow, rect, interaction, profile)
	}

	changed := false
	switch n.kind {
	case nodeRow, nodeColumn:
		horizontal := n.kind == nodeRow
		layout := n.resolvedLinearLayout(rect, horizontal, profile)
		var offset uint32
		for index := range n.children {
			childRect := layout.childRect(rect, horizontal, index, offset)
			changed = n.children[index].prepareAt(childRect, interaction, profile) || changed
			offset = saturatingAdd32(offset, layout.allocation(index))
		}
	case nodeStack:
		for index := range n.children {
			changed = n.children[index].prepareAt(rect, interaction, profile) || changed
		}
	case nodePadding:
		changed = n.child.prepareAt(insetRect(rect, n.insets.Left, n.insets.Top, n.insets.Right, n.insets.Bottom), interaction, profile)
	case nodeBorder:
		changed = n.child.prepareAt(insetRect(rect, 1, 1, 1, 1), interaction, profile)
	case nodeAlign:
		changed = n.child.prepareAt(alignedChildRect(rect, n.child, n.horizontal, n.vertical, profile), interaction, profile)
	case nodeClip:
		changed = n.child.prepareAt(rect, interaction, profile)
	case nodeModal:
		changed = n.child.prepareAt(rect, interaction, profile)
	case nodePanel:
		insets := panelContentInsets(n.panel)
		changed = n.child.prepareAt(insetRect(rect, insets.Left, insets.Top, insets.Right, insets.Bottom), interaction, profile)
	}
	return changed
}
