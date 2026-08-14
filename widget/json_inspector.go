package widget

import (
	"sort"
	"strconv"
	"strings"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

// DefaultJSONInspectorMaxScalarGraphemes is the default decoded String and Number preview limit
const DefaultJSONInspectorMaxScalarGraphemes = 80

// JSONInspectorState contains application-owned selection and expanded branch identities
//
// The zero value selects and expands the root
type JSONInspectorState struct {
	selected JSONPointer
	expanded *jsonInspectorExpanded
}

type jsonInspectorExpanded struct {
	values []JSONPointer
}

var defaultJSONInspectorExpanded = jsonInspectorExpanded{values: []JSONPointer{{}}}

// NewJSONInspectorState returns state from one selected pointer and unique expanded pointers
func NewJSONInspectorState(selected JSONPointer, expanded []JSONPointer) JSONInspectorState {
	values := append([]JSONPointer(nil), expanded...)
	sort.Slice(values, func(left, right int) bool {
		return values[left].value < values[right].value
	})
	unique := values[:0]
	for _, pointer := range values {
		if len(unique) == 0 || unique[len(unique)-1] != pointer {
			unique = append(unique, pointer)
		}
	}
	return JSONInspectorState{
		selected: selected,
		expanded: &jsonInspectorExpanded{values: unique},
	}
}

// Selected returns the selected complete JSON Pointer
func (s JSONInspectorState) Selected() JSONPointer {
	return s.selected
}

// Expanded returns an independent lexically ordered expanded-pointer slice
func (s JSONInspectorState) Expanded() []JSONPointer {
	return append([]JSONPointer(nil), s.expandedValues()...)
}

// IsExpanded reports whether one pointer is expanded
func (s JSONInspectorState) IsExpanded(pointer JSONPointer) bool {
	values := s.expandedValues()
	index := sort.Search(len(values), func(index int) bool {
		return values[index].value >= pointer.value
	})
	return index < len(values) && values[index] == pointer
}

// WithSelected returns state with a replacement selected pointer
func (s JSONInspectorState) WithSelected(selected JSONPointer) JSONInspectorState {
	s.selected = selected
	return s
}

// WithExpanded returns state with one pointer inserted into or removed from expansion
func (s JSONInspectorState) WithExpanded(pointer JSONPointer, expanded bool) JSONInspectorState {
	values := s.expandedValues()
	index := sort.Search(len(values), func(index int) bool {
		return values[index].value >= pointer.value
	})
	present := index < len(values) && values[index] == pointer
	if present == expanded {
		return s
	}
	next := make([]JSONPointer, 0, len(values)+1)
	next = append(next, values[:index]...)
	if expanded {
		next = append(next, pointer)
	} else {
		index++
	}
	next = append(next, values[index:]...)
	s.expanded = &jsonInspectorExpanded{values: next}
	return s
}

// Equal reports semantic selection and expansion equality
func (s JSONInspectorState) Equal(other JSONInspectorState) bool {
	if s.selected != other.selected {
		return false
	}
	left, right := s.expandedValues(), other.expandedValues()
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func (s JSONInspectorState) expandedValues() []JSONPointer {
	if s.expanded == nil {
		return defaultJSONInspectorExpanded.values
	}
	return s.expanded.values
}

// JSONInspectorCopyRequest independently owns one complete JSON value for application handling
type JSONInspectorCopyRequest struct {
	// Source is the stable inspector root Node ID
	Source tui.NodeID
	// Path is the selected complete JSON Pointer
	Path JSONPointer
	// Kind is the selected JSON value category
	Kind JSONKind
	// Text is an independent copy of the complete deterministic compact serialization
	Text string
}

// JSONInspectorStyle contains independent semantic style slots
type JSONInspectorStyle struct {
	// Key is used for object key contents
	Key vt.Style
	// String is used for decoded String previews
	String vt.Style
	// Number is used for Number previews
	Number vt.Style
	// Boolean is used for Boolean values
	Boolean vt.Style
	// Null is used for null values
	Null vt.Style
	// Punctuation is used for delimiters, indentation, and disclosure markers
	Punctuation vt.Style
	// Index is used for array indexes
	Index vt.Style
	// Summary is used for branch child counts
	Summary vt.Style
	// Selected is merged over every span in the selected row
	Selected vt.Style
	// Focused is merged over the selected row while the inspector owns focus
	Focused vt.Style
	// Disabled is merged over every span while the inspector is disabled
	Disabled vt.Style
}

// DefaultJSONInspectorStyle returns the standard attribute-only semantic styles
func DefaultJSONInspectorStyle() JSONInspectorStyle {
	return JSONInspectorStyle{
		Key:         vt.Style{Bold: true},
		Boolean:     vt.Style{Bold: true},
		Null:        vt.Style{Dim: true},
		Punctuation: vt.Style{Dim: true},
		Index:       vt.Style{Dim: true},
		Summary:     vt.Style{Dim: true},
		Selected:    vt.Style{Reverse: true},
		Focused:     vt.Style{Underline: true},
		Disabled:    vt.Style{Dim: true},
	}
}

// JSONInspector is a controlled bounded hierarchy view over one immutable JSON document
//
// Selection and expansion updates are returned through the application
// callback Copy requests contain complete source values and never access an
// operating-system or terminal clipboard directly
type JSONInspector[Message any] struct {
	id                     tui.NodeID
	document               JSONDocument
	state                  JSONInspectorState
	viewportHeight         int
	maximumScalarGraphemes int
	enabled                bool
	style                  JSONInspectorStyle
	onChange               func(JSONInspectorState) Message
	onCopy                 func(JSONInspectorCopyRequest) Message
}

// NewJSONInspector returns an inspector using application-owned controlled state
//
// A nil onChange function creates a disabled inspector
func NewJSONInspector[Message any](
	id tui.NodeID,
	document JSONDocument,
	state JSONInspectorState,
	onChange func(JSONInspectorState) Message,
) JSONInspector[Message] {
	state = normalizeJSONInspectorState(document, state)
	return JSONInspector[Message]{
		id: id, document: document, state: state,
		maximumScalarGraphemes: DefaultJSONInspectorMaxScalarGraphemes,
		enabled:                onChange != nil, style: DefaultJSONInspectorStyle(), onChange: onChange,
	}
}

// State returns the visually normalized controlled state
func (i JSONInspector[Message]) State() JSONInspectorState {
	return i.state
}

// Enabled sets whether the inspector can receive focus and emit messages
func (i JSONInspector[Message]) Enabled(enabled bool) JSONInspector[Message] {
	i.enabled = enabled && i.onChange != nil
	return i
}

// Viewport limits constructed rows to a deterministic window following selection
//
// A non-positive height constructs every visible row
func (i JSONInspector[Message]) Viewport(height int) JSONInspector[Message] {
	i.viewportHeight = max(height, 0)
	return i
}

// MaximumScalarGraphemes sets the positive decoded String and Number preview limit
//
// A non-positive limit restores DefaultJSONInspectorMaxScalarGraphemes
func (i JSONInspector[Message]) MaximumScalarGraphemes(maximum int) JSONInspector[Message] {
	if maximum <= 0 {
		maximum = DefaultJSONInspectorMaxScalarGraphemes
	}
	i.maximumScalarGraphemes = maximum
	return i
}

// Style replaces every semantic style slot
func (i JSONInspector[Message]) Style(style JSONInspectorStyle) JSONInspector[Message] {
	i.style = style
	return i
}

// OnCopy sets the application callback for complete selected-value copy requests
func (i JSONInspector[Message]) OnCopy(
	handler func(JSONInspectorCopyRequest) Message,
) JSONInspector[Message] {
	i.onCopy = handler
	return i
}

// ActionDescriptors returns the eight ordered semantic actions
//
// The order is activate, previous, next, first, last, collapse, expand, and copy
func (i JSONInspector[Message]) ActionDescriptors() []tui.ActionDescriptor {
	descriptors := jsonInspectorActionDescriptors(i.enabled, i.onCopy != nil)
	return append([]tui.ActionDescriptor(nil), descriptors[:]...)
}

// Node builds the public semantic node for this inspector
func (i JSONInspector[Message]) Node() tui.Node[Message] {
	return buildJSONInspectorNode(i)
}

var defaultJSONInspectorActionDescriptors = [8]tui.ActionDescriptor{
	tui.NewActionDescriptor(
		ActivateActionID,
		"Activate",
		[]tui.KeyBinding{
			tui.NewKeyBinding(tui.NewKeyStroke(vt.KeyEnter, vt.Modifiers{})),
			tui.NewKeyBinding(tui.NewCharacterKeyStroke(' ', vt.Modifiers{})),
		},
	),
	tui.NewActionDescriptor(
		SelectionPreviousActionID,
		"Previous",
		[]tui.KeyBinding{repeatableActionBinding(vt.KeyUp)},
	),
	tui.NewActionDescriptor(
		SelectionNextActionID,
		"Next",
		[]tui.KeyBinding{repeatableActionBinding(vt.KeyDown)},
	),
	tui.NewActionDescriptor(
		SelectionFirstActionID,
		"First",
		[]tui.KeyBinding{repeatableActionBinding(vt.KeyHome)},
	),
	tui.NewActionDescriptor(
		SelectionLastActionID,
		"Last",
		[]tui.KeyBinding{repeatableActionBinding(vt.KeyEnd)},
	),
	tui.NewActionDescriptor(
		CollapseActionID,
		"Collapse",
		[]tui.KeyBinding{repeatableActionBinding(vt.KeyLeft)},
	),
	tui.NewActionDescriptor(
		ExpandActionID,
		"Expand",
		[]tui.KeyBinding{repeatableActionBinding(vt.KeyRight)},
	),
	tui.NewActionDescriptor(
		InspectorCopyActionID,
		"Copy value",
		[]tui.KeyBinding{
			tui.NewKeyBinding(tui.NewCharacterKeyStroke('c', vt.Modifiers{Control: true})),
		},
	),
}

type jsonInspectorAction uint8

const (
	jsonInspectorActivate jsonInspectorAction = iota
	jsonInspectorPrevious
	jsonInspectorNext
	jsonInspectorFirst
	jsonInspectorLast
	jsonInspectorCollapse
	jsonInspectorExpand
	jsonInspectorCopy
)

func jsonInspectorActionDescriptors(enabled, hasCopyHandler bool) [8]tui.ActionDescriptor {
	var descriptors [8]tui.ActionDescriptor
	for index, descriptor := range defaultJSONInspectorActionDescriptors {
		available := enabled && (jsonInspectorAction(index) != jsonInspectorCopy || hasCopyHandler)
		availability := tui.ActionEnabled
		if !available {
			availability = tui.ActionDisabledPassThrough
		}
		descriptors[index] = descriptor.WithAvailability(availability)
	}
	return descriptors
}

type jsonInspectorActionContext[Message any] struct {
	id       tui.NodeID
	document JSONDocument
	state    JSONInspectorState
	selected int
	onChange func(JSONInspectorState) Message
	onCopy   func(JSONInspectorCopyRequest) Message
}

func buildJSONInspectorNode[Message any](inspector JSONInspector[Message]) tui.Node[Message] {
	selected, _ := inspector.document.indexOf(inspector.state.Selected())
	indices := jsonInspectorVisibleWindow(
		inspector.document, inspector.state, selected, inspector.viewportHeight,
	)
	descriptors := jsonInspectorActionDescriptors(inspector.enabled, inspector.onCopy != nil)
	context := &jsonInspectorActionContext[Message]{
		id: inspector.id, document: inspector.document, state: inspector.state,
		selected: selected, onChange: inspector.onChange, onCopy: inspector.onCopy,
	}
	rows := make([]tui.Node[Message], 0, len(indices))
	for _, index := range indices {
		record := inspector.document.record(index)
		isSelected := index == selected
		spans := jsonInspectorRowSpans(
			record,
			inspector.document.nodeAt(index).Serialized(),
			jsonInspectorExpandedBranch(record, inspector.state),
			inspector.maximumScalarGraphemes,
			inspector.style,
			inspector.enabled,
			isSelected,
		)
		rowID := jsonInspectorRowID(inspector.id, record.path)
		row := tui.Paragraph[Message](spans, tui.ParagraphOptions{
			Wrap: tui.WrapNone, Alignment: tui.AlignStart,
		}).WithID(rowID)
		if inspector.enabled {
			rowIndex := index
			row = row.OnEvent(rowID, func(event vt.Event) tui.EventResult[Message] {
				return jsonInspectorPointerResult(event, rowIndex, context)
			})
		}
		if isSelected && inspector.enabled {
			rows = append(rows, jsonInspectorActionTarget(
				tui.Column(row), context, descriptors, inspector.style.Focused,
			))
		} else {
			rows = append(rows, row)
		}
	}

	root := tui.Column(rows...)
	if inspector.viewportHeight > 0 {
		height := uint64(inspector.viewportHeight)
		if height > uint64(^uint32(0)) {
			height = uint64(^uint32(0))
		}
		root = root.WithLength(tui.Fixed(uint32(height)))
	}
	if inspector.enabled {
		return root
	}
	actions := make([]tui.Action[Message], len(descriptors))
	for index, descriptor := range descriptors {
		actions[index] = tui.NewAction[Message](descriptor, nil)
	}
	return root.WithID(inspector.id).OnActions(inspector.id, actions)
}

func jsonInspectorActionTarget[Message any](
	node tui.Node[Message],
	context *jsonInspectorActionContext[Message],
	descriptors [8]tui.ActionDescriptor,
	focusedStyle vt.Style,
) tui.Node[Message] {
	actions := make([]tui.Action[Message], len(descriptors))
	for index, descriptor := range descriptors {
		action := jsonInspectorAction(index)
		actions[index] = tui.NewAction(
			descriptor,
			func(tui.ActionEvent) tui.EventResult[Message] {
				return jsonInspectorActionResult(action, context)
			},
		)
	}
	return node.Focusable(context.id).
		WithFocusedStyle(focusedStyle).
		OnActions(context.id, actions).
		OnEvent(context.id, func(event vt.Event) tui.EventResult[Message] {
			if isBlockedJSONInspectorRepeat(event, context.onCopy != nil) {
				return tui.ConsumeResult[Message]()
			}
			return tui.IgnoreResult[Message]()
		})
}

func jsonInspectorActionResult[Message any](
	action jsonInspectorAction,
	context *jsonInspectorActionContext[Message],
) tui.EventResult[Message] {
	if action == jsonInspectorCopy {
		return jsonInspectorCopyResult(context)
	}
	next := jsonInspectorStateForAction(
		context.document, context.state, context.selected, action,
	)
	return emitJSONInspectorChange(
		tui.ConsumeResult[Message]().Focus(context.id), next, context,
	)
}

func jsonInspectorCopyResult[Message any](
	context *jsonInspectorActionContext[Message],
) tui.EventResult[Message] {
	if context.onCopy == nil {
		return tui.IgnoreResult[Message]()
	}
	node := context.document.nodeAt(context.selected)
	request := JSONInspectorCopyRequest{
		Source: context.id, Path: node.Path(), Kind: node.Kind(), Text: strings.Clone(node.Serialized()),
	}
	return tui.ConsumeResult[Message]().Focus(context.id).Emit(context.onCopy(request))
}

func jsonInspectorPointerResult[Message any](
	event vt.Event,
	index int,
	context *jsonInspectorActionContext[Message],
) tui.EventResult[Message] {
	if !isPointerActivationEvent(event) {
		return tui.IgnoreResult[Message]()
	}
	next := jsonInspectorStateForPointer(context.document, context.state, index)
	return emitJSONInspectorChange(
		tui.ConsumeResult[Message]().Focus(context.id), next, context,
	)
}

func emitJSONInspectorChange[Message any](
	result tui.EventResult[Message],
	next JSONInspectorState,
	context *jsonInspectorActionContext[Message],
) tui.EventResult[Message] {
	if next.Equal(context.state) {
		return result
	}
	return result.Emit(context.onChange(next))
}

func isBlockedJSONInspectorRepeat(event vt.Event, hasCopyHandler bool) bool {
	if event.Kind != vt.EventKey || event.Key.Action != vt.KeyRepeat {
		return false
	}
	key := event.Key
	return key.Code == vt.KeyEnter && key.Modifiers == (vt.Modifiers{}) ||
		key.Code == vt.KeyCharacter && key.Character == ' ' && key.Modifiers == (vt.Modifiers{}) ||
		hasCopyHandler && key.Code == vt.KeyCharacter && key.Character == 'c' && key.Modifiers == (vt.Modifiers{Control: true})
}

func jsonInspectorStateForAction(
	document JSONDocument,
	state JSONInspectorState,
	selected int,
	action jsonInspectorAction,
) JSONInspectorState {
	record := document.record(selected)
	switch action {
	case jsonInspectorActivate:
		if jsonInspectorBranch(record) {
			return state.WithExpanded(record.path, !jsonInspectorExpandedBranch(record, state))
		}
	case jsonInspectorPrevious:
		if index, ok := jsonInspectorPreviousVisible(document, state, selected); ok {
			return jsonInspectorSelectIndex(document, state, index)
		}
	case jsonInspectorNext:
		if index, ok := jsonInspectorNextVisible(document, state, selected); ok {
			return jsonInspectorSelectIndex(document, state, index)
		}
	case jsonInspectorFirst:
		return jsonInspectorSelectIndex(document, state, 0)
	case jsonInspectorLast:
		return jsonInspectorSelectIndex(document, state, jsonInspectorLastVisible(document, state))
	case jsonInspectorCollapse:
		if jsonInspectorExpandedBranch(record, state) {
			return state.WithExpanded(record.path, false)
		}
		if record.hasParent {
			return jsonInspectorSelectIndex(document, state, record.parent)
		}
	case jsonInspectorExpand:
		if jsonInspectorBranch(record) && !jsonInspectorExpandedBranch(record, state) {
			return state.WithExpanded(record.path, true)
		}
		if jsonInspectorExpandedBranch(record, state) {
			return jsonInspectorSelectIndex(document, state, selected+1)
		}
	}
	return state
}

func jsonInspectorStateForPointer(
	document JSONDocument,
	state JSONInspectorState,
	index int,
) JSONInspectorState {
	record := document.record(index)
	next := state.WithSelected(record.path)
	if jsonInspectorBranch(record) {
		next = next.WithExpanded(record.path, !jsonInspectorExpandedBranch(record, state))
	}
	return next
}

func jsonInspectorSelectIndex(
	document JSONDocument,
	state JSONInspectorState,
	index int,
) JSONInspectorState {
	return state.WithSelected(document.record(index).path)
}

func normalizeJSONInspectorState(
	document JSONDocument,
	state JSONInspectorState,
) JSONInspectorState {
	selected, ok := document.indexOf(state.Selected())
	if !ok {
		selected = 0
	}
	selected = normalizeJSONInspectorVisibleIndex(document, state, selected)
	return jsonInspectorSelectIndex(document, state, selected)
}

func normalizeJSONInspectorVisibleIndex(
	document JSONDocument,
	state JSONInspectorState,
	index int,
) int {
	normalized := index
	record := document.record(index)
	for record.hasParent {
		index = record.parent
		record = document.record(index)
		if jsonInspectorBranch(record) && !jsonInspectorExpandedBranch(record, state) {
			normalized = index
		}
	}
	return normalized
}

func jsonInspectorBranch(record *jsonDocumentNode) bool {
	return record.childCount > 0 && (record.kind == JSONArrayKind || record.kind == JSONObjectKind)
}

func jsonInspectorExpandedBranch(record *jsonDocumentNode, state JSONInspectorState) bool {
	return jsonInspectorBranch(record) && state.IsExpanded(record.path)
}

func jsonInspectorNextVisible(
	document JSONDocument,
	state JSONInspectorState,
	index int,
) (int, bool) {
	record := document.record(index)
	next := record.subtreeEnd
	if jsonInspectorExpandedBranch(record, state) {
		next = index + 1
	}
	return next, next < document.Len()
}

func jsonInspectorPreviousVisible(
	document JSONDocument,
	state JSONInspectorState,
	index int,
) (int, bool) {
	if index <= 0 {
		return 0, false
	}
	return normalizeJSONInspectorVisibleIndex(document, state, index-1), true
}

func jsonInspectorLastVisible(document JSONDocument, state JSONInspectorState) int {
	index := 0
	for {
		next, ok := jsonInspectorNextVisible(document, state, index)
		if !ok {
			return index
		}
		index = next
	}
}

func jsonInspectorVisibleWindow(
	document JSONDocument,
	state JSONInspectorState,
	selected int,
	height int,
) []int {
	if height <= 0 {
		visible := make([]int, 0, min(document.Len(), 1024))
		index := 0
		for {
			visible = append(visible, index)
			next, ok := jsonInspectorNextVisible(document, state, index)
			if !ok {
				return visible
			}
			index = next
		}
	}
	start := selected
	for range height / 2 {
		previous, ok := jsonInspectorPreviousVisible(document, state, start)
		if !ok {
			break
		}
		start = previous
	}
	visible := make([]int, 0, height)
	index := start
	for len(visible) < height {
		visible = append(visible, index)
		next, ok := jsonInspectorNextVisible(document, state, index)
		if !ok {
			break
		}
		index = next
	}
	for len(visible) < height {
		previous, ok := jsonInspectorPreviousVisible(document, state, visible[0])
		if !ok {
			break
		}
		visible = append(visible, 0)
		copy(visible[1:], visible[:len(visible)-1])
		visible[0] = previous
	}
	return visible
}

func jsonInspectorRowID(root tui.NodeID, path JSONPointer) tui.NodeID {
	var builder strings.Builder
	builder.Grow(32 + len(root) + len(path.value))
	builder.WriteString("nagi.json-inspector.row:")
	jsonInspectorWriteInt(&builder, len(root))
	builder.WriteByte(':')
	builder.WriteString(string(root))
	builder.WriteByte(':')
	jsonInspectorWriteInt(&builder, len(path.value))
	builder.WriteByte(':')
	builder.WriteString(path.value)
	return tui.NodeID(builder.String())
}

func jsonInspectorRowSpans(
	record *jsonDocumentNode,
	serializedValue string,
	expanded bool,
	maximumScalarGraphemes int,
	style JSONInspectorStyle,
	enabled bool,
	selected bool,
) []tui.TextSpan {
	overlay := vt.Style{}
	if !enabled {
		overlay = style.Disabled
	} else if selected {
		overlay = style.Selected
	}
	spans := make([]tui.TextSpan, 0, 12)
	jsonInspectorPushSpan(
		&spans, strings.Repeat("  ", int(record.depth)), style.Punctuation, overlay,
	)
	disclosure := "  "
	if jsonInspectorBranch(record) && expanded {
		disclosure = "▼ "
	} else if jsonInspectorBranch(record) {
		disclosure = "▶ "
	}
	jsonInspectorPushSpan(&spans, disclosure, style.Punctuation, overlay)
	switch record.label.kind {
	case jsonNodeObjectKeyLabel:
		jsonInspectorPushSpan(&spans, "\"", style.Punctuation, overlay)
		jsonInspectorPushSpan(
			&spans, escapeJSONStringContent(record.label.key), style.Key, overlay,
		)
		jsonInspectorPushSpan(&spans, "\": ", style.Punctuation, overlay)
	case jsonNodeArrayIndexLabel:
		jsonInspectorPushSpan(&spans, "[", style.Punctuation, overlay)
		jsonInspectorPushSpan(&spans, strconv.Itoa(record.label.index), style.Index, overlay)
		jsonInspectorPushSpan(&spans, "]: ", style.Punctuation, overlay)
	}
	jsonInspectorPushValueSpans(
		&spans, record, serializedValue, maximumScalarGraphemes, style, overlay,
	)
	return spans
}

func jsonInspectorPushValueSpans(
	spans *[]tui.TextSpan,
	record *jsonDocumentNode,
	serializedValue string,
	maximumScalarGraphemes int,
	style JSONInspectorStyle,
	overlay vt.Style,
) {
	switch record.kind {
	case JSONNullKind:
		jsonInspectorPushSpan(spans, "null", style.Null, overlay)
	case JSONBooleanKind:
		jsonInspectorPushSpan(spans, serializedValue, style.Boolean, overlay)
	case JSONNumberKind:
		preview, truncated := jsonInspectorScalarPreview(serializedValue, maximumScalarGraphemes)
		jsonInspectorPushSpan(spans, preview, style.Number, overlay)
		if truncated {
			jsonInspectorPushSpan(spans, "…", style.Number, overlay)
		}
	case JSONStringKind:
		preview, truncated := jsonInspectorScalarPreview(record.decodedString, maximumScalarGraphemes)
		jsonInspectorPushSpan(spans, "\"", style.Punctuation, overlay)
		jsonInspectorPushSpan(spans, escapeJSONStringContent(preview), style.String, overlay)
		if truncated {
			jsonInspectorPushSpan(spans, "…", style.String, overlay)
		}
		jsonInspectorPushSpan(spans, "\"", style.Punctuation, overlay)
	case JSONArrayKind:
		jsonInspectorPushBranchSummary(spans, "[", "]", record.childCount, style, overlay)
	case JSONObjectKind:
		jsonInspectorPushBranchSummary(spans, "{", "}", record.childCount, style, overlay)
	}
}

func jsonInspectorPushBranchSummary(
	spans *[]tui.TextSpan,
	opening, closing string,
	childCount int,
	style JSONInspectorStyle,
	overlay vt.Style,
) {
	jsonInspectorPushSpan(spans, opening, style.Punctuation, overlay)
	if childCount > 0 {
		jsonInspectorPushSpan(spans, strconv.Itoa(childCount), style.Summary, overlay)
	}
	jsonInspectorPushSpan(spans, closing, style.Punctuation, overlay)
}

func jsonInspectorPushSpan(spans *[]tui.TextSpan, text string, style, overlay vt.Style) {
	if text != "" {
		*spans = append(*spans, tui.NewTextSpan(text, style.Merge(overlay)))
	}
}

func jsonInspectorScalarPreview(value string, maximum int) (string, bool) {
	maximum = max(maximum, 1)
	iterator := celltext.IterateGraphemes(value)
	end := 0
	for range maximum {
		cluster, ok := iterator.Next()
		if !ok {
			return value, false
		}
		end = cluster.End
	}
	if _, ok := iterator.Next(); ok {
		return value[:end], true
	}
	return value, false
}

func escapeJSONStringContent(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	writeJSONStringContent(&builder, value)
	return builder.String()
}

func jsonInspectorWriteInt(builder *strings.Builder, value int) {
	var digits [24]byte
	builder.Write(strconv.AppendInt(digits[:0], int64(value), 10))
}
