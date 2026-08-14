# Lightweight prompt example

This example runs line-oriented Confirm, Select, Input, and Secret requests
inside a Nagi CLI handler. The handler passes its `context.Context` to every
request, so `Command.RunProcess` maps Ctrl-C to a structured cancelled outcome

Run it from the Go module root in an interactive terminal

```sh
go run ./examples/prompt
```

Prompts use standard input and standard error. The final non-secret result uses
standard output. Secret input temporarily disables terminal echo and restores
the complete saved terminal state before returning
