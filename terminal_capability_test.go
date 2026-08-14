package tui

import (
	"errors"
	"fmt"
	"testing"

	"github.com/mayahiro/nagitui-go/internal/conformance"
)

type capabilityContextApp struct {
	seen TerminalCapabilityProfile
}

func (*capabilityContextApp) Init() Effect[struct{}]           { return NoneEffect[struct{}]() }
func (*capabilityContextApp) Update(struct{}) Effect[struct{}] { return NoneEffect[struct{}]() }
func (*capabilityContextApp) Subscriptions() Subscription[struct{}] {
	return NoneSubscription[struct{}]()
}
func (app *capabilityContextApp) View(context ViewContext) Node[struct{}] {
	app.seen = context.TerminalCapabilities
	return Text[struct{}]("profile")
}

func TestEnvironmentCapabilityHintsKeepNoColorSeparate(t *testing.T) {
	trueColor := environmentProfile("xterm-256color", "truecolor", true)
	if trueColor.ColorLevel() != TerminalColorTrueColor || !trueColor.PrefersNoColor() {
		t.Fatalf("true-color profile = %#v", trueColor)
	}
	if trueColor.Hyperlinks() != TerminalFeatureUnknown || trueColor.Clipboard() != TerminalFeatureUnknown {
		t.Fatalf("unqueried feature support = %#v", trueColor)
	}

	dumb := environmentProfile("dumb", "", false)
	if dumb.ColorLevel() != TerminalColorMonochrome ||
		dumb.Hyperlinks() != TerminalFeatureUnsupported ||
		dumb.Clipboard() != TerminalFeatureUnsupported {
		t.Fatalf("dumb profile = %#v", dumb)
	}
}

func TestEnvironmentCapabilityProfilesMatchSharedFixtures(t *testing.T) {
	records, err := conformance.Load(
		"tui/terminal-capabilities.txt",
		"terminal-capabilities",
		"term", "colorterm", "no-color", "expected",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		term := record.Field("term")
		if term == "-" {
			term = ""
		}
		colorTerm := record.Field("colorterm")
		if colorTerm == "-" {
			colorTerm = ""
		}
		profile := environmentProfile(term, colorTerm, record.Field("no-color") != "-")
		if got := canonicalCapabilityProfile(profile); got != record.Field("expected") {
			t.Errorf("case %s: profile = %s, want %s", record.ID, got, record.Field("expected"))
		}
	}
}

func canonicalCapabilityProfile(profile TerminalCapabilityProfile) string {
	colors := [...]string{"unknown", "monochrome", "ansi16", "indexed256", "truecolor"}
	features := [...]string{"unknown", "unsupported", "supported"}
	return fmt.Sprintf(
		"%s,%t,%s,%s",
		colors[profile.ColorLevel()],
		profile.PrefersNoColor(),
		features[profile.Hyperlinks()],
		features[profile.Clipboard()],
	)
}

func TestKeyboardProfileMapsBindingSupportWithoutGrantingClipboard(t *testing.T) {
	profile := (TerminalCapabilityProfile{}).WithExtendedKeyboard(
		TerminalFeatureSupported,
		TerminalKeyboardKitty,
	)
	if profile.ModifiedKeySupport() != BindingSupported ||
		profile.KeyboardProtocol() != TerminalKeyboardKitty {
		t.Fatalf("keyboard profile = %#v", profile)
	}
	if profile.Clipboard() != TerminalFeatureUnknown {
		t.Fatalf("Clipboard = %d, want unknown", profile.Clipboard())
	}
}

func TestCapabilityProfileNormalizesUnknownGoEnumValues(t *testing.T) {
	profile := (TerminalCapabilityProfile{}).
		WithColorLevel(TerminalColorLevel(255)).
		WithHyperlinks(TerminalFeatureSupport(255)).
		WithClipboard(TerminalFeatureSupport(255)).
		WithExtendedKeyboard(TerminalFeatureSupport(255), TerminalKeyboardProtocol(255))
	if profile.ColorLevel() != TerminalColorUnknown ||
		profile.Hyperlinks() != TerminalFeatureUnknown ||
		profile.Clipboard() != TerminalFeatureUnknown ||
		profile.ExtendedKeyboard() != TerminalFeatureUnknown ||
		profile.KeyboardProtocol() != TerminalKeyboardLegacy {
		t.Fatalf("normalized profile = %#v", profile)
	}
}

func TestRuntimeConfigExposesDeterministicCapabilityProfile(t *testing.T) {
	profile := (TerminalCapabilityProfile{}).WithExtendedKeyboard(
		TerminalFeatureSupported,
		TerminalKeyboardKitty,
	)
	config := NewRuntimeConfig(Size{Width: 8, Height: 1})
	config.TerminalCapabilities = profile
	app := &capabilityContextApp{}
	runtime, err := NewRuntimeWithClock[struct{}](app, config, NewVirtualClock())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if app.seen != profile {
		t.Fatalf("ViewContext profile = %#v, want %#v", app.seen, profile)
	}
}
