package widget

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

func TestJSONDocumentFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t,
		"widgets/json-document.txt",
		"widget-json-document",
		"arrangement",
		"max-nodes",
		"max-depth",
		"max-string-bytes",
		"max-serialized-bytes",
		"expected-json",
		"expected-paths",
		"expected-kinds",
		"expected-depths",
		"expected-children",
		"expected-error",
	)
	for _, record := range records {
		limits := DefaultJSONDocumentLimits().
			WithMaxNodes(fixtureUint64(t, record.Field("max-nodes"))).
			WithMaxDepth(uint32(fixtureUint64(t, record.Field("max-depth")))).
			WithMaxStringBytes(fixtureUint64(t, record.Field("max-string-bytes"))).
			WithMaxSerializedBytes(fixtureUint64(t, record.Field("max-serialized-bytes")))
		document, err := NewJSONDocumentWithLimits(jsonFixtureArrangement(t, record.Field("arrangement")), limits)
		if expected := record.Field("expected-error"); expected != "-" {
			var resource *JSONDocumentError
			if !errors.As(err, &resource) {
				t.Errorf("case %s: error = %v, want resource error", record.ID, err)
				continue
			}
			if resource.Kind.String() != expected {
				t.Errorf("case %s: error kind = %s, want %s", record.ID, resource.Kind, expected)
			}
			continue
		}
		if err != nil {
			t.Errorf("case %s: %v", record.ID, err)
			continue
		}
		if got, want := document.Serialized(), record.Text("expected-json"); got != want {
			t.Errorf("case %s: serialized = %q, want %q", record.ID, got, want)
		}
		var paths, kinds []string
		var depths, children []string
		nodes := document.Nodes()
		for node, ok := nodes.Next(); ok; node, ok = nodes.Next() {
			paths = append(paths, node.Path().String())
			kinds = append(kinds, node.Kind().String())
			depths = append(depths, fixtureNumberText(uint64(node.Depth())))
			children = append(children, fixtureNumberText(uint64(node.ChildCount())))
		}
		assertJSONFixtureList(t, record.ID, "paths", paths, record.Text("expected-paths"))
		assertJSONFixtureList(t, record.ID, "kinds", kinds, record.Text("expected-kinds"))
		assertJSONFixtureList(t, record.ID, "depths", depths, strings.ReplaceAll(record.Field("expected-depths"), ",", "\n"))
		assertJSONFixtureList(t, record.ID, "children", children, strings.ReplaceAll(record.Field("expected-children"), ",", "\n"))
	}
}

func TestJSONSourceNormalizesUTF8CopiesInputAndRejectsNormalizedDuplicates(t *testing.T) {
	values := []JSONValue{NewJSONString(string([]byte{'a', 0xff, 'b'}))}
	array := NewJSONArray(values)
	values[0] = NewJSONBoolean(true)
	stored, ok := array.Array()
	if !ok || stored[0].Kind() != JSONStringKind {
		t.Fatalf("array did not own its input")
	}
	text, _ := stored[0].Text()
	if text != "a\uFFFDb" {
		t.Fatalf("normalized string = %q", text)
	}

	_, err := NewJSONObject([]JSONMember{
		NewJSONMember(string([]byte{0xff}), NewJSONNull()),
		NewJSONMember(string([]byte{0xfe}), NewJSONBoolean(true)),
	})
	var duplicate *DuplicateJSONKeyError
	if !errors.As(err, &duplicate) || duplicate.Key != "\uFFFD" {
		t.Fatalf("duplicate error = %#v", err)
	}
}

func TestJSONZeroValuesAreSafeAndImmutableStorageIsShared(t *testing.T) {
	var number JSONNumber
	if number.String() != "0" {
		t.Fatalf("zero JSONNumber = %q", number.String())
	}
	var document JSONDocument
	if document.Serialized() != "null" || document.Len() != 1 || document.Root().Kind() != JSONNullKind {
		t.Fatalf("zero JSONDocument is not null")
	}
	clone := document
	if document.data() != clone.data() {
		t.Fatalf("document copy did not share immutable storage")
	}
}

func jsonFixtureArrangement(t *testing.T, name string) JSONValue {
	t.Helper()
	switch name {
	case "all-scalars":
		number, err := NewJSONNumber("-12.50e+2")
		if err != nil {
			t.Fatal(err)
		}
		value, err := NewJSONObject([]JSONMember{
			NewJSONMember("text", NewJSONString("A\n日")),
			NewJSONMember("number", NewJSONNumberValue(number)),
			NewJSONMember("truth", NewJSONBoolean(true)),
			NewJSONMember("nothing", NewJSONNull()),
		})
		if err != nil {
			t.Fatal(err)
		}
		return value
	case "nested-paths":
		emptyObject, err := NewJSONObject(nil)
		if err != nil {
			t.Fatal(err)
		}
		value, err := NewJSONObject([]JSONMember{
			NewJSONMember("a/b~c", NewJSONArray([]JSONValue{emptyObject, NewJSONArray(nil)})),
		})
		if err != nil {
			t.Fatal(err)
		}
		return value
	default:
		t.Fatalf("unknown arrangement %q", name)
		return JSONValue{}
	}
}

func assertJSONFixtureList(t *testing.T, id, field string, actual []string, expected string) {
	t.Helper()
	if got := strings.Join(actual, "\n"); got != expected {
		t.Errorf("case %s: %s = %q, want %q", id, field, got, expected)
	}
}

func fixtureNumberText(value uint64) string {
	return strconv.FormatUint(value, 10)
}
