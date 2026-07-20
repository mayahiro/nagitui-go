package widget

import (
	"errors"
	"testing"

	"github.com/mayahiro/nagitui-go/internal/conformance"
)

func TestSpinnerFixtures(t *testing.T) {
	records, err := conformance.Load(
		"widgets/spinner.txt",
		"widget-spinner",
		"tick", "label", "expected",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		actual := renderedSpinner(fixtureUint64(t, record.Field("tick")), record.Text("label"))
		if expected := record.Text("expected"); actual != expected {
			t.Errorf("case %s: rendered = %q, want %q", record.ID, actual, expected)
		}
	}
}
