package widget

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/internal/conformance"
)

func TestScrollbarFixtures(t *testing.T) {
	records := loadWidgetFixtures(t, "widgets/scrollbar.txt", "widget-scrollbar", "content", "viewport", "offset", "track", "start", "length")
	for _, record := range records {
		start, length := scrollbarThumbGeometry(
			fixtureUint64(t, record.Field("content")),
			fixtureUint64(t, record.Field("viewport")),
			fixtureUint64(t, record.Field("offset")),
			uint16(fixtureUint64(t, record.Field("track"))),
		)
		if expected := uint16(fixtureUint64(t, record.Field("start"))); start != expected {
			t.Errorf("case %s: start = %d, want %d", record.ID, start, expected)
		}
		if expected := uint16(fixtureUint64(t, record.Field("length"))); length != expected {
			t.Errorf("case %s: length = %d, want %d", record.ID, length, expected)
		}
	}
}

func TestTextAreaEditFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t, "widgets/text-area-edit.txt", "widget-text-area-edit",
		"initial", "cursor", "operation", "text", "expected", "expected-cursor",
	)
	for _, record := range records {
		edit := textAreaInsert
		switch record.Field("operation") {
		case "insert":
		case "left":
			edit = textAreaLeft
		case "right":
			edit = textAreaRight
		case "up":
			edit = textAreaUp
		case "down":
			edit = textAreaDown
		case "home":
			edit = textAreaHome
		case "end":
			edit = textAreaEnd
		case "backspace":
			edit = textAreaBackspace
		case "delete":
			edit = textAreaDelete
		default:
			t.Fatalf("case %s: invalid operation %q", record.ID, record.Field("operation"))
		}
		actual := applyTextAreaEdit(
			NewTextAreaState(record.Text("initial"), fixtureInt(t, record.Field("cursor"))),
			edit,
			record.Text("text"),
		)
		actual.preferredColumn = 0
		actual.hasPreferred = false
		expected := NewTextAreaState(record.Text("expected"), fixtureInt(t, record.Field("expected-cursor")))
		if actual != expected {
			t.Errorf("case %s: state = %#v, want %#v", record.ID, actual, expected)
		}
	}
}

func TestTextAreaSelectionFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t, "widgets/text-area-selection.txt", "widget-text-area-selection",
		"initial", "cursor", "anchor", "operation", "text", "offset",
		"expected", "expected-cursor", "expected-anchor",
	)
	for _, record := range records {
		state := NewTextAreaState(record.Text("initial"), fixtureInt(t, record.Field("cursor"))).
			WithHorizontalOffset(fixtureInt(t, record.Field("offset")))
		if anchor := record.Field("anchor"); anchor != "-" {
			state = state.Select(fixtureInt(t, anchor))
		}
		var actual TextAreaState
		var handled bool
		switch record.Field("operation") {
		case "insert":
			actual = applyTextAreaEdit(state, textAreaInsert, record.Text("text"))
			handled = true
		case "backspace":
			actual = applyTextAreaEdit(state, textAreaBackspace, "")
			handled = true
		case "delete":
			actual = applyTextAreaEdit(state, textAreaDelete, "")
			handled = true
		case "left":
			actual = applyTextAreaMovement(state, textAreaLeft, false)
			handled = true
		case "right":
			actual = applyTextAreaMovement(state, textAreaRight, false)
			handled = true
		case "shift-left":
			actual = applyTextAreaMovement(state, textAreaLeft, true)
			handled = true
		case "shift-right":
			actual = applyTextAreaMovement(state, textAreaRight, true)
			handled = true
		case "select-all":
			actual = selectAllTextAreaState(state)
			handled = true
		default:
			t.Fatalf("case %s: invalid operation %q", record.ID, record.Field("operation"))
		}
		if !handled {
			t.Fatalf("case %s: operation was not handled", record.ID)
		}
		expected := NewTextAreaState(record.Text("expected"), fixtureInt(t, record.Field("expected-cursor"))).
			WithHorizontalOffset(fixtureInt(t, record.Field("offset")))
		if anchor := record.Field("expected-anchor"); anchor != "-" {
			expected = expected.Select(fixtureInt(t, anchor))
		}
		if actual != expected {
			t.Errorf("case %s: state = %#v, want %#v", record.ID, actual, expected)
		}
	}
}

func textAreaFixtureKey(code vt.KeyCode, shift, control bool, character rune) vt.Event {
	return vt.Event{Kind: vt.EventKey, Key: vt.KeyEvent{
		Code: code, Character: character, Modifiers: vt.Modifiers{Shift: shift, Control: control},
		Action: vt.KeyPress, Protocol: vt.KeyProtocolLegacy,
	}}
}

func TestCommandFilterFixtures(t *testing.T) {
	records := loadWidgetFixtures(t, "widgets/command-filter.txt", "widget-command-filter", "label", "keywords", "query", "match")
	for _, record := range records {
		var keywords []string
		if value := record.Field("keywords"); value != "-" {
			keywords = strings.Split(value, ",")
		}
		command := NewCommand("command", record.Text("label")).WithKeywords(keywords...)
		if actual, expected := commandMatches(command, record.Text("query")), fixtureBool(t, record.Field("match")); actual != expected {
			t.Errorf("case %s: match = %t, want %t", record.ID, actual, expected)
		}
	}
}

func TestListViewFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t, "widgets/list-view.txt", "widget-list-view",
		"labels", "query", "offset", "limit", "selected", "visible", "normalized",
	)
	for _, record := range records {
		labels := strings.Split(record.Text("labels"), "|")
		items := make([]ListItem, len(labels))
		for index, label := range labels {
			items[index] = NewListItem(tui.NodeID(label), label)
		}
		visible := listWindow(
			listVisibleIndices(items, record.Text("query")),
			fixtureInt(t, record.Field("offset")),
			fixtureInt(t, record.Field("limit")),
		)
		if actual, expected := fixtureIndices(visible), record.Field("visible"); actual != expected {
			t.Errorf("case %s: visible = %s, want %s", record.ID, actual, expected)
		}
		normalized, ok := normalizedListSelection(visible, fixtureInt(t, record.Field("selected")))
		actual := "none"
		if ok {
			actual = strconv.Itoa(normalized)
		}
		if expected := record.Field("normalized"); actual != expected {
			t.Errorf("case %s: normalized = %s, want %s", record.ID, actual, expected)
		}
	}
}

func TestTreeViewportFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t, "widgets/tree-viewport.txt", "widget-tree-viewport",
		"count", "selected", "height", "start", "end",
	)
	for _, record := range records {
		start, end := treeViewportRange(
			fixtureInt(t, record.Field("count")),
			fixtureInt(t, record.Field("selected")),
			fixtureInt(t, record.Field("height")),
		)
		if expected := fixtureInt(t, record.Field("start")); start != expected {
			t.Errorf("case %s: start = %d, want %d", record.ID, start, expected)
		}
		if expected := fixtureInt(t, record.Field("end")); end != expected {
			t.Errorf("case %s: end = %d, want %d", record.ID, end, expected)
		}
	}
}

func TestSparklineFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t, "widgets/sparkline.txt", "widget-sparkline", "values", "width", "bounds", "expected",
	)
	for _, record := range records {
		values := fixtureUint64List(t, record.Field("values"))
		minimum, maximum, hasBounds := uint64(0), uint64(0), false
		if bounds := record.Field("bounds"); bounds != "-" {
			parts := strings.Split(bounds, ",")
			if len(parts) != 2 {
				t.Fatalf("case %s: invalid bounds %q", record.ID, bounds)
			}
			minimum, maximum = fixtureUint64(t, parts[0]), fixtureUint64(t, parts[1])
			hasBounds = true
		}
		actual := renderSparkline(
			values, uint16(fixtureUint64(t, record.Field("width"))), minimum, maximum, hasBounds,
		)
		if expected := record.Text("expected"); actual != expected {
			t.Errorf("case %s: sparkline = %q, want %q", record.ID, actual, expected)
		}
	}
}

func TestBarChartFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t, "widgets/bar-chart.txt", "widget-bar-chart", "value", "maximum", "width", "filled",
	)
	for _, record := range records {
		actual := scaledBarCells(
			fixtureUint64(t, record.Field("value")),
			fixtureUint64(t, record.Field("maximum")),
			uint16(fixtureUint64(t, record.Field("width"))),
		)
		if expected := fixtureInt(t, record.Field("filled")); actual != expected {
			t.Errorf("case %s: filled = %d, want %d", record.ID, actual, expected)
		}
	}
}

func TestChartScaleFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t, "widgets/chart-scale.txt", "widget-chart-scale",
		"value", "minimum", "maximum", "cells", "expected",
	)
	for _, record := range records {
		actual := chartScale(
			int32(fixtureInt64(t, record.Field("value"))),
			int32(fixtureInt64(t, record.Field("minimum"))),
			int32(fixtureInt64(t, record.Field("maximum"))),
			fixtureInt(t, record.Field("cells")),
		)
		if expected := fixtureInt(t, record.Field("expected")); actual != expected {
			t.Errorf("case %s: cell = %d, want %d", record.ID, actual, expected)
		}
	}
}

func TestPaginatorFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t, "widgets/paginator.txt", "widget-paginator",
		"total", "page", "limit", "normalized", "start", "end", "previous", "next",
	)
	for _, record := range records {
		total := fixtureInt(t, record.Field("total"))
		page := fixtureInt(t, record.Field("page"))
		limit := fixtureInt(t, record.Field("limit"))
		normalized, ok := normalizedPage(page, total)
		actualNormalized := "none"
		if ok {
			actualNormalized = strconv.Itoa(normalized)
		}
		if expected := record.Field("normalized"); actualNormalized != expected {
			t.Errorf("case %s: normalized = %s, want %s", record.ID, actualNormalized, expected)
		}
		start, end := paginatorWindow(total, page, limit)
		if expected := fixtureInt(t, record.Field("start")); start != expected {
			t.Errorf("case %s: start = %d, want %d", record.ID, start, expected)
		}
		if expected := fixtureInt(t, record.Field("end")); end != expected {
			t.Errorf("case %s: end = %d, want %d", record.ID, end, expected)
		}
		if !ok {
			continue
		}
		previous, handled := paginatorPageForAction(page, total, paginatorPrevious)
		if !handled || strconv.Itoa(previous) != record.Field("previous") {
			t.Errorf("case %s: previous = %d, %t", record.ID, previous, handled)
		}
		next, handled := paginatorPageForAction(page, total, paginatorNext)
		if !handled || strconv.Itoa(next) != record.Field("next") {
			t.Errorf("case %s: next = %d, %t", record.ID, next, handled)
		}
	}
}

func TestFilePickerFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t, "widgets/file-picker.txt", "widget-file-picker",
		"hidden", "show-hidden", "selected", "viewport", "visible", "normalized",
		"start", "end", "page-up", "page-down",
	)
	for _, record := range records {
		hidden := fixtureBoolList(t, record.Field("hidden"))
		entries := make([]FilePickerEntry, len(hidden))
		for index, isHidden := range hidden {
			entries[index] = NewFilePickerFile(tui.NodeID(strconv.Itoa(index)), strconv.Itoa(index), strconv.Itoa(index)).WithHidden(isHidden)
		}
		visible := filePickerVisibleIndices(entries, fixtureBool(t, record.Field("show-hidden")))
		if actual, expected := fixtureIndices(visible), record.Field("visible"); actual != expected {
			t.Errorf("case %s: visible = %s, want %s", record.ID, actual, expected)
		}
		selected, ok := normalizedListSelection(visible, fixtureInt(t, record.Field("selected")))
		actualSelected := "none"
		if ok {
			actualSelected = strconv.Itoa(selected)
		}
		if expected := record.Field("normalized"); actualSelected != expected {
			t.Errorf("case %s: normalized = %s, want %s", record.ID, actualSelected, expected)
		}
		viewport := fixtureInt(t, record.Field("viewport"))
		start, end := 0, len(visible)
		if viewport > 0 && ok {
			start, end = treeViewportRange(len(visible), selected, viewport)
		}
		if expected := fixtureInt(t, record.Field("start")); start != expected {
			t.Errorf("case %s: start = %d, want %d", record.ID, start, expected)
		}
		if expected := fixtureInt(t, record.Field("end")); end != expected {
			t.Errorf("case %s: end = %d, want %d", record.ID, end, expected)
		}
		if !ok {
			continue
		}
		up, handled := filePickerPositionForAction(selected, len(visible), viewport, filePickerPreviousPage)
		if !handled || strconv.Itoa(up) != record.Field("page-up") {
			t.Errorf("case %s: page-up = %d, %t", record.ID, up, handled)
		}
		down, handled := filePickerPositionForAction(selected, len(visible), viewport, filePickerNextPage)
		if !handled || strconv.Itoa(down) != record.Field("page-down") {
			t.Errorf("case %s: page-down = %d, %t", record.ID, down, handled)
		}
	}
}

func TestCalendarFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t, "widgets/calendar.txt", "widget-calendar",
		"date", "delta-days", "expected-days", "delta-months", "expected-months",
		"weekday", "month-days", "monday-offset", "sunday-offset",
	)
	for _, record := range records {
		date := fixtureCalendarDate(t, record.Field("date"))
		if actual, expected := formatCalendarDate(addCalendarDays(date, fixtureInt(t, record.Field("delta-days")))), record.Field("expected-days"); actual != expected {
			t.Errorf("case %s: add days = %s, want %s", record.ID, actual, expected)
		}
		if actual, expected := formatCalendarDate(addCalendarMonths(date, fixtureInt(t, record.Field("delta-months")))), record.Field("expected-months"); actual != expected {
			t.Errorf("case %s: add months = %s, want %s", record.ID, actual, expected)
		}
		if actual, expected := calendarWeekday(date), fixtureInt(t, record.Field("weekday")); actual != expected {
			t.Errorf("case %s: weekday = %d, want %d", record.ID, actual, expected)
		}
		if actual, expected := calendarDaysInMonth(date.Year, int(date.Month)), fixtureInt(t, record.Field("month-days")); actual != expected {
			t.Errorf("case %s: month days = %d, want %d", record.ID, actual, expected)
		}
		first := NewCalendarDate(date.Year, int(date.Month), 1)
		if actual, expected := calendarMonthOffset(first, CalendarWeekStartsMonday), fixtureInt(t, record.Field("monday-offset")); actual != expected {
			t.Errorf("case %s: Monday offset = %d, want %d", record.ID, actual, expected)
		}
		if actual, expected := calendarMonthOffset(first, CalendarWeekStartsSunday), fixtureInt(t, record.Field("sunday-offset")); actual != expected {
			t.Errorf("case %s: Sunday offset = %d, want %d", record.ID, actual, expected)
		}
	}
}

func fixtureCalendarDate(t *testing.T, value string) CalendarDate {
	t.Helper()
	parts := strings.Split(value, "-")
	if len(parts) != 3 {
		t.Fatalf("invalid calendar date %q", value)
	}
	return NewCalendarDate(fixtureInt(t, parts[0]), fixtureInt(t, parts[1]), fixtureInt(t, parts[2]))
}

func formatCalendarDate(date CalendarDate) string {
	return fmt.Sprintf("%04d-%02d-%02d", date.Year, date.Month, date.Day)
}

func fixtureBoolList(t *testing.T, value string) []bool {
	t.Helper()
	if value == "-" {
		return nil
	}
	parts := strings.Split(value, ",")
	values := make([]bool, len(parts))
	for index, part := range parts {
		values[index] = fixtureBool(t, part)
	}
	return values
}

func fixtureUint64List(t *testing.T, value string) []uint64 {
	t.Helper()
	if value == "-" {
		return nil
	}
	parts := strings.Split(value, ",")
	values := make([]uint64, len(parts))
	for index, part := range parts {
		values[index] = fixtureUint64(t, part)
	}
	return values
}

func fixtureInt64(t *testing.T, value string) int64 {
	t.Helper()
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		t.Fatalf("invalid int64 %q: %v", value, err)
	}
	return parsed
}

func fixtureIndices(indices []int) string {
	if len(indices) == 0 {
		return "-"
	}
	values := make([]string, len(indices))
	for index, value := range indices {
		values[index] = strconv.Itoa(value)
	}
	return strings.Join(values, ",")
}

func loadWidgetFixtures(t *testing.T, relative, suite string, fields ...string) []conformance.Record {
	t.Helper()
	records, err := conformance.Load(relative, suite, fields...)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	return records
}
