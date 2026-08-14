package cli_test

import (
	"fmt"
	"testing"

	cli "github.com/mayahiro/nagicli-go"
)

const benchmarkValueResolutionGraphSize = 1_000

func benchmarkSelectedValueResolutionCommand(values int) *cli.Command {
	command := cli.NewCommand("root")
	for index := range values {
		id := fmt.Sprintf("value-%d", index)
		command.Option(cli.ValueOption(id).Long(id))
	}
	return command
}

func benchmarkUnselectedValueResolutionCommand(branches int) *cli.Command {
	command := cli.NewCommand("root").Subcommand(
		cli.NewCommand("run").Option(cli.ValueOption("selected").Long("selected")),
	)
	for index := range branches {
		id := fmt.Sprintf("branch-value-%d", index)
		command.Subcommand(
			cli.NewCommand(fmt.Sprintf("branch-%d", index)).
				Option(cli.ValueOption(id).Long(id)),
		)
	}
	return command
}

func benchmarkValueResolution(
	b *testing.B,
	command *cli.Command,
	arguments []string,
	expectedCalls int,
	lastValueID string,
) {
	calls := 0
	resolver := func(cli.ValueResolutionRequest) (cli.ValueResolution, *cli.Diagnostic) {
		calls++
		return cli.ReplaceValueResolution("benchmark-config", "value"), nil
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		calls = 0
		result, err := command.ParseWithValueResolver(arguments, nil, resolver)
		if err != nil {
			b.Fatal(err)
		}
		if calls != expectedCalls {
			b.Fatalf("resolver calls = %d, want %d", calls, expectedCalls)
		}
		values := result.Invocation().ParsedValues(lastValueID)
		if len(values) != 1 || values[0].Source() != cli.SourceExternal {
			b.Fatalf("values = %+v", values)
		}
	}
}

func BenchmarkValueResolverSingle(b *testing.B) {
	benchmarkValueResolution(
		b,
		benchmarkSelectedValueResolutionCommand(1),
		nil,
		1,
		"value-0",
	)
}

func BenchmarkValueResolver1000Selected(b *testing.B) {
	benchmarkValueResolution(
		b,
		benchmarkSelectedValueResolutionCommand(benchmarkValueResolutionGraphSize),
		nil,
		benchmarkValueResolutionGraphSize,
		"value-999",
	)
}

func BenchmarkValueResolver1000UnselectedBranches(b *testing.B) {
	benchmarkValueResolution(
		b,
		benchmarkUnselectedValueResolutionCommand(benchmarkValueResolutionGraphSize),
		[]string{"run"},
		1,
		"selected",
	)
}
