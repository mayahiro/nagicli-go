package clitest_test

import (
	"fmt"
	"io"
	"strings"
	"testing"

	cli "github.com/mayahiro/nagicli-go"
	"github.com/mayahiro/nagicli-go/clitest"
)

func TestDriverInjectsAndCapturesProcessServices(t *testing.T) {
	command := cli.NewCommand("sample").
		Argument(cli.Positional("value").Required()).
		Handle(func(context *cli.Context, invocation *cli.Invocation) (cli.Outcome, error) {
			value, _ := invocation.RawValue("value")
			_, err := fmt.Fprintf(context.Stdout(), "%s:%s\n", context.CurrentDirectory(), value)
			if err != nil {
				return cli.Outcome{}, cli.NewDiagnostic(cli.CodeIOError, err.Error())
			}
			return cli.Success(), nil
		})
	result, err := clitest.New(command).
		Arguments("value").
		Stdin([]byte("input")).
		CurrentDirectory("/work").
		Run()
	if err != nil {
		t.Fatal(err)
	}
	if result.Status() != cli.StatusSuccess {
		t.Fatalf("status = %d, want %d", result.Status(), cli.StatusSuccess)
	}
	if got := string(result.Stdout()); got != "/work:value\n" {
		t.Fatalf("stdout = %q", got)
	}
	if got := result.Stderr(); len(got) != 0 {
		t.Fatalf("stderr = %q", got)
	}
}

func TestDriverCanCancelBeforeHandlerExecution(t *testing.T) {
	called := false
	command := cli.NewCommand("sample").Handle(
		func(_ *cli.Context, _ *cli.Invocation) (cli.Outcome, error) {
			called = true
			return cli.Success(), nil
		},
	)
	result, err := clitest.New(command).Cancelled(true).Run()
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("handler ran after cancellation")
	}
	if result.Status() != cli.StatusCancelled {
		t.Fatalf("status = %d, want %d", result.Status(), cli.StatusCancelled)
	}
}

func TestDriverUsesRuntimePolicy(t *testing.T) {
	command := cli.NewCommand("sample").
		Subcommand(cli.NewCommand("child"))
	policy := cli.DefaultRuntimePolicy().WithHelpRenderer(commandPathRenderer{})
	result, err := clitest.New(command).
		Arguments("help", "child").
		Policy(policy).
		Run()
	if err != nil {
		t.Fatal(err)
	}
	if got := string(result.Stdout()); got != "custom help: sample/child\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestDriverInjectsValueResolver(t *testing.T) {
	command := cli.NewCommand("sample").
		Option(cli.ValueOption("profile").Long("profile")).
		Handle(func(_ *cli.Context, invocation *cli.Invocation) (cli.Outcome, error) {
			values := invocation.ParsedValues("profile")
			if len(values) != 1 || values[0].Raw() != "workspace" || values[0].Source() != cli.SourceExternal {
				t.Fatalf("values = %+v", values)
			}
			identity, present := values[0].Origin().Identity()
			if !present || identity != "test-config" {
				t.Fatalf("origin = %q, %t", identity, present)
			}
			return cli.Success(), nil
		})
	result, err := clitest.New(command).
		ValueResolver(func(cli.ValueResolutionRequest) (cli.ValueResolution, *cli.Diagnostic) {
			return cli.ReplaceValueResolution("test-config", "workspace"), nil
		}).
		Run()
	if err != nil {
		t.Fatal(err)
	}
	if result.Status() != cli.StatusSuccess {
		t.Fatalf("status = %d, want %d", result.Status(), cli.StatusSuccess)
	}
}

func TestDriverInjectsResponseFiles(t *testing.T) {
	command := cli.NewCommand("sample").
		Option(cli.ValueOption("profile").Long("profile")).
		Option(cli.ValueOption("theme").Long("theme")).
		Handle(func(runtime *cli.Context, invocation *cli.Invocation) (cli.Outcome, error) {
			if _, enabled := runtime.ResponseFileOptions(); !enabled {
				t.Fatal("handler Context lost Response File options")
			}
			if !invocation.Supplied("profile") {
				t.Fatal("Response File value was not command-line supplied")
			}
			values := invocation.ParsedValues("profile")
			if len(values) != 1 || values[0].Raw() != "workspace" || values[0].Source() != cli.SourceCommandLine {
				t.Fatalf("values = %+v", values)
			}
			theme := invocation.ParsedValues("theme")
			if len(theme) != 1 || theme[0].Raw() != "dark" || theme[0].Source() != cli.SourceExternal {
				t.Fatalf("theme = %+v", theme)
			}
			return cli.Success(), nil
		})
	result, err := clitest.New(command).
		Arguments("@args.txt").
		CurrentDirectory("/work").
		ValueResolver(func(request cli.ValueResolutionRequest) (cli.ValueResolution, *cli.Diagnostic) {
			if request.ValueID() != "theme" {
				t.Fatalf("resolver request = %q", request.ValueID())
			}
			return cli.ReplaceValueResolution("test-config", "dark"), nil
		}).
		ResponseFiles(
			cli.ResponseFileOptions{},
			cli.ResponseFileReaderFunc(func(request cli.ResponseFileReadRequest) ([]byte, *cli.Diagnostic) {
				if request.Path() != "/work/args.txt" {
					t.Fatalf("path = %q", request.Path())
				}
				return []byte("--profile workspace"), nil
			}),
		).
		Run()
	if err != nil {
		t.Fatal(err)
	}
	if result.Status() != cli.StatusSuccess {
		t.Fatalf("status = %d, want %d", result.Status(), cli.StatusSuccess)
	}
}

func TestResponseFileValuesKeepSensitiveRedaction(t *testing.T) {
	command := cli.NewCommand("sample").Option(
		cli.ValueOption("token").
			Long("token").
			Parser(cli.PossibleValuesParser("valid")).
			Sensitive(),
	)
	result, err := clitest.New(command).
		Arguments("@args.txt").
		ResponseFiles(
			cli.ResponseFileOptions{},
			cli.ResponseFileReaderFunc(func(cli.ResponseFileReadRequest) ([]byte, *cli.Diagnostic) {
				return []byte("--token response-secret"), nil
			}),
		).
		Run()
	if err != nil {
		t.Fatal(err)
	}
	stderr := string(result.Stderr())
	if result.Status() != cli.StatusUsage || !strings.Contains(stderr, cli.RedactedValue) || strings.Contains(stderr, "response-secret") {
		t.Fatalf("status = %d, stderr = %q", result.Status(), stderr)
	}
}

func TestStandardInputResponseFileConsumesContextStreamOnce(t *testing.T) {
	command := cli.NewCommand("sample").
		Option(cli.ValueOption("profile").Long("profile")).
		Handle(func(runtime *cli.Context, invocation *cli.Invocation) (cli.Outcome, error) {
			value, present := invocation.RawValue("profile")
			if !present || value != "workspace" {
				t.Fatalf("profile = %q, %t", value, present)
			}
			remaining, err := io.ReadAll(runtime.Stdin())
			if err != nil || len(remaining) != 0 {
				t.Fatalf("remaining stdin = %q, %v", remaining, err)
			}
			return cli.Success(), nil
		})
	result, err := clitest.New(command).
		Arguments("@-").
		Stdin([]byte("--profile workspace")).
		ResponseFiles(
			(cli.ResponseFileOptions{}).WithStandardInput(true),
			nil,
		).
		Run()
	if err != nil {
		t.Fatal(err)
	}
	if result.Status() != cli.StatusSuccess {
		t.Fatalf("status = %d", result.Status())
	}
}

func TestDriverReturnsOutputFailures(t *testing.T) {
	command := cli.NewCommand("sample")
	runtime := cli.NewContext(nil, failingWriter{}, io.Discard, nil, "/")
	_, err := command.Run(runtime, []string{"--help"})
	if err == nil {
		t.Fatal("Run succeeded with a failing output writer")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

type commandPathRenderer struct{}

func (commandPathRenderer) RenderHelp(document cli.HelpDocument) string {
	path := document.CommandPath()
	return fmt.Sprintf("custom help: %s\n", path[0]+"/"+path[1])
}
