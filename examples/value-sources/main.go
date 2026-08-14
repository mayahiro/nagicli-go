// Value Source Adapter maps application-owned configuration into CLI fallbacks
package main

import (
	"fmt"
	"os"
	"strings"

	cli "github.com/mayahiro/nagicli-go"
)

func application() *cli.Command {
	return cli.NewCommand("value-sources").
		About("Demonstrate application-owned value fallback resolution").
		Option(
			cli.ValueOption("profile").
				Long("profile").
				Default("local").
				Help("Execution profile"),
		).
		Option(
			cli.ValueOption("tag").
				Long("tag").
				Repeated().
				Default("baseline").
				Help("Ordered execution tag"),
		).
		Handle(runCommand)
}

func projectConfig(request cli.ValueResolutionRequest) (cli.ValueResolution, *cli.Diagnostic) {
	switch request.ValueID() {
	case "profile":
		return cli.ReplaceValueResolution("project-config", "workspace"), nil
	case "tag":
		return cli.MergeValueResolution("project-config", "configured"), nil
	default:
		return cli.ValueResolution{}, nil
	}
}

func runCommand(context *cli.Context, invocation *cli.Invocation) (cli.Outcome, error) {
	profile := describe(invocation.ParsedValues("profile")[0])
	tagValues := invocation.ParsedValues("tag")
	tags := make([]string, len(tagValues))
	for index := range tagValues {
		tags[index] = describe(tagValues[index])
	}
	if _, err := fmt.Fprintf(context.Stdout(), "profile=%s\ntags=%s\n", profile, strings.Join(tags, ",")); err != nil {
		return cli.Outcome{}, cli.NewDiagnostic(cli.CodeIOError, err.Error())
	}
	return cli.Success(), nil
}

func describe(value cli.ParsedValue) string {
	source := "default"
	switch value.Source() {
	case cli.SourceCommandLine:
		source = "command-line"
	case cli.SourceEnvironment:
		identity, _ := value.Origin().Identity()
		source = "environment(" + identity + ")"
	case cli.SourceExternal:
		identity, _ := value.Origin().Identity()
		source = "external(" + identity + ")"
	}
	return value.Raw() + ":" + source
}

func main() {
	status, err := application().RunProcessWithValueResolver(projectConfig)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(int(cli.StatusFailure))
	}
	os.Exit(int(status))
}
