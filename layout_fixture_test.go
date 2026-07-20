package tui

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/mayahiro/nagitui-go/internal/conformance"
)

func TestLinearAllocationsMatchSharedFixtures(t *testing.T) {
	records, err := conformance.Load(
		"layout/linear.txt",
		"layout-linear",
		"available",
		"lengths",
		"desired",
		"expected",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			lengths := parseFixtureList(t, record.Field("lengths"), parseFixtureLength)
			desired := parseFixtureList(t, record.Field("desired"), parseFixtureNumber)
			expected := parseFixtureList(t, record.Field("expected"), parseFixtureNumber)
			if len(lengths) != len(desired) {
				t.Fatalf("lengths has %d items, desired has %d", len(lengths), len(desired))
			}
			tracks := make([]layoutTrack, len(lengths))
			for index := range lengths {
				tracks[index] = layoutTrack{length: lengths[index], desired: desired[index]}
			}
			actual := allocate(parseFixtureNumber(t, record.Field("available")), tracks)
			if len(actual) != len(expected) {
				t.Fatalf("got %v, want %v", actual, expected)
			}
			for index := range actual {
				if actual[index] != expected[index] {
					t.Fatalf("got %v, want %v", actual, expected)
				}
			}
		})
	}
}

func parseFixtureList[T any](t *testing.T, value string, parse func(*testing.T, string) T) []T {
	t.Helper()
	if value == "-" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]T, len(parts))
	for index, part := range parts {
		result[index] = parse(t, part)
	}
	return result
}

func parseFixtureNumber(t *testing.T, value string) uint32 {
	t.Helper()
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		t.Fatalf("parse %q: %v", value, err)
	}
	return uint32(parsed)
}

func parseFixtureLength(t *testing.T, value string) Length {
	t.Helper()
	parts := strings.Split(value, ":")
	switch parts[0] {
	case "auto":
		if len(parts) == 1 {
			return Auto()
		}
	case "fixed":
		if len(parts) == 2 {
			return Fixed(parseFixtureNumber(t, parts[1]))
		}
	case "flex":
		if len(parts) == 2 {
			return Flex(parseFixtureNumber(t, parts[1]))
		}
	case "percent":
		if len(parts) == 2 {
			return Percent(parseFixtureNumber(t, parts[1]))
		}
	case "minmax":
		if len(parts) == 4 {
			return MinMax(
				parseFixtureNumber(t, parts[1]),
				parseFixtureNumber(t, parts[2]),
				parseFixtureNumber(t, parts[3]),
			)
		}
	}
	t.Fatalf("invalid length %q", value)
	return Length{}
}
