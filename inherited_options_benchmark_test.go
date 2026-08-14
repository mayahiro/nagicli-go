package cli_test

import (
	"fmt"
	"testing"

	cli "github.com/mayahiro/nagicli-go"
)

const (
	benchmarkOptionOccurrences = 1_000
	benchmarkUnrelatedBranches = 100
	benchmarkOptionsPerBranch  = 8
)

func benchmarkInheritedCommand(unrelatedBranches int, deprecated bool) *cli.Command {
	verbose := cli.Count("verbose").Long("verbose").Inherited()
	if deprecated {
		verbose.Deprecated("--log-level")
	}
	root := cli.NewCommand("root").
		Option(verbose).
		Subcommand(cli.NewCommand("run"))
	for branchIndex := range unrelatedBranches {
		branch := cli.NewCommand(fmt.Sprintf("branch-%d", branchIndex))
		for optionIndex := range benchmarkOptionsPerBranch {
			name := fmt.Sprintf("option-%d", optionIndex)
			branch.Option(cli.Flag(name).Long(name).Inherited())
		}
		root.Subcommand(branch)
	}
	return root
}

func benchmarkInheritedArguments() []string {
	arguments := make([]string, benchmarkOptionOccurrences+1)
	arguments[0] = "run"
	for index := 1; index < len(arguments); index++ {
		arguments[index] = "--verbose"
	}
	return arguments
}

func benchmarkInheritedOptions(b *testing.B, unrelatedBranches int, deprecated bool) {
	command := benchmarkInheritedCommand(unrelatedBranches, deprecated)
	arguments := benchmarkInheritedArguments()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		result, err := command.Parse(arguments)
		if err != nil {
			b.Fatal(err)
		}
		count, ok := result.Invocation().Count("verbose")
		if !ok || count != benchmarkOptionOccurrences {
			b.Fatalf("verbose count = %d, %t", count, ok)
		}
	}
}

func BenchmarkInheritedOptionsSelectedPath(b *testing.B) {
	benchmarkInheritedOptions(b, 0, false)
}

func BenchmarkInheritedOptions100UnrelatedBranches(b *testing.B) {
	benchmarkInheritedOptions(b, benchmarkUnrelatedBranches, false)
}

func BenchmarkDeprecatedOption1000Occurrences(b *testing.B) {
	benchmarkInheritedOptions(b, 0, true)
}

func benchmarkValueOption(b *testing.B, sensitive bool) {
	token := cli.ValueOption("token").Long("token").Repeated()
	if sensitive {
		token.Sensitive()
	}
	command := cli.NewCommand("root").Option(token)
	arguments := make([]string, 0, benchmarkOptionOccurrences*2)
	for range benchmarkOptionOccurrences {
		arguments = append(arguments, "--token", "value")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		result, err := command.Parse(arguments)
		if err != nil {
			b.Fatal(err)
		}
		invocation := result.Invocation()
		value, present := invocation.RawValue("token")
		if !present || value != "value" {
			b.Fatalf("token value = %q, %t", value, present)
		}
		if invocation.ValueIsSensitive("token") != sensitive {
			b.Fatalf("token sensitivity = %t, want %t", invocation.ValueIsSensitive("token"), sensitive)
		}
	}
}

func BenchmarkValueOption1000Occurrences(b *testing.B) {
	benchmarkValueOption(b, false)
}

func BenchmarkSensitiveValueOption1000Occurrences(b *testing.B) {
	benchmarkValueOption(b, true)
}
