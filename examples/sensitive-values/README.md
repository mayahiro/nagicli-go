# Sensitive Value example

This example marks a generic Value option as Sensitive. Nagi redacts the
configured default in generated Help while preserving explicit raw and typed
access for the handler

Run Help from the Go CLI repository root:

```sh
go run ./examples/sensitive-values --help
```

The option description contains `default: <redacted>` and still shows the
environment variable name because it is metadata rather than a resolved value

Run the handler without printing the value:

```sh
go run ./examples/sensitive-values --token demo-token
```

The handler reports the source, byte length, and Sensitive marker. It obtains
the original value through explicit Invocation access and deliberately does
not write it to output

Sensitive metadata also redacts framework-generated parser failures and Go
formatting, and suppresses finite-value and dynamic completion for that target.
It does not remove argv from process inspection or prevent application code
from logging a value that it explicitly reads
