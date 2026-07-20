package widget

import (
	"strconv"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

// PaginatorMode controls the page indicator representation
type PaginatorMode uint8

const (
	// PaginatorDots renders one circle per visible page
	PaginatorDots PaginatorMode = iota
	// PaginatorNumeric renders the current and total page numbers
	PaginatorNumeric
)

// PaginatorStyle contains the visual styles used by a Paginator
type PaginatorStyle struct {
	// Normal is used by unselected page indicators
	Normal vt.Style
	// Selected is used by the application-selected page
	Selected vt.Style
	// Focused is merged over the indicator that owns focus
	Focused vt.Style
	// Disabled is used when page changes are unavailable
	Disabled vt.Style
}

// DefaultPaginatorStyle returns the standard paginator styles
func DefaultPaginatorStyle() PaginatorStyle {
	return PaginatorStyle{
		Selected: vt.Style{Bold: true}, Focused: vt.Style{Underline: true}, Disabled: vt.Style{Dim: true},
	}
}

// Paginator is a controlled zero-based page selector
type Paginator[Message any] struct {
	id       tui.NodeID
	page     int
	total    int
	limit    int
	mode     PaginatorMode
	enabled  bool
	style    PaginatorStyle
	onChange func(int) Message
}

// NewPaginator returns a controlled page selector
//
// A nil onChange function creates a disabled paginator.
func NewPaginator[Message any](id tui.NodeID, page, total int, onChange func(int) Message) Paginator[Message] {
	return Paginator[Message]{
		id: id, page: page, total: max(total, 0), limit: 7,
		enabled: onChange != nil, style: DefaultPaginatorStyle(), onChange: onChange,
	}
}

// Mode replaces the page indicator representation
func (p Paginator[Message]) Mode(mode PaginatorMode) Paginator[Message] {
	p.mode = mode
	return p
}

// IndicatorLimit limits the number of dot indicators
//
// A non-positive limit shows every page.
func (p Paginator[Message]) IndicatorLimit(limit int) Paginator[Message] {
	p.limit = max(limit, 0)
	return p
}

// Enabled sets whether the paginator can receive focus and emit messages
func (p Paginator[Message]) Enabled(enabled bool) Paginator[Message] {
	p.enabled = enabled && p.onChange != nil
	return p
}

// Style replaces the paginator styles
func (p Paginator[Message]) Style(style PaginatorStyle) Paginator[Message] {
	p.style = style
	return p
}

// Node builds the public semantic node for this paginator
func (p Paginator[Message]) Node() tui.Node[Message] {
	page, hasPage := normalizedPage(p.page, p.total)
	style := p.style.Normal
	if !p.enabled || !hasPage {
		style = p.style.Disabled
	}
	if p.mode == PaginatorNumeric || !hasPage {
		current := 0
		if hasPage {
			current = page + 1
		}
		node := tui.StyledText[Message](strconv.Itoa(current)+"/"+strconv.Itoa(p.total), style)
		if !p.enabled || !hasPage {
			return node.WithID(p.id)
		}
		return node.Focusable(p.id).WithFocusedStyle(p.style.Focused).OnEvent(p.id, p.navigationHandler(page))
	}

	start, end := paginatorWindow(p.total, page, p.limit)
	children := make([]tui.Node[Message], 0, (end-start)*2)
	for candidate := start; candidate < end; candidate++ {
		if candidate > start {
			children = append(children, tui.Text[Message](" "))
		}
		candidateID := paginatorPageID(p.id, candidate)
		if candidate == page {
			selectedStyle := p.style.Selected
			if !p.enabled {
				selectedStyle = p.style.Disabled
			}
			selected := tui.StyledText[Message]("●", selectedStyle).WithID(candidateID)
			if !p.enabled {
				children = append(children, selected)
				continue
			}
			children = append(children, tui.Column(selected).
				Focusable(p.id).
				WithFocusedStyle(p.style.Focused).
				OnEvent(p.id, p.navigationHandler(page)))
			continue
		}
		candidatePage := candidate
		candidateStyle := p.style.Normal
		if !p.enabled {
			candidateStyle = p.style.Disabled
		}
		node := tui.StyledText[Message]("○", candidateStyle).WithID(candidateID)
		if p.enabled {
			node = node.OnEvent(candidateID, func(event vt.Event) tui.EventResult[Message] {
				if !isActivationEvent(event) {
					return tui.IgnoreResult[Message]()
				}
				return tui.ConsumeResult[Message]().Focus(p.id).Emit(p.onChange(candidatePage))
			})
		}
		children = append(children, node)
	}
	root := tui.Row(children...)
	if !p.enabled {
		return root.WithID(p.id)
	}
	return root
}

func (p Paginator[Message]) navigationHandler(page int) func(vt.Event) tui.EventResult[Message] {
	return func(event vt.Event) tui.EventResult[Message] {
		next, handled := paginatorPageForEvent(page, p.total, event)
		if !handled {
			return tui.IgnoreResult[Message]()
		}
		result := tui.ConsumeResult[Message]().Focus(p.id)
		if next != page {
			result = result.Emit(p.onChange(next))
		}
		return result
	}
}

func normalizedPage(page, total int) (int, bool) {
	if total <= 0 {
		return 0, false
	}
	return min(max(page, 0), total-1), true
}

func paginatorWindow(total, page, limit int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	if limit <= 0 || limit >= total {
		return 0, total
	}
	page, _ = normalizedPage(page, total)
	start := min(max(page-limit/2, 0), total-limit)
	return start, start + limit
}

func paginatorPageForEvent(page, total int, event vt.Event) (int, bool) {
	page, ok := normalizedPage(page, total)
	if !ok || event.Kind != vt.EventKey || event.Key.Action == vt.KeyRelease {
		return 0, false
	}
	modifiers := event.Key.Modifiers
	if modifiers.Alt || modifiers.Control || modifiers.Meta {
		return 0, false
	}
	switch event.Key.Code {
	case vt.KeyLeft, vt.KeyUp, vt.KeyPageUp:
		return max(page-1, 0), true
	case vt.KeyRight, vt.KeyDown, vt.KeyPageDown:
		return min(page+1, total-1), true
	case vt.KeyHome:
		return 0, true
	case vt.KeyEnd:
		return total - 1, true
	default:
		return 0, false
	}
}

func paginatorPageID(root tui.NodeID, page int) tui.NodeID {
	return tui.NewNodeID(root.String() + "/page/" + strconv.Itoa(page))
}
