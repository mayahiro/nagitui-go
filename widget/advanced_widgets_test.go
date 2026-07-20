package widget

import (
	"strings"
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

type advancedMessage struct {
	kind     string
	index    int
	value    bool
	text     string
	textArea TextAreaState
}

type advancedApp struct {
	checked      bool
	radio        bool
	tab          int
	option       int
	row          int
	tree         int
	treeExpanded bool
	text         TextAreaState
	query        string
	command      int
	activated    int
	wasActivated bool
}

func (*advancedApp) Init() tui.Effect[advancedMessage] {
	return tui.NoneEffect[advancedMessage]()
}

func (a *advancedApp) Update(message advancedMessage) tui.Effect[advancedMessage] {
	switch message.kind {
	case "check":
		a.checked = message.value
	case "radio":
		a.radio = true
	case "tab":
		a.tab = message.index
	case "option":
		a.option = message.index
	case "row":
		a.row = message.index
	case "tree":
		a.tree = message.index
	case "toggle-tree":
		if message.index != 0 {
			panic("unexpected tree branch")
		}
		a.treeExpanded = message.value
	case "edit":
		a.text = message.textArea
	case "query":
		a.query = message.text
	case "command":
		a.command = message.index
	case "activate":
		a.activated = message.index
		a.wasActivated = true
	}
	return tui.NoneEffect[advancedMessage]()
}

func (*advancedApp) Subscriptions() tui.Subscription[advancedMessage] {
	return tui.NoneSubscription[advancedMessage]()
}

func (a *advancedApp) View(_ tui.ViewContext) tui.Node[advancedMessage] {
	return tui.Column(
		NewScrollbar[advancedMessage](100, 20, 40, 10).Orientation(ScrollbarHorizontal).Node(),
		NewCheckbox(tui.NewNodeID("check"), "Enabled", a.checked, func(value bool) advancedMessage {
			return advancedMessage{kind: "check", value: value}
		}).Node(),
		NewRadio(tui.NewNodeID("radio"), "Primary", a.radio, func() advancedMessage {
			return advancedMessage{kind: "radio"}
		}).Node(),
		NewTabs(
			tui.NewNodeID("tabs"),
			[]TabItem{
				NewTabItem(tui.NewNodeID("tab-a"), "A"),
				NewTabItem(tui.NewNodeID("tab-b"), "B"),
			},
			a.tab,
			func(index int) advancedMessage { return advancedMessage{kind: "tab", index: index} },
		).Node(),
		NewSelect(tui.NewNodeID("select"), []string{"First", "Second"}, a.option, func(index int) advancedMessage {
			return advancedMessage{kind: "option", index: index}
		}).Node(),
		NewTable(
			tui.NewNodeID("table"),
			[]TableColumn{NewTableColumn("Name", tui.Fixed(8)), NewTableColumn("State", tui.Fixed(6))},
			[]TableRow{
				NewTableRow(tui.NewNodeID("row-a"), []string{"Alpha", "Ready"}),
				NewTableRow(tui.NewNodeID("row-b"), []string{"Beta", "Busy"}),
			},
			a.row,
			func(index int) advancedMessage { return advancedMessage{kind: "row", index: index} },
		).Node(),
		NewTree(
			tui.NewNodeID("tree"),
			[]TreeItem{
				NewTreeBranch(tui.NewNodeID("tree-root"), "Root", 0, a.treeExpanded),
				NewTreeLeaf(tui.NewNodeID("tree-child"), "Child", 1),
				NewTreeLeaf(tui.NewNodeID("tree-peer"), "Peer", 0),
			},
			a.tree,
			func(index int) advancedMessage { return advancedMessage{kind: "tree", index: index} },
		).OnToggle(func(index int, expanded bool) advancedMessage {
			return advancedMessage{kind: "toggle-tree", index: index, value: expanded}
		}).Node(),
		NewTextArea(tui.NewNodeID("text-area"), a.text, func(state TextAreaState) advancedMessage {
			return advancedMessage{kind: "edit", textArea: state}
		}).Placeholder("Notes").Node(),
		NewCommandPalette(
			tui.NewNodeID("palette"), tui.NewNodeID("palette-input"), a.query,
			[]Command{
				NewCommand(tui.NewNodeID("command-open"), "Open"),
				NewCommand(tui.NewNodeID("command-save"), "Save"),
			},
			a.command,
			func(query string) advancedMessage { return advancedMessage{kind: "query", text: query} },
			func(index int) advancedMessage { return advancedMessage{kind: "command", index: index} },
			func(index int) advancedMessage { return advancedMessage{kind: "activate", index: index} },
		).Title("Commands").Node(),
	)
}

func TestExtendedWidgetsRenderAndRoutePublicEvents(t *testing.T) {
	app := &advancedApp{}
	runtime, err := tui.NewRuntimeWithClock(app, tui.NewRuntimeConfig(tui.Size{Width: 60, Height: 30}), tui.NewVirtualClock())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()

	initial, err := runtime.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	if row := widgetRowText(initial.Surface(), 0); !strings.HasPrefix(row, "────██────") {
		t.Fatalf("scrollbar row = %q", row)
	}
	if row := widgetRowText(initial.Surface(), 1); !strings.HasPrefix(row, "[ ] Enabled") {
		t.Fatalf("checkbox row = %q", row)
	}
	if row := widgetRowText(initial.Surface(), 5); !strings.HasPrefix(row, "  Name") {
		t.Fatalf("table row = %q", row)
	}

	advancedActivate(t, runtime, "check", vt.KeyEnter)
	if !app.checked {
		t.Fatal("checkbox did not change")
	}
	advancedActivate(t, runtime, "radio", vt.KeyEnter)
	if !app.radio {
		t.Fatal("radio did not select")
	}
	advancedActivate(t, runtime, "tab-a", vt.KeyRight)
	if app.tab != 1 {
		t.Fatalf("tab = %d", app.tab)
	}
	advancedActivate(t, runtime, "select", vt.KeyEnter)
	if app.option != 1 {
		t.Fatalf("option = %d", app.option)
	}
	advancedActivate(t, runtime, "table", vt.KeyDown)
	if app.row != 1 {
		t.Fatalf("row = %d", app.row)
	}
	advancedActivate(t, runtime, "tree", vt.KeyRight)
	if !app.treeExpanded {
		t.Fatal("tree did not expand")
	}

	if focused, err := runtime.RequestFocus(tui.NewNodeID("text-area")); err != nil || !focused {
		t.Fatalf("text focus = %t, %v", focused, err)
	}
	if _, err := runtime.DispatchEvent(vt.Event{Kind: vt.EventText, Text: "日"}); err != nil {
		t.Fatal(err)
	}
	if count, err := runtime.ProcessPending(); err != nil || count != 1 {
		t.Fatalf("text updates = %d, %v", count, err)
	}
	if app.text != NewTextAreaStateAtEnd("日") {
		t.Fatalf("text state = %#v", app.text)
	}

	if focused, err := runtime.RequestFocus(tui.NewNodeID("palette-input")); err != nil || !focused {
		t.Fatalf("palette focus = %t, %v", focused, err)
	}
	if _, err := runtime.DispatchEvent(vt.Event{Kind: vt.EventText, Text: "s"}); err != nil {
		t.Fatal(err)
	}
	if count, err := runtime.ProcessPending(); err != nil || count != 1 {
		t.Fatalf("query updates = %d, %v", count, err)
	}
	if app.query != "s" {
		t.Fatalf("query = %q", app.query)
	}
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.DispatchEvent(widgetKey(vt.KeyEnter)); err != nil {
		t.Fatal(err)
	}
	if count, err := runtime.ProcessPending(); err != nil || count != 1 {
		t.Fatalf("activation updates = %d, %v", count, err)
	}
	if !app.wasActivated || app.activated != 1 {
		t.Fatalf("activation = %t, %d", app.wasActivated, app.activated)
	}
}

func advancedActivate(t *testing.T, runtime *tui.Runtime[advancedMessage], id string, code vt.KeyCode) {
	t.Helper()
	if focused, err := runtime.RequestFocus(tui.NewNodeID(id)); err != nil || !focused {
		t.Fatalf("focus %s = %t, %v", id, focused, err)
	}
	if _, err := runtime.DispatchEvent(widgetKey(code)); err != nil {
		t.Fatal(err)
	}
	if count, err := runtime.ProcessPending(); err != nil || count != 1 {
		t.Fatalf("updates for %s = %d, %v", id, count, err)
	}
}
