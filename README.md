# Nagi TUI for Go

[日本語](README_ja.md)

Nagi TUI for Go provides immutable Terminal Presentation Rules, a native
cell-based TUI runtime, Unicode-aware semantic nodes, 27 standard widgets,
supervised asynchronous work, subscriptions, and deterministic test support

## Requirements

- Go 1.25 or newer
- Linux or macOS on x86-64 or ARM64

## Installation

```sh
go get github.com/mayahiro/nagitui-go@v0.1.0
```

## Quick start

Run the [minimal stateful counter](examples/counter/README.md):

```sh
go run ./examples/counter
```

## Packages

| Package | Responsibility |
| --- | --- |
| Module root `tui` | Terminal Presentation Rules, App lifecycle, semantic nodes, scoped key maps, layout, events, Effects, Subscriptions, and terminal loop |
| `surface` | Geometry, Cells, Surface drawing, composition, diffing, and snapshots |
| `widget` | 27 standard widgets built from the public TUI API |
| `tuitest` | Virtual input, resize, time, effects, subscriptions, and frame inspection |
| `github.com/mayahiro/nagi-go/content` | Shared source-neutral Content used by Terminal Presentation Rules |
| `github.com/mayahiro/nagi-go/text` | Shared Unicode 17 text primitives |
| `github.com/mayahiro/nagi-go/vt` | Shared typed terminal input/output, Color, Attributes, and Style |

The root package re-exports Surface geometry and VT style types for application
convenience. Their canonical definitions remain in `surface` and `vt`

The [Nagi semantic specifications](https://github.com/mayahiro/nagi/tree/main/spec)
define behavior shared with the Rust implementation

## Testing applications

Package `tuitest` drives messages, terminal input, resize, virtual time,
Effects, Subscriptions, Runtime notices, frame inspection, and active resolved
action queries without a real terminal

The shared [event-driven application architecture](https://github.com/mayahiro/nagi/blob/main/docs/EVENT_DRIVEN_APPLICATIONS.md)
explains how process output and timers enter Nagi without a second UI loop

`RuntimeConfig.WidthProfile` and `TerminalOptions.WidthProfile` select one cell
width policy for Core measurement, rendering, hit geometry, and cursor
placement. Pass `ViewContext.WidthProfile` to width-sensitive widget builders.
Unexpected asynchronous lifecycle transitions are available through the
bounded Runtime notice queue or the terminal notice-handler entry points

## Examples

Run commands from the Go repository root in a real terminal

| Example | Command |
| --- | --- |
| [Terminal Presentation Rules](examples/presentation/README.md) | `go run ./examples/presentation` |
| [Counter](examples/counter/README.md) | `go run ./examples/counter` |
| [Command palette](examples/command-palette/README.md) | `go run ./examples/command-palette` |
| [Async search](examples/async-search/README.md) | `go run ./examples/async-search` |
| [Event-driven log viewer](examples/log-viewer/README.md) | `go run ./examples/log-viewer` |
| [Virtual scroll](examples/virtual-scroll/README.md) | `go run ./examples/virtual-scroll` |
| [Variable-height feed](examples/virtual-feed/README.md) | `go run ./examples/virtual-feed` |
| [Widget gallery](examples/widget-gallery/README.md) | `go run ./examples/widget-gallery` |
| [Extended widget gallery](examples/extended-widget-gallery/README.md) | `go run ./examples/extended-widget-gallery` |
| [Dashboard](examples/dashboard/README.md) | `go run ./examples/dashboard` |
| [Filtered list](examples/filtered-list/README.md) | `go run ./examples/filtered-list` |
| [File browser](examples/file-browser/README.md) | `go run ./examples/file-browser` |
| [Multi-pane log viewer](examples/multi-pane-log-viewer/README.md) | `go run ./examples/multi-pane-log-viewer` |
| [Form validation](examples/form-validation/README.md) | `go run ./examples/form-validation` |

## Limitations

Terminal input and output must be connected to a terminal. Mouse reporting is
disabled by default. Raw mode and screen restoration are best effort on normal
return, error, and panic paths. Process abort, nested terminal sessions,
suspend and resume, and `/dev/tty` acquisition are not supported

`ScrollViewport` clips and scrolls an eager child tree. Large data sets can use
`VirtualScrollViewport`, which declares the complete cell extent and constructs
only the current visible or bounded-overscan `VirtualFragment`.
`Node.RevealDescendant` keeps a stable descendant ID visible without moving
focus; virtual targets must be present in the current fragment

`VirtualFlow` retains variable item heights and stable anchors across append,
prepend, removal, streaming updates, and width changes while constructing only
the visible fragment and Cell-bounded overscan. It has zero intrinsic height,
so assign a layout `Length`. `widget.VirtualFeed` adds end following and
application-controlled empty, loading, and unread slots without owning domain
state

`TextArea` keeps no-wrap behavior by default. `SoftWrap` adds visual-line
navigation, `BoundaryNavigation` can pass Up and Down through at visual
boundaries, and `Viewport` follows an application-identified zero-width typed
cursor anchor without an extra Tab stop. The cursor does not draw a caret
grapheme or shift following text

`Composer` adds controlled submit and history recall, automatic row bounds,
optional validation content, and insertion limits over `TextArea`. Applications
retain ownership of message meaning, history persistence, and sensitive-value
policy

`SelectableText` adds controlled grapheme-aligned keyboard selection over
immutable styled content. Copy actions emit application messages; clipboard
I/O, pointer selection, and redaction policy remain application concerns

`Disclosure` keeps expanded state in the application and constructs its body
only while expanded. Core Modal scopes focus their first descendant on entry
and return to previous focus on close by default; both targets are configurable.
`Node.BlockUnhandledEvents` adds an opt-in hard input boundary when a modal must
also stop unhandled raw Events and terminal fallback mapping

`Dialog` composes application-defined action lists, lazy controlled details,
explicit default and cancel targets, focus policies, and Cell-width action
wrapping. `ConfirmDialog` is the explicit-default two-action convenience

## License

Source code is available under the MIT License. Generated Unicode data is
distributed under the [Unicode License v3](UNICODE-LICENSE)
