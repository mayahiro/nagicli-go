// Command completion demonstrates generated shell scripts and their reserved protocol
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	cli "github.com/mayahiro/nagicli-go"
	"github.com/mayahiro/nagicli-go/completion"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	command := applicationCommand()
	engine, err := cli.NewCompletionEngine(command)
	if err != nil {
		return err
	}
	arguments := os.Args[1:]
	if len(arguments) > 0 && arguments[0] == "generate" {
		if len(arguments) != 2 {
			return errors.New("usage: completion generate <bash|zsh|fish|powershell>")
		}
		shell, ok := parseShell(arguments[1])
		if !ok {
			return errors.New("usage: completion generate <bash|zsh|fish|powershell>")
		}
		script, generationErr := completion.Generate(shell, engine)
		if generationErr != nil {
			return generationErr
		}
		_, err = fmt.Fprint(os.Stdout, script)
		return err
	}

	handled, err := completion.Handle(
		context.Background(),
		engine,
		arguments,
		os.Stdout,
	)
	if err != nil || handled {
		return err
	}
	_, err = command.RunProcess()
	return err
}

func applicationCommand() *cli.Command {
	return cli.NewCommand("completion").
		Option(
			cli.ValueOption("profile").
				Long("profile").
				Parser(cli.StringParser()).
				CompletionProvider(profileCompletion).
				Help("Execution profile"),
		).
		Handle(runCommand).
		Subcommand(cli.NewCommand("inspect").Handle(runCommand))
}

func profileCompletion(
	ctx context.Context,
	_ cli.CompletionRequest,
) ([]cli.CompletionCandidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []cli.CompletionCandidate{
		cli.NewCompletionCandidate("dev"),
		cli.NewCompletionCandidate("prod"),
		cli.NewCompletionCandidate("preview").WithDescription("Preview profile"),
	}, nil
}

func runCommand(context *cli.Context, invocation *cli.Invocation) (cli.Outcome, error) {
	profile, ok := invocation.RawValue("profile")
	if !ok {
		profile = "dev"
	}
	_, err := fmt.Fprintf(
		context.Stdout(),
		"command=%s profile=%s\n",
		strings.Join(invocation.CommandPath(), "/"),
		profile,
	)
	if err != nil {
		return cli.Outcome{}, cli.NewDiagnostic(cli.CodeIOError, err.Error())
	}
	return cli.Success(), nil
}

func parseShell(value string) (completion.Shell, bool) {
	switch value {
	case "bash":
		return completion.Bash, true
	case "zsh":
		return completion.Zsh, true
	case "fish":
		return completion.Fish, true
	case "powershell":
		return completion.PowerShell, true
	default:
		return 0, false
	}
}
