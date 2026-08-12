package tui

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/mayahiro/nagitui-go/internal/conformance"
)

func TestResponsiveRowLayoutsMatchSharedFixtures(t *testing.T) {
	records, err := conformance.Load(
		"layout/responsive-row.txt",
		"responsive-row-layout",
		"rect", "gap", "placements", "priorities", "widths", "expected",
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
			placements := strings.Split(record.Field("placements"), ",")
			priorities := strings.Split(record.Field("priorities"), ",")
			widths := strings.Split(record.Field("widths"), ",")
			if len(placements) != len(priorities) || len(placements) != len(widths) {
				t.Fatal("fixture list lengths differ")
			}
			metrics := make([]responsiveRowMetric, len(placements))
			for index := range placements {
				priority, err := strconv.ParseUint(priorities[index], 10, 16)
				if err != nil {
					t.Fatal(err)
				}
				metrics[index] = responsiveRowMetric{
					placement:    fixtureResponsiveRowPlacement(t, placements[index]),
					priority:     uint16(priority),
					desiredWidth: splitPaneFixtureUint(t, widths[index]),
				}
			}
			actual := resolveResponsiveRowLayout(
				splitPaneFixtureRect(t, record.Field("rect")),
				metrics,
				splitPaneFixtureUint(t, record.Field("gap")),
			)
			expected := strings.Split(record.Field("expected"), ",")
			actualRects := actual.slice()
			if len(actualRects) != len(expected) {
				t.Fatalf("rect count = %d, want %d", len(actualRects), len(expected))
			}
			for index, value := range expected {
				if value == "none" {
					if actualRects[index].visible {
						t.Fatalf("rect %d = %+v, want none", index, actualRects[index].rect)
					}
					continue
				}
				want := splitPaneFixtureRect(t, value)
				if !actualRects[index].visible || actualRects[index].rect != want {
					t.Fatalf("rect %d = %+v visible=%t, want %+v", index, actualRects[index].rect, actualRects[index].visible, want)
				}
			}
		})
	}
}

func TestResponsiveRowUnknownPlacementNormalizesToStart(t *testing.T) {
	item := NewResponsiveRowItem(Text[struct{}]("x")).Placement(ResponsiveRowPlacement(255))
	if item.ConfiguredPlacement() != ResponsiveRowStart {
		t.Fatalf("placement = %v, want start", item.ConfiguredPlacement())
	}
}

func TestResponsiveRowInlineResolutionDoesNotAllocate(t *testing.T) {
	metrics := make([]responsiveRowMetric, 8)
	for index := range metrics {
		metrics[index] = responsiveRowMetric{
			placement: ResponsiveRowPlacement(index % 3),
			priority:  uint16(index), desiredWidth: 3,
		}
	}
	var layout resolvedResponsiveRowLayout
	resolveResponsiveRowLayoutInto(&layout, Rect{Width: 40, Height: 1}, metrics, 1)
	if allocations := testing.AllocsPerRun(1000, func() {
		resolveResponsiveRowLayoutInto(&layout, Rect{Width: 40, Height: 1}, metrics, 1)
	}); allocations != 0 {
		t.Fatalf("allocations = %f, want 0", allocations)
	}
}

func TestResponsiveRowOverflowStorageKeepsEveryItem(t *testing.T) {
	metrics := make([]responsiveRowMetric, inlineResponsiveRowItems+8)
	for index := range metrics {
		metrics[index] = responsiveRowMetric{desiredWidth: 1}
	}
	layout := resolveResponsiveRowLayout(
		Rect{Width: uint32(len(metrics)), Height: 1}, metrics, 0,
	)
	if len(layout.slice()) != len(metrics) {
		t.Fatalf("rect count = %d, want %d", len(layout.slice()), len(metrics))
	}
	for index, itemRect := range layout.slice() {
		if !itemRect.visible || itemRect.rect.X != int32(index) {
			t.Fatalf("rect %d = %+v", index, itemRect)
		}
	}
}

func fixtureResponsiveRowPlacement(t *testing.T, value string) ResponsiveRowPlacement {
	t.Helper()
	switch value {
	case "start":
		return ResponsiveRowStart
	case "center":
		return ResponsiveRowCenter
	case "end":
		return ResponsiveRowEnd
	default:
		t.Fatalf("invalid placement %q", value)
		return 0
	}
}
