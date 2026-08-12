package widget

import (
	"testing"

	celltext "github.com/mayahiro/nagi-go/text"
	tui "github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/surface"
)

type widthProfileMessage struct{}

type widthProfileTextAreaApp struct{}

func (*widthProfileTextAreaApp) Init() tui.Effect[widthProfileMessage] {
	return tui.NoneEffect[widthProfileMessage]()
}

func (*widthProfileTextAreaApp) Update(widthProfileMessage) tui.Effect[widthProfileMessage] {
	return tui.NoneEffect[widthProfileMessage]()
}

func (*widthProfileTextAreaApp) Subscriptions() tui.Subscription[widthProfileMessage] {
	return tui.NoneSubscription[widthProfileMessage]()
}

func (*widthProfileTextAreaApp) View(context tui.ViewContext) tui.Node[widthProfileMessage] {
	return NewTextArea(
		"input",
		NewTextAreaState("·X", len("·")),
		func(TextAreaState) widthProfileMessage { return widthProfileMessage{} },
	).
		WidthProfile(context.WidthProfile).
		SoftWrap(int(context.Size.Width)).
		Node()
}

func TestTextAreaUsesRuntimeCJKProfileForWrapAndCursor(t *testing.T) {
	config := tui.NewRuntimeConfig(tui.Size{Width: 2, Height: 2})
	config.WidthProfile = celltext.CJKWidth()
	runtime, err := tui.NewRuntimeWithClock[widthProfileMessage](
		&widthProfileTextAreaApp{},
		config,
		tui.NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if focused, err := runtime.RequestFocus("input"); err != nil || !focused {
		t.Fatalf("RequestFocus = %t, %v", focused, err)
	}
	frame, err := runtime.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	if frame == nil {
		t.Fatal("focus did not produce a frame")
	}
	if cursor, ok := frame.Surface().Cursor(); !ok || cursor != (surface.Cursor{X: 0, Y: 1}) {
		t.Fatalf("cursor = %+v, %t, want 0,1", cursor, ok)
	}
}

func TestChartMarkerFallbackRespectsWidthProfile(t *testing.T) {
	tests := []struct {
		name    string
		marker  string
		profile celltext.WidthProfile
		want    string
	}{
		{name: "empty modern", profile: celltext.ModernWidth(), want: "•"},
		{name: "empty cjk", profile: celltext.CJKWidth(), want: "*"},
		{name: "ambiguous cjk", marker: "·", profile: celltext.CJKWidth(), want: "*"},
		{name: "ascii cjk", marker: "x", profile: celltext.CJKWidth(), want: "x"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := chartMarker(test.marker, test.profile); got != test.want {
				t.Fatalf("chartMarker(%q) = %q, want %q", test.marker, got, test.want)
			}
		})
	}
}
