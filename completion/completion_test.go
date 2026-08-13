package completion_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	cli "github.com/mayahiro/nagicli-go"
	"github.com/mayahiro/nagicli-go/completion"
)

func TestGeneratedScriptsMatchSharedGoldenFiles(t *testing.T) {
	engine := completionEngine(t, nil)
	for name, shell := range map[string]completion.Shell{
		"bash":       completion.Bash,
		"zsh":        completion.Zsh,
		"fish":       completion.Fish,
		"powershell": completion.PowerShell,
	} {
		t.Run(name, func(t *testing.T) {
			want := readCompletionGolden(t, name+".txt")
			got, err := completion.Generate(shell, engine)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("generated script differs\ngot:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestProtocolHandlesStaticAndDynamicCandidates(t *testing.T) {
	provider := func(_ context.Context, _ cli.CompletionRequest) ([]cli.CompletionCandidate, error) {
		return []cli.CompletionCandidate{
			cli.NewCompletionCandidate("prod"),
			cli.NewCompletionCandidate("preview").WithDescription("Preview profile").WithAppendSpace(false),
		}, nil
	}
	engine := completionEngine(t, provider)

	var bash bytes.Buffer
	handled, err := completion.Handle(
		context.Background(),
		engine,
		[]string{completion.ProtocolToken, "bash", "p", "--profile"},
		&bash,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("request was not handled")
	}
	if got, want := bash.String(), "prod\tprod\tprod\tvalue\tspace\npreview\tpreview\tPreview profile\tvalue\tnone\n"; got != want {
		t.Fatalf("bash protocol = %q, want %q", got, want)
	}

	var fish bytes.Buffer
	handled, err = completion.Handle(
		context.Background(),
		engine,
		[]string{completion.ProtocolToken, "fish", "p", "--profile"},
		&fish,
	)
	if err != nil || !handled {
		t.Fatalf("handled=%t error=%v", handled, err)
	}
	if got, want := fish.String(), "prod\tprod\npreview\tPreview profile\n"; got != want {
		t.Fatalf("fish protocol = %q, want %q", got, want)
	}
}

func TestProtocolDecoratesDeprecatedStaticCandidates(t *testing.T) {
	engine, err := cli.NewCompletionEngine(
		cli.NewCommand("qed").
			Subcommand(cli.NewCommand("old").About("Old command").Deprecated("run")).
			Subcommand(cli.NewCommand("run")),
	)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	handled, err := completion.Handle(
		context.Background(),
		engine,
		[]string{completion.ProtocolToken, "bash", "o"},
		&output,
	)
	if err != nil || !handled {
		t.Fatalf("handled=%t error=%v", handled, err)
	}
	if got, want := output.String(), "old\told\tOld command [deprecated: use run]\tcommand\tspace\n"; got != want {
		t.Fatalf("protocol = %q, want %q", got, want)
	}
}

func TestProtocolPassThroughAndErrors(t *testing.T) {
	engine := completionEngine(t, nil)
	var output bytes.Buffer
	handled, err := completion.Handle(context.Background(), engine, []string{"run"}, &output)
	if err != nil || handled || output.Len() != 0 {
		t.Fatalf("handled=%t output=%q error=%v", handled, output.String(), err)
	}

	for name, arguments := range map[string][]string{
		"missing": {completion.ProtocolToken},
		"shell":   {completion.ProtocolToken, "unknown", ""},
	} {
		t.Run(name, func(t *testing.T) {
			handled, err := completion.Handle(context.Background(), engine, arguments, &output)
			if !handled {
				t.Fatal("reserved request was not handled")
			}
			var protocolError *completion.ProtocolError
			if !errors.As(err, &protocolError) || protocolError.Kind() != completion.ProtocolErrorInvalidRequest {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestProtocolPreservesCompletionError(t *testing.T) {
	cause := errors.New("provider failed")
	engine := completionEngine(t, func(context.Context, cli.CompletionRequest) ([]cli.CompletionCandidate, error) {
		return nil, cause
	})
	_, err := completion.Handle(
		context.Background(),
		engine,
		[]string{completion.ProtocolToken, "bash", "", "--profile"},
		&bytes.Buffer{},
	)
	var protocolError *completion.ProtocolError
	if !errors.As(err, &protocolError) || protocolError.Kind() != completion.ProtocolErrorCompletion {
		t.Fatalf("error = %v", err)
	}
	var completionError *cli.CompletionError
	if !errors.As(err, &completionError) || completionError.Kind() != cli.CompletionErrorProvider {
		t.Fatalf("error = %v, want provider CompletionError", err)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("error = %v, want provider cause", err)
	}
}

func TestProtocolReportsWriterErrors(t *testing.T) {
	engine := completionEngine(t, nil)
	for name, writer := range map[string]io.Writer{
		"nil":     nil,
		"failure": failingWriter{},
	} {
		t.Run(name, func(t *testing.T) {
			handled, err := completion.Handle(
				context.Background(),
				engine,
				[]string{completion.ProtocolToken, "bash", "p", "--profile"},
				writer,
			)
			if !handled {
				t.Fatal("reserved request was not handled")
			}
			var protocolError *completion.ProtocolError
			if !errors.As(err, &protocolError) || protocolError.Kind() != completion.ProtocolErrorIO {
				t.Fatalf("error = %v, want protocol I/O error", err)
			}
		})
	}
}

func completionEngine(t *testing.T, provider cli.CompletionProvider) *cli.CompletionEngine {
	t.Helper()
	option := cli.ValueOption("profile").
		Long("profile").
		Parser(cli.PossibleValuesParser("dev", "prod"))
	if provider != nil {
		option.CompletionProvider(provider)
	}
	engine, err := cli.NewCompletionEngine(
		cli.NewCommand("qed").
			Version("1.0.0").
			Option(option).
			Subcommand(cli.NewCommand("run")),
	)
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func readCompletionGolden(t *testing.T, name string) string {
	t.Helper()
	root := os.Getenv("NAGI_FIXTURES")
	if root == "" {
		t.Skip("NAGI_FIXTURES is not configured")
	}
	path := filepath.Join(root, "cli", "completion", name)
	value, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(fmt.Errorf("read %s: %w", path, err))
	}
	return string(value)
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}
