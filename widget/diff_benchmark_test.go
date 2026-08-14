package widget

import (
	"fmt"
	"strconv"
	"testing"

	tui "github.com/mayahiro/nagitui-go"
)

var benchmarkDiffDocument DiffDocument
var benchmarkDiffLayout DiffLayout
var benchmarkDiffText string
var benchmarkDiffViewNode tui.Node[struct{}]

func BenchmarkDiffDocument100K(b *testing.B) {
	lines := benchmarkDiffLines(b, 100_000)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		document, err := NewDiffDocument(lines, true)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkDiffDocument = document
	}
}

func BenchmarkDiffLayout100K(b *testing.B) {
	document, err := NewDiffDocument(benchmarkDiffLines(b, 100_000), true)
	if err != nil {
		b.Fatal(err)
	}
	options := DefaultDiffLayoutOptions().WithViewportWidth(80)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		layout, err := NewDiffLayout(document, options)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkDiffLayout = layout
	}
}

func BenchmarkDiffLayoutCache100K(b *testing.B) {
	document, err := NewDiffDocument(benchmarkDiffLines(b, 100_000), true)
	if err != nil {
		b.Fatal(err)
	}
	var cache DiffLayoutCache
	options := DefaultDiffLayoutOptions().WithViewportWidth(80)
	if _, err := cache.Resolve(1, document, options); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		layout, err := cache.Resolve(1, document, options)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkDiffLayout = layout
	}
}

func BenchmarkDiffCopy100K(b *testing.B) {
	document, err := NewDiffDocument(benchmarkDiffLines(b, 100_000), true)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		text, ok := document.CopyTextForLines(0, document.LineCount())
		if !ok {
			b.Fatal("copy unavailable")
		}
		benchmarkDiffText = text
	}
}

func BenchmarkDiffViewViewport(b *testing.B) {
	for _, count := range []int{8, 100_000} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			document, err := NewDiffDocument(benchmarkDiffLines(b, count), true)
			if err != nil {
				b.Fatal(err)
			}
			layout, err := NewDiffLayout(document, DefaultDiffLayoutOptions().WithViewportWidth(80))
			if err != nil {
				b.Fatal(err)
			}
			state := NewDiffViewState(count / 2)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchmarkDiffViewNode = NewDiffView(
					"diff", layout, state, func(DiffViewState) struct{} { return struct{}{} },
				).Viewport(8).OnCopy(func(DiffCopyRequest) struct{} { return struct{}{} }).Node()
			}
		})
	}
}

func benchmarkDiffLines(b *testing.B, count int) []DiffLine {
	b.Helper()
	lines := make([]DiffLine, count)
	for index := range lines {
		number := uint64(index) + 1
		content := mustBenchmarkCodeLine(b, fmt.Sprintf("%06d let value = \"Nagi\";", index))
		var line DiffLine
		var err error
		switch index % 3 {
		case 0:
			line, err = NewDiffContextLine(number, number, content)
		case 1:
			line, err = NewDiffDeletionLine(number, content)
		default:
			line, err = NewDiffAdditionLine(number, content)
		}
		if err != nil {
			b.Fatal(err)
		}
		lines[index] = line
	}
	return lines
}
