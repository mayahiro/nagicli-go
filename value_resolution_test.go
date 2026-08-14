package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	cli "github.com/mayahiro/nagicli-go"
)

func TestValueResolutionMatchesSharedFixtures(t *testing.T) {
	records := loadFixtures(
		t,
		"cli/value-resolution.txt",
		"cli-value-resolution",
		"arrangement", "argv", "env", "expected", "calls",
	)
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			arrangement := record.Field("arrangement")
			command := valueResolutionCommand(arrangement)
			var calls []string
			resolver := cli.ValueResolver(func(request cli.ValueResolutionRequest) (cli.ValueResolution, *cli.Diagnostic) {
				calls = append(calls, strings.Join(request.CommandIDPath(), "/")+":"+request.ValueID())
				return resolveValue(arrangement, request)
			})
			result, err := command.ParseWithValueResolver(
				arguments(record.Bytes("argv")),
				environment(record.Bytes("env")),
				resolver,
			)
			if got, want := snapshotValueResolutionResult(result, err), record.Field("expected"); got != want {
				t.Fatalf("snapshot = %q, want %q", got, want)
			}
			if got, want := strings.Join(calls, "|"), record.Field("calls"); got != want {
				t.Fatalf("calls = %q, want %q", got, want)
			}
		})
	}
}

func TestValueResolutionRequestExposesSelectedAndDeclarationPaths(t *testing.T) {
	command := valueResolutionCommand("selected")
	var snapshot []string
	_, err := command.ParseWithValueResolver(
		[]string{"run"},
		nil,
		func(request cli.ValueResolutionRequest) (cli.ValueResolution, *cli.Diagnostic) {
			snapshot = append(snapshot, fmt.Sprintf(
				"selected=%s:%s;declaration=%s:%s;value=%s;repeated=%t;sensitive=%t",
				strings.Join(request.SelectedCommandPath(), "/"),
				strings.Join(request.SelectedCommandIDPath(), "/"),
				strings.Join(request.CommandPath(), "/"),
				strings.Join(request.CommandIDPath(), "/"),
				request.ValueID(),
				request.IsRepeated(),
				request.IsSensitive(),
			))
			return resolveValue("selected", request)
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"selected=root/run:root-id/run-id;declaration=root:root-id;value=root-value;repeated=false;sensitive=false",
		"selected=root/run:root-id/run-id;declaration=root/run:root-id/run-id;value=profile;repeated=false;sensitive=false",
	}
	if strings.Join(snapshot, "|") != strings.Join(want, "|") {
		t.Fatalf("snapshot = %v, want %v", snapshot, want)
	}
}

func TestRuntimeContextResolvesValuesBeforeHandlerExecution(t *testing.T) {
	command := cli.NewCommand("root").
		Option(cli.ValueOption("profile").Long("profile")).
		Handle(func(runtime *cli.Context, invocation *cli.Invocation) (cli.Outcome, error) {
			if runtime.ValueResolver() == nil {
				t.Fatal("handler Context lost the Value Resolver")
			}
			values := invocation.ParsedValues("profile")
			if len(values) != 1 || values[0].Raw() != "workspace" || values[0].Source() != cli.SourceExternal {
				t.Fatalf("values = %+v", values)
			}
			identity, present := values[0].Origin().Identity()
			if !present || identity != "project-config" {
				t.Fatalf("origin = %q, %t", identity, present)
			}
			return cli.Success(), nil
		})
	runtime := cli.NewContextWithCancellation(
		bytes.NewReader(nil),
		&bytes.Buffer{},
		&bytes.Buffer{},
		nil,
		".",
		context.Background(),
	).WithValueResolver(func(cli.ValueResolutionRequest) (cli.ValueResolution, *cli.Diagnostic) {
		return cli.ReplaceValueResolution("project-config", "workspace"), nil
	})

	outcome, err := command.Run(runtime, nil)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status() != cli.StatusSuccess {
		t.Fatalf("status = %d", outcome.Status())
	}
}

func TestResolverSkipsNonValueInputsHelpAndVersion(t *testing.T) {
	calls := 0
	resolver := func(cli.ValueResolutionRequest) (cli.ValueResolution, *cli.Diagnostic) {
		calls++
		return cli.ValueResolution{}, nil
	}
	nonValues := cli.NewCommand("root").
		Option(cli.Flag("force").Long("force")).
		Option(cli.Count("verbose").Short('v')).
		Argument(cli.Positional("input"))
	if _, err := nonValues.ParseWithValueResolver(
		[]string{"--force", "-vv", "input"},
		nil,
		resolver,
	); err != nil {
		t.Fatal(err)
	}

	terminalActions := cli.NewCommand("root").
		Version("1.0.0").
		Option(cli.ValueOption("config").Long("config"))
	for _, arguments := range [][]string{{"--help"}, {"--version"}} {
		if _, err := terminalActions.ParseWithValueResolver(arguments, nil, resolver); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 0 {
		t.Fatalf("resolver calls = %d, want 0", calls)
	}
}

func TestSensitiveFormatAndDefaultJSONDoNotExposeExternalIdentity(t *testing.T) {
	command := cli.NewCommand("root").Option(
		cli.ValueOption("token").
			Long("token").
			Parser(cli.PossibleValuesParser("accepted")).
			Sensitive(),
	)
	valid, err := command.ParseWithValueResolver(
		nil,
		nil,
		func(cli.ValueResolutionRequest) (cli.ValueResolution, *cli.Diagnostic) {
			return cli.ReplaceValueResolution("private-profile", "accepted"), nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	formatted := fmt.Sprintf("%v", valid.Invocation().ParsedValues("token")[0])
	if strings.Contains(formatted, "accepted") || strings.Contains(formatted, "private-profile") {
		t.Fatalf("formatted value exposed protected data: %q", formatted)
	}
	resolution := cli.ReplaceValueResolution("private-profile", "resolver-secret")
	if formattedResolution := fmt.Sprintf("%v", resolution); strings.Contains(formattedResolution, "resolver-secret") || strings.Contains(formattedResolution, "private-profile") {
		t.Fatalf("formatted resolution exposed its raw value: %q", formattedResolution)
	}

	_, err = command.ParseWithValueResolver(
		nil,
		nil,
		func(cli.ValueResolutionRequest) (cli.ValueResolution, *cli.Diagnostic) {
			return cli.ReplaceValueResolution("private-profile", "rejected"), nil
		},
	)
	if err == nil {
		t.Fatal("invalid external value was accepted")
	}
	diagnostic := err.(*cli.Diagnostic)
	json := (cli.JSONDiagnosticRenderer{}).RenderDiagnostic(diagnostic)
	if strings.Contains(json, "private-profile") || strings.Contains(json, "origin") || strings.Contains(json, "rejected") {
		t.Fatalf("JSON exposed external source metadata: %q", json)
	}
}

func TestValueResolutionOwnsConstructorInputAndAccessorOutput(t *testing.T) {
	input := []string{"one", "two"}
	resolution := cli.ReplaceValueResolution("config", input...)
	input[0] = "mutated"
	output := resolution.Values()
	output[1] = "mutated"
	if got := strings.Join(resolution.Values(), ","); got != "one,two" {
		t.Fatalf("values = %q", got)
	}
}

func TestNilValueResolverBehavesAsNoResolver(t *testing.T) {
	command := cli.NewCommand("root").Option(
		cli.ValueOption("config").Long("config").Default("default"),
	)
	result, err := command.ParseWithValueResolver(nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	values := result.Invocation().ParsedValues("config")
	if len(values) != 1 || values[0].Raw() != "default" || values[0].Source() != cli.SourceDefault {
		t.Fatalf("values = %+v", values)
	}
}

func valueResolutionCommand(arrangement string) *cli.Command {
	switch arrangement {
	case "repeated-replace", "repeated-merge":
		return cli.NewCommand("root").ID("root-id").Option(
			cli.ValueOption("tag").Long("tag").Repeated().Default("base"),
		)
	case "selected":
		return cli.NewCommand("root").
			ID("root-id").
			Option(cli.ValueOption("root-value").Long("root-value")).
			Subcommand(
				cli.NewCommand("unused").
					ID("unused-id").
					Option(cli.ValueOption("ignored").Long("ignored")),
			).
			Subcommand(
				cli.NewCommand("run").
					ID("run-id").
					Option(cli.ValueOption("profile").Long("profile")),
			)
	case "invalid-value":
		return cli.NewCommand("root").ID("root-id").Option(
			cli.ValueOption("config").
				Long("config").
				Parser(cli.PossibleValuesParser("valid")),
		)
	case "sensitive-invalid":
		return cli.NewCommand("root").ID("root-id").Option(
			cli.ValueOption("token").
				Long("token").
				Parser(cli.PossibleValuesParser("valid")).
				Sensitive(),
		)
	default:
		return cli.NewCommand("root").ID("root-id").Option(
			cli.ValueOption("config").
				Long("config").
				Environment("NAGI_CONFIG").
				Default("default"),
		)
	}
}

func resolveValue(
	arrangement string,
	request cli.ValueResolutionRequest,
) (cli.ValueResolution, *cli.Diagnostic) {
	switch arrangement {
	case "single":
		return cli.ReplaceValueResolution("config-file", "external"), nil
	case "unresolved":
		return cli.ValueResolution{}, nil
	case "repeated-replace":
		return cli.ReplaceValueResolution("config-file", "one", "two"), nil
	case "repeated-merge":
		return cli.MergeValueResolution("config-file", "one", "two"), nil
	case "selected":
		switch request.ValueID() {
		case "root-value":
			return cli.ReplaceValueResolution("project-config", "root"), nil
		case "profile":
			return cli.ReplaceValueResolution("project-config", "child"), nil
		default:
			return cli.ValueResolution{}, nil
		}
	case "invalid-multiple":
		return cli.ReplaceValueResolution("config-file", "one", "two"), nil
	case "invalid-merge":
		return cli.MergeValueResolution("config-file", "one"), nil
	case "empty":
		return cli.ReplaceValueResolution("config-file"), nil
	case "invalid-source":
		return cli.ReplaceValueResolution("bad source", "one"), nil
	case "raw-invalid-utf8":
		return cli.ReplaceValueResolution("config-file", string([]byte{0xff, 0xfe, 'A'})), nil
	case "invalid-value":
		return cli.ReplaceValueResolution("config-file", "bad"), nil
	case "sensitive-invalid":
		if !request.IsSensitive() {
			panic("Sensitive metadata was not propagated")
		}
		return cli.ReplaceValueResolution("secret-store", "bad"), nil
	case "resolver-error":
		return cli.ValueResolution{}, cli.NewDiagnostic(
			cli.DiagnosticCode("config-load"),
			"configuration lookup failed",
		).WithCategory(cli.CategoryExecution)
	default:
		panic("unknown arrangement " + arrangement)
	}
}

func snapshotValueResolutionResult(result cli.ParseResult, err error) string {
	if err != nil {
		diagnostic, ok := err.(*cli.Diagnostic)
		if !ok {
			panic(fmt.Sprintf("unexpected error %T", err))
		}
		return snapshotValueResolutionDiagnostic(diagnostic)
	}
	if result.Kind() == cli.ParseHelp {
		return "action;kind=help"
	}
	if result.Kind() == cli.ParseVersion {
		return "action;kind=version"
	}
	invocation := result.Invocation()
	scopes := make([]string, 0, len(invocation.Scopes()))
	for _, scope := range invocation.Scopes() {
		values := make([]string, 0, len(scope.ValueIDs()))
		for _, id := range scope.ValueIDs() {
			parsed := scope.ParsedValues(id)
			snapshots := make([]string, len(parsed))
			for index := range parsed {
				snapshots[index] = snapshotResolvedValue(parsed[index])
			}
			values = append(values, id+"="+strings.Join(snapshots, "+"))
		}
		scopes = append(scopes, strings.Join(scope.CommandIDPath(), "/")+"{"+strings.Join(values, ",")+"}")
	}
	return "ok;scopes=" + strings.Join(scopes, "|")
}

func snapshotResolvedValue(value cli.ParsedValue) string {
	var source string
	switch value.Source() {
	case cli.SourceCommandLine:
		source = "cli"
	case cli.SourceEnvironment:
		identity, _ := value.Origin().Identity()
		source = "env(" + identity + ")"
	case cli.SourceDefault:
		source = "default"
	case cli.SourceExternal:
		identity, _ := value.Origin().Identity()
		source = "external(" + identity + ")"
	default:
		panic("unknown ValueSource")
	}
	return fmt.Sprintf("%s:%X", source, []byte(value.Raw()))
}

func snapshotValueResolutionDiagnostic(diagnostic *cli.Diagnostic) string {
	targets := diagnostic.Targets()
	if len(targets) == 0 {
		panic("fixture diagnostic has no target")
	}
	target := targets[0]
	origin := "none"
	if valueOrigin, ok := target.ValueOrigin(); ok {
		switch valueOrigin.Source() {
		case cli.SourceCommandLine:
			origin = "cli"
		case cli.SourceEnvironment:
			identity, _ := valueOrigin.Identity()
			origin = "env(" + identity + ")"
		case cli.SourceDefault:
			origin = "default"
		case cli.SourceExternal:
			identity, _ := valueOrigin.Identity()
			origin = "external(" + identity + ")"
		}
	}
	return fmt.Sprintf(
		"error;code=%s;target=%s@%s:%s;origin=%s;sensitive=%t",
		diagnostic.Code(),
		target.Kind(),
		strings.Join(target.CommandIDPath(), "/"),
		target.ValueID(),
		origin,
		target.IsSensitive(),
	)
}
