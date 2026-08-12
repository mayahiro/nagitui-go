package main

import (
	"fmt"
	"os"

	"github.com/mayahiro/nagi-go/vt"
	tui "github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/widget"
)

type messageKind uint8

const (
	stateMessage messageKind = iota
	copyMessage
	toggleWrapMessage
)

type message struct {
	kind    messageKind
	state   widget.DiffViewState
	request widget.DiffCopyRequest
}

type example struct {
	document widget.DiffDocument
	cache    *widget.DiffLayoutCache
	state    widget.DiffViewState
	wrap     bool
	copied   string
}

func newExample() *example {
	oldRange := mustRange(1, 3)
	newRange := mustRange(1, 4)
	lines := []widget.DiffLine{
		widget.NewDiffMetadataLine(mustLine("diff --git a/main.go b/main.go")),
		widget.NewDiffMetadataLine(mustLine("--- a/main.go")),
		widget.NewDiffMetadataLine(mustLine("+++ b/main.go")),
		widget.NewDiffHunkLine(widget.NewDiffHunk(oldRange, newRange), mustLine("@@ -1,3 +1,4 @@")),
		mustContext(1, 1, "func main() {"),
		mustDeletion(2, "\tfmt.Println(\"old\")"),
		mustAddition(2, "\tmessage := \"Hello, Nagi\""),
		mustAddition(3, "\tfmt.Println(message)"),
		mustContext(3, 4, "}"),
	}
	document, err := widget.NewDiffDocument(lines, true)
	if err != nil {
		panic(err)
	}
	return &example{document: document, cache: &widget.DiffLayoutCache{}}
}

func mustLine(text string) widget.CodeLine {
	line, err := widget.NewCodeLine(text)
	if err != nil {
		panic(err)
	}
	return line
}

func mustRange(start, count uint64) widget.DiffRange {
	value, err := widget.NewDiffRange(start, count)
	if err != nil {
		panic(err)
	}
	return value
}

func mustContext(oldLine, newLine uint64, text string) widget.DiffLine {
	line, err := widget.NewDiffContextLine(oldLine, newLine, mustLine(text))
	if err != nil {
		panic(err)
	}
	return line
}

func mustAddition(newLine uint64, text string) widget.DiffLine {
	line, err := widget.NewDiffAdditionLine(newLine, mustLine(text))
	if err != nil {
		panic(err)
	}
	return line
}

func mustDeletion(oldLine uint64, text string) widget.DiffLine {
	line, err := widget.NewDiffDeletionLine(oldLine, mustLine(text))
	if err != nil {
		panic(err)
	}
	return line
}

func (*example) Init() tui.Effect[message] { return tui.NoneEffect[message]() }

func (a *example) Update(value message) tui.Effect[message] {
	switch value.kind {
	case stateMessage:
		a.state = value.state
	case copyMessage:
		a.copied = fmt.Sprintf(
			"Copy lines %d..%d: %d unified bytes",
			value.request.LineStart+1, value.request.LineEnd, len(value.request.Text),
		)
	case toggleWrapMessage:
		a.wrap = !a.wrap
	}
	return tui.NoneEffect[message]()
}

func (*example) Subscriptions() tui.Subscription[message] {
	return tui.NoneSubscription[message]()
}

func (a *example) View(context tui.ViewContext) tui.Node[message] {
	viewportWidth := uint32(1)
	if context.Size.Width > 2 {
		viewportWidth = context.Size.Width - 2
	}
	key := uint64(viewportWidth) << 1
	if a.wrap {
		key |= 1
	}
	layout, err := a.cache.Resolve(
		key, a.document,
		widget.DefaultDiffLayoutOptions().
			WithViewportWidth(viewportWidth).
			WithWrap(a.wrap).
			WithWidthProfile(context.WidthProfile),
	)
	if err != nil {
		panic(err)
	}
	viewportHeight := max(int(context.Size.Height)-5, 1)
	status := a.copied
	if status == "" {
		status = "Control-C copies selected unified lines"
	}
	return tui.Border(
		tui.Column(
			tui.StyledText[message](fmt.Sprintf("DiffView  wrap=%t", a.wrap), vt.Style{Bold: true}).WithLength(tui.Fixed(1)),
			widget.NewDiffView(
				"diff", layout, a.state,
				func(state widget.DiffViewState) message {
					return message{kind: stateMessage, state: state}
				},
			).Viewport(viewportHeight).OnCopy(func(request widget.DiffCopyRequest) message {
				return message{kind: copyMessage, request: request}
			}).Node().WithLength(tui.Flex(1)),
			tui.Text[message](status).WithLength(tui.Fixed(1)),
			tui.Text[message]("Arrows select and scroll, Shift-Up/Down extends, W toggles wrap").WithLength(tui.Fixed(1)),
		),
		vt.Style{},
	)
}

func mapEvent(event vt.Event) tui.EventAction[message] {
	if event.Kind == vt.EventKey && (event.Key.Code == vt.KeyEscape ||
		(event.Key.Code == vt.KeyCharacter && event.Key.Character == 'q')) {
		return tui.ExitAction[message]()
	}
	if event.Kind == vt.EventText && event.Text == "w" ||
		event.Kind == vt.EventKey && event.Key.Code == vt.KeyCharacter &&
			event.Key.Character == 'w' && event.Key.Modifiers == (vt.Modifiers{}) {
		return tui.MessageAction(message{kind: toggleWrapMessage})
	}
	return tui.IgnoreAction[message]()
}

func run() error {
	options := tui.DefaultTerminalOptions()
	options.FocusFirst = true
	mouseTracking := vt.MouseTrackingPress
	options.MouseTracking = &mouseTracking
	return tui.RunTerminal[message](newExample(), options, mapEvent)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
