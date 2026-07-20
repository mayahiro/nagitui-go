# Multi-pane log viewer

This example combines Nagi TUI's subscription runtime with a bounded log model
and a multi-pane interface

`List` selects a source, `Table` renders a fixed header over a viewport,
`Paragraph` shows details, and `Panel` defines the screen regions. A simulated
subscription uses latest-value delivery and stops while the application is
paused

Run it from the Go repository root:

```sh
go run ./examples/multi-pane-log-viewer
```

Use Tab to move between panes, arrow keys to select, p to pause, and Escape or
Control-C to exit
