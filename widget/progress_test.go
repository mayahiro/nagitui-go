package widget

import (
	"errors"
	"strconv"
	"testing"

	"github.com/mayahiro/nagitui-go/internal/conformance"
)

func TestProgressFixtures(t *testing.T) {
	records, err := conformance.Load(
		"widgets/progress.txt",
		"widget-progress",
		"current", "total", "width", "filled", "expected",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		current := fixtureUint64(t, record.Field("current"))
		total := fixtureUint64(t, record.Field("total"))
		width := uint16(fixtureUint64(t, record.Field("width")))
		if actual, expected := completedCells(current, total, width), fixtureInt(t, record.Field("filled")); actual != expected {
			t.Errorf("case %s: filled = %d, want %d", record.ID, actual, expected)
		}
		if actual, expected := renderedProgress(current, total, width), record.Text("expected"); actual != expected {
			t.Errorf("case %s: rendered = %q, want %q", record.ID, actual, expected)
		}
	}
}

func fixtureUint64(t *testing.T, value string) uint64 {
	t.Helper()
	number, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		t.Fatalf("invalid uint64 %q: %v", value, err)
	}
	return number
}
