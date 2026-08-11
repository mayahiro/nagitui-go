package widget

import (
	"strings"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

// SelectableTextContent is immutable semantic text and styled runs displayed by SelectableText
type SelectableTextContent struct {
	inner *selectableTextContentData
}

type selectableTextContentData struct {
	text     string
	spans    []tui.TextSpan
	copyable bool
}

// NewPlainSelectableTextContent returns content containing one default-style span
func NewPlainSelectableTextContent(text string) SelectableTextContent {
	return NewSelectableTextContent([]tui.TextSpan{tui.NewTextSpan(text, vt.Style{})})
}

// NewSelectableTextContent returns content from ordered styled spans
//
// A hidden span makes the complete content non-copyable so presentation state
// cannot implicitly expose hidden text through a copy callback.
func NewSelectableTextContent(spans []tui.TextSpan) SelectableTextContent {
	owned := make([]tui.TextSpan, len(spans))
	copyable := true
	for index, span := range spans {
		span.Text = celltext.NormalizeUTF8(span.Text)
		owned[index] = span
		copyable = copyable && !span.Style.Hidden
	}
	text := ""
	if len(owned) == 1 {
		text = owned[0].Text
	} else if len(owned) > 1 {
		var builder strings.Builder
		for _, span := range owned {
			builder.WriteString(span.Text)
		}
		text = builder.String()
	}
	return SelectableTextContent{inner: &selectableTextContentData{
		text: text, spans: owned, copyable: copyable,
	}}
}

// Text returns the semantic UTF-8 document text
func (c SelectableTextContent) Text() string {
	if c.inner == nil {
		return ""
	}
	return c.inner.text
}

// Spans returns a copy of the immutable styled runs in display order
func (c SelectableTextContent) Spans() []tui.TextSpan {
	if c.inner == nil {
		return nil
	}
	return append([]tui.TextSpan(nil), c.inner.spans...)
}

// IsCopyable reports whether this content may be supplied to a copy callback
func (c SelectableTextContent) IsCopyable() bool {
	return c.inner == nil || c.inner.copyable
}

// NormalizeState returns selection offsets aligned to this content's grapheme boundaries
func (c SelectableTextContent) NormalizeState(state SelectableTextState) SelectableTextState {
	return normalizeSelectableTextState(c.Text(), state)
}

func (c SelectableTextContent) spans() []tui.TextSpan {
	if c.inner == nil {
		return nil
	}
	return c.inner.spans
}

// SelectableTextState is an application-owned cursor and optional selection for SelectableText
type SelectableTextState struct {
	cursor          int
	selectionAnchor int
	hasSelection    bool
}

// NewSelectableTextState returns a collapsed selection at a UTF-8 byte offset
//
// The current content normalizes the offset when a widget is built.
func NewSelectableTextState(cursor int) SelectableTextState {
	return SelectableTextState{cursor: cursor, selectionAnchor: cursor}
}

// NewSelectableTextStateWithSelection returns a selection between two UTF-8 byte offsets
//
// The current content normalizes both offsets when a widget is built.
func NewSelectableTextStateWithSelection(cursor, anchor int) SelectableTextState {
	return SelectableTextState{
		cursor: cursor, selectionAnchor: anchor, hasSelection: cursor != anchor,
	}
}

// Cursor returns the cursor UTF-8 byte offset
func (s SelectableTextState) Cursor() int {
	return s.cursor
}

// SelectionAnchor returns the selection anchor when a non-empty selection exists
func (s SelectableTextState) SelectionAnchor() (int, bool) {
	return s.selectionAnchor, s.hasSelection
}

// Selection returns the ordered non-empty UTF-8 byte selection range
func (s SelectableTextState) Selection() (int, int, bool) {
	if !s.hasSelection {
		return 0, 0, false
	}
	return min(s.cursor, s.selectionAnchor), max(s.cursor, s.selectionAnchor), true
}

// Select returns state selecting from anchor to the current cursor
func (s SelectableTextState) Select(anchor int) SelectableTextState {
	s.selectionAnchor = anchor
	s.hasSelection = anchor != s.cursor
	return s
}

// ClearSelection returns state with the selection collapsed at the current cursor
func (s SelectableTextState) ClearSelection() SelectableTextState {
	s.selectionAnchor = s.cursor
	s.hasSelection = false
	return s
}

// TextCopyKind identifies the semantic source represented by a TextCopyRequest
type TextCopyKind uint8

const (
	// TextCopySelection represents the current non-empty selection
	TextCopySelection TextCopyKind = iota
	// TextCopyDocument represents the complete text document
	TextCopyDocument
)

// TextCopyRequest is an application-handled request to copy semantic text
type TextCopyRequest struct {
	// Source is the stable source Node ID
	Source tui.NodeID
	// Kind identifies a selection or complete document request
	Kind TextCopyKind
	// Text is owned semantic UTF-8 text
	Text string
	// Start is the inclusive UTF-8 byte offset in the original document
	Start int
	// End is the exclusive UTF-8 byte offset in the original document
	End int
}

// SelectableTextStyle contains visual styles used by SelectableText
type SelectableTextStyle struct {
	// Selection is merged over selected graphemes
	Selection vt.Style
	// Focused is merged over the widget while it owns focus
	Focused vt.Style
	// Disabled is merged over every span while the widget is disabled
	Disabled vt.Style
}

// DefaultSelectableTextStyle returns the standard selection, focus, and disabled styles
func DefaultSelectableTextStyle() SelectableTextStyle {
	return SelectableTextStyle{
		Selection: vt.Style{Reverse: true},
		Focused:   vt.Style{Underline: true},
		Disabled:  vt.Style{Dim: true},
	}
}

// SelectableText provides controlled keyboard selection and copy actions over one styled document
//
// Copy requests are emitted as application messages. This widget does not
// access an OS or terminal clipboard and does not perform pointer selection.
type SelectableText[Message any] struct {
	id       tui.NodeID
	content  SelectableTextContent
	state    SelectableTextState
	options  tui.ParagraphOptions
	enabled  bool
	style    SelectableTextStyle
	onChange func(SelectableTextState) Message
	onCopy   func(TextCopyRequest) Message
}

// NewSelectableText returns a selectable document using application-owned state
//
// A nil onChange function creates a disabled widget.
func NewSelectableText[Message any](
	id tui.NodeID,
	content SelectableTextContent,
	state SelectableTextState,
	onChange func(SelectableTextState) Message,
) SelectableText[Message] {
	state = content.NormalizeState(state)
	return SelectableText[Message]{
		id: id, content: content, state: state,
		options: tui.DefaultParagraphOptions(), enabled: onChange != nil,
		style: DefaultSelectableTextStyle(), onChange: onChange,
	}
}

// ParagraphOptions sets paragraph wrapping and alignment
func (s SelectableText[Message]) ParagraphOptions(options tui.ParagraphOptions) SelectableText[Message] {
	s.options = options
	return s
}

// Enabled sets whether this document can receive focus and selection actions
func (s SelectableText[Message]) Enabled(enabled bool) SelectableText[Message] {
	s.enabled = enabled && s.onChange != nil
	return s
}

// Style replaces selection, focus, and disabled styles
func (s SelectableText[Message]) Style(style SelectableTextStyle) SelectableText[Message] {
	s.style = style
	return s
}

// OnCopy sets the application callback for selection and document copy requests
func (s SelectableText[Message]) OnCopy(handler func(TextCopyRequest) Message) SelectableText[Message] {
	s.onCopy = handler
	return s
}

// ActionDescriptors returns the 19 ordered semantic action descriptors
func (s SelectableText[Message]) ActionDescriptors() []tui.ActionDescriptor {
	descriptors := selectableTextActionDescriptors(
		s.enabled, s.onCopy != nil, s.content.IsCopyable(), s.content, s.state,
	)
	return append([]tui.ActionDescriptor(nil), descriptors[:]...)
}

// Node builds the public semantic node for this selectable document
func (s SelectableText[Message]) Node() tui.Node[Message] {
	state := s.content.NormalizeState(s.state)
	descriptors := selectableTextActionDescriptors(
		s.enabled, s.onCopy != nil, s.content.IsCopyable(), s.content, state,
	)
	spans := selectableTextSpans(s.content, state, s.style, s.enabled)
	node := tui.Paragraph[Message](spans, s.options)
	if !s.enabled {
		actions := make([]tui.Action[Message], len(descriptors))
		for index, descriptor := range descriptors {
			actions[index] = tui.NewAction[Message](descriptor, nil)
		}
		return node.WithID(s.id).OnActions(s.id, actions)
	}

	context := &selectableTextActionContext[Message]{
		id: s.id, content: s.content, state: state,
		onChange: s.onChange, onCopy: s.onCopy,
	}
	actions := make([]tui.Action[Message], len(descriptors))
	for index, descriptor := range descriptors {
		action := selectableTextActions[index]
		actions[index] = tui.NewAction(descriptor, func(tui.ActionEvent) tui.EventResult[Message] {
			return selectableTextActionResult(action, context)
		})
	}
	return node.Focusable(s.id).
		WithFocusedStyle(s.style.Focused).
		OnActions(s.id, actions)
}

const selectableTextActionCount = 19

type selectableTextAction uint8

const (
	selectableTextCursorLeft selectableTextAction = iota
	selectableTextCursorRight
	selectableTextCursorWordLeft
	selectableTextCursorWordRight
	selectableTextCursorLineStart
	selectableTextCursorLineEnd
	selectableTextCursorDocumentStart
	selectableTextCursorDocumentEnd
	selectableTextSelectionExtendLeft
	selectableTextSelectionExtendRight
	selectableTextSelectionExtendWordLeft
	selectableTextSelectionExtendWordRight
	selectableTextSelectionExtendLineStart
	selectableTextSelectionExtendLineEnd
	selectableTextSelectionExtendDocumentStart
	selectableTextSelectionExtendDocumentEnd
	selectableTextSelectAll
	selectableTextCopySelection
	selectableTextCopyDocument
)

var selectableTextActions = [selectableTextActionCount]selectableTextAction{
	selectableTextCursorLeft,
	selectableTextCursorRight,
	selectableTextCursorWordLeft,
	selectableTextCursorWordRight,
	selectableTextCursorLineStart,
	selectableTextCursorLineEnd,
	selectableTextCursorDocumentStart,
	selectableTextCursorDocumentEnd,
	selectableTextSelectionExtendLeft,
	selectableTextSelectionExtendRight,
	selectableTextSelectionExtendWordLeft,
	selectableTextSelectionExtendWordRight,
	selectableTextSelectionExtendLineStart,
	selectableTextSelectionExtendLineEnd,
	selectableTextSelectionExtendDocumentStart,
	selectableTextSelectionExtendDocumentEnd,
	selectableTextSelectAll,
	selectableTextCopySelection,
	selectableTextCopyDocument,
}

var defaultSelectableTextActionDescriptors = [selectableTextActionCount]tui.ActionDescriptor{
	selectableTextActionDescriptor(
		tui.TextCursorLeftActionID, "Move cursor left",
		[]tui.KeyBinding{selectableTextKeyBinding(vt.KeyLeft, vt.Modifiers{})},
	),
	selectableTextActionDescriptor(
		tui.TextCursorRightActionID, "Move cursor right",
		[]tui.KeyBinding{selectableTextKeyBinding(vt.KeyRight, vt.Modifiers{})},
	),
	selectableTextActionDescriptor(
		tui.TextCursorWordLeftActionID, "Move to previous word",
		[]tui.KeyBinding{selectableTextKeyBinding(vt.KeyLeft, vt.Modifiers{Control: true})},
	),
	selectableTextActionDescriptor(
		tui.TextCursorWordRightActionID, "Move to next word",
		[]tui.KeyBinding{selectableTextKeyBinding(vt.KeyRight, vt.Modifiers{Control: true})},
	),
	selectableTextActionDescriptor(
		tui.TextCursorLineStartActionID, "Move to line start",
		[]tui.KeyBinding{selectableTextKeyBinding(vt.KeyHome, vt.Modifiers{})},
	),
	selectableTextActionDescriptor(
		tui.TextCursorLineEndActionID, "Move to line end",
		[]tui.KeyBinding{selectableTextKeyBinding(vt.KeyEnd, vt.Modifiers{})},
	),
	selectableTextActionDescriptor(
		tui.TextCursorDocumentStartActionID, "Move to document start",
		[]tui.KeyBinding{selectableTextKeyBinding(vt.KeyHome, vt.Modifiers{Control: true})},
	),
	selectableTextActionDescriptor(
		tui.TextCursorDocumentEndActionID, "Move to document end",
		[]tui.KeyBinding{selectableTextKeyBinding(vt.KeyEnd, vt.Modifiers{Control: true})},
	),
	selectableTextActionDescriptor(
		tui.TextSelectionExtendLeftActionID, "Extend selection left",
		[]tui.KeyBinding{selectableTextKeyBinding(vt.KeyLeft, vt.Modifiers{Shift: true})},
	),
	selectableTextActionDescriptor(
		tui.TextSelectionExtendRightActionID, "Extend selection right",
		[]tui.KeyBinding{selectableTextKeyBinding(vt.KeyRight, vt.Modifiers{Shift: true})},
	),
	selectableTextActionDescriptor(
		tui.TextSelectionExtendWordLeftActionID, "Extend selection to previous word",
		[]tui.KeyBinding{selectableTextKeyBinding(vt.KeyLeft, vt.Modifiers{Shift: true, Control: true})},
	),
	selectableTextActionDescriptor(
		tui.TextSelectionExtendWordRightActionID, "Extend selection to next word",
		[]tui.KeyBinding{selectableTextKeyBinding(vt.KeyRight, vt.Modifiers{Shift: true, Control: true})},
	),
	selectableTextActionDescriptor(
		tui.TextSelectionExtendLineStartActionID, "Extend selection to line start",
		[]tui.KeyBinding{selectableTextKeyBinding(vt.KeyHome, vt.Modifiers{Shift: true})},
	),
	selectableTextActionDescriptor(
		tui.TextSelectionExtendLineEndActionID, "Extend selection to line end",
		[]tui.KeyBinding{selectableTextKeyBinding(vt.KeyEnd, vt.Modifiers{Shift: true})},
	),
	selectableTextActionDescriptor(
		tui.TextSelectionExtendDocumentStartActionID, "Extend selection to document start",
		[]tui.KeyBinding{selectableTextKeyBinding(vt.KeyHome, vt.Modifiers{Shift: true, Control: true})},
	),
	selectableTextActionDescriptor(
		tui.TextSelectionExtendDocumentEndActionID, "Extend selection to document end",
		[]tui.KeyBinding{selectableTextKeyBinding(vt.KeyEnd, vt.Modifiers{Shift: true, Control: true})},
	),
	selectableTextActionDescriptor(
		tui.TextSelectAllActionID, "Select all",
		[]tui.KeyBinding{selectableTextCharacterBinding('a', vt.Modifiers{Control: true}, tui.RepeatAllow)},
	),
	selectableTextActionDescriptor(
		tui.TextCopySelectionActionID, "Copy selection",
		[]tui.KeyBinding{selectableTextCharacterBinding('c', vt.Modifiers{Control: true}, tui.RepeatInitialOnly)},
	),
	selectableTextActionDescriptor(
		tui.TextCopyDocumentActionID, "Copy document",
		[]tui.KeyBinding{selectableTextCharacterBinding('c', vt.Modifiers{Shift: true, Control: true}, tui.RepeatInitialOnly)},
	),
}

func selectableTextActionDescriptor(
	id tui.ActionID,
	label string,
	bindings []tui.KeyBinding,
) tui.ActionDescriptor {
	return tui.NewActionDescriptor(id, label, bindings)
}

func selectableTextKeyBinding(code vt.KeyCode, modifiers vt.Modifiers) tui.KeyBinding {
	return tui.NewKeyBinding(tui.NewKeyStroke(code, modifiers)).WithRepeatPolicy(tui.RepeatAllow)
}

func selectableTextCharacterBinding(
	character rune,
	modifiers vt.Modifiers,
	repeat tui.RepeatPolicy,
) tui.KeyBinding {
	return tui.NewKeyBinding(tui.NewCharacterKeyStroke(character, modifiers)).WithRepeatPolicy(repeat)
}

func selectableTextActionDescriptors(
	enabled, hasCopyHandler, copyable bool,
	content SelectableTextContent,
	state SelectableTextState,
) [selectableTextActionCount]tui.ActionDescriptor {
	state = content.NormalizeState(state)
	var descriptors [selectableTextActionCount]tui.ActionDescriptor
	for index, descriptor := range defaultSelectableTextActionDescriptors {
		available := enabled
		switch selectableTextActions[index] {
		case selectableTextCopySelection:
			_, _, selected := state.Selection()
			available = available && hasCopyHandler && copyable && selected
		case selectableTextCopyDocument:
			available = available && hasCopyHandler && copyable && content.Text() != ""
		}
		availability := tui.ActionDisabledPassThrough
		if available {
			availability = tui.ActionEnabled
		}
		descriptors[index] = descriptor.WithAvailability(availability)
	}
	return descriptors
}

type selectableTextActionContext[Message any] struct {
	id       tui.NodeID
	content  SelectableTextContent
	state    SelectableTextState
	onChange func(SelectableTextState) Message
	onCopy   func(TextCopyRequest) Message
}

func selectableTextActionResult[Message any](
	action selectableTextAction,
	context *selectableTextActionContext[Message],
) tui.EventResult[Message] {
	switch action {
	case selectableTextCopySelection:
		return selectableTextCopyResult(TextCopySelection, context)
	case selectableTextCopyDocument:
		return selectableTextCopyResult(TextCopyDocument, context)
	}
	next := selectableTextStateForAction(context.content, context.state, action)
	result := tui.ConsumeResult[Message]().Focus(context.id)
	if next == context.state {
		return result
	}
	return result.Emit(context.onChange(next))
}

func selectableTextCopyResult[Message any](
	kind TextCopyKind,
	context *selectableTextActionContext[Message],
) tui.EventResult[Message] {
	if context.onCopy == nil {
		return tui.IgnoreResult[Message]()
	}
	start, end := 0, len(context.content.Text())
	if kind == TextCopySelection {
		var selected bool
		start, end, selected = context.state.Selection()
		if !selected {
			return tui.IgnoreResult[Message]()
		}
	}
	request := TextCopyRequest{
		Source: context.id,
		Kind:   kind,
		Text:   strings.Clone(context.content.Text()[start:end]),
		Start:  start,
		End:    end,
	}
	return tui.ConsumeResult[Message]().Focus(context.id).Emit(context.onCopy(request))
}

func selectableTextStateForAction(
	content SelectableTextContent,
	state SelectableTextState,
	action selectableTextAction,
) SelectableTextState {
	state = content.NormalizeState(state)
	text := content.Text()
	switch action {
	case selectableTextCursorLeft:
		if start, _, selected := state.Selection(); selected {
			return NewSelectableTextState(start)
		}
		return moveSelectableText(state, previousSelectableTextCursor(text, state.cursor), false)
	case selectableTextCursorRight:
		if _, end, selected := state.Selection(); selected {
			return NewSelectableTextState(end)
		}
		return moveSelectableText(state, nextSelectableTextCursor(text, state.cursor), false)
	case selectableTextCursorWordLeft:
		return moveSelectableText(state, previousSelectableTextWord(text, state.cursor), false)
	case selectableTextCursorWordRight:
		return moveSelectableText(state, nextSelectableTextWord(text, state.cursor), false)
	case selectableTextCursorLineStart:
		start, _ := currentSelectableTextLine(text, state.cursor)
		return moveSelectableText(state, start, false)
	case selectableTextCursorLineEnd:
		_, end := currentSelectableTextLine(text, state.cursor)
		return moveSelectableText(state, end, false)
	case selectableTextCursorDocumentStart:
		return moveSelectableText(state, 0, false)
	case selectableTextCursorDocumentEnd:
		return moveSelectableText(state, len(text), false)
	case selectableTextSelectionExtendLeft:
		return moveSelectableText(state, previousSelectableTextCursor(text, state.cursor), true)
	case selectableTextSelectionExtendRight:
		return moveSelectableText(state, nextSelectableTextCursor(text, state.cursor), true)
	case selectableTextSelectionExtendWordLeft:
		return moveSelectableText(state, previousSelectableTextWord(text, state.cursor), true)
	case selectableTextSelectionExtendWordRight:
		return moveSelectableText(state, nextSelectableTextWord(text, state.cursor), true)
	case selectableTextSelectionExtendLineStart:
		start, _ := currentSelectableTextLine(text, state.cursor)
		return moveSelectableText(state, start, true)
	case selectableTextSelectionExtendLineEnd:
		_, end := currentSelectableTextLine(text, state.cursor)
		return moveSelectableText(state, end, true)
	case selectableTextSelectionExtendDocumentStart:
		return moveSelectableText(state, 0, true)
	case selectableTextSelectionExtendDocumentEnd:
		return moveSelectableText(state, len(text), true)
	case selectableTextSelectAll:
		return NewSelectableTextStateWithSelection(len(text), 0)
	default:
		return state
	}
}

func moveSelectableText(state SelectableTextState, target int, extend bool) SelectableTextState {
	anchor := state.cursor
	if state.hasSelection {
		anchor = state.selectionAnchor
	}
	if extend {
		return NewSelectableTextStateWithSelection(target, anchor)
	}
	return NewSelectableTextState(target)
}

func previousSelectableTextCursor(text string, cursor int) int {
	if previous, ok := celltext.PreviousGraphemeBoundary(text, cursor); ok {
		return previous
	}
	return 0
}

func nextSelectableTextCursor(text string, cursor int) int {
	if next, ok := celltext.NextGraphemeBoundary(text, cursor); ok {
		return next
	}
	return len(text)
}

func previousSelectableTextWord(text string, cursor int) int {
	wordStart := 0
	inWord := false
	iterator := celltext.IterateGraphemes(text[:cursor])
	for grapheme, ok := iterator.Next(); ok; grapheme, ok = iterator.Next() {
		if selectableTextWordSeparator(grapheme.Text) {
			inWord = false
		} else if !inWord {
			wordStart = grapheme.Start
			inWord = true
		}
	}
	return wordStart
}

func nextSelectableTextWord(text string, cursor int) int {
	sawWord := false
	sawSeparatorAfterWord := false
	iterator := celltext.IterateGraphemes(text[cursor:])
	for grapheme, ok := iterator.Next(); ok; grapheme, ok = iterator.Next() {
		separator := selectableTextWordSeparator(grapheme.Text)
		switch {
		case !separator && !sawWord:
			if grapheme.Start != 0 {
				return cursor + grapheme.Start
			}
			sawWord = true
		case separator && sawWord:
			sawSeparatorAfterWord = true
		case !separator && sawSeparatorAfterWord:
			return cursor + grapheme.Start
		}
	}
	return len(text)
}

func selectableTextWordSeparator(text string) bool {
	for _, character := range text {
		if !((character >= '\u0009' && character <= '\u000d') || character == ' ') {
			return false
		}
	}
	return true
}

func currentSelectableTextLine(text string, cursor int) (int, int) {
	start := 0
	iterator := celltext.IterateGraphemes(text)
	for grapheme, ok := iterator.Next(); ok; grapheme, ok = iterator.Next() {
		if grapheme.Text == "\r" || grapheme.Text == "\n" || grapheme.Text == "\r\n" {
			if cursor >= start && cursor <= grapheme.Start {
				return start, grapheme.Start
			}
			start = grapheme.End
		}
	}
	return start, len(text)
}

func normalizeSelectableTextState(text string, state SelectableTextState) SelectableTextState {
	requestedCursor := min(max(state.cursor, 0), len(text))
	requestedAnchor := min(max(state.selectionAnchor, 0), len(text))
	state.cursor = 0
	state.selectionAnchor = 0
	iterator := celltext.IterateGraphemes(text)
	for grapheme, ok := iterator.Next(); ok; grapheme, ok = iterator.Next() {
		if grapheme.End <= requestedCursor {
			state.cursor = grapheme.End
		}
		if grapheme.End <= requestedAnchor {
			state.selectionAnchor = grapheme.End
		}
		if grapheme.End > requestedCursor && grapheme.End > requestedAnchor {
			break
		}
	}
	if !state.hasSelection || state.cursor == state.selectionAnchor {
		state.selectionAnchor = state.cursor
		state.hasSelection = false
	}
	return state
}

func selectableTextSpans(
	content SelectableTextContent,
	state SelectableTextState,
	style SelectableTextStyle,
	enabled bool,
) []tui.TextSpan {
	selectionStart, selectionEnd, hasSelection := state.Selection()
	spans := content.spans()
	output := make([]tui.TextSpan, 0, len(spans)+2)
	offset := 0
	for _, span := range spans {
		start := offset
		end := start + len(span.Text)
		offset = end
		base := span.Style
		if !enabled {
			base = base.Merge(style.Disabled)
		}
		if !hasSelection || selectionStart >= end || selectionEnd <= start {
			output = append(output, span.WithStyle(base))
			continue
		}
		selectedStart := max(selectionStart, start) - start
		selectedEnd := min(selectionEnd, end) - start
		output = appendSelectableTextSpan(output, span.Text[:selectedStart], base)
		output = appendSelectableTextSpan(
			output,
			span.Text[selectedStart:selectedEnd],
			base.Merge(style.Selection),
		)
		output = appendSelectableTextSpan(output, span.Text[selectedEnd:], base)
	}
	return output
}

func appendSelectableTextSpan(output []tui.TextSpan, text string, style vt.Style) []tui.TextSpan {
	if text != "" {
		output = append(output, tui.NewTextSpan(text, style))
	}
	return output
}
