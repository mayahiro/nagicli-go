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
go get github.com/mayahiro/nagicli-go@v0.3.2
```

## Quick start

Run the [basic command example](examples/basic/README.md):

```sh
go run ./examples/basic Nagi
```

## Capabilities

- Local and explicitly inherited long, short, clustered, repeated, required, defaulted, and environment-backed options
- Positional arguments, nested subcommands, aliases, and `--` termination
- Raw byte strings, UTF-8 strings, signed 64-bit integers, finite values, and custom typed parsers
- Source-aware option relations, four portable option-group rules, and typed Invocation validators
- Command-local value IDs, exact stable-ID scopes, and fallible required typed access
- Structured deterministic Help, controllable subcommand Usage Variants, custom sections and renderers, and `help [COMMAND...]`
- Stable Diagnostic codes, categories, value targets, and hints with plain or stable JSON rendering and configurable exit-code mapping
- Injected stdin, stdout, stderr, environment, current directory, and `context.Context` cancellation
- Parser-first dispatch and parsed-Invocation execution for command-by-command adoption
- Immutable handler-free completion resolution, dynamic providers, and Bash, Zsh, Fish, and PowerShell generators
- Optional line-oriented Confirm, Select, Input, and Secret prompts with injected I/O
- Optional synchronous TTY status, spinner, progress, and plain-log fallback with injected I/O
- Process-free application tests through package `clitest`

The shared [CLI semantics](https://github.com/mayahiro/nagi/blob/main/spec/cli.md)
define the observable contract and Rust parity. The
[public CLI API guide](https://github.com/mayahiro/nagi/blob/main/docs/CLI_API.md)
explains inherited options, command-local scopes, completion, Help
presentation, structured validators, and staged adoption

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
| [Staged adoption](examples/staged/README.md) | `go run ./examples/staged inspect page` |
| [JSON Diagnostic](examples/json-diagnostic/README.md) | `go run ./examples/json-diagnostic` |
| [Shell completion](examples/completion/README.md) | `go run ./examples/completion generate bash` |
| [Lightweight prompts](examples/prompt/README.md) | `go run ./examples/prompt` |
| [TTY-aware status](examples/status/README.md) | `go run ./examples/status` |

All examples are included in `go build ./...`

## Limitations

Shell-specific generation, line-oriented Prompt, and synchronous Status
Reporter are optional, and applications own completion installation, dynamic
candidate I/O, credential handling, approval policy, status timing, and
progress meaning. Configuration-file loading and CLI-to-TUI
integration are not provided. Long-running handlers and completion
providers must poll the injected cancellation context cooperatively. The
portable graph does not model arbitrary invocation grammars. Help-only Usage
Variants can document validator-backed forms without changing parser semantics

## License

Source code is available under the MIT License
