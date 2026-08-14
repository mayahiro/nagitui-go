package widget

import (
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
//
// The root owns standard activation and vertical selection actions.
// Left-button press stays raw on each row so keyboard rebinding does not
// remove pointer selection.
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

// ActionDescriptors returns the ordered semantic actions declared by this list
//
// The order is activate, previous, next, first, and last. Every descriptor is
// disabled-pass-through when the list is disabled or has no item after
// filtering and windowing.
func (l List[Message]) ActionDescriptors() []tui.ActionDescriptor {
	descriptors := verticalCollectionActionDescriptors(
		l.enabled && listHasVisibleItems(l.items, l.filter, l.windowOffset, l.windowLimit, l.hasWindow),
	)
	return append([]tui.ActionDescriptor(nil), descriptors[:]...)
}

// Node builds the public semantic node for this list
func (l List[Message]) Node() tui.Node[Message] {
	visible := listVisibleIndices(l.items, l.filter)
	if l.hasWindow {
		visible = listWindow(visible, l.windowOffset, l.windowLimit)
	}
	selected, hasSelection := normalizedListSelection(visible, l.selected)
	descriptors := verticalCollectionActionDescriptors(l.enabled && hasSelection)
	if l.hasViewport {
		return l.virtualNode(visible, selected, hasSelection, descriptors)
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
			if isPointerActivationEvent(event) {
				return tui.MessageResult(l.onSelect(selection)).Focus(l.id)
			}
			return tui.IgnoreResult[Message]()
		})
		if !isSelected {
			children = append(children, row)
			continue
		}
		children = append(children, listActionTarget(
			tui.Column(row), l.id, visible, selected, true, l.style.Focused, l.onSelect, descriptors,
		))
	}

	root := tui.Column(children...)
	if !l.enabled || !hasSelection {
		root = root.WithID(l.id).OnActions(l.id, disabledCollectionActions[Message](descriptors))
	}
	return root
}

func (l List[Message]) virtualNode(
	visible []int,
	selected int,
	hasSelection bool,
	descriptors [5]tui.ActionDescriptor,
) tui.Node[Message] {
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
				rows = append(rows, l.virtualRow(visible, selected, hasSelection, position, descriptors))
			}
			visibleRows := tui.Padding(
				tui.Column(rows...),
				tui.Insets{Top: cellCount(start)},
			)
			layers := []tui.Node[Message]{visibleRows}
			if l.enabled && hasSelection && (selected < start || selected >= end) {
				proxy := listActionTarget(
					tui.Spacer[Message](0, 1), l.id, visible, selected, false, l.style.Focused, l.onSelect, descriptors,
				)
				layers = append(layers, tui.Padding(proxy, tui.Insets{Top: cellCount(selected)}))
			}
			return tui.NewVirtualFragment(tui.ScrollOffset{}, tui.Stack(layers...))
		},
	).TabStop(false).WithLength(l.viewportHeight)
	root := tui.Column(viewport)
	if !l.enabled || !hasSelection {
		root = root.WithID(l.id).OnActions(l.id, disabledCollectionActions[Message](descriptors))
	}
	return root
}

func (l List[Message]) virtualRow(
	visible []int,
	selected int,
	hasSelection bool,
	position int,
	descriptors [5]tui.ActionDescriptor,
) tui.Node[Message] {
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
		if isPointerActivationEvent(event) {
			return tui.MessageResult(l.onSelect(selection)).Focus(l.id)
		}
		return tui.IgnoreResult[Message]()
	})
	if !isSelected {
		return row.WithLength(tui.Fixed(1))
	}
	return listActionTarget(
		tui.Column(row), l.id, visible, position, true, l.style.Focused, l.onSelect, descriptors,
	).WithLength(tui.Fixed(1))
}

func listActionTarget[Message any](
	node tui.Node[Message],
	rootID tui.NodeID,
	visible []int,
	selected int,
	applyFocusedStyle bool,
	focusedStyle vt.Style,
	onSelect func(int) Message,
	descriptors [5]tui.ActionDescriptor,
) tui.Node[Message] {
	node = node.Focusable(rootID).OnActions(
		rootID,
		newListActions(descriptors, rootID, visible, selected, onSelect),
	)
	if applyFocusedStyle {
		node = node.WithFocusedStyle(focusedStyle)
	}
	return node
}

func newListActions[Message any](
	descriptors [5]tui.ActionDescriptor,
	rootID tui.NodeID,
	visible []int,
	selected int,
	onSelect func(int) Message,
) []tui.Action[Message] {
	actions := make([]tui.Action[Message], len(descriptors))
	for index, descriptor := range descriptors {
		action := collectionAction(index)
		actions[index] = tui.NewAction(descriptor, func(tui.ActionEvent) tui.EventResult[Message] {
			return listActionResult(action, rootID, visible, selected, onSelect)
		})
	}
	return actions
}

func listActionResult[Message any](
	action collectionAction,
	rootID tui.NodeID,
	visible []int,
	selected int,
	onSelect func(int) Message,
) tui.EventResult[Message] {
	result := tui.ConsumeResult[Message]().Focus(rootID)
	navigation, navigates := collectionNavigation(action)
	if !navigates {
		return result.Emit(onSelect(visible[selected]))
	}
	next, _ := navigateSelection(len(visible), selected, navigation)
	if next != selected {
		result = result.Emit(onSelect(visible[next]))
	}
	return result
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
	visible := make([]int, 0, len(items))
	for index, item := range items {
		if listItemMatches(item.Label, query) {
			visible = append(visible, index)
		}
	}
	return visible
}

func listHasVisibleItems(
	items []ListItem,
	query string,
	windowOffset int,
	windowLimit int,
	hasWindow bool,
) bool {
	if hasWindow && windowLimit <= 0 {
		return false
	}
	remaining := 0
	if hasWindow {
		remaining = max(windowOffset, 0)
	}
	for _, item := range items {
		if !listItemMatches(item.Label, query) {
			continue
		}
		if remaining == 0 {
			return true
		}
		remaining--
	}
	return false
}

func listItemMatches(label, query string) bool {
	if query == "" {
		return true
	}
	if len(query) > len(label) {
		return false
	}
	for start := 0; start <= len(label)-len(query); start++ {
		matches := true
		for offset := range len(query) {
			if asciiFoldByte(label[start+offset]) != asciiFoldByte(query[offset]) {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}

func asciiFoldByte(value byte) byte {
	if value >= 'A' && value <= 'Z' {
		return value + ('a' - 'A')
	}
	return value
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
