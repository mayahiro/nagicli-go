// Sensitive Value redaction preserves explicit handler access
package main

import (
	"fmt"
	"os"

	cli "github.com/mayahiro/nagicli-go"
)

func application() *cli.Command {
	return cli.NewCommand("sensitive-values").
		About("Demonstrate generic Sensitive Value metadata").
		Option(
			cli.ValueOption("token").
				Long("token").
				Parser(cli.StringParser()).
				Environment("NAGI_TOKEN").
				Default("demo-token").
				Sensitive().
				Help("Authentication token"),
		).
		Handle(runCommand)
}

func runCommand(context *cli.Context, invocation *cli.Invocation) (cli.Outcome, error) {
	parsed := invocation.ParsedValues("token")[0]
	source := "default"
	switch parsed.Source() {
	case cli.SourceCommandLine:
		source = "command line"
	case cli.SourceEnvironment:
		source = "environment"
	case cli.SourceExternal:
		source = "external"
	}
	if _, err := fmt.Fprintf(
		context.Stdout(),
		"received a %d-byte token from %s; sensitive=%t\n",
		len(parsed.Raw()),
		source,
		parsed.IsSensitive(),
	); err != nil {
		return cli.Outcome{}, cli.NewDiagnostic(cli.CodeIOError, err.Error())
	}
	return cli.Success(), nil
}

func main() {
	status, err := application().RunProcess()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(int(cli.StatusFailure))
	}
	os.Exit(int(status))
}
