package widget

import (
	"fmt"
	"strconv"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

// SuggestionID is one opaque stable candidate identity
type SuggestionID struct {
	value string
}

// NewSuggestionID returns an application-defined candidate identity with valid UTF-8
func NewSuggestionID(value string) SuggestionID {
	return SuggestionID{value: celltext.NormalizeUTF8(value)}
}

// String returns the application-defined identity
func (id SuggestionID) String() string {
	return id.value
}

// DuplicateSuggestionIDError reports a repeated stable candidate identity
type DuplicateSuggestionIDError struct {
	// ID is the duplicated candidate identity
	ID SuggestionID
}

// Error returns the duplicate candidate diagnostic
func (e *DuplicateSuggestionIDError) Error() string {
	return fmt.Sprintf("duplicate suggestion ID %s", e.ID.String())
}

// SuggestionItems is an immutable unique candidate order shared across view rebuilds
type SuggestionItems struct {
	inner *suggestionItemsData
}

type suggestionItemsData struct {
	items     []SuggestionID
	positions map[SuggestionID]int
}

// NewSuggestionItems returns a validated immutable candidate order
func NewSuggestionItems(candidates []SuggestionID) (SuggestionItems, error) {
	owned := append([]SuggestionID(nil), candidates...)
	positions := make(map[SuggestionID]int, len(owned))
	for index, candidate := range owned {
		if _, exists := positions[candidate]; exists {
			return SuggestionItems{}, &DuplicateSuggestionIDError{ID: candidate}
		}
		positions[candidate] = index
	}
	return SuggestionItems{inner: &suggestionItemsData{items: owned, positions: positions}}, nil
}

// Len returns the number of candidates
func (i SuggestionItems) Len() int {
	if i.inner == nil {
		return 0
	}
	return len(i.inner.items)
}

// Empty reports whether this candidate order is empty
func (i SuggestionItems) Empty() bool {
	return i.Len() == 0
}

// Item returns one candidate by current index
func (i SuggestionItems) Item(index int) (SuggestionID, bool) {
	if i.inner == nil || index < 0 || index >= len(i.inner.items) {
		return SuggestionID{}, false
	}
	return i.inner.items[index], true
}

// Items returns a copy of the immutable ordered candidates
func (i SuggestionItems) Items() []SuggestionID {
	if i.inner == nil {
		return nil
	}
	return append([]SuggestionID(nil), i.inner.items...)
}

func (i SuggestionItems) items() []SuggestionID {
	if i.inner == nil {
		return nil
	}
	return i.inner.items
}

func (i SuggestionItems) position(id SuggestionID) (int, bool) {
	if i.inner == nil {
		return 0, false
	}
	index, ok := i.inner.positions[id]
	return index, ok
}

// SuggestionPopupStatus is application-owned asynchronous display state
type SuggestionPopupStatus uint8

const (
	// SuggestionPopupReady means candidate data is current for the application request
	SuggestionPopupReady SuggestionPopupStatus = iota
	// SuggestionPopupLoading means a newer application request is still running
	SuggestionPopupLoading
)

// SuggestionRowContext is passed to an application suggestion row builder
type SuggestionRowContext struct {
	index    int
	id       SuggestionID
	selected bool
}

// Index returns the candidate index in application order
func (c SuggestionRowContext) Index() int {
	return c.index
}

// ID returns the stable application candidate identity
func (c SuggestionRowContext) ID() SuggestionID {
	return c.id
}

// Selected reports whether this candidate is the normalized selection
func (c SuggestionRowContext) Selected() bool {
	return c.selected
}

// SuggestionPopupStyle contains popup-owned visual styles
type SuggestionPopupStyle struct {
	// Border is used by the popup border
	Border vt.Style
	// Notice is used by default loading and empty notices
	Notice vt.Style
}

// DefaultSuggestionPopupStyle returns a dim notice and default border
func DefaultSuggestionPopupStyle() SuggestionPopupStyle {
	return SuggestionPopupStyle{Notice: vt.Style{Dim: true}}
}

// SuggestionPopup is a controlled anchored suggestion list that leaves focus in base content
//
// The application owns query parsing, asynchronous work, ranking, candidates,
// selection, and acceptance meaning, while the widget provides placement,
// bounded row construction, keyboard routing, pointer activation, and status
// presentation
type SuggestionPopup[Message any] struct {
	id          tui.NodeID
	base        tui.Node[Message]
	anchor      tui.NodeID
	focusOwner  tui.NodeID
	candidates  SuggestionItems
	selected    SuggestionID
	hasSelected bool
	status      SuggestionPopupStatus
	open        bool
	enabled     bool
	visibleRows int
	placement   tui.AnchoredOverlayOptions
	style       SuggestionPopupStyle
	loading     tui.Node[Message]
	hasLoading  bool
	empty       tui.Node[Message]
	hasEmpty    bool
	row         func(SuggestionRowContext) tui.Node[Message]
	onSelect    func(SuggestionID) Message
	onAccept    func(SuggestionID) Message
	onDismiss   func() Message
}

// NewSuggestionPopup returns an open controlled popup
//
// A nil row builder or callback disables candidate interaction
func NewSuggestionPopup[Message any](
	id tui.NodeID,
	base tui.Node[Message],
	anchor tui.NodeID,
	focusOwner tui.NodeID,
	candidates SuggestionItems,
	selected SuggestionID,
	hasSelected bool,
	row func(SuggestionRowContext) tui.Node[Message],
	onSelect func(SuggestionID) Message,
	onAccept func(SuggestionID) Message,
	onDismiss func() Message,
) SuggestionPopup[Message] {
	enabled := row != nil && onSelect != nil && onAccept != nil && onDismiss != nil
	if row == nil {
		row = func(context SuggestionRowContext) tui.Node[Message] {
			return tui.Text[Message](context.ID().String())
		}
	}
	return SuggestionPopup[Message]{
		id: id, base: base, anchor: anchor, focusOwner: focusOwner,
		candidates: candidates,
		selected:   selected, hasSelected: hasSelected,
		status: SuggestionPopupReady, open: true, enabled: enabled, visibleRows: 8,
		placement: tui.DefaultAnchoredOverlayOptions(), style: DefaultSuggestionPopupStyle(),
		row: row, onSelect: onSelect, onAccept: onAccept, onDismiss: onDismiss,
	}
}

// Open sets whether the popup layer and its scoped actions are present
func (p SuggestionPopup[Message]) Open(open bool) SuggestionPopup[Message] {
	p.open = open
	return p
}

// Enabled sets whether candidate interaction is available while retaining status content
func (p SuggestionPopup[Message]) Enabled(enabled bool) SuggestionPopup[Message] {
	p.enabled = enabled && p.row != nil && p.onSelect != nil && p.onAccept != nil && p.onDismiss != nil
	return p
}

// Status sets the application-owned asynchronous display state
//
// Unknown values are treated as Ready
func (p SuggestionPopup[Message]) Status(status SuggestionPopupStatus) SuggestionPopup[Message] {
	if status != SuggestionPopupLoading {
		status = SuggestionPopupReady
	}
	p.status = status
	return p
}

// VisibleRows limits constructed candidate rows to a positive visible window
func (p SuggestionPopup[Message]) VisibleRows(rows int) SuggestionPopup[Message] {
	p.visibleRows = max(rows, 1)
	return p
}

// Placement replaces anchored overlay placement and size limits
func (p SuggestionPopup[Message]) Placement(options tui.AnchoredOverlayOptions) SuggestionPopup[Message] {
	p.placement = options
	return p
}

// Style replaces popup-owned styles
func (p SuggestionPopup[Message]) Style(style SuggestionPopupStyle) SuggestionPopup[Message] {
	p.style = style
	return p
}

// Loading replaces the loading notice with an application-provided Node
func (p SuggestionPopup[Message]) Loading(node tui.Node[Message]) SuggestionPopup[Message] {
	p.loading = node
	p.hasLoading = true
	return p
}

// Empty replaces the empty-result notice with an application-provided Node
func (p SuggestionPopup[Message]) Empty(node tui.Node[Message]) SuggestionPopup[Message] {
	p.empty = node
	p.hasEmpty = true
	return p
}

// ActionDescriptors returns accept, previous, next, and dismiss in dispatch order
func (p SuggestionPopup[Message]) ActionDescriptors() []tui.ActionDescriptor {
	hasCandidates := p.status == SuggestionPopupReady && !p.candidates.Empty()
	descriptors := suggestionActionDescriptors(p.open, p.enabled, hasCandidates)
	return append([]tui.ActionDescriptor(nil), descriptors[:]...)
}

// Node builds the public semantic Node for this suggestion popup
func (p SuggestionPopup[Message]) Node() tui.Node[Message] {
	if !p.open {
		return p.base
	}
	selected, hasSelection := 0, false
	if p.status == SuggestionPopupReady {
		selected, hasSelection = normalizedSuggestionSelection(
			p.candidates, p.selected, p.hasSelected,
		)
	}
	descriptors := suggestionActionDescriptors(true, p.enabled, hasSelection)
	layer := p.layer(selected, hasSelection)
	scope := suggestionKeyScope(p.id, p.enabled && hasSelection)
	return tui.AnchoredOverlayWithOptions(p.base, p.anchor, layer, p.placement).
		WithKeyScope(scope).
		OnActions(p.id, suggestionActions(
			descriptors, p.candidates, selected, hasSelection, p.focusOwner,
			p.onSelect, p.onAccept, p.onDismiss,
		)).
		OnEvent(p.id, func(event vt.Event) tui.EventResult[Message] {
			if p.enabled && suggestionBlockedRepeat(event) {
				return tui.ConsumeResult[Message]().Focus(p.focusOwner)
			}
			return tui.IgnoreResult[Message]()
		})
}

func (p SuggestionPopup[Message]) layer(selected int, hasSelection bool) tui.Node[Message] {
	var body tui.Node[Message]
	switch {
	case p.status == SuggestionPopupLoading:
		if p.hasLoading {
			body = p.loading
		} else {
			body = tui.StyledText[Message]("Loading...", p.style.Notice)
		}
	case p.candidates.Empty():
		if p.hasEmpty {
			body = p.empty
		} else {
			body = tui.StyledText[Message]("No suggestions", p.style.Notice)
		}
	default:
		start, end := suggestionWindow(p.candidates.Len(), selected, p.visibleRows)
		candidates := p.candidates.items()
		rows := make([]tui.Node[Message], 0, end-start)
		for index := start; index < end; index++ {
			id := candidates[index]
			isSelected := hasSelection && index == selected
			row := p.row(SuggestionRowContext{index: index, id: id, selected: isSelected})
			rowID := suggestionRowNodeID(p.id, id)
			if p.enabled {
				row = row.OnEvent(rowID, func(event vt.Event) tui.EventResult[Message] {
					if !isPointerActivationEvent(event) {
						return tui.IgnoreResult[Message]()
					}
					result := tui.ConsumeResult[Message]().Focus(p.focusOwner)
					if !isSelected {
						result = result.Emit(p.onSelect(id))
					}
					return result.Emit(p.onAccept(id))
				})
			} else {
				row = row.WithID(rowID)
			}
			rows = append(rows, row)
		}
		body = tui.Column(rows...)
	}
	return tui.Border(body, p.style.Border)
}

func suggestionActions[Message any](
	descriptors [4]tui.ActionDescriptor,
	candidates SuggestionItems,
	selected int,
	hasSelection bool,
	focusOwner tui.NodeID,
	onSelect func(SuggestionID) Message,
	onAccept func(SuggestionID) Message,
	onDismiss func() Message,
) []tui.Action[Message] {
	actions := make([]tui.Action[Message], len(descriptors))
	for action, descriptor := range descriptors {
		action := action
		actions[action] = tui.NewAction(descriptor, func(tui.ActionEvent) tui.EventResult[Message] {
			result := tui.ConsumeResult[Message]().Focus(focusOwner)
			if action == 3 {
				return result.Emit(onDismiss())
			}
			if !hasSelection {
				return result
			}
			if action == 0 {
				candidate, _ := candidates.Item(selected)
				return result.Emit(onAccept(candidate))
			}
			navigation := navigationDown
			if action == 1 {
				navigation = navigationUp
			}
			next, _ := navigateSelection(candidates.Len(), selected, navigation)
			if next != selected {
				candidate, _ := candidates.Item(next)
				result = result.Emit(onSelect(candidate))
			}
			return result
		})
	}
	return actions
}

func suggestionActionDescriptors(open, enabled, hasCandidates bool) [4]tui.ActionDescriptor {
	candidates := tui.ActionDisabledPassThrough
	if open && enabled && hasCandidates {
		candidates = tui.ActionEnabled
	}
	dismiss := tui.ActionDisabledPassThrough
	if open && enabled {
		dismiss = tui.ActionEnabled
	}
	return [4]tui.ActionDescriptor{
		suggestionAcceptDescriptor.WithAvailability(candidates),
		suggestionPreviousDescriptor.WithAvailability(candidates),
		suggestionNextDescriptor.WithAvailability(candidates),
		suggestionDismissDescriptor.WithAvailability(dismiss),
	}
}

func suggestionKeyScope(id tui.NodeID, interceptCandidates bool) tui.KeyScope {
	keyMap := tui.NewKeyMap()
	if interceptCandidates {
		for _, action := range []tui.ActionID{
			tui.TextCursorUpActionID,
			tui.TextCursorDownActionID,
			tui.TextInsertLineBreakActionID,
			ComposerSubmitActionID,
			HistoryPreviousActionID,
			HistoryNextActionID,
		} {
			var err error
			keyMap, err = keyMap.Rebind(action, nil)
			if err != nil {
				panic("widget: duplicate internal suggestion Action ID")
			}
		}
	}
	return tui.NewKeyScope(id, keyMap)
}

func normalizedSuggestionSelection(
	candidates SuggestionItems,
	selected SuggestionID,
	hasSelected bool,
) (int, bool) {
	if candidates.Empty() {
		return 0, false
	}
	if hasSelected {
		if index, exists := candidates.position(selected); exists {
			return index, true
		}
	}
	return 0, true
}

func suggestionWindow(count, selected, rows int) (int, int) {
	limit := min(max(rows, 1), count)
	start := min(max(selected+1-limit, 0), count-limit)
	return start, start + limit
}

func suggestionRowNodeID(popup tui.NodeID, candidate SuggestionID) tui.NodeID {
	popupValue := popup.String()
	value := candidate.String()
	return tui.NodeID(
		"suggestion-row:" + strconv.Itoa(len(popupValue)) + ":" + popupValue + ":" +
			strconv.Itoa(len(value)) + ":" + value,
	)
}

func suggestionBlockedRepeat(event vt.Event) bool {
	if event.Kind != vt.EventKey || event.Key.Action != vt.KeyRepeat || event.Key.Modifiers != (vt.Modifiers{}) {
		return false
	}
	return event.Key.Code == vt.KeyEnter || event.Key.Code == vt.KeyEscape
}

var suggestionAcceptDescriptor = tui.NewActionDescriptor(
	SuggestionAcceptActionID,
	"Accept suggestion",
	[]tui.KeyBinding{tui.NewKeyBinding(tui.NewKeyStroke(vt.KeyEnter, vt.Modifiers{}))},
)

var suggestionPreviousDescriptor = tui.NewActionDescriptor(
	SelectionPreviousActionID,
	selectionPreviousActionLabel,
	[]tui.KeyBinding{repeatableActionBinding(vt.KeyUp)},
)

var suggestionNextDescriptor = tui.NewActionDescriptor(
	SelectionNextActionID,
	selectionNextActionLabel,
	[]tui.KeyBinding{repeatableActionBinding(vt.KeyDown)},
)

var suggestionDismissDescriptor = tui.NewActionDescriptor(
	SuggestionDismissActionID,
	"Dismiss suggestions",
	[]tui.KeyBinding{tui.NewKeyBinding(tui.NewKeyStroke(vt.KeyEscape, vt.Modifiers{}))},
)
