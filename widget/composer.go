package widget

import (
	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

const (
	defaultComposerMinRows = 1
	defaultComposerMaxRows = 6
	composerActionCount    = 3 + textAreaActionCount
)

// ComposerState is application-owned editor, history position, and draft state
type ComposerState struct {
	textArea        TextAreaState
	historyIndex    int
	hasHistoryIndex bool
	draft           TextAreaState
	hasDraft        bool
}

// NewComposerState returns state outside history browsing
func NewComposerState(textArea TextAreaState) ComposerState {
	return ComposerState{textArea: normalizeTextAreaState(textArea)}
}

// NewComposerStateAtEnd returns state with the cursor at the end of value
func NewComposerStateAtEnd(value string) ComposerState {
	return NewComposerState(NewTextAreaStateAtEnd(value))
}

// TextArea returns the controlled TextArea state
func (s ComposerState) TextArea() TextAreaState {
	return s.textArea
}

// HistoryIndex returns the oldest-to-newest entry currently being browsed
func (s ComposerState) HistoryIndex() (int, bool) {
	return s.historyIndex, s.hasHistoryIndex
}

// Draft returns the TextArea state restored after browsing past newest history
func (s ComposerState) Draft() (TextAreaState, bool) {
	return s.draft, s.hasDraft
}

// WithTextArea replaces editor state and leaves history browsing
func (s ComposerState) WithTextArea(textArea TextAreaState) ComposerState {
	s.textArea = normalizeTextAreaState(textArea)
	s.historyIndex = 0
	s.hasHistoryIndex = false
	s.draft = TextAreaState{}
	s.hasDraft = false
	return s
}

// ComposerOverflowPolicy controls insertion that would exceed maximum length
//
// Values other than ComposerOverflowTruncate use reject behavior.
type ComposerOverflowPolicy uint8

const (
	// ComposerOverflowReject rejects the complete Text or Paste edit
	ComposerOverflowReject ComposerOverflowPolicy = iota
	// ComposerOverflowTruncate inserts the longest grapheme-aligned prefix that fits
	ComposerOverflowTruncate
)

type composerLengthUnit uint8

const (
	composerLengthUTF8Bytes composerLengthUnit = iota
	composerLengthGraphemes
)

type composerLengthLimit struct {
	maximum  int
	unit     composerLengthUnit
	overflow ComposerOverflowPolicy
}

// Composer is a controlled multiline message editor with submit and recall actions
//
// Composer owns no persistence or submit meaning. History entries are supplied
// oldest-to-newest by the application. Its root, viewport, and caret IDs must
// be distinct and stable.
type Composer[Message any] struct {
	id             tui.NodeID
	viewportID     tui.NodeID
	caretID        tui.NodeID
	state          ComposerState
	history        []string
	enabled        bool
	submitEnabled  bool
	placeholder    string
	style          TextAreaStyle
	selectionStyle vt.Style
	widthProfile   celltext.WidthProfile
	wrapWidth      int
	hasWrap        bool
	minRows        int
	maxRows        int
	lengthLimit    composerLengthLimit
	hasLengthLimit bool
	validation     tui.Node[Message]
	hasValidation  bool
	onChange       func(ComposerState) Message
	onSubmit       func() Message
	onUndo         func() Message
	onRedo         func() Message
}

// NewComposer returns a Composer using application-owned state
//
// A nil change or submit callback creates a disabled Composer.
func NewComposer[Message any](
	id, viewportID, caretID tui.NodeID,
	state ComposerState,
	onChange func(ComposerState) Message,
	onSubmit func() Message,
) Composer[Message] {
	return Composer[Message]{
		id: id, viewportID: viewportID, caretID: caretID,
		state:   normalizeComposerState(state),
		enabled: onChange != nil && onSubmit != nil, submitEnabled: true,
		style: DefaultTextAreaStyle(), selectionStyle: vt.Style{Reverse: true},
		widthProfile: celltext.ModernWidth(),
		minRows:      defaultComposerMinRows, maxRows: defaultComposerMaxRows,
		onChange: onChange, onSubmit: onSubmit,
	}
}

// Enabled sets whether the Composer can receive focus and edit or submit
func (c Composer[Message]) Enabled(enabled bool) Composer[Message] {
	c.enabled = enabled && c.onChange != nil && c.onSubmit != nil
	return c
}

// SubmitEnabled sets whether submit is valid while leaving editing enabled
func (c Composer[Message]) SubmitEnabled(enabled bool) Composer[Message] {
	c.submitEnabled = enabled
	return c
}

// Placeholder sets text shown when the value is empty
func (c Composer[Message]) Placeholder(placeholder string) Composer[Message] {
	c.placeholder = celltext.NormalizeUTF8(placeholder)
	return c
}

// Style replaces TextArea visual styles
func (c Composer[Message]) Style(style TextAreaStyle) Composer[Message] {
	c.style = style
	return c
}

// SelectionStyle sets the style merged over selected text
func (c Composer[Message]) SelectionStyle(style vt.Style) Composer[Message] {
	c.selectionStyle = style
	return c
}

// WidthProfile sets the terminal cell-width policy used by editing and layout
//
// Pass ViewContext.WidthProfile to keep the widget aligned with its Runtime.
func (c Composer[Message]) WidthProfile(profile celltext.WidthProfile) Composer[Message] {
	c.widthProfile = profile
	return c
}

// NoWrap uses logical lines without soft wrapping
func (c Composer[Message]) NoWrap() Composer[Message] {
	c.wrapWidth = 0
	c.hasWrap = false
	return c
}

// SoftWrap wraps visual lines at a terminal-cell width
//
// A non-positive width is normalized to one Cell. Applications should
// recompute the width from ViewContext after a resize.
func (c Composer[Message]) SoftWrap(width int) Composer[Message] {
	c.wrapWidth = max(width, 1)
	c.hasWrap = true
	return c
}

// Rows clamps automatic editor height between normalized row bounds
//
// minRows is normalized to at least one and maxRows to at least that minimum.
func (c Composer[Message]) Rows(minRows, maxRows int) Composer[Message] {
	c.minRows = max(minRows, 1)
	c.maxRows = max(maxRows, c.minRows)
	return c
}

// MaximumUTF8Bytes limits future insertion without rewriting existing text
//
// A negative maximum is normalized to zero.
func (c Composer[Message]) MaximumUTF8Bytes(maximum int, overflow ComposerOverflowPolicy) Composer[Message] {
	c.lengthLimit = composerLengthLimit{
		maximum: max(maximum, 0), unit: composerLengthUTF8Bytes, overflow: overflow,
	}
	c.hasLengthLimit = true
	return c
}

// MaximumGraphemes limits future insertion without rewriting existing text
//
// A negative maximum is normalized to zero.
func (c Composer[Message]) MaximumGraphemes(maximum int, overflow ComposerOverflowPolicy) Composer[Message] {
	c.lengthLimit = composerLengthLimit{
		maximum: max(maximum, 0), unit: composerLengthGraphemes, overflow: overflow,
	}
	c.hasLengthLimit = true
	return c
}

// History replaces oldest-to-newest recall entries without owning persistence
func (c Composer[Message]) History(entries []string) Composer[Message] {
	c.history = make([]string, len(entries))
	for index, entry := range entries {
		c.history[index] = celltext.NormalizeUTF8(entry)
	}
	return c
}

// Validation renders an application-provided Node below the editor viewport
func (c Composer[Message]) Validation(validation tui.Node[Message]) Composer[Message] {
	c.validation = validation
	c.hasValidation = true
	return c
}

// OnUndo sets the message emitted for Control-Z
func (c Composer[Message]) OnUndo(handler func() Message) Composer[Message] {
	c.onUndo = handler
	return c
}

// OnRedo sets the message emitted for Control-Y and Control-Shift-Z
func (c Composer[Message]) OnRedo(handler func() Message) Composer[Message] {
	c.onRedo = handler
	return c
}

// VisibleRows returns editor viewport height before parent layout clamping
func (c Composer[Message]) VisibleRows() uint32 {
	return composerVisibleRows(c.state.textArea, c.wrapWidth, c.hasWrap, c.minRows, c.maxRows, c.widthProfile)
}

// ActionDescriptors returns Composer actions followed by inherited TextArea actions
//
// The first three actions are submit, history previous, and history next. The
// remaining 18 preserve TextArea order with Composer newline defaults.
func (c Composer[Message]) ActionDescriptors() []tui.ActionDescriptor {
	text := composerTextActionDescriptors(
		c.enabled, c.onUndo != nil, c.onRedo != nil,
		c.state.textArea, c.wrapWidth, c.hasWrap,
		c.widthProfile,
	)
	hasUp, hasDown := composerTextDirections(text)
	leading := composerLeadingActionDescriptors(
		c.enabled, c.submitEnabled, !hasUp && composerHasHistoryPrevious(c.state, len(c.history)),
		!hasDown && c.state.hasHistoryIndex,
	)
	descriptors := make([]tui.ActionDescriptor, 0, composerActionCount)
	descriptors = append(descriptors, leading[:]...)
	descriptors = append(descriptors, text[:]...)
	return descriptors
}

// Node builds the public semantic Node for this Composer
func (c Composer[Message]) Node() tui.Node[Message] {
	rows := c.VisibleRows()
	hasUndo, hasRedo := c.onUndo != nil, c.onRedo != nil
	current := c.state
	onChange := c.onChange
	textArea := NewTextArea(c.id, c.state.textArea, func(textArea TextAreaState) Message {
		return onChange(composerChangedState(current, textArea))
	}).Enabled(c.enabled).
		Placeholder(c.placeholder).
		Style(c.style).
		SelectionStyle(c.selectionStyle).
		WidthProfile(c.widthProfile).
		BoundaryNavigation(TextAreaBoundaryBubble).
		Viewport(c.viewportID, c.caretID, tui.Fixed(rows))
	if c.hasWrap {
		textArea = textArea.SoftWrap(c.wrapWidth)
	}
	if c.onUndo != nil {
		textArea = textArea.OnUndo(c.onUndo)
	}
	if c.onRedo != nil {
		textArea = textArea.OnRedo(c.onRedo)
	}

	textDescriptors := composerTextActionDescriptors(
		c.enabled, hasUndo, hasRedo, c.state.textArea, c.wrapWidth, c.hasWrap,
		c.widthProfile,
	)
	hasUp, hasDown := composerTextDirections(textDescriptors)
	var previous, next ComposerState
	hasPrevious, hasNext := false, false
	if !hasUp {
		previous, hasPrevious = composerHistoryPrevious(c.state, c.history)
	}
	if !hasDown {
		next, hasNext = composerHistoryNext(c.state, c.history)
	}
	leadingDescriptors := composerLeadingActionDescriptors(
		c.enabled, c.submitEnabled, hasPrevious, hasNext,
	)
	leadingActions := composerLeadingActions(
		leadingDescriptors, c.id, c.onChange, c.onSubmit,
		previous, hasPrevious, next, hasNext,
	)
	var insertionPolicy textAreaInsertionPolicy
	if c.hasLengthLimit {
		insertionPolicy = composerInsertionPolicy(c.lengthLimit)
	}
	editor := textArea.nodeWithActions(textDescriptors, leadingActions[:], insertionPolicy)
	children := []tui.Node[Message]{editor}
	if c.hasValidation {
		children = append(children, c.validation)
	}
	return tui.Column(children...)
}

func normalizeComposerState(state ComposerState) ComposerState {
	state.textArea = normalizeTextAreaState(state.textArea)
	if !state.hasHistoryIndex {
		state.historyIndex = 0
		state.draft = TextAreaState{}
		state.hasDraft = false
	} else if state.hasDraft {
		state.draft = normalizeTextAreaState(state.draft)
	}
	return state
}

func composerTextActionDescriptors(
	enabled, hasUndo, hasRedo bool,
	state TextAreaState,
	wrapWidth int,
	hasWrap bool,
	profile celltext.WidthProfile,
) [textAreaActionCount]tui.ActionDescriptor {
	descriptors := textAreaActionDescriptors(
		enabled, hasUndo, hasRedo, TextAreaBoundaryBubble, state, wrapWidth, hasWrap, profile,
	)
	for index, descriptor := range descriptors {
		if descriptor.ID() == tui.TextInsertLineBreakActionID {
			descriptors[index] = composerLineBreakDescriptor.WithAvailability(descriptor.Availability())
			break
		}
	}
	return descriptors
}

func composerTextDirections(descriptors [textAreaActionCount]tui.ActionDescriptor) (bool, bool) {
	hasUp, hasDown := false, false
	for _, descriptor := range descriptors {
		if descriptor.ID() == tui.TextCursorUpActionID {
			hasUp = descriptor.Availability() == tui.ActionEnabled
		}
		if descriptor.ID() == tui.TextCursorDownActionID {
			hasDown = descriptor.Availability() == tui.ActionEnabled
		}
	}
	return hasUp, hasDown
}

func composerLeadingActionDescriptors(
	enabled, submitEnabled, hasPrevious, hasNext bool,
) [3]tui.ActionDescriptor {
	submitAvailability := tui.ActionDisabledPassThrough
	if enabled && submitEnabled {
		submitAvailability = tui.ActionEnabled
	} else if enabled {
		submitAvailability = tui.ActionDisabledConsume
	}
	return [3]tui.ActionDescriptor{
		composerSubmitDescriptor.WithAvailability(submitAvailability),
		historyPreviousDescriptor.WithAvailability(composerHistoryAvailability(enabled && hasPrevious)),
		historyNextDescriptor.WithAvailability(composerHistoryAvailability(enabled && hasNext)),
	}
}

func composerHistoryAvailability(enabled bool) tui.ActionAvailability {
	if enabled {
		return tui.ActionEnabled
	}
	return tui.ActionDisabledPassThrough
}

func composerLeadingActions[Message any](
	descriptors [3]tui.ActionDescriptor,
	id tui.NodeID,
	onChange func(ComposerState) Message,
	onSubmit func() Message,
	previous ComposerState,
	hasPrevious bool,
	next ComposerState,
	hasNext bool,
) [3]tui.Action[Message] {
	return [3]tui.Action[Message]{
		tui.NewAction(descriptors[0], func(tui.ActionEvent) tui.EventResult[Message] {
			return tui.ConsumeResult[Message]().Focus(id).Emit(onSubmit())
		}),
		tui.NewAction(descriptors[1], func(tui.ActionEvent) tui.EventResult[Message] {
			result := tui.ConsumeResult[Message]().Focus(id)
			if hasPrevious {
				result = result.Emit(onChange(previous))
			}
			return result
		}),
		tui.NewAction(descriptors[2], func(tui.ActionEvent) tui.EventResult[Message] {
			result := tui.ConsumeResult[Message]().Focus(id)
			if hasNext {
				result = result.Emit(onChange(next))
			}
			return result
		}),
	}
}

func composerChangedState(current ComposerState, textArea TextAreaState) ComposerState {
	next := current
	valueChanged := next.textArea.value != textArea.value
	next.textArea = textArea
	if valueChanged {
		next.historyIndex = 0
		next.hasHistoryIndex = false
		next.draft = TextAreaState{}
		next.hasDraft = false
	}
	return normalizeComposerState(next)
}

func composerHistoryPrevious(state ComposerState, history []string) (ComposerState, bool) {
	if !composerHasHistoryPrevious(state, len(history)) {
		return ComposerState{}, false
	}
	target := len(history) - 1
	if state.hasHistoryIndex {
		target = min(state.historyIndex, len(history)) - 1
	}
	next := state
	if !next.hasDraft {
		next.draft = next.textArea
		next.hasDraft = true
	}
	next.historyIndex = target
	next.hasHistoryIndex = true
	next.textArea = NewTextAreaStateAtEnd(history[target])
	return normalizeComposerState(next), true
}

func composerHasHistoryPrevious(state ComposerState, historyLength int) bool {
	if historyLength == 0 {
		return false
	}
	return !state.hasHistoryIndex || state.historyIndex != 0
}

func composerHistoryNext(state ComposerState, history []string) (ComposerState, bool) {
	if !state.hasHistoryIndex {
		return ComposerState{}, false
	}
	next := state
	if state.historyIndex+1 < len(history) {
		target := state.historyIndex + 1
		next.historyIndex = target
		next.textArea = NewTextAreaStateAtEnd(history[target])
		return normalizeComposerState(next), true
	}
	if next.hasDraft {
		next.textArea = next.draft
	}
	next.historyIndex = 0
	next.hasHistoryIndex = false
	next.draft = TextAreaState{}
	next.hasDraft = false
	return normalizeComposerState(next), true
}

func composerVisibleRows(
	state TextAreaState,
	wrapWidth int,
	hasWrap bool,
	minRows int,
	maxRows int,
	profile celltext.WidthProfile,
) uint32 {
	minimum := max(minRows, 1)
	maximum := max(maxRows, minimum)
	rows := min(max(textAreaVisualLineCount(state, wrapWidth, hasWrap, profile), minimum), maximum)
	return cellCount(rows)
}

func composerInsertionPolicy(limit composerLengthLimit) textAreaInsertionPolicy {
	return func(state TextAreaState, inserted string) (string, bool) {
		return composerInsertion(limit, state, inserted)
	}
}

func composerInsertion(
	limit composerLengthLimit,
	state TextAreaState,
	inserted string,
) (string, bool) {
	selectionStart, selectionEnd, selected := state.Selection()
	selectedText := ""
	if selected {
		selectedText = state.value[selectionStart:selectionEnd]
	}
	base := composerMeasuredLength(state.value, limit.unit) - composerMeasuredLength(selectedText, limit.unit)
	insertedLength := composerMeasuredLength(inserted, limit.unit)
	available := max(limit.maximum-base, 0)
	if base <= limit.maximum && insertedLength <= available {
		return inserted, true
	}
	if limit.overflow != ComposerOverflowTruncate {
		return "", false
	}
	boundary := 0
	graphemeCount := 0
	iterator := celltext.IterateGraphemes(inserted)
	for grapheme, ok := iterator.Next(); ok; grapheme, ok = iterator.Next() {
		if limit.unit == composerLengthUTF8Bytes {
			if grapheme.End > available {
				break
			}
		} else {
			if graphemeCount == available {
				break
			}
			graphemeCount++
		}
		boundary = grapheme.End
	}
	return inserted[:boundary], true
}

func composerMeasuredLength(value string, unit composerLengthUnit) int {
	if unit == composerLengthUTF8Bytes {
		return len(value)
	}
	count := 0
	iterator := celltext.IterateGraphemes(value)
	for _, ok := iterator.Next(); ok; _, ok = iterator.Next() {
		count++
	}
	return count
}

var composerSubmitDescriptor = tui.NewActionDescriptor(
	ComposerSubmitActionID,
	"Submit",
	[]tui.KeyBinding{tui.NewKeyBinding(tui.NewKeyStroke(vt.KeyEnter, vt.Modifiers{}))},
)

var historyPreviousDescriptor = tui.NewActionDescriptor(
	HistoryPreviousActionID,
	"Previous history entry",
	[]tui.KeyBinding{composerRepeatableBinding(vt.KeyUp, vt.Modifiers{})},
)

var historyNextDescriptor = tui.NewActionDescriptor(
	HistoryNextActionID,
	"Next history entry",
	[]tui.KeyBinding{composerRepeatableBinding(vt.KeyDown, vt.Modifiers{})},
)

var composerLineBreakDescriptor = tui.NewActionDescriptor(
	tui.TextInsertLineBreakActionID,
	"Insert line break",
	[]tui.KeyBinding{
		composerRepeatableBinding(vt.KeyEnter, vt.Modifiers{Shift: true}),
		composerRepeatableBinding(vt.KeyEnter, vt.Modifiers{Alt: true}),
		tui.NewKeyBinding(tui.NewCharacterKeyStroke('o', vt.Modifiers{Control: true})).
			WithRepeatPolicy(tui.RepeatAllow).
			WithSupport(tui.BindingSupported),
	},
)

func composerRepeatableBinding(code vt.KeyCode, modifiers vt.Modifiers) tui.KeyBinding {
	return tui.NewKeyBinding(tui.NewKeyStroke(code, modifiers)).WithRepeatPolicy(tui.RepeatAllow)
}
