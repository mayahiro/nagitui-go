package widget

import (
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/surface"
)

type widgetMessage struct {
	kind     string
	selected int
}

type widgetApp struct {
	presses  int
	selected int
	modal    bool
}

func (a *widgetApp) Init() tui.Effect[widgetMessage] {
	return tui.NoneEffect[widgetMessage]()
}

func (a *widgetApp) Update(message widgetMessage) tui.Effect[widgetMessage] {
	switch message.kind {
	case "press":
		a.presses++
	case "select":
		a.selected = message.selected
	case "close":
		a.modal = false
	}
	return tui.NoneEffect[widgetMessage]()
}

func (a *widgetApp) Subscriptions() tui.Subscription[widgetMessage] {
	return tui.NoneSubscription[widgetMessage]()
}

func (a *widgetApp) View(_ tui.ViewContext) tui.Node[widgetMessage] {
	base := tui.Column(
		NewButton(tui.NewNodeID("save"), "Save", func() widgetMessage {
			return widgetMessage{kind: "press"}
		}).Node(),
		NewList(
			tui.NewNodeID("list"),
			[]ListItem{
				NewListItem(tui.NewNodeID("item-one"), "One"),
				NewListItem(tui.NewNodeID("item-two"), "Two"),
			},
			a.selected,
			func(selected int) widgetMessage {
				return widgetMessage{kind: "select", selected: selected}
			},
		).Node(),
		NewProgress[widgetMessage](1, 4, 4).Node(),
		NewSpinner[widgetMessage](1).Label("Work").Node(),
	)
	if !a.modal {
		return base
	}
	return tui.Stack(
		base,
		NewModal(
			tui.NewNodeID("modal"),
			NewButton(tui.NewNodeID("inside"), "OK", func() widgetMessage {
				return widgetMessage{kind: "close"}
			}).Node(),
		).Title("Confirm").OnEscape(func() widgetMessage {
			return widgetMessage{kind: "close"}
		}).Node(),
	)
}

func TestPublicWidgetNodesRenderAndRouteEvents(t *testing.T) {
	app := &widgetApp{}
	runtime, err := tui.NewRuntimeWithClock(app, tui.NewRuntimeConfig(tui.Size{Width: 20, Height: 10}), tui.NewVirtualClock())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()

	initial, err := runtime.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	for row, expected := range []string{
		"[ Save ]            ",
		"> One               ",
		"  Two               ",
		"█░░░                ",
		"⠙ Work              ",
	} {
		if actual := widgetRowText(initial.Surface(), int32(row)); actual != expected {
			t.Fatalf("row %d = %q, want %q", row, actual, expected)
		}
	}

	if focused, err := runtime.RequestFocus(tui.NewNodeID("save")); err != nil || !focused {
		t.Fatalf("focus save = %t, %v", focused, err)
	}
	focusFrame, err := runtime.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	buttonCell, _ := focusFrame.Surface().Cell(0, 0)
	if !buttonCell.Style().Reverse {
		t.Fatal("focused button is not reverse styled")
	}
	if _, err := runtime.DispatchEvent(widgetKey(vt.KeyEnter)); err != nil {
		t.Fatal(err)
	}
	if count, err := runtime.ProcessPending(); err != nil || count != 1 {
		t.Fatalf("button updates = %d, %v", count, err)
	}
	if app.presses != 1 {
		t.Fatalf("presses = %d, want 1", app.presses)
	}

	if focused, err := runtime.RequestFocus(tui.NewNodeID("list")); err != nil || !focused {
		t.Fatalf("focus item = %t, %v", focused, err)
	}
	if _, err := runtime.DispatchEvent(widgetKey(vt.KeyDown)); err != nil {
		t.Fatal(err)
	}
	if count, err := runtime.ProcessPending(); err != nil || count != 1 {
		t.Fatalf("list updates = %d, %v", count, err)
	}
	if app.selected != 1 {
		t.Fatalf("selected = %d, want 1", app.selected)
	}
	if focused, ok := runtime.Interaction().Focused(); !ok || focused != tui.NewNodeID("list") {
		t.Fatalf("focused item = %q, %t", focused, ok)
	}

	app.modal = true
	runtime.RequestFrame()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if focused, err := runtime.RequestFocus(tui.NewNodeID("save")); err != nil || focused {
		t.Fatalf("outside focus = %t, %v", focused, err)
	}
	if focused, err := runtime.RequestFocus(tui.NewNodeID("inside")); err != nil || !focused {
		t.Fatalf("inside focus = %t, %v", focused, err)
	}
	if _, err := runtime.DispatchEvent(widgetKey(vt.KeyEscape)); err != nil {
		t.Fatal(err)
	}
	if count, err := runtime.ProcessPending(); err != nil || count != 1 {
		t.Fatalf("modal updates = %d, %v", count, err)
	}
	if app.modal {
		t.Fatal("modal remained visible")
	}
}

type compositeMessage struct {
	selected int
	selects  bool
}

type compositeApp struct {
	selected int
}

func (*compositeApp) Init() tui.Effect[compositeMessage] {
	return tui.NoneEffect[compositeMessage]()
}

func (*compositeApp) Subscriptions() tui.Subscription[compositeMessage] {
	return tui.NoneSubscription[compositeMessage]()
}

func (a *compositeApp) Update(message compositeMessage) tui.Effect[compositeMessage] {
	if message.selects {
		a.selected = message.selected
	}
	return tui.NoneEffect[compositeMessage]()
}

func (a *compositeApp) View(tui.ViewContext) tui.Node[compositeMessage] {
	rows := make([]TableRow, 5)
	for index := range rows {
		rows[index] = NewTableRow(
			tui.NodeID("row-"+string(rune('0'+index))),
			[]string{string(rune('0' + index)), "value-" + string(rune('0'+index))},
		)
	}
	return tui.Column(
		NewTable(
			"table",
			[]TableColumn{
				NewTableColumn("ID", tui.Fixed(4)),
				NewTableColumn("Value", tui.Flex(1)),
			},
			rows,
			a.selected,
			func(selected int) compositeMessage {
				return compositeMessage{selected: selected, selects: true}
			},
		).Viewport("table-body", tui.Fixed(2)).Node(),
		NewButton("after", "After", func() compositeMessage { return compositeMessage{} }).Node(),
	)
}

func TestTableIsOneTabStopAndSelectionFollowsViewport(t *testing.T) {
	app := &compositeApp{}
	runtime, err := tui.NewRuntimeWithClock[compositeMessage](
		app,
		tui.NewRuntimeConfig(tui.Size{Width: 24, Height: 6}),
		tui.NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if focused, err := runtime.RequestFocus("table"); err != nil || !focused {
		t.Fatalf("focus table = %t, %v", focused, err)
	}
	if _, err := runtime.DispatchEvent(widgetKey(vt.KeyTab)); err != nil {
		t.Fatal(err)
	}
	if focused, ok := runtime.Interaction().Focused(); !ok || focused != "after" {
		t.Fatalf("focus after Tab = %q, %t", focused, ok)
	}

	if focused, err := runtime.RequestFocus("table"); err != nil || !focused {
		t.Fatalf("refocus table = %t, %v", focused, err)
	}
	if _, err := runtime.DispatchEvent(widgetKey(vt.KeyEnd)); err != nil {
		t.Fatal(err)
	}
	if count, err := runtime.ProcessPending(); err != nil || count != 1 {
		t.Fatalf("select last = %d, %v", count, err)
	}
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	state, ok := runtime.Interaction().ScrollState("table-body")
	if !ok || state.Offset.Y != 3 || state.Maximum.Y != 3 {
		t.Fatalf("scroll state = %+v, %t", state, ok)
	}

	app.selected = 0
	runtime.RequestFrame()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.DispatchEvent(vt.Event{Kind: vt.EventMouse, Mouse: vt.MouseEvent{
		Kind: vt.MousePress, Button: vt.MouseLeft, X: 0, Y: 2,
	}}); err != nil {
		t.Fatal(err)
	}
	if count, err := runtime.ProcessPending(); err != nil || count != 1 {
		t.Fatalf("mouse selection = %d, %v", count, err)
	}
	if app.selected != 1 {
		t.Fatalf("selected = %d", app.selected)
	}
	if focused, ok := runtime.Interaction().Focused(); !ok || focused != "table" {
		t.Fatalf("focus after click = %q, %t", focused, ok)
	}
}

type listViewportApp struct {
	selected int
}

func (*listViewportApp) Init() tui.Effect[int] {
	return tui.NoneEffect[int]()
}

func (a *listViewportApp) Update(selected int) tui.Effect[int] {
	a.selected = selected
	return tui.NoneEffect[int]()
}

func (*listViewportApp) Subscriptions() tui.Subscription[int] {
	return tui.NoneSubscription[int]()
}

func (a *listViewportApp) View(tui.ViewContext) tui.Node[int] {
	items := make([]ListItem, 5)
	for index := range items {
		items[index] = NewListItem(tui.NodeID("list-row-"+string(rune('0'+index))), string(rune('0'+index)))
	}
	return NewList("large-list", items, a.selected, func(selected int) int {
		return selected
	}).Viewport("list-body", tui.Fixed(2)).Node()
}

func TestListSelectionFollowsVirtualViewport(t *testing.T) {
	app := &listViewportApp{}
	runtime, err := tui.NewRuntimeWithClock[int](
		app,
		tui.NewRuntimeConfig(tui.Size{Width: 12, Height: 2}),
		tui.NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if focused, err := runtime.RequestFocus("large-list"); err != nil || !focused {
		t.Fatalf("focus list = %t, %v", focused, err)
	}
	if _, err := runtime.DispatchEvent(widgetKey(vt.KeyEnd)); err != nil {
		t.Fatal(err)
	}
	if count, err := runtime.ProcessPending(); err != nil || count != 1 {
		t.Fatalf("select last = %d, %v", count, err)
	}
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	state, ok := runtime.Interaction().ScrollState("list-body")
	if !ok || state.Offset.Y != 3 || state.Maximum.Y != 3 {
		t.Fatalf("scroll state = %+v, %t", state, ok)
	}

	app.selected = 0
	runtime.RequestFrame()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.DispatchEvent(vt.Event{Kind: vt.EventMouse, Mouse: vt.MouseEvent{
		Kind: vt.MousePress, Button: vt.MouseLeft, X: 0, Y: 1,
	}}); err != nil {
		t.Fatal(err)
	}
	if count, err := runtime.ProcessPending(); err != nil || count != 1 {
		t.Fatalf("mouse selection = %d, %v", count, err)
	}
	if app.selected != 1 {
		t.Fatalf("selected = %d", app.selected)
	}
	if focused, ok := runtime.Interaction().Focused(); !ok || focused != "large-list" {
		t.Fatalf("focus after click = %q, %t", focused, ok)
	}
}

type wrappedListViewportApp struct{}

func (*wrappedListViewportApp) Init() tui.Effect[int] {
	return tui.NoneEffect[int]()
}

func (*wrappedListViewportApp) Update(int) tui.Effect[int] {
	return tui.NoneEffect[int]()
}

func (*wrappedListViewportApp) Subscriptions() tui.Subscription[int] {
	return tui.NoneSubscription[int]()
}

func (*wrappedListViewportApp) View(tui.ViewContext) tui.Node[int] {
	return NewList(
		"wrapped-list",
		[]ListItem{
			NewListItem("wrapped-first", "ABCDEFGHIJ"),
			NewListItem("wrapped-second", "B"),
		},
		0,
		func(selected int) int { return selected },
	).Viewport("wrapped-list-body", tui.Fixed(2)).Node()
}

func TestVirtualListRowsAreOneCellHigh(t *testing.T) {
	runtime, err := tui.NewRuntimeWithClock[int](
		&wrappedListViewportApp{},
		tui.NewRuntimeConfig(tui.Size{Width: 6, Height: 2}),
		tui.NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()

	frame, err := runtime.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	if actual := widgetRowText(frame.Surface(), 0); actual != "> ABCD" {
		t.Fatalf("first row = %q", actual)
	}
	if actual := widgetRowText(frame.Surface(), 1); actual != "  B   " {
		t.Fatalf("second row = %q", actual)
	}
}

type multilineTableViewportApp struct{}

func (*multilineTableViewportApp) Init() tui.Effect[int] {
	return tui.NoneEffect[int]()
}

func (*multilineTableViewportApp) Update(int) tui.Effect[int] {
	return tui.NoneEffect[int]()
}

func (*multilineTableViewportApp) Subscriptions() tui.Subscription[int] {
	return tui.NoneSubscription[int]()
}

func (*multilineTableViewportApp) View(tui.ViewContext) tui.Node[int] {
	return NewTable(
		"multiline-table",
		[]TableColumn{NewTableColumn("Value", tui.Fixed(4))},
		[]TableRow{
			NewTableRow("multiline-first", []string{"A\nX"}),
			NewTableRow("multiline-second", []string{"B"}),
		},
		0,
		func(selected int) int { return selected },
	).Viewport("multiline-table-body", tui.Fixed(2)).Node()
}

func TestVirtualTableRowsAreOneCellHigh(t *testing.T) {
	runtime, err := tui.NewRuntimeWithClock[int](
		&multilineTableViewportApp{},
		tui.NewRuntimeConfig(tui.Size{Width: 6, Height: 3}),
		tui.NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()

	frame, err := runtime.RenderIfDirty()
	if err != nil {
		t.Fatal(err)
	}
	cell, ok := frame.Surface().Cell(2, 2)
	if !ok || cell.Content() != "B" {
		t.Fatalf("second row cell = %q, %t", cell.Content(), ok)
	}
}

func widgetKey(code vt.KeyCode) vt.Event {
	return vt.Event{Kind: vt.EventKey, Key: vt.KeyEvent{
		Code: code, Action: vt.KeyPress, Protocol: vt.KeyProtocolLegacy,
	}}
}

func widgetRowText(rendered *surface.Surface, row int32) string {
	var text string
	for column := uint32(0); column < rendered.Width(); column++ {
		cell, _ := rendered.Cell(int32(column), row)
		text += cell.Content()
	}
	return text
}
