# TTY-aware status example

This example drives a transient spinner and determinate progress from ordinary
application state

Run it from the Go CLI repository root:

```sh
go run ./examples/status
```

On a terminal, updates replace one line on standard error and reserve the last
terminal Cell to avoid autowrap. When standard error is redirected, the same
Snapshots become newline-delimited plain logs without terminal controls or
spinner-frame noise

`Reporter` does not start a timer or goroutine. The application owns update
timing, cancellation, progress meaning, and serialization with other
standard-error output. Use `Reporter.Log` to write a permanent line while
preserving an active transient status
