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
	preferredColumn  int
	hasPreferred     bool
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

// PreferredColumn returns the terminal-cell column retained across vertical
// movement
func (s TextAreaState) PreferredColumn() (int, bool) {
	return s.preferredColumn, s.hasPreferred
}

// TextAreaBoundaryNavigation controls vertical movement at the first and last
// visual line
type TextAreaBoundaryNavigation uint8

const (
	// TextAreaBoundaryConsume consumes a boundary action without emitting state
	TextAreaBoundaryConsume TextAreaBoundaryNavigation = iota
	// TextAreaBoundaryBubble lets an ancestor action handle boundary movement
	TextAreaBoundaryBubble
)

// TextAreaStyle contains the visual styles used by a TextArea
type TextAreaStyle struct {
	// Normal is used by editable text
	Normal vt.Style
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
		Placeholder: vt.Style{Dim: true}, Focused: vt.Style{Underline: true},
		Disabled: vt.Style{Dim: true},
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
	widthProfile   celltext.WidthProfile
	wrapWidth      int
	hasWrap        bool
	boundary       TextAreaBoundaryNavigation
	viewport       *textAreaViewport
	onChange       func(TextAreaState) Message
	onUndo         func() Message
	onRedo         func() Message
}

type textAreaViewport struct {
	id      tui.NodeID
	caretID tui.NodeID
	height  tui.Length
}

type textAreaInsertionPolicy func(TextAreaState, string) (string, bool)

// NewTextArea returns a text area using application-owned editing state
//
// A nil onChange function creates a disabled text area
func NewTextArea[Message any](id tui.NodeID, state TextAreaState, onChange func(TextAreaState) Message) TextArea[Message] {
	state = normalizeTextAreaState(state)
	return TextArea[Message]{
		id: id, state: state, enabled: onChange != nil,
		style: DefaultTextAreaStyle(), selectionStyle: vt.Style{Reverse: true},
		widthProfile: celltext.ModernWidth(), onChange: onChange,
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

// WidthProfile sets the terminal cell-width policy used by editing and layout
//
// Pass ViewContext.WidthProfile to keep the widget aligned with its Runtime.
func (a TextArea[Message]) WidthProfile(profile celltext.WidthProfile) TextArea[Message] {
	a.widthProfile = profile
	return a
}

// NoWrap uses logical lines with the application-owned horizontal offset
func (a TextArea[Message]) NoWrap() TextArea[Message] {
	a.hasWrap = false
	a.wrapWidth = 0
	return a
}

// SoftWrap wraps visual lines at a terminal-cell width without changing text
//
// A non-positive width is normalized to one Cell. Applications should
// recompute the width from ViewContext after a resize.
func (a TextArea[Message]) SoftWrap(width int) TextArea[Message] {
	a.hasWrap = true
	a.wrapWidth = max(width, 1)
	return a
}

// BoundaryNavigation sets how Up and Down behave at visual-line boundaries
func (a TextArea[Message]) BoundaryNavigation(navigation TextAreaBoundaryNavigation) TextArea[Message] {
	a.boundary = navigation
	return a
}

// Viewport wraps the editor in a vertical viewport that follows its caret
//
// viewportID, caretID, and the TextArea root ID must be distinct stable IDs.
// The viewport is not an additional Tab stop.
func (a TextArea[Message]) Viewport(viewportID, caretID tui.NodeID, height tui.Length) TextArea[Message] {
	a.viewport = &textAreaViewport{id: viewportID, caretID: caretID, height: height}
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
// descriptor is disabled-pass-through when the text area is disabled. Bubble
// mode also disables Up or Down at its corresponding visual edge.
func (a TextArea[Message]) ActionDescriptors() []tui.ActionDescriptor {
	descriptors := textAreaActionDescriptors(
		a.enabled, a.onUndo != nil, a.onRedo != nil,
		a.boundary, a.state, a.wrapWidth, a.hasWrap,
		a.widthProfile,
	)
	return append([]tui.ActionDescriptor(nil), descriptors[:]...)
}

// Node builds the public semantic node for this text area
func (a TextArea[Message]) Node() tui.Node[Message] {
	descriptors := textAreaActionDescriptors(
		a.enabled, a.onUndo != nil, a.onRedo != nil,
		a.boundary, a.state, a.wrapWidth, a.hasWrap,
		a.widthProfile,
	)
	return a.nodeWithActions(descriptors, nil, nil)
}

func (a TextArea[Message]) nodeWithActions(
	descriptors [textAreaActionCount]tui.ActionDescriptor,
	leadingActions []tui.Action[Message],
	insertionPolicy textAreaInsertionPolicy,
) tui.Node[Message] {
	var caretID tui.NodeID
	hasCaretID := a.viewport != nil
	if hasCaretID {
		caretID = a.viewport.caretID
	}
	content := textAreaContent[Message](
		a.state, a.placeholder, a.enabled, a.style, a.selectionStyle,
		a.wrapWidth, a.hasWrap, a.id, caretID, hasCaretID,
		a.widthProfile,
	)
	if !a.enabled {
		textActions := disabledTextAreaActions[Message](descriptors)
		actions := textActions[:]
		if len(leadingActions) != 0 {
			actions = append(append(
				make([]tui.Action[Message], 0, len(leadingActions)+len(textActions)),
				leadingActions...,
			), textActions[:]...)
		}
		node := content.WithID(a.id).OnActions(a.id, actions)
		return textAreaViewportNode(node, a.viewport)
	}
	context := &textAreaActionContext[Message]{
		id: a.id, state: a.state, wrapWidth: a.wrapWidth, hasWrap: a.hasWrap,
		widthProfile: a.widthProfile,
		onChange:     a.onChange, onUndo: a.onUndo, onRedo: a.onRedo,
		insertionPolicy: insertionPolicy,
	}
	textActions := newTextAreaActions(descriptors, context)
	actions := textActions[:]
	if len(leadingActions) != 0 {
		actions = append(append(
			make([]tui.Action[Message], 0, len(leadingActions)+len(textActions)),
			leadingActions...,
		), textActions[:]...)
	}
	node := content.
		Focusable(a.id).
		WithFocusedStyle(a.style.Focused).
		OnActions(a.id, actions).
		OnEvent(a.id, func(event vt.Event) tui.EventResult[Message] {
			next, handled, accepted := rawTextAreaEditForEvent(
				context.state, event, context.insertionPolicy,
			)
			if !handled {
				return tui.IgnoreResult[Message]()
			}
			if !accepted {
				return tui.ConsumeResult[Message]().Focus(context.id)
			}
			return textAreaChangeResult(context, next)
		})
	return textAreaViewportNode(node, a.viewport)
}

func textAreaViewportNode[Message any](node tui.Node[Message], viewport *textAreaViewport) tui.Node[Message] {
	if viewport == nil {
		return node
	}
	return tui.ScrollViewportWithOptions(
		viewport.id,
		node,
		tui.ScrollViewportOptions[Message]{Axis: tui.ScrollAxisVertical},
	).RevealDescendant(viewport.caretID).TabStop(false).WithLength(viewport.height)
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

func textAreaActionDescriptors(
	enabled, hasUndo, hasRedo bool,
	boundary TextAreaBoundaryNavigation,
	state TextAreaState,
	wrapWidth int,
	hasWrap bool,
	profile celltext.WidthProfile,
) [textAreaActionCount]tui.ActionDescriptor {
	hasUp, hasDown := true, true
	if boundary == TextAreaBoundaryBubble {
		hasUp, hasDown = textAreaVisualLineDirections(state, wrapWidth, hasWrap, profile)
	}
	descriptors := defaultTextAreaActionDescriptors
	for index := range descriptors {
		available := enabled
		action := textAreaSemanticAction(index)
		if action == textAreaUndo {
			available = available && hasUndo
		} else if action == textAreaRedo {
			available = available && hasRedo
		} else if action == textAreaCursorUp || action == textAreaSelectionExtendUp {
			available = available && hasUp
		} else if action == textAreaCursorDown || action == textAreaSelectionExtendDown {
			available = available && hasDown
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
	id              tui.NodeID
	state           TextAreaState
	wrapWidth       int
	hasWrap         bool
	widthProfile    celltext.WidthProfile
	onChange        func(TextAreaState) Message
	onUndo          func() Message
	onRedo          func() Message
	insertionPolicy textAreaInsertionPolicy
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
	case textAreaInsertLineBreak:
		next, accepted := applyTextAreaInsertion(context.state, "\n", context.insertionPolicy)
		if !accepted {
			return tui.ConsumeResult[Message]().Focus(context.id)
		}
		return textAreaChangeResult(context, next)
	default:
		return textAreaChangeResult(
			context,
			textAreaStateForAction(context.state, action, context.wrapWidth, context.hasWrap, context.widthProfile),
		)
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

func textAreaStateForAction(
	state TextAreaState,
	action textAreaSemanticAction,
	wrapWidth int,
	hasWrap bool,
	profile celltext.WidthProfile,
) TextAreaState {
	switch action {
	case textAreaCursorLeft:
		return applyTextAreaMovement(state, textAreaLeft, false)
	case textAreaCursorRight:
		return applyTextAreaMovement(state, textAreaRight, false)
	case textAreaCursorUp:
		return applyVerticalTextAreaMovement(state, false, false, wrapWidth, hasWrap, profile)
	case textAreaCursorDown:
		return applyVerticalTextAreaMovement(state, true, false, wrapWidth, hasWrap, profile)
	case textAreaCursorLineStart:
		return applyTextAreaMovement(state, textAreaHome, false)
	case textAreaCursorLineEnd:
		return applyTextAreaMovement(state, textAreaEnd, false)
	case textAreaSelectionExtendLeft:
		return applyTextAreaMovement(state, textAreaLeft, true)
	case textAreaSelectionExtendRight:
		return applyTextAreaMovement(state, textAreaRight, true)
	case textAreaSelectionExtendUp:
		return applyVerticalTextAreaMovement(state, false, true, wrapWidth, hasWrap, profile)
	case textAreaSelectionExtendDown:
		return applyVerticalTextAreaMovement(state, true, true, wrapWidth, hasWrap, profile)
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

func textAreaContent[Message any](
	state TextAreaState,
	placeholder string,
	enabled bool,
	style TextAreaStyle,
	selectionStyle vt.Style,
	wrapWidth int,
	hasWrap bool,
	focusOwner tui.NodeID,
	caretID tui.NodeID,
	hasCaretID bool,
	profile celltext.WidthProfile,
) tui.Node[Message] {
	state = normalizeTextAreaState(state)
	if state.value == "" {
		if enabled {
			return tui.Row(
				textAreaCaret[Message](focusOwner, caretID, hasCaretID),
				tui.StyledText[Message](placeholder, style.Placeholder),
			)
		}
		return tui.StyledText[Message](placeholder, style.Disabled)
	}

	lines := textAreaVisualLineRanges(state.value, wrapWidth, hasWrap, profile)
	cursorLine := textAreaVisualLineIndex(lines, state.cursor)
	cursorWraps := textAreaCursorWraps(state, lines[cursorLine], wrapWidth, hasWrap, profile)
	horizontalOffset := state.horizontalOffset
	if hasWrap {
		horizontalOffset = 0
	}
	nodes := make([]tui.Node[Message], 0, len(lines)+1)
	for index, line := range lines {
		lineStyle := style.Normal
		if !enabled {
			lineStyle = style.Disabled
		}
		lineHasCaretID := enabled && index == cursorLine && !cursorWraps && hasCaretID
		nodes = append(nodes, textAreaLineContent[Message](
			state, line, enabled && index == cursorLine && !cursorWraps,
			lineStyle, selectionStyle, horizontalOffset,
			focusOwner, caretID, lineHasCaretID,
			profile,
		))
		if index == cursorLine && cursorWraps {
			if enabled {
				nodes = append(nodes, textAreaCaret[Message](focusOwner, caretID, hasCaretID))
			} else {
				nodes = append(nodes, tui.StyledText[Message]("", style.Disabled))
			}
		}
	}
	return tui.Column(nodes...)
}

func textAreaLineContent[Message any](
	state TextAreaState,
	line textAreaRange,
	cursorLine bool,
	normalStyle, selectionStyle vt.Style,
	horizontalOffset int,
	focusOwner tui.NodeID,
	caretID tui.NodeID,
	hasCaretID bool,
	profile celltext.WidthProfile,
) tui.Node[Message] {
	lineText := state.value[line.start:line.end]
	visibleStart := line.start + textAreaVisibleStart(lineText, horizontalOffset, profile)
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
		cursorCell, ok := celltext.CellAtByte(lineText, state.cursor-line.start, profile)
		cursorVisible = ok && cursorCell >= horizontalOffset
	}
	parts := make([]tui.Node[Message], 0, len(boundaries)*2)
	if cursorLine && !cursorVisible && hasCaretID {
		parts = append(parts, tui.StyledText[Message]("", normalStyle).WithID(caretID))
	}
	for index := 0; index+1 < len(boundaries); index++ {
		start, end := boundaries[index], boundaries[index+1]
		if cursorVisible && state.cursor == start {
			parts = append(parts, textAreaCaret[Message](focusOwner, caretID, hasCaretID))
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
		parts = append(parts, textAreaCaret[Message](focusOwner, caretID, hasCaretID))
	}
	if len(parts) == 0 {
		return tui.StyledText[Message]("", normalStyle)
	}
	return tui.Row(parts...)
}

func textAreaCaret[Message any](focusOwner, id tui.NodeID, hasID bool) tui.Node[Message] {
	caret := tui.CursorAnchor[Message](focusOwner)
	if hasID {
		caret = caret.WithID(id)
	}
	return caret
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

func textAreaVisibleStart(line string, offset int, profile celltext.WidthProfile) int {
	if offset <= 0 {
		return 0
	}
	cells := 0
	graphemes := celltext.IterateGraphemes(line)
	for grapheme, ok := graphemes.Next(); ok; grapheme, ok = graphemes.Next() {
		if cells >= offset {
			return grapheme.Start
		}
		cells += celltext.GraphemeWidth(grapheme.Text, profile)
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

func rawTextAreaEditForEvent(
	state TextAreaState,
	event vt.Event,
	insertionPolicy textAreaInsertionPolicy,
) (TextAreaState, bool, bool) {
	if event.Kind == vt.EventText || event.Kind == vt.EventPaste {
		next, accepted := applyTextAreaInsertion(state, event.Text, insertionPolicy)
		return next, true, accepted
	}
	return TextAreaState{}, false, false
}

func applyTextAreaInsertion(
	state TextAreaState,
	inserted string,
	insertionPolicy textAreaInsertionPolicy,
) (TextAreaState, bool) {
	inserted = celltext.NormalizeUTF8(inserted)
	if insertionPolicy != nil {
		var accepted bool
		inserted, accepted = insertionPolicy(state, inserted)
		if !accepted {
			return state, false
		}
	}
	return applyTextAreaEdit(state, textAreaInsert, inserted), true
}

func selectAllTextAreaState(state TextAreaState) TextAreaState {
	state = normalizeTextAreaState(state)
	state.cursor = len(state.value)
	state.selectionAnchor = 0
	state.hasSelection = state.cursor != 0
	state.preferredColumn = 0
	state.hasPreferred = false
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
		return applyVerticalTextAreaMovement(state, false, false, 0, false, celltext.ModernWidth())
	case textAreaDown:
		return applyVerticalTextAreaMovement(state, true, false, 0, false, celltext.ModernWidth())
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
		if start == cursor {
			return state
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
		if end == cursor {
			return state
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
			return textAreaMovedState(state, start, false, 0, false)
		}
		if edit == textAreaRight {
			return textAreaMovedState(state, end, false, 0, false)
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
	case textAreaHome:
		target = currentTextAreaLine(state.value, state.cursor).start
	case textAreaEnd:
		target = currentTextAreaLine(state.value, state.cursor).end
	default:
		panic("widget: invalid text area movement")
	}
	if target == state.cursor && !state.hasSelection {
		return state
	}
	return textAreaMovedState(state, target, extend, 0, false)
}

func applyVerticalTextAreaMovement(
	state TextAreaState,
	down bool,
	extend bool,
	wrapWidth int,
	hasWrap bool,
	profile celltext.WidthProfile,
) TextAreaState {
	state = normalizeTextAreaState(state)
	lines := textAreaVisualLineRanges(state.value, wrapWidth, hasWrap, profile)
	current := textAreaVisualLineIndex(lines, state.cursor)
	target := max(current-1, 0)
	if down {
		target = min(current+1, len(lines)-1)
	}
	if target == current {
		return state
	}
	preferred := state.preferredColumn
	if !state.hasPreferred {
		currentLine := state.value[lines[current].start:lines[current].end]
		var ok bool
		preferred, ok = celltext.CellAtByte(
			currentLine,
			state.cursor-lines[current].start,
			profile,
		)
		if !ok {
			preferred = 0
		}
	}
	targetLine := state.value[lines[target].start:lines[target].end]
	relative := len(celltext.Truncate(targetLine, preferred, profile))
	return textAreaMovedState(
		state,
		lines[target].start+relative,
		extend,
		preferred,
		true,
	)
}

func textAreaMovedState(
	state TextAreaState,
	cursor int,
	extend bool,
	preferredColumn int,
	hasPreferred bool,
) TextAreaState {
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
	state.preferredColumn = preferredColumn
	state.hasPreferred = hasPreferred
	return normalizeTextAreaState(state)
}

func textAreaEditedState(state TextAreaState, value string, cursor int) TextAreaState {
	if value == state.value && cursor == state.cursor && !state.hasSelection {
		return state
	}
	return normalizeTextAreaState(TextAreaState{
		value: value, cursor: cursor, horizontalOffset: state.horizontalOffset,
	})
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

func textAreaVisualLineRanges(value string, wrapWidth int, hasWrap bool, profile celltext.WidthProfile) []textAreaRange {
	logicalLines := textAreaLineRanges(value)
	if !hasWrap {
		return logicalLines
	}
	wrapWidth = max(wrapWidth, 1)
	visualLines := make([]textAreaRange, 0, len(logicalLines))
	for _, logical := range logicalLines {
		if logical.start == logical.end {
			visualLines = append(visualLines, logical)
			continue
		}
		start := logical.start
		cells := 0
		graphemes := celltext.IterateGraphemes(value[logical.start:logical.end])
		for grapheme, ok := graphemes.Next(); ok; grapheme, ok = graphemes.Next() {
			graphemeStart := logical.start + grapheme.Start
			width := celltext.GraphemeWidth(grapheme.Text, profile)
			next := cells + width
			if width != 0 && graphemeStart != start && next > wrapWidth {
				visualLines = append(visualLines, textAreaRange{start: start, end: graphemeStart})
				start = graphemeStart
				cells = width
			} else {
				cells = next
			}
		}
		visualLines = append(visualLines, textAreaRange{start: start, end: logical.end})
	}
	return visualLines
}

func textAreaCursorWraps(
	state TextAreaState,
	line textAreaRange,
	wrapWidth int,
	hasWrap bool,
	profile celltext.WidthProfile,
) bool {
	return hasWrap && state.cursor == line.end && line.start != line.end &&
		celltext.Width(state.value[line.start:line.end], profile) == max(wrapWidth, 1)
}

func textAreaVisualLineCount(state TextAreaState, wrapWidth int, hasWrap bool, profile celltext.WidthProfile) int {
	state = normalizeTextAreaState(state)
	lines := textAreaVisualLineRanges(state.value, wrapWidth, hasWrap, profile)
	count := len(lines)
	if textAreaCursorWraps(state, lines[textAreaVisualLineIndex(lines, state.cursor)], wrapWidth, hasWrap, profile) {
		count++
	}
	return count
}

func textAreaVisualLineIndex(lines []textAreaRange, cursor int) int {
	for index, line := range lines {
		if cursor < line.start || cursor > line.end {
			continue
		}
		if cursor < line.end || index+1 == len(lines) || lines[index+1].start != line.end {
			return index
		}
	}
	return max(len(lines)-1, 0)
}

func textAreaVisualLineDirections(
	state TextAreaState,
	wrapWidth int,
	hasWrap bool,
	profile celltext.WidthProfile,
) (bool, bool) {
	state = normalizeTextAreaState(state)
	lines := textAreaVisualLineRanges(state.value, wrapWidth, hasWrap, profile)
	current := textAreaVisualLineIndex(lines, state.cursor)
	return current > 0, current+1 < len(lines)
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
