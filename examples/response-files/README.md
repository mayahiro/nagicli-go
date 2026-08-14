# Response File example

This example enables bounded Response File expansion explicitly through
`ProcessOptions`. Ordinary `RunProcess` and parser entry points continue to
treat leading `@` literally

Run it from the Go CLI repository root:

```sh
go run ./examples/response-files @examples/response-files/arguments.txt
```

Expected output:

```text
profile=workspace
tags=alpha|release candidate
```

The tracked argument file demonstrates comments, ordinary tokens, and double
quotes. Nagi performs no shell, variable, glob, tilde, or environment
expansion. Relative nested files resolve from the including file. Use `@@name`
for a literal `@name`

Standard input through `@-` remains disabled unless
`ResponseFileOptions.WithStandardInput(true)` is selected. Production
filesystem reads use `FilesystemResponseFileReader`; embedded applications and
tests can inject another `ResponseFileReader`
