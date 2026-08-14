package tui

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/mayahiro/nagitui-go/internal/conformance"
)

func TestSplitPaneLayoutsMatchSharedFixtures(t *testing.T) {
	records, err := conformance.Load(
		"layout/split-pane.txt",
		"split-pane-layout",
		"rect", "axis", "ratio", "primary-min", "secondary-min", "collapse",
		"expected-primary", "expected-divider", "expected-secondary", "expected-collapse",
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
			options := DefaultSplitPaneOptions()
			options.Axis = splitPaneFixtureAxis(t, record.Field("axis"))
			options.Ratio = uint16(splitPaneFixtureUint(t, record.Field("ratio")))
			options.PrimaryMinimum = splitPaneFixtureUint(t, record.Field("primary-min"))
			options.SecondaryMinimum = splitPaneFixtureUint(t, record.Field("secondary-min"))
			options.Collapse = splitPaneFixtureCollapse(t, record.Field("collapse"))
			actual := resolveSplitPaneLayout(splitPaneFixtureRect(t, record.Field("rect")), options)
			if expected := splitPaneFixtureRect(t, record.Field("expected-primary")); actual.primary != expected {
				t.Fatalf("primary = %+v, want %+v", actual.primary, expected)
			}
			if value := record.Field("expected-divider"); value == "none" {
				if actual.hasDivider {
					t.Fatalf("divider = %+v, want none", actual.divider)
				}
			} else if expected := splitPaneFixtureRect(t, value); !actual.hasDivider || actual.divider != expected {
				t.Fatalf("divider = %+v, %t, want %+v", actual.divider, actual.hasDivider, expected)
			}
			if expected := splitPaneFixtureRect(t, record.Field("expected-secondary")); actual.secondary != expected {
				t.Fatalf("secondary = %+v, want %+v", actual.secondary, expected)
			}
			if value := record.Field("expected-collapse"); value == "none" {
				if actual.hasCollapsed {
					t.Fatalf("collapse = %v, want none", actual.collapsed)
				}
			} else if expected := splitPaneFixtureCollapse(t, value); !actual.hasCollapsed || actual.collapsed != expected {
				t.Fatalf("collapse = %v, %t, want %v", actual.collapsed, actual.hasCollapsed, expected)
			}
		})
	}
}

func TestSplitPaneUnknownOptionsNormalizeDeterministically(t *testing.T) {
	options := DefaultSplitPaneOptions()
	options.Axis = SplitPaneAxis(255)
	options.Collapse = SplitPaneCollapse(255)
	options.PrimaryMinimum = 2
	options.SecondaryMinimum = 2
	layout := resolveSplitPaneLayout(Rect{Width: 4, Height: 3}, options)
	if !layout.hasCollapsed || layout.collapsed != SplitPaneCollapseSecondary || layout.primary.Width != 4 {
		t.Fatalf("layout = %+v, want horizontal secondary collapse", layout)
	}
}

func splitPaneFixtureAxis(t *testing.T, value string) SplitPaneAxis {
	t.Helper()
	switch value {
	case "horizontal":
		return SplitPaneHorizontal
	case "vertical":
		return SplitPaneVertical
	default:
		t.Fatalf("invalid axis %q", value)
		return 0
	}
}

func splitPaneFixtureCollapse(t *testing.T, value string) SplitPaneCollapse {
	t.Helper()
	switch value {
	case "primary":
		return SplitPaneCollapsePrimary
	case "secondary":
		return SplitPaneCollapseSecondary
	default:
		t.Fatalf("invalid collapse %q", value)
		return 0
	}
}

func splitPaneFixtureRect(t *testing.T, value string) Rect {
	t.Helper()
	parts := strings.Split(value, ":")
	if len(parts) != 4 {
		t.Fatalf("invalid rect %q", value)
	}
	x, err := strconv.ParseInt(parts[0], 10, 32)
	if err != nil {
		t.Fatal(err)
	}
	y, err := strconv.ParseInt(parts[1], 10, 32)
	if err != nil {
		t.Fatal(err)
	}
	return Rect{
		X: int32(x), Y: int32(y),
		Width: splitPaneFixtureUint(t, parts[2]), Height: splitPaneFixtureUint(t, parts[3]),
	}
}

func splitPaneFixtureUint(t *testing.T, value string) uint32 {
	t.Helper()
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		t.Fatal(err)
	}
	return uint32(parsed)
}
