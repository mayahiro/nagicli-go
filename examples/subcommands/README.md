# Nested subcommands

This example requires a `start` subcommand and uses a repeatable `-v` count
option

Run it from the Go repository root:

```sh
go run ./examples/subcommands start -vv
```

It prints `starting with verbosity 2`. Pass `--help` before or after the
subcommand, or run `go run ./examples/subcommands help start`, to inspect the
generated help for that command path
