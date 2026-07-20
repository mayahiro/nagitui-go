package widget

import (
	"github.com/mayahiro/nagi-go/vt"
	"math/bits"
	"strings"

	"github.com/mayahiro/nagitui-go"
)

// ProgressStyle contains the visual styles used by a Progress
type ProgressStyle struct {
	// Complete is used by completed cells
	Complete vt.Style
	// Remaining is used by remaining cells
	Remaining vt.Style
}

// DefaultProgressStyle returns the standard progress styles
func DefaultProgressStyle() ProgressStyle {
	return ProgressStyle{
		Complete:  vt.Style{Bold: true},
		Remaining: vt.Style{Dim: true},
	}
}

// Progress is a bounded-width determinate progress indicator
type Progress[Message any] struct {
	current uint64
	total   uint64
	width   uint16
	style   ProgressStyle
}

// NewProgress returns a determinate progress indicator
func NewProgress[Message any](current, total uint64, width uint16) Progress[Message] {
	return Progress[Message]{
		current: current,
		total:   total,
		width:   width,
		style:   DefaultProgressStyle(),
	}
}

// Style replaces the progress styles
func (p Progress[Message]) Style(style ProgressStyle) Progress[Message] {
	p.style = style
	return p
}

// Node builds the public semantic node for this progress indicator
func (p Progress[Message]) Node() tui.Node[Message] {
	filled := completedCells(p.current, p.total, p.width)
	return tui.Row(
		tui.StyledText[Message](strings.Repeat("█", filled), p.style.Complete),
		tui.StyledText[Message](strings.Repeat("░", int(p.width)-filled), p.style.Remaining),
	)
}

func completedCells(current, total uint64, width uint16) int {
	if total == 0 {
		return 0
	}
	current = min(current, total)
	high, low := bits.Mul64(current, uint64(width))
	quotient, _ := bits.Div64(high, low, total)
	return int(quotient)
}

func renderedProgress(current, total uint64, width uint16) string {
	filled := completedCells(current, total, width)
	return strings.Repeat("█", filled) + strings.Repeat("░", int(width)-filled)
}
