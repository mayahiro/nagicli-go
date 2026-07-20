# Nagi CLI Go実装

[English](README.md)

Nagi CLI Go実装は、検証済みCommand Graph、typed value、注入可能なprocess service、structured Diagnostic、明示的なExit Statusを中心とするnative command application frameworkです

Helpのterminal Cell幅計算にNagi Textを使用し、Nagi SurfaceとNagi TUIには依存しません

## 要件

- Go 1.25以降
- Process統合はx86-64またはARM64のLinuxとmacOS

## v0.2.0 release後の導入

```sh
go get github.com/mayahiro/nagicli-go@v0.2.0
```

## Quick start

```go
package main

import (
	"fmt"
	"os"

	cli "github.com/mayahiro/nagicli-go"
)

func main() {
	command := cli.NewCommand("greet").
		About("Print a greeting").
		Argument(cli.Positional("name").Parser(cli.StringParser()).Required()).
		Handle(func(context *cli.Context, invocation *cli.Invocation) (cli.Outcome, error) {
			name, _ := cli.ValueAs[string](invocation, "name")
			if _, err := fmt.Fprintf(context.Stdout(), "Hello, %s!\n", name); err != nil {
				return cli.Outcome{}, cli.NewDiagnostic(cli.CodeIOError, err.Error())
			}
			return cli.Success(), nil
		})

	status, err := command.RunProcess()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(int(cli.StatusFailure))
	}
	os.Exit(int(status))
}
```

Public parserはprogram nameを除いたargumentを受け取ります

`RunProcess`は`os.Args[1:]`、standard I/O、environment、current directoryを使用し、SIGINTを協調的cancellationへ変換しますが、`os.Exit`は呼びません

## 機能

- Long、short、cluster、repeated、required、default、environment fallback付きoption
- Positional argument、nested subcommand、alias、`--` terminator
- Raw byte string、UTF-8 string、signed 64-bit integer、finite value、custom typed parser
- 決定的Help、安定Diagnostic code、0、1、2、130のExit Status
- 注入可能なstdin、stdout、stderr、environment、current directory、`context.Context` cancellation
- `github.com/mayahiro/nagicli-go/clitest`によるprocessなしのapplication test

完全な契約は共有[CLI semantics](https://github.com/mayahiro/nagi/blob/main/spec/cli.md)と[public CLI API guide](https://github.com/mayahiro/nagi/blob/main/docs/CLI_API_ja.md)を参照してください

## Application test

`clitest`はprocess起動やsignal handler設定を行わず、process inputを注入してstatusとoutputを取得します

```go
result, err := clitest.New(command).
	Arguments("Nagi").
	Environment("LANG", "C").
	CurrentDirectory("/work").
	Run()
```

## Example

```sh
go run ./examples/basic Nagi
go run ./examples/subcommands start -vv
```

両exampleは`make build`でbuildされます

## 開発

Family superprojectのcheckoutから共有fixtureを含む全確認を実行します

```sh
NAGI_FIXTURES=../fixtures GOWORK=../go.work make check
```

`NAGI_FIXTURES`を指定しない場合はconformance testだけをskipし、package test、example、format、vetは実行します

## 制約

v0.2.0 coreはshell completion、設定file読み込み、interactive prompt、TUI統合を提供しません

長時間実行するHandlerは注入されたcancellation contextを協調的に確認する必要があります

## License

Source codeはMIT Licenseで提供します
