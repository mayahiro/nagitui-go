package widget

import (
	"strings"
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	tui "github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/surface"
)

type codeViewRuntimeMessage struct {
	state *CodeViewState
	copy  *CodeCopyRequest
}

type codeViewRuntimeApp struct {
	layout   CodeLayout
	state    CodeViewState
	keyMap   tui.KeyMap
	messages []codeViewRuntimeMessage
}

func (*codeViewRuntimeApp) Init() tui.Effect[codeViewRuntimeMessage] {
	return tui.NoneEffect[codeViewRuntimeMessage]()
}

func (a *codeViewRuntimeApp) Update(message codeViewRuntimeMessage) tui.Effect[codeViewRuntimeMessage] {
	if message.state != nil {
		a.state = *message.state
	}
	a.messages = append(a.messages, message)
	return tui.NoneEffect[codeViewRuntimeMessage]()
}

func (*codeViewRuntimeApp) Subscriptions() tui.Subscription[codeViewRuntimeMessage] {
	return tui.NoneSubscription[codeViewRuntimeMessage]()
}

func (a *codeViewRuntimeApp) View(tui.ViewContext) tui.Node[codeViewRuntimeMessage] {
	view := NewCodeView(
		"code", a.layout, a.state,
		func(state CodeViewState) codeViewRuntimeMessage {
			return codeViewRuntimeMessage{state: &state}
		},
	).Viewport(3).OnCopy(func(request CodeCopyRequest) codeViewRuntimeMessage {
		return codeViewRuntimeMessage{copy: &request}
	}).Node()
	return tui.Column(view).WithKeyScope(tui.NewKeyScope("scope", a.keyMap))
}

func TestCodeViewRoutesActionsRendersAndCopiesCompleteSource(t *testing.T) {
	lines := make([]CodeLine, 3)
	for index, text := range []string{"a\t日", "abcdefghij", "last"} {
		lines[index] = mustCodeLine(t, text)
	}
	document, err := NewCodeDocument(lines, true)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := NewCodeLayout(
		document, DefaultCodeLayoutOptions().WithViewportWidth(12),
	)
	if err != nil {
		t.Fatal(err)
	}
	keyMap, err := tui.NewKeyMap().Rebind(
		SelectionNextActionID,
		[]tui.KeyBinding{tui.NewKeyBinding(tui.NewCharacterKeyStroke('j', vt.Modifiers{}))},
	)
	if err != nil {
		t.Fatal(err)
	}
	app := &codeViewRuntimeApp{layout: layout, keyMap: keyMap}
	runtime, err := tui.NewRuntimeWithClock(
		app, tui.NewRuntimeConfig(tui.Size{Width: 12, Height: 3}), tui.NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	frame, err := runtime.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(codeViewSurfaceRow(frame.Surface(), 0), "1 | a   日") {
		t.Fatalf("first row = %q", codeViewSurfaceRow(frame.Surface(), 0))
	}
	if actual := codeViewSurfaceRow(frame.Surface(), 1); actual != "2 | abcdefgh" {
		t.Fatalf("second row = %q", actual)
	}
	if focused, err := runtime.RequestFocus("code"); err != nil || !focused {
		t.Fatalf("focus = %t, %v", focused, err)
	}

	codeViewRuntimeKey(t, runtime, vt.KeyCharacter, 'j', vt.Modifiers{}, vt.KeyPress, 1)
	codeViewRuntimeKey(t, runtime, vt.KeyCharacter, 'j', vt.Modifiers{}, vt.KeyPress, 1)
	if app.state.Cursor() != 2 {
		t.Fatalf("cursor = %d", app.state.Cursor())
	}
	codeViewRuntimeKey(
		t, runtime, vt.KeyCharacter, 'c', vt.Modifiers{Control: true}, vt.KeyPress, 1,
	)
	request := app.messages[len(app.messages)-1].copy
	if request == nil || request.Text != "last\n" || request.LineStart != 2 || request.LineEnd != 3 {
		t.Fatalf("copy request = %#v", request)
	}
	codeViewRuntimeKey(
		t, runtime, vt.KeyCharacter, 'c', vt.Modifiers{Control: true}, vt.KeyRepeat, 0,
	)
	codeViewRuntimeEvent(t, runtime, vt.Event{Kind: vt.EventMouse, Mouse: vt.MouseEvent{
		Kind: vt.MousePress, Button: vt.MouseLeft, X: 0, Y: 0,
	}}, 1)
	if app.state.Cursor() != 0 {
		t.Fatalf("pointer cursor = %d", app.state.Cursor())
	}
}

func codeViewRuntimeKey(
	t *testing.T,
	runtime *tui.Runtime[codeViewRuntimeMessage],
	code vt.KeyCode,
	character rune,
	modifiers vt.Modifiers,
	action vt.KeyAction,
	expectedMessages int,
) {
	t.Helper()
	codeViewRuntimeEvent(t, runtime, vt.Event{Kind: vt.EventKey, Key: vt.KeyEvent{
		Code: code, Character: character, Modifiers: modifiers, Action: action,
		Protocol: vt.KeyProtocolLegacy,
	}}, expectedMessages)
}

func codeViewRuntimeEvent(
	t *testing.T,
	runtime *tui.Runtime[codeViewRuntimeMessage],
	event vt.Event,
	expectedMessages int,
) {
	t.Helper()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	dispatch, err := runtime.DispatchEvent(event)
	if err != nil {
		t.Fatal(err)
	}
	if !dispatch.Consumed() || dispatch.Messages() != expectedMessages {
		t.Fatalf("dispatch = consumed %t, messages %d", dispatch.Consumed(), dispatch.Messages())
	}
	if count, err := runtime.ProcessPending(); err != nil || count != expectedMessages {
		t.Fatalf("processed = %d, %v", count, err)
	}
}

func codeViewSurfaceRow(rendered *surface.Surface, row int32) string {
	var text strings.Builder
	for column := uint32(0); column < rendered.Width(); column++ {
		cell, _ := rendered.Cell(int32(column), row)
		text.WriteString(cell.Content())
	}
	return text.String()
}
