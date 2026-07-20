package tui

import (
	"github.com/mayahiro/nagi-go/vt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ANSITextOptions controls paragraph behavior after safe ANSI SGR parsing
type ANSITextOptions struct {
	// Paragraph contains wrapping and alignment settings for parsed spans
	Paragraph ParagraphOptions
}

// ANSIText parses SGR styling and discards every non-display control sequence
//
// CSI commands other than SGR, OSC, DCS, SOS, PM, APC, raw escape sequences,
// and non-line-breaking control characters never reach terminal output.
func ANSIText[Message any](input string, options ANSITextOptions) Node[Message] {
	return Paragraph[Message](parseANSISpans(input), options.Paragraph)
}

func parseANSISpans(input string) []TextSpan {
	spans := make([]TextSpan, 0, 1)
	var text strings.Builder
	style := vt.Style{}
	for index := 0; index < len(input); {
		character, next := nextANSICharacter(input, index)
		switch character {
		case '\x1b':
			after, nextStyle, hasStyle := parseANSIEscape(input, next, style)
			if hasStyle && nextStyle != style {
				pushANSISpan(&spans, &text, style)
				style = nextStyle
			}
			index = after
		case '\u009b':
			after, body, sgr := scanANSICSI(input, next)
			if sgr {
				nextStyle := applyANSISGR(style, body)
				if nextStyle != style {
					pushANSISpan(&spans, &text, style)
					style = nextStyle
				}
			}
			index = after
		case '\u009d':
			index = skipANSIControlString(input, next, true)
		case '\u0090', '\u0098', '\u009e', '\u009f':
			index = skipANSIControlString(input, next, false)
		case '\r', '\n':
			text.WriteRune(character)
			index = next
		default:
			if !unicode.IsControl(character) {
				text.WriteRune(character)
			}
			index = next
		}
	}
	pushANSISpan(&spans, &text, style)
	return spans
}

func parseANSIEscape(input string, start int, style vt.Style) (int, vt.Style, bool) {
	if start >= len(input) {
		return start, style, false
	}
	introducer, next := nextANSICharacter(input, start)
	switch introducer {
	case '[':
		after, body, sgr := scanANSICSI(input, next)
		if !sgr {
			return after, style, false
		}
		return after, applyANSISGR(style, body), true
	case ']':
		return skipANSIControlString(input, next, true), style, false
	case 'P', 'X', '^', '_':
		return skipANSIControlString(input, next, false), style, false
	default:
		return skipANSIEscapeSequence(input, start), style, false
	}
}

func scanANSICSI(input string, start int) (int, string, bool) {
	for index := start; index < len(input); index++ {
		value := input[index]
		if value >= 0x40 && value <= 0x7e {
			return index + 1, input[start:index], value == 'm'
		}
		if value == '\n' || value == '\r' || value == '\x1b' {
			return index, "", false
		}
	}
	return len(input), "", false
}

func skipANSIControlString(input string, start int, bellTerminated bool) int {
	for index := start; index < len(input); {
		character, next := nextANSICharacter(input, index)
		if character == '\u009c' || bellTerminated && character == '\a' {
			return next
		}
		if character == '\x1b' && next < len(input) {
			terminator, after := nextANSICharacter(input, next)
			if terminator == '\\' {
				return after
			}
		}
		index = next
	}
	return len(input)
}

func skipANSIEscapeSequence(input string, start int) int {
	index := start
	for index < len(input) {
		character, next := nextANSICharacter(input, index)
		if character >= 0x20 && character <= 0x2f {
			index = next
			continue
		}
		if character >= 0x30 && character <= 0x7e {
			return next
		}
		break
	}
	return index
}

func applyANSISGR(style vt.Style, body string) vt.Style {
	if body == "" {
		return vt.Style{}
	}
	for _, value := range []byte(body) {
		if (value < '0' || value > '9') && value != ';' && value != ':' {
			return style
		}
	}
	parameters := strings.Split(body, ";")
	for index := 0; index < len(parameters); {
		parameter := parameters[index]
		if strings.ContainsRune(parameter, ':') {
			applyANSIColonSGR(&style, parameter)
			index++
			continue
		}
		code, ok := parseANSIParameter(parameter)
		if !ok {
			index++
			continue
		}
		if code == 38 || code == 48 || code == 58 {
			if color, consumed, ok := ansiSemicolonColor(parameters[index+1:]); ok {
				setANSIColor(&style, code, color)
				index += consumed + 1
				continue
			}
			index++
			continue
		}
		applyANSIBasicSGR(&style, code)
		index++
	}
	return style
}

func parseANSIParameter(parameter string) (int, bool) {
	if parameter == "" {
		return 0, true
	}
	value, err := strconv.Atoi(parameter)
	return value, err == nil
}

func ansiSemicolonColor(parameters []string) (vt.Color, int, bool) {
	if len(parameters) == 0 {
		return vt.Color{}, 0, false
	}
	mode, ok := parseANSIParameter(parameters[0])
	if !ok {
		return vt.Color{}, 0, false
	}
	switch mode {
	case 5:
		if len(parameters) < 2 {
			return vt.Color{}, 0, false
		}
		index, ok := ansiColorComponent(parameters[1])
		return vt.IndexedColor(index), 2, ok
	case 2:
		if len(parameters) < 4 {
			return vt.Color{}, 0, false
		}
		red, redOK := ansiColorComponent(parameters[1])
		green, greenOK := ansiColorComponent(parameters[2])
		blue, blueOK := ansiColorComponent(parameters[3])
		return vt.RGBColor(red, green, blue), 4, redOK && greenOK && blueOK
	default:
		return vt.Color{}, 0, false
	}
}

func applyANSIColonSGR(style *vt.Style, parameter string) {
	parts := strings.Split(parameter, ":")
	code, ok := parseANSIParameter(parts[0])
	if !ok {
		return
	}
	if code == 4 {
		style.Underline = len(parts) < 2 || parts[1] != "0"
		return
	}
	if code != 38 && code != 48 && code != 58 {
		applyANSIBasicSGR(style, code)
		return
	}
	if len(parts) < 3 {
		return
	}
	mode, ok := parseANSIParameter(parts[1])
	if !ok {
		return
	}
	var color vt.Color
	switch mode {
	case 5:
		index, ok := ansiColorComponent(parts[2])
		if !ok {
			return
		}
		color = vt.IndexedColor(index)
	case 2:
		components := make([]uint8, 0, 4)
		for _, part := range parts[2:] {
			if part == "" {
				continue
			}
			component, ok := ansiColorComponent(part)
			if !ok {
				return
			}
			components = append(components, component)
		}
		if len(components) < 3 {
			return
		}
		components = components[len(components)-3:]
		color = vt.RGBColor(components[0], components[1], components[2])
	default:
		return
	}
	setANSIColor(style, code, color)
}

func ansiColorComponent(parameter string) (uint8, bool) {
	value, ok := parseANSIParameter(parameter)
	return uint8(value), ok && value >= 0 && value <= 255
}

func applyANSIBasicSGR(style *vt.Style, code int) {
	switch {
	case code == 0:
		*style = vt.Style{}
	case code == 1:
		style.Bold = true
	case code == 2:
		style.Dim = true
	case code == 3:
		style.Italic = true
	case code == 4 || code == 21:
		style.Underline = true
	case code == 5 || code == 6:
		style.Blink = true
	case code == 7:
		style.Reverse = true
	case code == 8:
		style.Hidden = true
	case code == 9:
		style.Strikethrough = true
	case code == 22:
		style.Bold, style.Dim = false, false
	case code == 23:
		style.Italic = false
	case code == 24:
		style.Underline = false
	case code == 25:
		style.Blink = false
	case code == 27:
		style.Reverse = false
	case code == 28:
		style.Hidden = false
	case code == 29:
		style.Strikethrough = false
	case code >= 30 && code <= 37:
		style.Foreground = vt.IndexedColor(uint8(code - 30))
	case code == 39:
		style.Foreground = vt.DefaultColor()
	case code >= 40 && code <= 47:
		style.Background = vt.IndexedColor(uint8(code - 40))
	case code == 49:
		style.Background = vt.DefaultColor()
	case code == 59:
		style.UnderlineColor = vt.OptionalColor{}
	case code >= 90 && code <= 97:
		style.Foreground = vt.IndexedColor(uint8(code - 90 + 8))
	case code >= 100 && code <= 107:
		style.Background = vt.IndexedColor(uint8(code - 100 + 8))
	}
}

func setANSIColor(style *vt.Style, code int, color vt.Color) {
	switch code {
	case 38:
		style.Foreground = color
	case 48:
		style.Background = color
	case 58:
		style.UnderlineColor = vt.SomeColor(color)
	}
}

func nextANSICharacter(input string, index int) (rune, int) {
	character, width := utf8.DecodeRuneInString(input[index:])
	return character, index + width
}

func pushANSISpan(spans *[]TextSpan, text *strings.Builder, style vt.Style) {
	if text.Len() == 0 {
		return
	}
	*spans = append(*spans, NewTextSpan(text.String(), style))
	text.Reset()
}
