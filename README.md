# Nagi CLI for Go

[日本語](README_ja.md)

Nagi CLI for Go is a native command-application framework built around a
validated command graph, typed values, injected process services, structured
diagnostics, and explicit exit status

It uses Nagi Text for terminal-cell-aware help alignment and does not depend on
Nagi Surface or Nagi TUI

## Requirements

- Go 1.25 or newer
- Linux or macOS on x86-64 or ARM64 for process integration

## Installation after the v0.2.0 release

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

Public parsing methods accept arguments after the program name. `RunProcess`
reads `os.Args[1:]`, injects standard I/O, environment and current directory,
converts SIGINT into cooperative cancellation, and returns instead of calling
`os.Exit`

## Capabilities

- Long, short, clustered, repeated, required, defaulted, and environment-backed options
- Positional arguments, nested subcommands, aliases, and `--` termination
- Raw byte strings, UTF-8 strings, signed 64-bit integers, finite values, and custom typed parsers
- Deterministic help, stable diagnostic codes, and statuses 0, 1, 2, and 130
- Injected stdin, stdout, stderr, environment, current directory, and `context.Context` cancellation
- Process-free application tests through `github.com/mayahiro/nagicli-go/clitest`

See the shared [CLI semantics](https://github.com/mayahiro/nagi/blob/main/spec/cli.md)
and [public CLI API guide](https://github.com/mayahiro/nagi/blob/main/docs/CLI_API.md)
for the complete contract

## Testing applications

`clitest` injects process inputs and captures status and output without starting
a process or installing signal handlers

```go
result, err := clitest.New(command).
	Arguments("Nagi").
	Environment("LANG", "C").
	CurrentDirectory("/work").
	Run()
```

## Examples

```sh
go run ./examples/basic Nagi
go run ./examples/subcommands start -vv
```

Both examples are built by `make build`

## Development

From the family superproject checkout, run the complete shared-fixture suite

```sh
NAGI_FIXTURES=../fixtures GOWORK=../go.work make check
```

Without `NAGI_FIXTURES`, the conformance tests are skipped while package tests,
examples, formatting, and vet checks still run

## Limitations

The v0.2.0 core does not provide shell completion, configuration-file loading,
interactive prompts, or TUI integration. Handlers are responsible for polling
the injected cancellation context during long-running work

## License

Source code is available under the MIT License
