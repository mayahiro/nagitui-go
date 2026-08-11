# Nagi TUI Go実装

[English](README.md)

Nagi TUI Go実装はnative Cell-based TUI runtime、Unicode対応semantic Node、25個の標準Widget、supervised async work、Subscription、決定的test supportを提供します

## 要件

- Go 1.25以降
- x86-64またはARM64のLinuxとmacOS

## 導入

```sh
go get github.com/mayahiro/nagitui-go@v0.1.0
```

## Quick start

[最小のstateful counter](examples/counter/README.md)を実行します

```sh
go run ./examples/counter
```

## Package

| Package | 責務 |
| --- | --- |
| Module root `tui` | App lifecycle、semantic Node、Scoped KeyMap、layout、event、Effect、Subscription、terminal loop |
| `surface` | Geometry、Cell、Surface描画、composition、diff、snapshot |
| `widget` | Public TUI APIから構築した25個の標準Widget |
| `tuitest` | Virtual input、resize、time、Effect、Subscription、frame検査 |
| `github.com/mayahiro/nagi-go/text` | 共有Unicode 17 text primitive |
| `github.com/mayahiro/nagi-go/vt` | 共有typed terminal input／output、Color、Attributes、Style |

Root packageはapplicationから使いやすくするためSurfaceのGeometry型とVTのStyle型を再公開します。Canonical定義は`surface`と`vt`に維持します

[Nagi semantic specification](https://github.com/mayahiro/nagi/tree/main/spec)がRust実装と共有する挙動を定義します

## Application test

`tuitest` packageは実terminalを使わずにMessage、terminal input、resize、virtual time、Effect、Subscription、frame、activeなresolved actionを操作できます

共有の[event-driven application architecture](https://github.com/mayahiro/nagi/blob/main/docs/EVENT_DRIVEN_APPLICATIONS_ja.md)では、第2のUI loopを作らずprocess outputとtimerをNagiへ渡す方法を説明します

## Example

Go repository rootから実terminalで実行します

| Example | Command |
| --- | --- |
| [Counter](examples/counter/README.md) | `go run ./examples/counter` |
| [Command palette](examples/command-palette/README.md) | `go run ./examples/command-palette` |
| [Async search](examples/async-search/README.md) | `go run ./examples/async-search` |
| [Event-driven log viewer](examples/log-viewer/README.md) | `go run ./examples/log-viewer` |
| [Virtual scroll](examples/virtual-scroll/README.md) | `go run ./examples/virtual-scroll` |
| [Widget gallery](examples/widget-gallery/README.md) | `go run ./examples/widget-gallery` |
| [Extended widget gallery](examples/extended-widget-gallery/README.md) | `go run ./examples/extended-widget-gallery` |
| [Dashboard](examples/dashboard/README.md) | `go run ./examples/dashboard` |
| [Filter付きList](examples/filtered-list/README.md) | `go run ./examples/filtered-list` |
| [File browser](examples/file-browser/README.md) | `go run ./examples/file-browser` |
| [Multi-pane log viewer](examples/multi-pane-log-viewer/README.md) | `go run ./examples/multi-pane-log-viewer` |
| [Form validation](examples/form-validation/README.md) | `go run ./examples/form-validation` |

## 制約

Terminal inputとoutputはterminalへ接続されている必要があります。Mouse reportは既定で無効です。Raw modeとscreen stateは正常return、error、panic経路でbest effortとして復元します。Process abort、nested terminal session、suspendとresume、`/dev/tty`取得には対応していません

`ScrollViewport`はeagerなchild treeをclipしてscrollします。大規模dataでは`VirtualScrollViewport`を使用し、content全体のCell extentを宣言して現在表示する範囲または上限付きoverscanの`VirtualFragment`だけを構築できます。`Node.RevealDescendant`はfocusを移動せずstable descendant IDを表示範囲内に保ち、virtual targetは現在のfragment内に存在する必要があります

`TextArea`はdefaultでno-wrap挙動を維持します。`SoftWrap`はvisual-line navigationを追加し、`BoundaryNavigation`はvisual boundaryのUpとDownをpass-throughへ切り替えられ、`Viewport`はTab stopを増やさずapplication suppliedのcaret IDへ追従します

`Composer`は`TextArea`へcontrolled submitとhistory recall、自動row境界、任意のvalidation content、挿入制限を加えます。Applicationはmessageの意味、history persistence、sensitive value policyを引き続き所有します

`Disclosure`はexpanded stateをApplicationに維持し、expanded時だけbodyを構築します。Core Modal scopeはdefaultでentry時に最初のdescendantへfocusし、close時に以前のfocusへ戻り、両方のtargetを設定できます

`Dialog`はapplication-defined action list、lazy controlled details、明示的なdefaultとcancel target、focus policy、Cell幅によるaction wrappingを構成します。`ConfirmDialog`はdefaultを明示する二action convenienceです

## License

Source codeはMIT Licenseで提供します。生成済みUnicode dataは[Unicode License v3](UNICODE-LICENSE)で配布します
