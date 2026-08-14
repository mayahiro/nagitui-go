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

var defaultPaginatorActionDescriptors = [4]tui.ActionDescriptor{
	tui.NewActionDescriptor(
		SelectionPreviousActionID,
		selectionPreviousActionLabel,
		[]tui.KeyBinding{
			repeatableActionBinding(vt.KeyLeft),
			repeatableActionBinding(vt.KeyUp),
			repeatableActionBinding(vt.KeyPageUp),
		},
	),
	tui.NewActionDescriptor(
		SelectionNextActionID,
		selectionNextActionLabel,
		[]tui.KeyBinding{
			repeatableActionBinding(vt.KeyRight),
			repeatableActionBinding(vt.KeyDown),
			repeatableActionBinding(vt.KeyPageDown),
		},
	),
	tui.NewActionDescriptor(
		SelectionFirstActionID,
		selectionFirstActionLabel,
		[]tui.KeyBinding{repeatableActionBinding(vt.KeyHome)},
	),
	tui.NewActionDescriptor(
		SelectionLastActionID,
		selectionLastActionLabel,
		[]tui.KeyBinding{repeatableActionBinding(vt.KeyEnd)},
	),
}

type paginatorAction uint8

const (
	paginatorPrevious paginatorAction = iota
	paginatorNext
	paginatorFirst
	paginatorLast
)

var paginatorActions = [4]paginatorAction{
	paginatorPrevious,
	paginatorNext,
	paginatorFirst,
	paginatorLast,
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

// ActionDescriptors returns the ordered semantic navigation actions declared by the root
func (p Paginator[Message]) ActionDescriptors() []tui.ActionDescriptor {
	descriptors := paginatorActionDescriptors(p.enabled && p.total > 0)
	return append([]tui.ActionDescriptor(nil), descriptors[:]...)
}

// Node builds the public semantic node for this paginator
func (p Paginator[Message]) Node() tui.Node[Message] {
	page, hasPage := normalizedPage(p.page, p.total)
	descriptors := paginatorActionDescriptors(p.enabled && hasPage)
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
			return node.WithID(p.id).OnActions(p.id, disabledPaginatorActions[Message](descriptors))
		}
		return node.Focusable(p.id).
			WithFocusedStyle(p.style.Focused).
			OnActions(p.id, paginatorSemanticActions(descriptors, page, p.total, p.id, p.onChange))
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
				OnActions(p.id, paginatorSemanticActions(descriptors, page, p.total, p.id, p.onChange)))
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
				if !isPointerActivationEvent(event) {
					return tui.IgnoreResult[Message]()
				}
				return tui.ConsumeResult[Message]().Focus(p.id).Emit(p.onChange(candidatePage))
			})
		}
		children = append(children, node)
	}
	root := tui.Row(children...)
	if !p.enabled {
		return root.WithID(p.id).OnActions(p.id, disabledPaginatorActions[Message](descriptors))
	}
	return root
}

func paginatorSemanticActions[Message any](
	descriptors [4]tui.ActionDescriptor,
	page, total int,
	focusID tui.NodeID,
	onChange func(int) Message,
) []tui.Action[Message] {
	actions := make([]tui.Action[Message], len(descriptors))
	for index, descriptor := range descriptors {
		action := paginatorActions[index]
		actions[index] = tui.NewAction(descriptor, func(tui.ActionEvent) tui.EventResult[Message] {
			return paginatorActionResult(action, page, total, focusID, onChange)
		})
	}
	return actions
}

func disabledPaginatorActions[Message any](descriptors [4]tui.ActionDescriptor) []tui.Action[Message] {
	actions := make([]tui.Action[Message], len(descriptors))
	for index, descriptor := range descriptors {
		actions[index] = tui.NewAction[Message](descriptor, nil)
	}
	return actions
}

func paginatorActionResult[Message any](
	action paginatorAction,
	page, total int,
	focusID tui.NodeID,
	onChange func(int) Message,
) tui.EventResult[Message] {
	next, ok := paginatorPageForAction(page, total, action)
	if !ok {
		next = page
	}
	result := tui.ConsumeResult[Message]().Focus(focusID)
	if next != page {
		result = result.Emit(onChange(next))
	}
	return result
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

func paginatorPageForAction(page, total int, action paginatorAction) (int, bool) {
	page, ok := normalizedPage(page, total)
	if !ok {
		return 0, false
	}
	switch action {
	case paginatorPrevious:
		return max(page-1, 0), true
	case paginatorNext:
		return min(page+1, total-1), true
	case paginatorFirst:
		return 0, true
	case paginatorLast:
		return total - 1, true
	default:
		return 0, false
	}
}

func paginatorActionDescriptors(enabled bool) [4]tui.ActionDescriptor {
	availability := tui.ActionEnabled
	if !enabled {
		availability = tui.ActionDisabledPassThrough
	}
	return [4]tui.ActionDescriptor{
		defaultPaginatorActionDescriptors[0].WithAvailability(availability),
		defaultPaginatorActionDescriptors[1].WithAvailability(availability),
		defaultPaginatorActionDescriptors[2].WithAvailability(availability),
		defaultPaginatorActionDescriptors[3].WithAvailability(availability),
	}
}

func paginatorPageID(root tui.NodeID, page int) tui.NodeID {
	return tui.NewNodeID(root.String() + "/page/" + strconv.Itoa(page))
}
