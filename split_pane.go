package tui

import "github.com/mayahiro/nagi-go/vt"

// SplitPaneAxis controls the main-axis direction of a split-pane layout
type SplitPaneAxis uint8

const (
	// SplitPaneHorizontal places the primary pane before the secondary pane from left to right
	SplitPaneHorizontal SplitPaneAxis = iota
	// SplitPaneVertical places the primary pane before the secondary pane from top to bottom
	SplitPaneVertical
)

// SplitPaneCollapse identifies the pane omitted when both minima do not fit
type SplitPaneCollapse uint8

const (
	// SplitPaneCollapseSecondary omits the secondary pane and preserves the primary pane
	SplitPaneCollapseSecondary SplitPaneCollapse = iota
	// SplitPaneCollapsePrimary omits the primary pane and preserves the secondary pane
	SplitPaneCollapsePrimary
)

// SplitPaneOptions contains the layout policy used by the Core split-pane node
type SplitPaneOptions struct {
	// Axis controls the direction in which the panes are arranged. Unknown values use horizontal.
	Axis SplitPaneAxis
	// Ratio is the primary-pane share in basis points and is clamped to 0 through 10,000
	Ratio uint16
	// PrimaryMinimum is the smallest expanded primary extent and normalizes to at least one Cell
	PrimaryMinimum uint32
	// SecondaryMinimum is the smallest expanded secondary extent and normalizes to at least one Cell
	SecondaryMinimum uint32
	// Collapse identifies the pane omitted when the assigned extent is insufficient. Unknown values use secondary.
	Collapse SplitPaneCollapse
	// DividerStyle is used by the one-Cell divider
	DividerStyle vt.Style
}

// DefaultSplitPaneOptions returns a horizontal equal split that collapses the secondary pane
func DefaultSplitPaneOptions() SplitPaneOptions {
	return SplitPaneOptions{
		Ratio: 5_000, PrimaryMinimum: 1, SecondaryMinimum: 1,
		Collapse: SplitPaneCollapseSecondary,
	}
}

type resolvedSplitPaneLayout struct {
	primary, secondary Rect
	divider            Rect
	hasDivider         bool
	collapsed          SplitPaneCollapse
	hasCollapsed       bool
}

func resolveSplitPaneLayout(rect Rect, options SplitPaneOptions) resolvedSplitPaneLayout {
	axis := normalizedSplitPaneAxis(options.Axis)
	main := rect.Width
	if axis == SplitPaneVertical {
		main = rect.Height
	}
	primaryMinimum := max(options.PrimaryMinimum, 1)
	secondaryMinimum := max(options.SecondaryMinimum, 1)
	required := uint64(primaryMinimum) + 1 + uint64(secondaryMinimum)
	if uint64(main) < required {
		return collapsedSplitPaneLayout(rect, axis, normalizedSplitPaneCollapse(options.Collapse))
	}

	usable := main - 1
	ratio := min(options.Ratio, uint16(10_000))
	ideal := uint32(uint64(usable) * uint64(ratio) / 10_000)
	primaryExtent := min(max(ideal, primaryMinimum), usable-secondaryMinimum)
	return expandedSplitPaneLayout(rect, axis, primaryExtent)
}

func expandedSplitPaneLayout(rect Rect, axis SplitPaneAxis, primaryExtent uint32) resolvedSplitPaneLayout {
	if axis == SplitPaneVertical {
		dividerY := saturatingCoordinate(rect.Y, primaryExtent)
		return resolvedSplitPaneLayout{
			primary:    Rect{X: rect.X, Y: rect.Y, Width: rect.Width, Height: primaryExtent},
			divider:    Rect{X: rect.X, Y: dividerY, Width: rect.Width, Height: 1},
			hasDivider: true,
			secondary: Rect{
				X: rect.X, Y: saturatingCoordinate(dividerY, 1), Width: rect.Width,
				Height: rect.Height - primaryExtent - 1,
			},
		}
	}
	dividerX := saturatingCoordinate(rect.X, primaryExtent)
	return resolvedSplitPaneLayout{
		primary:    Rect{X: rect.X, Y: rect.Y, Width: primaryExtent, Height: rect.Height},
		divider:    Rect{X: dividerX, Y: rect.Y, Width: 1, Height: rect.Height},
		hasDivider: true,
		secondary: Rect{
			X: saturatingCoordinate(dividerX, 1), Y: rect.Y,
			Width: rect.Width - primaryExtent - 1, Height: rect.Height,
		},
	}
}

func collapsedSplitPaneLayout(rect Rect, axis SplitPaneAxis, collapse SplitPaneCollapse) resolvedSplitPaneLayout {
	emptyPrimary := Rect{X: rect.X, Y: rect.Y, Height: rect.Height}
	emptySecondary := Rect{X: saturatingCoordinate(rect.X, rect.Width), Y: rect.Y, Height: rect.Height}
	if axis == SplitPaneVertical {
		emptyPrimary = Rect{X: rect.X, Y: rect.Y, Width: rect.Width}
		emptySecondary = Rect{X: rect.X, Y: saturatingCoordinate(rect.Y, rect.Height), Width: rect.Width}
	}
	if collapse == SplitPaneCollapsePrimary {
		return resolvedSplitPaneLayout{
			primary: emptyPrimary, secondary: rect, collapsed: collapse, hasCollapsed: true,
		}
	}
	return resolvedSplitPaneLayout{
		primary: rect, secondary: emptySecondary, collapsed: collapse, hasCollapsed: true,
	}
}

func normalizedSplitPaneAxis(axis SplitPaneAxis) SplitPaneAxis {
	if axis == SplitPaneVertical {
		return axis
	}
	return SplitPaneHorizontal
}

func normalizedSplitPaneCollapse(collapse SplitPaneCollapse) SplitPaneCollapse {
	if collapse == SplitPaneCollapsePrimary {
		return collapse
	}
	return SplitPaneCollapseSecondary
}
