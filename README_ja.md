# Nagi CLI Go実装

[English](README.md)

Nagi CLI Go実装は検証済みCommand Graph、typed value、注入可能なprocess service、structured HelpとDiagnostic、policyで制御するExit Statusを持つnative command application frameworkを提供します

Helpのterminal Cell幅計算にNagi Textを使用し、Nagi SurfaceとNagi TUIには依存しません

## 要件

- Go 1.25以降
- Process統合はx86-64またはARM64のLinuxとmacOS

## 導入

```sh
go get github.com/mayahiro/nagicli-go@v0.3.2
```

## Quick start

[Basic command example](examples/basic/README.md)を実行します

```sh
go run ./examples/basic Nagi
```

## 機能

- Localまたは明示的に継承するlong、short、cluster、repeated、required、default、environment fallback付きoption
- Positional argument、nested subcommand、alias、`--` terminator
- Raw byte string、UTF-8 string、signed 64-bit integer、finite value、custom typed parser
- Source-aware option relation、4種類のportable option-group rule、typed Invocation validator
- Command-local value ID、stable IDによるexact scope、失敗理由を返すrequired typed access
- Structured deterministic Help、制御可能なsubcommand Usage Variant、custom sectionとrenderer、`help [COMMAND...]`
- 安定Diagnostic code、category、value target、hint、設定可能なrenderingとexit-code mapping
- 注入可能なstdin、stdout、stderr、environment、current directory、`context.Context` cancellation
- Command単位の段階導入に使うparser-first dispatchとparsed Invocation実行
- Handlerを含まないimmutable completion解決、dynamic provider、Bash、Zsh、Fish、PowerShell generator
- 注入可能なI/Oを持つ任意の行指向Confirm、Select、Input、Secret
- `clitest` packageによるprocessなしのapplication test

共有[CLI semantics](https://github.com/mayahiro/nagi/blob/main/spec/cli.md)が観測可能な契約とRust parityを定義します

[Public CLI API guide](https://github.com/mayahiro/nagi/blob/main/docs/CLI_API_ja.md)では継承Option、command-local scope、completion、Help presentation、structured validator、段階導入を説明します

## Application test

`clitest` packageはprocess起動やsignal handler設定を行わずにprocess inputを注入してstatusとoutputを取得します。Basic exampleには[実行可能なapplication test](examples/basic/main_test.go)があります

```sh
go test ./examples/basic
```

## Example

| Example | Command |
| --- | --- |
| [Basic command](examples/basic/README.md) | `go run ./examples/basic Nagi` |
| [Nested subcommands](examples/subcommands/README.md) | `go run ./examples/subcommands start -vv` |
| [段階導入](examples/staged/README.md) | `go run ./examples/staged inspect page` |
| [Shell completion](examples/completion/README.md) | `go run ./examples/completion generate bash` |
| [軽量prompt](examples/prompt/README.md) | `go run ./examples/prompt` |

全exampleは`go build ./...`の対象です

## 制約

Shell固有生成と行指向Promptは任意packageです

Completion installation、dynamic candidate I/O、credential管理、approval policyはApplicationが所有します

設定file読み込みとCLIからTUIへの統合は提供しません

長時間実行するHandlerとcompletion providerは注入されたcancellation contextを協調的に確認する必要があります

Portable graphは任意のinvocation grammarを表現しません

Help-only Usage Variantはparser semanticsを変更せずにvalidatorで支えるformを記述できます

## License

Source codeはMIT Licenseで提供します
