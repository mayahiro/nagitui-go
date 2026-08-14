package tui

import (
	"math"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go/surface"
)

// Insets contains padding widths around a node
type Insets struct {
	// Top is the number of cells above the child
	Top uint32
	// Right is the number of cells to the child's right
	Right uint32
	// Bottom is the number of cells below the child
	Bottom uint32
	// Left is the number of cells to the child's left
	Left uint32
}

// UniformInsets returns equal insets on every side
func UniformInsets(cells uint32) Insets {
	return Insets{Top: cells, Right: cells, Bottom: cells, Left: cells}
}

// HorizontalAlignment controls horizontal placement inside an alignment node
type HorizontalAlignment uint8

const (
	// AlignStart places content at the left edge
	AlignStart HorizontalAlignment = iota
	// AlignCenter centers content horizontally
	AlignCenter
	// AlignEnd places content at the right edge
	AlignEnd
)

// VerticalAlignment controls vertical placement inside an alignment node
type VerticalAlignment uint8

const (
	// AlignTop places content at the top edge
	AlignTop VerticalAlignment = iota
	// AlignMiddle centers content vertically
	AlignMiddle
	// AlignBottom places content at the bottom edge
	AlignBottom
)

// AnchoredOverlaySide selects the preferred vertical side of an anchor
type AnchoredOverlaySide uint8

const (
	// AnchoredOverlayBelow places the overlay after the anchor row
	AnchoredOverlayBelow AnchoredOverlaySide = iota
	// AnchoredOverlayAbove places the overlay before the anchor row
	AnchoredOverlayAbove
)

// AnchoredOverlayFallback controls placement when the preferred side is too small
type AnchoredOverlayFallback uint8

const (
	// AnchoredOverlayFlip uses the opposite side when it has more available rows
	AnchoredOverlayFlip AnchoredOverlayFallback = iota
	// AnchoredOverlayClip keeps the preferred side and clips to its available rows
	AnchoredOverlayClip
)

// AnchoredOverlayOptions controls anchored front-layer placement and size
//
// Unknown Side, Alignment, and Fallback values use Below, Start, and Flip
type AnchoredOverlayOptions struct {
	// Side is the preferred vertical side of the anchor
	Side AnchoredOverlaySide
	// Alignment positions the layer horizontally relative to the anchor
	Alignment HorizontalAlignment
	// Gap is the number of empty rows between the anchor and layer
	Gap uint32
	// Fallback controls placement when the preferred side cannot contain the natural height
	Fallback AnchoredOverlayFallback
	// MaximumWidth is the greatest layer width, or zero for the available boundary width
	MaximumWidth uint32
	// MaximumHeight is the greatest layer height, or zero for the available boundary height
	MaximumHeight uint32
}

// DefaultAnchoredOverlayOptions returns below-start placement with flip fallback
func DefaultAnchoredOverlayOptions() AnchoredOverlayOptions {
	return AnchoredOverlayOptions{}
}

// ScrollViewportOptions controls ScrollViewport behavior
type ScrollViewportOptions[Message any] struct {
	// Axis selects the axes controlled by user and programmatic scrolling
	Axis ScrollAxis
	// StickToEnd follows content growth while the viewport remains at its end
	StickToEnd bool
	// EnsureFocusedVisible scrolls a focused descendant into view
	EnsureFocusedVisible bool
	// OnScroll optionally maps user scroll state changes to application messages
	OnScroll func(ScrollState) Message
}

// DefaultScrollViewportOptions returns two-dimensional scrolling without
// automatic following or focus tracking
func DefaultScrollViewportOptions[Message any]() ScrollViewportOptions[Message] {
	return ScrollViewportOptions[Message]{Axis: ScrollAxisBoth}
}

// VirtualViewport is the visible content range requested by a virtual
// ScrollViewport
type VirtualViewport struct {
	// Offset is the first visible cell in content coordinates
	Offset ScrollOffset
	// Size is the visible viewport size in cells
	Size Size
	// ContentSize is the complete resolved content extent in cells
	ContentSize Size
}

// VirtualFragment is one lazily constructed visible or overscanned fragment
type VirtualFragment[Message any] struct {
	// Origin is the fragment origin in content coordinates
	Origin ScrollOffset
	// Node is the semantic subtree beginning at Origin
	Node Node[Message]
}

type modalInitialFocusKind uint8

const (
	modalInitialFocusFirst modalInitialFocusKind = iota
	modalInitialFocusTarget
	modalInitialFocusNone
)

// ModalInitialFocus selects focus when a modal becomes active
//
// The zero value selects the first focusable node in the modal scope.
type ModalInitialFocus struct {
	kind   modalInitialFocusKind
	target NodeID
}

// ModalInitialFocusFirst selects the first focusable node in a modal scope
func ModalInitialFocusFirst() ModalInitialFocus {
	return ModalInitialFocus{kind: modalInitialFocusFirst}
}

// ModalInitialFocusTarget selects a stable node and falls back to the first focusable node
func ModalInitialFocusTarget(id NodeID) ModalInitialFocus {
	return ModalInitialFocus{kind: modalInitialFocusTarget, target: id}
}

// ModalInitialFocusNone leaves a modal scope unfocused
func ModalInitialFocusNone() ModalInitialFocus {
	return ModalInitialFocus{kind: modalInitialFocusNone}
}

type modalReturnFocusKind uint8

const (
	modalReturnFocusPrevious modalReturnFocusKind = iota
	modalReturnFocusTarget
	modalReturnFocusNone
)

// ModalReturnFocus selects focus when a modal stops being active
//
// The zero value returns to the node focused immediately before modal entry.
type ModalReturnFocus struct {
	kind   modalReturnFocusKind
	target NodeID
}

// ModalReturnFocusPrevious returns to the node focused before modal entry
func ModalReturnFocusPrevious() ModalReturnFocus {
	return ModalReturnFocus{kind: modalReturnFocusPrevious}
}

// ModalReturnFocusTarget selects a stable node after a modal closes
func ModalReturnFocusTarget(id NodeID) ModalReturnFocus {
	return ModalReturnFocus{kind: modalReturnFocusTarget, target: id}
}

// ModalReturnFocusNone leaves the resumed scope unfocused
func ModalReturnFocusNone() ModalReturnFocus {
	return ModalReturnFocus{kind: modalReturnFocusNone}
}

// ModalFocusOptions contains entry and return focus policies for one modal scope
type ModalFocusOptions struct {
	// Initial is applied when the modal becomes active
	Initial ModalInitialFocus
	// ReturnFocus is applied when the modal stops being active
	ReturnFocus ModalReturnFocus
}

// DefaultModalFocusOptions returns first-on-entry and previous-on-close policies
func DefaultModalFocusOptions() ModalFocusOptions {
	return ModalFocusOptions{
		Initial:     ModalInitialFocusFirst(),
		ReturnFocus: ModalReturnFocusPrevious(),
	}
}

// NewVirtualFragment returns a fragment beginning at origin
func NewVirtualFragment[Message any](origin ScrollOffset, node Node[Message]) VirtualFragment[Message] {
	return VirtualFragment[Message]{Origin: origin, Node: node}
}

type virtualCacheState[Message any] struct {
	valid    bool
	request  VirtualViewport
	fragment VirtualFragment[Message]
}

type nodePayload[Message any] struct {
	surface        *surface.Surface
	virtualSize    Size
	virtualBuilder func(VirtualViewport) VirtualFragment[Message]
	virtualCache   virtualCacheState[Message]
	virtualFlow    *virtualFlowNodePayload[Message]
	responsive     *responsiveRowPayload[Message]
	anchored       *anchoredOverlayPayload[Message]
}

type anchoredOverlayPayload[Message any] struct {
	base    Node[Message]
	anchor  NodeID
	overlay Node[Message]
	options AnchoredOverlayOptions
	cache   anchoredOverlayFrame
}

type anchoredOverlayFrame struct {
	valid      bool
	rect       Rect
	clip       Rect
	overlay    Rect
	hasOverlay bool
}

type virtualFlowNodePayload[Message any] struct {
	source  VirtualFlowSource[Message]
	options VirtualFlowOptions[Message]
	cache   virtualFlowFrame[Message]
}

type virtualFlowFrame[Message any] struct {
	valid         bool
	rect          Rect
	offset        uint32
	contentHeight uint32
	generation    uint64
	items         []virtualFlowBuiltItem[Message]
}

type virtualFlowBuiltItem[Message any] struct {
	index  int
	origin uint32
	height uint32
	node   Node[Message]
}

// Node is a semantic view node rebuilt by an application for each frame
type Node[Message any] struct {
	kind             nodeKind
	content          string
	style            vt.Style
	spans            []TextSpan
	paragraph        ParagraphOptions
	split            SplitPaneOptions
	richTextCache    *richTextLayoutCache
	payload          *nodePayload[Message]
	intrinsicSize    Size
	gap              uint32
	title            string
	panel            PanelOptions
	children         []Node[Message]
	linearCache      *linearLayoutCache
	child            *Node[Message]
	insets           Insets
	horizontal       HorizontalAlignment
	vertical         VerticalAlignment
	length           Length
	id               NodeID
	hasID            bool
	blocksUnhandled  bool
	focusable        bool
	focusedStyle     vt.Style
	hasFocusedStyle  bool
	handler          eventHandler[Message]
	pointerHandler   pointerEventHandler[Message]
	keyInteraction   *nodeKeyInteraction[Message]
	onChange         func(string) Message
	placeholder      string
	placeholderStyle vt.Style
	cursorOwner      NodeID
	scroll           ScrollViewportOptions[Message]
}

type nodeKind uint8

const (
	nodeText nodeKind = iota
	nodeRichText
	nodeSurface
	nodeSpacer
	nodeGap
	nodeCursorAnchor
	nodeTextInput
	nodeRow
	nodeColumn
	nodeResponsiveRow
	nodeSplitPane
	nodeStack
	nodeOverlay
	nodeAnchoredOverlay
	nodePadding
	nodeBorder
	nodeAlign
	nodeClip
	nodeScrollViewport
	nodeVirtualScrollViewport
	nodeVirtualFlow
	nodeModal
	nodePanel
)

type layoutLimit struct {
	value   uint32
	bounded bool
}

type layoutConstraints struct {
	width, height layoutLimit
}

func boundedConstraints(size Size) layoutConstraints {
	return layoutConstraints{
		width:  layoutLimit{value: size.Width, bounded: true},
		height: layoutLimit{value: size.Height, bounded: true},
	}
}

// Text returns a default-style text node
func Text[Message any](content string) Node[Message] {
	return StyledText[Message](content, vt.Style{})
}

// StyledText returns a styled text node
func StyledText[Message any](content string, style vt.Style) Node[Message] {
	return Node[Message]{kind: nodeText, content: content, style: style}
}

// RichText returns inline styled text with grapheme-safe hard wrapping
func RichText[Message any](spans ...TextSpan) Node[Message] {
	return Node[Message]{
		kind:          nodeRichText,
		spans:         cloneTextSpans(spans),
		paragraph:     ParagraphOptions{Wrap: WrapHard, Alignment: AlignStart},
		richTextCache: &richTextLayoutCache{},
	}
}

// Paragraph returns inline styled text using the supplied wrapping and alignment
func Paragraph[Message any](spans []TextSpan, options ParagraphOptions) Node[Message] {
	return paragraphOwned[Message](cloneTextSpans(spans), options)
}

func paragraphOwned[Message any](spans []TextSpan, options ParagraphOptions) Node[Message] {
	return Node[Message]{
		kind:          nodeRichText,
		spans:         spans,
		paragraph:     options,
		richTextCache: &richTextLayoutCache{},
	}
}

// SurfaceNode captures an independent snapshot of a public Surface as a node
//
// A nil source produces an empty node. Surface cells remain typed and cannot
// introduce raw terminal escape sequences.
func SurfaceNode[Message any](source *surface.Surface) Node[Message] {
	if source == nil {
		return Node[Message]{kind: nodeSurface}
	}
	return Node[Message]{
		kind:    nodeSurface,
		payload: &nodePayload[Message]{surface: source.Clone()},
	}
}

// Spacer returns an invisible node with a fixed measured size
func Spacer[Message any](width, height uint32) Node[Message] {
	return Node[Message]{kind: nodeSpacer, intrinsicSize: Size{Width: width, Height: height}}
}

// Gap returns spacing interpreted along the main axis of its immediate Row or Column
//
// Outside a Row or Column, a Gap has zero measured size.
func Gap[Message any](cells uint32) Node[Message] {
	return Node[Message]{kind: nodeGap, gap: cells}
}

// CursorAnchor returns a zero-width cursor position shown while focusOwner is
// focused
//
// The anchor measures one row high, consumes no horizontal layout space, and
// sets the output Surface cursor only while its position is visible
func CursorAnchor[Message any](focusOwner NodeID) Node[Message] {
	return Node[Message]{kind: nodeCursorAnchor, cursorOwner: focusOwner}
}

// Row returns a horizontal container
//
// The returned node retains the supplied variadic slice. Callers must not
// mutate that slice after construction.
func Row[Message any](children ...Node[Message]) Node[Message] {
	return Node[Message]{kind: nodeRow, children: children, linearCache: &linearLayoutCache{}}
}

// Column returns a vertical container
//
// The returned node retains the supplied variadic slice. Callers must not
// mutate that slice after construction.
func Column[Message any](children ...Node[Message]) Node[Message] {
	return Node[Message]{kind: nodeColumn, children: children, linearCache: &linearLayoutCache{}}
}

// ResponsiveRow returns a priority-aware three-region horizontal container
//
// Supplied item Nodes are eager. Items that do not fit the assigned width are
// omitted from preparation, semantic indexing, hit testing, routing, and
// rendering. Higher priorities are retained first and source order breaks
// equal-priority ties. The returned node retains the supplied item slice;
// callers must not mutate it after construction.
func ResponsiveRow[Message any](
	items []ResponsiveRowItem[Message],
	options ResponsiveRowOptions,
) Node[Message] {
	return Node[Message]{
		kind: nodeResponsiveRow,
		payload: &nodePayload[Message]{responsive: &responsiveRowPayload[Message]{
			items: items, options: options,
		}},
	}
}

// SplitPane returns a responsive two-pane container with a one-Cell divider
//
// Both supplied Nodes are eager, but only panes present in the resolved layout
// participate in preparation, semantic indexing, hit testing, and rendering
// The configured collapse pane is omitted when the assigned main-axis extent
// cannot satisfy both normalized minima plus the divider
func SplitPane[Message any](primary, secondary Node[Message], options SplitPaneOptions) Node[Message] {
	return Node[Message]{kind: nodeSplitPane, children: []Node[Message]{primary, secondary}, split: options}
}

// Stack returns a front-to-back overlay container
//
// The returned node retains the supplied variadic slice. Callers must not
// mutate that slice after construction.
func Stack[Message any](children ...Node[Message]) Node[Message] {
	return Node[Message]{kind: nodeStack, children: children}
}

// Overlay places a front layer over a base without adding the layer to measurement
//
// Both children receive the complete assigned rectangle. The base is prepared,
// indexed, and rendered first, so the layer is topmost for overlapping pointer hits.
func Overlay[Message any](base, layer Node[Message]) Node[Message] {
	return Node[Message]{kind: nodeOverlay, children: []Node[Message]{base, layer}}
}

// AnchoredOverlay places a front layer relative to an identified base descendant
//
// The default prefers below-start placement, flips above when that side has
// more room, and constrains the layer to this node's visible boundary
func AnchoredOverlay[Message any](
	base Node[Message],
	anchor NodeID,
	overlay Node[Message],
) Node[Message] {
	return AnchoredOverlayWithOptions(base, anchor, overlay, DefaultAnchoredOverlayOptions())
}

// AnchoredOverlayWithOptions places a configured front layer relative to an identified descendant
//
// The overlay does not affect measurement and is omitted from rendering, hit
// testing, and routing while the anchor is absent or not visible
func AnchoredOverlayWithOptions[Message any](
	base Node[Message],
	anchor NodeID,
	overlay Node[Message],
	options AnchoredOverlayOptions,
) Node[Message] {
	return Node[Message]{
		kind: nodeAnchoredOverlay,
		payload: &nodePayload[Message]{anchored: &anchoredOverlayPayload[Message]{
			base: base, anchor: anchor, overlay: overlay, options: options,
		}},
	}
}

// Padding wraps a child in fixed padding
func Padding[Message any](child Node[Message], insets Insets) Node[Message] {
	return Node[Message]{kind: nodePadding, child: &child, insets: insets}
}

// Border wraps a child in a single-cell Unicode border
func Border[Message any](child Node[Message], style vt.Style) Node[Message] {
	return Node[Message]{kind: nodeBorder, child: &child, style: style}
}

// Panel returns a titled single-border container with one-cell inner padding
func Panel[Message any](child Node[Message], title string) Node[Message] {
	return PanelWithOptions(child, title, DefaultPanelOptions())
}

// PanelWithOptions returns a titled container with configured border, padding, and styles
func PanelWithOptions[Message any](child Node[Message], title string, options PanelOptions) Node[Message] {
	return Node[Message]{kind: nodePanel, child: &child, title: title, panel: options}
}

// Align places a child within the rectangle assigned to the node
func Align[Message any](child Node[Message], horizontal HorizontalAlignment, vertical VerticalAlignment) Node[Message] {
	return Node[Message]{
		kind:       nodeAlign,
		child:      &child,
		horizontal: horizontal,
		vertical:   vertical,
	}
}

// Clip limits a child's drawing to the assigned rectangle
func Clip[Message any](child Node[Message]) Node[Message] {
	return Node[Message]{kind: nodeClip, child: &child}
}

// TextInput returns a one-line grapheme-aware input with retained cursor state
func TextInput[Message any](id NodeID, value string, onChange func(string) Message) Node[Message] {
	return StyledTextInput(id, value, "", vt.Style{}, vt.Style{Dim: true}, onChange)
}

// StyledTextInput returns a styled one-line input with placeholder text
func StyledTextInput[Message any](
	id NodeID,
	value, placeholder string,
	style, placeholderStyle vt.Style,
	onChange func(string) Message,
) Node[Message] {
	return Node[Message]{
		kind:             nodeTextInput,
		content:          celltext.NormalizeUTF8(value),
		style:            style,
		id:               id,
		hasID:            true,
		focusable:        true,
		onChange:         onChange,
		placeholder:      celltext.NormalizeUTF8(placeholder),
		placeholderStyle: placeholderStyle,
	}
}

// ScrollViewport returns a clipped viewport with runtime-owned cell offset
//
// The supplied child tree is eager. Use VirtualScrollViewport when content
// construction must be bounded by the visible region.
func ScrollViewport[Message any](id NodeID, child Node[Message]) Node[Message] {
	return ScrollViewportWithOptions(id, child, DefaultScrollViewportOptions[Message]())
}

// ScrollViewportWithOptions returns a clipped viewport with configured behavior
//
// The supplied child tree is eager. Use VirtualScrollViewportWithOptions when
// content construction must be bounded by the visible region.
func ScrollViewportWithOptions[Message any](
	id NodeID,
	child Node[Message],
	options ScrollViewportOptions[Message],
) Node[Message] {
	return Node[Message]{
		kind:      nodeScrollViewport,
		child:     &child,
		id:        id,
		hasID:     true,
		focusable: true,
		scroll:    options,
	}
}

// VirtualScrollViewport returns a viewport that constructs only a visible
// content fragment
//
// contentSize declares the complete scrollable cell extent without
// constructing it. The builder is cached for one resolved visible range
// during the semantic frame and may return bounded overscan before that range.
// A nil builder produces empty fragments.
func VirtualScrollViewport[Message any](
	id NodeID,
	contentSize Size,
	builder func(VirtualViewport) VirtualFragment[Message],
) Node[Message] {
	return VirtualScrollViewportWithOptions(
		id,
		contentSize,
		DefaultScrollViewportOptions[Message](),
		builder,
	)
}

// VirtualScrollViewportWithOptions returns a virtual viewport with configured
// scrolling behavior
//
// Only the cached fragment participates in measurement, rendering, focus, hit
// testing, and event routing. Fragment Node IDs therefore represent visible or
// overscanned content and must remain stable across requests.
func VirtualScrollViewportWithOptions[Message any](
	id NodeID,
	contentSize Size,
	options ScrollViewportOptions[Message],
	builder func(VirtualViewport) VirtualFragment[Message],
) Node[Message] {
	if builder == nil {
		builder = func(VirtualViewport) VirtualFragment[Message] {
			return NewVirtualFragment(ScrollOffset{}, Column[Message]())
		}
	}
	return Node[Message]{
		kind:      nodeVirtualScrollViewport,
		id:        id,
		hasID:     true,
		focusable: true,
		scroll:    options,
		payload: &nodePayload[Message]{
			virtualSize:    contentSize,
			virtualBuilder: builder,
		},
	}
}

// VirtualFlow returns a vertical viewport for stable variable-height items
//
// Only items intersecting the visible range and bounded Cell overscan are
// built. Item heights are measured from their Nodes and retained by the
// runtime across semantic frames. The viewport has zero intrinsic height;
// assign it a layout Length or place it where the parent supplies a rectangle.
func VirtualFlow[Message any](
	id NodeID,
	source VirtualFlowSource[Message],
) Node[Message] {
	return VirtualFlowWithOptions(id, source, DefaultVirtualFlowOptions[Message]())
}

// VirtualFlowWithOptions returns a variable-height flow with configured scrolling
//
// The viewport has zero intrinsic height; assign it a layout Length or place
// it where the parent supplies a rectangle.
func VirtualFlowWithOptions[Message any](
	id NodeID,
	source VirtualFlowSource[Message],
	options VirtualFlowOptions[Message],
) Node[Message] {
	return Node[Message]{
		kind: nodeVirtualFlow, id: id, hasID: true, focusable: true,
		payload: &nodePayload[Message]{
			virtualFlow: &virtualFlowNodePayload[Message]{source: source, options: options},
		},
	}
}

// Modal marks a subtree as the active modal routing and focus scope
//
// The default focus lifecycle selects the first focusable descendant on entry
// and returns to the previously focused node on close.
func Modal[Message any](id NodeID, child Node[Message]) Node[Message] {
	return ModalWithFocus(id, child, DefaultModalFocusOptions())
}

// ModalWithFocus returns a modal scope with explicit entry and return focus policies
func ModalWithFocus[Message any](id NodeID, child Node[Message], focus ModalFocusOptions) Node[Message] {
	return Node[Message]{
		kind: nodeModal, child: &child, id: id, hasID: true,
		keyInteraction: &nodeKeyInteraction[Message]{modalFocus: focus, hasModalFocus: true},
	}
}

// WithID attaches a stable semantic identity without changing focus behavior
func (n Node[Message]) WithID(id NodeID) Node[Message] {
	n.id = id
	n.hasID = true
	return n
}

// BlockUnhandledEvents consumes routed events that remain unhandled after this
// identified node has processed its actions, built-in behavior, pointer
// handler, and raw handler
//
// The boundary prevents the event from reaching ancestors and terminal-level
// fallback mapping. It has no effect until the node has a stable identity.
func (n Node[Message]) BlockUnhandledEvents() Node[Message] {
	n.blocksUnhandled = true
	return n
}

// Focusable makes the node focusable under a stable identity
func (n Node[Message]) Focusable(id NodeID) Node[Message] {
	n.id = id
	n.hasID = true
	n.focusable = true
	return n
}

// TabStop controls Tab traversal participation without changing the stable ID
func (n Node[Message]) TabStop(enabled bool) Node[Message] {
	n.focusable = enabled
	return n
}

// WithFocusedStyle merges style over this node's clipped rectangle while it
// owns focus. It does not change measurement, layout, hit testing, or routing.
// The node must also have a stable identity, normally through Focusable or
// OnEvent.
func (n Node[Message]) WithFocusedStyle(style vt.Style) Node[Message] {
	n.focusedStyle = style
	n.hasFocusedStyle = true
	return n
}

// FocusFallback prefers a stable focus target when a focused node in this subtree disappears
//
// The target must remain focusable in the next frame and belong to the active
// modal scope. Nested declarations override outer declarations. When the
// target is unavailable, normal deterministic reconciliation is used.
func (n Node[Message]) FocusFallback(target NodeID) Node[Message] {
	n.keyInteraction = cloneNodeKeyInteraction(n.keyInteraction)
	n.keyInteraction.focusFallback = target
	n.keyInteraction.hasFocusFallback = true
	return n
}

// OnEvent attaches an event handler under a stable identity
func (n Node[Message]) OnEvent(id NodeID, handler func(vt.Event) EventResult[Message]) Node[Message] {
	n.id = id
	n.hasID = true
	n.handler = handler
	return n
}

// OnPointerEvent attaches a geometry-aware mouse event handler under a stable identity
//
// The handler receives Node-local coordinates, clipping, Runtime width policy,
// paragraph text hit information, and the nearest ScrollViewport Raw OnEvent
// handling remains independent and runs afterward when this handler does not
// consume the event
func (n Node[Message]) OnPointerEvent(
	id NodeID,
	handler func(PointerEventContext) EventResult[Message],
) Node[Message] {
	n.id = id
	n.hasID = true
	n.pointerHandler = handler
	return n
}

// OnActions attaches a complete semantic action group under a stable owner
// identity. A later call replaces the complete group.
func (n Node[Message]) OnActions(id NodeID, actions []Action[Message]) Node[Message] {
	n.id = id
	n.hasID = true
	n.keyInteraction = cloneNodeKeyInteraction(n.keyInteraction)
	n.keyInteraction.actions = append([]Action[Message](nil), actions...)
	return n
}

// WithKeyScope attaches one immutable KeyMap scope to this semantic node
//
// The scope ID becomes the node identity. Later identity modifiers may replace
// it without retaining a stale scope identity. A later call replaces the
// complete scope.
func (n Node[Message]) WithKeyScope(scope KeyScope) Node[Message] {
	n.id = scope.id
	n.hasID = true
	n.keyInteraction = cloneNodeKeyInteraction(n.keyInteraction)
	n.keyInteraction.scope = &nodeKeyScope{
		keyMap:      scope.keyMap,
		propagation: scope.propagation,
	}
	return n
}

// RevealDescendant keeps an identified descendant visible inside this ScrollViewport
//
// The target must be present below an eager ScrollViewport or in the current
// fragment of a virtual ScrollViewport. A later call replaces the target. On
// other node kinds this metadata has no effect.
func (n Node[Message]) RevealDescendant(target NodeID) Node[Message] {
	if n.kind != nodeScrollViewport && n.kind != nodeVirtualScrollViewport && n.kind != nodeVirtualFlow {
		return n
	}
	n.keyInteraction = cloneNodeKeyInteraction(n.keyInteraction)
	n.keyInteraction.revealTarget = target
	n.keyInteraction.hasRevealTarget = true
	return n
}

func cloneNodeKeyInteraction[Message any](
	interaction *nodeKeyInteraction[Message],
) *nodeKeyInteraction[Message] {
	cloned := &nodeKeyInteraction[Message]{}
	if interaction == nil {
		return cloned
	}
	cloned.actions = interaction.actions
	cloned.revealTarget = interaction.revealTarget
	cloned.hasRevealTarget = interaction.hasRevealTarget
	cloned.modalFocus = interaction.modalFocus
	cloned.hasModalFocus = interaction.hasModalFocus
	cloned.focusFallback = interaction.focusFallback
	cloned.hasFocusFallback = interaction.hasFocusFallback
	if interaction.scope != nil {
		scope := *interaction.scope
		cloned.scope = &scope
	}
	return cloned
}

// WithLength sets the node's main-axis sizing rule in a row or column
func (n Node[Message]) WithLength(length Length) Node[Message] {
	n.length = length
	return n
}

func (n Node[Message]) renderTo(target *surface.Surface, interaction *InteractionState) {
	n.renderToProfile(target, interaction, celltext.ModernWidth())
}

func (n Node[Message]) renderToProfile(target *surface.Surface, interaction *InteractionState, profile celltext.WidthProfile) {
	bounds := Rect{Width: target.Width(), Height: target.Height()}
	n.render(target, bounds, bounds, interaction, profile)
}

func (n Node[Message]) measure(constraints layoutConstraints, profile celltext.WidthProfile) Size {
	var measured Size
	switch n.kind {
	case nodeText:
		measured = measureText(n.content, constraints, profile)
	case nodeRichText:
		measured = measureRichText(n.spans, n.paragraph, n.richTextCache, constraints, profile)
	case nodeSurface:
		if n.payload != nil && n.payload.surface != nil {
			measured = Size{Width: n.payload.surface.Width(), Height: n.payload.surface.Height()}
		}
	case nodeSpacer:
		measured = n.intrinsicSize
	case nodeGap:
		measured = Size{}
	case nodeCursorAnchor:
		measured = Size{Height: 1}
	case nodeTextInput:
		measured = Size{
			Width:  intToUint32(max(celltext.Width(n.content, profile), celltext.Width(n.placeholder, profile))),
			Height: 1,
		}
	case nodeRow:
		measured = measureLinear(n.children, constraints, true, profile)
	case nodeColumn:
		measured = measureLinear(n.children, constraints, false, profile)
	case nodeResponsiveRow:
		measured = measureResponsiveRow(n.payload.responsive, constraints, profile)
	case nodeSplitPane:
		measured = measureSplitPane(n.children[0], n.children[1], constraints, n.split, profile)
	case nodeStack:
		for _, child := range n.children {
			childSize := child.measure(constraints, profile)
			measured.Width = max(measured.Width, childSize.Width)
			measured.Height = max(measured.Height, childSize.Height)
		}
	case nodeOverlay:
		measured = n.children[0].measure(constraints, profile)
	case nodeAnchoredOverlay:
		measured = n.payload.anchored.base.measure(constraints, profile)
	case nodePadding:
		measured = addNodeSize(
			n.child.measure(shrinkConstraints(constraints, n.insets), profile),
			saturatingAdd32(n.insets.Left, n.insets.Right),
			saturatingAdd32(n.insets.Top, n.insets.Bottom),
		)
	case nodeBorder:
		measured = addNodeSize(
			n.child.measure(shrinkConstraints(constraints, UniformInsets(1)), profile),
			2,
			2,
		)
	case nodePanel:
		insets := panelContentInsets(n.panel)
		measured = addNodeSize(
			n.child.measure(shrinkConstraints(constraints, insets), profile),
			saturatingAdd32(insets.Left, insets.Right),
			saturatingAdd32(insets.Top, insets.Bottom),
		)
	case nodeAlign, nodeClip, nodeScrollViewport, nodeModal:
		measured = n.child.measure(constraints, profile)
	case nodeVirtualScrollViewport:
		measured = n.payload.virtualSize
	case nodeVirtualFlow:
		measured = virtualFlowIntrinsicSize(constraints)
	default:
		panic("nagi-tui: invalid node kind")
	}
	return clampNodeSize(measured, constraints)
}

func measureSplitPane[Message any](
	primary, secondary Node[Message],
	constraints layoutConstraints,
	options SplitPaneOptions,
	profile celltext.WidthProfile,
) Size {
	childConstraints := constraints
	axis := normalizedSplitPaneAxis(options.Axis)
	if axis == SplitPaneVertical {
		childConstraints.height = layoutLimit{}
	} else {
		childConstraints.width = layoutLimit{}
	}
	primarySize := primary.measure(childConstraints, profile)
	secondarySize := secondary.measure(childConstraints, profile)
	if axis == SplitPaneVertical {
		return Size{
			Width:  max(primarySize.Width, secondarySize.Width),
			Height: saturatingAdd32(saturatingAdd32(primarySize.Height, 1), secondarySize.Height),
		}
	}
	return Size{
		Width:  saturatingAdd32(saturatingAdd32(primarySize.Width, 1), secondarySize.Width),
		Height: max(primarySize.Height, secondarySize.Height),
	}
}

func measureText(content string, constraints layoutConstraints, profile celltext.WidthProfile) Size {
	maxCells := math.MaxInt
	if constraints.width.bounded {
		maxCells = int(constraints.width.value)
	}
	lines := celltext.IterateWrappedLines(content, maxCells, profile)
	width, height := 0, 0
	for line, ok := lines.Next(); ok; line, ok = lines.Next() {
		width = max(width, line.Width)
		height++
	}
	return Size{Width: intToUint32(width), Height: intToUint32(height)}
}

func measureLinear[Message any](children []Node[Message], constraints layoutConstraints, horizontal bool, profile celltext.WidthProfile) Size {
	childConstraints := constraints
	if horizontal {
		childConstraints.width = layoutLimit{}
	} else {
		childConstraints.height = layoutLimit{}
	}
	var measured Size
	for _, child := range children {
		childSize := child.measure(childConstraints, profile)
		if child.kind == nodeGap {
			if horizontal {
				childSize.Width = child.gap
			} else {
				childSize.Height = child.gap
			}
		}
		if horizontal {
			measured.Width = saturatingAdd32(measured.Width, childSize.Width)
			measured.Height = max(measured.Height, childSize.Height)
		} else {
			measured.Width = max(measured.Width, childSize.Width)
			measured.Height = saturatingAdd32(measured.Height, childSize.Height)
		}
	}
	return measured
}

func shrinkConstraints(constraints layoutConstraints, insets Insets) layoutConstraints {
	constraints.width = subtractLimit(constraints.width, saturatingAdd32(insets.Left, insets.Right))
	constraints.height = subtractLimit(constraints.height, saturatingAdd32(insets.Top, insets.Bottom))
	return constraints
}

func subtractLimit(limit layoutLimit, value uint32) layoutLimit {
	if limit.bounded {
		limit.value -= min(limit.value, value)
	}
	return limit
}

func addNodeSize(size Size, horizontal, vertical uint32) Size {
	return Size{
		Width:  saturatingAdd32(size.Width, horizontal),
		Height: saturatingAdd32(size.Height, vertical),
	}
}

func clampNodeSize(size Size, constraints layoutConstraints) Size {
	if constraints.width.bounded {
		size.Width = min(size.Width, constraints.width.value)
	}
	if constraints.height.bounded {
		size.Height = min(size.Height, constraints.height.value)
	}
	return size
}

func intToUint32(value int) uint32 {
	if value <= 0 {
		return 0
	}
	return uint32(min(uint64(value), uint64(math.MaxUint32)))
}
