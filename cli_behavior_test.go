package cli_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	cli "github.com/mayahiro/nagicli-go"
)

func TestCommandSelectionErrorsAreDistinct(t *testing.T) {
	command := cli.NewCommand("root").
		RequireSubcommand().
		Subcommand(cli.NewCommand("child"))

	_, err := command.Parse(nil)
	assertDiagnosticCode(t, err, cli.CodeMissingSubcommand)
	_, err = command.Parse([]string{"other"})
	assertDiagnosticCode(t, err, cli.CodeUnknownCommand)
}

func TestPositionalsDisableLaterSubcommandSelection(t *testing.T) {
	command := cli.NewCommand("root").
		Argument(cli.Positional("values").Repeated()).
		Subcommand(cli.NewCommand("child"))
	result, err := command.Parse([]string{"value", "child"})
	if err != nil {
		t.Fatal(err)
	}
	invocation := result.Invocation()
	if got := invocation.CommandPath(); len(got) != 1 || got[0] != "root" {
		t.Fatalf("command path = %v", got)
	}
	values := invocation.ParsedValues("values")
	if len(values) != 2 || values[1].Raw() != "child" {
		t.Fatalf("values = %v", values)
	}
}

func TestParentOptionsAreNotRecognizedAfterChildSelection(t *testing.T) {
	command := cli.NewCommand("root").
		Option(cli.Flag("root-option").Long("root-option")).
		Subcommand(cli.NewCommand("child"))
	_, err := command.Parse([]string{"child", "--root-option"})
	assertDiagnosticCode(t, err, cli.CodeUnknownOption)
}

func TestGraphValidationRejectsPathAndSiblingCollisions(t *testing.T) {
	tests := map[string]*cli.Command{
		"path ID": cli.NewCommand("root").
			Option(cli.Flag("same").Long("root-option")).
			Subcommand(cli.NewCommand("child").Argument(cli.Positional("same"))),
		"sibling spelling": cli.NewCommand("root").
			Subcommand(cli.NewCommand("first").Alias("shared")).
			Subcommand(cli.NewCommand("shared")),
		"non-final repeated positional": cli.NewCommand("root").
			Argument(cli.Positional("many").Repeated()).
			Argument(cli.Positional("last")),
		"cross-command relation": cli.NewCommand("root").
			Option(cli.Flag("parent").Long("parent")).
			Subcommand(cli.NewCommand("child").Option(cli.Flag("child-option").Long("child-option").Requires("parent"))),
	}
	for name, command := range tests {
		t.Run(name, func(t *testing.T) {
			assertDiagnosticCode(t, command.Validate(), cli.CodeInvalidSpecification)
		})
	}
}

func TestRuntimeReportsMissingAndGenericHandlerErrors(t *testing.T) {
	for name, command := range map[string]*cli.Command{
		"missing": cli.NewCommand("root"),
		"generic": cli.NewCommand("root").Handle(func(*cli.Context, *cli.Invocation) (cli.Outcome, error) {
			return cli.Outcome{}, errors.New("failed")
		}),
	} {
		t.Run(name, func(t *testing.T) {
			stderr := &bytes.Buffer{}
			runtime := cli.NewContext(bytes.NewReader(nil), &bytes.Buffer{}, stderr, nil, "/")
			outcome, err := command.Run(runtime, nil)
			if err != nil {
				t.Fatal(err)
			}
			if outcome.Status() != cli.StatusFailure {
				t.Fatalf("status = %d", outcome.Status())
			}
			if !bytes.HasPrefix(stderr.Bytes(), []byte("error[")) {
				t.Fatalf("stderr = %q", stderr.Bytes())
			}
		})
	}
}

func TestCancellationAfterHandlerOverridesOnlySuccess(t *testing.T) {
	for name, handlerStatus := range map[string]cli.ExitStatus{
		"success": cli.StatusSuccess,
		"failure": 7,
	} {
		t.Run(name, func(t *testing.T) {
			cancellation, cancel := context.WithCancel(context.Background())
			command := cli.NewCommand("root").Handle(func(*cli.Context, *cli.Invocation) (cli.Outcome, error) {
				cancel()
				return cli.NewOutcome(handlerStatus), nil
			})
			runtime := cli.NewContextWithCancellation(
				bytes.NewReader(nil),
				&bytes.Buffer{},
				&bytes.Buffer{},
				nil,
				"/",
				cancellation,
			)
			outcome, err := command.Run(runtime, nil)
			if err != nil {
				t.Fatal(err)
			}
			want := handlerStatus
			if handlerStatus == cli.StatusSuccess {
				want = cli.StatusCancelled
			}
			if outcome.Status() != want {
				t.Fatalf("status = %d, want %d", outcome.Status(), want)
			}
		})
	}
}

func assertDiagnosticCode(t *testing.T, err error, want cli.DiagnosticCode) {
	t.Helper()
	var diagnostic *cli.Diagnostic
	if !errors.As(err, &diagnostic) {
		t.Fatalf("error = %v, want Diagnostic", err)
	}
	if diagnostic.Code() != want {
		t.Fatalf("code = %s, want %s", diagnostic.Code(), want)
	}
}
