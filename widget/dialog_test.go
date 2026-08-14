package widget

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/surface"
)

type dialogFixtureApp struct {
	kind             string
	defaultAction    string
	cancelAction     string
	confirmEnabled   bool
	cancelEnabled    bool
	details          string
	initial          string
	wrap             *uint32
	destructive      bool
	keyMap           tui.KeyMap
	outer            bool
	detailBodyBuilds int
	messages         []string
}

func (*dialogFixtureApp) Init() tui.Effect[string] { return tui.NoneEffect[string]() }
func (*dialogFixtureApp) Subscriptions() tui.Subscription[string] {
	return tui.NoneSubscription[string]()
}
func (a *dialogFixtureApp) Update(message string) tui.Effect[string] {
	a.messages = append(a.messages, message)
	return tui.NoneEffect[string]()
}
func (a *dialogFixtureApp) View(tui.ViewContext) tui.Node[string] {
	confirm := NewDialogAction(
		tui.NewNodeID("confirm"), "Confirm", func() string { return "confirm" },
	).Enabled(a.confirmEnabled)
	cancel := NewDialogAction(
		tui.NewNodeID("cancel"), "Cancel", func() string { return "cancel" },
	).Enabled(a.cancelEnabled)
	body := tui.Text[string]("Body").Focusable(tui.NewNodeID("body"))
	var node tui.Node[string]
	if a.kind == "confirm" {
		var defaultAction ConfirmDialogDefault
		switch a.defaultAction {
		case "confirm":
			defaultAction = ConfirmDialogDefaultConfirm()
		case "cancel":
			defaultAction = ConfirmDialogDefaultCancel()
		default:
			panic("invalid ConfirmDialog default " + a.defaultAction)
		}
		dialog := NewConfirmDialog(
			tui.NewNodeID("dialog"), body, confirm, cancel, defaultAction,
		).Title(tui.StyledText[string]("Question", vt.Style{Bold: true}))
		if details, ok := a.fixtureDetails(); ok {
			dialog = dialog.Details(details)
		}
		if a.wrap != nil {
			dialog = dialog.ActionWrapWidth(*a.wrap)
		}
		switch a.initial {
		case "derived":
		case "body":
			dialog = dialog.InitialFocus(tui.ModalInitialFocusTarget(tui.NewNodeID("body")))
		case "none":
			dialog = dialog.InitialFocus(tui.ModalInitialFocusNone())
		default:
			panic("invalid Dialog initial policy " + a.initial)
		}
		if a.destructive {
			dialog = dialog.DestructiveStyle(dialogDestructiveButtonStyle())
		}
		node = dialog.Node()
	} else {
		dialog := NewDialog(
			tui.NewNodeID("dialog"),
			body,
			[]DialogAction[string]{
				confirm,
				cancel,
				NewDialogAction(tui.NewNodeID("later"), "Later", func() string { return "later" }),
			},
		).Title(tui.StyledText[string]("Question", vt.Style{Bold: true}))
		if target, ok := dialogFixtureTarget(a.defaultAction); ok {
			dialog = dialog.DefaultAction(target)
		}
		if target, ok := dialogFixtureTarget(a.cancelAction); ok {
			dialog = dialog.CancelAction(target)
		}
		if details, ok := a.fixtureDetails(); ok {
			dialog = dialog.Details(details)
		}
		if a.wrap != nil {
			dialog = dialog.ActionWrapWidth(*a.wrap)
		}
		switch a.initial {
		case "derived":
		case "body":
			dialog = dialog.InitialFocus(tui.ModalInitialFocusTarget(tui.NewNodeID("body")))
		case "none":
			dialog = dialog.InitialFocus(tui.ModalInitialFocusNone())
		default:
			panic("invalid Dialog initial policy " + a.initial)
		}
		node = dialog.Node()
	}
	node = node.WithKeyScope(tui.NewKeyScope(tui.NewNodeID("dialog-scope"), a.keyMap))
	if a.outer {
		return tui.Padding(node, tui.UniformInsets(0)).OnEvent(
			tui.NewNodeID("outer"),
			func(vt.Event) tui.EventResult[string] { return tui.MessageResult("raw") },
		)
	}
	return node
}

func (a *dialogFixtureApp) fixtureDetails() (Disclosure[string], bool) {
	if a.details == "none" {
		return Disclosure[string]{}, false
	}
	expanded := false
	switch a.details {
	case "collapsed":
	case "expanded":
		expanded = true
	default:
		panic("invalid Dialog details " + a.details)
	}
	return NewDisclosure(
		tui.NewNodeID("details"),
		tui.Text[string]("Details"),
		expanded,
		func(next bool) string { return "details:" + strconv.FormatBool(next) },
	).Body(func() tui.Node[string] {
		a.detailBodyBuilds++
		return tui.Text[string]("Detail body")
	}), true
}

func TestDialogActionsMatchSharedFixtures(t *testing.T) {
	records := loadWidgetFixtures(
		t,
		"widgets/dialog.txt",
		"widget-dialog",
		"kind", "default", "cancel", "confirm-enabled", "cancel-enabled", "details",
		"initial", "start", "event", "scope", "outer", "wrap", "destructive",
		"message", "consumed", "focus", "builds", "rows", "availability",
	)
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			descriptors := dialogFixtureDescriptors(
				record.Field("kind"),
				record.Field("default"),
				record.Field("cancel"),
				fixtureBool(t, record.Field("confirm-enabled")),
				fixtureBool(t, record.Field("cancel-enabled")),
			)
			dialogAssertDescriptorContract(t, descriptors)
			expectedAvailability := dialogFixtureStrings(record.Field("availability"))
			actualAvailability := []string{
				dialogFixtureAvailability(descriptors[0].Availability()),
				dialogFixtureAvailability(descriptors[1].Availability()),
			}
			if !slices.Equal(actualAvailability, expectedAvailability) {
				t.Fatalf("availability = %#v, want %#v", actualAvailability, expectedAvailability)
			}

			app := &dialogFixtureApp{
				kind:           record.Field("kind"),
				defaultAction:  record.Field("default"),
				cancelAction:   record.Field("cancel"),
				confirmEnabled: fixtureBool(t, record.Field("confirm-enabled")),
				cancelEnabled:  fixtureBool(t, record.Field("cancel-enabled")),
				details:        record.Field("details"),
				initial:        record.Field("initial"),
				wrap:           dialogFixtureWrap(t, record.Field("wrap")),
				destructive:    fixtureBool(t, record.Field("destructive")),
				keyMap:         dialogFixtureKeyMap(t, record.Field("scope")),
				outer:          record.Field("outer") == "raw",
			}
			runtime, err := tui.NewRuntimeWithClock(
				app,
				tui.NewRuntimeConfig(tui.Size{Width: 50, Height: 14}),
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

			if record.Field("start") != "auto" {
				focused, err := runtime.RequestFocus(tui.NewNodeID(record.Field("start")))
				if err != nil || !focused {
					t.Fatalf("start focus = %t, %v", focused, err)
				}
			}

			var dispatch *tui.EventDispatch
			if record.Field("event") != "none" {
				event := dialogFixtureEvent(t, record.Field("event"), frame.Surface())
				eventDispatch, err := runtime.DispatchEvent(event)
				if err != nil {
					t.Fatal(err)
				}
				dispatch = &eventDispatch
				if _, err := runtime.ProcessPending(); err != nil {
					t.Fatal(err)
				}
			}

			expectedMessages := dialogFixtureStrings(record.Field("message"))
			if !slices.Equal(app.messages, expectedMessages) {
				t.Errorf("messages = %#v, want %#v", app.messages, expectedMessages)
			}
			dialogAssertDispatch(
				t, dispatch, len(expectedMessages), fixtureBool(t, record.Field("consumed")),
			)
			focus, hasFocus := runtime.Interaction().Focused()
			expectedFocus := record.Field("focus")
			if expectedFocus == "none" {
				if hasFocus {
					t.Errorf("focus = %q, want none", focus)
				}
			} else if !hasFocus || focus.String() != expectedFocus {
				t.Errorf("focus = %q, %t, want %q", focus, hasFocus, expectedFocus)
			}
			expectedBuilds, err := strconv.Atoi(record.Field("builds"))
			if err != nil {
				t.Fatal(err)
			}
			if app.detailBodyBuilds != expectedBuilds {
				t.Errorf("detail builds = %d, want %d", app.detailBodyBuilds, expectedBuilds)
			}
			expectedRows, err := strconv.Atoi(record.Field("rows"))
			if err != nil {
				t.Fatal(err)
			}
			if actual := dialogActionRowCount(frame.Surface()); actual != expectedRows {
				t.Errorf("action rows = %d, want %d", actual, expectedRows)
			}
			if fixtureBool(t, record.Field("destructive")) {
				x, y, ok := dialogFindText(frame.Surface(), "Confirm")
				if !ok {
					t.Fatal("Confirm label is missing")
				}
				cell, _ := frame.Surface().Cell(int32(x), int32(y))
				if index, ok := cell.Style().Foreground.Index(); !ok || index != 1 {
					t.Errorf("destructive foreground = %+v", cell.Style().Foreground)
				}
			}
		})
	}
}

func TestConfirmDialogRejectsAnImplicitDefault(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewConfirmDialog accepted the zero ConfirmDialogDefault")
		}
	}()
	NewConfirmDialog(
		tui.NewNodeID("dialog"),
		tui.Text[string]("Body"),
		NewDialogAction(tui.NewNodeID("confirm"), "Confirm", func() string { return "confirm" }),
		NewDialogAction(tui.NewNodeID("cancel"), "Cancel", func() string { return "cancel" }),
		ConfirmDialogDefault{},
	)
}

func TestConfirmDialogStyleBuilderDoesNotMutateTheSource(t *testing.T) {
	base := NewConfirmDialog(
		tui.NewNodeID("dialog"),
		tui.Text[string]("Body"),
		NewDialogAction(tui.NewNodeID("confirm"), "Confirm", func() string { return "confirm" }),
		NewDialogAction(tui.NewNodeID("cancel"), "Cancel", func() string { return "cancel" }),
		ConfirmDialogDefaultCancel(),
	)
	styled := base.DestructiveStyle(dialogDestructiveButtonStyle())

	if _, indexed := base.dialog.actions[0].style.Normal.Foreground.Index(); indexed {
		t.Fatal("DestructiveStyle mutated the source ConfirmDialog")
	}
	if index, indexed := styled.dialog.actions[0].style.Normal.Foreground.Index(); !indexed || index != 1 {
		t.Fatalf("styled confirm foreground = %+v", styled.dialog.actions[0].style.Normal.Foreground)
	}
}

type dialogReturnFocusApp struct {
	open bool
}

func (*dialogReturnFocusApp) Init() tui.Effect[string] { return tui.NoneEffect[string]() }
func (*dialogReturnFocusApp) Subscriptions() tui.Subscription[string] {
	return tui.NoneSubscription[string]()
}
func (a *dialogReturnFocusApp) Update(message string) tui.Effect[string] {
	if message == "cancel" {
		a.open = false
	}
	return tui.NoneEffect[string]()
}
func (a *dialogReturnFocusApp) View(tui.ViewContext) tui.Node[string] {
	background := tui.Row(
		tui.Text[string]("Opener").Focusable(tui.NewNodeID("opener")),
		tui.Text[string]("Return").Focusable(tui.NewNodeID("return-target")),
	)
	if !a.open {
		return background
	}
	dialog := NewDialog(
		tui.NewNodeID("dialog"),
		tui.Text[string]("Body"),
		[]DialogAction[string]{
			NewDialogAction(tui.NewNodeID("confirm"), "Confirm", func() string { return "confirm" }),
			NewDialogAction(tui.NewNodeID("cancel"), "Cancel", func() string { return "cancel" }),
		},
	).DefaultAction(tui.NewNodeID("confirm")).
		CancelAction(tui.NewNodeID("cancel")).
		ReturnFocus(tui.ModalReturnFocusTarget(tui.NewNodeID("return-target"))).
		Node()
	return tui.Stack(background, dialog)
}

func TestDialogDelegatesEntryAndReturnFocusToModalLifecycle(t *testing.T) {
	app := &dialogReturnFocusApp{}
	runtime, err := tui.NewRuntimeWithClock(
		app,
		tui.NewRuntimeConfig(tui.Size{Width: 30, Height: 6}),
		tui.NewVirtualClock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if focused, err := runtime.RequestFocus(tui.NewNodeID("opener")); err != nil || !focused {
		t.Fatalf("opener focus = %t, %v", focused, err)
	}

	app.open = true
	runtime.RequestFrame()
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if focus, ok := runtime.Interaction().Focused(); !ok || focus != tui.NewNodeID("confirm") {
		t.Fatalf("dialog entry focus = %q, %t", focus, ok)
	}

	if _, err := runtime.DispatchEvent(vt.Event{
		Kind: vt.EventKey,
		Key:  vt.KeyEvent{Code: vt.KeyEscape, Action: vt.KeyPress},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ProcessPending(); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.RenderIfDirty(); err != nil {
		t.Fatal(err)
	}
	if focus, ok := runtime.Interaction().Focused(); !ok || focus != tui.NewNodeID("return-target") {
		t.Fatalf("dialog return focus = %q, %t", focus, ok)
	}
}

type wideLabelDialogApp struct{}

func (*wideLabelDialogApp) Init() tui.Effect[string] { return tui.NoneEffect[string]() }
func (*wideLabelDialogApp) Update(string) tui.Effect[string] {
	return tui.NoneEffect[string]()
}
func (*wideLabelDialogApp) Subscriptions() tui.Subscription[string] {
	return tui.NoneSubscription[string]()
}
func (*wideLabelDialogApp) View(tui.ViewContext) tui.Node[string] {
	return NewDialog(
		tui.NewNodeID("dialog"),
		tui.Text[string]("Body"),
		[]DialogAction[string]{
			NewDialogAction(tui.NewNodeID("run"), "実行", func() string { return "run" }),
			NewDialogAction(tui.NewNodeID("back"), "戻る", func() string { return "back" }),
		},
	).DefaultAction(tui.NewNodeID("run")).
		CancelAction(tui.NewNodeID("back")).
		ActionWrapWidth(17).
		Node()
}

func TestDialogWrapsActionsByTerminalCellsInsteadOfUTF8Bytes(t *testing.T) {
	runtime, err := tui.NewRuntimeWithClock(
		&wideLabelDialogApp{},
		tui.NewRuntimeConfig(tui.Size{Width: 30, Height: 6}),
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
	runRow, runFound := dialogFindRow(frame.Surface(), "実行")
	backRow, backFound := dialogFindRow(frame.Surface(), "戻る")
	if !runFound || !backFound || runRow != backRow {
		t.Fatalf("wide-label rows = %d/%t and %d/%t", runRow, runFound, backRow, backFound)
	}
}

func dialogFixtureDescriptors(
	kind, defaultAction, cancelTarget string,
	confirmEnabled, cancelEnabled bool,
) [2]tui.ActionDescriptor {
	confirm := NewDialogAction(
		tui.NewNodeID("confirm"), "Confirm", func() string { return "confirm" },
	).Enabled(confirmEnabled)
	cancel := NewDialogAction(
		tui.NewNodeID("cancel"), "Cancel", func() string { return "cancel" },
	).Enabled(cancelEnabled)
	if kind == "confirm" {
		var selected ConfirmDialogDefault
		switch defaultAction {
		case "confirm":
			selected = ConfirmDialogDefaultConfirm()
		case "cancel":
			selected = ConfirmDialogDefaultCancel()
		default:
			panic("invalid ConfirmDialog default " + defaultAction)
		}
		return NewConfirmDialog(
			tui.NewNodeID("dialog"), tui.Text[string]("Body"), confirm, cancel, selected,
		).ActionDescriptors()
	}
	dialog := NewDialog(
		tui.NewNodeID("dialog"),
		tui.Text[string]("Body"),
		[]DialogAction[string]{
			confirm,
			cancel,
			NewDialogAction(tui.NewNodeID("later"), "Later", func() string { return "later" }),
		},
	)
	if target, ok := dialogFixtureTarget(defaultAction); ok {
		dialog = dialog.DefaultAction(target)
	}
	if target, ok := dialogFixtureTarget(cancelTarget); ok {
		dialog = dialog.CancelAction(target)
	}
	return dialog.ActionDescriptors()
}

func dialogAssertDescriptorContract(t *testing.T, descriptors [2]tui.ActionDescriptor) {
	t.Helper()
	if descriptors[0].ID() != ConfirmActionID || descriptors[0].Label() != "Confirm" {
		t.Errorf("confirm descriptor = %q %q", descriptors[0].ID(), descriptors[0].Label())
	}
	if descriptors[1].ID() != DismissActionID || descriptors[1].Label() != "Dismiss" {
		t.Errorf("dismiss descriptor = %q %q", descriptors[1].ID(), descriptors[1].Label())
	}
	if actual := descriptors[0].DefaultBindings()[0].Stroke().Notation(); actual != "Enter" {
		t.Errorf("confirm binding = %q", actual)
	}
	if actual := descriptors[1].DefaultBindings()[0].Stroke().Notation(); actual != "Escape" {
		t.Errorf("dismiss binding = %q", actual)
	}
}

func dialogFixtureTarget(value string) (tui.NodeID, bool) {
	if value == "none" {
		return "", false
	}
	return tui.NewNodeID(value), true
}

func dialogFixtureWrap(t *testing.T, value string) *uint32 {
	t.Helper()
	if value == "none" {
		return nil
	}
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		t.Fatal(err)
	}
	width := uint32(parsed)
	return &width
}

func dialogFixtureKeyMap(t *testing.T, value string) tui.KeyMap {
	t.Helper()
	keyMap := tui.NewKeyMap()
	var err error
	switch value {
	case "default":
		return keyMap
	case "rebind":
		keyMap, err = keyMap.Rebind(ConfirmActionID, []tui.KeyBinding{
			tui.NewKeyBinding(tui.NewCharacterKeyStroke('x', vt.Modifiers{})),
		})
		if err == nil {
			keyMap, err = keyMap.Rebind(DismissActionID, []tui.KeyBinding{
				tui.NewKeyBinding(tui.NewCharacterKeyStroke('y', vt.Modifiers{})),
			})
		}
	case "unbind-confirm":
		keyMap, err = keyMap.Rebind(ConfirmActionID, nil)
	case "unbind-dismiss":
		keyMap, err = keyMap.Rebind(DismissActionID, nil)
	default:
		t.Fatalf("invalid Dialog key scope %q", value)
	}
	if err != nil {
		t.Fatal(err)
	}
	return keyMap
}

func dialogFixtureEvent(t *testing.T, value string, rendered *surface.Surface) vt.Event {
	t.Helper()
	keyboard := func(code vt.KeyCode, action vt.KeyAction) vt.Event {
		return vt.Event{
			Kind: vt.EventKey,
			Key:  vt.KeyEvent{Code: code, Action: action},
		}
	}
	switch value {
	case "enter":
		return keyboard(vt.KeyEnter, vt.KeyPress)
	case "repeat-enter":
		return keyboard(vt.KeyEnter, vt.KeyRepeat)
	case "escape":
		return keyboard(vt.KeyEscape, vt.KeyPress)
	case "space":
		return vt.Event{
			Kind: vt.EventKey,
			Key:  vt.KeyEvent{Code: vt.KeyCharacter, Character: ' ', Action: vt.KeyPress},
		}
	case "x":
		return vt.Event{
			Kind: vt.EventKey,
			Key:  vt.KeyEvent{Code: vt.KeyCharacter, Character: 'x', Action: vt.KeyPress},
		}
	case "y":
		return vt.Event{
			Kind: vt.EventKey,
			Key:  vt.KeyEvent{Code: vt.KeyCharacter, Character: 'y', Action: vt.KeyPress},
		}
	case "pointer-confirm":
		return dialogPointerEvent(t, rendered, "Confirm")
	case "pointer-cancel":
		return dialogPointerEvent(t, rendered, "Cancel")
	default:
		t.Fatalf("invalid Dialog event %q", value)
		return vt.Event{}
	}
}

func dialogPointerEvent(t *testing.T, rendered *surface.Surface, label string) vt.Event {
	t.Helper()
	x, y, ok := dialogFindText(rendered, label)
	if !ok {
		t.Fatalf("missing %s button", label)
	}
	return vt.Event{Kind: vt.EventMouse, Mouse: vt.MouseEvent{
		Kind: vt.MousePress, Button: vt.MouseLeft, X: x, Y: y,
	}}
}

func dialogActionRowCount(rendered *surface.Surface) int {
	rows := 0
	for y := uint32(0); y < rendered.Height(); y++ {
		row := dialogSurfaceRow(rendered, y)
		if strings.Contains(row, "Confirm") || strings.Contains(row, "Cancel") || strings.Contains(row, "Later") {
			rows++
		}
	}
	return rows
}

func dialogFindText(rendered *surface.Surface, needle string) (uint32, uint32, bool) {
	characters := []rune(needle)
	for y := uint32(0); y < rendered.Height(); y++ {
		for x := uint32(0); x+uint32(len(characters)) <= rendered.Width(); x++ {
			matches := true
			for offset, character := range characters {
				cell, _ := rendered.Cell(int32(x+uint32(offset)), int32(y))
				if cell.Content() != string(character) {
					matches = false
					break
				}
			}
			if matches {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

func dialogSurfaceRow(rendered *surface.Surface, y uint32) string {
	var row strings.Builder
	for x := uint32(0); x < rendered.Width(); x++ {
		cell, _ := rendered.Cell(int32(x), int32(y))
		row.WriteString(cell.Content())
	}
	return row.String()
}

func dialogFindRow(rendered *surface.Surface, needle string) (uint32, bool) {
	for y := uint32(0); y < rendered.Height(); y++ {
		if strings.Contains(dialogSurfaceRow(rendered, y), needle) {
			return y, true
		}
	}
	return 0, false
}

func dialogDestructiveButtonStyle() ButtonStyle {
	style := DefaultButtonStyle()
	style.Normal.Foreground = vt.IndexedColor(1)
	return style
}

func dialogAssertDispatch(
	t *testing.T,
	dispatch *tui.EventDispatch,
	messages int,
	consumed bool,
) {
	t.Helper()
	if dispatch == nil {
		if messages != 0 || consumed {
			t.Fatalf("missing dispatch, messages = %d, consumed = %t", messages, consumed)
		}
		return
	}
	if dispatch.Messages() != messages {
		t.Errorf("dispatch messages = %d, want %d", dispatch.Messages(), messages)
	}
	if dispatch.Consumed() != consumed {
		t.Errorf("dispatch consumed = %t, want %t", dispatch.Consumed(), consumed)
	}
}

func dialogFixtureAvailability(value tui.ActionAvailability) string {
	switch value {
	case tui.ActionEnabled:
		return "enabled"
	case tui.ActionDisabledPassThrough:
		return "pass"
	case tui.ActionDisabledConsume:
		return "consume"
	default:
		panic("invalid Dialog action availability")
	}
}

func dialogFixtureStrings(value string) []string {
	if value == "-" {
		return nil
	}
	return strings.Split(value, ",")
}
