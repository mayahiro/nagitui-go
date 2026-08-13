package tui

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mayahiro/nagi-go/vt"
)

// ActionID is a stable, key-independent action identity
type ActionID string

const (
	// FocusNextActionID is the stable Action ID for moving focus forward
	FocusNextActionID ActionID = "nagi.focus.next"
	// FocusPreviousActionID is the stable Action ID for moving focus backward
	FocusPreviousActionID ActionID = "nagi.focus.previous"
	// ScrollPageUpActionID is the stable Action ID for scrolling one page toward the start
	ScrollPageUpActionID ActionID = "nagi.scroll.page-up"
	// ScrollPageDownActionID is the stable Action ID for scrolling one page toward the end
	ScrollPageDownActionID ActionID = "nagi.scroll.page-down"
	// ScrollStartActionID is the stable Action ID for scrolling to the enabled-axis beginning
	ScrollStartActionID ActionID = "nagi.scroll.start"
	// ScrollEndActionID is the stable Action ID for scrolling to the enabled-axis end
	ScrollEndActionID ActionID = "nagi.scroll.end"
	// TextCursorLeftActionID is the stable Action ID for moving a text cursor left
	TextCursorLeftActionID ActionID = "nagi.text.cursor.left"
	// TextCursorRightActionID is the stable Action ID for moving a text cursor right
	TextCursorRightActionID ActionID = "nagi.text.cursor.right"
	// TextCursorWordLeftActionID is the stable Action ID for moving a text cursor to the previous word
	TextCursorWordLeftActionID ActionID = "nagi.text.cursor.word-left"
	// TextCursorWordRightActionID is the stable Action ID for moving a text cursor to the next word
	TextCursorWordRightActionID ActionID = "nagi.text.cursor.word-right"
	// TextCursorUpActionID is the stable Action ID for moving a text cursor up
	TextCursorUpActionID ActionID = "nagi.text.cursor.up"
	// TextCursorDownActionID is the stable Action ID for moving a text cursor down
	TextCursorDownActionID ActionID = "nagi.text.cursor.down"
	// TextCursorLineStartActionID is the stable Action ID for moving a text cursor to the current line start
	TextCursorLineStartActionID ActionID = "nagi.text.cursor.line-start"
	// TextCursorLineEndActionID is the stable Action ID for moving a text cursor to the current line end
	TextCursorLineEndActionID ActionID = "nagi.text.cursor.line-end"
	// TextCursorDocumentStartActionID is the stable Action ID for moving a text cursor to the document start
	TextCursorDocumentStartActionID ActionID = "nagi.text.cursor.document-start"
	// TextCursorDocumentEndActionID is the stable Action ID for moving a text cursor to the document end
	TextCursorDocumentEndActionID ActionID = "nagi.text.cursor.document-end"
	// TextSelectionExtendLeftActionID is the stable Action ID for extending text selection left
	TextSelectionExtendLeftActionID ActionID = "nagi.text.selection.extend-left"
	// TextSelectionExtendRightActionID is the stable Action ID for extending text selection right
	TextSelectionExtendRightActionID ActionID = "nagi.text.selection.extend-right"
	// TextSelectionExtendWordLeftActionID is the stable Action ID for extending text selection to the previous word
	TextSelectionExtendWordLeftActionID ActionID = "nagi.text.selection.extend-word-left"
	// TextSelectionExtendWordRightActionID is the stable Action ID for extending text selection to the next word
	TextSelectionExtendWordRightActionID ActionID = "nagi.text.selection.extend-word-right"
	// TextSelectionExtendUpActionID is the stable Action ID for extending text selection up
	TextSelectionExtendUpActionID ActionID = "nagi.text.selection.extend-up"
	// TextSelectionExtendDownActionID is the stable Action ID for extending text selection down
	TextSelectionExtendDownActionID ActionID = "nagi.text.selection.extend-down"
	// TextSelectionExtendLineStartActionID is the stable Action ID for extending text selection to the current line start
	TextSelectionExtendLineStartActionID ActionID = "nagi.text.selection.extend-line-start"
	// TextSelectionExtendLineEndActionID is the stable Action ID for extending text selection to the current line end
	TextSelectionExtendLineEndActionID ActionID = "nagi.text.selection.extend-line-end"
	// TextSelectionExtendDocumentStartActionID is the stable Action ID for extending text selection to the document start
	TextSelectionExtendDocumentStartActionID ActionID = "nagi.text.selection.extend-document-start"
	// TextSelectionExtendDocumentEndActionID is the stable Action ID for extending text selection to the document end
	TextSelectionExtendDocumentEndActionID ActionID = "nagi.text.selection.extend-document-end"
	// TextSelectAllActionID is the stable Action ID for selecting one complete semantic text document
	TextSelectAllActionID ActionID = "nagi.text.select-all"
	// TextCopySelectionActionID is the stable Action ID for copying the current semantic text selection
	TextCopySelectionActionID ActionID = "nagi.text.copy-selection"
	// TextCopyDocumentActionID is the stable Action ID for copying one complete semantic text document
	TextCopyDocumentActionID ActionID = "nagi.text.copy-document"
	// TextDeleteBackwardActionID is the stable Action ID for deleting text backward
	TextDeleteBackwardActionID ActionID = "nagi.text.delete.backward"
	// TextDeleteForwardActionID is the stable Action ID for deleting text forward
	TextDeleteForwardActionID ActionID = "nagi.text.delete.forward"
	// TextInsertLineBreakActionID is the stable Action ID for inserting a line break
	TextInsertLineBreakActionID ActionID = "nagi.text.insert-line-break"
	// TextUndoActionID is the stable Action ID for undoing a text edit
	TextUndoActionID ActionID = "nagi.text.undo"
	// TextRedoActionID is the stable Action ID for redoing a text edit
	TextRedoActionID ActionID = "nagi.text.redo"
)

// NewActionID returns an Action ID from an application-defined stable value
func NewActionID(value string) ActionID {
	return ActionID(value)
}

// String returns the application-defined identity
func (id ActionID) String() string {
	return string(id)
}

// ActionAvailability controls whether an action participates in key matching
type ActionAvailability uint8

const (
	// ActionEnabled makes the action an active candidate
	ActionEnabled ActionAvailability = iota
	// ActionDisabledPassThrough leaves the action unavailable and allows matching to continue
	ActionDisabledPassThrough
	// ActionDisabledConsume leaves the action unavailable but makes it an active blocking candidate
	ActionDisabledConsume
)

func (a ActionAvailability) candidate() bool {
	return a == ActionEnabled || a == ActionDisabledConsume
}

// RepeatPolicy controls whether an explicit key-repeat event may match
type RepeatPolicy uint8

const (
	// RepeatInitialOnly accepts an initial press or a legacy action of unknown kind
	RepeatInitialOnly RepeatPolicy = iota
	// RepeatAllow also accepts an explicit repeat event
	RepeatAllow
)

// BindingSupport is terminal capability metadata for one key binding
type BindingSupport uint8

const (
	// BindingSupportUnknown means terminal support is not known
	BindingSupportUnknown BindingSupport = iota
	// BindingSupported means the terminal is known to report the stroke distinctly
	BindingSupported
	// BindingUnsupported means the terminal is not known to report the stroke distinctly
	BindingUnsupported
)

// KeyStroke is one normalized logical key and exact modifier set
type KeyStroke struct {
	code       vt.KeyCode
	character  rune
	function   uint8
	functional uint32
	modifiers  vt.Modifiers
}

// NewKeyStroke returns a named-key stroke
//
// Use NewCharacterKeyStroke for vt.KeyCharacter and NewFunctionKeyStroke for
// vt.KeyFunction.
func NewKeyStroke(code vt.KeyCode, modifiers vt.Modifiers) KeyStroke {
	return KeyStroke{code: code, modifiers: modifiers.WithoutLocks()}
}

// NewCharacterKeyStroke returns a Character stroke
func NewCharacterKeyStroke(character rune, modifiers vt.Modifiers) KeyStroke {
	return KeyStroke{code: vt.KeyCharacter, character: character, modifiers: modifiers.WithoutLocks()}
}

// NewFunctionKeyStroke returns a Function-key stroke
func NewFunctionKeyStroke(number uint8, modifiers vt.Modifiers) KeyStroke {
	return KeyStroke{code: vt.KeyFunction, function: number, modifiers: modifiers.WithoutLocks()}
}

// NewFunctionalKeyStroke returns a protocol-defined functional-key stroke
func NewFunctionalKeyStroke(number uint32, modifiers vt.Modifiers) KeyStroke {
	return KeyStroke{code: vt.KeyFunctional, functional: number, modifiers: modifiers.WithoutLocks()}
}

// Code returns the logical key category
func (s KeyStroke) Code() vt.KeyCode {
	return s.code
}

// Character returns the scalar for a Character stroke
func (s KeyStroke) Character() (rune, bool) {
	return s.character, s.code == vt.KeyCharacter
}

// Function returns the number for a Function-key stroke
func (s KeyStroke) Function() (uint8, bool) {
	return s.function, s.code == vt.KeyFunction
}

// Functional returns the protocol-defined number for a Functional-key stroke
func (s KeyStroke) Functional() (uint32, bool) {
	return s.functional, s.code == vt.KeyFunctional
}

// Modifiers returns the exact modifier set
func (s KeyStroke) Modifiers() vt.Modifiers {
	return s.modifiers
}

// KeyStrokeFromEvent normalizes one Key or single-scalar Text event
//
// Release, Paste, multi-scalar Text, and non-key events return false.
func KeyStrokeFromEvent(event vt.Event) (KeyStroke, bool) {
	switch event.Kind {
	case vt.EventKey:
		if event.Key.Action == vt.KeyRelease {
			return KeyStroke{}, false
		}
		return keyStrokeFromKeyEvent(event.Key), true
	case vt.EventText:
		character, size := utf8.DecodeRuneInString(event.Text)
		if size == 0 || size != len(event.Text) || (character == utf8.RuneError && size == 1) {
			return KeyStroke{}, false
		}
		return NewCharacterKeyStroke(character, vt.Modifiers{}), true
	default:
		return KeyStroke{}, false
	}
}

func keyStrokeFromKeyEvent(event vt.KeyEvent) KeyStroke {
	switch event.Code {
	case vt.KeyCharacter:
		return NewCharacterKeyStroke(event.Character, event.Modifiers)
	case vt.KeyFunction:
		return NewFunctionKeyStroke(event.Function, event.Modifiers)
	case vt.KeyFunctional:
		return NewFunctionalKeyStroke(event.Functional, event.Modifiers)
	default:
		return NewKeyStroke(event.Code, event.Modifiers)
	}
}

// Notation returns deterministic user-facing English notation
func (s KeyStroke) Notation() string {
	var notation strings.Builder
	if s.modifiers.Control {
		notation.WriteString("Ctrl+")
	}
	if s.modifiers.Alt {
		notation.WriteString("Alt+")
	}
	if s.modifiers.Shift {
		notation.WriteString("Shift+")
	}
	if s.modifiers.Meta {
		notation.WriteString("Meta+")
	}
	if s.modifiers.Super {
		notation.WriteString("Super+")
	}
	if s.modifiers.Hyper {
		notation.WriteString("Hyper+")
	}
	switch s.code {
	case vt.KeyCharacter:
		switch {
		case s.character == ' ':
			notation.WriteString("Space")
		case unicode.IsControl(s.character):
			fmt.Fprintf(&notation, "U+%04X", s.character)
		default:
			notation.WriteRune(s.character)
		}
	case vt.KeyEnter:
		notation.WriteString("Enter")
	case vt.KeyTab:
		notation.WriteString("Tab")
	case vt.KeyBackspace:
		notation.WriteString("Backspace")
	case vt.KeyEscape:
		notation.WriteString("Escape")
	case vt.KeyUp:
		notation.WriteString("Up")
	case vt.KeyDown:
		notation.WriteString("Down")
	case vt.KeyRight:
		notation.WriteString("Right")
	case vt.KeyLeft:
		notation.WriteString("Left")
	case vt.KeyHome:
		notation.WriteString("Home")
	case vt.KeyEnd:
		notation.WriteString("End")
	case vt.KeyInsert:
		notation.WriteString("Insert")
	case vt.KeyDelete:
		notation.WriteString("Delete")
	case vt.KeyPageUp:
		notation.WriteString("PageUp")
	case vt.KeyPageDown:
		notation.WriteString("PageDown")
	case vt.KeyFunction:
		fmt.Fprintf(&notation, "F%d", s.function)
	case vt.KeyFunctional:
		fmt.Fprintf(&notation, "Functional(%d)", s.functional)
	default:
		notation.WriteString("Unknown")
	}
	return notation.String()
}

// KeyBinding is one stroke with repeat and terminal-support metadata
type KeyBinding struct {
	stroke       KeyStroke
	repeatPolicy RepeatPolicy
	support      BindingSupport
}

// NewKeyBinding returns an initial-only binding with unknown terminal support
func NewKeyBinding(stroke KeyStroke) KeyBinding {
	return KeyBinding{stroke: stroke}
}

// WithRepeatPolicy returns a binding with the supplied repeat policy
func (b KeyBinding) WithRepeatPolicy(policy RepeatPolicy) KeyBinding {
	b.repeatPolicy = policy
	return b
}

// WithSupport returns a binding with the supplied terminal-support metadata
//
// Support metadata affects presentation but never event matching.
func (b KeyBinding) WithSupport(support BindingSupport) KeyBinding {
	b.support = support
	return b
}

// Stroke returns the normalized stroke
func (b KeyBinding) Stroke() KeyStroke {
	return b.stroke
}

// RepeatPolicy returns the repeat policy
func (b KeyBinding) RepeatPolicy() RepeatPolicy {
	return b.repeatPolicy
}

// Support returns terminal-support metadata
func (b KeyBinding) Support() BindingSupport {
	return b.support
}

// Matches reports whether a normalized event matches this binding
func (b KeyBinding) Matches(event vt.Event) bool {
	if !b.MatchesStroke(event) {
		return false
	}
	if event.Kind == vt.EventText {
		return true
	}
	switch event.Key.Action {
	case vt.KeyActionUnknown, vt.KeyPress:
		return true
	case vt.KeyRepeat:
		return b.repeatPolicy == RepeatAllow
	default:
		return false
	}
}

// MatchesStroke reports whether an event has this binding's normalized stroke
//
// Unlike Matches, this ignores repeat policy. Disabled-consume actions use it
// to keep repeated input inside the same semantic boundary without invoking a
// handler that is initial-only.
func (b KeyBinding) MatchesStroke(event vt.Event) bool {
	stroke, ok := KeyStrokeFromEvent(event)
	return ok && stroke == b.stroke
}

// ActionDescriptor is one handler-independent action definition
type ActionDescriptor struct {
	id              ActionID
	label           string
	defaultBindings []KeyBinding
	availability    ActionAvailability
	helpVisible     bool
}

// NewActionDescriptor returns an enabled, Help-visible action descriptor
func NewActionDescriptor(id ActionID, label string, bindings []KeyBinding) ActionDescriptor {
	return ActionDescriptor{
		id: id, label: label, defaultBindings: cloneKeyBindings(bindings),
		availability: ActionEnabled, helpVisible: true,
	}
}

// WithAvailability returns a descriptor with the supplied availability
func (d ActionDescriptor) WithAvailability(availability ActionAvailability) ActionDescriptor {
	d.availability = availability
	return d
}

// WithHelpVisible returns a descriptor with the supplied Help visibility
func (d ActionDescriptor) WithHelpVisible(visible bool) ActionDescriptor {
	d.helpVisible = visible
	return d
}

// ID returns the stable Action ID
func (d ActionDescriptor) ID() ActionID {
	return d.id
}

// Label returns the user-facing label
func (d ActionDescriptor) Label() string {
	return d.label
}

// DefaultBindings returns a copy of the ordered default bindings
func (d ActionDescriptor) DefaultBindings() []KeyBinding {
	return cloneKeyBindings(d.defaultBindings)
}

// Availability returns current availability
func (d ActionDescriptor) Availability() ActionAvailability {
	return d.availability
}

// HelpVisible reports whether Help projections normally include the action
func (d ActionDescriptor) HelpVisible() bool {
	return d.helpVisible
}

// ActionEvent is one semantic action invocation after key resolution
type ActionEvent struct {
	action ActionID
	stroke KeyStroke
}

// Action returns the resolved Action ID
func (e ActionEvent) Action() ActionID {
	return e.action
}

// Stroke returns the normalized stroke that triggered the action
func (e ActionEvent) Stroke() KeyStroke {
	return e.stroke
}

// Action pairs a handler-independent descriptor with one Node-local handler
type Action[Message any] struct {
	descriptor ActionDescriptor
	handler    func(ActionEvent) EventResult[Message]
}

// NewAction returns an action from a descriptor and semantic handler
//
// A nil handler ignores matching invocations and allows local Core handling
// and raw event routing to continue.
func NewAction[Message any](
	descriptor ActionDescriptor,
	handler func(ActionEvent) EventResult[Message],
) Action[Message] {
	return Action[Message]{descriptor: descriptor, handler: handler}
}

// Descriptor returns the handler-independent action descriptor
func (a Action[Message]) Descriptor() ActionDescriptor {
	return a.descriptor
}

func (a Action[Message]) invoke(event ActionEvent) EventResult[Message] {
	if a.handler == nil {
		return IgnoreResult[Message]()
	}
	return a.handler(event)
}

type keyOverride struct {
	action   ActionID
	bindings []KeyBinding
}

// KeyMap is one immutable Action-ID-to-binding override layer
type KeyMap struct {
	overrides []keyOverride
}

// NewKeyMap returns an empty override layer
func NewKeyMap() KeyMap {
	return KeyMap{}
}

// Rebind returns a new layer with one complete binding-list replacement
//
// An empty replacement unbinds the action. Rebinding an Action ID already
// present in this layer returns DuplicateActionOverrideError.
func (m KeyMap) Rebind(action ActionID, bindings []KeyBinding) (KeyMap, error) {
	if _, ok := m.bindingsView(action); ok {
		return m, &DuplicateActionOverrideError{Action: action}
	}
	overrides := append([]keyOverride(nil), m.overrides...)
	overrides = append(overrides, keyOverride{action: action, bindings: cloneKeyBindings(bindings)})
	return KeyMap{overrides: overrides}, nil
}

// Bindings returns a copy of the replacement when this layer names action
func (m KeyMap) Bindings(action ActionID) ([]KeyBinding, bool) {
	bindings, ok := m.bindingsView(action)
	return cloneKeyBindings(bindings), ok
}

func (m KeyMap) bindingsView(action ActionID) ([]KeyBinding, bool) {
	for _, bindingOverride := range m.overrides {
		if bindingOverride.action == action {
			return bindingOverride.bindings, true
		}
	}
	return nil, false
}

// Empty reports whether the layer contains no overrides
func (m KeyMap) Empty() bool {
	return len(m.overrides) == 0
}

// DuplicateActionOverrideError reports a repeated Action ID in one KeyMap
type DuplicateActionOverrideError struct {
	// Action is the duplicated Action ID
	Action ActionID
}

// Error returns the duplicate override diagnostic
func (e *DuplicateActionOverrideError) Error() string {
	return "duplicate key override for ActionID " + e.Action.String()
}

// KeyScopePropagation controls Runtime ancestor action propagation at one scope
type KeyScopePropagation uint8

const (
	// KeyScopeContinue continues resolving actions on ancestor Nodes
	KeyScopeContinue KeyScopePropagation = iota
	// KeyScopeStopAtScope skips actions outside the scope while preserving raw routing
	KeyScopeStopAtScope
)

// KeyScope is one active semantic scope and its immutable KeyMap layer
type KeyScope struct {
	id          NodeID
	keyMap      KeyMap
	propagation KeyScopePropagation
}

// NewKeyScope returns a scope identified by a stable semantic Node ID
func NewKeyScope(id NodeID, keyMap KeyMap) KeyScope {
	return KeyScope{id: id, keyMap: keyMap}
}

// WithPropagation returns a scope with the supplied action propagation behavior
func (s KeyScope) WithPropagation(propagation KeyScopePropagation) KeyScope {
	s.propagation = propagation
	return s
}

// ID returns the semantic scope identity
func (s KeyScope) ID() NodeID {
	return s.id
}

// KeyMap returns this scope's immutable override layer
func (s KeyScope) KeyMap() KeyMap {
	return s.keyMap
}

// Propagation returns action propagation behavior after this scope
func (s KeyScope) Propagation() KeyScopePropagation {
	return s.propagation
}

// ResolvedAction is one action after active KeyMap layers have been applied
type ResolvedAction struct {
	id           ActionID
	label        string
	bindings     []KeyBinding
	availability ActionAvailability
	helpVisible  bool
}

// ID returns the stable Action ID
func (a ResolvedAction) ID() ActionID {
	return a.id
}

// Label returns the user-facing label
func (a ResolvedAction) Label() string {
	return a.label
}

// Bindings returns a copy of the ordered effective bindings
func (a ResolvedAction) Bindings() []KeyBinding {
	return cloneKeyBindings(a.bindings)
}

// Availability returns current availability
func (a ResolvedAction) Availability() ActionAvailability {
	return a.availability
}

// HelpVisible reports whether Help projections normally include the action
func (a ResolvedAction) HelpVisible() bool {
	return a.helpVisible
}

// ResolvedActions contains ordered actions for one owner and active scope path
type ResolvedActions struct {
	owner     NodeID
	scopePath []NodeID
	actions   []ResolvedAction
}

// Owner returns the semantic owner identity
func (a ResolvedActions) Owner() NodeID {
	return a.owner
}

// ScopePath returns a copy of active scope IDs in root-to-target order
func (a ResolvedActions) ScopePath() []NodeID {
	return append([]NodeID(nil), a.scopePath...)
}

// Actions returns a copy of actions in declaration order
func (a ResolvedActions) Actions() []ResolvedAction {
	return append([]ResolvedAction(nil), a.actions...)
}

// HelpActions returns actions normally included in Help projections
func (a ResolvedActions) HelpActions() []ResolvedAction {
	visible := make([]ResolvedAction, 0, len(a.actions))
	for _, action := range a.actions {
		if action.helpVisible {
			visible = append(visible, action)
		}
	}
	return visible
}

// BindingConflictKind identifies one action-group binding conflict
type BindingConflictKind uint8

const (
	// ConflictDuplicateAction means an Action ID was declared more than once
	ConflictDuplicateAction BindingConflictKind = iota
	// ConflictDuplicateBinding means one action contained the same stroke more than once
	ConflictDuplicateBinding
	// ConflictAmbiguousBinding means two active candidates contained the same stroke
	ConflictAmbiguousBinding
)

// BindingConflictError is one deterministic conflict for an action owner
type BindingConflictError struct {
	kind      BindingConflictKind
	owner     NodeID
	scopePath []NodeID
	actions   []ActionID
	stroke    KeyStroke
	hasStroke bool
}

// Kind returns the conflict category
func (e *BindingConflictError) Kind() BindingConflictKind {
	return e.kind
}

// Owner returns the semantic owner identity
func (e *BindingConflictError) Owner() NodeID {
	return e.owner
}

// ScopePath returns a copy of active scope IDs in root-to-target order
func (e *BindingConflictError) ScopePath() []NodeID {
	return append([]NodeID(nil), e.scopePath...)
}

// Actions returns involved Action IDs in declaration order
func (e *BindingConflictError) Actions() []ActionID {
	return append([]ActionID(nil), e.actions...)
}

// Stroke returns the conflicting stroke when the category has one
func (e *BindingConflictError) Stroke() (KeyStroke, bool) {
	return e.stroke, e.hasStroke
}

// Error returns the deterministic conflict diagnostic
func (e *BindingConflictError) Error() string {
	action := func(index int) ActionID {
		if index >= 0 && index < len(e.actions) {
			return e.actions[index]
		}
		return ""
	}
	switch e.kind {
	case ConflictDuplicateAction:
		return fmt.Sprintf("duplicate ActionID %s on NodeID %s", action(0), e.owner)
	case ConflictDuplicateBinding:
		return fmt.Sprintf(
			"duplicate key binding %s for ActionID %s on NodeID %s",
			e.stroke.Notation(), action(0), e.owner,
		)
	default:
		return fmt.Sprintf(
			"ambiguous key binding %s for ActionIDs %s and %s on NodeID %s",
			e.stroke.Notation(), action(0), action(1), e.owner,
		)
	}
}

// ResolveActions applies active root-to-target KeyMap layers to one owner
//
// ResolveActions is pure and does not install actions into a Runtime. It
// preserves action, binding, and scope order and returns the first conflict in
// that stable order.
func ResolveActions(owner NodeID, actions []ActionDescriptor, scopes []KeyScope) (ResolvedActions, error) {
	scopePath := make([]NodeID, len(scopes))
	for index, scope := range scopes {
		scopePath[index] = scope.id
	}
	seenActions := make(map[ActionID]struct{}, len(actions))
	activeBindings := make(map[KeyStroke]ActionID)
	resolved := make([]ResolvedAction, 0, len(actions))

	for _, action := range actions {
		if _, duplicate := seenActions[action.id]; duplicate {
			return ResolvedActions{}, newBindingConflict(
				ConflictDuplicateAction, owner, scopePath, []ActionID{action.id}, KeyStroke{}, false,
			)
		}
		seenActions[action.id] = struct{}{}

		bindings := action.defaultBindings
		for _, scope := range scopes {
			if replacement, ok := scope.keyMap.bindingsView(action.id); ok {
				bindings = replacement
			}
		}

		seenStrokes := make(map[KeyStroke]struct{}, len(bindings))
		for _, binding := range bindings {
			if _, duplicate := seenStrokes[binding.stroke]; duplicate {
				return ResolvedActions{}, newBindingConflict(
					ConflictDuplicateBinding, owner, scopePath, []ActionID{action.id}, binding.stroke, true,
				)
			}
			seenStrokes[binding.stroke] = struct{}{}
		}

		if action.availability.candidate() {
			for _, binding := range bindings {
				if existing, duplicate := activeBindings[binding.stroke]; duplicate {
					return ResolvedActions{}, newBindingConflict(
						ConflictAmbiguousBinding,
						owner,
						scopePath,
						[]ActionID{existing, action.id},
						binding.stroke,
						true,
					)
				}
				activeBindings[binding.stroke] = action.id
			}
		}

		resolved = append(resolved, ResolvedAction{
			id: action.id, label: action.label, bindings: bindings,
			availability: action.availability, helpVisible: action.helpVisible,
		})
	}

	return ResolvedActions{owner: owner, scopePath: scopePath, actions: resolved}, nil
}

func newBindingConflict(
	kind BindingConflictKind,
	owner NodeID,
	scopePath []NodeID,
	actions []ActionID,
	stroke KeyStroke,
	hasStroke bool,
) *BindingConflictError {
	return &BindingConflictError{
		kind: kind, owner: owner, scopePath: append([]NodeID(nil), scopePath...),
		actions: append([]ActionID(nil), actions...),
		stroke:  stroke, hasStroke: hasStroke,
	}
}

func cloneKeyBindings(bindings []KeyBinding) []KeyBinding {
	return append([]KeyBinding(nil), bindings...)
}
