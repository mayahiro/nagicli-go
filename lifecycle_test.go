package cli_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	cli "github.com/mayahiro/nagicli-go"
)

func TestLifecycleMetadataMatchesSharedFixtures(t *testing.T) {
	records := loadFixtures(
		t,
		"cli/lifecycle.txt",
		"cli-lifecycle",
		"arrangement",
		"expected",
	)
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			if got, want := lifecycleArrangement(t, record.Field("arrangement")), record.Text("expected"); got != want {
				t.Fatalf("snapshot = %q, want %q", got, want)
			}
		})
	}
}

func TestHelpPlainRendererMarksDeprecatedEntriesWithoutExposingHiddenEntries(t *testing.T) {
	command := lifecycleCommand()
	root, err := command.RenderHelp([]string{"nagi"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(root, "Old command [deprecated: use nagi run]") ||
		!strings.Contains(root, "Legacy mode [deprecated: use --verbose]") ||
		strings.Contains(root, "internal") {
		t.Fatalf("root Help = %q", root)
	}
	old, err := command.RenderHelp([]string{"nagi", "old"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(old, "Deprecated: use nagi run") {
		t.Fatalf("old Help = %q", old)
	}
	onlyHidden, err := cli.NewCommand("nagi").
		Subcommand(cli.NewCommand("internal").Hidden()).
		RenderHelp([]string{"nagi"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(onlyHidden, "<COMMAND>") || strings.Contains(onlyHidden, "internal") {
		t.Fatalf("only-hidden Help = %q", onlyHidden)
	}
}

func TestInvocationNoticeAccessorsReturnOwnedCopies(t *testing.T) {
	invocation := parsedInvocation(t, lifecycleCommand(), "--legacy", "run", "-m")
	notices := invocation.DeprecationNotices()
	notices[0] = cli.DeprecationNotice{}
	path := invocation.DeprecationNotices()[1].CommandPath()
	path[0] = "changed"
	if got := snapshotNotices(invocation.DeprecationNotices()); !strings.Contains(got, "option@nagi@") ||
		!strings.Contains(got, "option@nagi/run@") {
		t.Fatalf("notices mutated = %q", got)
	}
}

func TestHiddenOptionValueCompletionDoesNotRunItsProvider(t *testing.T) {
	providerCalls := 0
	command := cli.NewCommand("nagi").Option(
		cli.ValueOption("secret").
			Long("secret").
			Hidden().
			CompletionProvider(func(context.Context, cli.CompletionRequest) ([]cli.CompletionCandidate, error) {
				providerCalls++
				return []cli.CompletionCandidate{cli.NewCompletionCandidate("private")}, nil
			}),
	)
	engine, err := cli.NewCompletionEngine(command)
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.Complete(
		context.Background(),
		cli.NewCompletionInput([]string{"--secret"}, ""),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates()) != 0 || providerCalls != 0 {
		t.Fatalf("candidates=%v provider calls=%d", result.Candidates(), providerCalls)
	}
}

func TestNoticeOutputFailurePreventsHandlerExecution(t *testing.T) {
	handlerCalled := false
	command := cli.NewCommand("nagi").
		Option(cli.Count("legacy").Long("legacy").Deprecated("--verbose")).
		Handle(func(*cli.Context, *cli.Invocation) (cli.Outcome, error) {
			handlerCalled = true
			return cli.Success(), nil
		})
	runtimeContext := cli.NewContext(
		bytes.NewReader(nil),
		&bytes.Buffer{},
		lifecycleFailingWriter{},
		nil,
		".",
	)
	_, err := command.RunWithPolicy(
		runtimeContext,
		[]string{"--legacy"},
		cli.DefaultRuntimePolicy().WithDeprecationNoticeRenderer(cli.PlainDeprecationNoticeRenderer{}),
	)
	if err == nil || handlerCalled {
		t.Fatalf("error=%v handler called=%t", err, handlerCalled)
	}
}

func TestInvalidUTF8DeprecationReplacementIsRejected(t *testing.T) {
	err := cli.NewCommand("nagi").Deprecated(string([]byte{0xff})).Validate()
	if err == nil {
		t.Fatal("invalid UTF-8 replacement was accepted")
	}
	var diagnostic *cli.Diagnostic
	if !errors.As(err, &diagnostic) || diagnostic.Code() != cli.CodeInvalidSpecification {
		t.Fatalf("error = %v", err)
	}
}

func lifecycleArrangement(t *testing.T, name string) string {
	t.Helper()
	switch name {
	case "root-help":
		return snapshotLifecycleHelp(t, lifecycleCommand(), "nagi")
	case "old-help":
		return snapshotLifecycleHelp(t, lifecycleCommand(), "nagi", "old")
	case "hidden-help":
		return snapshotLifecycleHelp(t, lifecycleCommand(), "nagi", "internal")
	case "root-completion":
		return snapshotLifecycleCompletion(t, lifecycleCommand(), nil, "")
	case "hidden-completion":
		return snapshotLifecycleCompletion(t, lifecycleCommand(), []string{"internal"}, "")
	case "hidden-value-completion":
		return snapshotLifecycleCompletion(t, lifecycleCommand(), []string{"--secret"}, "")
	case "only-hidden-help":
		return snapshotLifecycleHelp(
			t,
			cli.NewCommand("nagi").Subcommand(cli.NewCommand("internal").Hidden()),
			"nagi",
		)
	case "only-hidden-completion":
		return snapshotLifecycleCompletion(
			t,
			cli.NewCommand("nagi").Subcommand(cli.NewCommand("internal").Hidden()),
			nil,
			"",
		)
	case "hidden-parse":
		invocation := parsedInvocation(t, lifecycleCommand(), "internal", "--trace", "--internal")
		internal, _ := invocation.Flag("internal")
		trace, _ := invocation.Flag("trace")
		return fmt.Sprintf(
			"path=%s;internal=%t;trace=%t;notices=%s",
			strings.Join(invocation.CommandPath(), "/"),
			internal,
			trace,
			snapshotNotices(invocation.DeprecationNotices()),
		)
	case "deprecated-alias":
		return snapshotNotices(parsedInvocation(t, lifecycleCommand(), "o").DeprecationNotices())
	case "deprecated-root":
		return snapshotNotices(parsedInvocation(
			t,
			cli.NewCommand("nagi").ID("root-id").Deprecated("qed").Handle(successHandler),
		).DeprecationNotices())
	case "deprecated-command-order":
		return snapshotNotices(parsedInvocation(
			t,
			lifecycleCommand(),
			"--legacy",
			"old",
		).DeprecationNotices())
	case "deprecated-order":
		return snapshotNotices(parsedInvocation(
			t,
			lifecycleCommand(),
			"--legacy",
			"--legacy",
			"run",
			"-m",
		).DeprecationNotices())
	case "deprecated-default":
		return snapshotNotices(parsedInvocation(
			t,
			cli.NewCommand("nagi").
				Option(cli.ValueOption("old").Long("old").Default("fallback").Deprecated("--new")).
				Handle(successHandler),
		).DeprecationNotices())
	case "deprecated-environment":
		command := cli.NewCommand("nagi").
			Option(cli.ValueOption("old").Long("old").Environment("NAGI_OLD").Deprecated("--new")).
			Handle(successHandler)
		result, err := command.ParseWithEnvironment(nil, map[string]string{"NAGI_OLD": "fallback"})
		if err != nil {
			t.Fatal(err)
		}
		return snapshotNotices(result.Invocation().DeprecationNotices())
	case "runtime-enabled":
		return lifecycleRuntimeOutput(t, true)
	case "runtime-default":
		return lifecycleRuntimeOutput(t, false)
	case "invalid-command":
		return validationCode(t, cli.NewCommand("nagi").Deprecated(""))
	case "invalid-option":
		return validationCode(t, cli.NewCommand("nagi").Option(
			cli.Flag("old").Long("old").Deprecated("bad\nreplacement"),
		))
	default:
		t.Fatalf("unknown lifecycle arrangement %q", name)
		return ""
	}
}

func lifecycleCommand() *cli.Command {
	return cli.NewCommand("nagi").
		ID("root-id").
		Option(cli.Flag("quiet").Long("quiet").Inherited().Help("Quiet output")).
		Option(cli.Flag("internal").Long("internal").Inherited().Hidden().Help("Internal switch")).
		Option(cli.ValueOption("secret").Long("secret").Hidden().Parser(cli.PossibleValuesParser("one", "two"))).
		Option(cli.Count("legacy").Long("legacy").Short('l').Inherited().Deprecated("--verbose").Help("Legacy mode")).
		Option(cli.Count("verbose").Long("verbose").Short('v').Inherited().Help("Verbosity")).
		OptionGroup(cli.AtMostOne("verbosity", "legacy", "verbose")).
		OptionGroup(cli.AtMostOne("internal-pair", "internal", "quiet")).
		Subcommand(cli.NewCommand("old").ID("old-id").Alias("o").About("Old command").Deprecated("nagi run").Handle(successHandler)).
		Subcommand(cli.NewCommand("internal").ID("internal-id").Hidden().About("Internal command").
			Option(cli.Flag("trace").Long("trace").Hidden()).
			Handle(successHandler)).
		Subcommand(cli.NewCommand("run").ID("run-id").About("Run").
			Option(cli.Flag("old-mode").Long("old-mode").Short('m').Deprecated("--mode")).
			Option(cli.Flag("mode").Long("mode")).
			Handle(successHandler))
}

func successHandler(*cli.Context, *cli.Invocation) (cli.Outcome, error) {
	return cli.Success(), nil
}

func parsedInvocation(t *testing.T, command *cli.Command, arguments ...string) *cli.Invocation {
	t.Helper()
	result, err := command.Parse(arguments)
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind() != cli.ParseInvocation || result.Invocation() == nil {
		t.Fatalf("result = %v", result.Kind())
	}
	return result.Invocation()
}

func snapshotLifecycleHelp(t *testing.T, command *cli.Command, path ...string) string {
	t.Helper()
	document, err := command.HelpDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf(
		"command=%s;commands=%s;options=%s;inherited=%s;relations=%s;groups=%s",
		helpDocumentReplacement(document),
		helpEntries(document.Commands()),
		helpEntries(document.Options()),
		inheritedHelpEntries(document.InheritedOptions()),
		helpRelations(document.OptionRelations()),
		helpGroups(document.OptionGroups()),
	)
}

func helpDocumentReplacement(document cli.HelpDocument) string {
	deprecation, ok := document.Deprecation()
	if !ok {
		return "none"
	}
	return deprecation.Replacement()
}

func helpEntries(entries []cli.HelpEntry) string {
	if len(entries) == 0 {
		return "none"
	}
	parts := make([]string, len(entries))
	for index, entry := range entries {
		parts[index] = entry.ID + ":none"
		if deprecation, ok := entry.Deprecation(); ok {
			parts[index] = entry.ID + ":" + deprecation.Replacement()
		}
	}
	return strings.Join(parts, ",")
}

func inheritedHelpEntries(entries []cli.HelpInheritedOption) string {
	if len(entries) == 0 {
		return "none"
	}
	parts := make([]string, len(entries))
	for index, entry := range entries {
		parts[index] = entry.ID + ":none"
		if deprecation, ok := entry.Deprecation(); ok {
			parts[index] = entry.ID + ":" + deprecation.Replacement()
		}
	}
	return strings.Join(parts, ",")
}

func helpRelations(relations []cli.HelpOptionRelation) string {
	if len(relations) == 0 {
		return "none"
	}
	parts := make([]string, len(relations))
	for index, relation := range relations {
		parts[index] = relation.SourceID
	}
	return strings.Join(parts, ",")
}

func helpGroups(groups []cli.HelpOptionGroup) string {
	if len(groups) == 0 {
		return "none"
	}
	parts := make([]string, len(groups))
	for index, group := range groups {
		parts[index] = group.ID
	}
	return strings.Join(parts, ",")
}

func snapshotLifecycleCompletion(
	t *testing.T,
	command *cli.Command,
	arguments []string,
	current string,
) string {
	t.Helper()
	engine, err := cli.NewCompletionEngine(command)
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.Complete(context.Background(), cli.NewCompletionInput(arguments, current))
	if err != nil {
		t.Fatal(err)
	}
	candidates := result.Candidates()
	parts := make([]string, len(candidates))
	for index, candidate := range candidates {
		parts[index] = candidate.Value() + ":none"
		if deprecation, ok := candidate.Deprecation(); ok {
			parts[index] = candidate.Value() + ":" + deprecation.Replacement()
		}
	}
	return strings.Join(parts, ",")
}

func snapshotNotices(notices []cli.DeprecationNotice) string {
	if len(notices) == 0 {
		return "none"
	}
	parts := make([]string, len(notices))
	for index, notice := range notices {
		kind := "option"
		if notice.TargetKind() == cli.DeprecationTargetCommand {
			kind = "command"
		}
		valueID := notice.ValueID()
		if valueID == "" {
			valueID = "-"
		}
		parts[index] = fmt.Sprintf(
			"%s@%s@%s@%s@%s@%s",
			kind,
			strings.Join(notice.CommandPath(), "/"),
			strings.Join(notice.CommandIDPath(), "/"),
			valueID,
			notice.Spelling(),
			notice.Replacement(),
		)
	}
	return strings.Join(parts, ",")
}

func lifecycleRuntimeOutput(t *testing.T, enabled bool) string {
	t.Helper()
	var stderr bytes.Buffer
	runtimeContext := cli.NewContextWithCancellation(
		bytes.NewReader(nil),
		&bytes.Buffer{},
		&stderr,
		nil,
		".",
		context.Background(),
	)
	policy := cli.DefaultRuntimePolicy()
	if enabled {
		policy = policy.WithDeprecationNoticeRenderer(cli.PlainDeprecationNoticeRenderer{})
	}
	outcome, err := lifecycleCommand().RunWithPolicy(
		runtimeContext,
		[]string{"--legacy", "run", "-m"},
		policy,
	)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != cli.Success() {
		t.Fatalf("outcome = %v", outcome.Status())
	}
	return stderr.String()
}

func validationCode(t *testing.T, command *cli.Command) string {
	t.Helper()
	err := command.Validate()
	if err == nil {
		t.Fatal("invalid replacement was accepted")
	}
	var diagnostic *cli.Diagnostic
	if !strings.Contains(err.Error(), "invalid deprecation replacement") ||
		!asDiagnostic(err, &diagnostic) {
		t.Fatalf("error = %v", err)
	}
	return string(diagnostic.Code())
}

func asDiagnostic(err error, target **cli.Diagnostic) bool {
	diagnostic, ok := err.(*cli.Diagnostic)
	if ok {
		*target = diagnostic
	}
	return ok
}

type lifecycleFailingWriter struct{}

func (lifecycleFailingWriter) Write([]byte) (int, error) {
	return 0, errors.New("notice output failed")
}
