package tui

import (
	"errors"
	"strconv"
	"testing"

	"github.com/mayahiro/nagi-go/content"
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go/internal/conformance"
)

type contentProjectionFixture struct {
	content content.Content
	sheet   PresentationSheet
	options ContentProjectionOptions
	states  []PresentationState
}

type contentProjectionFixtureApp struct {
	fixture contentProjectionFixture
}

func (*contentProjectionFixtureApp) Init() Effect[struct{}] { return NoneEffect[struct{}]() }
func (*contentProjectionFixtureApp) Update(struct{}) Effect[struct{}] {
	return NoneEffect[struct{}]()
}
func (*contentProjectionFixtureApp) Subscriptions() Subscription[struct{}] {
	return NoneSubscription[struct{}]()
}
func (a *contentProjectionFixtureApp) View(ViewContext) Node[struct{}] {
	node, err := ProjectContentWithStates[struct{}](
		a.fixture.content,
		a.fixture.sheet,
		a.fixture.options,
		func(content.Element) []PresentationState { return a.fixture.states },
	)
	if err != nil {
		panic(err)
	}
	return node
}

func TestContentProjectionFixtures(t *testing.T) {
	records, err := conformance.Load(
		"presentation/projection.txt",
		"content-node-projection",
		"width",
		"height",
		"expected-kind",
		"expected-depth",
		"expected",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		fixture := newContentProjectionFixture(t, record.ID)
		_, err := ProjectContentWithStates[struct{}](
			fixture.content,
			fixture.sheet,
			fixture.options,
			func(content.Element) []PresentationState { return fixture.states },
		)
		expectedKind := record.Field("expected-kind")
		if expectedKind != "ok" {
			var projectionError *ContentProjectionError
			if !errors.As(err, &projectionError) {
				t.Fatalf("case %s error = %v", record.ID, err)
			}
			if projectionError.Kind().String() != expectedKind {
				t.Fatalf("case %s kind = %s, want %s", record.ID, projectionError.Kind(), expectedKind)
			}
			if projectionError.Depth() != projectionFixtureNumber(t, record.Field("expected-depth")) {
				t.Fatalf("case %s depth = %d", record.ID, projectionError.Depth())
			}
			continue
		}
		if err != nil {
			t.Fatalf("case %s error = %v", record.ID, err)
		}
		runtime, err := NewRuntimeWithClock[struct{}](
			&contentProjectionFixtureApp{fixture: fixture},
			NewRuntimeConfig(Size{
				Width:  projectionFixtureNumber(t, record.Field("width")),
				Height: projectionFixtureNumber(t, record.Field("height")),
			}),
			NewVirtualClock(),
		)
		if err != nil {
			t.Fatal(err)
		}
		frame, err := runtime.RenderIfDirty()
		if err != nil {
			t.Fatalf("case %s render error = %v", record.ID, err)
		}
		actual := frame.Surface().Snapshot()
		if expected := record.Text("expected"); actual != expected {
			t.Fatalf("case %s snapshot mismatch\ngot:\n%s\nwant:\n%s", record.ID, actual, expected)
		}
	}
}

func newContentProjectionFixture(t *testing.T, caseID string) contentProjectionFixture {
	t.Helper()
	switch caseID {
	case "paragraph-inline-style":
		paragraph := projectionFixtureRole(t, "paragraph")
		emphasis := projectionFixtureRole(t, "emphasis")
		inline := projectionFixtureRoles(
			t,
			content.NewInline([]content.Content{content.NewText("日")}),
			emphasis,
		)
		root := projectionFixtureRoles(
			t,
			content.NewParagraph([]content.Content{
				content.NewText("A"),
				inline.Content(),
				content.NewHardBreak(),
				content.NewText("B"),
			}),
			paragraph,
		)
		rules := []PresentationRule{
			NewPresentationRule(
				RolePresentationSelector(paragraph),
				PresentationDeclaration{}.
					WithVisualSeparator(SetDeclarationValue("|")).
					WithWrap(SetDeclarationValue(WrapHard)).
					WithAlignment(SetDeclarationValue(AlignCenter)),
			),
			NewPresentationRule(
				RolePresentationSelector(emphasis),
				PresentationDeclaration{}.WithTextStyle(
					TextStyleDeclaration{}.
						WithBold(SetDeclarationValue(false)).
						WithUnderline(SetDeclarationValue(true)),
				),
			),
		}
		return contentProjectionFixture{
			content: root.Content(),
			sheet:   NewPresentationSheet(rules),
			options: ContentProjectionOptions{}.WithBaseStyle(vt.Style{Bold: true}),
		}
	case "flow-boundaries":
		layout := projectionFixtureRole(t, "layout")
		root := projectionFixtureRoles(t, content.NewFlow([]content.Content{
			content.NewParagraph([]content.Content{content.NewText("A")}).Content(),
			content.NewInline([]content.Content{content.NewText("B")}).Content(),
			content.NewParagraph([]content.Content{content.NewText("C")}).Content(),
		}), layout)
		rule := NewPresentationRule(
			RolePresentationSelector(layout),
			PresentationDeclaration{}.
				WithGap(SetDeclarationValue(uint32(1))).
				WithVisualSeparator(SetDeclarationValue("-")).
				WithTextStyle(TextStyleDeclaration{}.WithDim(SetDeclarationValue(true))),
		)
		return contentProjectionFixture{content: root.Content(), sheet: NewPresentationSheet([]PresentationRule{rule})}
	case "sequence-boundaries":
		layout := projectionFixtureRole(t, "layout")
		root := projectionFixtureRoles(t, content.NewSequence([]content.Content{
			content.NewText("A"),
			content.NewParagraph([]content.Content{content.NewText("B")}).Content(),
			content.NewInline([]content.Content{content.NewText("C")}).Content(),
		}), layout)
		rule := NewPresentationRule(
			RolePresentationSelector(layout),
			PresentationDeclaration{}.
				WithGap(SetDeclarationValue(uint32(1))).
				WithVisualSeparator(SetDeclarationValue("|")),
		)
		return contentProjectionFixture{content: root.Content(), sheet: NewPresentationSheet([]PresentationRule{rule})}
	case "state-display-override":
		overrideRole := projectionFixtureRole(t, "override")
		selected, _ := NewPresentationState("selected")
		root := projectionFixtureRoles(t, content.NewFlow([]content.Content{
			content.NewParagraph([]content.Content{content.NewText("A")}).Content(),
			content.NewParagraph([]content.Content{content.NewText("B")}).Content(),
		}), overrideRole)
		rule, err := NewPresentationRule(
			RolePresentationSelector(overrideRole),
			PresentationDeclaration{}.
				WithDisplay(SetDeclarationValue(PresentationDisplaySequence)).
				WithVisualSeparator(SetDeclarationValue("/")),
		).Requiring(selected)
		if err != nil {
			t.Fatal(err)
		}
		return contentProjectionFixture{
			content: root.Content(),
			sheet:   NewPresentationSheet([]PresentationRule{rule}),
			options: ContentProjectionOptions{}.WithBaseStyle(vt.Style{Italic: true}),
			states:  []PresentationState{selected},
		}
	case "identity-annotation-boundary":
		id, _ := content.NewElementID("same")
		annotation, _ := content.NewAnnotationID("action")
		child := func(value string) content.Content {
			element, err := content.NewParagraph([]content.Content{content.NewText(value)}).WithID(id)
			if err != nil {
				t.Fatal(err)
			}
			element, err = element.WithAnnotation(annotation)
			if err != nil {
				t.Fatal(err)
			}
			return element.Content()
		}
		return contentProjectionFixture{content: content.NewFlow([]content.Content{child("A"), child("B")}).Content()}
	case "paragraph-length":
		tall := projectionFixtureRole(t, "tall")
		first := projectionFixtureRoles(
			t,
			content.NewParagraph([]content.Content{content.NewText("A")}),
			tall,
		)
		rule := NewPresentationRule(
			RolePresentationSelector(tall),
			PresentationDeclaration{}.WithLength(SetDeclarationValue(Fixed(2))),
		)
		return contentProjectionFixture{
			content: content.NewFlow([]content.Content{
				first.Content(),
				content.NewParagraph([]content.Content{content.NewText("B")}).Content(),
			}).Content(),
			sheet: NewPresentationSheet([]PresentationRule{rule}),
		}
	case "inline-layout-ignored":
		inlineRole := projectionFixtureRole(t, "inline-layout")
		inline := projectionFixtureRoles(
			t,
			content.NewInline([]content.Content{content.NewText("A")}),
			inlineRole,
		)
		rule := NewPresentationRule(
			RolePresentationSelector(inlineRole),
			PresentationDeclaration{}.
				WithLength(SetDeclarationValue(Fixed(3))).
				WithGap(SetDeclarationValue(uint32(3))).
				WithWrap(SetDeclarationValue(WrapNone)).
				WithAlignment(SetDeclarationValue(AlignEnd)),
		)
		return contentProjectionFixture{
			content: content.NewFlow([]content.Content{
				inline.Content(),
				content.NewParagraph([]content.Content{content.NewText("B")}).Content(),
			}).Content(),
			sheet: NewPresentationSheet([]PresentationRule{rule}),
		}
	case "semantic-boundary-ignored":
		paragraph, err := content.NewParagraph([]content.Content{
			content.NewText("A"), content.NewText("B"),
		}).WithBoundary(content.BoundarySpace)
		if err != nil {
			t.Fatal(err)
		}
		return contentProjectionFixture{content: paragraph.Content()}
	case "root-text":
		return contentProjectionFixture{content: content.NewText("A")}
	case "invalid-inline-block":
		return contentProjectionFixture{content: content.NewParagraph([]content.Content{content.NewFlow(nil).Content()}).Content()}
	case "content-node-limit":
		return limitedContentProjectionFixture(ContentProjectionLimits{}.WithMaxContentNodes(2))
	case "output-node-limit":
		return contentProjectionFixture{
			content: content.NewFlow([]content.Content{content.NewText("A"), content.NewText("B")}).Content(),
			options: ContentProjectionOptions{}.WithLimits(
				ContentProjectionLimits{}.WithMaxOutputNodes(1),
			),
		}
	case "span-limit":
		return limitedContentProjectionFixture(ContentProjectionLimits{}.WithMaxSpans(1))
	case "depth-limit":
		return limitedContentProjectionFixture(ContentProjectionLimits{}.WithMaxDepth(1))
	case "visual-byte-limit":
		return limitedContentProjectionFixture(ContentProjectionLimits{}.WithMaxVisualBytes(1))
	default:
		t.Fatalf("unknown projection fixture %q", caseID)
		return contentProjectionFixture{}
	}
}

func limitedContentProjectionFixture(limits ContentProjectionLimits) contentProjectionFixture {
	return contentProjectionFixture{
		content: content.NewParagraph([]content.Content{content.NewText("A"), content.NewText("B")}).Content(),
		options: ContentProjectionOptions{}.WithLimits(limits),
	}
}

func projectionFixtureRole(t *testing.T, value string) content.Role {
	t.Helper()
	role, err := content.NewRole(value)
	if err != nil {
		t.Fatal(err)
	}
	return role
}

func projectionFixtureRoles(t *testing.T, element content.Element, roles ...content.Role) content.Element {
	t.Helper()
	element, err := element.WithRoles(roles)
	if err != nil {
		t.Fatal(err)
	}
	return element
}

func projectionFixtureNumber(t *testing.T, value string) uint32 {
	t.Helper()
	number, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		t.Fatal(err)
	}
	return uint32(number)
}
