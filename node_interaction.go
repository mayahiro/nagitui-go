package tui

import "github.com/mayahiro/nagi-go/vt"

func (n *Node[Message]) buildTreeIndex(size Size, interaction *InteractionState) (treeIndex, error) {
	bounds := Rect{Width: size.Width, Height: size.Height}
	index := newTreeIndex()
	if err := n.buildIndex(bounds, bounds, "", false, true, interaction, &index); err != nil {
		return treeIndex{}, err
	}
	return index, nil
}

func (n *Node[Message]) prepareInteraction(size Size, interaction *InteractionState) {
	n.prepareAt(Rect{Width: size.Width, Height: size.Height}, interaction)
}

func (n *Node[Message]) handleEvent(id NodeID, event vt.Event) (EventResult[Message], bool) {
	node := n.find(id)
	if node == nil || node.handler == nil {
		return EventResult[Message]{}, false
	}
	return node.handler(event), true
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
	if node == nil || node.kind != nodeScrollViewport {
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
	}
	return nil
}

func (n *Node[Message]) buildIndex(
	rect, clip Rect,
	parent NodeID,
	hasParent, root bool,
	interaction *InteractionState,
	tree *treeIndex,
) error {
	childParent := parent
	hasChildParent := hasParent
	if n.hasID {
		kind := interactiveGeneric
		switch n.kind {
		case nodeTextInput:
			kind = interactiveTextInput
		case nodeScrollViewport:
			kind = interactiveScrollViewport
		case nodeModal:
			kind = interactiveModal
		}
		if err := tree.register(nodeRecord{
			id:         n.id,
			parent:     parent,
			hasParent:  hasParent,
			rect:       rect,
			clip:       clip,
			focusable:  n.focusable,
			hasHandler: n.handler != nil,
			kind:       kind,
		}, root); err != nil {
			return err
		}
		childParent = n.id
		hasChildParent = true
	}

	switch n.kind {
	case nodeText, nodeRichText, nodeSurface, nodeSpacer, nodeGap, nodeTextInput:
		return nil
	case nodeRow, nodeColumn:
		for childIndex, childRect := range linearRects(n.children, rect, n.kind == nodeRow) {
			if err := n.children[childIndex].buildIndex(childRect, clip, childParent, hasChildParent, false, interaction, tree); err != nil {
				return err
			}
		}
	case nodeStack:
		for child := range n.children {
			if err := n.children[child].buildIndex(rect, clip, childParent, hasChildParent, false, interaction, tree); err != nil {
				return err
			}
		}
	case nodePadding:
		childRect := insetRect(rect, n.insets.Left, n.insets.Top, n.insets.Right, n.insets.Bottom)
		return n.child.buildIndex(childRect, clip, childParent, hasChildParent, false, interaction, tree)
	case nodeBorder:
		return n.child.buildIndex(insetRect(rect, 1, 1, 1, 1), clip, childParent, hasChildParent, false, interaction, tree)
	case nodeAlign:
		return n.child.buildIndex(alignedChildRect(rect, n.child, n.horizontal, n.vertical), clip, childParent, hasChildParent, false, interaction, tree)
	case nodeClip:
		return n.child.buildIndex(rect, clip.Intersection(rect), childParent, hasChildParent, false, interaction, tree)
	case nodeScrollViewport:
		childRect := scrollChildRect(rect, n.child, interaction.ScrollOffset(n.id), n.scroll.Axis)
		return n.child.buildIndex(childRect, clip.Intersection(rect), childParent, hasChildParent, false, interaction, tree)
	case nodeModal:
		return n.child.buildIndex(rect, clip, childParent, hasChildParent, false, interaction, tree)
	case nodePanel:
		insets := panelContentInsets(n.panel)
		childRect := insetRect(rect, insets.Left, insets.Top, insets.Right, insets.Bottom)
		return n.child.buildIndex(childRect, clip, childParent, hasChildParent, false, interaction, tree)
	}
	return nil
}

func (n *Node[Message]) prepareAt(rect Rect, interaction *InteractionState) {
	switch n.kind {
	case nodeTextInput:
		interaction.ensureTextInput(n.id, n.content)
	case nodeScrollViewport:
		content := n.child.measure(scrollConstraints(rect, n.scroll.Axis))
		width := max(content.Width, rect.Width)
		height := max(content.Height, rect.Height)
		state := interaction.prepareScroll(
			n.id,
			ScrollOffset{X: width - rect.Width, Y: height - rect.Height},
			n.scroll.Axis,
			n.scroll.StickToEnd,
		)
		n.child.prepareAt(scrollChildRect(rect, n.child, state.Offset, n.scroll.Axis), interaction)
		return
	}

	switch n.kind {
	case nodeRow, nodeColumn:
		for index, childRect := range linearRects(n.children, rect, n.kind == nodeRow) {
			n.children[index].prepareAt(childRect, interaction)
		}
	case nodeStack:
		for index := range n.children {
			n.children[index].prepareAt(rect, interaction)
		}
	case nodePadding:
		n.child.prepareAt(insetRect(rect, n.insets.Left, n.insets.Top, n.insets.Right, n.insets.Bottom), interaction)
	case nodeBorder:
		n.child.prepareAt(insetRect(rect, 1, 1, 1, 1), interaction)
	case nodeAlign:
		n.child.prepareAt(alignedChildRect(rect, n.child, n.horizontal, n.vertical), interaction)
	case nodeClip:
		n.child.prepareAt(rect, interaction)
	case nodeModal:
		n.child.prepareAt(rect, interaction)
	case nodePanel:
		insets := panelContentInsets(n.panel)
		n.child.prepareAt(insetRect(rect, insets.Left, insets.Top, insets.Right, insets.Bottom), interaction)
	}
}
