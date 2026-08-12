package widget

import (
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	tui "github.com/mayahiro/nagitui-go"
)

type diffViewRuntimeMessage struct {
	state *DiffViewState
	copy  *DiffCopyRequest
}

type diffViewRuntimeApp struct {
	layout   DiffLayout
	state    DiffViewState
	keyMap   tui.KeyMap
	messages []diffViewRuntimeMessage
}

func (*diffViewRuntimeApp) Init() tui.Effect[diffViewRuntimeMessage] {
	return tui.NoneEffect[diffViewRuntimeMessage]()
}

func (a *diffViewRuntimeApp) Update(message diffViewRuntimeMessage) tui.Effect[diffViewRuntimeMessage] {
	if message.state != nil {
		a.state = *message.state
	}
	a.messages = append(a.messages, message)
	return tui.NoneEffect[diffViewRuntimeMessage]()
}

func (*diffViewRuntimeApp) Subscriptions() tui.Subscription[diffViewRuntimeMessage] {
	return tui.NoneSubscription[diffViewRuntimeMessage]()
}

func (a *diffViewRuntimeApp) View(tui.ViewContext) tui.Node[diffViewRuntimeMessage] {
	view := NewDiffView(
		"diff", a.layout, a.state,
		func(state DiffViewState) diffViewRuntimeMessage {
			return diffViewRuntimeMessage{state: &state}
		},
	).Viewport(4).OnCopy(func(request DiffCopyRequest) diffViewRuntimeMessage {
		return diffViewRuntimeMessage{copy: &request}
	}).Node()
	return tui.Column(view).WithKeyScope(tui.NewKeyScope("scope", a.keyMap))
}

func TestDiffViewRoutesActionsRendersAndCopiesUnifiedSource(t *testing.T) {
	oldRange := mustDiffRange(t, 1, 2)
	newRange := mustDiffRange(t, 1, 2)
	context, _ := NewDiffContextLine(1, 1, mustDiffCodeLine(t, "same"))
	deletion, _ := NewDiffDeletionLine(2, mustDiffCodeLine(t, "old"))
	addition, _ := NewDiffAdditionLine(2, mustDiffCodeLine(t, "new"))
	document, err := NewDiffDocument([]DiffLine{
		NewDiffMetadataLine(mustDiffCodeLine(t, "diff --git a/a b/a")),
		NewDiffHunkLine(NewDiffHunk(oldRange, newRange), mustDiffCodeLine(t, "@@ -1,2 +1,2 @@")),
		context, deletion, addition,
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := NewDiffLayout(document, DefaultDiffLayoutOptions().WithViewportWidth(16))
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
	app := &diffViewRuntimeApp{layout: layout, keyMap: keyMap}
	runtime, err := tui.NewRuntimeWithClock(
		app, tui.NewRuntimeConfig(tui.Size{Width: 16, Height: 4}), tui.NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	frame, err := runtime.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	if actual := codeViewSurfaceRow(frame.Surface(), 2); actual != "1 1   same      " {
		t.Fatalf("context row = %q", actual)
	}
	if actual := codeViewSurfaceRow(frame.Surface(), 3); actual != "2   - old       " {
		t.Fatalf("deletion row = %q", actual)
	}
	if focused, err := runtime.RequestFocus("diff"); err != nil || !focused {
		t.Fatalf("focus = %t, %v", focused, err)
	}
	for range 3 {
		diffViewRuntimeKey(t, runtime, 'j', vt.Modifiers{}, vt.KeyPress, 1)
	}
	if app.state.Cursor() != 3 {
		t.Fatalf("cursor = %d", app.state.Cursor())
	}
	diffViewRuntimeKey(t, runtime, 'c', vt.Modifiers{Control: true}, vt.KeyPress, 1)
	request := app.messages[len(app.messages)-1].copy
	if request == nil || request.Text != "-old" || request.LineStart != 3 || request.LineEnd != 4 {
		t.Fatalf("copy request = %#v", request)
	}
	diffViewRuntimeKey(t, runtime, 'c', vt.Modifiers{Control: true}, vt.KeyRepeat, 0)
	diffViewRuntimeEvent(t, runtime, vt.Event{Kind: vt.EventMouse, Mouse: vt.MouseEvent{
		Kind: vt.MousePress, Button: vt.MouseLeft, X: 0, Y: 0,
	}}, 1)
	if app.state.Cursor() != 1 {
		t.Fatalf("pointer cursor = %d", app.state.Cursor())
	}
}

func diffViewRuntimeKey(
	t *testing.T,
	runtime *tui.Runtime[diffViewRuntimeMessage],
	character rune,
	modifiers vt.Modifiers,
	action vt.KeyAction,
	expectedMessages int,
) {
	t.Helper()
	diffViewRuntimeEvent(t, runtime, vt.Event{Kind: vt.EventKey, Key: vt.KeyEvent{
		Code: vt.KeyCharacter, Character: character, Modifiers: modifiers,
		Action: action, Protocol: vt.KeyProtocolLegacy,
	}}, expectedMessages)
}

func diffViewRuntimeEvent(
	t *testing.T,
	runtime *tui.Runtime[diffViewRuntimeMessage],
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
		t.Fatalf("dispatch = consumed %t messages %d", dispatch.Consumed(), dispatch.Messages())
	}
	if count, err := runtime.ProcessPending(); err != nil || count != expectedMessages {
		t.Fatalf("processed = %d, %v", count, err)
	}
}
