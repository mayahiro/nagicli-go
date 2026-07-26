# Basic command

This example defines one required String positional value, writes through the
injected Context, and returns an explicit Exit Status

Run it from the Go repository root:

```sh
go run ./examples/basic Nagi
```

It prints `Hello, Nagi!`. Pass `--help` to inspect the named Help-only Usage
Variant, structured example, note, and link rendered with the command help

The adjacent [`main_test.go`](main_test.go) demonstrates the same application
through the process-free `clitest` driver
