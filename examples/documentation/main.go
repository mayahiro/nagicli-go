// Command documentation renders every visible command Help Document without
// retaining all pages
package main

import (
	"fmt"
	"os"

	cli "github.com/mayahiro/nagicli-go"
	"github.com/mayahiro/nagicli-go/document"
)

func main() {
	format := "markdown"
	if len(os.Args) > 1 {
		format = os.Args[1]
	}
	command := cli.NewCommand("qed").
		About("Manage coding sessions").
		Option(
			cli.ValueOption("config").
				Long("config").
				Help("Configuration path").
				Inherited(),
		).
		Subcommand(
			cli.NewCommand("run").
				About("Run one task").
				Option(cli.Flag("verbose").Long("verbose")),
		)

	first := true
	err := command.VisitHelpDocuments(func(help cli.HelpDocument) bool {
		if !first {
			fmt.Println("---")
		}
		first = false
		if format == "man" {
			fmt.Print((document.ManRenderer{}).Render(help))
		} else {
			fmt.Print((document.MarkdownRenderer{}).Render(help))
		}
		return true
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
