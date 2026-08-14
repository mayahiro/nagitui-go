package tui

import (
	"fmt"

	"github.com/mayahiro/nagi-go/content"
	"github.com/mayahiro/nagi-go/vt"
)

const (
	// DefaultContentProjectionMaxContentNodes is the default maximum number of
	// Content node occurrences visited by one projection.
	DefaultContentProjectionMaxContentNodes uint64 = 100_000
	// DefaultContentProjectionMaxOutputNodes is the default maximum number of
	// TUI Nodes created by one projection.
	DefaultContentProjectionMaxOutputNodes uint64 = 100_000
	// DefaultContentProjectionMaxSpans is the default maximum number of styled
	// text spans created by one projection.
	DefaultContentProjectionMaxSpans uint64 = 100_000
	// DefaultContentProjectionMaxDepth is the default maximum Content and output
	// tree depth accepted by one projection.
	DefaultContentProjectionMaxDepth uint32 = 128
	// MaxContentProjectionDepth is the hard projection depth supported by the
	// recursive TUI Node backend.
	MaxContentProjectionDepth uint32 = 256
	// DefaultContentProjectionMaxVisualBytes is the default maximum emitted
	// UTF-8 byte count accepted by one projection.
	DefaultContentProjectionMaxVisualBytes uint64 = 16 * 1024 * 1024
)

// ContentProjectionLimits bounds eager work performed by one Content-to-Node
// projection.
//
// Its zero value selects all defaults. Passing zero to a WithMax method
// restores that property's default. Depth values above
// MaxContentProjectionDepth are capped at that backend safety limit.
type ContentProjectionLimits struct {
	maxContentNodes uint64
	maxOutputNodes  uint64
	maxSpans        uint64
	maxDepth        uint32
	maxVisualBytes  uint64
}

// DefaultContentProjectionLimits returns the bounded default projection
// limits.
func DefaultContentProjectionLimits() ContentProjectionLimits {
	return ContentProjectionLimits{
		maxContentNodes: DefaultContentProjectionMaxContentNodes,
		maxOutputNodes:  DefaultContentProjectionMaxOutputNodes,
		maxSpans:        DefaultContentProjectionMaxSpans,
		maxDepth:        DefaultContentProjectionMaxDepth,
		maxVisualBytes:  DefaultContentProjectionMaxVisualBytes,
	}
}

// MaxContentNodes returns the maximum visited Content node occurrence count.
func (l ContentProjectionLimits) MaxContentNodes() uint64 {
	return l.normalized().maxContentNodes
}

// WithMaxContentNodes returns these limits with a maximum Content node
// occurrence count.
func (l ContentProjectionLimits) WithMaxContentNodes(value uint64) ContentProjectionLimits {
	l = l.normalized()
	l.maxContentNodes = defaultUint64(value, DefaultContentProjectionMaxContentNodes)
	return l
}

// MaxOutputNodes returns the maximum generated TUI Node count.
func (l ContentProjectionLimits) MaxOutputNodes() uint64 {
	return l.normalized().maxOutputNodes
}

// WithMaxOutputNodes returns these limits with a maximum generated TUI Node
// count.
func (l ContentProjectionLimits) WithMaxOutputNodes(value uint64) ContentProjectionLimits {
	l = l.normalized()
	l.maxOutputNodes = defaultUint64(value, DefaultContentProjectionMaxOutputNodes)
	return l
}

// MaxSpans returns the maximum generated styled text span count.
func (l ContentProjectionLimits) MaxSpans() uint64 {
	return l.normalized().maxSpans
}

// WithMaxSpans returns these limits with a maximum generated styled text span
// count.
func (l ContentProjectionLimits) WithMaxSpans(value uint64) ContentProjectionLimits {
	l = l.normalized()
	l.maxSpans = defaultUint64(value, DefaultContentProjectionMaxSpans)
	return l
}

// MaxDepth returns the maximum accepted Content and output tree depth.
func (l ContentProjectionLimits) MaxDepth() uint32 {
	return l.normalized().maxDepth
}

// WithMaxDepth returns these limits with a maximum Content and output tree
// depth.
//
// Zero restores the default and values above the supported backend limit are
// capped.
func (l ContentProjectionLimits) WithMaxDepth(value uint32) ContentProjectionLimits {
	l = l.normalized()
	if value == 0 {
		value = DefaultContentProjectionMaxDepth
	}
	l.maxDepth = min(value, MaxContentProjectionDepth)
	return l
}

// MaxVisualBytes returns the maximum emitted UTF-8 byte count.
func (l ContentProjectionLimits) MaxVisualBytes() uint64 {
	return l.normalized().maxVisualBytes
}

// WithMaxVisualBytes returns these limits with a maximum emitted UTF-8 byte
// count.
func (l ContentProjectionLimits) WithMaxVisualBytes(value uint64) ContentProjectionLimits {
	l = l.normalized()
	l.maxVisualBytes = defaultUint64(value, DefaultContentProjectionMaxVisualBytes)
	return l
}

func (l ContentProjectionLimits) normalized() ContentProjectionLimits {
	if l.maxContentNodes == 0 {
		l.maxContentNodes = DefaultContentProjectionMaxContentNodes
	}
	if l.maxOutputNodes == 0 {
		l.maxOutputNodes = DefaultContentProjectionMaxOutputNodes
	}
	if l.maxSpans == 0 {
		l.maxSpans = DefaultContentProjectionMaxSpans
	}
	if l.maxDepth == 0 {
		l.maxDepth = DefaultContentProjectionMaxDepth
	}
	l.maxDepth = min(l.maxDepth, MaxContentProjectionDepth)
	if l.maxVisualBytes == 0 {
		l.maxVisualBytes = DefaultContentProjectionMaxVisualBytes
	}
	return l
}

func defaultUint64(value, defaultValue uint64) uint64 {
	if value == 0 {
		return defaultValue
	}
	return value
}

// ContentProjectionOptions controls one Content-to-Node projection.
//
// Its zero value uses the terminal-default root style and bounded default
// limits.
type ContentProjectionOptions struct {
	baseStyle vt.Style
	limits    ContentProjectionLimits
}

// DefaultContentProjectionOptions returns terminal-default projection options.
func DefaultContentProjectionOptions() ContentProjectionOptions {
	return ContentProjectionOptions{limits: DefaultContentProjectionLimits()}
}

// BaseStyle returns the style inherited by the root Content node.
func (o ContentProjectionOptions) BaseStyle() vt.Style {
	return o.baseStyle
}

// WithBaseStyle returns these options with a replacement root inherited style.
func (o ContentProjectionOptions) WithBaseStyle(value vt.Style) ContentProjectionOptions {
	o.baseStyle = value
	return o
}

// Limits returns the bounded eager-work limits.
func (o ContentProjectionOptions) Limits() ContentProjectionLimits {
	return o.limits.normalized()
}

// WithLimits returns these options with replacement eager-work limits.
func (o ContentProjectionOptions) WithLimits(value ContentProjectionLimits) ContentProjectionOptions {
	o.limits = value.normalized()
	return o
}

// ContentProjectionErrorKind is a stable category of Content-to-Node
// projection failure.
type ContentProjectionErrorKind uint8

const (
	// ContentProjectionInvalidLayoutTree reports a block display inside an
	// inline formatting context.
	ContentProjectionInvalidLayoutTree ContentProjectionErrorKind = iota
	// ContentProjectionContentNodeLimit reports a visited Content node limit.
	ContentProjectionContentNodeLimit
	// ContentProjectionOutputNodeLimit reports a generated TUI Node limit.
	ContentProjectionOutputNodeLimit
	// ContentProjectionSpanLimit reports a generated styled text span limit.
	ContentProjectionSpanLimit
	// ContentProjectionDepthLimit reports a Content or output depth limit.
	ContentProjectionDepthLimit
	// ContentProjectionVisualByteLimit reports an emitted UTF-8 byte limit.
	ContentProjectionVisualByteLimit
)

// String returns the stable language-independent error identifier.
func (k ContentProjectionErrorKind) String() string {
	switch k {
	case ContentProjectionInvalidLayoutTree:
		return "invalid-layout-tree"
	case ContentProjectionContentNodeLimit:
		return "content-node-limit"
	case ContentProjectionOutputNodeLimit:
		return "output-node-limit"
	case ContentProjectionSpanLimit:
		return "span-limit"
	case ContentProjectionDepthLimit:
		return "depth-limit"
	case ContentProjectionVisualByteLimit:
		return "visual-byte-limit"
	default:
		return "unknown-content-projection-error"
	}
}

// ContentProjectionError is a structured Content-to-Node projection failure.
type ContentProjectionError struct {
	kind               ContentProjectionErrorKind
	depth              uint32
	limit              uint64
	observed           uint64
	hasResource        bool
	rejectedDisplay    PresentationDisplay
	hasRejectedDisplay bool
}

// Error returns the projection diagnostic.
func (e *ContentProjectionError) Error() string {
	if e.hasResource {
		return fmt.Sprintf(
			"content projection %s at depth %d: limit %d, observed %d",
			e.kind,
			e.depth,
			e.limit,
			e.observed,
		)
	}
	return fmt.Sprintf("content projection %s at depth %d", e.kind, e.depth)
}

// Kind returns the stable error category.
func (e *ContentProjectionError) Kind() ContentProjectionErrorKind {
	return e.kind
}

// Depth returns the one-based Content or output depth where the error arose.
func (e *ContentProjectionError) Depth() uint32 {
	return e.depth
}

// Limit returns the exceeded limit for a resource error.
func (e *ContentProjectionError) Limit() (uint64, bool) {
	return e.limit, e.hasResource
}

// Observed returns the first observed value above a resource limit.
func (e *ContentProjectionError) Observed() (uint64, bool) {
	return e.observed, e.hasResource
}

// RejectedDisplay returns the block display rejected inside inline content.
func (e *ContentProjectionError) RejectedDisplay() (PresentationDisplay, bool) {
	return e.rejectedDisplay, e.hasRejectedDisplay
}

func newContentProjectionResourceError(
	kind ContentProjectionErrorKind,
	depth uint32,
	limit uint64,
	observed uint64,
) *ContentProjectionError {
	return &ContentProjectionError{
		kind: kind, depth: depth, limit: limit, observed: observed, hasResource: true,
	}
}

func newContentProjectionLayoutError(
	depth uint32,
	display PresentationDisplay,
) *ContentProjectionError {
	return &ContentProjectionError{
		kind:               ContentProjectionInvalidLayoutTree,
		depth:              depth,
		rejectedDisplay:    display,
		hasRejectedDisplay: true,
	}
}

// ContentProjectionStateResolver returns application states for one Content
// Element.
//
// Projection calls the resolver synchronously once per visited Element and
// does not retain the returned slice.
type ContentProjectionStateResolver func(content.Element) []PresentationState

// ProjectContent projects Content through a Presentation Sheet without
// application states.
//
// The returned Node has no implicit NodeID or annotation handler. Inline roots
// and inline children directly under Flow or Sequence are promoted to
// anonymous Paragraph Nodes.
func ProjectContent[Message any](
	root content.Content,
	sheet PresentationSheet,
	options ContentProjectionOptions,
) (Node[Message], error) {
	return ProjectContentWithStates[Message](root, sheet, options, nil)
}

// ProjectContentWithStates projects Content through a Presentation Sheet with
// per-element application states. A nil statesFor resolver uses an empty State
// set for every Element.
func ProjectContentWithStates[Message any](
	root content.Content,
	sheet PresentationSheet,
	options ContentProjectionOptions,
	statesFor ContentProjectionStateResolver,
) (Node[Message], error) {
	projector := contentProjector[Message]{
		sheet: sheet, limits: options.limits.normalized(), statesFor: statesFor,
	}
	return projector.projectBox(root, options.baseStyle, 1, 1)
}

type contentProjector[Message any] struct {
	sheet        PresentationSheet
	limits       ContentProjectionLimits
	statesFor    ContentProjectionStateResolver
	contentNodes uint64
	outputNodes  uint64
	spans        uint64
	visualBytes  uint64
}

func (p *contentProjector[Message]) projectBox(
	root content.Content,
	inheritedStyle vt.Style,
	contentDepth uint32,
	outputDepth uint32,
) (Node[Message], error) {
	if err := p.enterContent(contentDepth); err != nil {
		return Node[Message]{}, err
	}
	switch root.Kind() {
	case content.ContentText:
		if err := p.enterOutput(outputDepth); err != nil {
			return Node[Message]{}, err
		}
		text, _ := root.Text()
		spans := make([]TextSpan, 0, 1)
		if err := p.pushSpan(&spans, text, inheritedStyle, contentDepth); err != nil {
			return Node[Message]{}, err
		}
		return paragraphOwned[Message](spans, DefaultParagraphOptions()), nil
	case content.ContentHardBreak:
		if err := p.enterOutput(outputDepth); err != nil {
			return Node[Message]{}, err
		}
		spans := make([]TextSpan, 0, 1)
		if err := p.pushSpan(&spans, "\n", inheritedStyle, contentDepth); err != nil {
			return Node[Message]{}, err
		}
		return paragraphOwned[Message](spans, DefaultParagraphOptions()), nil
	case content.ContentElement:
		element, _ := root.Element()
		computed := p.resolve(element, inheritedStyle)
		switch computed.Display() {
		case PresentationDisplayInline:
			return p.projectInlineBox(element, computed, contentDepth, outputDepth)
		case PresentationDisplayParagraph:
			return p.projectParagraph(element, computed, contentDepth, outputDepth)
		case PresentationDisplayFlow, PresentationDisplaySequence:
			return p.projectContainer(element, computed, contentDepth, outputDepth)
		}
	}
	panic("unreachable Content or Presentation kind")
}

func (p *contentProjector[Message]) projectInlineBox(
	element content.Element,
	computed ComputedPresentation,
	contentDepth uint32,
	outputDepth uint32,
) (Node[Message], error) {
	if err := p.enterOutput(outputDepth); err != nil {
		return Node[Message]{}, err
	}
	spans := make([]TextSpan, 0)
	if err := p.appendInlineChildren(element, computed, contentDepth, &spans); err != nil {
		return Node[Message]{}, err
	}
	return paragraphOwned[Message](spans, DefaultParagraphOptions()), nil
}

func (p *contentProjector[Message]) projectParagraph(
	element content.Element,
	computed ComputedPresentation,
	contentDepth uint32,
	outputDepth uint32,
) (Node[Message], error) {
	if err := p.enterOutput(outputDepth); err != nil {
		return Node[Message]{}, err
	}
	spans := make([]TextSpan, 0)
	if err := p.appendInlineChildren(element, computed, contentDepth, &spans); err != nil {
		return Node[Message]{}, err
	}
	return paragraphOwned[Message](spans, ParagraphOptions{
		Wrap: computed.Wrap(), Alignment: computed.Alignment(),
	}).WithLength(computed.Length()), nil
}

func (p *contentProjector[Message]) projectContainer(
	element content.Element,
	computed ComputedPresentation,
	contentDepth uint32,
	outputDepth uint32,
) (Node[Message], error) {
	if err := p.enterOutput(outputDepth); err != nil {
		return Node[Message]{}, err
	}
	contentChildDepth := contentDepth + 1
	outputChildDepth := outputDepth + 1
	children := make([]Node[Message], 0, min(element.ChildCount(), 64))
	for index := 0; index < element.ChildCount(); index++ {
		if index != 0 {
			if separator, ok := computed.VisualSeparator(); ok && separator != "" {
				if err := p.enterOutput(outputChildDepth); err != nil {
					return Node[Message]{}, err
				}
				spans := make([]TextSpan, 0, 1)
				if err := p.pushSpan(&spans, separator, computed.Style(), outputChildDepth); err != nil {
					return Node[Message]{}, err
				}
				children = append(children, paragraphOwned[Message](spans, DefaultParagraphOptions()))
			}
			if computed.Gap() != 0 {
				if err := p.enterOutput(outputChildDepth); err != nil {
					return Node[Message]{}, err
				}
				children = append(children, Gap[Message](computed.Gap()))
			}
		}
		child, _ := element.Child(index)
		projected, err := p.projectBox(child, computed.Style(), contentChildDepth, outputChildDepth)
		if err != nil {
			return Node[Message]{}, err
		}
		children = append(children, projected)
	}
	if computed.Display() == PresentationDisplayFlow {
		return Column(children...).WithLength(computed.Length()), nil
	}
	return Row(children...).WithLength(computed.Length()), nil
}

func (p *contentProjector[Message]) appendInlineChildren(
	element content.Element,
	computed ComputedPresentation,
	contentDepth uint32,
	spans *[]TextSpan,
) error {
	childDepth := contentDepth + 1
	for index := 0; index < element.ChildCount(); index++ {
		if index != 0 {
			if separator, ok := computed.VisualSeparator(); ok && separator != "" {
				if err := p.pushSpan(spans, separator, computed.Style(), childDepth); err != nil {
					return err
				}
			}
		}
		child, _ := element.Child(index)
		if err := p.appendInline(child, computed.Style(), childDepth, spans); err != nil {
			return err
		}
	}
	return nil
}

func (p *contentProjector[Message]) appendInline(
	root content.Content,
	inheritedStyle vt.Style,
	contentDepth uint32,
	spans *[]TextSpan,
) error {
	if err := p.enterContent(contentDepth); err != nil {
		return err
	}
	switch root.Kind() {
	case content.ContentText:
		text, _ := root.Text()
		return p.pushSpan(spans, text, inheritedStyle, contentDepth)
	case content.ContentHardBreak:
		return p.pushSpan(spans, "\n", inheritedStyle, contentDepth)
	case content.ContentElement:
		element, _ := root.Element()
		computed := p.resolve(element, inheritedStyle)
		if computed.Display() != PresentationDisplayInline {
			return newContentProjectionLayoutError(contentDepth, computed.Display())
		}
		return p.appendInlineChildren(element, computed, contentDepth, spans)
	default:
		panic("unreachable Content kind")
	}
}

func (p *contentProjector[Message]) resolve(
	element content.Element,
	inheritedStyle vt.Style,
) ComputedPresentation {
	var states []PresentationState
	if p.statesFor != nil {
		states = p.statesFor(element)
	}
	return p.sheet.Resolve(element, inheritedStyle, states)
}

func (p *contentProjector[Message]) enterContent(depth uint32) error {
	if err := p.checkDepth(depth); err != nil {
		return err
	}
	observed := saturatingIncrement(p.contentNodes)
	if observed > p.limits.maxContentNodes {
		return newContentProjectionResourceError(
			ContentProjectionContentNodeLimit, depth, p.limits.maxContentNodes, observed,
		)
	}
	p.contentNodes = observed
	return nil
}

func (p *contentProjector[Message]) enterOutput(depth uint32) error {
	if err := p.checkDepth(depth); err != nil {
		return err
	}
	observed := saturatingIncrement(p.outputNodes)
	if observed > p.limits.maxOutputNodes {
		return newContentProjectionResourceError(
			ContentProjectionOutputNodeLimit, depth, p.limits.maxOutputNodes, observed,
		)
	}
	p.outputNodes = observed
	return nil
}

func (p *contentProjector[Message]) checkDepth(depth uint32) error {
	if depth > p.limits.maxDepth {
		return newContentProjectionResourceError(
			ContentProjectionDepthLimit, depth, uint64(p.limits.maxDepth), uint64(depth),
		)
	}
	return nil
}

func (p *contentProjector[Message]) pushSpan(
	spans *[]TextSpan,
	text string,
	style vt.Style,
	depth uint32,
) error {
	observedBytes := saturatingAdd(p.visualBytes, uint64(len(text)))
	if observedBytes > p.limits.maxVisualBytes {
		return newContentProjectionResourceError(
			ContentProjectionVisualByteLimit, depth, p.limits.maxVisualBytes, observedBytes,
		)
	}
	observedSpans := saturatingIncrement(p.spans)
	if observedSpans > p.limits.maxSpans {
		return newContentProjectionResourceError(
			ContentProjectionSpanLimit, depth, p.limits.maxSpans, observedSpans,
		)
	}
	p.visualBytes = observedBytes
	p.spans = observedSpans
	*spans = append(*spans, NewTextSpan(text, style))
	return nil
}

func saturatingIncrement(value uint64) uint64 {
	return saturatingAdd(value, 1)
}

func saturatingAdd(left, right uint64) uint64 {
	result := left + right
	if result < left {
		return ^uint64(0)
	}
	return result
}
