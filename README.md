# Nagi CLI for Go

[日本語](README_ja.md)

Nagi CLI for Go provides a native command-application framework with a
validated Command Graph, typed values, injected process services, structured
Help and Diagnostics, and policy-controlled Exit Status

It uses Nagi Text for terminal-Cell-aware help alignment and does not depend on
Nagi Surface or Nagi TUI

## Requirements

- Go 1.25 or newer
- Linux or macOS on x86-64 or ARM64 for process integration

## Installation

```sh
go get github.com/mayahiro/nagicli-go@v0.3.1
```

## Quick start

Run the [basic command example](examples/basic/README.md):

```sh
go run ./examples/basic Nagi
```

## Capabilities

- Long, short, clustered, repeated, required, defaulted, and environment-backed options
- Positional arguments, nested subcommands, aliases, and `--` termination
- Raw byte strings, UTF-8 strings, signed 64-bit integers, finite values, and custom typed parsers
- Source-aware option relations, four portable option-group rules, and typed Invocation validators
- Structured deterministic Help, stable Usage Variants, custom sections and renderers, and `help [COMMAND...]`
- Stable Diagnostic codes and categories with configurable rendering and exit-code mapping
- Injected stdin, stdout, stderr, environment, current directory, and `context.Context` cancellation
- Process-free application tests through package `clitest`

The shared [CLI semantics](https://github.com/mayahiro/nagi/blob/main/spec/cli.md)
define the observable contract and Rust parity

## Testing applications

Package `clitest` injects process inputs and captures status and output without
starting a process or installing signal handlers. The basic example includes an
[executable application test](examples/basic/main_test.go)

```sh
go test ./examples/basic
```

## Examples

| Example | Command |
| --- | --- |
| [Basic command](examples/basic/README.md) | `go run ./examples/basic Nagi` |
| [Nested subcommands](examples/subcommands/README.md) | `go run ./examples/subcommands start -vv` |

Both examples are included in `go build ./...`

## Limitations

Shell completion, configuration-file loading, interactive prompts, and TUI
integration are not provided. Long-running handlers must poll the injected
cancellation context cooperatively. The portable graph does not model
arbitrary invocation grammars. Help-only Usage Variants can document
validator-backed forms without changing parser semantics

## License

Source code is available under the MIT License
