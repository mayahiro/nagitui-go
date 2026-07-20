package tui

import (
	"bytes"
	"testing"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go/surface"
)

func TestFirstFrameWritesCompleteRowsAndCursor(t *testing.T) {
	current, err := surface.New(3, 1)
	if err != nil {
		t.Fatal(err)
	}
	current.Write(0, 0, "A日", vt.Style{Bold: true}, celltext.ModernWidth())
	current.SetCursor(surface.Cursor{X: 2})

	encoded := vt.Encode(rendererOperations(nil, current), vt.ModernCapabilities())

	if !bytes.Contains(encoded, []byte("A日")) {
		t.Fatalf("encoded frame does not contain rendered text: %q", encoded)
	}
	if !bytes.HasPrefix(encoded, []byte("\x1B[?2026h")) {
		t.Fatalf("encoded frame does not begin synchronized update: %q", encoded)
	}
	if !bytes.HasSuffix(encoded, []byte("\x1B[?2026l")) {
		t.Fatalf("encoded frame does not end synchronized update: %q", encoded)
	}
}

func TestUnchangedSecondFrameHasNoTextAndBaselineOmitsSync(t *testing.T) {
	current, err := surface.New(2, 1)
	if err != nil {
		t.Fatal(err)
	}

	encoded := vt.Encode(rendererOperations(current, current), vt.BaselineCapabilities())

	if bytes.Contains(encoded, []byte("?2026")) {
		t.Fatalf("baseline frame contains synchronized update sequence: %q", encoded)
	}
	if bytes.Contains(encoded, []byte("  ")) {
		t.Fatalf("unchanged frame contains cell text: %q", encoded)
	}
}
