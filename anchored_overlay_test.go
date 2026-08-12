package tui

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go/internal/conformance"
)

func TestAnchoredOverlayPlacementMatchesSharedFixtures(t *testing.T) {
	records, err := conformance.Load(
		"layout/anchored-overlay.txt",
		"anchored-overlay-placement",
		"boundary", "anchor", "anchor-clip", "overlay", "side", "alignment", "gap",
		"fallback", "max-width", "max-height", "expected",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			overlaySize := anchoredFixtureSize(t, record.Field("overlay"))
			options := AnchoredOverlayOptions{
				Side:          anchoredFixtureSide(t, record.Field("side")),
				Alignment:     anchoredFixtureAlignment(t, record.Field("alignment")),
				Gap:           anchoredFixtureUint(t, record.Field("gap")),
				Fallback:      anchoredFixtureFallback(t, record.Field("fallback")),
				MaximumWidth:  anchoredFixtureUint(t, record.Field("max-width")),
				MaximumHeight: anchoredFixtureUint(t, record.Field("max-height")),
			}
			actual, ok := resolveAnchoredOverlayRect(
				anchoredFixtureRect(t, record.Field("anchor")),
				anchoredFixtureRect(t, record.Field("anchor-clip")),
				anchoredFixtureRect(t, record.Field("boundary")),
				anchoredNode(overlaySize),
				options,
				celltext.ModernWidth(),
			)
			if record.Field("expected") == "none" {
				if ok {
					t.Fatalf("placement = %+v, want none", actual)
				}
				return
			}
			expected := anchoredFixtureRect(t, record.Field("expected"))
			if !ok || actual != expected {
				t.Fatalf("placement = %+v, %t, want %+v", actual, ok, expected)
			}
		})
	}
}

func TestAnchoredOverlayMeasurementIsExactlyBaseMeasurement(t *testing.T) {
	base := Spacer[struct{}](2, 3)
	overlay := Spacer[struct{}](20, 30)
	anchored := AnchoredOverlay(base, NewNodeID("anchor"), overlay)
	if actual := anchored.measure(layoutConstraints{}, celltext.ModernWidth()); actual != (Size{Width: 2, Height: 3}) {
		t.Fatalf("measure = %+v, want 2x3", actual)
	}
}

func TestAnchoredOverlayWarmedPlacementDoesNotAllocate(t *testing.T) {
	overlay := Spacer[struct{}](8, 4)
	anchor := Rect{X: 3, Y: 2, Width: 1, Height: 1}
	boundary := Rect{Width: 20, Height: 10}
	options := DefaultAnchoredOverlayOptions()
	if _, ok := resolveAnchoredOverlayRect(anchor, anchor, boundary, &overlay, options, celltext.ModernWidth()); !ok {
		t.Fatal("warmup placement is absent")
	}
	allocations := testing.AllocsPerRun(1000, func() {
		if _, ok := resolveAnchoredOverlayRect(anchor, anchor, boundary, &overlay, options, celltext.ModernWidth()); !ok {
			panic("placement is absent")
		}
	})
	if allocations != 0 {
		t.Fatalf("warmed placement allocations = %f, want 0", allocations)
	}
}

type anchoredOverlayMessage uint8

const (
	anchoredBaseMessage anchoredOverlayMessage = iota
	anchoredFrontMessage
)

type anchoredOverlayApp struct {
	anchorPresent bool
	cursorAnchor  bool
	messages      []anchoredOverlayMessage
}

func (*anchoredOverlayApp) Init() Effect[anchoredOverlayMessage] {
	return NoneEffect[anchoredOverlayMessage]()
}

func (a *anchoredOverlayApp) Update(message anchoredOverlayMessage) Effect[anchoredOverlayMessage] {
	a.messages = append(a.messages, message)
	return NoneEffect[anchoredOverlayMessage]()
}

func (*anchoredOverlayApp) Subscriptions() Subscription[anchoredOverlayMessage] {
	return NoneSubscription[anchoredOverlayMessage]()
}

func (a *anchoredOverlayApp) View(ViewContext) Node[anchoredOverlayMessage] {
	anchorID := NewNodeID("other")
	if a.anchorPresent {
		anchorID = NewNodeID("anchor")
	}
	anchor := Text[anchoredOverlayMessage]("anchor").WithID(anchorID)
	anchorRow := anchor
	if a.cursorAnchor {
		anchorRow = Row(
			Text[anchoredOverlayMessage]("A"),
			CursorAnchor[anchoredOverlayMessage](NewNodeID("owner")).WithID(anchorID),
			Text[anchoredOverlayMessage]("B"),
		)
	}
	base := Column(
		anchorRow,
		Text[anchoredOverlayMessage]("base").OnEvent(NewNodeID("base"), func(event vt.Event) EventResult[anchoredOverlayMessage] {
			if event.Kind == vt.EventMouse {
				return ConsumeResult[anchoredOverlayMessage]().Emit(anchoredBaseMessage)
			}
			return IgnoreResult[anchoredOverlayMessage]()
		}),
	)
	front := Text[anchoredOverlayMessage]("popup").OnEvent(NewNodeID("overlay"), func(event vt.Event) EventResult[anchoredOverlayMessage] {
		if event.Kind == vt.EventMouse {
			return ConsumeResult[anchoredOverlayMessage]().Emit(anchoredFrontMessage)
		}
		return IgnoreResult[anchoredOverlayMessage]()
	})
	return AnchoredOverlay(base, NewNodeID("anchor"), front)
}

func TestAnchoredOverlayRendersAndRoutesAfterOverlappingBase(t *testing.T) {
	app := &anchoredOverlayApp{anchorPresent: true}
	runtime, err := NewRuntimeWithClock(
		app, NewRuntimeConfig(Size{Width: 8, Height: 3}), NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	frame, err := runtime.RenderIfDirty()
	if err != nil || frame == nil {
		t.Fatalf("render = %v, %v", frame, err)
	}
	cell, ok := frame.Surface().Cell(0, 1)
	if !ok || cell.Content() != "p" {
		t.Fatalf("front cell = %#v, %t, want p", cell, ok)
	}
	dispatch, err := runtime.DispatchEvent(anchoredPointer(0, 1))
	if err != nil || !dispatch.Consumed() {
		t.Fatalf("dispatch = %+v, %v", dispatch, err)
	}
	if _, err := runtime.ProcessPending(); err != nil {
		t.Fatal(err)
	}
	if !slicesEqual(app.messages, []anchoredOverlayMessage{anchoredFrontMessage}) {
		t.Fatalf("messages = %#v, want overlay", app.messages)
	}
}

func TestAnchoredOverlayMissingAnchorOmitsFrontLayer(t *testing.T) {
	app := &anchoredOverlayApp{}
	runtime, err := NewRuntimeWithClock(
		app, NewRuntimeConfig(Size{Width: 8, Height: 3}), NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	frame, err := runtime.RenderIfDirty()
	if err != nil || frame == nil {
		t.Fatalf("render = %v, %v", frame, err)
	}
	cell, ok := frame.Surface().Cell(0, 1)
	if !ok || cell.Content() != "b" {
		t.Fatalf("base cell = %#v, %t, want b", cell, ok)
	}
	dispatch, err := runtime.DispatchEvent(anchoredPointer(0, 1))
	if err != nil || !dispatch.Consumed() {
		t.Fatalf("dispatch = %+v, %v", dispatch, err)
	}
	if _, err := runtime.ProcessPending(); err != nil {
		t.Fatal(err)
	}
	if !slicesEqual(app.messages, []anchoredOverlayMessage{anchoredBaseMessage}) {
		t.Fatalf("messages = %#v, want base", app.messages)
	}
}

func TestAnchoredOverlayUsesZeroWidthCursorAsPlacementPoint(t *testing.T) {
	app := &anchoredOverlayApp{anchorPresent: true, cursorAnchor: true}
	runtime, err := NewRuntimeWithClock(
		app, NewRuntimeConfig(Size{Width: 8, Height: 3}), NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	frame, err := runtime.RenderIfDirty()
	if err != nil || frame == nil {
		t.Fatalf("render = %v, %v", frame, err)
	}
	cell, ok := frame.Surface().Cell(1, 1)
	if !ok || cell.Content() != "p" {
		t.Fatalf("cursor popup cell = %#v, %t, want p", cell, ok)
	}
}

func anchoredPointer(x, y uint32) vt.Event {
	return vt.Event{Kind: vt.EventMouse, Mouse: vt.MouseEvent{
		Kind: vt.MousePress, Button: vt.MouseLeft, X: x, Y: y,
	}}
}

func slicesEqual[Value comparable](left, right []Value) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func anchoredNode(size Size) *Node[struct{}] {
	node := Spacer[struct{}](size.Width, size.Height)
	return &node
}

func anchoredFixtureRect(t *testing.T, value string) Rect {
	t.Helper()
	parts := strings.Split(value, ":")
	if len(parts) != 4 {
		t.Fatalf("invalid Rect %q", value)
	}
	return Rect{
		X:      int32(anchoredFixtureInt(t, parts[0])),
		Y:      int32(anchoredFixtureInt(t, parts[1])),
		Width:  uint32(anchoredFixtureInt(t, parts[2])),
		Height: uint32(anchoredFixtureInt(t, parts[3])),
	}
}

func anchoredFixtureSize(t *testing.T, value string) Size {
	t.Helper()
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		t.Fatalf("invalid Size %q", value)
	}
	return Size{
		Width:  uint32(anchoredFixtureInt(t, parts[0])),
		Height: uint32(anchoredFixtureInt(t, parts[1])),
	}
}

func anchoredFixtureInt(t *testing.T, value string) int {
	t.Helper()
	number, err := strconv.Atoi(value)
	if err != nil {
		t.Fatal(err)
	}
	return number
}

func anchoredFixtureUint(t *testing.T, value string) uint32 {
	t.Helper()
	return uint32(anchoredFixtureInt(t, value))
}

func anchoredFixtureSide(t *testing.T, value string) AnchoredOverlaySide {
	t.Helper()
	switch value {
	case "below":
		return AnchoredOverlayBelow
	case "above":
		return AnchoredOverlayAbove
	default:
		t.Fatalf("invalid side %q", value)
		return 0
	}
}

func anchoredFixtureAlignment(t *testing.T, value string) HorizontalAlignment {
	t.Helper()
	switch value {
	case "start":
		return AlignStart
	case "center":
		return AlignCenter
	case "end":
		return AlignEnd
	default:
		t.Fatalf("invalid alignment %q", value)
		return 0
	}
}

func anchoredFixtureFallback(t *testing.T, value string) AnchoredOverlayFallback {
	t.Helper()
	switch value {
	case "flip":
		return AnchoredOverlayFlip
	case "clip":
		return AnchoredOverlayClip
	default:
		t.Fatalf("invalid fallback %q", value)
		return 0
	}
}
