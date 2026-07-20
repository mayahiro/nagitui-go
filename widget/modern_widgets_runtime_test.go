package widget

import (
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

type modernWidgetMessage struct {
	kind  string
	index int
	date  CalendarDate
	text  TextAreaState
}

type modernWidgetApp struct {
	page    int
	file    int
	opened  int
	back    bool
	date    CalendarDate
	tree    int
	text    TextAreaState
	history *TextAreaHistory
}

func newModernWidgetApp() *modernWidgetApp {
	text := NewTextAreaStateAtEnd("ab")
	return &modernWidgetApp{
		page: 1, opened: -1, date: NewCalendarDate(2024, 2, 28), text: text,
		history: NewTextAreaHistory(text),
	}
}

func (*modernWidgetApp) Init() tui.Effect[modernWidgetMessage] {
	return tui.NoneEffect[modernWidgetMessage]()
}

func (a *modernWidgetApp) Update(message modernWidgetMessage) tui.Effect[modernWidgetMessage] {
	switch message.kind {
	case "page":
		a.page = message.index
	case "file":
		a.file = message.index
	case "open":
		a.opened = message.index
	case "back":
		a.back = true
	case "date":
		a.date = message.date
	case "tree":
		a.tree = message.index
	case "edit":
		a.history.Record(message.text)
		a.text = message.text
	case "undo":
		if state, ok := a.history.Undo(); ok {
			a.text = state
		}
	case "redo":
		if state, ok := a.history.Redo(); ok {
			a.text = state
		}
	}
	return tui.NoneEffect[modernWidgetMessage]()
}

func (*modernWidgetApp) Subscriptions() tui.Subscription[modernWidgetMessage] {
	return tui.NoneSubscription[modernWidgetMessage]()
}

func (a *modernWidgetApp) View(_ tui.ViewContext) tui.Node[modernWidgetMessage] {
	files := []FilePickerEntry{
		NewFilePickerFile("file-a", "A", "a"),
		NewFilePickerDirectory("file-b", "B", "b"),
		NewFilePickerFile("file-c", "C", "c"),
		NewFilePickerFile("file-d", "D", "d"),
	}
	treeItems := make([]TreeItem, 5)
	for index := range treeItems {
		treeItems[index] = NewTreeLeaf(tui.NewNodeID(string(rune('a'+index))), "Item"+string(rune('0'+index)), 0)
	}
	return tui.Column(
		NewPaginator("pages", a.page, 5, func(page int) modernWidgetMessage {
			return modernWidgetMessage{kind: "page", index: page}
		}).Node(),
		NewFilePicker("files", files, a.file, func(index int) modernWidgetMessage {
			return modernWidgetMessage{kind: "file", index: index}
		}).Viewport(2).OnOpen(func(index int) modernWidgetMessage {
			return modernWidgetMessage{kind: "open", index: index}
		}).OnBack(func() modernWidgetMessage {
			return modernWidgetMessage{kind: "back"}
		}).Node(),
		NewCalendar("calendar", a.date.Year, int(a.date.Month), a.date, func(date CalendarDate) modernWidgetMessage {
			return modernWidgetMessage{kind: "date", date: date}
		}).Node(),
		NewTree("tree", treeItems, a.tree, func(index int) modernWidgetMessage {
			return modernWidgetMessage{kind: "tree", index: index}
		}).Viewport(2).Node(),
		NewTextArea("area", a.text, func(state TextAreaState) modernWidgetMessage {
			return modernWidgetMessage{kind: "edit", text: state}
		}).OnUndo(func() modernWidgetMessage {
			return modernWidgetMessage{kind: "undo"}
		}).OnRedo(func() modernWidgetMessage {
			return modernWidgetMessage{kind: "redo"}
		}).Node(),
	)
}

func TestModernWidgetsRouteEventsAndSurviveResize(t *testing.T) {
	app := newModernWidgetApp()
	runtime, err := tui.NewRuntimeWithClock(
		app, tui.NewRuntimeConfig(tui.Size{Width: 40, Height: 20}), tui.NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}

	modernWidgetKey(t, runtime, "pages", vt.KeyRight, vt.Modifiers{})
	if app.page != 2 {
		t.Fatalf("page = %d", app.page)
	}
	modernWidgetKey(t, runtime, "files", vt.KeyPageDown, vt.Modifiers{})
	if app.file != 2 {
		t.Fatalf("file = %d", app.file)
	}
	modernWidgetKey(t, runtime, "files", vt.KeyEnter, vt.Modifiers{})
	if app.opened != 2 {
		t.Fatalf("opened = %d", app.opened)
	}
	modernWidgetKey(t, runtime, "files", vt.KeyBackspace, vt.Modifiers{})
	if !app.back {
		t.Fatal("back handler was not called")
	}

	modernWidgetKey(t, runtime, "calendar", vt.KeyRight, vt.Modifiers{})
	if app.date != NewCalendarDate(2024, 2, 29) {
		t.Fatalf("date after Right = %#v", app.date)
	}
	modernWidgetKey(t, runtime, "calendar", vt.KeyPageDown, vt.Modifiers{})
	if app.date != NewCalendarDate(2024, 3, 29) {
		t.Fatalf("date after PageDown = %#v", app.date)
	}

	modernWidgetKey(t, runtime, "tree", vt.KeyEnd, vt.Modifiers{})
	if app.tree != 4 {
		t.Fatalf("tree = %d", app.tree)
	}
	if focused, ok := runtime.Interaction().Focused(); !ok || focused != tui.NewNodeID("tree") {
		t.Fatalf("tree focus = %q, %t", focused, ok)
	}

	modernWidgetKey(t, runtime, "area", vt.KeyCharacter, vt.Modifiers{Control: true}, 'a')
	if _, _, ok := app.text.Selection(); !ok {
		t.Fatal("Control-A did not select text")
	}
	modernWidgetEvent(t, runtime, vt.Event{Kind: vt.EventText, Text: "X"})
	if app.text.Value() != "X" {
		t.Fatalf("edited text = %q", app.text.Value())
	}
	modernWidgetKey(t, runtime, "area", vt.KeyCharacter, vt.Modifiers{Control: true}, 'z')
	if app.text.Value() != "ab" {
		t.Fatalf("undo text = %q", app.text.Value())
	}
	modernWidgetKey(t, runtime, "area", vt.KeyCharacter, vt.Modifiers{Control: true}, 'y')
	if app.text.Value() != "X" {
		t.Fatalf("redo text = %q", app.text.Value())
	}

	for _, size := range []tui.Size{{Width: 12, Height: 10}, {Width: 60, Height: 24}} {
		runtime.Resize(size)
		frame, err := runtime.RenderIfDirty()
		if err != nil {
			t.Fatal(err)
		}
		if frame.Surface().Width() != size.Width || frame.Surface().Height() != size.Height {
			t.Fatalf("resized surface = %dx%d, want %dx%d", frame.Surface().Width(), frame.Surface().Height(), size.Width, size.Height)
		}
	}
}

func modernWidgetKey(t *testing.T, runtime *tui.Runtime[modernWidgetMessage], id tui.NodeID, code vt.KeyCode, modifiers vt.Modifiers, character ...rune) {
	t.Helper()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if focused, err := runtime.RequestFocus(id); err != nil || !focused {
		t.Fatalf("focus %s = %t, %v", id, focused, err)
	}
	key := vt.KeyEvent{Code: code, Modifiers: modifiers, Action: vt.KeyPress, Protocol: vt.KeyProtocolLegacy}
	if len(character) > 0 {
		key.Character = character[0]
	}
	modernWidgetEvent(t, runtime, vt.Event{Kind: vt.EventKey, Key: key})
}

func modernWidgetEvent(t *testing.T, runtime *tui.Runtime[modernWidgetMessage], event vt.Event) {
	t.Helper()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.DispatchEvent(event); err != nil {
		t.Fatal(err)
	}
	if count, err := runtime.ProcessPending(); err != nil || count != 1 {
		t.Fatalf("processed messages = %d, %v", count, err)
	}
}
