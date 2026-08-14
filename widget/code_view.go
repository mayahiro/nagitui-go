package widget

import (
	"fmt"
	"strings"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
	tui "github.com/mayahiro/nagitui-go"
)

// CodeViewState contains application-owned line selection and horizontal offset
//
// The zero value selects the first logical line with no horizontal offset
type CodeViewState struct {
	cursor           int
	anchor           int
	hasSelection     bool
	horizontalOffset uint32
}

// NewCodeViewState returns a collapsed logical-line selection
func NewCodeViewState(cursor int) CodeViewState {
	return CodeViewState{cursor: cursor, anchor: cursor}
}

// NewCodeViewStateWithSelection returns an inclusive logical-line selection
func NewCodeViewStateWithSelection(cursor, anchor int) CodeViewState {
	return CodeViewState{cursor: cursor, anchor: anchor, hasSelection: cursor != anchor}
}

// Cursor returns the selected logical line
func (s CodeViewState) Cursor() int { return s.cursor }

// SelectionAnchor returns the anchor when multiple logical lines are selected
func (s CodeViewState) SelectionAnchor() (int, bool) { return s.anchor, s.hasSelection }

// SelectedLines returns the inclusive selection as an ordered end-exclusive range
func (s CodeViewState) SelectedLines() (start, end int) {
	return min(s.cursor, s.anchor), max(s.cursor, s.anchor) + 1
}

// HorizontalOffset returns the no-wrap horizontal cell offset
func (s CodeViewState) HorizontalOffset() uint32 { return s.horizontalOffset }

// WithHorizontalOffset returns this state with a replacement cell offset
func (s CodeViewState) WithHorizontalOffset(value uint32) CodeViewState {
	s.horizontalOffset = value
	return s
}

// CodeCopyKind identifies the semantic source represented by a CodeCopyRequest
type CodeCopyKind uint8

const (
	// CodeCopySelection represents the selected complete logical lines
	CodeCopySelection CodeCopyKind = iota
	// CodeCopyDocument represents the complete code document
	CodeCopyDocument
)

// CodeCopyRequest independently owns semantic source text for application handling
type CodeCopyRequest struct {
	// Source is the stable CodeView root Node ID
	Source tui.NodeID
	// Kind identifies selected lines or the complete document
	Kind CodeCopyKind
	// Text is independently owned semantic UTF-8 source text
	Text string
	// LineStart is the inclusive logical-line index in the original document
	LineStart int
	// LineEnd is the exclusive logical-line index in the original document
	LineEnd int
	// ByteStart is the inclusive UTF-8 byte offset in the original document
	ByteStart int
	// ByteEnd is the exclusive UTF-8 byte offset in the original document
	ByteEnd int
}

// CodeViewStyle contains independent semantic style slots
type CodeViewStyle struct {
	// LineNumber is used for the sticky line-number gutter
	LineNumber vt.Style
	// Continuation is merged over the marker in a wrapped row
	Continuation vt.Style
	// Current is merged over the current logical line
	Current vt.Style
	// Selection is merged over every selected logical line
	Selection vt.Style
	// Focused is merged over the complete view while it owns focus
	Focused vt.Style
	// Disabled is merged over every span while the view is disabled
	Disabled vt.Style
}

// DefaultCodeViewStyle returns standard attribute-only semantic styles
func DefaultCodeViewStyle() CodeViewStyle {
	return CodeViewStyle{
		LineNumber: vt.Style{Dim: true}, Continuation: vt.Style{Dim: true},
		Selection: vt.Style{Reverse: true}, Focused: vt.Style{Underline: true},
		Disabled: vt.Style{Dim: true},
	}
}

// CodeView is a controlled bounded terminal view over one immutable CodeLayout
//
// Navigation and copy requests are returned to the application The view does
// not parse syntax, read files, or access a clipboard backend
type CodeView[Message any] struct {
	id             tui.NodeID
	layout         CodeLayout
	state          CodeViewState
	viewportHeight int
	horizontalStep uint32
	enabled        bool
	style          CodeViewStyle
	onChange       func(CodeViewState) Message
	onCopy         func(CodeCopyRequest) Message
}

// NewCodeView returns a view using application-owned controlled state
//
// A nil onChange function creates a disabled view
func NewCodeView[Message any](
	id tui.NodeID,
	layout CodeLayout,
	state CodeViewState,
	onChange func(CodeViewState) Message,
) CodeView[Message] {
	return CodeView[Message]{
		id: id, layout: layout, state: normalizeCodeViewState(layout, state),
		horizontalStep: 4, enabled: onChange != nil,
		style: DefaultCodeViewStyle(), onChange: onChange,
	}
}

// State returns the visually normalized controlled state
func (v CodeView[Message]) State() CodeViewState { return v.state }

// Enabled sets whether the view can receive focus and emit messages
func (v CodeView[Message]) Enabled(value bool) CodeView[Message] {
	v.enabled = value && v.onChange != nil
	return v
}

// Viewport limits constructed rows to a deterministic window following selection
//
// A non-positive height constructs every visual row
func (v CodeView[Message]) Viewport(height int) CodeView[Message] {
	v.viewportHeight = max(height, 0)
	return v
}

// HorizontalStep sets the positive no-wrap horizontal navigation step in cells
//
// Zero uses one cell
func (v CodeView[Message]) HorizontalStep(value uint32) CodeView[Message] {
	if value == 0 {
		value = 1
	}
	v.horizontalStep = value
	return v
}

// Style replaces every semantic style slot
func (v CodeView[Message]) Style(value CodeViewStyle) CodeView[Message] {
	v.style = value
	return v
}

// OnCopy sets the application callback for semantic source copy requests
func (v CodeView[Message]) OnCopy(handler func(CodeCopyRequest) Message) CodeView[Message] {
	v.onCopy = handler
	return v
}

// ActionDescriptors returns the thirteen ordered semantic actions
//
// The order is previous, next, first, last, four selection-extension actions,
// horizontal previous and next, select all, copy selection, and copy document
func (v CodeView[Message]) ActionDescriptors() []tui.ActionDescriptor {
	descriptors := codeViewActionDescriptors(v.enabled, v.onCopy != nil, v.layout, v.state)
	return append([]tui.ActionDescriptor(nil), descriptors[:]...)
}

// Node builds the public semantic node for this view
func (v CodeView[Message]) Node() tui.Node[Message] { return buildCodeViewNode(v) }

type codeViewAction uint8

const (
	codeViewPrevious codeViewAction = iota
	codeViewNext
	codeViewFirst
	codeViewLast
	codeViewExtendPrevious
	codeViewExtendNext
	codeViewExtendFirst
	codeViewExtendLast
	codeViewHorizontalPrevious
	codeViewHorizontalNext
	codeViewSelectAll
	codeViewCopySelection
	codeViewCopyDocument
	codeViewActionCount
)

var codeViewActions = [codeViewActionCount]codeViewAction{
	codeViewPrevious, codeViewNext, codeViewFirst, codeViewLast,
	codeViewExtendPrevious, codeViewExtendNext, codeViewExtendFirst, codeViewExtendLast,
	codeViewHorizontalPrevious, codeViewHorizontalNext, codeViewSelectAll,
	codeViewCopySelection, codeViewCopyDocument,
}

var defaultCodeViewActionDescriptors = [codeViewActionCount]tui.ActionDescriptor{
	repeatableCodeViewDescriptor(SelectionPreviousActionID, "Previous line", vt.KeyUp, vt.Modifiers{}),
	repeatableCodeViewDescriptor(SelectionNextActionID, "Next line", vt.KeyDown, vt.Modifiers{}),
	repeatableCodeViewDescriptor(SelectionFirstActionID, "First line", vt.KeyHome, vt.Modifiers{Control: true}),
	repeatableCodeViewDescriptor(SelectionLastActionID, "Last line", vt.KeyEnd, vt.Modifiers{Control: true}),
	repeatableCodeViewDescriptor(SelectionExtendPreviousActionID, "Extend to previous line", vt.KeyUp, vt.Modifiers{Shift: true}),
	repeatableCodeViewDescriptor(SelectionExtendNextActionID, "Extend to next line", vt.KeyDown, vt.Modifiers{Shift: true}),
	repeatableCodeViewDescriptor(SelectionExtendFirstActionID, "Extend to first line", vt.KeyHome, vt.Modifiers{Control: true, Shift: true}),
	repeatableCodeViewDescriptor(SelectionExtendLastActionID, "Extend to last line", vt.KeyEnd, vt.Modifiers{Control: true, Shift: true}),
	repeatableCodeViewDescriptor(HorizontalScrollPreviousActionID, "Scroll left", vt.KeyLeft, vt.Modifiers{}),
	repeatableCodeViewDescriptor(HorizontalScrollNextActionID, "Scroll right", vt.KeyRight, vt.Modifiers{}),
	tui.NewActionDescriptor(
		tui.TextSelectAllActionID, "Select all lines",
		[]tui.KeyBinding{tui.NewKeyBinding(tui.NewCharacterKeyStroke('a', vt.Modifiers{Control: true})).WithRepeatPolicy(tui.RepeatAllow)},
	),
	tui.NewActionDescriptor(
		tui.TextCopySelectionActionID, "Copy selected lines",
		[]tui.KeyBinding{tui.NewKeyBinding(tui.NewCharacterKeyStroke('c', vt.Modifiers{Control: true}))},
	),
	tui.NewActionDescriptor(
		tui.TextCopyDocumentActionID, "Copy document",
		[]tui.KeyBinding{tui.NewKeyBinding(tui.NewCharacterKeyStroke('c', vt.Modifiers{Control: true, Shift: true}))},
	),
}

func repeatableCodeViewDescriptor(
	id tui.ActionID,
	label string,
	code vt.KeyCode,
	modifiers vt.Modifiers,
) tui.ActionDescriptor {
	return tui.NewActionDescriptor(
		id, label,
		[]tui.KeyBinding{tui.NewKeyBinding(tui.NewKeyStroke(code, modifiers)).WithRepeatPolicy(tui.RepeatAllow)},
	)
}

func codeViewActionDescriptors(
	enabled, hasCopyHandler bool,
	layout CodeLayout,
	state CodeViewState,
) [codeViewActionCount]tui.ActionDescriptor {
	lines := layout.Document().LineCount()
	state = normalizeCodeViewState(layout, state)
	var descriptors [codeViewActionCount]tui.ActionDescriptor
	for index, action := range codeViewActions {
		available := enabled && lines > 0
		if available {
			switch action {
			case codeViewHorizontalPrevious, codeViewHorizontalNext:
				available = !layout.Options().Wraps() && layout.MaximumRowWidth() > layout.CodeWidth()
			case codeViewCopySelection:
				start, end := state.SelectedLines()
				available = hasCopyHandler && layout.Document().IsRangeCopyable(start, end)
			case codeViewCopyDocument:
				available = hasCopyHandler && layout.Document().IsRangeCopyable(0, lines)
			}
		}
		availability := tui.ActionDisabledPassThrough
		if available {
			availability = tui.ActionEnabled
		}
		descriptors[index] = defaultCodeViewActionDescriptors[index].WithAvailability(availability)
	}
	return descriptors
}

type codeViewActionContext[Message any] struct {
	id             tui.NodeID
	layout         CodeLayout
	state          CodeViewState
	horizontalStep uint32
	onChange       func(CodeViewState) Message
	onCopy         func(CodeCopyRequest) Message
}

func buildCodeViewNode[Message any](view CodeView[Message]) tui.Node[Message] {
	state := normalizeCodeViewState(view.layout, view.state)
	descriptors := codeViewActionDescriptors(view.enabled, view.onCopy != nil, view.layout, state)
	context := &codeViewActionContext[Message]{
		id: view.id, layout: view.layout, state: state, horizontalStep: view.horizontalStep,
		onChange: view.onChange, onCopy: view.onCopy,
	}
	window := codeViewVisibleRowWindow(view.layout, state.cursor, view.viewportHeight)
	selectedStart, selectedEnd := state.SelectedLines()
	rows := make([]tui.Node[Message], 0, window.end-window.start)
	for rowIndex := window.start; rowIndex < window.end; rowIndex++ {
		record, _ := view.layout.row(rowIndex)
		spans := codeViewRowSpans(
			view.layout, record, state.horizontalOffset,
			record.line >= selectedStart && record.line < selectedEnd,
			record.line == state.cursor, view.enabled, view.style,
		)
		if len(spans) == 0 {
			spans = []tui.TextSpan{tui.NewTextSpan("", vt.Style{})}
		}
		rowID := codeViewRowID(view.id, rowIndex)
		row := tui.Paragraph[Message](spans, tui.ParagraphOptions{
			Wrap: tui.WrapNone, Alignment: tui.AlignStart,
		}).WithID(rowID)
		if view.enabled {
			line := record.line
			row = row.OnEvent(rowID, func(event vt.Event) tui.EventResult[Message] {
				return codeViewPointerResult(event, line, context)
			})
		}
		rows = append(rows, row)
	}
	root := tui.Column(rows...)
	if view.viewportHeight > 0 {
		height := uint64(view.viewportHeight)
		if height > uint64(^uint32(0)) {
			height = uint64(^uint32(0))
		}
		root = root.WithLength(tui.Fixed(uint32(height)))
	}
	actions := make([]tui.Action[Message], len(descriptors))
	for index, descriptor := range descriptors {
		action := codeViewAction(index)
		actions[index] = tui.NewAction(descriptor, func(tui.ActionEvent) tui.EventResult[Message] {
			return codeViewActionResult(action, context)
		})
	}
	if !view.enabled {
		return root.WithID(view.id).OnActions(view.id, actions)
	}
	return root.Focusable(view.id).
		WithFocusedStyle(view.style.Focused).
		OnActions(view.id, actions).
		OnEvent(view.id, func(event vt.Event) tui.EventResult[Message] {
			selectionCopy, documentCopy := codeViewCopyActionsEnabled(
				context.layout, context.state, context.onCopy != nil,
			)
			if isBlockedCodeViewCopyRepeat(event, selectionCopy, documentCopy) {
				return tui.ConsumeResult[Message]()
			}
			return tui.IgnoreResult[Message]()
		})
}

func codeViewActionResult[Message any](
	action codeViewAction,
	context *codeViewActionContext[Message],
) tui.EventResult[Message] {
	if action == codeViewCopySelection {
		return codeViewCopyResult(CodeCopySelection, context)
	}
	if action == codeViewCopyDocument {
		return codeViewCopyResult(CodeCopyDocument, context)
	}
	next := codeViewStateForAction(context.layout, context.state, action, context.horizontalStep)
	return emitCodeViewChange(tui.ConsumeResult[Message]().Focus(context.id), next, context)
}

func codeViewCopyResult[Message any](
	kind CodeCopyKind,
	context *codeViewActionContext[Message],
) tui.EventResult[Message] {
	if context.onCopy == nil {
		return tui.IgnoreResult[Message]()
	}
	request, ok := codeViewCopyRequest(kind, context)
	if !ok {
		return tui.IgnoreResult[Message]()
	}
	return tui.ConsumeResult[Message]().Focus(context.id).Emit(context.onCopy(request))
}

func codeViewCopyRequest[Message any](
	kind CodeCopyKind,
	context *codeViewActionContext[Message],
) (CodeCopyRequest, bool) {
	document := context.layout.Document()
	lineStart, lineEnd := 0, document.LineCount()
	if kind == CodeCopySelection {
		lineStart, lineEnd = context.state.SelectedLines()
	}
	if !document.IsRangeCopyable(lineStart, lineEnd) {
		return CodeCopyRequest{}, false
	}
	byteStart, byteEnd, ok := document.ByteRangeForLines(lineStart, lineEnd)
	if !ok {
		return CodeCopyRequest{}, false
	}
	return CodeCopyRequest{
		Source: context.id, Kind: kind, Text: strings.Clone(document.Text()[byteStart:byteEnd]),
		LineStart: lineStart, LineEnd: lineEnd, ByteStart: byteStart, ByteEnd: byteEnd,
	}, true
}

func codeViewPointerResult[Message any](
	event vt.Event,
	line int,
	context *codeViewActionContext[Message],
) tui.EventResult[Message] {
	if !isPointerActivationEvent(event) {
		return tui.IgnoreResult[Message]()
	}
	var next CodeViewState
	if event.Mouse.Modifiers.Shift {
		anchor, selected := context.state.SelectionAnchor()
		if !selected {
			anchor = context.state.Cursor()
		}
		next = NewCodeViewStateWithSelection(line, anchor)
	} else {
		next = NewCodeViewState(line)
	}
	next = next.WithHorizontalOffset(context.state.HorizontalOffset())
	return emitCodeViewChange(tui.ConsumeResult[Message]().Focus(context.id), next, context)
}

func emitCodeViewChange[Message any](
	result tui.EventResult[Message],
	next CodeViewState,
	context *codeViewActionContext[Message],
) tui.EventResult[Message] {
	next = normalizeCodeViewState(context.layout, next)
	if next == context.state {
		return result
	}
	return result.Emit(context.onChange(next))
}

func codeViewStateForAction(
	layout CodeLayout,
	state CodeViewState,
	action codeViewAction,
	horizontalStep uint32,
) CodeViewState {
	state = normalizeCodeViewState(layout, state)
	lines := layout.Document().LineCount()
	if lines == 0 {
		return state
	}
	last := lines - 1
	moveTo := func(target int) CodeViewState {
		return NewCodeViewState(target).WithHorizontalOffset(state.horizontalOffset)
	}
	extendTo := func(target int) CodeViewState {
		anchor, selected := state.SelectionAnchor()
		if !selected {
			anchor = state.cursor
		}
		return NewCodeViewStateWithSelection(target, anchor).WithHorizontalOffset(state.horizontalOffset)
	}
	switch action {
	case codeViewPrevious:
		return moveTo(max(state.cursor-1, 0))
	case codeViewNext:
		return moveTo(min(state.cursor+1, last))
	case codeViewFirst:
		return moveTo(0)
	case codeViewLast:
		return moveTo(last)
	case codeViewExtendPrevious:
		return extendTo(max(state.cursor-1, 0))
	case codeViewExtendNext:
		return extendTo(min(state.cursor+1, last))
	case codeViewExtendFirst:
		return extendTo(0)
	case codeViewExtendLast:
		return extendTo(last)
	case codeViewHorizontalPrevious:
		if state.horizontalOffset < horizontalStep {
			state.horizontalOffset = 0
		} else {
			state.horizontalOffset -= horizontalStep
		}
		return state
	case codeViewHorizontalNext:
		state.horizontalOffset = saturatingAdd32(state.horizontalOffset, horizontalStep)
		return state
	case codeViewSelectAll:
		return NewCodeViewStateWithSelection(last, 0).WithHorizontalOffset(state.horizontalOffset)
	default:
		return state
	}
}

func normalizeCodeViewState(layout CodeLayout, state CodeViewState) CodeViewState {
	last := max(layout.Document().LineCount()-1, 0)
	cursor, anchor := min(max(state.cursor, 0), last), min(max(state.anchor, 0), last)
	var next CodeViewState
	if state.hasSelection {
		next = NewCodeViewStateWithSelection(cursor, anchor)
	} else {
		next = NewCodeViewState(cursor)
	}
	maximumOffset := uint32(0)
	if !layout.Options().Wraps() && layout.MaximumRowWidth() > layout.CodeWidth() {
		maximumOffset = layout.MaximumRowWidth() - layout.CodeWidth()
	}
	next.horizontalOffset = min(state.horizontalOffset, maximumOffset)
	return next
}

func codeViewVisibleRowWindow(layout CodeLayout, selectedLine, height int) codeIndexRange {
	if height <= 0 || height >= layout.VisualRowCount() {
		return codeIndexRange{end: layout.VisualRowCount()}
	}
	selectedRow := layout.rowsForLine(selectedLine).start
	start := max(selectedRow-height/2, 0)
	start = min(start, layout.VisualRowCount()-height)
	return codeIndexRange{start: start, end: start + height}
}

func codeViewRowSpans(
	layout CodeLayout,
	row codeVisualRow,
	horizontalOffset uint32,
	selected, current, enabled bool,
	style CodeViewStyle,
) []tui.TextSpan {
	spans := make([]tui.TextSpan, 0, len(row.spans)+1)
	if layout.GutterWidth() > 0 {
		var content string
		gutterStyle := style.LineNumber
		if row.continuation {
			content = fmt.Sprintf("%*s ", int(layout.GutterWidth())-1, ">")
			gutterStyle = gutterStyle.Merge(style.Continuation)
		} else {
			content = fmt.Sprintf("%*d | ", int(layout.GutterWidth())-3, row.line+1)
		}
		spans = append(spans, tui.NewTextSpan(
			content, codeViewRowOverlay(gutterStyle, selected, current, enabled, style),
		))
	}
	visible := sliceCodeSpansByCells(
		row.spans, row.checkpoints, horizontalOffset, layout.CodeWidth(), layout.Options().WidthProfile(),
	)
	for _, span := range visible {
		spans = append(spans, tui.NewTextSpan(
			span.Text, codeViewRowOverlay(span.Style, selected, current, enabled, style),
		))
	}
	return spans
}

func codeViewRowOverlay(
	base vt.Style,
	selected, current, enabled bool,
	style CodeViewStyle,
) vt.Style {
	result := base
	if current {
		result = result.Merge(style.Current)
	}
	if selected {
		result = result.Merge(style.Selection)
	}
	if !enabled {
		result = result.Merge(style.Disabled)
	}
	return result
}

func sliceCodeSpansByCells(
	spans []tui.TextSpan,
	checkpoints []codeRowCheckpoint,
	offset, width uint32,
	profile celltext.WidthProfile,
) []tui.TextSpan {
	end := saturatingAdd32(offset, width)
	cell, firstSpan, firstByte := uint32(0), 0, 0
	low, high := 0, len(checkpoints)
	for low < high {
		middle := low + (high-low)/2
		if checkpoints[middle].cell <= offset {
			low = middle + 1
		} else {
			high = middle
		}
	}
	if low > 0 {
		checkpoint := checkpoints[low-1]
		cell, firstSpan, firstByte = checkpoint.cell, checkpoint.span, checkpoint.byte
	}
	builder := styledCodeRowBuilder{}
	for spanIndex := firstSpan; spanIndex < len(spans); spanIndex++ {
		span := spans[spanIndex]
		text := span.Text
		if spanIndex == firstSpan {
			text = text[firstByte:]
		}
		iterator := celltext.IterateGraphemes(text)
		for grapheme, ok := iterator.Next(); ok; grapheme, ok = iterator.Next() {
			graphemeWidth := uint32(celltext.GraphemeWidth(grapheme.Text, profile))
			next := saturatingAdd32(cell, graphemeWidth)
			visible := cell >= offset && next <= end && (graphemeWidth > 0 || cell < end)
			if visible {
				builder.push(grapheme.Text, span.Style)
			}
			cell = next
			if cell >= end {
				break
			}
		}
		if cell >= end {
			break
		}
	}
	output, _ := builder.finish()
	return output
}

func codeViewRowID(root tui.NodeID, row int) tui.NodeID {
	return tui.NewNodeID(fmt.Sprintf("%s:visual-row:%d", root, row))
}

func codeViewCopyActionsEnabled(
	layout CodeLayout,
	state CodeViewState,
	hasCopyHandler bool,
) (selection, document bool) {
	source := layout.Document()
	start, end := state.SelectedLines()
	return hasCopyHandler && source.IsRangeCopyable(start, end),
		hasCopyHandler && source.IsRangeCopyable(0, source.LineCount())
}

func isBlockedCodeViewCopyRepeat(
	event vt.Event,
	selectionCopyEnabled, documentCopyEnabled bool,
) bool {
	if event.Kind != vt.EventKey || event.Key.Action != vt.KeyRepeat ||
		event.Key.Code != vt.KeyCharacter || event.Key.Character != 'c' {
		return false
	}
	return selectionCopyEnabled && event.Key.Modifiers == (vt.Modifiers{Control: true}) ||
		documentCopyEnabled && event.Key.Modifiers == (vt.Modifiers{Control: true, Shift: true})
}
