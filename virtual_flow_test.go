package tui

import (
	"errors"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func TestVirtualFlowItemsRejectDuplicatesAndCopyInput(t *testing.T) {
	input := []VirtualFlowItem{NewVirtualFlowItem("a"), NewVirtualFlowItem("b")}
	items, err := NewVirtualFlowItems(input)
	if err != nil {
		t.Fatal(err)
	}
	input[0] = NewVirtualFlowItem("changed")
	first, _ := items.Item(0)
	if first.Key() != "a" {
		t.Fatalf("first key = %q, want a", first.Key())
	}

	_, err = NewVirtualFlowItems([]VirtualFlowItem{
		NewVirtualFlowItem("same"), NewVirtualFlowItem("same"),
	})
	var duplicate *DuplicateVirtualFlowItemKeyError
	if !errors.As(err, &duplicate) || duplicate.Key != "same" {
		t.Fatalf("duplicate error = %v", err)
	}
}

func TestVirtualFlowEstimatesScanOncePerStructuralOrWidthChange(t *testing.T) {
	items, err := NewVirtualFlowItems([]VirtualFlowItem{
		NewVirtualFlowItem("a"), NewVirtualFlowItem("b"), NewVirtualFlowItem("c"),
	})
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int64
	source := NewVirtualFlowSource(items, func(VirtualFlowItemContext) Node[struct{}] {
		return Spacer[struct{}](0, 1)
	}).EstimatedHeight(func(VirtualFlowItemContext) uint32 {
		calls.Add(1)
		return 1
	})
	flow := &virtualFlowInteraction{}

	if !reconcileVirtualFlow(flow, source, 8) || calls.Load() != 3 {
		t.Fatalf("initial reconcile calls = %d", calls.Load())
	}
	if reconcileVirtualFlow(flow, source, 8) || calls.Load() != 3 {
		t.Fatalf("unchanged reconcile calls = %d", calls.Load())
	}
	if !reconcileVirtualFlow(flow, source, 4) || calls.Load() != 6 {
		t.Fatalf("width reconcile calls = %d", calls.Load())
	}
}

func TestVirtualFlowTransitionsMatchSharedFixtures(t *testing.T) {
	records := interactionRecords(
		t,
		"interaction/virtual-flow.txt",
		"virtual-flow-transition",
		"initial",
		"next",
		"viewport",
		"initial-width",
		"next-width",
		"stick",
		"request",
		"overscan",
		"update",
		"expected-offset",
		"expected-maximum",
		"expected-at-end",
		"expected-anchor",
		"expected-visible",
		"expected-built",
	)
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			initial := fixtureVirtualFlowSource(t, record.Field("initial"), ResetVirtualFlowUpdate(1))
			next := fixtureVirtualFlowSource(t, record.Field("next"), fixtureVirtualFlowUpdate(t, record.Field("update")))
			viewport := fixtureUint32(t, record.Field("viewport"))
			initialWidth := fixtureUint32(t, record.Field("initial-width"))
			nextWidth := fixtureUint32(t, record.Field("next-width"))
			overscan := fixtureUint32(t, record.Field("overscan"))
			stickToEnd := fixtureVirtualFlowBool(t, record.Field("stick"))
			state := NewInteractionState()
			const id NodeID = "flow"

			prepareVirtualFlow(state, id, initial, initialWidth, viewport, overscan, stickToEnd)
			if request := record.Field("request"); request != "-" {
				state.requestScroll(id, ScrollOffset{Y: fixtureUint32(t, request)})
				prepareVirtualFlow(state, id, initial, initialWidth, viewport, overscan, stickToEnd)
			}

			window := prepareVirtualFlow(state, id, next, nextWidth, viewport, overscan, stickToEnd)
			flow, ok := state.VirtualFlowState(id)
			if !ok {
				t.Fatal("virtual flow state is missing")
			}
			if expected := fixtureUint32(t, record.Field("expected-offset")); flow.Scroll.Offset.Y != expected {
				t.Fatalf("offset = %d, want %d", flow.Scroll.Offset.Y, expected)
			}
			if expected := fixtureUint32(t, record.Field("expected-maximum")); flow.Scroll.Maximum.Y != expected {
				t.Fatalf("maximum = %d, want %d", flow.Scroll.Maximum.Y, expected)
			}
			if expected := fixtureVirtualFlowBool(t, record.Field("expected-at-end")); flow.Scroll.AtEnd != expected {
				t.Fatalf("at-end = %t, want %t", flow.Scroll.AtEnd, expected)
			}
			visibleStart, visibleEnd := fixtureVirtualFlowRange(t, record.Field("expected-visible"))
			if flow.VisibleStart != visibleStart || flow.VisibleEnd != visibleEnd {
				t.Fatalf("visible = %d:%d, want %d:%d", flow.VisibleStart, flow.VisibleEnd, visibleStart, visibleEnd)
			}
			builtStart, builtEnd := fixtureVirtualFlowRange(t, record.Field("expected-built"))
			if window.builtStart != builtStart || window.builtEnd != builtEnd {
				t.Fatalf("built = %d:%d, want %d:%d", window.builtStart, window.builtEnd, builtStart, builtEnd)
			}
			assertFixtureVirtualFlowAnchor(t, flow, record.Field("expected-anchor"))
		})
	}
}

func fixtureVirtualFlowSource(
	t *testing.T,
	value string,
	update VirtualFlowUpdate,
) VirtualFlowSource[struct{}] {
	t.Helper()
	heights := make(map[NodeID]uint32)
	items := make([]VirtualFlowItem, 0)
	if value != "-" {
		for _, entry := range strings.Split(value, ",") {
			key, height, ok := strings.Cut(entry, ":")
			if !ok {
				t.Fatalf("invalid virtual flow item %q", entry)
			}
			id := NodeID(key)
			heights[id] = fixtureUint32(t, height)
			items = append(items, NewVirtualFlowItem(id))
		}
	}
	order, err := NewVirtualFlowItems(items)
	if err != nil {
		t.Fatal(err)
	}
	return NewVirtualFlowSource(order, func(VirtualFlowItemContext) Node[struct{}] {
		return Spacer[struct{}](0, 1)
	}).EstimatedHeight(func(context VirtualFlowItemContext) uint32 {
		return heights[context.Key]
	}).Update(update)
}

func fixtureVirtualFlowUpdate(t *testing.T, value string) VirtualFlowUpdate {
	t.Helper()
	parts := strings.Split(value, ":")
	switch parts[0] {
	case "same":
		if len(parts) != 1 {
			t.Fatalf("invalid virtual flow update %q", value)
		}
		return ChangedVirtualFlowUpdate(1, 1, 0, 0)
	case "reset":
		if len(parts) != 2 {
			t.Fatalf("invalid virtual flow update %q", value)
		}
		return ResetVirtualFlowUpdate(fixtureVirtualFlowUint64(t, parts[1]))
	case "changed":
		if len(parts) != 5 {
			t.Fatalf("invalid virtual flow update %q", value)
		}
		return ChangedVirtualFlowUpdate(
			fixtureVirtualFlowUint64(t, parts[2]),
			fixtureVirtualFlowUint64(t, parts[1]),
			fixtureVirtualFlowInt(t, parts[3]),
			fixtureVirtualFlowInt(t, parts[4]),
		)
	default:
		t.Fatalf("invalid virtual flow update %q", value)
		return VirtualFlowUpdate{}
	}
}

func assertFixtureVirtualFlowAnchor(t *testing.T, actual VirtualFlowState, expected string) {
	t.Helper()
	if expected == "-" {
		if actual.HasAnchor {
			t.Fatalf("anchor = %+v, want none", actual.Anchor)
		}
		return
	}
	parts := strings.Split(expected, ":")
	if len(parts) != 3 {
		t.Fatalf("invalid virtual flow anchor %q", expected)
	}
	affinity := VirtualFlowAnchorStart
	switch parts[1] {
	case "start":
	case "end":
		affinity = VirtualFlowAnchorEnd
	default:
		t.Fatalf("invalid virtual flow affinity %q", parts[1])
	}
	want := VirtualFlowAnchor{
		Key: NodeID(parts[0]), Offset: fixtureUint32(t, parts[2]), Affinity: affinity,
	}
	if !actual.HasAnchor || actual.Anchor != want {
		t.Fatalf("anchor = %+v, present = %t, want %+v", actual.Anchor, actual.HasAnchor, want)
	}
}

func fixtureVirtualFlowRange(t *testing.T, value string) (int, int) {
	t.Helper()
	start, end, ok := strings.Cut(value, ":")
	if !ok {
		t.Fatalf("invalid virtual flow range %q", value)
	}
	return fixtureVirtualFlowInt(t, start), fixtureVirtualFlowInt(t, end)
}

func fixtureVirtualFlowBool(t *testing.T, value string) bool {
	t.Helper()
	switch value {
	case "0":
		return false
	case "1":
		return true
	default:
		t.Fatalf("invalid fixture boolean %q", value)
		return false
	}
}

func fixtureVirtualFlowInt(t *testing.T, value string) int {
	t.Helper()
	parsed, err := strconv.Atoi(value)
	if err != nil {
		t.Fatalf("parse %q: %v", value, err)
	}
	return parsed
}

func fixtureVirtualFlowUint64(t *testing.T, value string) uint64 {
	t.Helper()
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		t.Fatalf("parse %q: %v", value, err)
	}
	return parsed
}
