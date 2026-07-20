package surface

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/mayahiro/nagitui-go/internal/conformance"
)

func TestGeometryContainsFixtures(t *testing.T) {
	records := geometryRecords(t, "geometry/contains.txt", "geometry-contains", "rect", "point", "contains")
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			t.Parallel()
			got := fixtureRect(record.Field("rect")).Contains(fixturePoint(record.Field("point")))
			want, err := strconv.ParseBool(record.Field("contains"))
			if err != nil {
				t.Fatalf("invalid contains value: %v", err)
			}
			if got != want {
				t.Fatalf("Contains() = %t, want %t", got, want)
			}
		})
	}
}

func TestGeometryIntersectionFixtures(t *testing.T) {
	records := geometryRecords(t, "geometry/intersection.txt", "geometry-intersection", "a", "b", "intersection")
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			t.Parallel()
			got := fixtureRect(record.Field("a")).Intersection(fixtureRect(record.Field("b")))
			want := fixtureRect(record.Field("intersection"))
			if got != want {
				t.Fatalf("Intersection() = %+v, want %+v", got, want)
			}
		})
	}
}

func TestGeometryPointTranslationFixtures(t *testing.T) {
	records := geometryRecords(t, "geometry/point-translate.txt", "geometry-point-translate", "point", "delta", "translated")
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			t.Parallel()
			input := fixturePoint(record.Field("point"))
			delta := fixturePoint(record.Field("delta"))
			got, ok := input.Translate(delta.X, delta.Y)
			if record.Field("translated") == "none" {
				if ok {
					t.Fatalf("Translate() = %+v, true; want failure", got)
				}
				return
			}
			want := fixturePoint(record.Field("translated"))
			if !ok || got != want {
				t.Fatalf("Translate() = %+v, %t; want %+v, true", got, ok, want)
			}
		})
	}
}

func TestGeometryRectTranslationFixtures(t *testing.T) {
	records := geometryRecords(t, "geometry/rect-translate.txt", "geometry-rect-translate", "rect", "delta", "translated")
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			t.Parallel()
			input := fixtureRect(record.Field("rect"))
			delta := fixturePoint(record.Field("delta"))
			got, ok := input.Translate(delta.X, delta.Y)
			if record.Field("translated") == "none" {
				if ok {
					t.Fatalf("Translate() = %+v, true; want failure", got)
				}
				return
			}
			want := fixtureRect(record.Field("translated"))
			if !ok || got != want {
				t.Fatalf("Translate() = %+v, %t; want %+v, true", got, ok, want)
			}
		})
	}
}

func geometryRecords(t *testing.T, path, suite string, fields ...string) []conformance.Record {
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

func fixturePoint(value string) Point {
	values := fixtureNumbers(value, 2)
	return Point{X: int32(values[0]), Y: int32(values[1])}
}

func fixtureRect(value string) Rect {
	values := fixtureNumbers(value, 4)
	return Rect{X: int32(values[0]), Y: int32(values[1]), Width: uint32(values[2]), Height: uint32(values[3])}
}

func fixtureNumbers(value string, count int) []int64 {
	parts := strings.Split(value, ",")
	if len(parts) != count {
		panic("invalid fixture tuple " + value)
	}
	values := make([]int64, len(parts))
	for index, part := range parts {
		parsed, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			panic("invalid fixture integer " + part)
		}
		values[index] = parsed
	}
	return values
}
