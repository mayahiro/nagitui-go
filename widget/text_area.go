package widget

import (
	"sort"
	"unicode"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

// TextAreaState is application-owned value and grapheme-aligned cursor state
type TextAreaState struct {
	value            string
	cursor           int
	selectionAnchor  int
	hasSelection     bool
	horizontalOffset int
}

// NewTextAreaState returns state with cursor clamped down to a grapheme boundary
func NewTextAreaState(value string, cursor int) TextAreaState {
	return normalizeTextAreaState(TextAreaState{value: value, cursor: cursor})
}

// NewTextAreaStateAtEnd returns state with cursor at the end of the value
func NewTextAreaStateAtEnd(value string) TextAreaState {
	value = celltext.NormalizeUTF8(value)
	return normalizeTextAreaState(TextAreaState{value: value, cursor: len(value)})

}

// NewTextAreaStateWithSelection returns state selecting the grapheme-aligned
// range between anchor and cursor
func NewTextAreaStateWithSelection(value string, cursor, anchor int) TextAreaState {
	return normalizeTextAreaState(TextAreaState{
		value: value, cursor: cursor, selectionAnchor: anchor, hasSelection: true,
	})
}

// Value returns the normalized UTF-8 value without Unicode normalization
func (s TextAreaState) Value() string {
	return s.value
}

// Cursor returns the UTF-8 byte cursor at a grapheme boundary
func (s TextAreaState) Cursor() int {
	return s.cursor
}

// Selection returns the ordered UTF-8 byte range selected by the state
func (s TextAreaState) Selection() (start, end int, ok bool) {
	s = normalizeTextAreaState(s)
	if !s.hasSelection {
		return 0, 0, false
	}
	return min(s.cursor, s.selectionAnchor), max(s.cursor, s.selectionAnchor), true
}

// Select returns state selecting the range between anchor and the cursor
func (s TextAreaState) Select(anchor int) TextAreaState {
	s.selectionAnchor = anchor
	s.hasSelection = true
	return normalizeTextAreaState(s)
}

// ClearSelection returns state with the selection collapsed at the cursor
func (s TextAreaState) ClearSelection() TextAreaState {
	s.selectionAnchor = s.cursor
	s.hasSelection = false
	return normalizeTextAreaState(s)
}

// WithHorizontalOffset returns state rendered from the requested cell offset
func (s TextAreaState) WithHorizontalOffset(offset int) TextAreaState {
	s.horizontalOffset = max(offset, 0)
	return normalizeTextAreaState(s)
}

// HorizontalOffset returns the leading terminal cells omitted from each line
func (s TextAreaState) HorizontalOffset() int {
	return max(s.horizontalOffset, 0)
}

// TextAreaStyle contains the visual styles used by a TextArea
type TextAreaStyle struct {
	// Normal is used by editable text
	Normal vt.Style
	// Cursor is used by the visible cursor marker
	Cursor vt.Style
	// Placeholder is used by placeholder text
	Placeholder vt.Style
	// Focused is merged over the area while it owns focus
	Focused vt.Style
	// Disabled is used by text while the area is disabled
	Disabled vt.Style
}

// DefaultTextAreaStyle returns the standard text area styles
func DefaultTextAreaStyle() TextAreaStyle {
	return TextAreaStyle{
		Cursor: vt.Style{Reverse: true}, Placeholder: vt.Style{Dim: true},
		Focused: vt.Style{Underline: true}, Disabled: vt.Style{Dim: true},
	}
}

// TextArea is a controlled multiline editor with grapheme-safe cursor movement
type TextArea[Message any] struct {
	id             tui.NodeID
	state          TextAreaState
	placeholder    string
	enabled        bool
	style          TextAreaStyle
	selectionStyle vt.Style
	onChange       func(TextAreaState) Message
	onUndo         func() Message
	onRedo         func() Message
}

// NewTextArea returns a text area using application-owned editing state
//
// A nil onChange function creates a disabled text area
func NewTextArea[Message any](id tui.NodeID, state TextAreaState, onChange func(TextAreaState) Message) TextArea[Message] {
	state = normalizeTextAreaState(state)
	return TextArea[Message]{
		id: id, state: state, enabled: onChange != nil,
		style: DefaultTextAreaStyle(), selectionStyle: vt.Style{Reverse: true}, onChange: onChange,
	}
}

// Placeholder sets text shown when the value is empty
func (a TextArea[Message]) Placeholder(placeholder string) TextArea[Message] {
	a.placeholder = celltext.NormalizeUTF8(placeholder)
	return a
}

// Enabled sets whether the area can receive focus and edit its value
func (a TextArea[Message]) Enabled(enabled bool) TextArea[Message] {
	a.enabled = enabled && a.onChange != nil
	return a
}

// Style replaces the text area styles
func (a TextArea[Message]) Style(style TextAreaStyle) TextArea[Message] {
	a.style = style
	return a
}

// SelectionStyle sets the style merged over selected text
func (a TextArea[Message]) SelectionStyle(style vt.Style) TextArea[Message] {
	a.selectionStyle = style
	return a
}

// OnUndo sets the message emitted for Control-Z
func (a TextArea[Message]) OnUndo(handler func() Message) TextArea[Message] {
	a.onUndo = handler
	return a
}

// OnRedo sets the message emitted for Control-Y and Control-Shift-Z
func (a TextArea[Message]) OnRedo(handler func() Message) TextArea[Message] {
	a.onRedo = handler
	return a
}

// Node builds the public semantic node for this text area
func (a TextArea[Message]) Node() tui.Node[Message] {
	content := textAreaContent[Message](a.state, a.placeholder, a.enabled, a.style, a.selectionStyle)
	if !a.enabled {
		return content.WithID(a.id)
	}
	return content.
		Focusable(a.id).
		WithFocusedStyle(a.style.Focused).
		OnEvent(a.id, func(event vt.Event) tui.EventResult[Message] {
			if historyAction, ok := textAreaHistoryActionForEvent(event); ok {
				handler := a.onUndo
				if historyAction == textAreaRedoHistory {
					handler = a.onRedo
				}
				if handler == nil {
					return tui.IgnoreResult[Message]()
				}
				return tui.ConsumeResult[Message]().Focus(a.id).Emit(handler())
			}
			next, handled := textAreaEditForEvent(a.state, event)
			if !handled {
				return tui.IgnoreResult[Message]()
			}
			result := tui.ConsumeResult[Message]().Focus(a.id)
			if next != a.state {
				result = result.Emit(a.onChange(next))
			}
			return result
		})
}

func textAreaContent[Message any](state TextAreaState, placeholder string, enabled bool, style TextAreaStyle, selectionStyle vt.Style) tui.Node[Message] {
	state = normalizeTextAreaState(state)
	if state.value == "" {
		if enabled {
			return tui.Row(
				tui.StyledText[Message]("▏", style.Cursor),
				tui.StyledText[Message](placeholder, style.Placeholder),
			)
		}
		return tui.StyledText[Message](placeholder, style.Disabled)
	}

	lines := textAreaLineRanges(state.value)
	cursorLine := len(lines) - 1
	for index, line := range lines {
		if state.cursor >= line.start && state.cursor <= line.end {
			cursorLine = index
			break
		}
	}
	nodes := make([]tui.Node[Message], 0, len(lines))
	for index, line := range lines {
		lineStyle := style.Normal
		if !enabled {
			lineStyle = style.Disabled
		}
		nodes = append(nodes, textAreaLineContent[Message](state, line, enabled && index == cursorLine, lineStyle, style.Cursor, selectionStyle))
	}
	return tui.Column(nodes...)
}

func textAreaLineContent[Message any](state TextAreaState, line textAreaRange, cursorLine bool, normalStyle, cursorStyle, selectionStyle vt.Style) tui.Node[Message] {
	lineText := state.value[line.start:line.end]
	visibleStart := line.start + textAreaVisibleStart(lineText, state.horizontalOffset)
	selectionStart, selectionEnd, selected := state.Selection()
	boundaries := []int{visibleStart, line.end}
	if selected {
		boundaries = appendTextAreaBoundary(boundaries, selectionStart, visibleStart, line.end)
		boundaries = appendTextAreaBoundary(boundaries, selectionEnd, visibleStart, line.end)
	}
	if cursorLine {
		boundaries = appendTextAreaBoundary(boundaries, state.cursor, visibleStart, line.end)
	}
	sort.Ints(boundaries)
	boundaries = compactTextAreaBoundaries(boundaries)

	cursorVisible := false
	if cursorLine && state.cursor >= line.start && state.cursor <= line.end {
		cursorCell, ok := celltext.CellAtByte(lineText, state.cursor-line.start, celltext.ModernWidth())
		cursorVisible = ok && cursorCell >= state.horizontalOffset
	}
	parts := make([]tui.Node[Message], 0, len(boundaries)*2)
	for index := 0; index+1 < len(boundaries); index++ {
		start, end := boundaries[index], boundaries[index+1]
		if cursorVisible && state.cursor == start {
			parts = append(parts, tui.StyledText[Message]("▏", cursorStyle))
			cursorVisible = false
		}
		if start == end {
			continue
		}
		partStyle := normalStyle
		if selected && start >= selectionStart && start < selectionEnd {
			partStyle = partStyle.Merge(selectionStyle)
		}
		parts = append(parts, tui.StyledText[Message](state.value[start:end], partStyle))
	}
	if cursorVisible && state.cursor == line.end {
		parts = append(parts, tui.StyledText[Message]("▏", cursorStyle))
	}
	if len(parts) == 0 {
		return tui.StyledText[Message]("", normalStyle)
	}
	return tui.Row(parts...)
}

func appendTextAreaBoundary(boundaries []int, boundary, start, end int) []int {
	if boundary >= start && boundary <= end {
		return append(boundaries, boundary)
	}
	return boundaries
}

func compactTextAreaBoundaries(boundaries []int) []int {
	if len(boundaries) == 0 {
		return boundaries
	}
	output := boundaries[:1]
	for _, boundary := range boundaries[1:] {
		if boundary != output[len(output)-1] {
			output = append(output, boundary)
		}
	}
	return output
}

func textAreaVisibleStart(line string, offset int) int {
	if offset <= 0 {
		return 0
	}
	cells := 0
	graphemes := celltext.IterateGraphemes(line)
	for grapheme, ok := graphemes.Next(); ok; grapheme, ok = graphemes.Next() {
		if cells >= offset {
			return grapheme.Start
		}
		cells += celltext.GraphemeWidth(grapheme.Text, celltext.ModernWidth())
	}
	return len(line)
}

type textAreaEdit uint8

const (
	textAreaInsert textAreaEdit = iota
	textAreaLeft
	textAreaRight
	textAreaUp
	textAreaDown
	textAreaHome
	textAreaEnd
	textAreaBackspace
	textAreaDelete
)

func textAreaEditForEvent(state TextAreaState, event vt.Event) (TextAreaState, bool) {
	if event.Kind == vt.EventText || event.Kind == vt.EventPaste {
		return applyTextAreaEdit(state, textAreaInsert, event.Text), true
	}
	if event.Kind != vt.EventKey || event.Key.Action == vt.KeyRelease {
		return TextAreaState{}, false
	}
	modifiers := event.Key.Modifiers
	if modifiers.Alt || modifiers.Meta {
		return TextAreaState{}, false
	}
	if modifiers.Control {
		if event.Key.Code == vt.KeyCharacter && unicode.ToLower(event.Key.Character) == 'a' {
			state = normalizeTextAreaState(state)
			state.cursor = len(state.value)
			state.selectionAnchor = 0
			state.hasSelection = state.cursor != 0
			return state, true
		}
		return TextAreaState{}, false
	}
	edit := textAreaInsert
	inserted := ""
	switch event.Key.Code {
	case vt.KeyEnter:
		inserted = "\n"
	case vt.KeyLeft:
		edit = textAreaLeft
	case vt.KeyRight:
		edit = textAreaRight
	case vt.KeyUp:
		edit = textAreaUp
	case vt.KeyDown:
		edit = textAreaDown
	case vt.KeyHome:
		edit = textAreaHome
	case vt.KeyEnd:
		edit = textAreaEnd
	case vt.KeyBackspace:
		edit = textAreaBackspace
	case vt.KeyDelete:
		edit = textAreaDelete
	default:
		return TextAreaState{}, false
	}
	if edit >= textAreaLeft && edit <= textAreaEnd {
		return applyTextAreaMovement(state, edit, modifiers.Shift), true
	}
	return applyTextAreaEdit(state, edit, inserted), true
}

func applyTextAreaEdit(state TextAreaState, edit textAreaEdit, inserted string) TextAreaState {
	state = normalizeTextAreaState(state)
	value := state.value
	cursor := state.cursor
	switch edit {
	case textAreaInsert:
		inserted = celltext.NormalizeUTF8(inserted)
		start, end := cursor, cursor
		if selectionStart, selectionEnd, ok := state.Selection(); ok {
			start, end = selectionStart, selectionEnd
		}
		output := value[:start] + inserted + value[end:]
		intended := start + len(inserted)
		for _, boundary := range celltext.GraphemeBoundaries(output) {
			if boundary >= intended {
				return textAreaEditedState(state, output, boundary)
			}
		}
		return textAreaEditedState(state, output, len(output))
	case textAreaLeft:
		return applyTextAreaMovement(state, edit, false)
	case textAreaRight:
		return applyTextAreaMovement(state, edit, false)
	case textAreaUp:
		return applyTextAreaMovement(state, edit, false)
	case textAreaDown:
		return applyTextAreaMovement(state, edit, false)
	case textAreaHome:
		return applyTextAreaMovement(state, edit, false)
	case textAreaEnd:
		return applyTextAreaMovement(state, edit, false)
	case textAreaBackspace:
		if start, end, ok := state.Selection(); ok {
			return textAreaEditedState(state, value[:start]+value[end:], start)
		}
		start := cursor
		if previous, ok := celltext.PreviousGraphemeBoundary(value, cursor); ok {
			start = previous
		}
		return textAreaEditedState(state, value[:start]+value[cursor:], start)
	case textAreaDelete:
		if start, end, ok := state.Selection(); ok {
			return textAreaEditedState(state, value[:start]+value[end:], start)
		}
		end := cursor
		if next, ok := celltext.NextGraphemeBoundary(value, cursor); ok {
			end = next
		}
		return textAreaEditedState(state, value[:cursor]+value[end:], cursor)
	default:
		panic("widget: invalid text area edit")
	}
}

func applyTextAreaMovement(state TextAreaState, edit textAreaEdit, extend bool) TextAreaState {
	state = normalizeTextAreaState(state)
	if !extend && state.hasSelection {
		start, end, _ := state.Selection()
		if edit == textAreaLeft {
			return textAreaMovedState(state, start, false)
		}
		if edit == textAreaRight {
			return textAreaMovedState(state, end, false)
		}
	}
	target := state.cursor
	switch edit {
	case textAreaLeft:
		target, _ = celltext.PreviousGraphemeBoundary(state.value, state.cursor)
	case textAreaRight:
		if next, ok := celltext.NextGraphemeBoundary(state.value, state.cursor); ok {
			target = next
		} else {
			target = len(state.value)
		}
	case textAreaUp:
		target = verticalTextAreaCursor(state.value, state.cursor, false)
	case textAreaDown:
		target = verticalTextAreaCursor(state.value, state.cursor, true)
	case textAreaHome:
		target = currentTextAreaLine(state.value, state.cursor).start
	case textAreaEnd:
		target = currentTextAreaLine(state.value, state.cursor).end
	default:
		panic("widget: invalid text area movement")
	}
	return textAreaMovedState(state, target, extend)
}

func textAreaMovedState(state TextAreaState, cursor int, extend bool) TextAreaState {
	anchor := state.cursor
	if state.hasSelection {
		anchor = state.selectionAnchor
	}
	state.cursor = cursor
	if extend {
		state.selectionAnchor = anchor
		state.hasSelection = anchor != cursor
	} else {
		state.selectionAnchor = cursor
		state.hasSelection = false
	}
	return normalizeTextAreaState(state)
}

func textAreaEditedState(state TextAreaState, value string, cursor int) TextAreaState {
	return normalizeTextAreaState(TextAreaState{
		value: value, cursor: cursor, horizontalOffset: state.horizontalOffset,
	})
}

func verticalTextAreaCursor(value string, cursor int, down bool) int {
	lines := textAreaLineRanges(value)
	current := len(lines) - 1
	for index, line := range lines {
		if cursor >= line.start && cursor <= line.end {
			current = index
			break
		}
	}
	target := max(current-1, 0)
	if down {
		target = min(current+1, len(lines)-1)
	}
	currentLine := value[lines[current].start:lines[current].end]
	targetLine := value[lines[target].start:lines[target].end]
	column, ok := celltext.CellAtByte(currentLine, cursor-lines[current].start, celltext.ModernWidth())
	if !ok {
		column = 0
	}
	relative := len(celltext.Truncate(targetLine, column, celltext.ModernWidth()))
	return lines[target].start + relative
}

type textAreaHistoryAction uint8

const (
	textAreaUndoHistory textAreaHistoryAction = iota
	textAreaRedoHistory
)

func textAreaHistoryActionForEvent(event vt.Event) (textAreaHistoryAction, bool) {
	if event.Kind != vt.EventKey || event.Key.Action == vt.KeyRelease {
		return 0, false
	}
	modifiers := event.Key.Modifiers
	if !modifiers.Control || modifiers.Alt || modifiers.Meta || event.Key.Code != vt.KeyCharacter {
		return 0, false
	}
	switch unicode.ToLower(event.Key.Character) {
	case 'z':
		if modifiers.Shift {
			return textAreaRedoHistory, true
		}
		return textAreaUndoHistory, true
	case 'y':
		return textAreaRedoHistory, true
	default:
		return 0, false
	}
}

type textAreaRange struct {
	start int
	end   int
}

func currentTextAreaLine(value string, cursor int) textAreaRange {
	for _, line := range textAreaLineRanges(value) {
		if cursor >= line.start && cursor <= line.end {
			return line
		}
	}
	return textAreaRange{start: len(value), end: len(value)}
}

func textAreaLineRanges(value string) []textAreaRange {
	lines := make([]textAreaRange, 0, 1)
	start := 0
	graphemes := celltext.IterateGraphemes(value)
	for grapheme, ok := graphemes.Next(); ok; grapheme, ok = graphemes.Next() {
		if grapheme.Text == "\r" || grapheme.Text == "\n" || grapheme.Text == "\r\n" {
			lines = append(lines, textAreaRange{start: start, end: grapheme.Start})
			start = grapheme.End
		}
	}
	return append(lines, textAreaRange{start: start, end: len(value)})
}

func normalizeTextAreaCursor(value string, cursor int) int {
	value = celltext.NormalizeUTF8(value)
	cursor = min(max(cursor, 0), len(value))
	result := 0
	for _, boundary := range celltext.GraphemeBoundaries(value) {
		if boundary > cursor {
			break
		}
		result = boundary
	}
	return result
}

func normalizeTextAreaState(state TextAreaState) TextAreaState {
	state.value = celltext.NormalizeUTF8(state.value)
	state.cursor = normalizeTextAreaCursor(state.value, state.cursor)
	state.selectionAnchor = normalizeTextAreaCursor(state.value, state.selectionAnchor)
	state.horizontalOffset = max(state.horizontalOffset, 0)
	if !state.hasSelection || state.selectionAnchor == state.cursor {
		state.selectionAnchor = state.cursor
		state.hasSelection = false
	}
	return state
}
