package widget

import (
	"strings"
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/surface"
)

type jsonInspectorRuntimeMessage struct {
	state *JSONInspectorState
	copy  *JSONInspectorCopyRequest
}

type jsonInspectorRuntimeApp struct {
	document JSONDocument
	state    JSONInspectorState
	keyMap   tui.KeyMap
	messages []jsonInspectorRuntimeMessage
}

func (*jsonInspectorRuntimeApp) Init() tui.Effect[jsonInspectorRuntimeMessage] {
	return tui.NoneEffect[jsonInspectorRuntimeMessage]()
}

func (a *jsonInspectorRuntimeApp) Update(message jsonInspectorRuntimeMessage) tui.Effect[jsonInspectorRuntimeMessage] {
	if message.state != nil {
		a.state = *message.state
	}
	a.messages = append(a.messages, message)
	return tui.NoneEffect[jsonInspectorRuntimeMessage]()
}

func (*jsonInspectorRuntimeApp) Subscriptions() tui.Subscription[jsonInspectorRuntimeMessage] {
	return tui.NoneSubscription[jsonInspectorRuntimeMessage]()
}

func (a *jsonInspectorRuntimeApp) View(tui.ViewContext) tui.Node[jsonInspectorRuntimeMessage] {
	inspector := NewJSONInspector(
		"inspector",
		a.document,
		a.state,
		func(state JSONInspectorState) jsonInspectorRuntimeMessage {
			return jsonInspectorRuntimeMessage{state: &state}
		},
	).MaximumScalarGraphemes(5).OnCopy(func(request JSONInspectorCopyRequest) jsonInspectorRuntimeMessage {
		return jsonInspectorRuntimeMessage{copy: &request}
	}).Node()
	return tui.Column(inspector).WithKeyScope(tui.NewKeyScope("scope", a.keyMap))
}

func TestJSONInspectorRoutesActionsPointerAndCompleteCopy(t *testing.T) {
	keyMap, err := tui.NewKeyMap().Rebind(
		SelectionNextActionID,
		[]tui.KeyBinding{tui.NewKeyBinding(tui.NewCharacterKeyStroke('j', vt.Modifiers{}))},
	)
	if err != nil {
		t.Fatal(err)
	}
	app := &jsonInspectorRuntimeApp{
		document: jsonInspectorTestDocument(t), state: JSONInspectorState{}, keyMap: keyMap,
	}
	runtime, err := tui.NewRuntimeWithClock(
		app, tui.NewRuntimeConfig(tui.Size{Width: 60, Height: 10}), tui.NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	frame, err := runtime.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	if !jsonInspectorSurfaceRowContains(frame.Surface(), 2, "\"long\": \"abcde…\"") {
		t.Fatalf("long preview was not rendered on row 2")
	}
	if focused, err := runtime.RequestFocus("inspector"); err != nil || !focused {
		t.Fatalf("focus = %t, %v", focused, err)
	}

	jsonInspectorRuntimeKey(t, runtime, vt.KeyCharacter, 'j', vt.Modifiers{}, vt.KeyPress, 1)
	jsonInspectorRuntimeKey(t, runtime, vt.KeyCharacter, 'j', vt.Modifiers{}, vt.KeyPress, 1)
	if app.state.Selected().String() != "/long" {
		t.Fatalf("selected = %s", app.state.Selected())
	}
	jsonInspectorRuntimeKey(t, runtime, vt.KeyCharacter, 'c', vt.Modifiers{Control: true}, vt.KeyPress, 1)
	copyRequest := app.messages[len(app.messages)-1].copy
	if copyRequest == nil || copyRequest.Path.String() != "/long" || copyRequest.Text != "\"abcdef日ghi\"" {
		t.Fatalf("copy request = %#v", copyRequest)
	}
	jsonInspectorRuntimeKey(t, runtime, vt.KeyCharacter, 'c', vt.Modifiers{Control: true}, vt.KeyRepeat, 0)

	jsonInspectorRuntimeEvent(t, runtime, vt.Event{Kind: vt.EventMouse, Mouse: vt.MouseEvent{
		Kind: vt.MousePress, Button: vt.MouseLeft, X: 0, Y: 3,
	}}, 1)
	if app.state.Selected().String() != "/nested" || !app.state.IsExpanded(jsonInspectorTestPointer(t, "/nested")) {
		t.Fatalf("pointer state = %s, %v", app.state.Selected(), app.state.Expanded())
	}
	jsonInspectorRuntimeKey(t, runtime, vt.KeyRight, 0, vt.Modifiers{}, vt.KeyPress, 1)
	if app.state.Selected().String() != "/nested/flag" {
		t.Fatalf("right selected = %s", app.state.Selected())
	}
}

func jsonInspectorRuntimeKey(
	t *testing.T,
	runtime *tui.Runtime[jsonInspectorRuntimeMessage],
	code vt.KeyCode,
	character rune,
	modifiers vt.Modifiers,
	action vt.KeyAction,
	expectedMessages int,
) {
	t.Helper()
	jsonInspectorRuntimeEvent(t, runtime, vt.Event{Kind: vt.EventKey, Key: vt.KeyEvent{
		Code: code, Character: character, Modifiers: modifiers, Action: action,
		Protocol: vt.KeyProtocolLegacy,
	}}, expectedMessages)
}

func jsonInspectorRuntimeEvent(
	t *testing.T,
	runtime *tui.Runtime[jsonInspectorRuntimeMessage],
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

func jsonInspectorSurfaceRowContains(surface *surface.Surface, y int32, expected string) bool {
	var row string
	for x := int32(0); x < int32(surface.Width()); x++ {
		cell, ok := surface.Cell(x, y)
		if ok {
			row += cell.Content()
		}
	}
	return strings.Contains(row, expected)
}
