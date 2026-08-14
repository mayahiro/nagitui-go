package tui

import (
	"errors"
	"runtime"
	"testing"

	"github.com/mayahiro/nagi-go/content"
	"github.com/mayahiro/nagi-go/vt"
)

func TestDeclarationValueStates(t *testing.T) {
	var unspecified DeclarationValue[int]
	if unspecified.State() != DeclarationValueUnspecified {
		t.Fatalf("zero State() = %v, want unspecified", unspecified.State())
	}
	if _, ok := unspecified.Value(); ok {
		t.Fatal("zero Value() unexpectedly returned a concrete value")
	}

	set := SetDeclarationValue(42)
	if set.State() != DeclarationValueSet {
		t.Fatalf("set State() = %v, want set", set.State())
	}
	if value, ok := set.Value(); !ok || value != 42 {
		t.Fatalf("set Value() = %d, %t, want 42, true", value, ok)
	}

	initial := InitialDeclarationValue[int]()
	if initial.State() != DeclarationValueInitial {
		t.Fatalf("initial State() = %v, want initial", initial.State())
	}
	if _, ok := initial.Value(); ok {
		t.Fatal("initial Value() unexpectedly returned a concrete value")
	}
}

func TestPresentationCascadeCanClearEveryTextProperty(t *testing.T) {
	role := mustPresentationRole(t, "reset")
	element, err := content.NewInline(nil).WithRoles([]content.Role{role})
	if err != nil {
		t.Fatal(err)
	}
	initial := InitialDeclarationValue[bool]()
	text := TextStyleDeclaration{}.
		WithForeground(InitialDeclarationValue[vt.Color]()).
		WithBackground(InitialDeclarationValue[vt.Color]()).
		WithUnderlineColor(InitialDeclarationValue[vt.OptionalColor]()).
		WithBold(initial).
		WithDim(initial).
		WithItalic(initial).
		WithUnderline(initial).
		WithBlink(initial).
		WithReverse(initial).
		WithHidden(initial).
		WithStrikethrough(initial)
	declaration := PresentationDeclaration{}.WithTextStyle(text)
	sheet := NewPresentationSheet([]PresentationRule{
		NewPresentationRule(RolePresentationSelector(role), declaration),
	})
	inherited := vt.Style{
		Foreground:     vt.IndexedColor(1),
		Background:     vt.IndexedColor(2),
		UnderlineColor: vt.SomeColor(vt.IndexedColor(3)),
		Bold:           true,
		Dim:            true,
		Italic:         true,
		Underline:      true,
		Blink:          true,
		Reverse:        true,
		Hidden:         true,
		Strikethrough:  true,
	}

	computed := sheet.Resolve(element, inherited, nil)
	if computed.Style() != (vt.Style{}) {
		t.Fatalf("Style() = %+v, want terminal default", computed.Style())
	}
}

func TestPresentationRequiredStatesAndInvalidZeroState(t *testing.T) {
	selected := mustPresentationState(t, "selected")
	declaration := PresentationDeclaration{}.WithTextStyle(
		TextStyleDeclaration{}.WithReverse(SetDeclarationValue(true)),
	)
	rule := NewPresentationRule(AnyPresentationSelector(), declaration)
	if _, err := rule.Requiring(selected, selected); err == nil {
		t.Fatal("Requiring() accepted duplicate states")
	} else {
		var duplicate *DuplicatePresentationStateError
		if !errors.As(err, &duplicate) || duplicate.State != selected {
			t.Fatalf("Requiring() error = %v, want duplicate selected state", err)
		}
	}

	invalidRule, err := rule.Requiring(PresentationState{})
	if err != nil {
		t.Fatal(err)
	}
	invalidSheet := NewPresentationSheet([]PresentationRule{invalidRule})
	if got := invalidSheet.Resolve(content.Element{}, vt.Style{}, []PresentationState{{}}).Style(); got.Reverse {
		t.Fatal("zero required and active states matched")
	}

	selectedRule, err := rule.Requiring(selected)
	if err != nil {
		t.Fatal(err)
	}
	selectedSheet := NewPresentationSheet([]PresentationRule{selectedRule})
	if got := selectedSheet.Resolve(content.Element{}, vt.Style{}, []PresentationState{{}}).Style(); got.Reverse {
		t.Fatal("zero active state matched a valid required state")
	}
	if got := selectedSheet.Resolve(
		content.Element{}, vt.Style{}, []PresentationState{{}, selected},
	).Style(); !got.Reverse {
		t.Fatal("valid active state did not match beside a zero state")
	}
}

func TestPresentationSheetOwnsInputsAndCopiesShareStorage(t *testing.T) {
	selected := mustPresentationState(t, "selected")
	required := []PresentationState{selected}
	declaration := PresentationDeclaration{}.
		WithVisualSeparator(SetDeclarationValue(" | ")).
		WithTextStyle(TextStyleDeclaration{}.WithBold(SetDeclarationValue(true)))
	rule, err := NewPresentationRule(AnyPresentationSelector(), declaration).Requiring(required...)
	if err != nil {
		t.Fatal(err)
	}
	required[0] = PresentationState{}
	rules := []PresentationRule{rule}
	sheet := NewPresentationSheet(rules)
	copyOfSheet := sheet
	if !sheet.sharesStorage(copyOfSheet) {
		t.Fatal("PresentationSheet copy did not share immutable storage")
	}
	rules[0] = NewPresentationRule(AnyPresentationSelector(), PresentationDeclaration{})

	returnedRule, ok := sheet.Rule(0)
	if !ok {
		t.Fatal("Rule(0) did not return the stored rule")
	}
	returnedRequired := returnedRule.RequiredStates()
	returnedRequired[0] = PresentationState{}
	returnedRules := sheet.Rules()
	returnedRules[0] = PresentationRule{}

	computed := sheet.Resolve(content.Element{}, vt.Style{}, []PresentationState{selected})
	if !computed.Style().Bold {
		t.Fatal("input or returned slice mutation changed the stored rule")
	}
	if separator, present := computed.VisualSeparator(); !present || separator != " | " {
		t.Fatalf("VisualSeparator() = %q, %t, want visual separator", separator, present)
	}
	if sheet.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", sheet.Len())
	}
	if _, ok := sheet.Rule(-1); ok {
		t.Fatal("Rule(-1) unexpectedly returned a rule")
	}
	if _, ok := sheet.Rule(1); ok {
		t.Fatal("Rule(1) unexpectedly returned a rule")
	}
}

func TestPresentationRulePreservesRequiredStateOrder(t *testing.T) {
	focused := mustPresentationState(t, "focused")
	selected := mustPresentationState(t, "selected")
	input := []PresentationState{focused, selected}
	rule, err := NewPresentationRule(
		AnyPresentationSelector(), PresentationDeclaration{},
	).Requiring(input...)
	if err != nil {
		t.Fatal(err)
	}
	input[0] = PresentationState{}

	storedRule, ok := NewPresentationSheet([]PresentationRule{rule}).Rule(0)
	if !ok {
		t.Fatal("Rule(0) did not return the stored ordered states")
	}
	required := storedRule.RequiredStates()
	if len(required) != 2 || required[0] != focused || required[1] != selected {
		t.Fatalf("RequiredStates() = %v, want focused, selected", required)
	}
}

func TestPresentationUnknownEnumsAreUnspecified(t *testing.T) {
	invalid := PresentationDeclaration{}.
		WithDisplay(SetDeclarationValue(PresentationDisplay(255))).
		WithWrap(SetDeclarationValue(WrapMode(255))).
		WithAlignment(SetDeclarationValue(HorizontalAlignment(255)))
	if invalid.Display().State() != DeclarationValueUnspecified ||
		invalid.Wrap().State() != DeclarationValueUnspecified ||
		invalid.Alignment().State() != DeclarationValueUnspecified {
		t.Fatal("unknown concrete enum was not stored as unspecified")
	}
	valid := PresentationDeclaration{}.
		WithDisplay(SetDeclarationValue(PresentationDisplaySequence)).
		WithWrap(SetDeclarationValue(WrapNone)).
		WithAlignment(SetDeclarationValue(AlignEnd))
	orders := [][]PresentationDeclaration{
		{invalid, valid},
		{valid, invalid},
	}
	for _, order := range orders {
		rules := make([]PresentationRule, len(order))
		for index, declaration := range order {
			rules[index] = NewPresentationRule(AnyPresentationSelector(), declaration)
		}
		computed := NewPresentationSheet(rules).Resolve(content.NewFlow(nil), vt.Style{}, nil)
		if computed.Display() != PresentationDisplaySequence ||
			computed.Wrap() != WrapNone || computed.Alignment() != AlignEnd {
			t.Fatalf(
				"unknown enum changed computed presentation: display=%v wrap=%v alignment=%v",
				computed.Display(), computed.Wrap(), computed.Alignment(),
			)
		}
	}
}

func TestPresentationVisualSeparatorNormalizesInvalidUTF8(t *testing.T) {
	declaration := PresentationDeclaration{}.WithVisualSeparator(
		SetDeclarationValue(string([]byte{'A', 0xff, 0xfe, 'B'})),
	)
	declared, ok := declaration.VisualSeparator().Value()
	if !ok || declared != "A\uFFFDB" {
		t.Fatalf("declared VisualSeparator() = %q, %t, want normalized text", declared, ok)
	}
	computed := NewPresentationSheet([]PresentationRule{
		NewPresentationRule(AnyPresentationSelector(), declaration),
	}).Resolve(content.Element{}, vt.Style{}, nil)
	if separator, present := computed.VisualSeparator(); !present || separator != "A\uFFFDB" {
		t.Fatalf("computed VisualSeparator() = %q, %t, want normalized text", separator, present)
	}
}

func TestPresentationResolveWarmedAllocations(t *testing.T) {
	role := mustPresentationRole(t, "item")
	class := mustPresentationClass(t, "source.bold")
	selected := mustPresentationState(t, "selected")
	element, err := content.NewInline(nil).WithRoles([]content.Role{role})
	if err != nil {
		t.Fatal(err)
	}
	element, err = element.WithClasses([]content.Class{class})
	if err != nil {
		t.Fatal(err)
	}
	stateful, err := NewPresentationRule(
		RolePresentationSelector(role),
		PresentationDeclaration{}.WithTextStyle(
			TextStyleDeclaration{}.WithReverse(SetDeclarationValue(true)),
		),
	).Requiring(selected)
	if err != nil {
		t.Fatal(err)
	}
	sheet := NewPresentationSheet([]PresentationRule{
		NewPresentationRule(
			AnyPresentationSelector(),
			PresentationDeclaration{}.WithTextStyle(
				TextStyleDeclaration{}.WithForeground(SetDeclarationValue(vt.IndexedColor(1))),
			),
		),
		NewPresentationRule(
			ClassPresentationSelector(class),
			PresentationDeclaration{}.WithTextStyle(
				TextStyleDeclaration{}.WithBold(SetDeclarationValue(true)),
			),
		),
		stateful,
	})
	active := []PresentationState{selected}
	sheet.Resolve(element, vt.Style{}, active)

	allocations := testing.AllocsPerRun(1_000, func() {
		computed := sheet.Resolve(element, vt.Style{}, active)
		if !computed.Style().Bold || !computed.Style().Reverse {
			panic("unexpected computed presentation")
		}
		runtime.KeepAlive(computed)
	})
	if allocations != 0 {
		t.Fatalf("Resolve allocations = %f, want 0", allocations)
	}
}

func mustPresentationState(t *testing.T, value string) PresentationState {
	t.Helper()
	state, err := NewPresentationState(value)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func mustPresentationRole(t *testing.T, value string) content.Role {
	t.Helper()
	role, err := content.NewRole(value)
	if err != nil {
		t.Fatal(err)
	}
	return role
}

func mustPresentationClass(t *testing.T, value string) content.Class {
	t.Helper()
	class, err := content.NewClass(value)
	if err != nil {
		t.Fatal(err)
	}
	return class
}
