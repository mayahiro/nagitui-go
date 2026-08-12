package widget

import (
	"strconv"
	"testing"

	"github.com/mayahiro/nagitui-go"
)

var benchmarkJSONDocument JSONDocument
var benchmarkJSONInspectorNode tui.Node[struct{}]

func BenchmarkJSONDocument100K(b *testing.B) {
	root := benchmarkJSONArray(b, 99_999)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		document, err := NewJSONDocument(root)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkJSONDocument = document
	}
}

func BenchmarkJSONInspectorViewport(b *testing.B) {
	for _, count := range []int{8, 99_999} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			document, err := NewJSONDocument(benchmarkJSONArray(b, count))
			if err != nil {
				b.Fatal(err)
			}
			selected, err := NewJSONPointer("/" + strconv.Itoa(count/2))
			if err != nil {
				b.Fatal(err)
			}
			state := NewJSONInspectorState(selected, []JSONPointer{RootJSONPointer()})
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchmarkJSONInspectorNode = NewJSONInspector(
					"inspector", document, state, func(JSONInspectorState) struct{} { return struct{}{} },
				).Viewport(8).OnCopy(func(JSONInspectorCopyRequest) struct{} { return struct{}{} }).Node()
			}
		})
	}
}

func benchmarkJSONArray(b *testing.B, count int) JSONValue {
	b.Helper()
	values := make([]JSONValue, count)
	for index := range values {
		number, err := NewJSONNumber(strconv.Itoa(index))
		if err != nil {
			b.Fatal(err)
		}
		values[index] = NewJSONNumberValue(number)
	}
	return NewJSONArray(values)
}
