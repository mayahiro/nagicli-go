// Command json-diagnostic renders one structured Diagnostic as stable JSON
package main

import (
	"fmt"
	"io"
	"os"

	cli "github.com/mayahiro/nagicli-go"
)

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(output io.Writer) error {
	diagnostic := cli.NewDiagnostic(
		cli.DiagnosticCode("profile-blocked"),
		"profile is not available",
	).
		WithCategory(cli.CategoryUsage).
		WithCommandPath([]string{"nagi", "deploy"}).
		WithUsage("nagi deploy --profile <PROFILE>").
		WithTarget(cli.OptionTarget("profile").WithCommandIDPath("root", "deploy")).
		WithHint("choose an available profile")
	policy := cli.DefaultRuntimePolicy().WithDiagnosticRenderer(cli.JSONDiagnosticRenderer{})
	_, err := io.WriteString(output, policy.RenderDiagnostic(diagnostic))
	return err
}
