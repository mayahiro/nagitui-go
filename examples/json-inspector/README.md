# JSON inspector

This example builds typed immutable JSON, keeps selection and expansion in the
application, and shows the complete value received through a copy request

Run it from `nagitui-go` with

```sh
go run ./examples/json-inspector
```

Use the arrow keys to navigate, Enter to toggle a branch, and Control-C to
request the complete selected JSON value. Left-button press selects and toggles
a branch. Press Escape or Q to exit

The example displays the copy request instead of choosing an OS or terminal
clipboard policy
