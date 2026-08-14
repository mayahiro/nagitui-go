package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mayahiro/nagi-go/content"
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go/internal/conformance"
)

func TestPresentationStateFixtures(t *testing.T) {
	records := presentationRecords(
		t, "presentation/states.txt", "presentation-states", "value", "expected",
	)
	for _, record := range records {
		_, err := NewPresentationStateBytes(record.Bytes("value"))
		actual := "ok"
		if err != nil {
			var identifierError *content.IdentifierError
			if !errors.As(err, &identifierError) {
				t.Fatalf("case %s: unexpected error %v", record.ID, err)
			}
			actual = identifierError.Kind().String()
		}
		if want := record.Field("expected"); actual != want {
			t.Errorf("case %s: state result = %q, want %q", record.ID, actual, want)
		}
	}
}

func TestPresentationSelectorFixtures(t *testing.T) {
	records := presentationRecords(
		t, "presentation/selectors.txt", "presentation-selectors",
		"selector", "roles", "classes", "required", "active", "expected",
	)
	for _, record := range records {
		element := presentationFixtureElement(
			t, "inline", record.Field("roles"), record.Field("classes"),
		)
		selector := presentationFixtureSelector(t, record.Field("selector"))
		rule, err := NewPresentationRule(
			selector,
			PresentationDeclaration{}.WithTextStyle(
				TextStyleDeclaration{}.WithBold(SetDeclarationValue(true)),
			),
		).Requiring(presentationFixtureStates(t, record.Field("required"))...)
		if err != nil {
			t.Fatalf("case %s: Requiring() error = %v", record.ID, err)
		}
		computed := NewPresentationSheet([]PresentationRule{rule}).Resolve(
			element,
			vt.Style{},
			presentationFixtureStates(t, record.Field("active")),
		)
		actual := "miss"
		if computed.Style().Bold {
			actual = "match"
		}
		if want := record.Field("expected"); actual != want {
			t.Errorf("case %s: selector result = %q, want %q", record.ID, actual, want)
		}
	}
}

func TestPresentationCascadeFixtures(t *testing.T) {
	records := presentationRecords(
		t, "presentation/cascade.txt", "presentation-cascade",
		"element", "roles", "classes", "states", "inherited", "rules", "expected",
	)
	for _, record := range records {
		element := presentationFixtureElement(
			t, record.Field("element"), record.Field("roles"), record.Field("classes"),
		)
		computed := presentationFixtureSheet(t, record.Field("rules")).Resolve(
			element,
			presentationFixtureInherited(record.Field("inherited")),
			presentationFixtureStates(t, record.Field("states")),
		)
		if actual, want := presentationComputedSignature(computed), record.Text("expected"); actual != want {
			t.Errorf("case %s: computed = %q, want %q", record.ID, actual, want)
		}
	}
}

func presentationRecords(
	t *testing.T,
	relative string,
	suite string,
	fields ...string,
) []conformance.Record {
	t.Helper()
	records, err := conformance.Load(relative, suite, fields...)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	return records
}

func presentationFixtureElement(t *testing.T, kind, roleList, classList string) content.Element {
	t.Helper()
	var element content.Element
	switch kind {
	case "inline":
		element = content.NewInline(nil)
	case "flow":
		element = content.NewFlow(nil)
	case "paragraph":
		element = content.NewParagraph(nil)
	case "sequence":
		element = content.NewSequence(nil)
	default:
		t.Fatalf("unknown presentation element kind %q", kind)
	}

	roleValues := presentationFixtureList(roleList)
	roles := make([]content.Role, len(roleValues))
	for index, value := range roleValues {
		roles[index] = mustPresentationRole(t, value)
	}
	var err error
	element, err = element.WithRoles(roles)
	if err != nil {
		t.Fatal(err)
	}
	classValues := presentationFixtureList(classList)
	classes := make([]content.Class, len(classValues))
	for index, value := range classValues {
		classes[index] = mustPresentationClass(t, value)
	}
	element, err = element.WithClasses(classes)
	if err != nil {
		t.Fatal(err)
	}
	return element
}

func presentationFixtureSelector(t *testing.T, value string) PresentationSelector {
	t.Helper()
	if value == "any" {
		return AnyPresentationSelector()
	}
	kind, token, ok := strings.Cut(value, ":")
	if !ok {
		t.Fatalf("invalid presentation selector %q", value)
	}
	switch kind {
	case "role":
		return RolePresentationSelector(mustPresentationRole(t, token))
	case "class":
		return ClassPresentationSelector(mustPresentationClass(t, token))
	default:
		t.Fatalf("unknown presentation selector %q", value)
		return PresentationSelector{}
	}
}

func presentationFixtureStates(t *testing.T, value string) []PresentationState {
	t.Helper()
	values := presentationFixtureList(value)
	states := make([]PresentationState, len(values))
	for index, token := range values {
		states[index] = mustPresentationState(t, token)
	}
	return states
}

func presentationFixtureList(value string) []string {
	if value == "none" {
		return nil
	}
	return strings.Split(value, ",")
}

func presentationFixtureSheet(t *testing.T, name string) PresentationSheet {
	t.Helper()
	if name == "none" {
		return PresentationSheet{}
	}
	heading := mustPresentationRole(t, "heading")
	item := mustPresentationRole(t, "item")
	code := mustPresentationRole(t, "code")
	reset := mustPresentationRole(t, "reset")
	underline := mustPresentationRole(t, "underline")
	layout := mustPresentationRole(t, "layout")
	separatorRole := mustPresentationRole(t, "separator")
	sourceBold := mustPresentationClass(t, "source.bold")
	resetClass := mustPresentationClass(t, "reset")
	selected := mustPresentationState(t, "selected")
	focused := mustPresentationState(t, "focused")
	disabled := mustPresentationState(t, "disabled")

	bold := PresentationDeclaration{}.WithTextStyle(
		TextStyleDeclaration{}.WithBold(SetDeclarationValue(true)),
	)
	headingDeclaration := PresentationDeclaration{}.WithTextStyle(
		TextStyleDeclaration{}.
			WithForeground(SetDeclarationValue(vt.IndexedColor(6))).
			WithBold(SetDeclarationValue(true)),
	)
	sourceDeclaration := PresentationDeclaration{}.WithTextStyle(
		TextStyleDeclaration{}.
			WithForeground(SetDeclarationValue(vt.IndexedColor(3))).
			WithBold(SetDeclarationValue(true)),
	)

	var rules []PresentationRule
	switch name {
	case "universal":
		rules = []PresentationRule{NewPresentationRule(AnyPresentationSelector(), bold)}
	case "heading":
		rules = []PresentationRule{
			NewPresentationRule(RolePresentationSelector(heading), headingDeclaration),
		}
	case "source-class":
		rules = []PresentationRule{
			NewPresentationRule(ClassPresentationSelector(sourceBold), sourceDeclaration),
		}
	case "stateful":
		rule, err := NewPresentationRule(
			RolePresentationSelector(item),
			PresentationDeclaration{}.WithTextStyle(
				TextStyleDeclaration{}.WithReverse(SetDeclarationValue(true)),
			),
		).Requiring(selected, focused)
		if err != nil {
			t.Fatal(err)
		}
		rules = []PresentationRule{rule}
	case "ordered":
		disabledRule, err := NewPresentationRule(
			RolePresentationSelector(code),
			PresentationDeclaration{}.WithTextStyle(
				TextStyleDeclaration{}.
					WithForeground(SetDeclarationValue(vt.DefaultColor())).
					WithItalic(SetDeclarationValue(false)),
			),
		).Requiring(disabled)
		if err != nil {
			t.Fatal(err)
		}
		rules = []PresentationRule{
			NewPresentationRule(
				AnyPresentationSelector(),
				PresentationDeclaration{}.WithTextStyle(
					TextStyleDeclaration{}.
						WithForeground(SetDeclarationValue(vt.IndexedColor(1))).
						WithBold(SetDeclarationValue(true)),
				),
			),
			NewPresentationRule(
				RolePresentationSelector(code),
				PresentationDeclaration{}.WithTextStyle(
					TextStyleDeclaration{}.
						WithForeground(SetDeclarationValue(vt.IndexedColor(2))).
						WithItalic(SetDeclarationValue(true)),
				),
			),
			NewPresentationRule(
				ClassPresentationSelector(sourceBold),
				PresentationDeclaration{}.WithTextStyle(
					TextStyleDeclaration{}.
						WithBold(SetDeclarationValue(false)).
						WithDim(SetDeclarationValue(true)),
				),
			),
			disabledRule,
		}
	case "text-initial":
		initialBoolean := InitialDeclarationValue[bool]()
		text := TextStyleDeclaration{}.
			WithForeground(InitialDeclarationValue[vt.Color]()).
			WithBackground(InitialDeclarationValue[vt.Color]()).
			WithUnderlineColor(InitialDeclarationValue[vt.OptionalColor]()).
			WithBold(initialBoolean).
			WithDim(initialBoolean).
			WithItalic(initialBoolean).
			WithUnderline(initialBoolean).
			WithBlink(initialBoolean).
			WithReverse(initialBoolean).
			WithHidden(initialBoolean).
			WithStrikethrough(initialBoolean)
		rules = []PresentationRule{
			NewPresentationRule(
				RolePresentationSelector(reset),
				PresentationDeclaration{}.WithTextStyle(text),
			),
		}
	case "underline-default":
		rules = []PresentationRule{
			NewPresentationRule(
				RolePresentationSelector(underline),
				PresentationDeclaration{}.WithTextStyle(
					TextStyleDeclaration{}.
						WithUnderlineColor(SetDeclarationValue(vt.SomeColor(vt.DefaultColor()))).
						WithUnderline(SetDeclarationValue(true)),
				),
			),
		}
	case "layout-set", "layout-initial":
		setLayout := PresentationDeclaration{}.
			WithDisplay(SetDeclarationValue(PresentationDisplaySequence)).
			WithLength(SetDeclarationValue(Fixed(3))).
			WithGap(SetDeclarationValue(uint32(2))).
			WithVisualSeparator(SetDeclarationValue(" | ")).
			WithWrap(SetDeclarationValue(WrapNone)).
			WithAlignment(SetDeclarationValue(AlignEnd))
		rules = []PresentationRule{
			NewPresentationRule(RolePresentationSelector(layout), setLayout),
		}
		if name == "layout-initial" {
			initialLayout := PresentationDeclaration{}.
				WithDisplay(InitialDeclarationValue[PresentationDisplay]()).
				WithLength(InitialDeclarationValue[Length]()).
				WithGap(InitialDeclarationValue[uint32]()).
				WithVisualSeparator(InitialDeclarationValue[string]()).
				WithWrap(InitialDeclarationValue[WrapMode]()).
				WithAlignment(InitialDeclarationValue[HorizontalAlignment]())
			rules = append(rules,
				NewPresentationRule(ClassPresentationSelector(resetClass), initialLayout),
			)
		}
	case "unspecified":
		rules = []PresentationRule{
			NewPresentationRule(RolePresentationSelector(heading), headingDeclaration),
			NewPresentationRule(AnyPresentationSelector(), PresentationDeclaration{}),
		}
	case "separator-empty":
		rules = []PresentationRule{
			NewPresentationRule(
				RolePresentationSelector(separatorRole),
				PresentationDeclaration{}.WithVisualSeparator(SetDeclarationValue("")),
			),
		}
	case "separator-invalid":
		rules = []PresentationRule{
			NewPresentationRule(
				RolePresentationSelector(separatorRole),
				PresentationDeclaration{}.WithVisualSeparator(
					SetDeclarationValue(string([]byte{0xff})),
				),
			),
		}
	default:
		t.Fatalf("unknown presentation rule arrangement %q", name)
	}
	return NewPresentationSheet(rules)
}

func presentationFixtureInherited(name string) vt.Style {
	switch name {
	case "default":
		return vt.Style{}
	case "accent":
		return vt.Style{
			Foreground:     vt.IndexedColor(2),
			Background:     vt.IndexedColor(4),
			UnderlineColor: vt.SomeColor(vt.IndexedColor(3)),
			Bold:           true,
			Italic:         true,
		}
	default:
		panic("unknown presentation inherited style " + name)
	}
}

func presentationComputedSignature(computed ComputedPresentation) string {
	return strings.Join([]string{
		presentationDisplayName(computed.Display()),
		presentationLengthName(computed.Length()),
		fmt.Sprintf("%d", computed.Gap()),
		presentationVisualSeparatorName(computed),
		presentationWrapName(computed.Wrap()),
		presentationAlignmentName(computed.Alignment()),
		presentationStyleName(computed.Style()),
	}, "|")
}

func presentationDisplayName(display PresentationDisplay) string {
	switch display {
	case PresentationDisplayInline:
		return "inline"
	case PresentationDisplayFlow:
		return "flow"
	case PresentationDisplayParagraph:
		return "paragraph"
	case PresentationDisplaySequence:
		return "sequence"
	default:
		return "invalid"
	}
}

func presentationLengthName(length Length) string {
	switch length.Kind() {
	case LengthAuto:
		return "auto"
	case LengthFixed:
		value, _ := length.Value()
		return fmt.Sprintf("fixed:%d", value)
	default:
		return "unexpected"
	}
}

func presentationVisualSeparatorName(computed ComputedPresentation) string {
	separator, present := computed.VisualSeparator()
	if !present {
		return "absent"
	}
	const hex = "0123456789ABCDEF"
	var encoded strings.Builder
	encoded.Grow(len("present:") + len(separator)*2)
	encoded.WriteString("present:")
	for _, value := range []byte(separator) {
		encoded.WriteByte(hex[value>>4])
		encoded.WriteByte(hex[value&0x0F])
	}
	return encoded.String()
}

func presentationWrapName(wrap WrapMode) string {
	switch wrap {
	case WrapWord:
		return "word"
	case WrapHard:
		return "hard"
	case WrapNone:
		return "none"
	default:
		return "invalid"
	}
}

func presentationAlignmentName(alignment HorizontalAlignment) string {
	switch alignment {
	case AlignStart:
		return "start"
	case AlignCenter:
		return "center"
	case AlignEnd:
		return "end"
	default:
		return "invalid"
	}
}

func presentationStyleName(style vt.Style) string {
	styles := map[string]vt.Style{
		"default": {},
		"accent":  presentationFixtureInherited("accent"),
		"bold":    {Bold: true},
		"heading": {Foreground: vt.IndexedColor(6), Bold: true},
		"source":  {Foreground: vt.IndexedColor(3), Bold: true},
		"selected": {
			Reverse: true,
		},
		"ordered": {
			Dim: true,
		},
		"underline-default": {
			UnderlineColor: vt.SomeColor(vt.DefaultColor()), Underline: true,
		},
	}
	for name, expected := range styles {
		if style == expected {
			return name
		}
	}
	return fmt.Sprintf("unknown:%+v", style)
}
