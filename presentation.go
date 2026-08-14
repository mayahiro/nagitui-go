package tui

import (
	"github.com/mayahiro/nagi-go/content"
	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
)

// PresentationState is one validated application-supplied state token used by
// presentation rule matching.
//
// Its zero value is invalid and never participates in matching.
type PresentationState struct {
	token content.Class
}

// NewPresentationState creates a state using the portable Content token
// grammar.
func NewPresentationState(value string) (PresentationState, error) {
	token, err := content.NewClass(value)
	if err != nil {
		return PresentationState{}, err
	}
	return PresentationState{token: token}, nil
}

// NewPresentationStateBytes creates a state without repairing invalid UTF-8.
func NewPresentationStateBytes(value []byte) (PresentationState, error) {
	token, err := content.NewClassBytes(value)
	if err != nil {
		return PresentationState{}, err
	}
	return PresentationState{token: token}, nil
}

// String returns the portable state token.
func (s PresentationState) String() string {
	return s.token.String()
}

func (s PresentationState) valid() bool {
	return s.token.String() != ""
}

type presentationSelectorKind uint8

const (
	presentationSelectorAny presentationSelectorKind = iota
	presentationSelectorRole
	presentationSelectorClass
)

// PresentationSelector is one exact universal, Role, or Class selector.
//
// Its zero value is the universal selector.
type PresentationSelector struct {
	kind  presentationSelectorKind
	role  content.Role
	class content.Class
}

// AnyPresentationSelector returns a selector that matches every Content
// element.
func AnyPresentationSelector() PresentationSelector {
	return PresentationSelector{}
}

// RolePresentationSelector returns an exact semantic Role selector.
//
// A selector containing the zero Role never matches.
func RolePresentationSelector(role content.Role) PresentationSelector {
	return PresentationSelector{kind: presentationSelectorRole, role: role}
}

// ClassPresentationSelector returns an exact presentation Class selector.
//
// A selector containing the zero Class never matches.
func ClassPresentationSelector(class content.Class) PresentationSelector {
	return PresentationSelector{kind: presentationSelectorClass, class: class}
}

// IsAny reports whether this is the universal selector.
func (s PresentationSelector) IsAny() bool {
	return s.kind == presentationSelectorAny
}

// Role returns the exact Role when this is a Role selector.
func (s PresentationSelector) Role() (content.Role, bool) {
	return s.role, s.kind == presentationSelectorRole
}

// Class returns the exact Class when this is a Class selector.
func (s PresentationSelector) Class() (content.Class, bool) {
	return s.class, s.kind == presentationSelectorClass
}

func (s PresentationSelector) matches(element content.Element) bool {
	switch s.kind {
	case presentationSelectorAny:
		return true
	case presentationSelectorRole:
		return s.role.String() != "" && element.HasRole(s.role)
	case presentationSelectorClass:
		return s.class.String() != "" && element.HasClass(s.class)
	default:
		return false
	}
}

// DeclarationValueState identifies how one presentation property affects the
// current computed value.
type DeclarationValueState uint8

const (
	// DeclarationValueUnspecified leaves the current computed value unchanged.
	DeclarationValueUnspecified DeclarationValueState = iota
	// DeclarationValueSet replaces the current computed value with a concrete value.
	DeclarationValueSet
	// DeclarationValueInitial restores the property's element-specific initial value.
	DeclarationValueInitial
)

// DeclarationValue is an unspecified, concrete, or initial property value.
//
// Its zero value is unspecified.
type DeclarationValue[T any] struct {
	state DeclarationValueState
	value T
}

// SetDeclarationValue returns a declaration containing a concrete property
// value.
func SetDeclarationValue[T any](value T) DeclarationValue[T] {
	return DeclarationValue[T]{state: DeclarationValueSet, value: value}
}

// InitialDeclarationValue returns a declaration that restores a property's
// element-specific initial value.
func InitialDeclarationValue[T any]() DeclarationValue[T] {
	return DeclarationValue[T]{state: DeclarationValueInitial}
}

// State returns whether this value is unspecified, concrete, or initial.
func (v DeclarationValue[T]) State() DeclarationValueState {
	return v.state
}

// Value returns the concrete value when State is DeclarationValueSet.
func (v DeclarationValue[T]) Value() (T, bool) {
	return v.value, v.state == DeclarationValueSet
}

// TextStyleDeclaration contains independently declared inheritable terminal
// text properties.
//
// Its zero value leaves every property unspecified.
type TextStyleDeclaration struct {
	foreground     DeclarationValue[vt.Color]
	background     DeclarationValue[vt.Color]
	underlineColor DeclarationValue[vt.OptionalColor]
	bold           DeclarationValue[bool]
	dim            DeclarationValue[bool]
	italic         DeclarationValue[bool]
	underline      DeclarationValue[bool]
	blink          DeclarationValue[bool]
	reverse        DeclarationValue[bool]
	hidden         DeclarationValue[bool]
	strikethrough  DeclarationValue[bool]
}

// WithForeground returns this declaration with a foreground property.
func (d TextStyleDeclaration) WithForeground(value DeclarationValue[vt.Color]) TextStyleDeclaration {
	d.foreground = value
	return d
}

// Foreground returns the foreground property.
func (d TextStyleDeclaration) Foreground() DeclarationValue[vt.Color] {
	return d.foreground
}

// WithBackground returns this declaration with a background property.
func (d TextStyleDeclaration) WithBackground(value DeclarationValue[vt.Color]) TextStyleDeclaration {
	d.background = value
	return d
}

// Background returns the background property.
func (d TextStyleDeclaration) Background() DeclarationValue[vt.Color] {
	return d.background
}

// WithUnderlineColor returns this declaration with an optional underline
// color property.
func (d TextStyleDeclaration) WithUnderlineColor(value DeclarationValue[vt.OptionalColor]) TextStyleDeclaration {
	d.underlineColor = value
	return d
}

// UnderlineColor returns the optional underline color property.
func (d TextStyleDeclaration) UnderlineColor() DeclarationValue[vt.OptionalColor] {
	return d.underlineColor
}

// WithBold returns this declaration with a Bold property.
func (d TextStyleDeclaration) WithBold(value DeclarationValue[bool]) TextStyleDeclaration {
	d.bold = value
	return d
}

// Bold returns the Bold property.
func (d TextStyleDeclaration) Bold() DeclarationValue[bool] {
	return d.bold
}

// WithDim returns this declaration with a Dim property.
func (d TextStyleDeclaration) WithDim(value DeclarationValue[bool]) TextStyleDeclaration {
	d.dim = value
	return d
}

// Dim returns the Dim property.
func (d TextStyleDeclaration) Dim() DeclarationValue[bool] {
	return d.dim
}

// WithItalic returns this declaration with an Italic property.
func (d TextStyleDeclaration) WithItalic(value DeclarationValue[bool]) TextStyleDeclaration {
	d.italic = value
	return d
}

// Italic returns the Italic property.
func (d TextStyleDeclaration) Italic() DeclarationValue[bool] {
	return d.italic
}

// WithUnderline returns this declaration with an Underline property.
func (d TextStyleDeclaration) WithUnderline(value DeclarationValue[bool]) TextStyleDeclaration {
	d.underline = value
	return d
}

// Underline returns the Underline property.
func (d TextStyleDeclaration) Underline() DeclarationValue[bool] {
	return d.underline
}

// WithBlink returns this declaration with a Blink property.
func (d TextStyleDeclaration) WithBlink(value DeclarationValue[bool]) TextStyleDeclaration {
	d.blink = value
	return d
}

// Blink returns the Blink property.
func (d TextStyleDeclaration) Blink() DeclarationValue[bool] {
	return d.blink
}

// WithReverse returns this declaration with a Reverse property.
func (d TextStyleDeclaration) WithReverse(value DeclarationValue[bool]) TextStyleDeclaration {
	d.reverse = value
	return d
}

// Reverse returns the Reverse property.
func (d TextStyleDeclaration) Reverse() DeclarationValue[bool] {
	return d.reverse
}

// WithHidden returns this declaration with a Hidden property.
func (d TextStyleDeclaration) WithHidden(value DeclarationValue[bool]) TextStyleDeclaration {
	d.hidden = value
	return d
}

// Hidden returns the Hidden property.
func (d TextStyleDeclaration) Hidden() DeclarationValue[bool] {
	return d.hidden
}

// WithStrikethrough returns this declaration with a Strikethrough property.
func (d TextStyleDeclaration) WithStrikethrough(value DeclarationValue[bool]) TextStyleDeclaration {
	d.strikethrough = value
	return d
}

// Strikethrough returns the Strikethrough property.
func (d TextStyleDeclaration) Strikethrough() DeclarationValue[bool] {
	return d.strikethrough
}

// PresentationDisplay is the terminal layout mode computed for one Content
// element.
type PresentationDisplay uint8

const (
	// PresentationDisplayInline selects inline presentation.
	PresentationDisplayInline PresentationDisplay = iota
	// PresentationDisplayFlow selects flow presentation.
	PresentationDisplayFlow
	// PresentationDisplayParagraph selects paragraph presentation.
	PresentationDisplayParagraph
	// PresentationDisplaySequence selects sequence presentation.
	PresentationDisplaySequence
)

// PresentationDeclaration contains independently declared text and layout
// properties.
//
// Its zero value leaves every property unspecified.
type PresentationDeclaration struct {
	display         DeclarationValue[PresentationDisplay]
	length          DeclarationValue[Length]
	gap             DeclarationValue[uint32]
	visualSeparator DeclarationValue[string]
	wrap            DeclarationValue[WrapMode]
	alignment       DeclarationValue[HorizontalAlignment]
	textStyle       TextStyleDeclaration
}

// WithDisplay returns this declaration with a display property. An unknown
// concrete display is stored as unspecified.
func (d PresentationDeclaration) WithDisplay(value DeclarationValue[PresentationDisplay]) PresentationDeclaration {
	if display, ok := value.Value(); ok && !validPresentationDisplay(display) {
		value = DeclarationValue[PresentationDisplay]{}
	}
	d.display = value
	return d
}

// Display returns the display property.
func (d PresentationDeclaration) Display() DeclarationValue[PresentationDisplay] {
	return d.display
}

// WithLength returns this declaration with a main-axis Length property.
func (d PresentationDeclaration) WithLength(value DeclarationValue[Length]) PresentationDeclaration {
	d.length = value
	return d
}

// Length returns the main-axis Length property.
func (d PresentationDeclaration) Length() DeclarationValue[Length] {
	return d.length
}

// WithGap returns this declaration with a child gap property in terminal cells.
func (d PresentationDeclaration) WithGap(value DeclarationValue[uint32]) PresentationDeclaration {
	d.gap = value
	return d
}

// Gap returns the child gap property in terminal cells.
func (d PresentationDeclaration) Gap() DeclarationValue[uint32] {
	return d.gap
}

// WithVisualSeparator returns this declaration with a presentation-only
// separator after normalizing invalid UTF-8 runs. A concrete empty separator
// remains present and distinct from initial.
func (d PresentationDeclaration) WithVisualSeparator(value DeclarationValue[string]) PresentationDeclaration {
	if separator, ok := value.Value(); ok {
		value = SetDeclarationValue(celltext.NormalizeUTF8(separator))
	}
	d.visualSeparator = value
	return d
}

// VisualSeparator returns the presentation-only separator property.
func (d PresentationDeclaration) VisualSeparator() DeclarationValue[string] {
	return d.visualSeparator
}

// WithWrap returns this declaration with a paragraph wrapping property. An
// unknown concrete mode is stored as unspecified.
func (d PresentationDeclaration) WithWrap(value DeclarationValue[WrapMode]) PresentationDeclaration {
	if wrap, ok := value.Value(); ok && !validPresentationWrap(wrap) {
		value = DeclarationValue[WrapMode]{}
	}
	d.wrap = value
	return d
}

// Wrap returns the paragraph wrapping property.
func (d PresentationDeclaration) Wrap() DeclarationValue[WrapMode] {
	return d.wrap
}

// WithAlignment returns this declaration with a horizontal alignment property.
// An unknown concrete alignment is stored as unspecified.
func (d PresentationDeclaration) WithAlignment(value DeclarationValue[HorizontalAlignment]) PresentationDeclaration {
	if alignment, ok := value.Value(); ok && !validPresentationAlignment(alignment) {
		value = DeclarationValue[HorizontalAlignment]{}
	}
	d.alignment = value
	return d
}

// Alignment returns the horizontal alignment property.
func (d PresentationDeclaration) Alignment() DeclarationValue[HorizontalAlignment] {
	return d.alignment
}

// WithTextStyle returns this declaration with all supplied text properties.
func (d PresentationDeclaration) WithTextStyle(value TextStyleDeclaration) PresentationDeclaration {
	d.textStyle = value
	return d
}

// TextStyle returns the inheritable text property declaration.
func (d PresentationDeclaration) TextStyle() TextStyleDeclaration {
	return d.textStyle
}

// DuplicatePresentationStateError reports a repeated required state in one
// presentation rule.
type DuplicatePresentationStateError struct {
	// State is the repeated state.
	State PresentationState
}

// Error returns the duplicate-state diagnostic.
func (e *DuplicatePresentationStateError) Error() string {
	return "duplicate required presentation state " + e.State.String()
}

// PresentationRule pairs one selector and required state set with one
// declaration.
type PresentationRule struct {
	selector    PresentationSelector
	required    []PresentationState
	declaration PresentationDeclaration
}

// NewPresentationRule returns a rule with no required application state.
func NewPresentationRule(selector PresentationSelector, declaration PresentationDeclaration) PresentationRule {
	return PresentationRule{selector: selector, declaration: declaration}
}

// Requiring returns this rule with a replacement all-of required state set.
//
// The returned rule owns and preserves the supplied state sequence. Repeated
// states return DuplicatePresentationStateError.
func (r PresentationRule) Requiring(states ...PresentationState) (PresentationRule, error) {
	owned := append([]PresentationState(nil), states...)
	for index, state := range owned {
		for prior := range index {
			if owned[prior] == state {
				return r, &DuplicatePresentationStateError{State: state}
			}
		}
	}
	r.required = owned
	return r, nil
}

// Selector returns the rule's exact selector.
func (r PresentationRule) Selector() PresentationSelector {
	return r.selector
}

// RequiredStates returns a copy of the all-of required state set in input
// order.
func (r PresentationRule) RequiredStates() []PresentationState {
	return append([]PresentationState(nil), r.required...)
}

// Declaration returns the rule's property declaration.
func (r PresentationRule) Declaration() PresentationDeclaration {
	return r.declaration
}

func (r PresentationRule) clone() PresentationRule {
	r.required = append([]PresentationState(nil), r.required...)
	return r
}

func (r PresentationRule) matches(element content.Element, active []PresentationState) bool {
	if !r.selector.matches(element) {
		return false
	}
	for _, required := range r.required {
		if !containsPresentationState(active, required) {
			return false
		}
	}
	return true
}

func containsPresentationState(states []PresentationState, expected PresentationState) bool {
	if !expected.valid() {
		return false
	}
	for _, state := range states {
		if state.valid() && state == expected {
			return true
		}
	}
	return false
}

// PresentationSheet is one immutable source-ordered presentation rule set.
//
// Its zero value is an empty sheet.
type PresentationSheet struct {
	inner *presentationSheetData
}

type presentationSheetData struct {
	rules []PresentationRule
}

// NewPresentationSheet returns an immutable sheet that owns the supplied rule
// sequence.
func NewPresentationSheet(rules []PresentationRule) PresentationSheet {
	if len(rules) == 0 {
		return PresentationSheet{}
	}
	owned := make([]PresentationRule, len(rules))
	for index, rule := range rules {
		owned[index] = rule.clone()
	}
	return PresentationSheet{inner: &presentationSheetData{rules: owned}}
}

// Len returns the number of source-ordered rules.
func (s PresentationSheet) Len() int {
	if s.inner == nil {
		return 0
	}
	return len(s.inner.rules)
}

// Rule returns one rule by zero-based source-order index.
func (s PresentationSheet) Rule(index int) (PresentationRule, bool) {
	if s.inner == nil || index < 0 || index >= len(s.inner.rules) {
		return PresentationRule{}, false
	}
	return s.inner.rules[index].clone(), true
}

// Rules returns a copy of the source-ordered rules.
func (s PresentationSheet) Rules() []PresentationRule {
	if s.inner == nil {
		return nil
	}
	rules := make([]PresentationRule, len(s.inner.rules))
	for index, rule := range s.inner.rules {
		rules[index] = rule.clone()
	}
	return rules
}

// Resolve computes terminal presentation for one element and active state set.
//
// Text properties begin with inherited and layout properties begin with their
// element-specific initial values. Matching rules are applied once each in
// sheet source order.
func (s PresentationSheet) Resolve(
	element content.Element,
	inherited vt.Style,
	active []PresentationState,
) ComputedPresentation {
	initialDisplay := initialPresentationDisplay(element.Kind())
	computed := ComputedPresentation{
		display: initialDisplay,
		length:  Auto(),
		wrap:    WrapWord,
		align:   AlignStart,
		style:   inherited,
	}
	if s.inner == nil {
		return computed
	}
	for _, rule := range s.inner.rules {
		if !rule.matches(element, active) {
			continue
		}
		computed.apply(rule.declaration, initialDisplay)
	}
	return computed
}

func (s PresentationSheet) sharesStorage(other PresentationSheet) bool {
	return s.inner == other.inner
}

// ComputedPresentation contains resolved terminal text and layout values for
// one Content element.
//
// Its zero value is inline, Auto, zero gap, no visual separator, word wrapping,
// start alignment, and the terminal default Style.
type ComputedPresentation struct {
	display            PresentationDisplay
	length             Length
	gap                uint32
	visualSeparator    string
	hasVisualSeparator bool
	wrap               WrapMode
	align              HorizontalAlignment
	style              vt.Style
}

// Display returns the resolved terminal layout mode.
func (p ComputedPresentation) Display() PresentationDisplay {
	return p.display
}

// Length returns the resolved main-axis sizing rule.
func (p ComputedPresentation) Length() Length {
	return p.length
}

// Gap returns the resolved child gap in terminal cells.
func (p ComputedPresentation) Gap() uint32 {
	return p.gap
}

// VisualSeparator returns the resolved presentation-only separator and whether
// it is present.
func (p ComputedPresentation) VisualSeparator() (string, bool) {
	return p.visualSeparator, p.hasVisualSeparator
}

// Wrap returns the resolved paragraph wrapping mode.
func (p ComputedPresentation) Wrap() WrapMode {
	return p.wrap
}

// Alignment returns the resolved horizontal alignment.
func (p ComputedPresentation) Alignment() HorizontalAlignment {
	return p.align
}

// Style returns the resolved computed terminal Style.
func (p ComputedPresentation) Style() vt.Style {
	return p.style
}

func (p *ComputedPresentation) apply(declaration PresentationDeclaration, initialDisplay PresentationDisplay) {
	p.display = applyPresentationDisplay(p.display, initialDisplay, declaration.display)
	p.length = applyDeclarationValue(p.length, Auto(), declaration.length)
	p.gap = applyDeclarationValue(p.gap, uint32(0), declaration.gap)
	switch declaration.visualSeparator.state {
	case DeclarationValueSet:
		p.visualSeparator = declaration.visualSeparator.value
		p.hasVisualSeparator = true
	case DeclarationValueInitial:
		p.visualSeparator = ""
		p.hasVisualSeparator = false
	}
	p.wrap = applyPresentationWrap(p.wrap, declaration.wrap)
	p.align = applyPresentationAlignment(p.align, declaration.alignment)
	p.style = applyTextStyleDeclaration(p.style, declaration.textStyle)
}

func applyPresentationDisplay(
	current, initial PresentationDisplay,
	declaration DeclarationValue[PresentationDisplay],
) PresentationDisplay {
	switch declaration.state {
	case DeclarationValueSet:
		if validPresentationDisplay(declaration.value) {
			return declaration.value
		}
	case DeclarationValueInitial:
		return initial
	}
	return current
}

func validPresentationDisplay(display PresentationDisplay) bool {
	return display <= PresentationDisplaySequence
}

func applyPresentationWrap(current WrapMode, declaration DeclarationValue[WrapMode]) WrapMode {
	switch declaration.state {
	case DeclarationValueSet:
		if validPresentationWrap(declaration.value) {
			return declaration.value
		}
	case DeclarationValueInitial:
		return WrapWord
	}
	return current
}

func validPresentationWrap(wrap WrapMode) bool {
	return wrap <= WrapNone
}

func applyPresentationAlignment(
	current HorizontalAlignment,
	declaration DeclarationValue[HorizontalAlignment],
) HorizontalAlignment {
	switch declaration.state {
	case DeclarationValueSet:
		if validPresentationAlignment(declaration.value) {
			return declaration.value
		}
	case DeclarationValueInitial:
		return AlignStart
	}
	return current
}

func validPresentationAlignment(alignment HorizontalAlignment) bool {
	return alignment <= AlignEnd
}

func applyTextStyleDeclaration(style vt.Style, declaration TextStyleDeclaration) vt.Style {
	style.Foreground = applyDeclarationValue(style.Foreground, vt.DefaultColor(), declaration.foreground)
	style.Background = applyDeclarationValue(style.Background, vt.DefaultColor(), declaration.background)
	style.UnderlineColor = applyDeclarationValue(style.UnderlineColor, vt.OptionalColor{}, declaration.underlineColor)
	style.Bold = applyDeclarationValue(style.Bold, false, declaration.bold)
	style.Dim = applyDeclarationValue(style.Dim, false, declaration.dim)
	style.Italic = applyDeclarationValue(style.Italic, false, declaration.italic)
	style.Underline = applyDeclarationValue(style.Underline, false, declaration.underline)
	style.Blink = applyDeclarationValue(style.Blink, false, declaration.blink)
	style.Reverse = applyDeclarationValue(style.Reverse, false, declaration.reverse)
	style.Hidden = applyDeclarationValue(style.Hidden, false, declaration.hidden)
	style.Strikethrough = applyDeclarationValue(style.Strikethrough, false, declaration.strikethrough)
	return style
}

func applyDeclarationValue[T any](current, initial T, declaration DeclarationValue[T]) T {
	switch declaration.state {
	case DeclarationValueSet:
		return declaration.value
	case DeclarationValueInitial:
		return initial
	default:
		return current
	}
}

func initialPresentationDisplay(kind content.ElementKind) PresentationDisplay {
	switch kind {
	case content.ElementFlow:
		return PresentationDisplayFlow
	case content.ElementParagraph:
		return PresentationDisplayParagraph
	case content.ElementSequence:
		return PresentationDisplaySequence
	default:
		return PresentationDisplayInline
	}
}
