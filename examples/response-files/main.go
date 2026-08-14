// Response Files expand opt-in argument containers before command parsing
package main

import (
	"fmt"
	"os"
	"strings"

	cli "github.com/mayahiro/nagicli-go"
)

func application() *cli.Command {
	return cli.NewCommand("response-files").
		About("Demonstrate bounded Response File expansion").
		Option(
			cli.ValueOption("profile").
				Long("profile").
				Required().
				Help("Execution profile"),
		).
		Option(
			cli.ValueOption("tag").
				Long("tag").
				Repeated().
				Help("Ordered execution tag"),
		).
		Handle(runCommand)
}

func runCommand(context *cli.Context, invocation *cli.Invocation) (cli.Outcome, error) {
	profile, _ := invocation.RawValue("profile")
	values := invocation.ParsedValues("tag")
	tags := make([]string, len(values))
	for index := range values {
		tags[index] = values[index].Raw()
	}
	if _, err := fmt.Fprintf(
		context.Stdout(),
		"profile=%s\ntags=%s\n",
		profile,
		strings.Join(tags, "|"),
	); err != nil {
		return cli.Outcome{}, cli.NewDiagnostic(cli.CodeIOError, err.Error())
	}
	return cli.Success(), nil
}

func main() {
	options := cli.DefaultProcessOptions().WithResponseFiles(cli.ResponseFileOptions{})
	status, err := application().RunProcessWithOptions(options)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(int(cli.StatusFailure))
	}
	os.Exit(int(status))
}
