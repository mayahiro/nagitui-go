package widget

import (
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

// TableColumn is one sized column rendered by a Table
type TableColumn struct {
	// Title is the displayed heading
	Title string
	// Width is the Core sizing rule used for each cell
	Width tui.Length
}

// NewTableColumn returns a table column using a Core Length sizing rule
func NewTableColumn(title string, width tui.Length) TableColumn {
	return TableColumn{Title: title, Width: width}
}

// TableRow is one stable row rendered by a Table
type TableRow struct {
	// ID is the application-defined stable identity of this row
	ID tui.NodeID
	// Cells contains values associated with columns by position
	Cells []string
}

// NewTableRow returns a table row with a stable identity
func NewTableRow(id tui.NodeID, cells []string) TableRow {
	return TableRow{ID: id, Cells: append([]string(nil), cells...)}
}

// TableStyle contains the visual styles used by a Table
type TableStyle struct {
	// Header is used by the heading row
	Header vt.Style
	// Normal is used by unselected data rows
	Normal vt.Style
	// Selected is used by the application-selected row
	Selected vt.Style
	// Focused is merged over the selected row while the table owns focus
	Focused vt.Style
	// Disabled is used by every data row while the table is disabled
	Disabled vt.Style
}

// DefaultTableStyle returns the standard table styles
func DefaultTableStyle() TableStyle {
	return TableStyle{
		Header: vt.Style{Bold: true}, Selected: vt.Style{Reverse: true},
		Focused: vt.Style{Underline: true}, Disabled: vt.Style{Dim: true},
	}
}

// Table is a sized-column table with one composite focus target
type Table[Message any] struct {
	id               tui.NodeID
	columns          []TableColumn
	columnAlignments []tui.HorizontalAlignment
	rows             []TableRow
	selected         int
	enabled          bool
	style            TableStyle
	viewportID       tui.NodeID
	viewportHeight   tui.Length
	hasViewport      bool
	onSelect         func(int) Message
}

// NewTable returns an enabled table using application-owned selection state
//
// A nil onSelect function creates a disabled table
func NewTable[Message any](id tui.NodeID, columns []TableColumn, rows []TableRow, selected int, onSelect func(int) Message) Table[Message] {
	return Table[Message]{
		id: id, columns: append([]TableColumn(nil), columns...), rows: cloneTableRows(rows),
		columnAlignments: make([]tui.HorizontalAlignment, len(columns)),
		selected:         selected, enabled: onSelect != nil, style: DefaultTableStyle(), onSelect: onSelect,
	}
}

// Enabled sets whether the table can receive focus and emit selection messages
func (t Table[Message]) Enabled(enabled bool) Table[Message] {
	t.enabled = enabled && t.onSelect != nil
	return t
}

// Style replaces the table styles
func (t Table[Message]) Style(style TableStyle) Table[Message] {
	t.style = style
	return t
}

// ColumnAlignment sets horizontal alignment for one zero-based column
//
// Out-of-range columns are ignored. The default is start alignment.
func (t Table[Message]) ColumnAlignment(column int, alignment tui.HorizontalAlignment) Table[Message] {
	if column < 0 || column >= len(t.columnAlignments) {
		return t
	}
	t.columnAlignments = append([]tui.HorizontalAlignment(nil), t.columnAlignments...)
	if alignment < tui.AlignStart || alignment > tui.AlignEnd {
		alignment = tui.AlignStart
	}
	t.columnAlignments[column] = alignment
	return t
}

// Viewport keeps the header fixed and wraps data rows in a Core ScrollViewport
//
// viewportID must be distinct from the table root and row IDs. Applications
// may control its retained offset through Runtime.SetScrollOffset. The
// viewport does not virtualize rows; pass a bounded row set for large
// collections.
func (t Table[Message]) Viewport(viewportID tui.NodeID, bodyHeight tui.Length) Table[Message] {
	t.viewportID = viewportID
	t.viewportHeight = bodyHeight
	t.hasViewport = true
	return t
}

// Node builds the public semantic node for this table
func (t Table[Message]) Node() tui.Node[Message] {
	rowCount := len(t.rows)
	selected, hasSelection := navigateSelection(rowCount, t.selected, navigationNormalize)
	headings := make([]string, len(t.columns))
	for index, column := range t.columns {
		headings[index] = column.Title
	}
	header := tableRowNode[Message]("  ", headings, t.columns, t.columnAlignments, t.style.Header)
	rows := make([]tui.Node[Message], 0, len(t.rows))
	for index, row := range t.rows {
		isSelected := hasSelection && index == selected
		style := t.style.Normal
		if isSelected {
			style = t.style.Selected
		}
		if !t.enabled {
			style = t.style.Disabled
		}
		marker := "  "
		if isSelected {
			marker = "> "
		}
		node := tableRowNode[Message](marker, row.Cells, t.columns, t.columnAlignments, style)
		if !t.enabled {
			rows = append(rows, node.WithID(row.ID))
			continue
		}
		selection := index
		rowID := row.ID
		rowNode := node.WithID(rowID).OnEvent(rowID, func(event vt.Event) tui.EventResult[Message] {
			if !isActivationEvent(event) {
				return tui.IgnoreResult[Message]()
			}
			return tui.MessageResult(t.onSelect(selection)).Focus(t.id)
		})
		if !isSelected {
			rows = append(rows, rowNode)
			continue
		}
		rows = append(rows, tui.Column(rowNode).
			Focusable(t.id).
			WithFocusedStyle(t.style.Focused).
			OnEvent(t.id, func(event vt.Event) tui.EventResult[Message] {
				next, handled := navigationEvent(event, rowCount, selected)
				if !handled {
					return tui.IgnoreResult[Message]()
				}
				result := tui.ConsumeResult[Message]().Focus(t.id)
				if next != selected {
					result = result.Emit(t.onSelect(next))
				}
				return result
			}))
	}

	var root tui.Node[Message]
	if t.hasViewport {
		body := tui.ScrollViewportWithOptions(
			t.viewportID,
			tui.Column(rows...),
			tui.ScrollViewportOptions[Message]{
				Axis:                 tui.ScrollAxisVertical,
				EnsureFocusedVisible: true,
			},
		).TabStop(false).WithLength(t.viewportHeight)
		root = tui.Column(header, body)
	} else {
		children := make([]tui.Node[Message], 0, len(rows)+1)
		children = append(children, header)
		children = append(children, rows...)
		root = tui.Column(children...)
	}
	if t.enabled && hasSelection {
		return root
	}
	return root.WithID(t.id)
}

func cloneTableRows(rows []TableRow) []TableRow {
	cloned := make([]TableRow, len(rows))
	for index, row := range rows {
		cloned[index] = NewTableRow(row.ID, row.Cells)
	}
	return cloned
}

func tableRowNode[Message any](
	marker string,
	values []string,
	columns []TableColumn,
	alignments []tui.HorizontalAlignment,
	style vt.Style,
) tui.Node[Message] {
	children := make([]tui.Node[Message], 0, len(columns)*2+1)
	children = append(children, tui.StyledText[Message](marker, style))
	for index, column := range columns {
		if index != 0 {
			children = append(children, tui.StyledText[Message](" │ ", style))
		}
		value := ""
		if index < len(values) {
			value = values[index]
		}
		alignment := tui.AlignStart
		if index < len(alignments) {
			alignment = alignments[index]
		}
		cell := tui.Paragraph[Message](
			[]tui.TextSpan{tui.NewTextSpan(value, style)},
			tui.ParagraphOptions{Wrap: tui.WrapNone, Alignment: alignment},
		).WithLength(column.Width)
		children = append(children, cell)
	}
	return tui.Row(children...)
}
