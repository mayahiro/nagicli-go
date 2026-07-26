package cli_test

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	cli "github.com/mayahiro/nagicli-go"
)

func TestCommandSelectionErrorsAreDistinct(t *testing.T) {
	command := cli.NewCommand("root").
		RequireSubcommand().
		Subcommand(cli.NewCommand("child"))

	_, err := command.Parse(nil)
	assertDiagnosticCode(t, err, cli.CodeMissingSubcommand)
	_, err = command.Parse([]string{"other"})
	assertDiagnosticCode(t, err, cli.CodeUnknownCommand)
}

func TestPositionalsDisableLaterSubcommandSelection(t *testing.T) {
	command := cli.NewCommand("root").
		Argument(cli.Positional("values").Repeated()).
		Subcommand(cli.NewCommand("child"))
	result, err := command.Parse([]string{"value", "child"})
	if err != nil {
		t.Fatal(err)
	}
	invocation := result.Invocation()
	if got := invocation.CommandPath(); len(got) != 1 || got[0] != "root" {
		t.Fatalf("command path = %v", got)
	}
	values := invocation.ParsedValues("values")
	if len(values) != 2 || values[1].Raw() != "child" {
		t.Fatalf("values = %v", values)
	}
}

func TestParentOptionsAreNotRecognizedAfterChildSelection(t *testing.T) {
	command := cli.NewCommand("root").
		Option(cli.Flag("root-option").Long("root-option")).
		Subcommand(cli.NewCommand("child"))
	_, err := command.Parse([]string{"child", "--root-option"})
	assertDiagnosticCode(t, err, cli.CodeUnknownOption)
}

func TestCommandLocalValueScopes(t *testing.T) {
	var rootValue, childValue string
	command := cli.NewCommand("root").
		ID("root-id").
		Option(cli.ValueOption("session").Long("session").Default("root")).
		Validator(func(invocation *cli.Invocation) *cli.Diagnostic {
			rootValue, _ = invocation.RawValue("session")
			return nil
		}).
		Subcommand(
			cli.NewCommand("run").
				ID("run-id").
				Option(cli.ValueOption("session").Long("session")).
				Validator(func(invocation *cli.Invocation) *cli.Diagnostic {
					childValue, _ = invocation.RawValue("session")
					return nil
				}),
		)
	result, err := command.Parse([]string{
		"--session", "parent",
		"run",
		"--session", "child",
	})
	if err != nil {
		t.Fatal(err)
	}
	invocation := result.Invocation()
	if rootValue != "parent" || childValue != "child" {
		t.Fatalf("validator scopes = root:%q child:%q", rootValue, childValue)
	}
	if got, _ := invocation.RawValue("session"); got != "child" {
		t.Fatalf("leaf lookup = %q", got)
	}
	root, ok := invocation.Scope("root-id")
	if !ok {
		t.Fatal("root scope was not found")
	}
	if got, _ := root.RawValue("session"); got != "parent" {
		t.Fatalf("root scope lookup = %q", got)
	}
	child, ok := invocation.Scope("root-id", "run-id")
	if !ok || !slices.Equal(child.CommandPath(), []string{"root", "run"}) {
		t.Fatalf("child scope = %+v, %t", child, ok)
	}
	if value, accessErr := cli.RequireValueAs[string](child, "session"); accessErr != nil || value != "child" {
		t.Fatalf("required child value = %q, %v", value, accessErr)
	}
	if _, accessErr := cli.RequireValueAs[int64](child, "session"); accessErr == nil ||
		accessErr.Kind() != cli.ValueTypeMismatch {
		t.Fatalf("type mismatch = %v", accessErr)
	}

	result, err = command.Parse([]string{"--session", "parent", "run"})
	if err != nil {
		t.Fatal(err)
	}
	if _, present := result.Invocation().RawValue("session"); present {
		t.Fatal("an unresolved child declaration did not shadow the parent")
	}
	if len(result.Invocation().Scopes()) != 2 {
		t.Fatalf("scope count = %d", len(result.Invocation().Scopes()))
	}
}

func TestGraphValidationRejectsLocalAndSiblingCollisions(t *testing.T) {
	tests := map[string]*cli.Command{
		"local value ID": cli.NewCommand("root").
			Option(cli.Flag("same").Long("root-option")).
			Argument(cli.Positional("same")),
		"sibling spelling": cli.NewCommand("root").
			Subcommand(cli.NewCommand("first").Alias("shared")).
			Subcommand(cli.NewCommand("shared")),
		"sibling stable ID": cli.NewCommand("root").
			Subcommand(cli.NewCommand("first").ID("shared")).
			Subcommand(cli.NewCommand("second").ID("shared")),
		"non-final repeated positional": cli.NewCommand("root").
			Argument(cli.Positional("many").Repeated()).
			Argument(cli.Positional("last")),
		"cross-command relation": cli.NewCommand("root").
			Option(cli.Flag("parent").Long("parent")).
			Subcommand(cli.NewCommand("child").Option(cli.Flag("child-option").Long("child-option").Requires("parent"))),
		"unknown group option": cli.NewCommand("root").
			Option(cli.Flag("known").Long("known")).
			OptionGroup(cli.ExactlyOne("source", "known", "missing")),
		"duplicate group member": cli.NewCommand("root").
			Option(cli.Flag("known").Long("known")).
			OptionGroup(cli.AtMostOne("source", "known", "known")),
		"duplicate usage variant": cli.NewCommand("root").
			UsageVariant("node", "<NODE>").
			UsageVariant("node", "<X> <Y>"),
		"invalid usage syntax": cli.NewCommand("root").
			UsageVariant("node", "<NODE>\n"),
		"invalid UTF-8 usage syntax": cli.NewCommand("root").
			UsageVariant("node", string([]byte{0xff})),
		"usage variant on required subcommand": cli.NewCommand("root").
			RequireSubcommand().
			UsageVariant("node", "<NODE>").
			Subcommand(cli.NewCommand("child")),
		"generated usage ID collision": cli.NewCommand("root").
			UsageVariant("subcommand", "<NODE>").
			Subcommand(cli.NewCommand("child")),
	}
	for name, command := range tests {
		t.Run(name, func(t *testing.T) {
			assertDiagnosticCode(t, command.Validate(), cli.CodeInvalidSpecification)
		})
	}
}

func TestSuppliedDistinguishesCommandLineFromFallbacks(t *testing.T) {
	command := cli.NewCommand("root").
		Option(cli.ValueOption("mode").Long("mode").Default("auto")).
		Option(cli.Flag("strict").Long("strict").RequiresSupplied("mode")).
		Option(cli.Flag("all").Long("all").ConflictsSupplied("mode"))

	result, err := command.Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Invocation().Contains("mode") || result.Invocation().Supplied("mode") {
		t.Fatalf("default mode presence = resolved:%t supplied:%t",
			result.Invocation().Contains("mode"),
			result.Invocation().Supplied("mode"))
	}

	_, err = command.Parse([]string{"--strict"})
	assertDiagnosticCode(t, err, cli.CodeRequires)

	if _, err = command.Parse([]string{"--all"}); err != nil {
		t.Fatalf("default value triggered supplied conflict: %v", err)
	}
	_, err = command.Parse([]string{"--all", "--mode", "manual"})
	assertDiagnosticCode(t, err, cli.CodeConflicts)

	result, err = command.Parse([]string{"--strict", "--mode", "manual"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Invocation().Supplied("strict") || !result.Invocation().Supplied("mode") {
		t.Fatalf("command-line values were not marked supplied")
	}
}

func TestOptionGroupsUseCommandLinePresenceByDefault(t *testing.T) {
	command := func(group *cli.OptionGroup) *cli.Command {
		return cli.NewCommand("root").
			Option(cli.ValueOption("session").Long("session").Default("default")).
			Option(cli.Flag("all").Long("all")).
			OptionGroup(group)
	}
	if _, err := command(cli.AtMostOne("target", "session", "all")).
		Parse([]string{"--all"}); err != nil {
		t.Fatalf("default value counted as command-line presence: %v", err)
	}
	_, err := command(
		cli.AtMostOne("target", "session", "all").Presence(cli.PresenceResolved),
	).Parse([]string{"--all"})
	assertDiagnosticCode(t, err, cli.CodeOptionGroup)
}

func TestOptionGroupKinds(t *testing.T) {
	tests := []struct {
		name    string
		group   *cli.OptionGroup
		argv    []string
		wantErr bool
	}{
		{"at-most-one valid", cli.AtMostOne("group", "a", "b"), []string{"--a"}, false},
		{"at-most-one invalid", cli.AtMostOne("group", "a", "b"), []string{"--a", "--b"}, true},
		{"exactly-one valid", cli.ExactlyOne("group", "a", "b"), []string{"--b"}, false},
		{"exactly-one invalid", cli.ExactlyOne("group", "a", "b"), nil, true},
		{"at-least-one valid", cli.AtLeastOne("group", "a", "b"), []string{"--a", "--b"}, false},
		{"at-least-one invalid", cli.AtLeastOne("group", "a", "b"), nil, true},
		{"all-or-none empty", cli.AllOrNone("group", "a", "b"), nil, false},
		{"all-or-none complete", cli.AllOrNone("group", "a", "b"), []string{"--a", "--b"}, false},
		{"all-or-none partial", cli.AllOrNone("group", "a", "b"), []string{"--a"}, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := cli.NewCommand("root").
				Option(cli.Flag("a").Long("a")).
				Option(cli.Flag("b").Long("b")).
				OptionGroup(test.group)
			_, err := command.Parse(test.argv)
			if test.wantErr {
				assertDiagnosticCode(t, err, cli.CodeOptionGroup)
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHelpDocumentExposesStructuredAdditions(t *testing.T) {
	command := cli.NewCommand("root").
		UsageVariant("node", "<NODE> [OPTIONS]").
		UsageVariant("coordinates", "<X> <Y> [OPTIONS]").
		Option(cli.Flag("a").Long("a").Conflicts("b")).
		Option(cli.Flag("b").Long("b")).
		OptionGroup(cli.AtMostOne("selection", "a", "b")).
		Example("basic", "root --a").
		Note("Choose one source").
		Link("guide", "https://example.com/guide").
		HelpSection(cli.NewHelpSection("details", "Details").Paragraph("Additional text")).
		Subcommand(cli.NewCommand("child"))
	document, err := command.HelpDocument([]string{"root"})
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Examples()) != 1 || len(document.Notes()) != 1 ||
		len(document.Links()) != 1 || len(document.Sections()) != 1 ||
		len(document.OptionRelations()) != 1 || len(document.OptionGroups()) != 1 ||
		len(document.UsageVariants()) != 3 {
		t.Fatalf("incomplete Help Document: %+v", document)
	}
	usages := document.UsageVariants()
	if usages[0].ID != "node" || usages[0].Syntax != "<NODE> [OPTIONS]" ||
		usages[0].CommandLine != "root <NODE> [OPTIONS]" ||
		document.Usage()[1] != "root <X> <Y> [OPTIONS]" ||
		usages[2].ID != "subcommand" ||
		usages[2].CommandLine != "root [OPTIONS] <COMMAND>" {
		t.Fatalf("Help usage metadata = %+v, lines = %v", usages, document.Usage())
	}
	options := document.Options()
	groups := document.OptionGroups()
	if options[0].ID != "a" || groups[0].OptionIDs[0] != "a" ||
		groups[0].OptionLabels[0] != "--a" {
		t.Fatalf("Help metadata lost stable IDs: options=%+v groups=%+v", options, groups)
	}
	relations := document.OptionRelations()
	if relations[0].SourceID != "a" || relations[0].TargetID != "b" ||
		relations[0].Kind != cli.HelpRelationConflicts {
		t.Fatalf("Help relation metadata = %+v", relations)
	}
}

func TestSubcommandUsagePresentationModes(t *testing.T) {
	child := func() *cli.Command {
		return cli.NewCommand("validate").
			ID("validate-id").
			UsageVariant("file", "--file <FILE>").
			UsageVariant("stdin", "--stdin")
	}
	hidden := cli.NewCommand("root").
		UsageVariant("direct", "<OLD> <NEW>").
		SubcommandUsage(cli.SubcommandUsageHidden).
		Subcommand(child())
	document, err := hidden.HelpDocument([]string{"root"})
	if err != nil {
		t.Fatal(err)
	}
	if variants := document.UsageVariants(); len(variants) != 1 ||
		variants[0].ID != "direct" {
		t.Fatalf("hidden variants = %+v", variants)
	}

	hiddenReservedID := cli.NewCommand("root").
		UsageVariant("subcommand", "<VALUE>").
		SubcommandUsage(cli.SubcommandUsageHidden).
		Subcommand(child())
	if err := hiddenReservedID.Validate(); err != nil {
		t.Fatal(err)
	}

	expanded := cli.NewCommand("root").
		ID("root-id").
		UsageVariant("direct", "<OLD> <NEW>").
		SubcommandUsage(cli.SubcommandUsageExpanded).
		Subcommand(child())
	document, err = expanded.HelpDocument([]string{"root"})
	if err != nil {
		t.Fatal(err)
	}
	variants := document.UsageVariants()
	if len(variants) != 3 ||
		variants[1].CommandLine != "root validate --file <FILE>" ||
		variants[2].CommandLine != "root validate --stdin" ||
		!slices.Equal(variants[1].CommandIDPath, []string{"root-id", "validate-id"}) {
		t.Fatalf("expanded variants = %+v", variants)
	}

	required := cli.NewCommand("root").
		RequireSubcommand().
		SubcommandUsage(cli.SubcommandUsageExpanded).
		Subcommand(child())
	document, err = required.HelpDocument([]string{"root"})
	if err != nil {
		t.Fatal(err)
	}
	if variants := document.UsageVariants(); len(variants) != 2 {
		t.Fatalf("required expanded variants = %+v", variants)
	}
}

func TestUsageVariantsRemainHelpOnly(t *testing.T) {
	generated, err := cli.NewCommand("root").HelpDocument([]string{"root"})
	if err != nil {
		t.Fatal(err)
	}
	if variants := generated.UsageVariants(); len(variants) != 1 ||
		variants[0].ID != "default" || variants[0].Syntax != "[OPTIONS]" {
		t.Fatalf("generated Help usage metadata = %+v", variants)
	}

	command := cli.NewCommand("root").
		UsageVariant("node", "<NODE>").
		Argument(cli.Positional("value").Required())
	document, err := command.HelpDocument([]string{"root"})
	if err != nil {
		t.Fatal(err)
	}
	if got := document.Usage()[0]; got != "root <NODE>" {
		t.Fatalf("Help usage = %q", got)
	}
	_, err = command.Parse(nil)
	var diagnostic *cli.Diagnostic
	if !errors.As(err, &diagnostic) {
		t.Fatalf("parse error = %v, want Diagnostic", err)
	}
	if got := diagnostic.Usage(); got != "root [OPTIONS] <VALUE>" {
		t.Fatalf("Diagnostic usage = %q", got)
	}
}

func TestValidatorReturnsStructuredDiagnostic(t *testing.T) {
	command := cli.NewCommand("root").Validator(func(*cli.Invocation) *cli.Diagnostic {
		return cli.NewDiagnostic(cli.DiagnosticCode("selection-required"), "rejected").
			WithCategory(cli.CategoryUsage).
			WithTarget(cli.OptionTarget("selection")).
			WithHint("choose one selection")
	})
	_, err := command.Parse(nil)
	var diagnostic *cli.Diagnostic
	if !errors.As(err, &diagnostic) || diagnostic.Category() != cli.CategoryUsage {
		t.Fatalf("diagnostic = %v", err)
	}
	if diagnostic.Code() != "selection-required" ||
		len(diagnostic.Targets()) != 1 ||
		len(diagnostic.Hints()) != 1 ||
		!slices.Equal(diagnostic.Targets()[0].CommandIDPath(), []string{"root"}) {
		t.Fatalf("structured diagnostic = %+v", diagnostic)
	}
	if rendered := diagnostic.Render(); !strings.Contains(rendered, "hint: choose one selection\n") {
		t.Fatalf("rendered diagnostic = %q", rendered)
	}
}

func TestParserAndRuntimeCanBeAdoptedInStages(t *testing.T) {
	command := cli.NewCommand("root").
		Argument(cli.Positional("value")).
		Handle(func(context *cli.Context, invocation *cli.Invocation) (cli.Outcome, error) {
			value, _ := invocation.RawValue("value")
			_, err := context.Stdout().Write([]byte(value))
			return cli.Success(), err
		})
	result, err := command.Parse([]string{"ready"})
	if err != nil {
		t.Fatal(err)
	}
	stdout := &bytes.Buffer{}
	context := cli.NewContext(&bytes.Buffer{}, stdout, &bytes.Buffer{}, nil, "/")
	outcome, err := command.RunParsed(context, result)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status() != cli.StatusSuccess || stdout.String() != "ready" {
		t.Fatalf("staged outcome = %d, %q", outcome.Status(), stdout.String())
	}

	diagnostic := cli.NewDiagnostic(cli.DiagnosticCode("selection-required"), "rejected").
		WithCategory(cli.CategoryUsage).
		WithHint("choose one")
	policy := cli.DefaultRuntimePolicy().WithExitCodePolicy(
		cli.DefaultExitCodePolicy().WithStatus(cli.CategoryUsage, 1),
	)
	if policy.StatusForDiagnostic(diagnostic) != 1 ||
		!strings.Contains(policy.RenderDiagnostic(diagnostic), "hint: choose one\n") {
		t.Fatal("parser-only policy helpers did not preserve rendering and status")
	}

	foreignResult, err := cli.NewCommand("root").ID("foreign-root").Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := command.RunParsed(context, foreignResult); err == nil {
		t.Fatal("foreign ParseResult was accepted")
	}
	if _, err := command.RunInvocation(context, foreignResult.Invocation()); err == nil {
		t.Fatal("foreign Invocation was accepted")
	}
}

func TestRuntimeReportsMissingAndGenericHandlerErrors(t *testing.T) {
	for name, command := range map[string]*cli.Command{
		"missing": cli.NewCommand("root"),
		"generic": cli.NewCommand("root").Handle(func(*cli.Context, *cli.Invocation) (cli.Outcome, error) {
			return cli.Outcome{}, errors.New("failed")
		}),
	} {
		t.Run(name, func(t *testing.T) {
			stderr := &bytes.Buffer{}
			runtime := cli.NewContext(bytes.NewReader(nil), &bytes.Buffer{}, stderr, nil, "/")
			outcome, err := command.Run(runtime, nil)
			if err != nil {
				t.Fatal(err)
			}
			if outcome.Status() != cli.StatusFailure {
				t.Fatalf("status = %d", outcome.Status())
			}
			if !bytes.HasPrefix(stderr.Bytes(), []byte("error[")) {
				t.Fatalf("stderr = %q", stderr.Bytes())
			}
		})
	}
}

func TestCancellationAfterHandlerOverridesOnlySuccess(t *testing.T) {
	for name, handlerStatus := range map[string]cli.ExitStatus{
		"success": cli.StatusSuccess,
		"failure": 7,
	} {
		t.Run(name, func(t *testing.T) {
			cancellation, cancel := context.WithCancel(context.Background())
			command := cli.NewCommand("root").Handle(func(*cli.Context, *cli.Invocation) (cli.Outcome, error) {
				cancel()
				return cli.NewOutcome(handlerStatus), nil
			})
			runtime := cli.NewContextWithCancellation(
				bytes.NewReader(nil),
				&bytes.Buffer{},
				&bytes.Buffer{},
				nil,
				"/",
				cancellation,
			)
			outcome, err := command.Run(runtime, nil)
			if err != nil {
				t.Fatal(err)
			}
			want := handlerStatus
			if handlerStatus == cli.StatusSuccess {
				want = cli.StatusCancelled
			}
			if outcome.Status() != want {
				t.Fatalf("status = %d, want %d", outcome.Status(), want)
			}
		})
	}
}

func TestRuntimePolicyMapsCancellation(t *testing.T) {
	cancellation, cancel := context.WithCancel(context.Background())
	cancel()
	command := cli.NewCommand("root").Handle(
		func(*cli.Context, *cli.Invocation) (cli.Outcome, error) {
			t.Fatal("handler ran after cancellation")
			return cli.Success(), nil
		},
	)
	runtime := cli.NewContextWithCancellation(
		bytes.NewReader(nil),
		&bytes.Buffer{},
		&bytes.Buffer{},
		nil,
		"/",
		cancellation,
	)
	policy := cli.DefaultRuntimePolicy().WithExitCodePolicy(
		cli.DefaultExitCodePolicy().WithStatus(cli.CategoryCancellation, 75),
	)
	outcome, err := command.RunWithPolicy(runtime, nil, policy)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status() != 75 {
		t.Fatalf("status = %d, want 75", outcome.Status())
	}
}

func assertDiagnosticCode(t *testing.T, err error, want cli.DiagnosticCode) {
	t.Helper()
	var diagnostic *cli.Diagnostic
	if !errors.As(err, &diagnostic) {
		t.Fatalf("error = %v, want Diagnostic", err)
	}
	if diagnostic.Code() != want {
		t.Fatalf("code = %s, want %s", diagnostic.Code(), want)
	}
}
