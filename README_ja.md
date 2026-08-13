# Nagi TUI Go実装

[English](README.md)

Nagi TUI Go実装はimmutableなTerminal Presentation Rules、上限付きContentからNodeへのprojection、native Cell-based TUI runtime、Unicode対応semantic Node、31個の標準Widget、supervised async work、Subscription、決定的test supportを提供します

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
| Module root `tui` | Terminal Presentation Rules、上限付きContentからNodeへのprojection、App lifecycle、semantic Node、Scoped KeyMap、layout、event、Effect、Subscription、terminal loop |
| `surface` | Geometry、Cell、Surface描画、composition、diff、snapshot |
| `widget` | Public TUI APIから構築した31個の標準Widget |
| `tuitest` | Virtual input、resize、time、Effect、Subscription、frame検査 |
| `github.com/mayahiro/nagi-go/content` | Presentation Rulesとprojectionが使用する共有source-neutral Content |
| `github.com/mayahiro/nagi-go/text` | 共有Unicode 17 text primitive |
| `github.com/mayahiro/nagi-go/vt` | 共有typed terminal input／output、Color、Attributes、Style |

Root packageはapplicationから使いやすくするためSurfaceのGeometry型とVTのStyle型を再公開します。Canonical定義は`surface`と`vt`に維持します

[Nagi semantic specification](https://github.com/mayahiro/nagi/tree/main/spec)がRust実装と共有する挙動を定義します

## Application test

`tuitest` packageは実terminalを使わずにMessage、terminal input、resize、virtual time、Effect、Subscription、pending terminal taskとclipboard request、Runtime notice、frame、activeなresolved actionを操作できます

共有の[event-driven application architecture](https://github.com/mayahiro/nagi/blob/main/docs/EVENT_DRIVEN_APPLICATIONS_ja.md)では、第2のUI loopを作らずprocess outputとtimerをNagiへ渡す方法を説明します

`RuntimeConfig.WidthProfile`と`TerminalOptions.WidthProfile`はCoreのmeasure、render、hit geometry、cursor配置で使うcell幅policyを1個選択します。幅計算を行うWidget builderには`ViewContext.WidthProfile`を渡します。予期しないasync lifecycle transitionは上限付きRuntime notice queueまたはterminal notice-handler entry pointから観測できます

`SuspendTerminalEffect`は標準runnerが通常terminalを復元して設定済みviewportを離れた後にApplication所有のblocking taskを実行します。Task return後はfull-screen viewportを再開するか新しいinline領域を確保し、pending decoder stateをresetしてfull redrawを強制します

`NewInlineTerminalViewport(height)`は同じRuntimeをmain screenの上限付き領域で実行し、最終frameをterminal historyへ残します。標準runnerがcursor取得、resize配置、座標変換、復元を所有します

## Example

Go repository rootから実terminalで実行します

| Example | Command |
| --- | --- |
| [Presentation RulesとContent projection](examples/presentation/README.md) | `go run ./examples/presentation` |
| [Counter](examples/counter/README.md) | `go run ./examples/counter` |
| [Command palette](examples/command-palette/README.md) | `go run ./examples/command-palette` |
| [Async search](examples/async-search/README.md) | `go run ./examples/async-search` |
| [Suggestion popup](examples/suggestion-popup/README.md) | `go run ./examples/suggestion-popup` |
| [JSON inspector](examples/json-inspector/README.md) | `go run ./examples/json-inspector` |
| [Code view](examples/code-view/README.md) | `go run ./examples/code-view` |
| [Diff view](examples/diff-view/README.md) | `go run ./examples/diff-view` |
| [Event-driven log viewer](examples/log-viewer/README.md) | `go run ./examples/log-viewer` |
| [Terminal suspendとresume](examples/terminal-suspend/README.md) | `go run ./examples/terminal-suspend` |
| [Inline terminal viewport](examples/inline-terminal/README.md) | `go run ./examples/inline-terminal` |
| [Virtual scroll](examples/virtual-scroll/README.md) | `go run ./examples/virtual-scroll` |
| [Variable-height feed](examples/virtual-feed/README.md) | `go run ./examples/virtual-feed` |
| [Widget gallery](examples/widget-gallery/README.md) | `go run ./examples/widget-gallery` |
| [Extended widget gallery](examples/extended-widget-gallery/README.md) | `go run ./examples/extended-widget-gallery` |
| [Dashboard](examples/dashboard/README.md) | `go run ./examples/dashboard` |
| [Filter付きList](examples/filtered-list/README.md) | `go run ./examples/filtered-list` |
| [File browser](examples/file-browser/README.md) | `go run ./examples/file-browser` |
| [Multi-pane log viewer](examples/multi-pane-log-viewer/README.md) | `go run ./examples/multi-pane-log-viewer` |
| [Form validation](examples/form-validation/README.md) | `go run ./examples/form-validation` |

## 制約

Terminal inputとoutputはterminalへ接続されている必要があります。Mouse reportは既定で無効です。Raw modeとscreen stateは正常return、error、panic経路でbest effortとして復元します。Applicationが要求する一時的なterminal suspendには対応します。Process abort、nested terminal session、Nagi processのjob-control suspend、`/dev/tty`取得には対応していません

`ScrollViewport`はeagerなchild treeをclipしてscrollします。大規模dataでは`VirtualScrollViewport`を使用し、content全体のCell extentを宣言して現在表示する範囲または上限付きoverscanの`VirtualFragment`だけを構築できます。`Node.RevealDescendant`はfocusを移動せずstable descendant IDを表示範囲内に保ち、virtual targetは現在のfragment内に存在する必要があります

`VirtualFlow`は可変item heightとstable anchorをappend、prepend、削除、streaming更新、幅変更にまたがって保持し、visible fragmentとCell単位の上限付きoverscanだけを構築します。Intrinsic高は0のためlayout `Length`を割り当てます。`widget.VirtualFeed`はdomain stateを所有せず、末尾追従とApplication制御のempty、loading、unread slotを追加します

`TextArea`はdefaultでno-wrap挙動を維持します。`SoftWrap`はvisual-line navigationを追加し、`BoundaryNavigation`はvisual boundaryのUpとDownをpass-throughへ切り替えられ、`Viewport`はTab stopを増やさずapplication suppliedのzero-width typed cursor anchorへ追従します。Cursorはcaret graphemeを描かず後続textを移動しません

`Composer`は`TextArea`へcontrolled submitとhistory recall、自動row境界、任意のvalidation content、挿入制限を加えます。Applicationはmessageの意味、history persistence、sensitive value policyを引き続き所有します

`widget.SuggestionPopup`はgeneric `AnchoredOverlay`へcontrolled stable candidate ID、selected rowを含むbounded window、差し替え可能なloadingとempty content、semantic action、focusを維持するpointer activationを組み合わせます。Query解析、ranking、async Effect、cancellation、acceptの意味はApplicationが所有します

`SelectableText`はimmutableなstyled contentへgrapheme境界に揃えたcontrolled keyboardと左button drag selectionを加えます

Stable IDによるpointer captureはcontrolled view再構築後も継続し、dragは最も近いviewportへ1 Cell単位のedge scrollを要求できます

Copy actionはApplication Messageを発行します。Applicationは`SetClipboardEffect`を返すことができ、`TerminalClipboardOSC52`はwrite-only terminal backendを明示的に有効化します。Redaction policy、terminal support検出、OS固有clipboard commandはWidgetの外側に維持します

`widget.JSONInspector`はimmutableなtyped `JSONDocument`をbounded row構築とgrapheme境界を保つscalar previewを持つcontrolled treeへ投影します。Copy requestは完全なcompact valueを保持し、parser、schema validation、redaction、clipboard policy、domain上の意味はApplicationが所有します

`widget.CodeView`はApplicationがstyleを付けたimmutableなlogical lineをmemo化したterminal幅layoutで投影します。Tab、wrap、line number、行選択、横scroll、上限付きNode構築、完全な行単位copyをsyntax parser、file、diffの意味、clipboard I/Oから独立させます

`widget.DiffView`はimmutableなtyped metadata、hunk、context、addition、deletion lineを受け取ります。上限付きCode projectionを再利用しながらold／new line number、unified marker、semantic style、要求時のunified copyを追加します。Diff parse、repository access、patch apply、approval policy、clipboard I/OはApplicationが所有します

Core `SplitPane`はhorizontalまたはverticalな二paneを1 Cellのdivider、paneごとのminimum、basis-point ratio、決定的なcollapse targetで割り当てます。省略されたpaneはrender、semantic routing、focus、lazy virtual preparationの対象外です。`widget.SplitPane`はpaneの意味を定義せず、controlledなF6 focus移動、axisに対応するkeyboard resize、divider dragを追加します

`widget.Drawer`はcontrolledなopen stateがtrueの間だけbodyを構築してviewport edgeへ配置し、defaultではCore Modalのfocusとroutingを再利用します。Open stateの永続化、outside-click挙動、drawer contentの意味はApplicationが所有します

`Disclosure`はexpanded stateをApplicationに維持し、expanded時だけbodyを構築します。Core Modal scopeはdefaultでentry時に最初のdescendantへfocusし、close時に以前のfocusへ戻り、両方のtargetを設定できます。Modalがunhandled raw Eventとterminal fallback mappingも止める必要がある場合は`Node.BlockUnhandledEvents`でopt-inのhard input boundaryを追加します

`Dialog`はapplication-defined action list、lazy controlled details、明示的なdefaultとcancel target、focus policy、Cell幅によるaction wrappingを構成します。`ConfirmDialog`はdefaultを明示する二action convenienceです

## License

Source codeはMIT Licenseで提供します。生成済みUnicode dataは[Unicode License v3](UNICODE-LICENSE)で配布します
