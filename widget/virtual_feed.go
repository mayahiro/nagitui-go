package widget

import tui "github.com/mayahiro/nagitui-go"

// VirtualFeed is a variable-height feed with application-owned status slots
//
// A VirtualFeed follows the end by default and delegates item identity,
// invalidation, measurement estimates, unread state, and paging decisions to
// the application. Only the Core tui.VirtualFlowState is retained by the
// runtime.
type VirtualFeed[Message any] struct {
	id              tui.NodeID
	source          tui.VirtualFlowSource[Message]
	options         tui.VirtualFlowOptions[Message]
	empty           *tui.Node[Message]
	loadingBefore   *tui.Node[Message]
	loadingAfter    *tui.Node[Message]
	unreadIndicator *tui.Node[Message]
}

// NewVirtualFeed returns a feed that follows its end with one Cell of overscan
func NewVirtualFeed[Message any](
	id tui.NodeID,
	source tui.VirtualFlowSource[Message],
) VirtualFeed[Message] {
	options := tui.DefaultVirtualFlowOptions[Message]()
	options.StickToEnd = true
	return VirtualFeed[Message]{id: id, source: source, options: options}
}

// FollowEnd sets whether the feed follows growth while its viewport is at the end
func (f VirtualFeed[Message]) FollowEnd(follow bool) VirtualFeed[Message] {
	f.options.StickToEnd = follow
	return f
}

// Overscan sets extra terminal Cells built before and after the visible range
func (f VirtualFeed[Message]) Overscan(cells uint32) VirtualFeed[Message] {
	f.options.Overscan = cells
	return f
}

// EnsureFocusedVisible sets whether focus movement reveals a built focused descendant
func (f VirtualFeed[Message]) EnsureFocusedVisible(ensure bool) VirtualFeed[Message] {
	f.options.EnsureFocusedVisible = ensure
	return f
}

// OnScroll sets an application message created after user scrolling changes state
func (f VirtualFeed[Message]) OnScroll(
	handler func(tui.ScrollState) Message,
) VirtualFeed[Message] {
	f.options.OnScroll = handler
	return f
}

// Empty sets the centered overlay shown when the source has no items
func (f VirtualFeed[Message]) Empty(node tui.Node[Message]) VirtualFeed[Message] {
	f.empty = &node
	return f
}

// LoadingBefore sets a status slot pinned before the scrollable body
func (f VirtualFeed[Message]) LoadingBefore(node tui.Node[Message]) VirtualFeed[Message] {
	f.loadingBefore = &node
	return f
}

// LoadingAfter sets a status slot pinned after the scrollable body
func (f VirtualFeed[Message]) LoadingAfter(node tui.Node[Message]) VirtualFeed[Message] {
	f.loadingAfter = &node
	return f
}

// UnreadIndicator sets a bottom-end overlay whose visibility is application-owned
func (f VirtualFeed[Message]) UnreadIndicator(node tui.Node[Message]) VirtualFeed[Message] {
	f.unreadIndicator = &node
	return f
}

// Node builds the public semantic node for this feed
func (f VirtualFeed[Message]) Node() tui.Node[Message] {
	empty := f.source.Items().Empty()
	flow := tui.VirtualFlowWithOptions(f.id, f.source, f.options).WithLength(tui.Flex(1))
	layers := []tui.Node[Message]{flow}
	if empty && f.empty != nil {
		layers = append(layers, tui.Align(*f.empty, tui.AlignCenter, tui.AlignMiddle))
	}
	if f.unreadIndicator != nil {
		layers = append(layers, tui.Align(*f.unreadIndicator, tui.AlignEnd, tui.AlignBottom))
	}
	body := tui.Stack(layers...).WithLength(tui.Flex(1))
	children := make([]tui.Node[Message], 0, 3)
	if f.loadingBefore != nil {
		children = append(children, *f.loadingBefore)
	}
	children = append(children, body)
	if f.loadingAfter != nil {
		children = append(children, *f.loadingAfter)
	}
	return tui.Column(children...)
}
