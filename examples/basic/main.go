// Command basic is a minimal Nagi CLI application
package main

import (
	"fmt"
	"os"

	cli "github.com/mayahiro/nagicli-go"
)

func application() *cli.Command {
	return cli.NewCommand("greet").
		About("Print a greeting").
		Version("0.2.0").
		Argument(cli.Positional("name").Parser(cli.StringParser()).Required().Help("Name to greet")).
		Example("named greeting", "greet Nagi").
		Note("Help and diagnostics are written separately from command output").
		Link("guide", "https://github.com/mayahiro/nagi/blob/main/docs/CLI_API.md").
		Handle(func(context *cli.Context, invocation *cli.Invocation) (cli.Outcome, error) {
			name, ok := cli.ValueAs[string](invocation, "name")
			if !ok {
				return cli.Outcome{}, cli.NewDiagnostic(cli.CodeHandlerError, "name is unavailable")
			}
			if _, err := fmt.Fprintf(context.Stdout(), "Hello, %s!\n", name); err != nil {
				return cli.Outcome{}, cli.NewDiagnostic(cli.CodeIOError, err.Error())
			}
			return cli.Success(), nil
		})
}

func run() int {
	status, err := application().RunProcess()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return int(cli.StatusFailure)
	}
	return int(status)
}

func main() {
	os.Exit(run())
}
