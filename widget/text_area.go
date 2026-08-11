package widget

import (
	"sort"

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
//
// Keyboard editing commands are declared as Core nagi.text.* semantic actions.
// Text and Paste remain raw editing input after local action resolution.
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

// ActionDescriptors returns the ordered semantic actions declared by this text area
//
// The order is cursor movement, selection extension, select all, backward and
// forward deletion, line break, undo, and redo. Undo and redo are
// disabled-pass-through when their corresponding handler is nil. Every
// descriptor is disabled-pass-through when the text area is disabled.
func (a TextArea[Message]) ActionDescriptors() []tui.ActionDescriptor {
	descriptors := textAreaActionDescriptors(a.enabled, a.onUndo != nil, a.onRedo != nil)
	return append([]tui.ActionDescriptor(nil), descriptors[:]...)
}

// Node builds the public semantic node for this text area
func (a TextArea[Message]) Node() tui.Node[Message] {
	content := textAreaContent[Message](a.state, a.placeholder, a.enabled, a.style, a.selectionStyle)
	descriptors := textAreaActionDescriptors(a.enabled, a.onUndo != nil, a.onRedo != nil)
	if !a.enabled {
		actions := disabledTextAreaActions[Message](descriptors)
		return content.WithID(a.id).OnActions(a.id, actions[:])
	}
	context := &textAreaActionContext[Message]{
		id: a.id, state: a.state, onChange: a.onChange, onUndo: a.onUndo, onRedo: a.onRedo,
	}
	actions := newTextAreaActions(descriptors, context)
	return content.
		Focusable(a.id).
		WithFocusedStyle(a.style.Focused).
		OnActions(a.id, actions[:]).
		OnEvent(a.id, func(event vt.Event) tui.EventResult[Message] {
			next, handled := rawTextAreaEditForEvent(context.state, event)
			if !handled {
				return tui.IgnoreResult[Message]()
			}
			return textAreaChangeResult(context, next)
		})
}

const textAreaActionCount = 18

type textAreaSemanticAction uint8

const (
	textAreaCursorLeft textAreaSemanticAction = iota
	textAreaCursorRight
	textAreaCursorUp
	textAreaCursorDown
	textAreaCursorLineStart
	textAreaCursorLineEnd
	textAreaSelectionExtendLeft
	textAreaSelectionExtendRight
	textAreaSelectionExtendUp
	textAreaSelectionExtendDown
	textAreaSelectionExtendLineStart
	textAreaSelectionExtendLineEnd
	textAreaSelectAll
	textAreaDeleteBackward
	textAreaDeleteForward
	textAreaInsertLineBreak
	textAreaUndo
	textAreaRedo
)

var defaultTextAreaActionDescriptors = [textAreaActionCount]tui.ActionDescriptor{
	textAreaActionDescriptor(
		tui.TextCursorLeftActionID, "Move cursor left",
		[]tui.KeyBinding{textAreaActionBinding(vt.KeyLeft, vt.Modifiers{})},
	),
	textAreaActionDescriptor(
		tui.TextCursorRightActionID, "Move cursor right",
		[]tui.KeyBinding{textAreaActionBinding(vt.KeyRight, vt.Modifiers{})},
	),
	textAreaActionDescriptor(
		tui.TextCursorUpActionID, "Move cursor up",
		[]tui.KeyBinding{textAreaActionBinding(vt.KeyUp, vt.Modifiers{})},
	),
	textAreaActionDescriptor(
		tui.TextCursorDownActionID, "Move cursor down",
		[]tui.KeyBinding{textAreaActionBinding(vt.KeyDown, vt.Modifiers{})},
	),
	textAreaActionDescriptor(
		tui.TextCursorLineStartActionID, "Move to line start",
		[]tui.KeyBinding{textAreaActionBinding(vt.KeyHome, vt.Modifiers{})},
	),
	textAreaActionDescriptor(
		tui.TextCursorLineEndActionID, "Move to line end",
		[]tui.KeyBinding{textAreaActionBinding(vt.KeyEnd, vt.Modifiers{})},
	),
	textAreaActionDescriptor(
		tui.TextSelectionExtendLeftActionID, "Extend selection left",
		[]tui.KeyBinding{textAreaActionBinding(vt.KeyLeft, vt.Modifiers{Shift: true})},
	),
	textAreaActionDescriptor(
		tui.TextSelectionExtendRightActionID, "Extend selection right",
		[]tui.KeyBinding{textAreaActionBinding(vt.KeyRight, vt.Modifiers{Shift: true})},
	),
	textAreaActionDescriptor(
		tui.TextSelectionExtendUpActionID, "Extend selection up",
		[]tui.KeyBinding{textAreaActionBinding(vt.KeyUp, vt.Modifiers{Shift: true})},
	),
	textAreaActionDescriptor(
		tui.TextSelectionExtendDownActionID, "Extend selection down",
		[]tui.KeyBinding{textAreaActionBinding(vt.KeyDown, vt.Modifiers{Shift: true})},
	),
	textAreaActionDescriptor(
		tui.TextSelectionExtendLineStartActionID, "Extend selection to line start",
		[]tui.KeyBinding{textAreaActionBinding(vt.KeyHome, vt.Modifiers{Shift: true})},
	),
	textAreaActionDescriptor(
		tui.TextSelectionExtendLineEndActionID, "Extend selection to line end",
		[]tui.KeyBinding{textAreaActionBinding(vt.KeyEnd, vt.Modifiers{Shift: true})},
	),
	textAreaActionDescriptor(
		tui.TextSelectAllActionID, "Select all",
		[]tui.KeyBinding{textAreaCharacterActionBinding('a', vt.Modifiers{Control: true})},
	),
	textAreaActionDescriptor(
		tui.TextDeleteBackwardActionID, "Delete backward",
		[]tui.KeyBinding{textAreaActionBinding(vt.KeyBackspace, vt.Modifiers{})},
	),
	textAreaActionDescriptor(
		tui.TextDeleteForwardActionID, "Delete forward",
		[]tui.KeyBinding{textAreaActionBinding(vt.KeyDelete, vt.Modifiers{})},
	),
	textAreaActionDescriptor(
		tui.TextInsertLineBreakActionID, "Insert line break",
		[]tui.KeyBinding{textAreaActionBinding(vt.KeyEnter, vt.Modifiers{})},
	),
	textAreaActionDescriptor(
		tui.TextUndoActionID, "Undo",
		[]tui.KeyBinding{textAreaCharacterActionBinding('z', vt.Modifiers{Control: true})},
	),
	textAreaActionDescriptor(
		tui.TextRedoActionID, "Redo",
		[]tui.KeyBinding{
			textAreaCharacterActionBinding('y', vt.Modifiers{Control: true}),
			textAreaCharacterActionBinding('z', vt.Modifiers{Control: true, Shift: true}),
		},
	),
}

func textAreaActionDescriptor(
	id tui.ActionID,
	label string,
	bindings []tui.KeyBinding,
) tui.ActionDescriptor {
	return tui.NewActionDescriptor(id, label, bindings)
}

func textAreaActionBinding(code vt.KeyCode, modifiers vt.Modifiers) tui.KeyBinding {
	return tui.NewKeyBinding(tui.NewKeyStroke(code, modifiers)).WithRepeatPolicy(tui.RepeatAllow)
}

func textAreaCharacterActionBinding(character rune, modifiers vt.Modifiers) tui.KeyBinding {
	return tui.NewKeyBinding(tui.NewCharacterKeyStroke(character, modifiers)).
		WithRepeatPolicy(tui.RepeatAllow)
}

func textAreaActionDescriptors(enabled, hasUndo, hasRedo bool) [textAreaActionCount]tui.ActionDescriptor {
	descriptors := defaultTextAreaActionDescriptors
	for index := range descriptors {
		available := enabled
		if textAreaSemanticAction(index) == textAreaUndo {
			available = available && hasUndo
		} else if textAreaSemanticAction(index) == textAreaRedo {
			available = available && hasRedo
		}
		availability := tui.ActionEnabled
		if !available {
			availability = tui.ActionDisabledPassThrough
		}
		descriptors[index] = descriptors[index].WithAvailability(availability)
	}
	return descriptors
}

type textAreaActionContext[Message any] struct {
	id       tui.NodeID
	state    TextAreaState
	onChange func(TextAreaState) Message
	onUndo   func() Message
	onRedo   func() Message
}

func newTextAreaActions[Message any](
	descriptors [textAreaActionCount]tui.ActionDescriptor,
	context *textAreaActionContext[Message],
) [textAreaActionCount]tui.Action[Message] {
	var actions [textAreaActionCount]tui.Action[Message]
	for index, descriptor := range descriptors {
		action := textAreaSemanticAction(index)
		actions[index] = tui.NewAction(descriptor, func(tui.ActionEvent) tui.EventResult[Message] {
			return textAreaActionResult(action, context)
		})
	}
	return actions
}

func disabledTextAreaActions[Message any](
	descriptors [textAreaActionCount]tui.ActionDescriptor,
) [textAreaActionCount]tui.Action[Message] {
	var actions [textAreaActionCount]tui.Action[Message]
	for index, descriptor := range descriptors {
		actions[index] = tui.NewAction[Message](descriptor, nil)
	}
	return actions
}

func textAreaActionResult[Message any](
	action textAreaSemanticAction,
	context *textAreaActionContext[Message],
) tui.EventResult[Message] {
	switch action {
	case textAreaUndo:
		if context.onUndo == nil {
			return tui.IgnoreResult[Message]()
		}
		return tui.ConsumeResult[Message]().Focus(context.id).Emit(context.onUndo())
	case textAreaRedo:
		if context.onRedo == nil {
			return tui.IgnoreResult[Message]()
		}
		return tui.ConsumeResult[Message]().Focus(context.id).Emit(context.onRedo())
	default:
		return textAreaChangeResult(context, textAreaStateForAction(context.state, action))
	}
}

func textAreaChangeResult[Message any](
	context *textAreaActionContext[Message],
	next TextAreaState,
) tui.EventResult[Message] {
	result := tui.ConsumeResult[Message]().Focus(context.id)
	if next != context.state {
		result = result.Emit(context.onChange(next))
	}
	return result
}

func textAreaStateForAction(state TextAreaState, action textAreaSemanticAction) TextAreaState {
	switch action {
	case textAreaCursorLeft:
		return applyTextAreaMovement(state, textAreaLeft, false)
	case textAreaCursorRight:
		return applyTextAreaMovement(state, textAreaRight, false)
	case textAreaCursorUp:
		return applyTextAreaMovement(state, textAreaUp, false)
	case textAreaCursorDown:
		return applyTextAreaMovement(state, textAreaDown, false)
	case textAreaCursorLineStart:
		return applyTextAreaMovement(state, textAreaHome, false)
	case textAreaCursorLineEnd:
		return applyTextAreaMovement(state, textAreaEnd, false)
	case textAreaSelectionExtendLeft:
		return applyTextAreaMovement(state, textAreaLeft, true)
	case textAreaSelectionExtendRight:
		return applyTextAreaMovement(state, textAreaRight, true)
	case textAreaSelectionExtendUp:
		return applyTextAreaMovement(state, textAreaUp, true)
	case textAreaSelectionExtendDown:
		return applyTextAreaMovement(state, textAreaDown, true)
	case textAreaSelectionExtendLineStart:
		return applyTextAreaMovement(state, textAreaHome, true)
	case textAreaSelectionExtendLineEnd:
		return applyTextAreaMovement(state, textAreaEnd, true)
	case textAreaSelectAll:
		return selectAllTextAreaState(state)
	case textAreaDeleteBackward:
		return applyTextAreaEdit(state, textAreaBackspace, "")
	case textAreaDeleteForward:
		return applyTextAreaEdit(state, textAreaDelete, "")
	case textAreaInsertLineBreak:
		return applyTextAreaEdit(state, textAreaInsert, "\n")
	default:
		panic("widget: history action does not produce text area state")
	}
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

func rawTextAreaEditForEvent(state TextAreaState, event vt.Event) (TextAreaState, bool) {
	if event.Kind == vt.EventText || event.Kind == vt.EventPaste {
		return applyTextAreaEdit(state, textAreaInsert, event.Text), true
	}
	return TextAreaState{}, false
}

func selectAllTextAreaState(state TextAreaState) TextAreaState {
	state = normalizeTextAreaState(state)
	state.cursor = len(state.value)
	state.selectionAnchor = 0
	state.hasSelection = state.cursor != 0
	return state
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
