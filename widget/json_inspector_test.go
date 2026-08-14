package widget

import (
	"strings"
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

func TestJSONInspectorFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t,
		"widgets/json-inspector.txt",
		"widget-json-inspector",
		"selected",
		"expanded",
		"event",
		"max-graphemes",
		"expected-selected",
		"expected-expanded",
		"expected-message",
		"expected-copy",
		"consumed",
	)
	document := jsonInspectorTestDocument(t)
	for _, record := range records {
		initial := normalizeJSONInspectorState(
			document,
			jsonInspectorFixtureState(t, record.Field("selected"), record.Field("expanded")),
		)
		next := initial
		message, copyValue := "-", "-"
		var consumed *bool
		event := record.Field("event")
		switch event {
		case "-":
		case "repeat-enter":
			value := isBlockedJSONInspectorRepeat(jsonInspectorRepeatKey(vt.KeyEnter, 0, vt.Modifiers{}), true)
			consumed = &value
		case "repeat-copy":
			value := isBlockedJSONInspectorRepeat(jsonInspectorRepeatKey(
				vt.KeyCharacter, 'c', vt.Modifiers{Control: true},
			), true)
			consumed = &value
		case "copy":
			selected, _ := document.indexOf(initial.Selected())
			node := document.nodeAt(selected)
			message = "copy:" + jsonInspectorFixturePointerText(node.Path())
			copyValue = node.Serialized()
			value := true
			consumed = &value
		case "pointer-nested":
			nested, _ := document.indexOf(jsonInspectorTestPointer(t, "/nested"))
			next = jsonInspectorStateForPointer(document, initial, nested)
			if !next.Equal(initial) {
				message = jsonInspectorFixtureStateMessage(next)
			}
			value := true
			consumed = &value
		default:
			selected, _ := document.indexOf(initial.Selected())
			next = jsonInspectorStateForAction(
				document, initial, selected, jsonInspectorFixtureAction(t, event),
			)
			if !next.Equal(initial) {
				message = jsonInspectorFixtureStateMessage(next)
			}
			value := true
			consumed = &value
		}

		if got, want := jsonInspectorFixturePointerText(next.Selected()), record.Field("expected-selected"); got != want {
			t.Errorf("case %s: selected = %s, want %s", record.ID, got, want)
		}
		if got, want := jsonInspectorFixtureExpandedText(next), record.Field("expected-expanded"); got != want {
			t.Errorf("case %s: expanded = %s, want %s", record.ID, got, want)
		}
		if message != record.Field("expected-message") {
			t.Errorf("case %s: message = %s, want %s", record.ID, message, record.Field("expected-message"))
		}
		if copyValue != record.Text("expected-copy") {
			t.Errorf("case %s: copy = %q, want %q", record.ID, copyValue, record.Text("expected-copy"))
		}
		consumedText := "-"
		if consumed != nil {
			consumedText = "false"
			if *consumed {
				consumedText = "true"
			}
		}
		if consumedText != record.Field("consumed") {
			t.Errorf("case %s: consumed = %s, want %s", record.ID, consumedText, record.Field("consumed"))
		}
		maximum := fixtureInt(t, record.Field("max-graphemes"))
		if preview, truncated := jsonInspectorScalarPreview("abcdef日ghi", maximum); preview != "abcde" || !truncated {
			t.Errorf("case %s: preview = %q, %t", record.ID, preview, truncated)
		}
	}
}

func TestJSONInspectorBoundedViewportAndCompleteCopy(t *testing.T) {
	values := make([]JSONValue, 100)
	for index := range values {
		number, err := NewJSONNumber(fixtureNumberText(uint64(index)))
		if err != nil {
			t.Fatal(err)
		}
		values[index] = NewJSONNumberValue(number)
	}
	document, err := NewJSONDocument(NewJSONArray(values))
	if err != nil {
		t.Fatal(err)
	}
	selected := jsonInspectorTestPointer(t, "/50")
	state := NewJSONInspectorState(selected, []JSONPointer{RootJSONPointer()})
	selectedIndex, _ := document.indexOf(selected)
	window := jsonInspectorVisibleWindow(document, state, selectedIndex, 7)
	if len(window) != 7 || document.nodeAt(window[0]).Path().String() != "/47" || document.nodeAt(window[6]).Path().String() != "/53" {
		t.Fatalf("window = %v", window)
	}
}

func TestJSONInspectorPreviewIsGraphemeSafeAndStateStorageIsImmutable(t *testing.T) {
	document := jsonInspectorTestDocument(t)
	node, _ := document.Get(jsonInspectorTestPointer(t, "/long"))
	record := document.record(node.index)
	spans := jsonInspectorRowSpans(
		record, node.Serialized(), false, 7, DefaultJSONInspectorStyle(), true, false,
	)
	var display strings.Builder
	for _, span := range spans {
		display.WriteString(span.Text)
	}
	if !strings.Contains(display.String(), "\"abcdef日…\"") {
		t.Fatalf("display = %q", display.String())
	}
	if node.Serialized() != "\"abcdef日ghi\"" {
		t.Fatalf("copy source = %q", node.Serialized())
	}

	var state JSONInspectorState
	clone := state
	if &state.expandedValues()[0] == &clone.expandedValues()[0] {
		// The address equality is expected for the immutable zero-value backing array
	} else {
		t.Fatalf("zero state did not share immutable expansion storage")
	}
	changed := clone.WithExpanded(jsonInspectorTestPointer(t, "/nested"), true)
	if state.IsExpanded(jsonInspectorTestPointer(t, "/nested")) || !changed.IsExpanded(jsonInspectorTestPointer(t, "/nested")) {
		t.Fatalf("state mutation escaped immutable copy")
	}
}

func TestJSONInspectorSemanticStylesAndOverlays(t *testing.T) {
	document := jsonInspectorStyleDocument(t)
	style := JSONInspectorStyle{
		Key:         jsonInspectorForegroundStyle(1),
		String:      jsonInspectorForegroundStyle(2),
		Number:      jsonInspectorForegroundStyle(3),
		Boolean:     jsonInspectorForegroundStyle(4),
		Null:        jsonInspectorForegroundStyle(5),
		Punctuation: jsonInspectorForegroundStyle(6),
		Index:       jsonInspectorForegroundStyle(7),
		Summary:     jsonInspectorForegroundStyle(8),
		Selected:    vt.Style{Background: vt.IndexedColor(9)},
		Disabled:    vt.Style{Background: vt.IndexedColor(10)},
	}
	rootSpans := jsonInspectorRowSpans(
		document.record(0), document.Root().Serialized(), true, 80, style, true, false,
	)
	assertJSONInspectorSpanForeground(t, rootSpans, "▼ ", 6)
	assertJSONInspectorSpanForeground(t, rootSpans, "5", 8)
	assertJSONInspectorValueStyles(t, document, style)

	textPointer := jsonInspectorTestPointer(t, "/text")
	textIndex, _ := document.indexOf(textPointer)
	selected := jsonInspectorRowSpans(
		document.record(textIndex), document.nodeAt(textIndex).Serialized(), false, 80, style, true, true,
	)
	for _, span := range selected {
		if span.Style.Background != vt.IndexedColor(9) {
			t.Fatalf("selected span %q background was not merged", span.Text)
		}
	}
	disabled := jsonInspectorRowSpans(
		document.record(textIndex), document.nodeAt(textIndex).Serialized(), false, 80, style, false, true,
	)
	for _, span := range disabled {
		if span.Style.Background != vt.IndexedColor(10) {
			t.Fatalf("disabled span %q background was not merged", span.Text)
		}
	}
}

func TestJSONInspectorCopyRepeatPassesThroughWithoutCopyHandler(t *testing.T) {
	event := jsonInspectorRepeatKey(vt.KeyCharacter, 'c', vt.Modifiers{Control: true})
	if isBlockedJSONInspectorRepeat(event, false) {
		t.Fatal("copy repeat was consumed without a copy handler")
	}
}

func jsonInspectorTestDocument(t *testing.T) JSONDocument {
	t.Helper()
	numberOne, _ := NewJSONNumber("1")
	numberTwo, _ := NewJSONNumber("2")
	nested, err := NewJSONObject([]JSONMember{
		NewJSONMember("flag", NewJSONBoolean(true)),
		NewJSONMember("items", NewJSONArray([]JSONValue{
			NewJSONNumberValue(numberOne), NewJSONNumberValue(numberTwo),
		})),
	})
	if err != nil {
		t.Fatal(err)
	}
	root, err := NewJSONObject([]JSONMember{
		NewJSONMember("short", NewJSONString("ok")),
		NewJSONMember("long", NewJSONString("abcdef日ghi")),
		NewJSONMember("nested", nested),
		NewJSONMember("empty", NewJSONArray(nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	document, err := NewJSONDocument(root)
	if err != nil {
		t.Fatal(err)
	}
	return document
}

func jsonInspectorStyleDocument(t *testing.T) JSONDocument {
	t.Helper()
	one, err := NewJSONNumber("1")
	if err != nil {
		t.Fatal(err)
	}
	two, err := NewJSONNumber("2")
	if err != nil {
		t.Fatal(err)
	}
	root, err := NewJSONObject([]JSONMember{
		NewJSONMember("text", NewJSONString("x")),
		NewJSONMember("number", NewJSONNumberValue(one)),
		NewJSONMember("boolean", NewJSONBoolean(true)),
		NewJSONMember("null", NewJSONNull()),
		NewJSONMember("array", NewJSONArray([]JSONValue{NewJSONNumberValue(two)})),
	})
	if err != nil {
		t.Fatal(err)
	}
	document, err := NewJSONDocument(root)
	if err != nil {
		t.Fatal(err)
	}
	return document
}

func jsonInspectorForegroundStyle(index uint8) vt.Style {
	return vt.Style{Foreground: vt.IndexedColor(index)}
}

func assertJSONInspectorValueStyles(t *testing.T, document JSONDocument, style JSONInspectorStyle) {
	t.Helper()
	for _, test := range []struct {
		path, text string
		color      uint8
	}{
		{path: "/text", text: "x", color: 2},
		{path: "/number", text: "1", color: 3},
		{path: "/boolean", text: "true", color: 4},
		{path: "/null", text: "null", color: 5},
	} {
		index, _ := document.indexOf(jsonInspectorTestPointer(t, test.path))
		spans := jsonInspectorRowSpans(
			document.record(index), document.nodeAt(index).Serialized(), false, 80, style, true, false,
		)
		assertJSONInspectorSpanForeground(t, spans, strings.TrimPrefix(test.path, "/"), 1)
		assertJSONInspectorSpanForeground(t, spans, test.text, test.color)
	}
	index, _ := document.indexOf(jsonInspectorTestPointer(t, "/array/0"))
	spans := jsonInspectorRowSpans(
		document.record(index), document.nodeAt(index).Serialized(), false, 80, style, true, false,
	)
	assertJSONInspectorSpanForeground(t, spans, "0", 7)
	assertJSONInspectorSpanForeground(t, spans, "2", 3)
}

func assertJSONInspectorSpanForeground(t *testing.T, spans []tui.TextSpan, text string, color uint8) {
	t.Helper()
	for _, span := range spans {
		if span.Text == text && span.Style.Foreground == vt.IndexedColor(color) {
			return
		}
	}
	t.Fatalf("missing span %q with indexed foreground %d", text, color)
}

func jsonInspectorFixtureState(t *testing.T, selected, expanded string) JSONInspectorState {
	t.Helper()
	var values []JSONPointer
	if expanded != "-" {
		for _, value := range strings.Split(expanded, ",") {
			values = append(values, jsonInspectorFixturePointer(t, value))
		}
	}
	return NewJSONInspectorState(jsonInspectorFixturePointer(t, selected), values)
}

func jsonInspectorFixturePointer(t *testing.T, value string) JSONPointer {
	t.Helper()
	if value == "$" {
		return RootJSONPointer()
	}
	return jsonInspectorTestPointer(t, value)
}

func jsonInspectorTestPointer(t *testing.T, value string) JSONPointer {
	t.Helper()
	pointer, err := NewJSONPointer(value)
	if err != nil {
		t.Fatal(err)
	}
	return pointer
}

func jsonInspectorFixturePointerText(pointer JSONPointer) string {
	if pointer.String() == "" {
		return "$"
	}
	return pointer.String()
}

func jsonInspectorFixtureExpandedText(state JSONInspectorState) string {
	values := state.Expanded()
	if len(values) == 0 {
		return "-"
	}
	text := make([]string, len(values))
	for index, pointer := range values {
		text[index] = jsonInspectorFixturePointerText(pointer)
	}
	return strings.Join(text, ",")
}

func jsonInspectorFixtureStateMessage(state JSONInspectorState) string {
	return "state:" + jsonInspectorFixturePointerText(state.Selected()) + ":" + jsonInspectorFixtureExpandedText(state)
}

func jsonInspectorFixtureAction(t *testing.T, value string) jsonInspectorAction {
	t.Helper()
	switch value {
	case "enter":
		return jsonInspectorActivate
	case "up":
		return jsonInspectorPrevious
	case "down":
		return jsonInspectorNext
	case "home":
		return jsonInspectorFirst
	case "end":
		return jsonInspectorLast
	case "left":
		return jsonInspectorCollapse
	case "right":
		return jsonInspectorExpand
	default:
		t.Fatalf("unknown event %q", value)
		return jsonInspectorActivate
	}
}

func jsonInspectorRepeatKey(code vt.KeyCode, character rune, modifiers vt.Modifiers) vt.Event {
	return vt.Event{Kind: vt.EventKey, Key: vt.KeyEvent{
		Code: code, Character: character, Modifiers: modifiers,
		Action: vt.KeyRepeat, Protocol: vt.KeyProtocolLegacy,
	}}
}
