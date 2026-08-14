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
	state   widget.CodeViewState
	request widget.CodeCopyRequest
}

type example struct {
	document widget.CodeDocument
	cache    *widget.CodeLayoutCache
	state    widget.CodeViewState
	wrap     bool
	copied   string
}

func newExample() *example {
	keyword := vt.Style{Foreground: vt.IndexedColor(5), Bold: true}
	stringStyle := vt.Style{Foreground: vt.IndexedColor(2)}
	comment := vt.Style{Dim: true}
	lines := []widget.CodeLine{
		mustStyledLine([]tui.TextSpan{
			tui.NewTextSpan("func", keyword), tui.NewTextSpan(" main() {", vt.Style{}),
		}),
		mustStyledLine([]tui.TextSpan{
			tui.NewTextSpan("\tgreeting", vt.Style{}), tui.NewTextSpan(" := ", vt.Style{}),
			tui.NewTextSpan("\"Hello, Nagi\"", stringStyle),
		}),
		mustLine("\tfmt.Println(greeting)"),
		mustStyledLine([]tui.TextSpan{
			tui.NewTextSpan("\t// Styled spans are supplied by the application", comment),
		}),
		mustLine("}"),
	}
	document, err := widget.NewCodeDocument(lines, true)
	if err != nil {
		panic(err)
	}
	return &example{document: document, cache: widget.NewCodeLayoutCache()}
}

func mustLine(text string) widget.CodeLine {
	line, err := widget.NewCodeLine(text)
	if err != nil {
		panic(err)
	}
	return line
}

func mustStyledLine(spans []tui.TextSpan) widget.CodeLine {
	line, err := widget.NewStyledCodeLine(spans)
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
			"Copy lines %d..%d: %d bytes",
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
		key,
		a.document,
		widget.DefaultCodeLayoutOptions().
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
		status = "Control-C copies selected complete lines"
	}
	return tui.Border(
		tui.Column(
			tui.StyledText[message](fmt.Sprintf("CodeView  wrap=%t", a.wrap), vt.Style{Bold: true}).WithLength(tui.Fixed(1)),
			widget.NewCodeView(
				"code", layout, a.state,
				func(state widget.CodeViewState) message {
					return message{kind: stateMessage, state: state}
				},
			).Viewport(viewportHeight).OnCopy(func(request widget.CodeCopyRequest) message {
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
