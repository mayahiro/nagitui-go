package widget

import tui "github.com/mayahiro/nagitui-go"

// StatusBarPriority controls slot retention under insufficient width
type StatusBarPriority uint8

const (
	// StatusBarLow is omitted before normal-priority slots
	StatusBarLow StatusBarPriority = iota
	// StatusBarNormal is the standard status priority
	StatusBarNormal
	// StatusBarHigh is retained before normal-priority slots
	StatusBarHigh
	// StatusBarCritical is retained before every other category
	StatusBarCritical
)

// StatusBarSlot is one arbitrary semantic Node displayed by a StatusBar
type StatusBarSlot[Message any] struct {
	node      tui.Node[Message]
	placement tui.ResponsiveRowPlacement
	priority  StatusBarPriority
}

// NewStatusBarSlot returns a start-aligned slot with normal retention priority
func NewStatusBarSlot[Message any](node tui.Node[Message]) StatusBarSlot[Message] {
	return StatusBarSlot[Message]{node: node, priority: StatusBarNormal}
}

// Placement sets the start, center, or end region used by the slot
func (s StatusBarSlot[Message]) Placement(placement tui.ResponsiveRowPlacement) StatusBarSlot[Message] {
	s.placement = placement
	return s
}

// Priority sets the slot retention priority
//
// Unknown values use normal priority
func (s StatusBarSlot[Message]) Priority(priority StatusBarPriority) StatusBarSlot[Message] {
	s.priority = normalizedStatusBarPriority(priority)
	return s
}

// ConfiguredPlacement returns the configured placement
func (s StatusBarSlot[Message]) ConfiguredPlacement() tui.ResponsiveRowPlacement {
	return tui.NewResponsiveRowItem(s.node).Placement(s.placement).ConfiguredPlacement()
}

// ConfiguredPriority returns the configured retention priority
func (s StatusBarSlot[Message]) ConfiguredPriority() StatusBarPriority {
	return normalizedStatusBarPriority(s.priority)
}

// StatusBar is a one-row responsive container for application-defined status Nodes
//
// StatusBar owns no status meaning, state, timer, task, or I/O. Hidden slots
// follow the Core ResponsiveRow semantic omission contract.
type StatusBar[Message any] struct {
	slots []StatusBarSlot[Message]
	gap   uint32
}

// NewStatusBar returns a status bar with one empty Cell between retained slots
//
// The returned value retains the supplied slot slice. Callers must not mutate
// it after construction.
func NewStatusBar[Message any](slots []StatusBarSlot[Message]) StatusBar[Message] {
	return StatusBar[Message]{slots: slots, gap: 1}
}

// Gap sets the empty Cells required between retained slots
func (b StatusBar[Message]) Gap(gap uint32) StatusBar[Message] {
	b.gap = gap
	return b
}

// Node builds the one-row public semantic node
func (b StatusBar[Message]) Node() tui.Node[Message] {
	items := make([]tui.ResponsiveRowItem[Message], len(b.slots))
	for index, slot := range b.slots {
		items[index] = tui.NewResponsiveRowItem(slot.node).
			Placement(slot.placement).
			Priority(statusBarPriorityValue(slot.priority))
	}
	return tui.ResponsiveRow(items, tui.ResponsiveRowOptions{Gap: b.gap, Height: 1})
}

func normalizedStatusBarPriority(priority StatusBarPriority) StatusBarPriority {
	if priority <= StatusBarCritical {
		return priority
	}
	return StatusBarNormal
}

func statusBarPriorityValue(priority StatusBarPriority) uint16 {
	switch normalizedStatusBarPriority(priority) {
	case StatusBarLow:
		return 0
	case StatusBarHigh:
		return 200
	case StatusBarCritical:
		return 300
	default:
		return 100
	}
}
