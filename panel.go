package tui

import (
	"github.com/mayahiro/nagi-go/vt"
)

// BorderKind selects a built-in one-cell panel border
type BorderKind uint8

const (
	// BorderSingle uses square single-line box drawing characters
	BorderSingle BorderKind = iota
	// BorderRounded uses rounded single-line corners
	BorderRounded
	// BorderDouble uses double-line box drawing characters
	BorderDouble
	// BorderThick uses heavy box drawing characters
	BorderThick
)

// PanelStyle contains the visual styles used by a Panel node
type PanelStyle struct {
	// Border styles the border cells
	Border vt.Style
	// Title styles the optional title
	Title vt.Style
	// Background fills the complete panel rectangle before child rendering
	Background vt.Style
}

// PanelOptions controls panel border, padding, and styles
type PanelOptions struct {
	// Border selects the border character set
	Border BorderKind
	// Padding is applied inside the one-cell border
	Padding Insets
	// Style controls panel colors and attributes
	Style PanelStyle
}

// DefaultPanelOptions returns a single border with one-cell inner padding
func DefaultPanelOptions() PanelOptions {
	return PanelOptions{Border: BorderSingle, Padding: UniformInsets(1)}
}

type borderGlyphs struct {
	topLeft, horizontal, topRight     string
	vertical, bottomLeft, bottomRight string
}

func glyphsForBorder(kind BorderKind) borderGlyphs {
	switch kind {
	case BorderRounded:
		return borderGlyphs{"╭", "─", "╮", "│", "╰", "╯"}
	case BorderDouble:
		return borderGlyphs{"╔", "═", "╗", "║", "╚", "╝"}
	case BorderThick:
		return borderGlyphs{"┏", "━", "┓", "┃", "┗", "┛"}
	case BorderSingle:
		fallthrough
	default:
		return borderGlyphs{"┌", "─", "┐", "│", "└", "┘"}
	}
}

func panelContentInsets(options PanelOptions) Insets {
	return Insets{
		Top:    saturatingAdd32(1, options.Padding.Top),
		Right:  saturatingAdd32(1, options.Padding.Right),
		Bottom: saturatingAdd32(1, options.Padding.Bottom),
		Left:   saturatingAdd32(1, options.Padding.Left),
	}
}
