# Nagi TUI Go実装

[English](README.md)

Nagi TUI Go実装は、セルベースのターミナルUIフレームワークNagi TUIのGoネイティブ実装です

宣言的application runtime、Unicode対応semantic Node、21個の標準Widget、supervised async work、Subscription、決定的test harnessを提供します

## 要件

- Go 1.25以降
- x86-64またはARM64のLinuxとmacOS

## 最初のrelease後のInstallation

v0.1.0の公開後にmoduleをapplicationへ追加します

```sh
go get github.com/mayahiro/nagitui-go@v0.1.0
```

Tagはrelease済みrepository revisionを選択します。選択されたmodule versionは`go.mod`と`go.sum`へ記録されます

## Quick start

次のcounterはEnterでapplication stateを更新し、Escapeで終了します

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

`App`はstateを所有し、`Update`でMessageを1個ずつ処理し、`Subscriptions`で長期sourceを宣言し、`View`でapplication stateと`ViewContext`からsemantic `Node` treeを再構築します。`ExitEffect`はterminal復元前に最後のdirty viewを描画します。`RunTerminalContext`は外部`context.Context` cancellationを追加します。`Init`と`Subscriptions`を使わない場合も明示的なno-work valueを返します

## Packageと機能

| Package | 用途 |
| --- | --- |
| Module root `tui` | App lifecycle、runtime、Core Node、layout、event、interaction、Effect、Subscription、terminal loop |
| `github.com/mayahiro/nagitui-go/surface` | Geometry、Cell、Surface描画、composition、diff、snapshot |
| `github.com/mayahiro/nagitui-go/widget` | 21個の標準Widget |
| `github.com/mayahiro/nagitui-go/tuitest` | Virtual input、resize、time、Effect、Subscription、frame検査 |
| `github.com/mayahiro/nagi-go/text` | 共有Unicode 17 grapheme、terminal幅profile、wrap、truncate、位置変換 |
| `github.com/mayahiro/nagi-go/vt` | 共有typed terminal input／output、Color、Attributes、Style |

Rootの`tui` packageはapplicationから使いやすくするため、Surface所有のGeometry型とVT所有のStyle型を再公開します。Canonical定義は`surface`と共有`vt` packageに維持します

Core compositionはText、RichText、Paragraph、安全なANSI SGR text、SurfaceNode、TextInput、Spacer、Gap、Row、Column、Stack、Padding、Border、Panel、Align、Clip、ScrollViewport、Modalを提供します

`widget` packageはList、Button、Modal、Progress、Spinner、Scrollbar、TextArea、Table、Tree、Tabs、Checkbox、Radio、Select、Command Palette、Sparkline、BarChart、Chart、Help、Paginator、FilePicker、Calendarを提供します

## Application test

`tuitest` packageは実terminalを使わずにMessage、terminal input、resize、virtual time、controlled Effect、manual Subscriptionを操作できます。FrameとMessage history、Interaction State、supervisor diagnostic、canonical Surface snapshotも検査できます

## Example

Repository rootから実terminalで実行します

| Example | Command |
| --- | --- |
| [Command palette](examples/command-palette/README.md) | `go run ./examples/command-palette` |
| [Async search](examples/async-search/README.md) | `go run ./examples/async-search` |
| [Log viewer](examples/log-viewer/README.md) | `go run ./examples/log-viewer` |
| [Widget gallery](examples/widget-gallery/README.md) | `go run ./examples/widget-gallery` |
| [Extended widget gallery](examples/extended-widget-gallery/README.md) | `go run ./examples/extended-widget-gallery` |
| [Dashboard](examples/dashboard/README.md) | `go run ./examples/dashboard` |
| [Filter付きList](examples/filtered-list/README.md) | `go run ./examples/filtered-list` |
| [File browser](examples/file-browser/README.md) | `go run ./examples/file-browser` |
| [Multi-pane log viewer](examples/multi-pane-log-viewer/README.md) | `go run ./examples/multi-pane-log-viewer` |
| [Form validation](examples/form-validation/README.md) | `go run ./examples/form-validation` |

## Terminalの挙動と制約

Terminalのtext selectionを維持するため、mouse reportは既定で無効です。Pointer inputが必要なapplicationでは`TerminalOptions`で有効にしてください

Standard inputとoutputはterminalへ接続されている必要があります。Raw modeとscreen stateは正常return、error、panic経路でbest effortとして復元します。Process abort、nested terminal session、suspendとresume、`/dev/tty`取得には対応していません

## ライセンス

Nagi TUI Go実装のsource codeはMIT Licenseで提供します。生成済みUnicode dataは[Unicode License v3](UNICODE-LICENSE)で配布します
