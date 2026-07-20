package widget

import (
	"errors"
	"strconv"
	"testing"

	"github.com/mayahiro/nagitui-go/internal/conformance"
)

func TestListNavigationFixtures(t *testing.T) {
	records, err := conformance.Load(
		"widgets/list-navigation.txt",
		"widget-list-navigation",
		"count", "selected", "action", "expected",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		actual, ok := navigateSelection(
			fixtureInt(t, record.Field("count")),
			fixtureInt(t, record.Field("selected")),
			fixtureNavigation(t, record.Field("action")),
		)
		if record.Field("expected") == "none" {
			if ok {
				t.Errorf("case %s: selection = %d, want none", record.ID, actual)
			}
			continue
		}
		expected := fixtureInt(t, record.Field("expected"))
		if !ok || actual != expected {
			t.Errorf("case %s: selection = %d, %t, want %d, true", record.ID, actual, ok, expected)
		}
	}
}

func fixtureNavigation(t *testing.T, value string) navigation {
	t.Helper()
	switch value {
	case "normalize":
		return navigationNormalize
	case "up":
		return navigationUp
	case "down":
		return navigationDown
	case "home":
		return navigationHome
	case "end":
		return navigationEnd
	default:
		t.Fatalf("invalid navigation %q", value)
		return navigationNormalize
	}
}

func fixtureInt(t *testing.T, value string) int {
	t.Helper()
	number, err := strconv.Atoi(value)
	if err != nil {
		t.Fatalf("invalid integer %q: %v", value, err)
	}
	return number
}
