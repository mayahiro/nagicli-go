// Command staged demonstrates incremental Nagi CLI adoption
package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	cli "github.com/mayahiro/nagicli-go"
)

func application() *cli.Command {
	return cli.NewCommand("tool").
		ID("tool-root").
		About("Inspect one target").
		Version("0.4.0").
		RequireSubcommand().
		Subcommand(
			cli.NewCommand("inspect").
				ID("inspect-command").
				Argument(
					cli.Positional("target").
						Parser(cli.StringParser()).
						Required().
						Help("Target to inspect"),
				).
				Validator(func(invocation *cli.Invocation) *cli.Diagnostic {
					target, _ := cli.ValueAs[string](invocation, "target")
					if target == "blocked" {
						return cli.NewDiagnostic(
							cli.DiagnosticCode("target-blocked"),
							"target 'blocked' cannot be inspected",
						).
							WithCategory(cli.CategoryUsage).
							WithTarget(cli.ArgumentTarget("target")).
							WithHint("choose another target")
					}
					return nil
				}).
				Handle(legacyInspect),
		)
}

func legacyInspect(context *cli.Context, invocation *cli.Invocation) (cli.Outcome, error) {
	target, accessErr := cli.RequireValueAs[string](invocation, "target")
	if accessErr != nil {
		return cli.Outcome{}, cli.NewDiagnostic(cli.CodeHandlerError, accessErr.Error()).
			WithTarget(cli.ArgumentTarget("target"))
	}
	if _, err := fmt.Fprintf(context.Stdout(), "legacy inspect: %s\n", target); err != nil {
		return cli.Outcome{}, cli.NewDiagnostic(cli.CodeIOError, err.Error())
	}
	return cli.Success(), nil
}

func run(arguments []string, context *cli.Context) (cli.ExitStatus, error) {
	command := application()
	policy := cli.DefaultRuntimePolicy().WithExitCodePolicy(
		cli.DefaultExitCodePolicy().WithStatus(cli.CategoryUsage, cli.StatusFailure),
	)
	result, err := command.Parse(arguments)
	if err != nil {
		var diagnostic *cli.Diagnostic
		if !errors.As(err, &diagnostic) {
			return cli.StatusFailure, err
		}
		if _, writeErr := context.Stderr().Write(
			[]byte(policy.RenderDiagnostic(diagnostic)),
		); writeErr != nil {
			return cli.StatusFailure, writeErr
		}
		return policy.StatusForDiagnostic(diagnostic), nil
	}
	var outcome cli.Outcome
	if result.Kind() == cli.ParseInvocation {
		if strings.Join(result.CommandIDPath(), "/") != "tool-root/inspect-command" {
			return cli.StatusFailure, errors.New(
				"no existing handler adapter for the selected command",
			)
		}
		outcome, err = command.RunInvocationWithPolicy(context, result.Invocation(), policy)
	} else {
		outcome, err = command.RunParsedWithPolicy(context, result, policy)
	}
	if err != nil {
		return cli.StatusFailure, err
	}
	return outcome.Status(), nil
}

func processContext() (*cli.Context, error) {
	currentDirectory, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	environment := make(map[string]string)
	for _, entry := range os.Environ() {
		name, value, found := strings.Cut(entry, "=")
		if found {
			environment[name] = value
		}
	}
	return cli.NewContext(
		os.Stdin,
		os.Stdout,
		os.Stderr,
		environment,
		currentDirectory,
	), nil
}

func main() {
	context, err := processContext()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(int(cli.StatusFailure))
	}
	status, err := run(os.Args[1:], context)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(int(cli.StatusFailure))
	}
	os.Exit(int(status))
}
