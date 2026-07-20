package tui

import (
	"context"
	"errors"
	"testing"

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
