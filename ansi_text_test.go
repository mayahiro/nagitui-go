package tui

import (
	"errors"
	"github.com/mayahiro/nagi-go/vt"
	"strconv"
	"strings"
	"testing"

	"github.com/mayahiro/nagitui-go/internal/conformance"
)

func TestANSISGRAndControlFiltering(t *testing.T) {
	spans := parseANSISpans("plain\x1b[31;1mred\x1b[0m!\x1b]8;;https://invalid.example\x07link\x1b]8;;\x07\x1b[2Jdone\b")
	var text strings.Builder
	for _, span := range spans {
		text.WriteString(span.Text)
		if strings.ContainsRune(span.Text, '\x1b') {
			t.Fatal("parsed span retained ESC")
		}
	}
	if text.String() != "plainred!linkdone" {
		t.Fatalf("text = %q", text.String())
	}
	if index, ok := spans[1].Style.Foreground.Index(); !ok || index != 1 || !spans[1].Style.Bold {
		t.Fatalf("red style = %+v", spans[1].Style)
	}
}

func TestANSIIndexedRGBColonAndAttributeResets(t *testing.T) {
	spans := parseANSISpans("\x1b[38;5;200mA\x1b[48;2;1;2;3mB\x1b[38:2::4:5:6;3mC\x1b[23;39;49mD")
	if index, ok := spans[0].Style.Foreground.Index(); !ok || index != 200 {
		t.Fatalf("indexed style = %+v", spans[0].Style)
	}
	if red, green, blue, ok := spans[1].Style.Background.RGB(); !ok || red != 1 || green != 2 || blue != 3 {
		t.Fatalf("background = %+v", spans[1].Style.Background)
	}
	if red, green, blue, ok := spans[2].Style.Foreground.RGB(); !ok || red != 4 || green != 5 || blue != 6 {
		t.Fatalf("foreground = %+v", spans[2].Style.Foreground)
	}
	if !spans[2].Style.Italic || spans[3].Style != (vt.Style{}) {
		t.Fatalf("reset styles = %+v, %+v", spans[2].Style, spans[3].Style)
	}
}

func TestANSITextUsesParagraphOptions(t *testing.T) {
	node := ANSIText[int]("A\x1b[32mBC", ANSITextOptions{
		Paragraph: ParagraphOptions{Wrap: WrapHard, Alignment: AlignEnd},
	})
	if node.kind != nodeRichText || node.paragraph.Wrap != WrapHard || node.paragraph.Alignment != AlignEnd {
		t.Fatalf("ANSIText node = %+v", node)
	}
}

func TestANSIParsingMatchesSharedFixtures(t *testing.T) {
	records, err := conformance.Load(
		"text/ansi.txt",
		"ansi-text",
		"input", "text", "foregrounds", "backgrounds", "attributes",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			spans := parseANSISpans(record.Text("input"))
			var text strings.Builder
			foregrounds := make([]string, len(spans))
			backgrounds := make([]string, len(spans))
			attributes := make([]string, len(spans))
			for index, span := range spans {
				text.WriteString(span.Text)
				foregrounds[index] = ansiColorSignature(span.Style.Foreground)
				backgrounds[index] = ansiColorSignature(span.Style.Background)
				attributes[index] = ansiAttributeSignature(span.Style)
			}
			if text.String() != record.Text("text") {
				t.Fatalf("text = %q", text.String())
			}
			if strings.Join(foregrounds, "|") != record.Field("foregrounds") {
				t.Fatalf("foregrounds = %q", strings.Join(foregrounds, "|"))
			}
			if strings.Join(backgrounds, "|") != record.Field("backgrounds") {
				t.Fatalf("backgrounds = %q", strings.Join(backgrounds, "|"))
			}
			if strings.Join(attributes, "|") != record.Field("attributes") {
				t.Fatalf("attributes = %q", strings.Join(attributes, "|"))
			}
		})
	}
}

func ansiColorSignature(color vt.Color) string {
	switch color.Kind() {
	case vt.ColorDefault:
		return "default"
	case vt.ColorIndexed:
		index, _ := color.Index()
		return "index:" + strconv.Itoa(int(index))
	case vt.ColorRGB:
		red, green, blue, _ := color.RGB()
		return "rgb:" + strconv.Itoa(int(red)) + "," + strconv.Itoa(int(green)) + "," + strconv.Itoa(int(blue))
	default:
		return "invalid"
	}
}

func ansiAttributeSignature(style vt.Style) string {
	var attributes []string
	if style.Bold {
		attributes = append(attributes, "bold")
	}
	if style.Dim {
		attributes = append(attributes, "dim")
	}
	if style.Italic {
		attributes = append(attributes, "italic")
	}
	if style.Underline {
		attributes = append(attributes, "underline")
	}
	if style.Blink {
		attributes = append(attributes, "blink")
	}
	if style.Reverse {
		attributes = append(attributes, "reverse")
	}
	if style.Hidden {
		attributes = append(attributes, "hidden")
	}
	if style.Strikethrough {
		attributes = append(attributes, "strikethrough")
	}
	if len(attributes) == 0 {
		return "-"
	}
	return strings.Join(attributes, "+")
}
