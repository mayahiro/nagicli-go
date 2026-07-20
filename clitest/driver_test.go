package clitest_test

import (
	"fmt"
	"io"
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
