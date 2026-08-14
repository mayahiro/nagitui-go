package tui

import (
	"os"
	"strings"
)

// TerminalCapabilityDetection controls standard terminal capability detection
type TerminalCapabilityDetection uint8

const (
	// TerminalCapabilityDetectionDisabled preserves explicit output and legacy input
	TerminalCapabilityDetectionDisabled TerminalCapabilityDetection = iota
	// TerminalCapabilityDetectionEnabled inspects hints and queries extended input
	TerminalCapabilityDetectionEnabled
)

// TerminalFeatureSupport is evidence available for one terminal feature
type TerminalFeatureSupport uint8

const (
	// TerminalFeatureUnknown means support could not be established
	TerminalFeatureUnknown TerminalFeatureSupport = iota
	// TerminalFeatureUnsupported means the terminal established unavailability
	TerminalFeatureUnsupported
	// TerminalFeatureSupported means the terminal established availability
	TerminalFeatureSupported
)

// TerminalColorLevel is the color level advertised by the environment
type TerminalColorLevel uint8

const (
	// TerminalColorUnknown means no reliable color hint is available
	TerminalColorUnknown TerminalColorLevel = iota
	// TerminalColorMonochrome requests a terminal without color
	TerminalColorMonochrome
	// TerminalColorANSI16 advertises the ANSI color baseline
	TerminalColorANSI16
	// TerminalColorIndexed256 advertises the indexed 256-color palette
	TerminalColorIndexed256
	// TerminalColorTrueColor advertises 24-bit color
	TerminalColorTrueColor
)

// TerminalKeyboardProtocol is the protocol enabled by the standard runner
type TerminalKeyboardProtocol uint8

const (
	// TerminalKeyboardLegacy is traditional terminal keyboard input
	TerminalKeyboardLegacy TerminalKeyboardProtocol = iota
	// TerminalKeyboardKitty uses Kitty keyboard protocol enhancements
	TerminalKeyboardKitty
)

// TerminalCapabilityProfile contains detected features and active input mode
//
// Feature support is observational metadata and does not grant clipboard or
// other output permission, which remains controlled by TerminalOptions.
type TerminalCapabilityProfile struct {
	colorLevel       TerminalColorLevel
	prefersNoColor   bool
	hyperlinks       TerminalFeatureSupport
	clipboard        TerminalFeatureSupport
	extendedKeyboard TerminalFeatureSupport
	keyboardProtocol TerminalKeyboardProtocol
}

// ColorLevel returns the advertised terminal color level
func (p TerminalCapabilityProfile) ColorLevel() TerminalColorLevel {
	return p.colorLevel
}

// PrefersNoColor reports a non-empty NO_COLOR environment preference
func (p TerminalCapabilityProfile) PrefersNoColor() bool {
	return p.prefersNoColor
}

// Hyperlinks returns detected hyperlink support
func (p TerminalCapabilityProfile) Hyperlinks() TerminalFeatureSupport {
	return p.hyperlinks
}

// Clipboard returns detected terminal clipboard support
func (p TerminalCapabilityProfile) Clipboard() TerminalFeatureSupport {
	return p.clipboard
}

// ExtendedKeyboard returns actively queried extended-keyboard support
func (p TerminalCapabilityProfile) ExtendedKeyboard() TerminalFeatureSupport {
	return p.extendedKeyboard
}

// KeyboardProtocol returns the keyboard protocol currently enabled by the runner
func (p TerminalCapabilityProfile) KeyboardProtocol() TerminalKeyboardProtocol {
	return p.keyboardProtocol
}

// ModifiedKeySupport maps extended-keyboard evidence to binding metadata
func (p TerminalCapabilityProfile) ModifiedKeySupport() BindingSupport {
	switch p.extendedKeyboard {
	case TerminalFeatureUnsupported:
		return BindingUnsupported
	case TerminalFeatureSupported:
		return BindingSupported
	default:
		return BindingSupportUnknown
	}
}

// WithColorLevel returns a copy with an explicit color level
func (p TerminalCapabilityProfile) WithColorLevel(level TerminalColorLevel) TerminalCapabilityProfile {
	p.colorLevel = normalizeTerminalColorLevel(level)
	return p
}

// WithNoColorPreference returns a copy with an explicit no-color preference
func (p TerminalCapabilityProfile) WithNoColorPreference(prefer bool) TerminalCapabilityProfile {
	p.prefersNoColor = prefer
	return p
}

// WithHyperlinks returns a copy with explicit hyperlink evidence
func (p TerminalCapabilityProfile) WithHyperlinks(support TerminalFeatureSupport) TerminalCapabilityProfile {
	p.hyperlinks = normalizeTerminalFeatureSupport(support)
	return p
}

// WithClipboard returns a copy with explicit terminal clipboard evidence
func (p TerminalCapabilityProfile) WithClipboard(support TerminalFeatureSupport) TerminalCapabilityProfile {
	p.clipboard = normalizeTerminalFeatureSupport(support)
	return p
}

// WithExtendedKeyboard returns a copy with keyboard evidence and active mode
func (p TerminalCapabilityProfile) WithExtendedKeyboard(
	support TerminalFeatureSupport,
	protocol TerminalKeyboardProtocol,
) TerminalCapabilityProfile {
	p.extendedKeyboard = normalizeTerminalFeatureSupport(support)
	p.keyboardProtocol = normalizeTerminalKeyboardProtocol(protocol)
	return p
}

func normalizeTerminalColorLevel(level TerminalColorLevel) TerminalColorLevel {
	if level > TerminalColorTrueColor {
		return TerminalColorUnknown
	}
	return level
}

func normalizeTerminalFeatureSupport(support TerminalFeatureSupport) TerminalFeatureSupport {
	if support > TerminalFeatureSupported {
		return TerminalFeatureUnknown
	}
	return support
}

func normalizeTerminalKeyboardProtocol(protocol TerminalKeyboardProtocol) TerminalKeyboardProtocol {
	if protocol > TerminalKeyboardKitty {
		return TerminalKeyboardLegacy
	}
	return protocol
}

func processEnvironmentProfile() TerminalCapabilityProfile {
	term, _ := os.LookupEnv("TERM")
	colorTerm, _ := os.LookupEnv("COLORTERM")
	noColor, hasNoColor := os.LookupEnv("NO_COLOR")
	return environmentProfile(term, colorTerm, hasNoColor && noColor != "")
}

func environmentProfile(term, colorTerm string, prefersNoColor bool) TerminalCapabilityProfile {
	lowerTerm := strings.ToLower(term)
	lowerColorTerm := strings.ToLower(colorTerm)
	profile := TerminalCapabilityProfile{prefersNoColor: prefersNoColor}
	switch {
	case lowerTerm == "dumb":
		profile.colorLevel = TerminalColorMonochrome
		profile.hyperlinks = TerminalFeatureUnsupported
		profile.clipboard = TerminalFeatureUnsupported
	case lowerColorTerm == "truecolor" || lowerColorTerm == "24bit":
		profile.colorLevel = TerminalColorTrueColor
	case strings.Contains(lowerTerm, "256color"):
		profile.colorLevel = TerminalColorIndexed256
	case term != "":
		profile.colorLevel = TerminalColorANSI16
	}
	return profile
}
