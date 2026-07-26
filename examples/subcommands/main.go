// Command subcommands demonstrates a nested Nagi CLI application
package main

import (
	"fmt"
	"os"

	cli "github.com/mayahiro/nagicli-go"
)

func application() *cli.Command {
	return cli.NewCommand("service").
		ID("service-root").
		About("Manage a service").
		Version("0.3.2").
		Option(
			cli.ValueOption("profile").
				Long("profile").
				Parser(cli.StringParser()).
				Default("default").
				Help("Root configuration profile"),
		).
		RequireSubcommand().
		SubcommandUsage(cli.SubcommandUsageExpanded).
		Subcommand(
			cli.NewCommand("start").
				ID("start-command").
				About("Start the service").
				UsageVariant("configured", "[OPTIONS]").
				Option(
					cli.ValueOption("profile").
						Long("profile").
						Parser(cli.StringParser()).
						Default("service").
						Help("Service profile"),
				).
				Option(cli.Count("verbose").Long("verbose").Short('v').Help("Increase verbosity")).
				Validator(func(invocation *cli.Invocation) *cli.Diagnostic {
					profile, accessErr := cli.RequireValueAs[string](invocation, "profile")
					if accessErr != nil {
						return cli.NewDiagnostic(cli.CodeValidation, accessErr.Error()).
							WithTarget(cli.OptionTarget("profile"))
					}
					if profile == "blocked" {
						return cli.NewDiagnostic(
							cli.DiagnosticCode("reserved-profile"),
							"profile 'blocked' cannot be started",
						).
							WithCategory(cli.CategoryUsage).
							WithTarget(cli.OptionTarget("profile")).
							WithHint("choose another service profile")
					}
					return nil
				}).
				Handle(func(context *cli.Context, invocation *cli.Invocation) (cli.Outcome, error) {
					profile, accessErr := cli.RequireValueAs[string](invocation, "profile")
					if accessErr != nil {
						return cli.Outcome{}, cli.NewDiagnostic(cli.CodeHandlerError, accessErr.Error()).
							WithTarget(cli.OptionTarget("profile"))
					}
					root, ok := invocation.Scope("service-root")
					if !ok {
						return cli.Outcome{}, cli.NewDiagnostic(
							cli.CodeHandlerError,
							"root command scope is unavailable",
						)
					}
					rootProfile, accessErr := cli.RequireValueAs[string](root, "profile")
					if accessErr != nil {
						return cli.Outcome{}, cli.NewDiagnostic(cli.CodeHandlerError, accessErr.Error()).
							WithTarget(
								cli.OptionTarget("profile").
									WithCommandIDPath("service-root"),
							)
					}
					verbosity, _ := invocation.Count("verbose")
					if _, err := fmt.Fprintf(
						context.Stdout(),
						"starting profile %s from root %s with verbosity %d\n",
						profile,
						rootProfile,
						verbosity,
					); err != nil {
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
