// Command virtual-feed renders stable items with variable terminal Cell heights
package main

import (
	"fmt"
	"log"

	"github.com/mayahiro/nagi-go/vt"
	tui "github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/widget"
)

type feedEntry struct {
	id     tui.NodeID
	label  string
	height uint32
}

type message struct {
	kind   string
	scroll tui.ScrollState
}

type feedExample struct {
	entries []feedEntry
	items   tui.VirtualFlowItems
	unread  bool
}

func newFeedExample() *feedExample {
	entries := make([]feedEntry, 60)
	items := make([]tui.VirtualFlowItem, len(entries))
	for index := range entries {
		id := tui.NodeID(fmt.Sprintf("entry-%d", index))
		entries[index] = feedEntry{
			id: id, label: fmt.Sprintf("Entry %02d", index), height: 1 + uint32(index%3),
		}
		items[index] = tui.NewVirtualFlowItem(id)
	}
	order, err := tui.NewVirtualFlowItems(items)
	if err != nil {
		panic(err)
	}
	return &feedExample{entries: entries, items: order, unread: true}
}

func (*feedExample) Init() tui.Effect[message] {
	return tui.NoneEffect[message]()
}

func (a *feedExample) Update(received message) tui.Effect[message] {
	switch received.kind {
	case "scroll":
		a.unread = !received.scroll.AtEnd
	case "quit":
		return tui.ExitEffect[message]()
	}
	return tui.NoneEffect[message]()
}

func (*feedExample) Subscriptions() tui.Subscription[message] {
	return tui.NoneSubscription[message]()
}

func (a *feedExample) View(tui.ViewContext) tui.Node[message] {
	entries := a.entries
	source := tui.NewVirtualFlowSource(a.items, func(context tui.VirtualFlowItemContext) tui.Node[message] {
		entry := entries[context.Index]
		rows := make([]tui.Node[message], entry.height)
		for line := range rows {
			if line == 0 {
				rows[line] = tui.Text[message](entry.label)
			} else {
				rows[line] = tui.Text[message](fmt.Sprintf("  continuation %d", line))
			}
		}
		return tui.Column(rows...).WithID(tui.NodeID("content-" + entry.id.String()))
	}).EstimatedHeight(func(context tui.VirtualFlowItemContext) uint32 {
		return entries[context.Index].height
	})
	feed := widget.NewVirtualFeed("feed", source).
		FollowEnd(false).
		OnScroll(func(state tui.ScrollState) message {
			return message{kind: "scroll", scroll: state}
		}).
		Empty(tui.Text[message]("No entries"))
	if a.unread {
		feed = feed.UnreadIndicator(tui.StyledText[message](
			" More entries below ", vt.Style{Reverse: true},
		))
	}
	return tui.Panel(
		tui.Column(
			feed.Node().WithLength(tui.Flex(1)),
			tui.Text[message]("PageUp/PageDown/Home/End or wheel scroll, Escape quits").
				WithLength(tui.Fixed(1)),
		),
		"Variable Feed",
	)
}

func main() {
	options := tui.DefaultTerminalOptions()
	options.FocusFirst = true
	err := tui.RunTerminal(
		newFeedExample(),
		options,
		func(event vt.Event) tui.EventAction[message] {
			if event.Kind == vt.EventKey && event.Key.Code == vt.KeyEscape {
				return tui.MessageAction(message{kind: "quit"})
			}
			return tui.IgnoreAction[message]()
		},
	)
	if err != nil {
		log.Fatal(err)
	}
}
