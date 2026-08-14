package cli_test

import (
	"context"
	"fmt"
	"testing"

	cli "github.com/mayahiro/nagicli-go"
)

func benchmarkCompletionCommand(unrelatedBranches int) *cli.Command {
	root := cli.NewCommand("root").
		Option(cli.Count("verbose").Long("verbose").Inherited()).
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

func benchmarkCompletion(b *testing.B, unrelatedBranches int) {
	engine, err := cli.NewCompletionEngine(benchmarkCompletionCommand(unrelatedBranches))
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		result, completionErr := engine.Complete(
			ctx,
			cli.NewCompletionInput([]string{"run"}, "--v"),
		)
		if completionErr != nil {
			b.Fatal(completionErr)
		}
		candidates := result.Candidates()
		if len(candidates) != 1 || candidates[0].Value() != "--verbose" {
			b.Fatalf("candidates = %+v", candidates)
		}
	}
}

func BenchmarkCompletionSelectedPath(b *testing.B) {
	benchmarkCompletion(b, 0)
}

func BenchmarkCompletion100UnrelatedBranches(b *testing.B) {
	benchmarkCompletion(b, benchmarkUnrelatedBranches)
}

func benchmarkHiddenCompletion(b *testing.B, pairs int) {
	root := cli.NewCommand("root")
	for index := range pairs {
		root.
			Option(cli.Flag(fmt.Sprintf("hidden-option-%d", index)).Long(fmt.Sprintf("hidden-option-%d", index)).Hidden()).
			Subcommand(cli.NewCommand(fmt.Sprintf("hidden-command-%d", index)).Hidden())
	}
	engine, err := cli.NewCompletionEngine(root)
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	input := cli.NewCompletionInput(nil, "")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		result, completionErr := engine.Complete(ctx, input)
		if completionErr != nil {
			b.Fatal(completionErr)
		}
		if candidates := result.Candidates(); len(candidates) != 2 {
			b.Fatalf("candidates = %+v", candidates)
		}
	}
}

func BenchmarkCompletionNoVisibleDeclarations(b *testing.B) {
	benchmarkHiddenCompletion(b, 0)
}

func BenchmarkCompletion1000HiddenPairs(b *testing.B) {
	benchmarkHiddenCompletion(b, 1_000)
}
