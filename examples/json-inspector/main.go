package main

import (
	"fmt"
	"os"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/widget"
)

type messageKind uint8

const (
	stateMessage messageKind = iota
	copyMessage
)

type message struct {
	kind    messageKind
	state   widget.JSONInspectorState
	request widget.JSONInspectorCopyRequest
}

type example struct {
	document widget.JSONDocument
	state    widget.JSONInspectorState
	copied   string
}

func newExample() *example {
	maxNodes, err := widget.NewJSONNumber("100000")
	if err != nil {
		panic(err)
	}
	maxDepth, err := widget.NewJSONNumber("128")
	if err != nil {
		panic(err)
	}
	limits, err := widget.NewJSONObject([]widget.JSONMember{
		widget.NewJSONMember("max_nodes", widget.NewJSONNumberValue(maxNodes)),
		widget.NewJSONMember("max_depth", widget.NewJSONNumberValue(maxDepth)),
	})
	if err != nil {
		panic(err)
	}
	root, err := widget.NewJSONObject([]widget.JSONMember{
		widget.NewJSONMember("component", widget.NewJSONString("JSONInspector")),
		widget.NewJSONMember("enabled", widget.NewJSONBoolean(true)),
		widget.NewJSONMember("features", widget.NewJSONArray([]widget.JSONValue{
			widget.NewJSONString("controlled selection"),
			widget.NewJSONString("bounded preview"),
			widget.NewJSONString("complete copy payload"),
		})),
		widget.NewJSONMember("limits", limits),
		widget.NewJSONMember("optional", widget.NewJSONNull()),
	})
	if err != nil {
		panic(err)
	}
	document, err := widget.NewJSONDocument(root)
	if err != nil {
		panic(err)
	}
	return &example{document: document}
}

func (*example) Init() tui.Effect[message] {
	return tui.NoneEffect[message]()
}

func (a *example) Update(msg message) tui.Effect[message] {
	switch msg.kind {
	case stateMessage:
		a.state = msg.state
	case copyMessage:
		path := msg.request.Path.String()
		if path == "" {
			path = "$"
		}
		a.copied = fmt.Sprintf("Copy %s: %s", path, msg.request.Text)
	}
	return tui.NoneEffect[message]()
}

func (*example) Subscriptions() tui.Subscription[message] {
	return tui.NoneSubscription[message]()
}

func (a *example) View(tui.ViewContext) tui.Node[message] {
	copied := a.copied
	if copied == "" {
		copied = "Control-C requests the complete selected value"
	}
	return tui.Border(
		tui.Column(
			tui.StyledText[message]("JSONInspector", vt.Style{Bold: true}).WithLength(tui.Fixed(1)),
			widget.NewJSONInspector(
				"inspector",
				a.document,
				a.state,
				func(state widget.JSONInspectorState) message {
					return message{kind: stateMessage, state: state}
				},
			).MaximumScalarGraphemes(24).OnCopy(func(request widget.JSONInspectorCopyRequest) message {
				return message{kind: copyMessage, request: request}
			}).Node().WithLength(tui.Flex(1)),
			tui.Text[message](copied).WithLength(tui.Fixed(1)),
			tui.Text[message]("Arrows navigate, Enter toggles, Esc or Q exits").WithLength(tui.Fixed(1)),
		),
		vt.Style{},
	)
}

func mapEvent(event vt.Event) tui.EventAction[message] {
	if event.Kind == vt.EventKey && (event.Key.Code == vt.KeyEscape ||
		(event.Key.Code == vt.KeyCharacter && event.Key.Character == 'q')) {
		return tui.ExitAction[message]()
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
