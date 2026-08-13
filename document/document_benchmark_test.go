package document_test

import (
	"fmt"
	"testing"

	cli "github.com/mayahiro/nagicli-go"
	"github.com/mayahiro/nagicli-go/document"
)

func BenchmarkDerivedHelpDocuments(b *testing.B) {
	for _, commandCount := range []int{1, 1_000} {
		command := benchmarkGraph(commandCount)
		for name, render := range map[string]func(cli.HelpDocument) string{
			"Markdown": func(help cli.HelpDocument) string {
				return (document.MarkdownRenderer{}).Render(help)
			},
			"Man": func(help cli.HelpDocument) string {
				return (document.ManRenderer{}).Render(help)
			},
		} {
			b.Run(fmt.Sprintf("%s/%d", name, commandCount), func(b *testing.B) {
				outputBytes := renderAll(b, command, render)
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					_ = renderAll(b, command, render)
				}
				b.ReportMetric(float64(outputBytes), "output-bytes/op")
			})
		}
	}
}

func benchmarkGraph(commandCount int) *cli.Command {
	root := cli.NewCommand("qed").About("Coding workspace")
	for index := 1; index < commandCount; index++ {
		root.Subcommand(
			cli.NewCommand(fmt.Sprintf("run-%d", index)).
				About("Run one deterministic task").
				Note("Generated benchmark command"),
		)
	}
	return root
}

func renderAll(
	b *testing.B,
	command *cli.Command,
	render func(cli.HelpDocument) string,
) int {
	b.Helper()
	outputBytes := 0
	if err := command.VisitHelpDocuments(func(help cli.HelpDocument) bool {
		outputBytes += len(render(help))
		return true
	}); err != nil {
		b.Fatal(err)
	}
	return outputBytes
}
