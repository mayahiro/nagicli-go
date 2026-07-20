# Nagi CLI Go実装

[English](README.md)

`github.com/mayahiro/nagicli-go`はNagi CLIのGo native実装用repositoryです

## 状態

このrepositoryはPhase F0の骨格です。独立moduleとrepository境界だけを定義し、Go packageやCLI実装はまだ提供しません

予定する製品はCommand Graph、typed Invocation、注入可能なContext、structured Diagnostic、Outcome、Exit Statusを中心とするcommand application frameworkです。詳細仕様のreview後にだけ実装を開始します

Nagi CLIは`github.com/mayahiro/nagi-go/text`と`github.com/mayahiro/nagi-go/vt`へ依存できます。Nagi SurfaceやNagi TUIへは依存しません

## 開発

```sh
make check
```

現在のcommandは空moduleをbuild可能な状態に保つ確認であり、public APIの実装済みを意味しません

## License

Source codeはMIT Licenseで提供します
