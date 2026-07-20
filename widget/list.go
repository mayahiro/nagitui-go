package widget

import (
	"strings"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

// ListItem is one stable item rendered by a List
type ListItem struct {
	// ID is the application-defined stable identity of this item
	ID tui.NodeID
	// Label is the displayed item text
	Label string
}

// NewListItem returns a list item with a stable identity
func NewListItem(id tui.NodeID, label string) ListItem {
	return ListItem{ID: id, Label: label}
}

// ListStyle contains the visual styles used by a List
type ListStyle struct {
	// Normal is used by unselected items
	Normal vt.Style
	// Selected is used by the application-selected item
	Selected vt.Style
	// Focused is merged over the selected item while the list owns focus
	Focused vt.Style
	// Disabled is used by every item while the list is disabled
	Disabled vt.Style
}

// DefaultListStyle returns the standard list styles
func DefaultListStyle() ListStyle {
	return ListStyle{
		Selected: vt.Style{Reverse: true},
		Focused:  vt.Style{Underline: true},
		Disabled: vt.Style{Dim: true},
	}
}

// List is a vertically arranged, keyboard and pointer selectable collection
type List[Message any] struct {
	id             tui.NodeID
	items          []ListItem
	selected       int
	enabled        bool
	style          ListStyle
	filter         string
	windowOffset   int
	windowLimit    int
	hasWindow      bool
	viewportID     tui.NodeID
	viewportHeight tui.Length
	hasViewport    bool
	onSelect       func(int) Message
}

// NewList returns an enabled list using application-owned selection state
//
// A nil onSelect function creates a disabled list
func NewList[Message any](id tui.NodeID, items []ListItem, selected int, onSelect func(int) Message) List[Message] {
	return List[Message]{
		id:       id,
		items:    append([]ListItem(nil), items...),
		selected: selected,
		enabled:  onSelect != nil,
		style:    DefaultListStyle(),
		onSelect: onSelect,
	}
}

// Enabled sets whether the list can receive focus and emit selection messages
func (l List[Message]) Enabled(enabled bool) List[Message] {
	l.enabled = enabled && l.onSelect != nil
	return l
}

// Style replaces the list styles
func (l List[Message]) Style(style ListStyle) List[Message] {
	l.style = style
	return l
}

// Filter keeps items whose labels contain query using ASCII case folding
//
// Selection callbacks continue to receive original item indices.
func (l List[Message]) Filter(query string) List[Message] {
	l.filter = query
	return l
}

// Window limits rendering and navigation to a filtered item window
//
// Negative values clamp to zero. A zero limit renders an empty window.
func (l List[Message]) Window(offset, limit int) List[Message] {
	l.windowOffset = max(offset, 0)
	l.windowLimit = max(limit, 0)
	l.hasWindow = true
	return l
}

// Paginate limits rendering to one zero-based page after filtering
//
// Negative values clamp to zero. A zero page size renders an empty page.
func (l List[Message]) Paginate(page, pageSize int) List[Message] {
	page = max(page, 0)
	pageSize = max(pageSize, 0)
	offset := page * pageSize
	if pageSize != 0 && offset/pageSize != page {
		offset = int(^uint(0) >> 1)
	}
	return l.Window(offset, pageSize)
}

// Viewport wraps the list in a virtual Core ScrollViewport using a sizing rule
//
// viewportID must be distinct from the list root and item IDs. Applications
// may control its retained offset through Runtime.SetScrollOffset. Semantic
// row construction is bounded by the visible height. Item metadata collection
// and filtering remain eager. Viewport rows are one Cell high and clip content
// that would otherwise wrap.
func (l List[Message]) Viewport(viewportID tui.NodeID, height tui.Length) List[Message] {
	l.viewportID = viewportID
	l.viewportHeight = height
	l.hasViewport = true
	return l
}

// Node builds the public semantic node for this list
func (l List[Message]) Node() tui.Node[Message] {
	visible := listVisibleIndices(l.items, l.filter)
	if l.hasWindow {
		visible = listWindow(visible, l.windowOffset, l.windowLimit)
	}
	selected, hasSelection := normalizedListSelection(visible, l.selected)
	if l.hasViewport {
		return l.virtualNode(visible, selected, hasSelection)
	}
	children := make([]tui.Node[Message], 0, len(visible))
	for position, originalIndex := range visible {
		item := l.items[originalIndex]
		isSelected := hasSelection && position == selected
		marker := "  "
		if isSelected {
			marker = "> "
		}
		style := l.style.Normal
		if isSelected {
			style = l.style.Selected
		}
		if !l.enabled {
			style = l.style.Disabled
		}
		node := tui.StyledText[Message](marker+item.Label, style)
		if !l.enabled {
			children = append(children, node.WithID(item.ID))
			continue
		}
		selection := originalIndex
		itemID := item.ID
		row := node.WithID(itemID).OnEvent(itemID, func(event vt.Event) tui.EventResult[Message] {
			if isActivationEvent(event) {
				return tui.MessageResult(l.onSelect(selection)).Focus(l.id)
			}
			return tui.IgnoreResult[Message]()
		})
		if !isSelected {
			children = append(children, row)
			continue
		}
		children = append(children, tui.Column(row).
			Focusable(l.id).
			WithFocusedStyle(l.style.Focused).
			OnEvent(l.id, func(event vt.Event) tui.EventResult[Message] {
				next, handled := navigationEvent(event, len(visible), selected)
				if !handled {
					return tui.IgnoreResult[Message]()
				}
				result := tui.ConsumeResult[Message]().Focus(l.id)
				if next != selected {
					result = result.Emit(l.onSelect(visible[next]))
				}
				return result
			}))
	}

	root := tui.Column(children...)
	if !l.enabled || !hasSelection {
		root = root.WithID(l.id)
	}
	return root
}

func (l List[Message]) virtualNode(visible []int, selected int, hasSelection bool) tui.Node[Message] {
	contentHeight := cellCount(len(visible))
	viewport := tui.VirtualScrollViewportWithOptions(
		l.viewportID,
		tui.Size{Height: contentHeight},
		tui.ScrollViewportOptions[Message]{
			Axis:                 tui.ScrollAxisVertical,
			EnsureFocusedVisible: true,
		},
		func(viewport tui.VirtualViewport) tui.VirtualFragment[Message] {
			start, end := virtualRange(viewport, len(visible))
			rows := make([]tui.Node[Message], 0, end-start)
			for position := start; position < end; position++ {
				rows = append(rows, l.virtualRow(visible, selected, hasSelection, position))
			}
			visibleRows := tui.Padding(
				tui.Column(rows...),
				tui.Insets{Top: cellCount(start)},
			)
			layers := []tui.Node[Message]{visibleRows}
			if l.enabled && hasSelection && (selected < start || selected >= end) {
				proxy := l.navigationTarget(
					tui.Spacer[Message](0, 1),
					visible,
					selected,
					false,
				)
				layers = append(layers, tui.Padding(proxy, tui.Insets{Top: cellCount(selected)}))
			}
			return tui.NewVirtualFragment(tui.ScrollOffset{}, tui.Stack(layers...))
		},
	).TabStop(false).WithLength(l.viewportHeight)
	root := tui.Column(viewport)
	if !l.enabled || !hasSelection {
		root = root.WithID(l.id)
	}
	return root
}

func (l List[Message]) virtualRow(visible []int, selected int, hasSelection bool, position int) tui.Node[Message] {
	originalIndex := visible[position]
	item := l.items[originalIndex]
	isSelected := hasSelection && position == selected
	marker := "  "
	if isSelected {
		marker = "> "
	}
	style := l.style.Normal
	if isSelected {
		style = l.style.Selected
	}
	if !l.enabled {
		style = l.style.Disabled
	}
	node := tui.StyledText[Message](marker+item.Label, style)
	if !l.enabled {
		return node.WithID(item.ID).WithLength(tui.Fixed(1))
	}
	selection := originalIndex
	itemID := item.ID
	row := node.WithID(itemID).OnEvent(itemID, func(event vt.Event) tui.EventResult[Message] {
		if isActivationEvent(event) {
			return tui.MessageResult(l.onSelect(selection)).Focus(l.id)
		}
		return tui.IgnoreResult[Message]()
	})
	if !isSelected {
		return row.WithLength(tui.Fixed(1))
	}
	return l.navigationTarget(tui.Column(row), visible, position, true).WithLength(tui.Fixed(1))
}

func (l List[Message]) navigationTarget(
	node tui.Node[Message],
	visible []int,
	selected int,
	applyFocusedStyle bool,
) tui.Node[Message] {
	node = node.Focusable(l.id).OnEvent(l.id, func(event vt.Event) tui.EventResult[Message] {
		next, handled := navigationEvent(event, len(visible), selected)
		if !handled {
			return tui.IgnoreResult[Message]()
		}
		result := tui.ConsumeResult[Message]().Focus(l.id)
		if next != selected {
			result = result.Emit(l.onSelect(visible[next]))
		}
		return result
	})
	if applyFocusedStyle {
		node = node.WithFocusedStyle(l.style.Focused)
	}
	return node
}

func virtualRange(viewport tui.VirtualViewport, length int) (int, int) {
	start := length
	if uint64(viewport.Offset.Y) < uint64(length) {
		start = int(viewport.Offset.Y)
	}
	count := length - start
	if uint64(viewport.Size.Height) < uint64(count) {
		count = int(viewport.Size.Height)
	}
	return start, start + count
}

func cellCount(count int) uint32 {
	if uint64(count) > uint64(^uint32(0)) {
		return ^uint32(0)
	}
	return uint32(count)
}

func listVisibleIndices(items []ListItem, query string) []int {
	query = asciiLower(query)
	visible := make([]int, 0, len(items))
	for index, item := range items {
		if query == "" || strings.Contains(asciiLower(item.Label), query) {
			visible = append(visible, index)
		}
	}
	return visible
}

func listWindow(indices []int, offset, limit int) []int {
	start := min(max(offset, 0), len(indices))
	limit = max(limit, 0)
	end := len(indices)
	if limit < len(indices)-start {
		end = start + limit
	}
	return append([]int(nil), indices[start:end]...)
}

func normalizedListSelection(visible []int, selected int) (int, bool) {
	if len(visible) == 0 {
		return 0, false
	}
	selected = max(selected, 0)
	position := 0
	for index, original := range visible {
		if original > selected {
			break
		}
		position = index
	}
	return position, true
}
