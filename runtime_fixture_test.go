package tui

import (
	"bytes"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go/internal/conformance"
)

type fixtureEcho struct {
	text string
}

func (*fixtureEcho) Init() Effect[string] {
	return NoneEffect[string]()
}

func (a *fixtureEcho) Update(message string) Effect[string] {
	a.text += message
	return NoneEffect[string]()
}

func (*fixtureEcho) Subscriptions() Subscription[string] {
	return NoneSubscription[string]()
}

func (a *fixtureEcho) View(_ ViewContext) Node[string] {
	return Border(Text[string](a.text), vt.Style{})
}

func TestRuntimeRoundtripFixtures(t *testing.T) {
	records, err := conformance.Load(
		"runtime/roundtrip.txt",
		"runtime-roundtrip",
		"width",
		"height",
		"input",
		"expected",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			width := runtimeFixtureNumber(t, record.Field("width"))
			height := runtimeFixtureNumber(t, record.Field("height"))
			input := record.Bytes("input")
			runtime, err := NewRuntimeWithClock[string](
				&fixtureEcho{},
				NewRuntimeConfig(Size{Width: width, Height: height}),
				NewVirtualClock(),
			)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runtime.RenderIfDirty(); err != nil {
				t.Fatal(err)
			}
			decoder := NewTimedInputDecoder(NewVirtualClock(), 25*time.Millisecond)
			for _, event := range decoder.Feed(input) {
				if event.Kind == vt.EventText {
					if err := runtime.Enqueue(event.Text); err != nil {
						t.Fatal(err)
					}
				}
			}
			frame, err := runtime.Step()
			if err != nil {
				t.Fatal(err)
			}
			if actual, expected := frame.Surface().Snapshot(), record.Text("expected"); actual != expected {
				t.Fatalf("snapshot mismatch\ngot:\n%s\nwant:\n%s", actual, expected)
			}
			output := vt.Encode(frame.Operations(), vt.BaselineCapabilities())
			if !bytes.Contains(output, input) {
				t.Fatalf("input %q did not reach VT output %q", input, output)
			}
		})
	}
}

type fixtureTextInput struct {
	value string
}

func (*fixtureTextInput) Init() Effect[string] { return NoneEffect[string]() }
func (*fixtureTextInput) Subscriptions() Subscription[string] {
	return NoneSubscription[string]()
}
func (a *fixtureTextInput) Update(value string) Effect[string] {
	a.value = value
	return NoneEffect[string]()
}
func (a *fixtureTextInput) View(_ ViewContext) Node[string] {
	return TextInput("input", a.value, func(value string) string { return value })
}

func TestTextInputRuntimeFixtures(t *testing.T) {
	records, err := conformance.Load(
		"interaction/text-input-runtime.txt",
		"text-input-runtime",
		"width",
		"height",
		"input",
		"expected",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			runtime, err := NewRuntimeWithClock[string](
				&fixtureTextInput{},
				NewRuntimeConfig(Size{
					Width:  runtimeFixtureNumber(t, record.Field("width")),
					Height: runtimeFixtureNumber(t, record.Field("height")),
				}),
				NewVirtualClock(),
			)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runtime.RenderIfDirty(); err != nil {
				t.Fatal(err)
			}
			if focused, err := runtime.RequestFocus("input"); err != nil || !focused {
				t.Fatalf("RequestFocus = %t, %v", focused, err)
			}
			decoder := NewTimedInputDecoder(NewVirtualClock(), 25*time.Millisecond)
			for _, event := range decoder.Feed(record.Bytes("input")) {
				if _, err := runtime.DispatchEvent(event); err != nil {
					t.Fatal(err)
				}
			}
			frame, err := runtime.Step()
			if err != nil {
				t.Fatal(err)
			}
			if actual, expected := frame.Surface().Snapshot(), record.Text("expected"); actual != expected {
				t.Fatalf("snapshot mismatch\ngot:\n%s\nwant:\n%s", actual, expected)
			}
		})
	}
}

func runtimeFixtureNumber(t *testing.T, value string) uint32 {
	t.Helper()
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		t.Fatalf("parse %q: %v", value, err)
	}
	return uint32(parsed)
}
