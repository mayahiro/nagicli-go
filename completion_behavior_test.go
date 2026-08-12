package cli_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	cli "github.com/mayahiro/nagicli-go"
)

func TestCompletionDoesNotRunParserValidatorHandlerOrUnrelatedProvider(t *testing.T) {
	var parserCalls atomic.Int64
	var validatorCalls atomic.Int64
	var handlerCalls atomic.Int64
	var activeProviderCalls atomic.Int64
	var unrelatedProviderCalls atomic.Int64

	command := cli.NewCommand("root").
		Option(
			cli.ValueOption("mode").
				Long("mode").
				Parser(cli.CustomParser("MODE", func(value string) (string, error) {
					parserCalls.Add(1)
					return value, nil
				})).
				CompletionProvider(func(_ context.Context, request cli.CompletionRequest) ([]cli.CompletionCandidate, error) {
					activeProviderCalls.Add(1)
					if request.Target().ValueID() != "mode" {
						t.Fatalf("target = %q", request.Target().ValueID())
					}
					return []cli.CompletionCandidate{cli.NewCompletionCandidate("auto")}, nil
				}),
		).
		Argument(
			cli.Positional("other").CompletionProvider(func(context.Context, cli.CompletionRequest) ([]cli.CompletionCandidate, error) {
				unrelatedProviderCalls.Add(1)
				return nil, nil
			}),
		).
		Validator(func(*cli.Invocation) *cli.Diagnostic {
			validatorCalls.Add(1)
			return nil
		}).
		Handle(func(*cli.Context, *cli.Invocation) (cli.Outcome, error) {
			handlerCalls.Add(1)
			return cli.Success(), nil
		})

	engine, err := cli.NewCompletionEngine(command)
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.Complete(
		context.Background(),
		cli.NewCompletionInput([]string{"--mode"}, "a"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Candidates(); len(got) != 1 || got[0].Value() != "auto" {
		t.Fatalf("candidates = %+v", got)
	}
	if parserCalls.Load() != 0 || validatorCalls.Load() != 0 || handlerCalls.Load() != 0 {
		t.Fatalf(
			"parser=%d validator=%d handler=%d",
			parserCalls.Load(),
			validatorCalls.Load(),
			handlerCalls.Load(),
		)
	}
	if activeProviderCalls.Load() != 1 || unrelatedProviderCalls.Load() != 0 {
		t.Fatalf(
			"active provider=%d unrelated provider=%d",
			activeProviderCalls.Load(),
			unrelatedProviderCalls.Load(),
		)
	}
}

func TestCompletionCancellationBeforeAndDuringProvider(t *testing.T) {
	command := cli.NewCommand("root").Option(
		cli.ValueOption("value").Long("value").CompletionProvider(
			func(context.Context, cli.CompletionRequest) ([]cli.CompletionCandidate, error) {
				t.Fatal("provider ran after pre-cancellation")
				return nil, nil
			},
		),
	)
	engine, err := cli.NewCompletionEngine(command)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = engine.Complete(ctx, cli.NewCompletionInput([]string{"--value"}, ""))
	assertCompletionError(t, err, cli.CompletionErrorCancelled)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}

	var cancelDuring context.CancelFunc
	duringCommand := cli.NewCommand("root").Option(
		cli.ValueOption("value").Long("value").CompletionProvider(
			func(context.Context, cli.CompletionRequest) ([]cli.CompletionCandidate, error) {
				cancelDuring()
				return []cli.CompletionCandidate{cli.NewCompletionCandidate("value")}, nil
			},
		),
	)
	duringEngine, err := cli.NewCompletionEngine(duringCommand)
	if err != nil {
		t.Fatal(err)
	}
	duringContext, cancelDuringFunction := context.WithCancel(context.Background())
	cancelDuring = cancelDuringFunction
	_, err = duringEngine.Complete(
		duringContext,
		cli.NewCompletionInput([]string{"--value"}, ""),
	)
	assertCompletionError(t, err, cli.CompletionErrorCancelled)
}

func TestCompletionProviderAndCandidateErrorsAreDistinct(t *testing.T) {
	providerCause := errors.New("provider unavailable")
	providerCommand := cli.NewCommand("root").Option(
		cli.ValueOption("value").Long("value").CompletionProvider(
			func(context.Context, cli.CompletionRequest) ([]cli.CompletionCandidate, error) {
				return nil, providerCause
			},
		),
	)
	providerEngine, err := cli.NewCompletionEngine(providerCommand)
	if err != nil {
		t.Fatal(err)
	}
	_, err = providerEngine.Complete(
		context.Background(),
		cli.NewCompletionInput([]string{"--value"}, ""),
	)
	assertCompletionError(t, err, cli.CompletionErrorProvider)
	if !errors.Is(err, providerCause) {
		t.Fatalf("error = %v, want provider cause", err)
	}

	for name, candidate := range map[string]cli.CompletionCandidate{
		"empty":        cli.NewCompletionCandidate(""),
		"control":      cli.NewCompletionCandidate("bad\nvalue"),
		"c1-control":   cli.NewCompletionCandidate("\u0085"),
		"invalid-utf8": cli.NewCompletionCandidate(string([]byte{0xff})),
		"invalid-kind": cli.NewCompletionCandidate("value").WithKind(cli.CompletionCandidateKind(255)),
	} {
		t.Run(name, func(t *testing.T) {
			command := cli.NewCommand("root").Option(
				cli.ValueOption("value").Long("value").CompletionProvider(
					func(context.Context, cli.CompletionRequest) ([]cli.CompletionCandidate, error) {
						return []cli.CompletionCandidate{candidate}, nil
					},
				),
			)
			engine, buildErr := cli.NewCompletionEngine(command)
			if buildErr != nil {
				t.Fatal(buildErr)
			}
			_, completionErr := engine.Complete(
				context.Background(),
				cli.NewCompletionInput([]string{"--value"}, ""),
			)
			assertCompletionError(t, completionErr, cli.CompletionErrorInvalidCandidate)
		})
	}
}

func TestCompletionEngineOwnsGraphSnapshot(t *testing.T) {
	command := cli.NewCommand("root").Subcommand(cli.NewCommand("first"))
	engine, err := cli.NewCompletionEngine(command)
	if err != nil {
		t.Fatal(err)
	}
	command.Subcommand(cli.NewCommand("second"))
	result, err := engine.Complete(context.Background(), cli.NewCompletionInput(nil, ""))
	if err != nil {
		t.Fatal(err)
	}
	values := completionCandidateValues(result.Candidates())
	if fmt.Sprint(values) != "[first help --help -h]" {
		t.Fatalf("values = %v", values)
	}
}

func TestCompletionProviderIsValueOnlyConfiguration(t *testing.T) {
	command := cli.NewCommand("root").Option(
		cli.Flag("flag").Long("flag").CompletionProvider(
			func(context.Context, cli.CompletionRequest) ([]cli.CompletionCandidate, error) {
				return nil, nil
			},
		),
	)
	if err := command.Validate(); err == nil {
		t.Fatal("Validate succeeded")
	}
}

func TestCompletionHelpPathRequiresRootSubcommands(t *testing.T) {
	command := cli.NewCommand("root").Argument(
		cli.Positional("value").Parser(cli.PossibleValuesParser("help", "other")),
	)
	engine, err := cli.NewCompletionEngine(command)
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.Complete(context.Background(), cli.NewCompletionInput(nil, "h"))
	if err != nil {
		t.Fatal(err)
	}
	candidates := result.Candidates()
	if len(candidates) != 1 || candidates[0].Value() != "help" || candidates[0].Kind() != cli.CompletionCandidateValue {
		t.Fatalf("candidates = %+v", candidates)
	}

	result, err = engine.Complete(context.Background(), cli.NewCompletionInput([]string{"help"}, ""))
	if err != nil {
		t.Fatal(err)
	}
	partial := result.Request().PartialOccurrences()
	if len(partial) != 1 {
		t.Fatalf("partial = %+v", partial)
	}
	if raw, ok := partial[0].Raw(); !ok || raw != "help" {
		t.Fatalf("raw = %q, %t", raw, ok)
	}
}

func TestCompletionEmptyCandidateMetadataIsAbsent(t *testing.T) {
	candidate := cli.NewCompletionCandidate("value").WithDisplayLabel("").WithDescription("")
	if candidate.DisplayLabel() != "value" || candidate.Description() != "" {
		t.Fatalf("candidate = %+v", candidate)
	}
}

func assertCompletionError(t *testing.T, err error, kind cli.CompletionErrorKind) {
	t.Helper()
	var completionError *cli.CompletionError
	if !errors.As(err, &completionError) {
		t.Fatalf("error = %v, want CompletionError", err)
	}
	if completionError.Kind() != kind {
		t.Fatalf("kind = %v, want %v", completionError.Kind(), kind)
	}
}

func completionCandidateValues(candidates []cli.CompletionCandidate) []string {
	values := make([]string, len(candidates))
	for index, candidate := range candidates {
		values[index] = candidate.Value()
	}
	return values
}
