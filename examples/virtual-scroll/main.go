// Command virtual-scroll renders a million-row virtual viewport
package main

import (
	"fmt"
	"log"

	"github.com/mayahiro/nagi-go/vt"
	tui "github.com/mayahiro/nagitui-go"
)

const rows uint32 = 1_000_000

type message struct {
	quit bool
}

type virtualScroll struct {
	exiting bool
}

func (*virtualScroll) Init() tui.Effect[message] {
	return tui.NoneEffect[message]()
}

func (a *virtualScroll) Update(msg message) tui.Effect[message] {
	if msg.quit {
		a.exiting = true
		return tui.ExitEffect[message]()
	}
	return tui.NoneEffect[message]()
}

func (*virtualScroll) Subscriptions() tui.Subscription[message] {
	return tui.NoneSubscription[message]()
}

func (a *virtualScroll) View(tui.ViewContext) tui.Node[message] {
	status := "1000000 rows"
	if a.exiting {
		status = "Stopping"
	}
	return tui.Panel(
		tui.Column(
			tui.Text[message](status).WithLength(tui.Fixed(1)),
			tui.VirtualScrollViewportWithOptions(
				"rows",
				tui.Size{Height: rows},
				tui.ScrollViewportOptions[message]{Axis: tui.ScrollAxisVertical},
				func(viewport tui.VirtualViewport) tui.VirtualFragment[message] {
					visible := make([]tui.Node[message], 0, int(viewport.Size.Height))
					end := viewport.Offset.Y + viewport.Size.Height
					for index := viewport.Offset.Y; index < end; index++ {
						visible = append(visible, tui.Text[message](fmt.Sprintf("Row %06d", index)).
							WithID(tui.NodeID(fmt.Sprintf("row-%d", index))).
							WithLength(tui.Fixed(1)))
					}
					return tui.NewVirtualFragment(
						tui.ScrollOffset{Y: viewport.Offset.Y},
						tui.Column(visible...),
					)
				},
			).WithLength(tui.Flex(1)),
			tui.Text[message]("PageUp/PageDown/Home/End or wheel scroll, Escape quits").
				WithLength(tui.Fixed(1)),
		),
		"Virtual Scroll",
	)
}

func main() {
	options := tui.DefaultTerminalOptions()
	options.FocusFirst = true
	err := tui.RunTerminal(
		&virtualScroll{},
		options,
		func(event vt.Event) tui.EventAction[message] {
			switch {
			case event.Kind == vt.EventKey && event.Key.Code == vt.KeyEscape:
				return tui.MessageAction(message{quit: true})
			case event.Kind == vt.EventText && (event.Text == "q" || event.Text == "Q"):
				return tui.MessageAction(message{quit: true})
			default:
				return tui.IgnoreAction[message]()
			}
		},
	)
	if err != nil {
		log.Fatal(err)
	}
}
