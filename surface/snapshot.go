package surface

import (
	"fmt"
	"strings"

	"github.com/mayahiro/nagi-go/vt"
)

func snapshot(surface *Surface) string {
	var output strings.Builder
	fmt.Fprintf(&output, "nagi-surface-v1\twidth=%d\theight=%d\tcursor=", surface.width, surface.height)
	if surface.hasCursor {
		fmt.Fprintf(&output, "%d,%d", surface.cursor.X, surface.cursor.Y)
	} else {
		output.WriteString("none")
	}
	output.WriteByte('\n')
	for row := uint32(0); row < surface.height; row++ {
		fmt.Fprintf(&output, "row=%d", row)
		for column := uint32(0); column < surface.width; column++ {
			output.WriteByte('\t')
			writeCell(&output, surface.cellAt(surface.index(int(column), int(row))))
		}
		output.WriteByte('\n')
	}
	return output.String()
}

func writeCell(output *strings.Builder, cell Cell) {
	writeContent(output, cell.Content())
	fmt.Fprintf(output, "/%d/", cell.Span().Cells())
	if cell.Continuation() {
		output.WriteString("cont")
	} else {
		output.WriteString("lead")
	}
	if cell.Opacity() == Transparent {
		output.WriteString("/transparent/")
	} else {
		output.WriteString("/opaque/")
	}
	style := cell.Style()
	writeColor(output, style.Foreground)
	output.WriteByte('/')
	writeColor(output, style.Background)
	output.WriteByte('/')
	if color, ok := style.UnderlineColor.Get(); ok {
		writeColor(output, color)
	} else {
		output.WriteString("none")
	}
	output.WriteByte('/')
	writeAttributes(output, style)
}

func writeContent(output *strings.Builder, content string) {
	if content == "" {
		output.WriteByte('-')
		return
	}
	wroteCharacter := false
	for _, character := range content {
		if wroteCharacter {
			output.WriteByte('+')
		}
		fmt.Fprintf(output, "U+%04X", character)
		wroteCharacter = true
	}
}

func writeColor(output *strings.Builder, color vt.Color) {
	switch color.Kind() {
	case vt.ColorIndexed:
		index, _ := color.Index()
		fmt.Fprintf(output, "indexed:%d", index)
	case vt.ColorRGB:
		red, green, blue, _ := color.RGB()
		fmt.Fprintf(output, "rgb:%02X%02X%02X", red, green, blue)
	default:
		output.WriteString("default")
	}
}

func writeAttributes(output *strings.Builder, style vt.Style) {
	attributes := []struct {
		enabled bool
		name    string
	}{
		{style.Bold, "bold"},
		{style.Dim, "dim"},
		{style.Italic, "italic"},
		{style.Underline, "underline"},
		{style.Blink, "blink"},
		{style.Reverse, "reverse"},
		{style.Hidden, "hidden"},
		{style.Strikethrough, "strikethrough"},
	}
	wroteAttribute := false
	for _, attribute := range attributes {
		if !attribute.enabled {
			continue
		}
		if wroteAttribute {
			output.WriteByte('+')
		}
		output.WriteString(attribute.name)
		wroteAttribute = true
	}
	if !wroteAttribute {
		output.WriteByte('-')
	}
}
