# Command palette

This example demonstrates the application runtime and low-level Core
nodes by implementing a searchable command list without the standard widget
package. It shows global text input mapping, grapheme-safe backspace, selection
state, and declarative rendering

Run it from the Go repository root:

```sh
go run ./examples/command-palette
```

Type to filter commands, use Up and Down to move selection, and press Enter or
Escape to exit. The example intentionally closes instead of executing a command
