package surface

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go/internal/conformance"
)

func TestSnapshotFixtures(t *testing.T) {
	records := surfaceRecords(t, "surface/snapshots.txt", "surface-snapshots",
		"width", "height", "background", "operations", "cursor", "expected")
	for _, record := range records {
		surface := fixtureSurface(t,
			record.Field("width"), record.Field("height"), record.Field("background"),
			record.Field("operations"), record.Field("cursor"))
		if got, want := surface.Snapshot(), record.Text("expected"); got != want {
			t.Errorf("case %s: Snapshot() =\n%s\nwant:\n%s", record.ID, got, want)
		}
	}
}

func TestCompositionFixtures(t *testing.T) {
	records := surfaceRecords(t, "surface/composition.txt", "surface-composition",
		"width", "height", "base-background", "base", "base-cursor",
		"layer-width", "layer-height", "layer-background", "layer", "layer-cursor",
		"offset", "expected")
	for _, record := range records {
		base := fixtureSurface(t,
			record.Field("width"), record.Field("height"), record.Field("base-background"),
			record.Field("base"), record.Field("base-cursor"))
		layer := fixtureSurface(t,
			record.Field("layer-width"), record.Field("layer-height"), record.Field("layer-background"),
			record.Field("layer"), record.Field("layer-cursor"))
		offsetX, offsetY := fixtureSignedPair(record.Field("offset"))
		base.Composite(layer, offsetX, offsetY)
		if got, want := base.Snapshot(), record.Text("expected"); got != want {
			t.Errorf("case %s: composed Snapshot() =\n%s\nwant:\n%s", record.ID, got, want)
		}
	}
}

func TestDiffFixtures(t *testing.T) {
	records := surfaceRecords(t, "surface/diff.txt", "surface-diff",
		"width", "height", "previous", "current", "expected")
	for _, record := range records {
		previous := fixtureSurface(t, record.Field("width"), record.Field("height"), "opaque", record.Field("previous"), "none")
		current := fixtureSurface(t, record.Field("width"), record.Field("height"), "opaque", record.Field("current"), "none")
		got := current.ChangedRuns(previous)
		want := fixtureRuns(record.Field("expected"))
		if !reflect.DeepEqual(got, want) {
			t.Errorf("case %s: ChangedRuns() = %v, want %v", record.ID, got, want)
		}
	}
}

func surfaceRecords(t *testing.T, path, suite string, fields ...string) []conformance.Record {
	t.Helper()
	records, err := conformance.Load(path, suite, fields...)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	return records
}

func fixtureSurface(t *testing.T, width, height, background, operations, cursor string) *Surface {
	t.Helper()
	var (
		surface *Surface
		err     error
	)
	switch background {
	case "opaque":
		surface, err = New(fixtureUnsigned(width), fixtureUnsigned(height))
	case "transparent":
		surface, err = NewTransparent(fixtureUnsigned(width), fixtureUnsigned(height))
	default:
		t.Fatalf("unknown surface background %s", background)
	}
	if err != nil {
		t.Fatalf("fixture surface allocation failed: %v", err)
	}
	applyFixtureOperations(surface, operations)
	if cursor != "none" {
		x, y := fixtureUnsignedPair(cursor)
		if !surface.SetCursor(Cursor{X: x, Y: y}) {
			t.Fatalf("fixture cursor %s is outside the surface", cursor)
		}
	}
	return surface
}

func applyFixtureOperations(surface *Surface, operations string) {
	if operations == "-" {
		return
	}
	for _, operation := range strings.Split(operations, ";") {
		fields := strings.Split(operation, ",")
		switch fields[0] {
		case "write":
			if len(fields) != 5 {
				panic("invalid write operation " + operation)
			}
			surface.Write(fixtureSigned(fields[1]), fixtureSigned(fields[2]), fixtureScalarText(fields[3]), fixtureStyle(fields[4]), celltext.ModernWidth())
		case "fill":
			if len(fields) != 6 {
				panic("invalid fill operation " + operation)
			}
			surface.Fill(fixtureSigned(fields[1]), fixtureSigned(fields[2]), fixtureUnsigned(fields[3]), fixtureUnsigned(fields[4]), fixtureStyle(fields[5]))
		case "fill-transparent":
			if len(fields) != 6 {
				panic("invalid transparent fill operation " + operation)
			}
			surface.FillTransparent(fixtureSigned(fields[1]), fixtureSigned(fields[2]), fixtureUnsigned(fields[3]), fixtureUnsigned(fields[4]), fixtureStyle(fields[5]))
		default:
			panic("unknown surface operation " + operation)
		}
	}
}

func fixtureStyle(value string) vt.Style {
	var style vt.Style
	if value == "-" {
		return style
	}
	for _, token := range strings.Split(value, "+") {
		switch {
		case strings.HasPrefix(token, "fg-"):
			style.Foreground = fixtureColor(strings.TrimPrefix(token, "fg-"))
		case strings.HasPrefix(token, "bg-"):
			style.Background = fixtureColor(strings.TrimPrefix(token, "bg-"))
		case strings.HasPrefix(token, "underline-color-"):
			style.UnderlineColor = vt.SomeColor(fixtureColor(strings.TrimPrefix(token, "underline-color-")))
		default:
			switch token {
			case "bold":
				style.Bold = true
			case "dim":
				style.Dim = true
			case "italic":
				style.Italic = true
			case "underline":
				style.Underline = true
			case "blink":
				style.Blink = true
			case "reverse":
				style.Reverse = true
			case "hidden":
				style.Hidden = true
			case "strikethrough":
				style.Strikethrough = true
			default:
				panic("unknown fixture style " + token)
			}
		}
	}
	return style
}

func fixtureColor(value string) vt.Color {
	if index, ok := strings.CutPrefix(value, "indexed-"); ok {
		number, err := strconv.ParseUint(index, 10, 8)
		if err != nil {
			panic(fmt.Sprintf("invalid color index %s: %v", index, err))
		}
		return vt.IndexedColor(uint8(number))
	}
	if rgb, ok := strings.CutPrefix(value, "rgb-"); ok {
		if len(rgb) != 6 {
			panic("invalid RGB color " + rgb)
		}
		return vt.RGBColor(uint8(fixtureHex(rgb[0:2])), uint8(fixtureHex(rgb[2:4])), uint8(fixtureHex(rgb[4:6])))
	}
	panic("unknown fixture color " + value)
}

func fixtureScalarText(value string) string {
	var output strings.Builder
	for _, scalar := range strings.Split(value, "+") {
		character := rune(fixtureHex(scalar))
		if !utf8.ValidRune(character) {
			panic("invalid Unicode scalar " + scalar)
		}
		output.WriteRune(character)
	}
	return output.String()
}

func fixtureRuns(value string) []ChangedRun {
	if value == "none" {
		return nil
	}
	rawRuns := strings.Split(value, ",")
	runs := make([]ChangedRun, len(rawRuns))
	for index, raw := range rawRuns {
		row, columns, ok := strings.Cut(raw, ":")
		if !ok {
			panic("invalid changed run " + raw)
		}
		start, end, ok := strings.Cut(columns, "-")
		if !ok {
			panic("invalid changed run " + raw)
		}
		runs[index] = ChangedRun{Row: fixtureUnsigned(row), Start: fixtureUnsigned(start), End: fixtureUnsigned(end)}
	}
	return runs
}

func fixtureUnsignedPair(value string) (uint32, uint32) {
	left, right, ok := strings.Cut(value, ",")
	if !ok {
		panic("invalid unsigned pair " + value)
	}
	return fixtureUnsigned(left), fixtureUnsigned(right)
}

func fixtureSignedPair(value string) (int32, int32) {
	left, right, ok := strings.Cut(value, ",")
	if !ok {
		panic("invalid signed pair " + value)
	}
	return fixtureSigned(left), fixtureSigned(right)
}

func fixtureUnsigned(value string) uint32 {
	number, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		panic(fmt.Sprintf("invalid unsigned integer %s: %v", value, err))
	}
	return uint32(number)
}

func fixtureSigned(value string) int32 {
	number, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		panic(fmt.Sprintf("invalid signed integer %s: %v", value, err))
	}
	return int32(number)
}

func fixtureHex(value string) uint32 {
	number, err := strconv.ParseUint(value, 16, 32)
	if err != nil {
		panic(fmt.Sprintf("invalid hexadecimal integer %s: %v", value, err))
	}
	return uint32(number)
}
