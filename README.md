# Nagi TUI for Go

[日本語](README_ja.md)

Nagi TUI for Go is the native Go implementation of the Nagi TUI cell-based
terminal user interface framework

It provides a declarative application runtime, Unicode-aware semantic nodes,
21 standard widgets, supervised asynchronous work, subscriptions, and a
deterministic test harness

## Requirements

- Go 1.25 or newer
- Linux or macOS on x86-64 or ARM64

## Installation after the first release

After v0.1.0 is published, add the module to an application

```sh
go get github.com/mayahiro/nagitui-go@v0.1.0
```

The tag will select the released repository revision. Go records the selected
module version in `go.mod` and `go.sum`

## Quick start

This counter updates application state on Enter and exits on Escape

```go
package main

import (
	"fmt"
	"log"

	"github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagi-go/vt"
)

type message struct {
	increment bool
	quit      bool
}

type counter struct {
	count   uint64
	exiting bool
}

func (*counter) Init() tui.Effect[message] {
	return tui.NoneEffect[message]()
}

func (a *counter) Update(msg message) tui.Effect[message] {
	if msg.quit {
		a.exiting = true
		return tui.ExitEffect[message]()
	}
	if msg.increment {
		a.count++
	}
	return tui.NoneEffect[message]()
}

func (*counter) Subscriptions() tui.Subscription[message] {
	return tui.NoneSubscription[message]()
}

func (a *counter) View(_ tui.ViewContext) tui.Node[message] {
	status := "Running"
	if a.exiting {
		status = "Stopping"
	}
	return tui.Panel(
		tui.Column(
			tui.Text[message](fmt.Sprintf("Count: %d", a.count)),
			tui.Text[message]("Status: "+status),
			tui.Text[message]("Press Enter to increment, Escape to exit"),
		),
		"Counter",
	)
}

func main() {
	err := tui.RunTerminal[message](
		&counter{},
		tui.DefaultTerminalOptions(),
		func(event vt.Event) tui.EventAction[message] {
			switch {
			case event.Kind == vt.EventKey && event.Key.Code == vt.KeyEnter:
				return tui.MessageAction(message{increment: true})
			case event.Kind == vt.EventKey && event.Key.Code == vt.KeyEscape:
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
```

An `App` owns its state, processes one Message at a time in `Update`, declares
long-lived sources through `Subscriptions`, and rebuilds a semantic `Node` tree
from application state and `ViewContext`. `ExitEffect` renders the final dirty
view before terminal restoration. `RunTerminalContext` adds external
`context.Context` cancellation. `Init` and `Subscriptions` return explicit
no-work values when they are unused

## Packages and capabilities

| Package | Use |
| --- | --- |
| Module root `tui` | App lifecycle, runtime, Core nodes, layout, events, interaction, Effects, Subscriptions, and terminal loop |
| `github.com/mayahiro/nagitui-go/surface` | Geometry, Cells, Surface drawing, composition, diffing, and snapshots |
| `github.com/mayahiro/nagitui-go/widget` | The 21 standard widgets |
| `github.com/mayahiro/nagitui-go/tuitest` | Virtual input, resize, time, effects, subscriptions, and frame inspection |
| `github.com/mayahiro/nagi-go/text` | Shared Unicode 17 graphemes, terminal-width profiles, wrapping, truncation, and positions |
| `github.com/mayahiro/nagi-go/vt` | Shared typed terminal input/output, Color, Attributes, and Style |

The root `tui` package re-exports the Surface-owned geometry types and VT-owned
style types for application convenience. Their canonical definitions remain in
`surface` and the shared `vt` package

Core composition includes Text, RichText, Paragraph, safe ANSI SGR text,
SurfaceNode, TextInput, Spacer, Gap, Row, Column, Stack, Padding, Border, Panel,
Align, Clip, ScrollViewport, and Modal nodes

Package `widget` includes List, Button, Modal, Progress, Spinner, Scrollbar,
TextArea, Table, Tree, Tabs, Checkbox, Radio, Select, Command Palette,
Sparkline, BarChart, Chart, Help, Paginator, FilePicker, and Calendar

## Testing applications

Package `tuitest` can drive messages, terminal input, resize, virtual time,
controlled Effects, and manual Subscriptions without a real terminal. It also
exposes frame and message history, interaction state, supervisor diagnostics,
and canonical Surface snapshots

## Examples

Run examples from the repository root in a real terminal

| Example | Command |
| --- | --- |
| [Command palette](examples/command-palette/README.md) | `go run ./examples/command-palette` |
| [Async search](examples/async-search/README.md) | `go run ./examples/async-search` |
| [Log viewer](examples/log-viewer/README.md) | `go run ./examples/log-viewer` |
| [Widget gallery](examples/widget-gallery/README.md) | `go run ./examples/widget-gallery` |
| [Extended widget gallery](examples/extended-widget-gallery/README.md) | `go run ./examples/extended-widget-gallery` |
| [Dashboard](examples/dashboard/README.md) | `go run ./examples/dashboard` |
| [Filtered list](examples/filtered-list/README.md) | `go run ./examples/filtered-list` |
| [File browser](examples/file-browser/README.md) | `go run ./examples/file-browser` |
| [Multi-pane log viewer](examples/multi-pane-log-viewer/README.md) | `go run ./examples/multi-pane-log-viewer` |
| [Form validation](examples/form-validation/README.md) | `go run ./examples/form-validation` |

## Terminal behavior and limitations

Mouse reporting is disabled by default so terminal text selection remains
available. Enable it in `TerminalOptions` when an application needs pointer
input

Standard input and output must be connected to a terminal. Restoration of raw
mode and screen state is best effort on normal return, error, and panic paths.
Process abort, nested terminal sessions, suspend and resume, and `/dev/tty`
acquisition are not supported

## License

Nagi TUI for Go source code is available under the MIT License. Generated
Unicode data is distributed under the [Unicode License v3](UNICODE-LICENSE)
