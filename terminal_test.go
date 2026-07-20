package tui

import (
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
	if options.FocusFirst {
		t.Fatal("FocusFirst = true, want false")
	}
	if options.MinimumFrameInterval != (time.Second+119)/120 {
		t.Fatalf("MinimumFrameInterval = %s, want 120 FPS cap", options.MinimumFrameInterval)
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

type testTerminalDeadline struct {
	duration time.Duration
	ok       bool
}

func (deadline testTerminalDeadline) TimeUntilDeadline() (time.Duration, bool) {
	return deadline.duration, deadline.ok
}

type testRuntimeDeadlines struct {
	effect       testTerminalDeadline
	subscription testTerminalDeadline
	frame        testTerminalDeadline
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
}
