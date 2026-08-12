package widget

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	tui "github.com/mayahiro/nagitui-go"
)

var benchmarkCodeDocument CodeDocument
var benchmarkCodeLayout CodeLayout
var benchmarkCodeViewNode tui.Node[struct{}]

func BenchmarkCodeDocument100K(b *testing.B) {
	lines := benchmarkCodeLines(b, 100_000)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		document, err := NewCodeDocument(lines, true)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkCodeDocument = document
	}
}

func BenchmarkCodeLayout100K(b *testing.B) {
	document, err := NewCodeDocument(benchmarkCodeLines(b, 100_000), true)
	if err != nil {
		b.Fatal(err)
	}
	options := DefaultCodeLayoutOptions().WithViewportWidth(80)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		layout, err := NewCodeLayout(document, options)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkCodeLayout = layout
	}
}

func BenchmarkCodeLayoutCache100K(b *testing.B) {
	document, err := NewCodeDocument(benchmarkCodeLines(b, 100_000), true)
	if err != nil {
		b.Fatal(err)
	}
	cache := NewCodeLayoutCache()
	options := DefaultCodeLayoutOptions().WithViewportWidth(80)
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
		benchmarkCodeLayout = layout
	}
}

func BenchmarkCodeViewViewport(b *testing.B) {
	for _, count := range []int{8, 100_000} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			document, err := NewCodeDocument(benchmarkCodeLines(b, count), true)
			if err != nil {
				b.Fatal(err)
			}
			layout, err := NewCodeLayout(document, DefaultCodeLayoutOptions().WithViewportWidth(80))
			if err != nil {
				b.Fatal(err)
			}
			state := NewCodeViewState(count / 2)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchmarkCodeViewNode = NewCodeView(
					"code", layout, state, func(CodeViewState) struct{} { return struct{}{} },
				).Viewport(8).OnCopy(func(CodeCopyRequest) struct{} { return struct{}{} }).Node()
			}
		})
	}
}

func BenchmarkCodeViewLongLineOffset(b *testing.B) {
	const bytes = 1024 * 1024
	document, err := NewCodeDocument([]CodeLine{mustBenchmarkCodeLine(b, strings.Repeat("a", bytes)+"END")}, false)
	if err != nil {
		b.Fatal(err)
	}
	layout, err := NewCodeLayout(
		document, DefaultCodeLayoutOptions().WithViewportWidth(80).WithLineNumbers(false),
	)
	if err != nil {
		b.Fatal(err)
	}
	state := NewCodeViewState(0).WithHorizontalOffset(bytes)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		benchmarkCodeViewNode = NewCodeView(
			"code", layout, state, func(CodeViewState) struct{} { return struct{}{} },
		).Viewport(1).Node()
	}
}

func benchmarkCodeLines(b *testing.B, count int) []CodeLine {
	b.Helper()
	lines := make([]CodeLine, count)
	for index := range lines {
		lines[index] = mustBenchmarkCodeLine(b, fmt.Sprintf("%06d let value = \"Nagi\";", index))
	}
	return lines
}

func mustBenchmarkCodeLine(b *testing.B, text string) CodeLine {
	b.Helper()
	line, err := NewCodeLine(text)
	if err != nil {
		b.Fatal(err)
	}
	return line
}
