package cli_test

import (
	"strings"
	"testing"

	cli "github.com/mayahiro/nagicli-go"
)

func BenchmarkJSONDiagnosticRenderer(b *testing.B) {
	b.Run("Structured", func(b *testing.B) {
		benchmarkJSONDiagnosticRenderer(b, benchmarkJSONDiagnostic())
	})
	b.Run("64KiBMessage", func(b *testing.B) {
		benchmarkJSONDiagnosticRenderer(
			b,
			cli.NewDiagnostic(cli.CodeHandlerError, strings.Repeat("x", 65_536)),
		)
	})
}

func benchmarkJSONDiagnosticRenderer(b *testing.B, diagnostic *cli.Diagnostic) {
	renderer := cli.JSONDiagnosticRenderer{}
	warmup := renderer.RenderDiagnostic(diagnostic)
	if !strings.HasSuffix(warmup, "}\n") {
		b.Fatalf("JSON = %q", warmup)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = renderer.RenderDiagnostic(diagnostic)
	}
}

func benchmarkJSONDiagnostic() *cli.Diagnostic {
	return cli.NewDiagnostic(cli.DiagnosticCode("profile-blocked"), `profile "prod" blocked`).
		WithCategory(cli.CategoryUsage).
		WithCommandPath([]string{"nagi", "deploy"}).
		WithUsage("nagi deploy --profile <PROFILE>").
		WithTarget(cli.OptionTarget("profile").WithCommandIDPath("root", "deploy")).
		WithTarget(cli.ArgumentTarget("target").WithCommandIDPath("root", "deploy")).
		WithHint("choose staging").
		WithHint("inspect the provider configuration")
}
