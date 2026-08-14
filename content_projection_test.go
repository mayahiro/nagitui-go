package tui

import (
	"errors"
	"runtime"
	"testing"

	"github.com/mayahiro/nagi-go/content"
	"github.com/mayahiro/nagi-go/vt"
)

func TestContentProjectionLimitsHaveBoundedZeroAndDepthBehavior(t *testing.T) {
	var limits ContentProjectionLimits
	if got := limits.WithMaxContentNodes(0).MaxContentNodes(); got != DefaultContentProjectionMaxContentNodes {
		t.Fatalf("MaxContentNodes() = %d", got)
	}
	if got := limits.WithMaxDepth(^uint32(0)).MaxDepth(); got != MaxContentProjectionDepth {
		t.Fatalf("MaxDepth() = %d", got)
	}
}

func TestContentProjectionZeroContentAndElementProjectSafely(t *testing.T) {
	if _, err := ProjectContent[struct{}](content.Content{}, PresentationSheet{}, ContentProjectionOptions{}); err != nil {
		t.Fatalf("zero Content error = %v", err)
	}
	if _, err := ProjectContent[struct{}](content.Element{}.Content(), PresentationSheet{}, ContentProjectionOptions{}); err != nil {
		t.Fatalf("zero Element error = %v", err)
	}
}

func TestContentProjectionRejectsBlockDisplayInsideInlineContent(t *testing.T) {
	root := content.NewParagraph([]content.Content{
		content.NewFlow(nil).Content(),
	}).Content()
	_, err := ProjectContent[struct{}](root, PresentationSheet{}, ContentProjectionOptions{})
	var projectionError *ContentProjectionError
	if !errors.As(err, &projectionError) {
		t.Fatalf("ProjectContent() error = %v", err)
	}
	if projectionError.Kind() != ContentProjectionInvalidLayoutTree || projectionError.Depth() != 2 {
		t.Fatalf("projection error = %#v", projectionError)
	}
	if display, ok := projectionError.RejectedDisplay(); !ok || display != PresentationDisplayFlow {
		t.Fatalf("RejectedDisplay() = %v, %t", display, ok)
	}
}

func TestContentProjectionStateResolverRunsOncePerVisitedElement(t *testing.T) {
	role, _ := content.NewRole("item")
	selected, _ := NewPresentationState("selected")
	rule, err := NewPresentationRule(
		RolePresentationSelector(role),
		PresentationDeclaration{}.WithTextStyle(
			TextStyleDeclaration{}.WithBold(SetDeclarationValue(true)),
		),
	).Requiring(selected)
	if err != nil {
		t.Fatal(err)
	}
	child, err := content.NewInline([]content.Content{content.NewText("value")}).WithRoles([]content.Role{role})
	if err != nil {
		t.Fatal(err)
	}
	root := content.NewParagraph([]content.Content{child.Content()}).Content()
	calls := 0
	_, err = ProjectContentWithStates[struct{}](
		root,
		NewPresentationSheet([]PresentationRule{rule}),
		ContentProjectionOptions{},
		func(content.Element) []PresentationState {
			calls++
			return []PresentationState{selected}
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("state resolver calls = %d, want 2", calls)
	}
}

func TestContentProjectionResourceLimitsReportFirstObservedValue(t *testing.T) {
	paragraph := content.NewParagraph([]content.Content{
		content.NewText("a"), content.NewText("b"),
	}).Content()
	flow := content.NewFlow([]content.Content{
		content.NewText("a"), content.NewText("b"),
	}).Content()
	tests := []struct {
		name   string
		root   content.Content
		limits ContentProjectionLimits
		kind   ContentProjectionErrorKind
	}{
		{"content", paragraph, ContentProjectionLimits{}.WithMaxContentNodes(2), ContentProjectionContentNodeLimit},
		{"output", flow, ContentProjectionLimits{}.WithMaxOutputNodes(1), ContentProjectionOutputNodeLimit},
		{"span", paragraph, ContentProjectionLimits{}.WithMaxSpans(1), ContentProjectionSpanLimit},
		{"depth", paragraph, ContentProjectionLimits{}.WithMaxDepth(1), ContentProjectionDepthLimit},
		{"bytes", paragraph, ContentProjectionLimits{}.WithMaxVisualBytes(1), ContentProjectionVisualByteLimit},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ProjectContent[struct{}](
				test.root,
				PresentationSheet{},
				ContentProjectionOptions{}.WithLimits(test.limits),
			)
			var projectionError *ContentProjectionError
			if !errors.As(err, &projectionError) {
				t.Fatalf("ProjectContent() error = %v", err)
			}
			if projectionError.Kind() != test.kind {
				t.Fatalf("Kind() = %s, want %s", projectionError.Kind(), test.kind)
			}
			limit, hasLimit := projectionError.Limit()
			observed, hasObserved := projectionError.Observed()
			if !hasLimit || !hasObserved || observed != limit+1 {
				t.Fatalf("resource = limit %d/%t observed %d/%t", limit, hasLimit, observed, hasObserved)
			}
		})
	}
}

func TestParagraphPublicConstructorStillOwnsItsInput(t *testing.T) {
	spans := []TextSpan{NewTextSpan("original", vt.Style{})}
	node := Paragraph[struct{}](spans, DefaultParagraphOptions())
	spans[0].Text = "changed"
	if node.spans[0].Text != "original" {
		t.Fatalf("Paragraph retained caller slice: %q", node.spans[0].Text)
	}
}

func TestContentProjectionEmptySeparatorConsumesNoOutputResource(t *testing.T) {
	sequenceRole, _ := content.NewRole("sequence")
	sequence, err := content.NewSequence([]content.Content{
		content.NewText("a"), content.NewText("b"),
	}).WithRoles([]content.Role{sequenceRole})
	if err != nil {
		t.Fatal(err)
	}
	sheet := NewPresentationSheet([]PresentationRule{NewPresentationRule(
		RolePresentationSelector(sequenceRole),
		PresentationDeclaration{}.WithVisualSeparator(SetDeclarationValue("")),
	)})
	_, err = ProjectContent[struct{}](
		sequence.Content(),
		sheet,
		ContentProjectionOptions{}.WithLimits(
			ContentProjectionLimits{}.
				WithMaxOutputNodes(3).
				WithMaxSpans(2),
		),
	)
	if err != nil {
		t.Fatalf("empty separator consumed output resources: %v", err)
	}
}

func TestContentProjectionDoesNotRetainFrameResults(t *testing.T) {
	root, sheet, options := contentProjectionBenchmarkInput()
	projectWindow := func() {
		for range 512 {
			node, err := ProjectContent[struct{}](root, sheet, options)
			if err != nil {
				t.Fatal(err)
			}
			runtime.KeepAlive(node)
		}
	}
	projectWindow()
	baseline := heapAllocationAfterGC()
	projectWindow()
	retained := heapAllocationAfterGC()
	const tolerance = uint64(1 << 20)
	if retained > baseline+tolerance {
		t.Fatalf("retained heap grew from %d to %d bytes", baseline, retained)
	}
	runtime.KeepAlive(root)
	runtime.KeepAlive(sheet)
}
