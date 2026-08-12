package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	cli "github.com/mayahiro/nagicli-go"
)

func TestCompletionMatchesSharedFixtures(t *testing.T) {
	records := loadFixtures(
		t,
		"cli/completion.txt",
		"cli-completion",
		"argv",
		"current",
		"expected",
	)
	engine, err := cli.NewCompletionEngine(completionFixtureCommand())
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			result, err := engine.Complete(
				context.Background(),
				cli.NewCompletionInput(arguments(record.Bytes("argv")), record.Text("current")),
			)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := snapshotCompletion(result), record.Field("expected"); got != want {
				t.Fatalf("snapshot = %q, want %q", got, want)
			}
		})
	}
}

func completionFixtureCommand() *cli.Command {
	provider := func(_ context.Context, request cli.CompletionRequest) ([]cli.CompletionCandidate, error) {
		switch request.Target().ValueID() {
		case "config":
			return []cli.CompletionCandidate{
				cli.NewCompletionCandidate("prod"),
				cli.NewCompletionCandidate("canary").WithAppendSpace(false),
			}, nil
		case "agent":
			return []cli.CompletionCandidate{
				cli.NewCompletionCandidate("atlas"),
				cli.NewCompletionCandidate("ada"),
				cli.NewCompletionCandidate("日本"),
			}, nil
		case "session":
			return []cli.CompletionCandidate{
				cli.NewCompletionCandidate("local"),
				cli.NewCompletionCandidate("session-a"),
			}, nil
		default:
			return nil, fmt.Errorf("unexpected completion target %q", request.Target().ValueID())
		}
	}
	return cli.NewCommand("qed").
		ID("root-id").
		Version("1.0.0").
		Option(cli.Count("verbose").Long("verbose").Short('v').Inherited().Help("Increase verbosity")).
		Option(
			cli.ValueOption("config").
				Long("config").
				Short('c').
				Inherited().
				Parser(cli.PossibleValuesParser("dev", "prod")).
				CompletionProvider(provider).
				Help("Configuration profile"),
		).
		Option(cli.ValueOption("output").Long("output").Parser(cli.PossibleValuesParser("text", "json"))).
		Subcommand(
			cli.NewCommand("run").
				ID("run-id").
				Alias("r").
				About("Run one session").
				Option(cli.Flag("dry-run").Long("dry-run").Help("Do not execute")).
				Option(
					cli.ValueOption("agent").
						Long("agent").
						Short('a').
						CompletionProvider(provider).
						Help("Agent name"),
				).
				Argument(
					cli.Positional("session").
						Repeated().
						Parser(cli.PossibleValuesParser("local")).
						CompletionProvider(provider),
				).
				Subcommand(cli.NewCommand("exec").ID("exec-id").About("Execute a task")),
		)
}

func snapshotCompletion(result cli.CompletionResult) string {
	request := result.Request()
	var output strings.Builder
	output.WriteString("path=")
	output.WriteString(strings.Join(request.CommandPath(), "/"))
	output.WriteString(";ids=")
	output.WriteString(strings.Join(request.CommandIDPath(), "/"))
	output.WriteString(";target=")
	output.WriteString(snapshotCompletionTarget(request.Target()))
	output.WriteString(";prefix=")
	output.WriteString(request.Prefix())
	output.WriteString(";partial=")
	partial := request.PartialOccurrences()
	if len(partial) == 0 {
		output.WriteString("none")
	}
	for index, occurrence := range partial {
		if index > 0 {
			output.WriteByte(',')
		}
		output.WriteString(snapshotCompletionTarget(occurrence.Target()))
		output.WriteByte('=')
		output.WriteString(snapshotOccurrenceKind(occurrence.Kind()))
		if raw, ok := occurrence.Raw(); ok {
			output.WriteByte(':')
			output.WriteString(raw)
		}
	}
	output.WriteString(";candidates=")
	for index, candidate := range result.Candidates() {
		if index > 0 {
			output.WriteByte(',')
		}
		output.WriteString(snapshotCandidateKind(candidate.Kind()))
		output.WriteByte(':')
		output.WriteString(candidate.Value())
		if candidate.AppendSpace() {
			output.WriteByte('+')
		} else {
			output.WriteByte('-')
		}
	}
	return output.String()
}

func snapshotCompletionTarget(target cli.CompletionTarget) string {
	var kind string
	switch target.Kind() {
	case cli.CompletionTargetCommand:
		kind = "command"
	case cli.CompletionTargetOption:
		kind = "option"
	case cli.CompletionTargetArgument:
		kind = "argument"
	default:
		kind = "unknown"
	}
	result := kind + "@" + strings.Join(target.CommandIDPath(), "/")
	if target.ValueID() != "" {
		result += ":" + target.ValueID()
	}
	return result
}

func snapshotOccurrenceKind(kind cli.CompletionOccurrenceKind) string {
	switch kind {
	case cli.CompletionOccurrenceFlag:
		return "flag"
	case cli.CompletionOccurrenceCount:
		return "count"
	case cli.CompletionOccurrenceValue:
		return "value"
	default:
		return "unknown"
	}
}

func snapshotCandidateKind(kind cli.CompletionCandidateKind) string {
	switch kind {
	case cli.CompletionCandidateCommand:
		return "command"
	case cli.CompletionCandidateOption:
		return "option"
	case cli.CompletionCandidateValue:
		return "value"
	default:
		return "unknown"
	}
}

func TestCompletionInputOwnsArguments(t *testing.T) {
	arguments := []string{"run"}
	input := cli.NewCompletionInput(arguments, "")
	arguments[0] = "changed"
	if got := input.Arguments(); !bytes.Equal([]byte(got[0]), []byte("run")) {
		t.Fatalf("arguments = %q", got)
	}
}
