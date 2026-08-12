# Nested subcommands

This example requires a `start` subcommand, expands its direct Usage Variant in
parent Help, makes root `--verbose` inherited, reuses the local `profile` value
ID in root and child scopes, and returns a structured validator Diagnostic for
a reserved profile

Run it from the Go repository root:

```sh
go run ./examples/subcommands start -vv
```

It prints
`starting profile service from root default with verbosity 2`. The inherited
`-v` is accepted after `start` and remains in the root scope. Local `--profile`
values before and after child selection belong to different scopes:

```sh
go run ./examples/subcommands \
  --profile platform start --profile canary -vv
```

Pass `--help` before or after the subcommand, or run
`go run ./examples/subcommands help start`, to inspect the generated Help.
Pass `start --profile blocked` to inspect the application Diagnostic code,
target, hint, Usage category, and default rendering
