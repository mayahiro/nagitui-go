package tui

import "github.com/mayahiro/nagi-go/vt"

// ColorKind is the VT-owned terminal color representation kind
type ColorKind = vt.ColorKind

const (
	// ColorDefault is the terminal's default color
	ColorDefault = vt.ColorDefault
	// ColorIndexed is an indexed terminal palette entry
	ColorIndexed = vt.ColorIndexed
	// ColorRGB is a 24-bit RGB color
	ColorRGB = vt.ColorRGB
)

// Color is the VT-owned terminal color type
type Color = vt.Color

// DefaultColor returns the terminal default color
func DefaultColor() Color {
	return vt.DefaultColor()
}

// IndexedColor returns an indexed terminal palette color
func IndexedColor(index uint8) Color {
	return vt.IndexedColor(index)
}

// RGBColor returns a 24-bit RGB color
func RGBColor(red, green, blue uint8) Color {
	return vt.RGBColor(red, green, blue)
}

// OptionalColor is the VT-owned optional terminal color type
type OptionalColor = vt.OptionalColor

// SomeColor returns an optional color containing color
func SomeColor(color Color) OptionalColor {
	return vt.SomeColor(color)
}

// Attributes is the VT-owned Boolean terminal attribute type
type Attributes = vt.Attributes

// Style is the VT-owned terminal style type
type Style = vt.Style
