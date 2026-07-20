# Nagi CLI for Go

[日本語](README_ja.md)

`github.com/mayahiro/nagicli-go` is reserved for the native Go implementation
of Nagi CLI

## Status

This repository is a Phase F0 scaffold. It defines the independent module and
repository boundary, but it does not yet provide a Go package or CLI
implementation

The planned product is a command-application framework centered on a Command
Graph, typed Invocation, injected Context, structured Diagnostic, Outcome, and
Exit Status. Implementation begins only after the detailed specification is
reviewed

Nagi CLI may depend on `github.com/mayahiro/nagi-go/text` and
`github.com/mayahiro/nagi-go/vt`. It must not depend on Nagi Surface or Nagi TUI

## Development

```sh
make check
```

The command currently verifies that the empty module remains buildable and
does not imply an implemented public API

## License

Source code is available under the MIT License
