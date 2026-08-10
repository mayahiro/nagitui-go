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
}

// Node is a semantic view node rebuilt by an application for each frame
type Node[Message any] struct {
	kind             nodeKind
	content          string
	style            vt.Style
	spans            []TextSpan
	paragraph        ParagraphOptions
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
	focusable        bool
	focusedStyle     vt.Style
	hasFocusedStyle  bool
	handler          eventHandler[Message]
	keyInteraction   *nodeKeyInteraction[Message]
	onChange         func(string) Message
	placeholder      string
	placeholderStyle vt.Style
	scroll           ScrollViewportOptions[Message]
}

type nodeKind uint8

const (
	nodeText nodeKind = iota
	nodeRichText
	nodeSurface
	nodeSpacer
	nodeGap
	nodeTextInput
	nodeRow
	nodeColumn
	nodeStack
	nodePadding
	nodeBorder
	nodeAlign
	nodeClip
	nodeScrollViewport
	nodeVirtualScrollViewport
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
	return Node[Message]{
		kind:          nodeRichText,
		spans:         cloneTextSpans(spans),
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

// Stack returns a front-to-back overlay container
//
// The returned node retains the supplied variadic slice. Callers must not
// mutate that slice after construction.
func Stack[Message any](children ...Node[Message]) Node[Message] {
	return Node[Message]{kind: nodeStack, children: children}
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

// Modal marks a subtree as the active modal routing and focus scope
func Modal[Message any](id NodeID, child Node[Message]) Node[Message] {
	return Node[Message]{kind: nodeModal, child: &child, id: id, hasID: true}
}

// WithID attaches a stable semantic identity without changing focus behavior
func (n Node[Message]) WithID(id NodeID) Node[Message] {
	n.id = id
	n.hasID = true
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

// OnEvent attaches an event handler under a stable identity
func (n Node[Message]) OnEvent(id NodeID, handler func(vt.Event) EventResult[Message]) Node[Message] {
	n.id = id
	n.hasID = true
	n.handler = handler
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

func cloneNodeKeyInteraction[Message any](
	interaction *nodeKeyInteraction[Message],
) *nodeKeyInteraction[Message] {
	cloned := &nodeKeyInteraction[Message]{}
	if interaction == nil {
		return cloned
	}
	cloned.actions = interaction.actions
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
	bounds := Rect{Width: target.Width(), Height: target.Height()}
	n.render(target, bounds, bounds, interaction)
}

func (n Node[Message]) measure(constraints layoutConstraints) Size {
	var measured Size
	switch n.kind {
	case nodeText:
		measured = measureText(n.content, constraints)
	case nodeRichText:
		measured = measureRichText(n.spans, n.paragraph, n.richTextCache, constraints)
	case nodeSurface:
		if n.payload != nil && n.payload.surface != nil {
			measured = Size{Width: n.payload.surface.Width(), Height: n.payload.surface.Height()}
		}
	case nodeSpacer:
		measured = n.intrinsicSize
	case nodeGap:
		measured = Size{}
	case nodeTextInput:
		measured = Size{
			Width:  intToUint32(max(celltext.Width(n.content, celltext.ModernWidth()), celltext.Width(n.placeholder, celltext.ModernWidth()))),
			Height: 1,
		}
	case nodeRow:
		measured = measureLinear(n.children, constraints, true)
	case nodeColumn:
		measured = measureLinear(n.children, constraints, false)
	case nodeStack:
		for _, child := range n.children {
			childSize := child.measure(constraints)
			measured.Width = max(measured.Width, childSize.Width)
			measured.Height = max(measured.Height, childSize.Height)
		}
	case nodePadding:
		measured = addNodeSize(
			n.child.measure(shrinkConstraints(constraints, n.insets)),
			saturatingAdd32(n.insets.Left, n.insets.Right),
			saturatingAdd32(n.insets.Top, n.insets.Bottom),
		)
	case nodeBorder:
		measured = addNodeSize(
			n.child.measure(shrinkConstraints(constraints, UniformInsets(1))),
			2,
			2,
		)
	case nodePanel:
		insets := panelContentInsets(n.panel)
		measured = addNodeSize(
			n.child.measure(shrinkConstraints(constraints, insets)),
			saturatingAdd32(insets.Left, insets.Right),
			saturatingAdd32(insets.Top, insets.Bottom),
		)
	case nodeAlign, nodeClip, nodeScrollViewport, nodeModal:
		measured = n.child.measure(constraints)
	case nodeVirtualScrollViewport:
		measured = n.payload.virtualSize
	default:
		panic("nagi-tui: invalid node kind")
	}
	return clampNodeSize(measured, constraints)
}

func measureText(content string, constraints layoutConstraints) Size {
	maxCells := math.MaxInt
	if constraints.width.bounded {
		maxCells = int(constraints.width.value)
	}
	lines := celltext.IterateWrappedLines(content, maxCells, celltext.ModernWidth())
	width, height := 0, 0
	for line, ok := lines.Next(); ok; line, ok = lines.Next() {
		width = max(width, line.Width)
		height++
	}
	return Size{Width: intToUint32(width), Height: intToUint32(height)}
}

func measureLinear[Message any](children []Node[Message], constraints layoutConstraints, horizontal bool) Size {
	childConstraints := constraints
	if horizontal {
		childConstraints.width = layoutLimit{}
	} else {
		childConstraints.height = layoutLimit{}
	}
	var measured Size
	for _, child := range children {
		childSize := child.measure(childConstraints)
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
