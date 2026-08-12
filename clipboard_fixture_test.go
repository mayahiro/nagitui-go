package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/mayahiro/nagitui-go/internal/conformance"
)

type clipboardFixtureMessage struct {
	composition string
	values      []string
}

type clipboardFixtureApp struct{}

func (*clipboardFixtureApp) Init() Effect[clipboardFixtureMessage] {
	return NoneEffect[clipboardFixtureMessage]()
}

func (*clipboardFixtureApp) Update(message clipboardFixtureMessage) Effect[clipboardFixtureMessage] {
	effects := make([]Effect[clipboardFixtureMessage], len(message.values))
	for index, value := range message.values {
		effects[index] = SetClipboardEffect[clipboardFixtureMessage](value)
	}
	var effect Effect[clipboardFixtureMessage]
	switch message.composition {
	case "single", "updates":
		effect = effects[0]
	case "batch":
		effect = BatchEffects(effects...)
	case "sequence":
		effect = SequenceEffects(effects...)
	default:
		panic("invalid clipboard fixture composition")
	}
	return effect.WithoutRedraw()
}

func (*clipboardFixtureApp) Subscriptions() Subscription[clipboardFixtureMessage] {
	return NoneSubscription[clipboardFixtureMessage]()
}

func (*clipboardFixtureApp) View(ViewContext) Node[clipboardFixtureMessage] {
	return Text[clipboardFixtureMessage]("clipboard")
}

func TestClipboardEffectMatchesSharedFixtures(t *testing.T) {
	records, err := conformance.Load(
		"effects/clipboard.txt",
		"effect-clipboard",
		"composition",
		"values",
		"expected",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}

	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			values := strings.Split(record.Text("values"), ",")
			runtime, err := NewRuntimeWithClock(
				&clipboardFixtureApp{},
				NewRuntimeConfig(Size{Width: 12, Height: 1}),
				NewVirtualClock(),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer runtime.Close()
			if _, err := runtime.RenderIfDirty(); err != nil {
				t.Fatal(err)
			}

			if record.Field("composition") == "updates" {
				for _, value := range values {
					if err := runtime.Enqueue(clipboardFixtureMessage{
						composition: "single", values: []string{value},
					}); err != nil {
						t.Fatal(err)
					}
				}
			} else if err := runtime.Enqueue(clipboardFixtureMessage{
				composition: record.Field("composition"), values: values,
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := runtime.ProcessPending(); err != nil {
				t.Fatal(err)
			}

			frame, err := runtime.RenderIfDirty()
			if err != nil {
				t.Fatal(err)
			}
			if frame != nil {
				t.Fatal("clipboard effect dirtied the view")
			}
			request, ok := runtime.PendingClipboardRequest()
			if !ok || request.Text() != record.Text("expected") {
				t.Fatalf("pending request = %q, %t, want %q", request.Text(), ok, record.Text("expected"))
			}
			request, ok = runtime.TakeClipboardRequest()
			if !ok || request.Text() != record.Text("expected") {
				t.Fatalf("taken request = %q, %t, want %q", request.Text(), ok, record.Text("expected"))
			}
			if _, ok := runtime.TakeClipboardRequest(); ok {
				t.Fatal("second take returned a request")
			}
		})
	}
}

func TestClipboardRequestNormalizesInvalidUTF8(t *testing.T) {
	request := NewClipboardRequest("a\xFFb")
	if request.Text() != "a\uFFFDb" {
		t.Fatalf("Text = %q, want normalized UTF-8", request.Text())
	}
}
