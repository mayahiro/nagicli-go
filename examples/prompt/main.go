// Command prompt demonstrates optional line-oriented interactive prompts
package main

import (
	"errors"
	"fmt"
	"os"

	cli "github.com/mayahiro/nagicli-go"
	"github.com/mayahiro/nagicli-go/prompt"
)

func main() {
	status, err := cli.NewCommand("prompt").Handle(runPrompt).RunProcess()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if status != cli.StatusSuccess {
		os.Exit(int(status))
	}
}

func runPrompt(context *cli.Context, _ *cli.Invocation) (cli.Outcome, error) {
	prompter := prompt.NewProcess()
	profile, err := prompter.Select(
		context.Cancellation(),
		prompt.NewSelect("Profile", "Local", "Production").WithDefault(0),
	)
	if err != nil {
		return cli.Outcome{}, promptDiagnostic(err)
	}
	name, err := prompter.Input(
		context.Cancellation(),
		prompt.NewInput("Display name").Required(),
	)
	if err != nil {
		return cli.Outcome{}, promptDiagnostic(err)
	}
	token, err := prompter.Secret(
		context.Cancellation(),
		prompt.NewSecret("Access token").Required(),
	)
	if err != nil {
		return cli.Outcome{}, promptDiagnostic(err)
	}
	accepted, err := prompter.Confirm(
		context.Cancellation(),
		prompt.NewConfirm("Save this profile?").WithDefault(true),
	)
	if err != nil {
		return cli.Outcome{}, promptDiagnostic(err)
	}
	_, err = fmt.Fprintf(
		context.Stdout(),
		"profile=%s name=%s token_bytes=%d accepted=%t\n",
		[]string{"local", "production"}[profile],
		name,
		len(token),
		accepted,
	)
	if err != nil {
		return cli.Outcome{}, cli.NewDiagnostic(cli.CodeIOError, err.Error())
	}
	return cli.Success(), nil
}

func promptDiagnostic(err error) error {
	var promptErr *prompt.Error
	if !errors.As(err, &promptErr) {
		return cli.NewDiagnostic(cli.CodeHandlerError, err.Error())
	}
	code := cli.CodeHandlerError
	switch promptErr.Kind() {
	case prompt.ErrorCanceled:
		code = cli.CodeCancelled
	case prompt.ErrorIO:
		code = cli.CodeIOError
	}
	return cli.NewDiagnostic(code, promptErr.Error())
}
