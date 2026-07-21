package tui

import (
	"strings"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go/surface"
)

func rendererOperations(previous, current *surface.Surface) []vt.TerminalOp {
	var runs []surface.ChangedRun
	if previous == nil {
		if current.Width() != 0 {
			runs = make([]surface.ChangedRun, current.Height())
			for row := range current.Height() {
				runs[row] = surface.ChangedRun{Row: row, End: current.Width()}
			}
		}
	} else {
		runs = current.ChangedRuns(previous)
	}
	if previous != nil && len(runs) == 0 {
		previousCursor, previousHasCursor := previous.Cursor()
		currentCursor, currentHasCursor := current.Cursor()
		if previousHasCursor == currentHasCursor && (!currentHasCursor || previousCursor == currentCursor) {
			return nil
		}
	}
	operations := make([]vt.TerminalOp, 0, 8)
	operations = append(operations, vt.BeginSynchronizedUpdate(), vt.HideCursor())

	for _, run := range runs {
		operations = append(operations, vt.MoveTo(run.Start, run.Row))
		var activeStyle vt.Style
		hasStyle := false
		var text strings.Builder
		text.Grow(int(run.End - run.Start))
		for column := run.Start; column < run.End; column++ {
			cell, ok := current.Cell(int32(column), int32(run.Row))
			if !ok || cell.Continuation() {
				continue
			}
			if !hasStyle || activeStyle != cell.Style() {
				operations = flushRendererText(operations, &text)
				activeStyle = cell.Style()
				hasStyle = true
				if activeStyle == (vt.Style{}) {
					operations = append(operations, vt.ResetStyle())
				} else {
					operations = append(operations, vt.SetStyle(rendererSgrStyle(activeStyle)))
				}
			}
			text.WriteString(cell.Content())
		}
		operations = flushRendererText(operations, &text)
	}

	operations = append(operations, vt.ResetStyle())
	if cursor, ok := current.Cursor(); ok {
		operations = append(operations, vt.MoveTo(cursor.X, cursor.Y), vt.ShowCursor())
	} else {
		operations = append(operations, vt.HideCursor())
	}
	return append(operations, vt.EndSynchronizedUpdate())
}

func flushRendererText(operations []vt.TerminalOp, text *strings.Builder) []vt.TerminalOp {
	if text.Len() != 0 {
		operations = append(operations, vt.WriteText(text.String()))
		text.Reset()
	}
	return operations
}

func rendererSgrStyle(style vt.Style) vt.SgrStyle {
	result := vt.SgrStyle{
		Foreground:    rendererSgrColor(style.Foreground),
		Background:    rendererSgrColor(style.Background),
		Bold:          style.Bold,
		Dim:           style.Dim,
		Italic:        style.Italic,
		Underline:     style.Underline,
		Blink:         style.Blink,
		Reverse:       style.Reverse,
		Hidden:        style.Hidden,
		Strikethrough: style.Strikethrough,
	}
	if color, ok := style.UnderlineColor.Get(); ok {
		result.UnderlineColor = vt.SomeSgrColor(rendererSgrColor(color))
	}
	return result
}

func rendererSgrColor(color vt.Color) vt.SgrColor {
	switch color.Kind() {
	case vt.ColorDefault:
		return vt.SgrColor{}
	case vt.ColorIndexed:
		index, _ := color.Index()
		return vt.IndexedSgrColor(index)
	case vt.ColorRGB:
		red, green, blue, _ := color.RGB()
		return vt.RGBSgrColor(red, green, blue)
	default:
		panic("nagi-tui: invalid surface color")
	}
}
