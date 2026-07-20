// Command subcommands demonstrates a nested Nagi CLI application
package main

import (
	"fmt"
	"os"

	cli "github.com/mayahiro/nagicli-go"
)

func application() *cli.Command {
	return cli.NewCommand("service").
		About("Manage a service").
		Version("0.2.0").
		RequireSubcommand().
		Subcommand(
			cli.NewCommand("start").
				About("Start the service").
				Option(cli.Count("verbose").Long("verbose").Short('v').Help("Increase verbosity")).
				Handle(func(context *cli.Context, invocation *cli.Invocation) (cli.Outcome, error) {
					verbosity, _ := invocation.Count("verbose")
					if _, err := fmt.Fprintf(context.Stdout(), "starting with verbosity %d\n", verbosity); err != nil {
						return cli.Outcome{}, cli.NewDiagnostic(cli.CodeIOError, err.Error())
					}
					return cli.Success(), nil
				}),
		)
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
