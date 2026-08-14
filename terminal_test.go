package tui

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mayahiro/nagi-go/vt"
)

func TestDefaultTerminalOptionsPreserveUnfocusedNonMouseBehavior(t *testing.T) {
	options := DefaultTerminalOptions()
	if options.MouseTracking != nil {
		t.Fatalf("MouseTracking = %v, want nil", options.MouseTracking)
	}
	if options.Clipboard != TerminalClipboardDisabled {
		t.Fatalf("Clipboard = %d, want disabled", options.Clipboard)
	}
	if options.CapabilityDetection != TerminalCapabilityDetectionDisabled {
		t.Fatalf("CapabilityDetection = %d, want disabled", options.CapabilityDetection)
	}
	if options.CapabilityQueryTimeout != 100*time.Millisecond {
		t.Fatalf("CapabilityQueryTimeout = %s, want 100ms", options.CapabilityQueryTimeout)
	}
	if _, inline := options.Viewport.InlineHeight(); inline {
		t.Fatal("Viewport is inline, want fullscreen")
	}
	if options.CursorQueryTimeout != 100*time.Millisecond {
		t.Fatalf("CursorQueryTimeout = %s, want 100ms", options.CursorQueryTimeout)
	}
	if options.FocusFirst {
		t.Fatal("FocusFirst = true, want false")
	}
	if options.MaxUpdatesPerCycle != DefaultMaxUpdatesPerCycle {
		t.Fatalf("MaxUpdatesPerCycle = %d, want %d", options.MaxUpdatesPerCycle, DefaultMaxUpdatesPerCycle)
	}
	if options.MinimumFrameInterval != (time.Second+119)/120 {
		t.Fatalf("MinimumFrameInterval = %s, want 120 FPS cap", options.MinimumFrameInterval)
	}
}

func TestDetectedColorLevelBoundsExplicitEncoderLevel(t *testing.T) {
	options := DefaultTerminalOptions()
	options.Capabilities = vt.ModernCapabilities()
	options.CapabilityDetection = TerminalCapabilityDetectionEnabled

	indexed := (TerminalCapabilityProfile{}).WithColorLevel(TerminalColorIndexed256)
	if got := resolvedOutputCapabilities(options, indexed).ColorLevel; got != vt.ColorIndexed256 {
		t.Fatalf("indexed ColorLevel = %d", got)
	}
	ansi := (TerminalCapabilityProfile{}).WithColorLevel(TerminalColorANSI16)
	if got := resolvedOutputCapabilities(options, ansi).ColorLevel; got != vt.ColorANSI16 {
		t.Fatalf("ANSI ColorLevel = %d", got)
	}
	monochrome := (TerminalCapabilityProfile{}).WithColorLevel(TerminalColorMonochrome)
	if got := resolvedOutputCapabilities(options, monochrome).ColorLevel; got != vt.ColorMonochrome {
		t.Fatalf("monochrome ColorLevel = %d", got)
	}
	trueColor := (TerminalCapabilityProfile{}).WithColorLevel(TerminalColorTrueColor)
	if got := resolvedOutputCapabilities(options, trueColor).ColorLevel; got != vt.ColorTrueColor {
		t.Fatalf("true ColorLevel = %d", got)
	}
	options.Capabilities = vt.BaselineCapabilities()
	if got := resolvedOutputCapabilities(options, trueColor).ColorLevel; got != vt.ColorIndexed256 {
		t.Fatalf("baseline ColorLevel = %d", got)
	}
}

func TestInlineTerminalViewportRequiresPositiveHeight(t *testing.T) {
	if _, err := NewInlineTerminalViewport(0); !errors.Is(err, ErrZeroInlineViewportHeight) {
		t.Fatalf("error = %v, want ErrZeroInlineViewportHeight", err)
	}
	viewport, err := NewInlineTerminalViewport(4)
	if err != nil {
		t.Fatal(err)
	}
	if height, ok := viewport.InlineHeight(); !ok || height != 4 {
		t.Fatalf("InlineHeight = %d, %t, want 4, true", height, ok)
	}
}

func TestRunTerminalContextReturnsPreexistingCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := RunTerminalContext[runtimeMessage](ctx, &counterApp{}, DefaultTerminalOptions(), func(vt.Event) EventAction[runtimeMessage] {
		return IgnoreAction[runtimeMessage]()
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

type clipboardTerminalApp struct{}

func (*clipboardTerminalApp) Init() Effect[string] {
	return NoneEffect[string]()
}

func (*clipboardTerminalApp) Update(message string) Effect[string] {
	return SetClipboardEffect[string](message).WithoutRedraw()
}

func (*clipboardTerminalApp) Subscriptions() Subscription[string] {
	return NoneSubscription[string]()
}

func (*clipboardTerminalApp) View(ViewContext) Node[string] {
	return Text[string]("view")
}

func TestClipboardOutputIsExplicitAndDoesNotRequireAFrame(t *testing.T) {
	runtime, err := NewRuntimeWithClock(
		&clipboardTerminalApp{},
		NewRuntimeConfig(Size{Width: 8, Height: 1}),
		NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if _, err := pendingTerminalOutputOperations(runtime, TerminalClipboardDisabled); err != nil {
		t.Fatal(err)
	}

	if err := runtime.Enqueue("copy"); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ProcessPending(); err != nil {
		t.Fatal(err)
	}
	operations, err := pendingTerminalOutputOperations(runtime, TerminalClipboardDisabled)
	if err != nil {
		t.Fatal(err)
	}
	if len(operations) != 0 {
		t.Fatalf("disabled clipboard operations = %d, want 0", len(operations))
	}

	if err := runtime.Enqueue("copy"); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ProcessPending(); err != nil {
		t.Fatal(err)
	}
	operations, err = pendingTerminalOutputOperations(runtime, TerminalClipboardOSC52)
	if err != nil {
		t.Fatal(err)
	}
	got := string(vt.Encode(operations, vt.BaselineCapabilities()))
	want := "\x1B]52;c;Y29weQ==\x1B\\"
	if got != want {
		t.Fatalf("clipboard output = %q, want %q", got, want)
	}
}

func TestClipboardOutputFollowsFrameInOneOperationBatch(t *testing.T) {
	runtime, err := NewRuntimeWithClock(
		&clipboardTerminalApp{},
		NewRuntimeConfig(Size{Width: 8, Height: 1}),
		NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if err := runtime.Enqueue("copy"); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ProcessPending(); err != nil {
		t.Fatal(err)
	}

	operations, err := pendingTerminalOutputOperations(runtime, TerminalClipboardOSC52)
	if err != nil {
		t.Fatal(err)
	}
	output := vt.Encode(operations, vt.BaselineCapabilities())
	clipboard := []byte("\x1B]52;c;Y29weQ==\x1B\\")
	if len(output) <= len(clipboard) || !bytes.HasSuffix(output, clipboard) {
		t.Fatalf("combined output = %q, want frame followed by clipboard", output)
	}
}

func TestRunTerminalRejectsInvalidClipboardModeBeforeOpeningTerminal(t *testing.T) {
	options := DefaultTerminalOptions()
	options.Clipboard = TerminalClipboard(255)
	err := RunTerminalContext[string](
		context.Background(),
		&clipboardTerminalApp{},
		options,
		func(vt.Event) EventAction[string] { return IgnoreAction[string]() },
	)
	if err == nil || err.Error() != "nagi-tui: invalid terminal clipboard mode 255" {
		t.Fatalf("error = %v, want invalid clipboard mode", err)
	}
}

func TestRunTerminalRejectsInvalidCapabilityOptionsBeforeOpeningTerminal(t *testing.T) {
	options := DefaultTerminalOptions()
	options.CapabilityDetection = TerminalCapabilityDetection(255)
	err := RunTerminalContext[string](
		context.Background(),
		&clipboardTerminalApp{},
		options,
		func(vt.Event) EventAction[string] { return IgnoreAction[string]() },
	)
	if err == nil || err.Error() != "nagi-tui: invalid terminal capability detection mode 255" {
		t.Fatalf("invalid mode error = %v", err)
	}

	options = DefaultTerminalOptions()
	options.CapabilityQueryTimeout = -time.Nanosecond
	err = RunTerminalContext[string](
		context.Background(),
		&clipboardTerminalApp{},
		options,
		func(vt.Event) EventAction[string] { return IgnoreAction[string]() },
	)
	if err == nil || err.Error() != "nagi-tui: terminal capability query timeout must not be negative" {
		t.Fatalf("negative timeout error = %v", err)
	}
}

type testTerminalDeadline struct {
	duration time.Duration
	ok       bool
}

func (deadline testTerminalDeadline) TimeUntilDeadline() (time.Duration, bool) {
	return deadline.duration, deadline.ok
}

type testRuntimeDeadlines struct {
	pending      bool
	effect       testTerminalDeadline
	subscription testTerminalDeadline
	frame        testTerminalDeadline
}

func (deadlines testRuntimeDeadlines) HasPendingUpdates() bool {
	return deadlines.pending
}

func (deadlines testRuntimeDeadlines) TimeUntilEffectDeadline() (time.Duration, bool) {
	return deadlines.effect.TimeUntilDeadline()
}

func (deadlines testRuntimeDeadlines) TimeUntilSubscriptionDeadline() (time.Duration, bool) {
	return deadlines.subscription.TimeUntilDeadline()
}

func (deadlines testRuntimeDeadlines) TimeUntilFrameDeadline() (time.Duration, bool) {
	return deadlines.frame.TimeUntilDeadline()
}

func TestTerminalWaitUsesNoTimeoutWithoutDeadline(t *testing.T) {
	if timeout, ok := terminalWaitDuration(testTerminalDeadline{}, testRuntimeDeadlines{}); ok || timeout != 0 {
		t.Fatalf("terminalWaitDuration = %s, %t, want indefinite wait", timeout, ok)
	}

	timeout, ok := terminalWaitDuration(
		testTerminalDeadline{duration: 25 * time.Millisecond, ok: true},
		testRuntimeDeadlines{
			effect:       testTerminalDeadline{duration: time.Second, ok: true},
			subscription: testTerminalDeadline{duration: 8 * time.Millisecond, ok: true},
		},
	)
	if !ok || timeout != 8*time.Millisecond {
		t.Fatalf("terminalWaitDuration = %s, %t, want 8ms deadline", timeout, ok)
	}

	timeout, ok = terminalWaitDuration(
		testTerminalDeadline{duration: 25 * time.Millisecond, ok: true},
		testRuntimeDeadlines{pending: true},
	)
	if !ok || timeout != 0 {
		t.Fatalf("terminalWaitDuration = %s, %t, want immediate pending-update cycle", timeout, ok)
	}
}
