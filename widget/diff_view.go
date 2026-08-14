package widget

import (
	"fmt"

	"github.com/mayahiro/nagi-go/vt"
	tui "github.com/mayahiro/nagitui-go"
)

// DiffViewState is the CodeViewState representation shared by line views
type DiffViewState = CodeViewState

// NewDiffViewState returns one collapsed logical-line selection
func NewDiffViewState(cursor int) DiffViewState { return NewCodeViewState(cursor) }

// NewDiffViewStateWithSelection returns one inclusive line selection
func NewDiffViewStateWithSelection(cursor, anchor int) DiffViewState {
	return NewCodeViewStateWithSelection(cursor, anchor)
}

// DiffCopyKind identifies one semantic unified source copy range
type DiffCopyKind uint8

const (
	// DiffCopySelection represents selected complete logical lines
	DiffCopySelection DiffCopyKind = iota
	// DiffCopyDocument represents the complete diff document
	DiffCopyDocument
)

// DiffCopyRequest is one independently owned unified-text copy request
type DiffCopyRequest struct {
	// Source is the stable DiffView root Node ID
	Source tui.NodeID
	// Kind identifies selected lines or the complete document
	Kind DiffCopyKind
	// Text is independently owned unified UTF-8 source text
	Text string
	// LineStart is the inclusive logical-line index in the original document
	LineStart int
	// LineEnd is the exclusive logical-line index in the original document
	LineEnd int
	// ByteStart is the inclusive conceptual UTF-8 byte offset
	ByteStart int
	// ByteEnd is the exclusive conceptual UTF-8 byte offset
	ByteEnd int
}

// DiffViewStyle contains independent semantic style slots
type DiffViewStyle struct {
	// Metadata is the base semantic style for metadata content and gutter
	Metadata vt.Style
	// Hunk is the base semantic style for hunk content and gutter
	Hunk vt.Style
	// Context is the base semantic style for context content and gutter
	Context vt.Style
	// Addition is the base semantic style for addition content and gutter
	Addition vt.Style
	// Deletion is the base semantic style for deletion content and gutter
	Deletion vt.Style
	// LineNumber is merged over line-kind styles for old and new numbers
	LineNumber vt.Style
	// Marker is merged over line-kind styles for the marker and following space
	Marker vt.Style
	// Continuation is merged over wrapped continuation markers
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

// DefaultDiffViewStyle returns standard semantic diff styles
func DefaultDiffViewStyle() DiffViewStyle {
	return DiffViewStyle{
		Metadata:   vt.Style{Dim: true},
		Hunk:       vt.Style{Foreground: vt.IndexedColor(6), Bold: true},
		Addition:   vt.Style{Foreground: vt.IndexedColor(2)},
		Deletion:   vt.Style{Foreground: vt.IndexedColor(1)},
		LineNumber: vt.Style{Dim: true}, Marker: vt.Style{Bold: true},
		Continuation: vt.Style{Dim: true}, Selection: vt.Style{Reverse: true},
		Focused: vt.Style{Underline: true}, Disabled: vt.Style{Dim: true},
	}
}

// DiffView is a controlled bounded unified terminal view over one DiffLayout
//
// Navigation and copy requests are returned to the application. The view does
// not parse diffs, read repositories, apply patches, or access a clipboard
type DiffView[Message any] struct {
	id             tui.NodeID
	layout         DiffLayout
	state          DiffViewState
	viewportHeight int
	horizontalStep uint32
	enabled        bool
	style          DiffViewStyle
	onChange       func(DiffViewState) Message
	onCopy         func(DiffCopyRequest) Message
}

// NewDiffView returns a view using application-owned controlled state
//
// A nil onChange function creates a disabled view
func NewDiffView[Message any](
	id tui.NodeID,
	layout DiffLayout,
	state DiffViewState,
	onChange func(DiffViewState) Message,
) DiffView[Message] {
	return DiffView[Message]{
		id: id, layout: layout,
		state:          normalizeCodeViewState(layout.codeLayout(), state),
		horizontalStep: 4, enabled: onChange != nil,
		style: DefaultDiffViewStyle(), onChange: onChange,
	}
}

// State returns the visually normalized controlled state
func (v DiffView[Message]) State() DiffViewState { return v.state }

// Enabled sets whether the view can receive focus and emit messages
func (v DiffView[Message]) Enabled(value bool) DiffView[Message] {
	v.enabled = value && v.onChange != nil
	return v
}

// Viewport limits constructed rows to a deterministic selection window
//
// A non-positive height constructs every visual row
func (v DiffView[Message]) Viewport(height int) DiffView[Message] {
	v.viewportHeight = max(height, 0)
	return v
}

// HorizontalStep sets the positive no-wrap movement step in cells
//
// Zero uses one cell
func (v DiffView[Message]) HorizontalStep(value uint32) DiffView[Message] {
	if value == 0 {
		value = 1
	}
	v.horizontalStep = value
	return v
}

// Style replaces every semantic style slot
func (v DiffView[Message]) Style(value DiffViewStyle) DiffView[Message] {
	v.style = value
	return v
}

// OnCopy sets the application callback for unified source copy requests
func (v DiffView[Message]) OnCopy(handler func(DiffCopyRequest) Message) DiffView[Message] {
	v.onCopy = handler
	return v
}

// ActionDescriptors returns the thirteen ordered semantic actions
//
// The order matches CodeView movement, extension, horizontal movement,
// select-all, copy-selection, and copy-document actions
func (v DiffView[Message]) ActionDescriptors() []tui.ActionDescriptor {
	descriptors := diffViewActionDescriptors(v.enabled, v.onCopy != nil, v.layout, v.state)
	return append([]tui.ActionDescriptor(nil), descriptors[:]...)
}

// Node builds the public semantic node for this view
func (v DiffView[Message]) Node() tui.Node[Message] { return buildDiffViewNode(v) }

func diffViewActionDescriptors(
	enabled, hasCopyHandler bool,
	layout DiffLayout,
	state DiffViewState,
) [codeViewActionCount]tui.ActionDescriptor {
	document := layout.Document()
	lines := document.LineCount()
	state = normalizeCodeViewState(layout.codeLayout(), state)
	var descriptors [codeViewActionCount]tui.ActionDescriptor
	for index, action := range codeViewActions {
		available := enabled && lines > 0
		if available {
			switch action {
			case codeViewHorizontalPrevious, codeViewHorizontalNext:
				available = !layout.Options().Wraps() && layout.MaximumRowWidth() > layout.CodeWidth()
			case codeViewCopySelection:
				start, end := state.SelectedLines()
				available = hasCopyHandler && document.IsRangeCopyable(start, end)
			case codeViewCopyDocument:
				available = hasCopyHandler && document.IsRangeCopyable(0, lines)
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

type diffViewActionContext[Message any] struct {
	id             tui.NodeID
	layout         DiffLayout
	state          DiffViewState
	horizontalStep uint32
	onChange       func(DiffViewState) Message
	onCopy         func(DiffCopyRequest) Message
}

func buildDiffViewNode[Message any](view DiffView[Message]) tui.Node[Message] {
	state := normalizeCodeViewState(view.layout.codeLayout(), view.state)
	descriptors := diffViewActionDescriptors(view.enabled, view.onCopy != nil, view.layout, state)
	context := &diffViewActionContext[Message]{
		id: view.id, layout: view.layout, state: state, horizontalStep: view.horizontalStep,
		onChange: view.onChange, onCopy: view.onCopy,
	}
	window := codeViewVisibleRowWindow(view.layout.codeLayout(), state.Cursor(), view.viewportHeight)
	selectedStart, selectedEnd := state.SelectedLines()
	rows := make([]tui.Node[Message], 0, window.end-window.start)
	for rowIndex := window.start; rowIndex < window.end; rowIndex++ {
		record, _ := view.layout.codeLayout().row(rowIndex)
		line, _ := view.layout.Document().Line(record.line)
		oldLine, hasOld := line.OldLine()
		newLine, hasNew := line.NewLine()
		spans := diffViewRowSpans(
			view.layout, record, line.Kind(), oldLine, hasOld, newLine, hasNew,
			state.HorizontalOffset(), record.line >= selectedStart && record.line < selectedEnd,
			record.line == state.Cursor(), view.enabled, view.style,
		)
		if len(spans) == 0 {
			spans = []tui.TextSpan{tui.NewTextSpan("", vt.Style{})}
		}
		rowID := diffViewRowID(view.id, rowIndex)
		row := tui.Paragraph[Message](spans, tui.ParagraphOptions{
			Wrap: tui.WrapNone, Alignment: tui.AlignStart,
		}).WithID(rowID)
		if view.enabled {
			lineIndex := record.line
			row = row.OnEvent(rowID, func(event vt.Event) tui.EventResult[Message] {
				return diffViewPointerResult(event, lineIndex, context)
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
			return diffViewActionResult(action, context)
		})
	}
	if !view.enabled {
		return root.WithID(view.id).OnActions(view.id, actions)
	}
	return root.Focusable(view.id).
		WithFocusedStyle(view.style.Focused).
		OnActions(view.id, actions).
		OnEvent(view.id, func(event vt.Event) tui.EventResult[Message] {
			selectionCopy, documentCopy := diffCopyActionsEnabled(
				context.layout.Document(), context.state, context.onCopy != nil,
			)
			if isBlockedCodeViewCopyRepeat(event, selectionCopy, documentCopy) {
				return tui.ConsumeResult[Message]()
			}
			return tui.IgnoreResult[Message]()
		})
}

func diffViewActionResult[Message any](
	action codeViewAction,
	context *diffViewActionContext[Message],
) tui.EventResult[Message] {
	if action == codeViewCopySelection {
		return diffViewCopyResult(DiffCopySelection, context)
	}
	if action == codeViewCopyDocument {
		return diffViewCopyResult(DiffCopyDocument, context)
	}
	next := codeViewStateForAction(
		context.layout.codeLayout(), context.state, action, context.horizontalStep,
	)
	return emitDiffViewChange(tui.ConsumeResult[Message]().Focus(context.id), next, context)
}

func diffViewCopyResult[Message any](
	kind DiffCopyKind,
	context *diffViewActionContext[Message],
) tui.EventResult[Message] {
	if context.onCopy == nil {
		return tui.IgnoreResult[Message]()
	}
	request, ok := diffViewCopyRequest(kind, context)
	if !ok {
		return tui.IgnoreResult[Message]()
	}
	return tui.ConsumeResult[Message]().Focus(context.id).Emit(context.onCopy(request))
}

func diffViewCopyRequest[Message any](
	kind DiffCopyKind,
	context *diffViewActionContext[Message],
) (DiffCopyRequest, bool) {
	document := context.layout.Document()
	lineStart, lineEnd := 0, document.LineCount()
	if kind == DiffCopySelection {
		lineStart, lineEnd = context.state.SelectedLines()
	}
	byteStart, byteEnd, ok := document.ByteRangeForLines(lineStart, lineEnd)
	if !ok {
		return DiffCopyRequest{}, false
	}
	text, ok := document.CopyTextForLines(lineStart, lineEnd)
	if !ok {
		return DiffCopyRequest{}, false
	}
	return DiffCopyRequest{
		Source: context.id, Kind: kind, Text: text,
		LineStart: lineStart, LineEnd: lineEnd, ByteStart: byteStart, ByteEnd: byteEnd,
	}, true
}

func diffViewPointerResult[Message any](
	event vt.Event,
	line int,
	context *diffViewActionContext[Message],
) tui.EventResult[Message] {
	if !isPointerActivationEvent(event) {
		return tui.IgnoreResult[Message]()
	}
	var next DiffViewState
	if event.Mouse.Modifiers.Shift {
		anchor, selected := context.state.SelectionAnchor()
		if !selected {
			anchor = context.state.Cursor()
		}
		next = NewCodeViewStateWithSelection(line, anchor).
			WithHorizontalOffset(context.state.HorizontalOffset())
	} else {
		next = NewCodeViewState(line).WithHorizontalOffset(context.state.HorizontalOffset())
	}
	return emitDiffViewChange(tui.ConsumeResult[Message]().Focus(context.id), next, context)
}

func emitDiffViewChange[Message any](
	result tui.EventResult[Message],
	next DiffViewState,
	context *diffViewActionContext[Message],
) tui.EventResult[Message] {
	next = normalizeCodeViewState(context.layout.codeLayout(), next)
	if next == context.state || context.onChange == nil {
		return result
	}
	return result.Emit(context.onChange(next))
}

func diffViewRowSpans(
	layout DiffLayout,
	row codeVisualRow,
	kind DiffLineKind,
	oldLine uint64,
	hasOld bool,
	newLine uint64,
	hasNew bool,
	horizontalOffset uint32,
	selected, current, enabled bool,
	style DiffViewStyle,
) []tui.TextSpan {
	spans := make([]tui.TextSpan, 0, len(row.spans)+2)
	if layout.GutterWidth() > 0 {
		if layout.OldNumberWidth() > 0 {
			oldNumber := diffLineNumber(oldLine, hasOld, layout.OldNumberWidth(), row.continuation)
			newNumber := diffLineNumber(newLine, hasNew, layout.NewNumberWidth(), row.continuation)
			spans = append(spans, tui.NewTextSpan(
				fmt.Sprintf("%s %s ", oldNumber, newNumber),
				diffRowOverlay(
					diffKindOverlay(style.LineNumber, kind, style),
					selected, current, enabled, style,
				),
			))
		}
		marker := ' '
		if row.continuation {
			marker = '>'
		} else if value, ok := kind.Marker(); ok {
			marker = value
		}
		markerStyle := style.Marker
		if row.continuation {
			markerStyle = markerStyle.Merge(style.Continuation)
		}
		spans = append(spans, tui.NewTextSpan(
			fmt.Sprintf("%c ", marker),
			diffRowOverlay(
				diffKindOverlay(markerStyle, kind, style),
				selected, current, enabled, style,
			),
		))
	}
	visible := sliceCodeSpansByCells(
		row.spans, row.checkpoints, horizontalOffset,
		layout.CodeWidth(), layout.Options().WidthProfile(),
	)
	for _, span := range visible {
		spans = append(spans, tui.NewTextSpan(
			span.Text,
			diffRowOverlay(
				diffKindOverlay(span.Style, kind, style),
				selected, current, enabled, style,
			),
		))
	}
	return spans
}

func diffLineNumber(value uint64, present bool, width uint32, continuation bool) string {
	if continuation || !present {
		return fmt.Sprintf("%*s", int(width), "")
	}
	return fmt.Sprintf("%*d", int(width), value)
}

func diffKindOverlay(base vt.Style, kind DiffLineKind, style DiffViewStyle) vt.Style {
	semantic := style.Context
	switch kind {
	case DiffLineMetadata:
		semantic = style.Metadata
	case DiffLineHunk:
		semantic = style.Hunk
	case DiffLineAddition:
		semantic = style.Addition
	case DiffLineDeletion:
		semantic = style.Deletion
	}
	return semantic.Merge(base)
}

func diffRowOverlay(
	base vt.Style,
	selected, current, enabled bool,
	style DiffViewStyle,
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

func diffViewRowID(root tui.NodeID, row int) tui.NodeID {
	return tui.NewNodeID(fmt.Sprintf("%s:diff-visual-row:%d", root, row))
}

func diffCopyActionsEnabled(
	document DiffDocument,
	state DiffViewState,
	hasCopyHandler bool,
) (selection, documentCopy bool) {
	start, end := state.SelectedLines()
	return hasCopyHandler && document.IsRangeCopyable(start, end),
		hasCopyHandler && document.IsRangeCopyable(0, document.LineCount())
}
