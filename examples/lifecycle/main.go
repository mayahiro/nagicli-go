// Command lifecycle hides internal syntax and reports deprecated syntax
package main

import (
	"fmt"
	"os"
	"strings"

	cli "github.com/mayahiro/nagicli-go"
)

func application() *cli.Command {
	return cli.NewCommand("lifecycle").
		About("Demonstrate command and option lifecycle metadata").
		Option(
			cli.Flag("legacy").
				Long("legacy").
				Deprecated("--verbose").
				Help("Use legacy output"),
		).
		Option(
			cli.Flag("internal").
				Long("internal").
				Hidden().
				Help("Internal switch"),
		).
		Subcommand(
			cli.NewCommand("old").
				About("Run the old entry point").
				Deprecated("lifecycle run").
				Handle(runCommand),
		).
		Subcommand(
			cli.NewCommand("internal").
				About("Run internal maintenance").
				Hidden().
				Handle(runCommand),
		).
		Subcommand(
			cli.NewCommand("run").
				About("Run the current entry point").
				Handle(runCommand),
		)
}

func runCommand(context *cli.Context, invocation *cli.Invocation) (cli.Outcome, error) {
	legacy, _ := invocation.Flag("legacy")
	if _, err := fmt.Fprintf(
		context.Stdout(),
		"running %s with legacy=%t\n",
		strings.Join(invocation.CommandPath(), " "),
		legacy,
	); err != nil {
		return cli.Outcome{}, cli.NewDiagnostic(cli.CodeIOError, err.Error())
	}
	return cli.Success(), nil
}

func run() int {
	policy := cli.DefaultRuntimePolicy().
		WithDeprecationNoticeRenderer(cli.PlainDeprecationNoticeRenderer{})
	status, err := application().RunProcessWithPolicy(policy)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return int(cli.StatusFailure)
	}
	return int(status)
}

func main() {
	os.Exit(run())
}
