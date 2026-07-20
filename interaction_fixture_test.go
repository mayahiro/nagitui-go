package tui

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mayahiro/nagitui-go/internal/conformance"
)

func TestFocusTransitionsMatchSharedFixtures(t *testing.T) {
	records := interactionRecords(t, "interaction/focus.txt", "focus-transition", "previous", "current", "focused", "action", "expected")
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			previous := fixtureNodeIDs(record.Field("previous"))
			current := fixtureNodeIDs(record.Field("current"))
			focused := fixtureOptionalNodeID(record.Field("focused"))
			var actual NodeID
			var ok bool
			switch record.Field("action") {
			case "reconcile":
				actual, ok = reconcileFocus(previous, current, focused)
			case "next":
				actual, ok = traverseFocus(current, focused, true)
			case "previous":
				actual, ok = traverseFocus(current, focused, false)
			default:
				t.Fatalf("invalid action %q", record.Field("action"))
			}
			expected := fixtureOptionalNodeID(record.Field("expected"))
			if expected == nil {
				if ok {
					t.Fatalf("focus = %q, true; want none", actual)
				}
			} else if !ok || actual != *expected {
				t.Fatalf("focus = %q, %t; want %q, true", actual, ok, *expected)
			}
		})
	}
}

func TestInteractionRetirementMatchesSharedFixtures(t *testing.T) {
	records := interactionRecords(
		t,
		"interaction/retirement.txt",
		"interaction-retirement",
		"previous-focus",
		"current-focus",
		"active",
		"focused",
		"capture",
		"text",
		"scroll",
		"expected-focused",
		"expected-capture",
		"expected-text",
		"expected-scroll",
	)
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			state := NewInteractionState()
			if focused := fixtureOptionalNodeID(record.Field("focused")); focused != nil {
				state.focused = *focused
				state.hasFocus = true
			}
			if capture := fixtureOptionalNodeID(record.Field("capture")); capture != nil {
				state.pointerCapture = *capture
				state.hasCapture = true
			}
			for _, id := range fixtureNodeIDs(record.Field("text")) {
				state.textInputs[id] = &TextInputState{}
			}
			for _, id := range fixtureNodeIDs(record.Field("scroll")) {
				state.scrolls[id] = &scrollInteraction{}
			}
			active := make(map[NodeID]struct{})
			for _, id := range fixtureNodeIDs(record.Field("active")) {
				active[id] = struct{}{}
			}

			state.reconcile(
				active,
				fixtureNodeIDs(record.Field("previous-focus")),
				fixtureNodeIDs(record.Field("current-focus")),
			)

			assertOptionalNodeID(t, "focus", state.focused, state.hasFocus, fixtureOptionalNodeID(record.Field("expected-focused")))
			assertOptionalNodeID(t, "capture", state.pointerCapture, state.hasCapture, fixtureOptionalNodeID(record.Field("expected-capture")))
			text := make([]NodeID, 0, len(state.textInputs))
			for id := range state.textInputs {
				text = append(text, id)
			}
			sort.Slice(text, func(i, j int) bool { return text[i] < text[j] })
			assertNodeIDs(t, "text", text, fixtureNodeIDs(record.Field("expected-text")))
			scroll := make([]NodeID, 0, len(state.scrolls))
			for id := range state.scrolls {
				scroll = append(scroll, id)
			}
			sort.Slice(scroll, func(i, j int) bool { return scroll[i] < scroll[j] })
			assertNodeIDs(t, "scroll", scroll, fixtureNodeIDs(record.Field("expected-scroll")))
		})
	}
}

func TestScrollClampingMatchesSharedFixtures(t *testing.T) {
	records := interactionRecords(
		t,
		"interaction/scroll.txt",
		"scroll-clamp",
		"content-width",
		"content-height",
		"viewport-width",
		"viewport-height",
		"request-x",
		"request-y",
		"expected-x",
		"expected-y",
	)
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			actual := clampScroll(
				fixtureUint32(t, record.Field("content-width")),
				fixtureUint32(t, record.Field("content-height")),
				fixtureUint32(t, record.Field("viewport-width")),
				fixtureUint32(t, record.Field("viewport-height")),
				ScrollOffset{
					X: fixtureUint32(t, record.Field("request-x")),
					Y: fixtureUint32(t, record.Field("request-y")),
				},
			)
			expected := ScrollOffset{
				X: fixtureUint32(t, record.Field("expected-x")),
				Y: fixtureUint32(t, record.Field("expected-y")),
			}
			if actual != expected {
				t.Fatalf("offset = %+v, want %+v", actual, expected)
			}
		})
	}
}

func TestVirtualScrollRequestsMatchSharedFixtures(t *testing.T) {
	records := interactionRecords(
		t,
		"interaction/virtual-scroll.txt",
		"virtual-scroll-request",
		"axis",
		"content-width",
		"content-height",
		"viewport-width",
		"viewport-height",
		"request-x",
		"request-y",
		"expected-build",
		"expected-x",
		"expected-y",
		"expected-width",
		"expected-height",
		"expected-content-width",
		"expected-content-height",
	)
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			axis, validAxis := map[string]ScrollAxis{
				"both":       ScrollAxisBoth,
				"vertical":   ScrollAxisVertical,
				"horizontal": ScrollAxisHorizontal,
			}[record.Field("axis")]
			if !validAxis {
				t.Fatalf("invalid axis %q", record.Field("axis"))
			}
			declared := Size{
				Width:  fixtureUint32(t, record.Field("content-width")),
				Height: fixtureUint32(t, record.Field("content-height")),
			}
			viewport := Rect{
				Width:  fixtureUint32(t, record.Field("viewport-width")),
				Height: fixtureUint32(t, record.Field("viewport-height")),
			}
			requested := ScrollOffset{
				X: fixtureUint32(t, record.Field("request-x")),
				Y: fixtureUint32(t, record.Field("request-y")),
			}
			expectedBuild := fixtureUint32(t, record.Field("expected-build")) == 1
			expected := VirtualViewport{
				Offset: ScrollOffset{
					X: fixtureUint32(t, record.Field("expected-x")),
					Y: fixtureUint32(t, record.Field("expected-y")),
				},
				Size: Size{
					Width:  fixtureUint32(t, record.Field("expected-width")),
					Height: fixtureUint32(t, record.Field("expected-height")),
				},
				ContentSize: Size{
					Width:  fixtureUint32(t, record.Field("expected-content-width")),
					Height: fixtureUint32(t, record.Field("expected-content-height")),
				},
			}
			cache := virtualCacheState[struct{}]{}
			builds := 0
			builder := func(request VirtualViewport) VirtualFragment[struct{}] {
				builds++
				return NewVirtualFragment(request.Offset, Column[struct{}]())
			}

			first, ok := ensureVirtualFragment(declared, axis, builder, &cache, viewport, requested)
			if ok != expectedBuild {
				t.Fatalf("first build = %t, want %t", ok, expectedBuild)
			}
			if ok && first.request != expected {
				t.Fatalf("first request = %+v, want %+v", first.request, expected)
			}
			second, ok := ensureVirtualFragment(declared, axis, builder, &cache, viewport, requested)
			if ok != expectedBuild {
				t.Fatalf("second build = %t, want %t", ok, expectedBuild)
			}
			if ok && second.request != expected {
				t.Fatalf("second request = %+v, want %+v", second.request, expected)
			}
			expectedBuilds := 0
			if expectedBuild {
				expectedBuilds = 1
			}
			if builds != expectedBuilds {
				t.Fatalf("builds = %d, want %d", builds, expectedBuilds)
			}
		})
	}
}

func TestTextInputEditsMatchSharedFixtures(t *testing.T) {
	records := interactionRecords(
		t,
		"interaction/text-input.txt",
		"text-input-edit",
		"initial",
		"cursor",
		"operation",
		"text",
		"expected",
		"expected-cursor",
	)
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			kind := map[string]textEditKind{
				"insert":    textEditInsert,
				"paste":     textEditPaste,
				"left":      textEditLeft,
				"right":     textEditRight,
				"home":      textEditHome,
				"end":       textEditEnd,
				"backspace": textEditBackspace,
				"delete":    textEditDelete,
			}[record.Field("operation")]
			actual, cursor := applyTextEdit(
				record.Text("initial"),
				int(fixtureUint32(t, record.Field("cursor"))),
				kind,
				record.Text("text"),
			)
			if expected := record.Text("expected"); actual != expected {
				t.Fatalf("text = %q, want %q", actual, expected)
			}
			if expected := int(fixtureUint32(t, record.Field("expected-cursor"))); cursor != expected {
				t.Fatalf("cursor = %d, want %d", cursor, expected)
			}
		})
	}
}

func TestFocusedRoutingMatchesSharedFixtures(t *testing.T) {
	records := interactionRecords(t, "interaction/routing.txt", "event-routing", "paths", "root", "focused", "consume", "expected")
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			parents := make(map[NodeID]*NodeID)
			for _, path := range strings.Split(record.Field("paths"), ",") {
				child, parent, ok := strings.Cut(path, ":")
				if !ok {
					t.Fatalf("invalid path %q", path)
				}
				if parent == "-" {
					parents[NodeID(child)] = nil
				} else {
					parentID := NodeID(parent)
					parents[NodeID(child)] = &parentID
				}
			}
			root := NodeID(record.Field("root"))
			focused := fixtureOptionalNodeID(record.Field("focused"))
			var actual []NodeID
			for _, id := range routePath(parents, &root, focused) {
				actual = append(actual, id)
				if id.String() == record.Field("consume") {
					break
				}
			}
			expected := fixtureNodeIDs(record.Field("expected"))
			if len(actual) != len(expected) {
				t.Fatalf("route = %v, want %v", actual, expected)
			}
			for index := range actual {
				if actual[index] != expected[index] {
					t.Fatalf("route = %v, want %v", actual, expected)
				}
			}
		})
	}
}

func TestModalRoutingMatchesSharedFixtures(t *testing.T) {
	records := interactionRecords(t, "interaction/modal.txt", "modal-routing", "paths", "root", "modal", "target", "consume", "expected")
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			root := NodeID(record.Field("root"))
			modal := NodeID(record.Field("modal"))
			index := newTreeIndex()
			for _, path := range strings.Split(record.Field("paths"), ",") {
				child, parent, ok := strings.Cut(path, ":")
				if !ok {
					t.Fatalf("invalid path %q", path)
				}
				id := NodeID(child)
				item := nodeRecord{
					id:         id,
					rect:       Rect{Width: 10, Height: 10},
					clip:       Rect{Width: 10, Height: 10},
					hasHandler: true,
				}
				if parent != "-" {
					item.parent = NodeID(parent)
					item.hasParent = true
				}
				if id == modal {
					item.kind = interactiveModal
				}
				if err := index.register(item, id == root); err != nil {
					t.Fatal(err)
				}
			}
			target := fixtureOptionalNodeID(record.Field("target"))
			var targetID NodeID
			if target != nil {
				targetID = *target
			}
			var actual []NodeID
			for _, id := range index.route(targetID, target != nil) {
				actual = append(actual, id)
				if id.String() == record.Field("consume") {
					break
				}
			}
			expected := fixtureNodeIDs(record.Field("expected"))
			if len(actual) != len(expected) {
				t.Fatalf("route = %v, want %v", actual, expected)
			}
			for position := range actual {
				if actual[position] != expected[position] {
					t.Fatalf("route = %v, want %v", actual, expected)
				}
			}
		})
	}
}

func TestPointerRoutingMatchesSharedFixtures(t *testing.T) {
	records := interactionRecords(t, "interaction/pointer.txt", "pointer-routing", "records", "point", "capture", "expected")
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			index := newTreeIndex()
			for _, item := range strings.Split(record.Field("records"), ";") {
				parts := strings.Split(item, "@")
				if len(parts) != 3 {
					t.Fatalf("invalid record %q", item)
				}
				if err := index.register(nodeRecord{
					id:         NodeID(parts[0]),
					rect:       fixtureInteractionRect(t, parts[1]),
					clip:       fixtureInteractionRect(t, parts[2]),
					hasHandler: true,
				}, false); err != nil {
					t.Fatal(err)
				}
			}
			var actual NodeID
			var ok bool
			if capture := fixtureOptionalNodeID(record.Field("capture")); capture != nil {
				actual, ok = *capture, true
			} else {
				values := fixtureInteractionNumbers(t, record.Field("point"), 2)
				actual, ok = index.hitTest(Point{X: int32(values[0]), Y: int32(values[1])})
			}
			expected := fixtureOptionalNodeID(record.Field("expected"))
			if expected == nil {
				if ok {
					t.Fatalf("target = %q, true; want none", actual)
				}
			} else if !ok || actual != *expected {
				t.Fatalf("target = %q, %t; want %q", actual, ok, *expected)
			}
		})
	}
}

func interactionRecords(t *testing.T, path, suite string, fields ...string) []conformance.Record {
	t.Helper()
	records, err := conformance.Load(path, suite, fields...)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	return records
}

func fixtureNodeIDs(value string) []NodeID {
	if value == "-" {
		return nil
	}
	parts := strings.Split(value, ",")
	ids := make([]NodeID, len(parts))
	for index, part := range parts {
		ids[index] = NodeID(part)
	}
	return ids
}

func fixtureOptionalNodeID(value string) *NodeID {
	if value == "none" {
		return nil
	}
	id := NodeID(value)
	return &id
}

func assertOptionalNodeID(t *testing.T, name string, actual NodeID, ok bool, expected *NodeID) {
	t.Helper()
	if expected == nil {
		if ok {
			t.Fatalf("%s = %q, true; want none", name, actual)
		}
		return
	}
	if !ok || actual != *expected {
		t.Fatalf("%s = %q, %t; want %q, true", name, actual, ok, *expected)
	}
}

func assertNodeIDs(t *testing.T, name string, actual, expected []NodeID) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Fatalf("%s = %v, want %v", name, actual, expected)
	}
	for index := range actual {
		if actual[index] != expected[index] {
			t.Fatalf("%s = %v, want %v", name, actual, expected)
		}
	}
}

func fixtureUint32(t *testing.T, value string) uint32 {
	t.Helper()
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		t.Fatalf("parse %q: %v", value, err)
	}
	return uint32(parsed)
}

func fixtureInteractionRect(t *testing.T, value string) Rect {
	values := fixtureInteractionNumbers(t, value, 4)
	return Rect{X: int32(values[0]), Y: int32(values[1]), Width: values[2], Height: values[3]}
}

func fixtureInteractionNumbers(t *testing.T, value string, count int) []uint32 {
	t.Helper()
	parts := strings.Split(value, ",")
	if len(parts) != count {
		t.Fatalf("tuple %q has %d values, want %d", value, len(parts), count)
	}
	values := make([]uint32, len(parts))
	for index, part := range parts {
		values[index] = fixtureUint32(t, part)
	}
	return values
}
