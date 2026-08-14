package cli_test

import (
	"bytes"
	"context"
	encodinghex "encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	cli "github.com/mayahiro/nagicli-go"
)

func TestSensitiveValuesMatchSharedFixtures(t *testing.T) {
	records := loadFixtures(
		t,
		"cli/sensitive.txt",
		"cli-sensitive",
		"arrangement",
		"expected",
	)
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			actual := sensitiveArrangement(record.Field("arrangement"))
			if expected := record.Field("expected"); actual != expected {
				t.Fatalf("snapshot = %q, want %q", actual, expected)
			}
		})
	}
}

func TestSensitiveFormattingDoesNotExposeValues(t *testing.T) {
	const secret = "debug-secret-value"
	command := cli.NewCommand("root").Option(
		cli.ValueOption("token").
			Long("token").
			Parser(cli.StringParser()).
			Sensitive(),
	)
	result, err := command.Parse([]string{"--token", secret})
	if err != nil {
		t.Fatal(err)
	}
	invocation := result.Invocation()
	parsed := invocation.ParsedValues("token")[0]
	for name, value := range map[string]any{
		"parsed":           parsed,
		"invocation":       invocation,
		"invocation-value": *invocation,
		"result":           result,
		"command":          command,
		"command-value":    *command,
	} {
		formatted := fmt.Sprintf("%+v", value)
		if strings.Contains(formatted, secret) {
			t.Fatalf("%s formatting leaked the Sensitive value: %s", name, formatted)
		}
	}
	if formatted := fmt.Sprintf("%+v", parsed); !strings.Contains(formatted, cli.RedactedValue) {
		t.Fatalf("ParsedValue formatting = %q, want redaction marker", formatted)
	}

	input := cli.NewCompletionInput([]string{"--token", secret}, "current-secret")
	formatted := fmt.Sprintf("%+v", input)
	if strings.Contains(formatted, secret) || strings.Contains(formatted, "current-secret") {
		t.Fatalf("CompletionInput formatting exposed raw input: %s", formatted)
	}

	engine, err := cli.NewCompletionEngine(command)
	if err != nil {
		t.Fatal(err)
	}
	completion, err := engine.Complete(
		context.Background(),
		cli.NewCompletionInput([]string{"--token"}, secret),
	)
	if err != nil {
		t.Fatal(err)
	}
	formatted = fmt.Sprintf("%+v", completion)
	if strings.Contains(formatted, secret) {
		t.Fatalf("CompletionResult formatting leaked the Sensitive value: %s", formatted)
	}
	if !strings.Contains(formatted, cli.RedactedValue) {
		t.Fatalf("CompletionResult formatting = %q, want redaction marker", formatted)
	}
}

func TestSensitiveJSONDiagnosticKeepsStableSchema(t *testing.T) {
	const secret = "diagnostic-json-secret"
	_, err := cli.NewCommand("root").
		Option(sensitiveToken()).
		Parse([]string{"--token", secret})
	diagnostic := sensitiveDiagnostic(err)
	rendered := (cli.JSONDiagnosticRenderer{}).RenderDiagnostic(diagnostic)
	if !strings.Contains(rendered, cli.RedactedValue) || strings.Contains(rendered, secret) {
		t.Fatalf("JSON Diagnostic did not redact the value: %q", rendered)
	}
	if strings.Contains(rendered, "expected one of") || strings.Contains(rendered, `"sensitive"`) {
		t.Fatalf("JSON Diagnostic changed the stable schema or exposed the parser reason: %q", rendered)
	}
}

func sensitiveArrangement(arrangement string) string {
	switch arrangement {
	case "metadata":
		option := cli.ValueOption("token").Sensitive()
		argument := cli.Positional("credential").Sensitive()
		plain := cli.ValueOption("mode")
		return fmt.Sprintf(
			"option=%t;argument=%t;plain=%t",
			option.IsSensitive(),
			argument.IsSensitive(),
			plain.IsSensitive(),
		)
	case "help":
		return sensitiveHelpSnapshot(false)
	case "inherited-help":
		return sensitiveHelpSnapshot(true)
	case "invalid-option":
		return invalidSensitiveSnapshot(
			cli.NewCommand("root").Option(sensitiveToken()),
			[]string{"--token", "bad"},
			nil,
		)
	case "invalid-attached":
		return invalidSensitiveSnapshot(
			cli.NewCommand("root").Option(sensitiveToken()),
			[]string{"--token=bad"},
			nil,
		)
	case "invalid-environment":
		return invalidSensitiveSnapshot(
			cli.NewCommand("root").Option(sensitiveToken().Environment("NAGI_TOKEN")),
			nil,
			map[string]string{"NAGI_TOKEN": "bad"},
		)
	case "invalid-default":
		return invalidSensitiveSnapshot(
			cli.NewCommand("root").Option(sensitiveToken().Default("bad")),
			nil,
			nil,
		)
	case "invalid-argument":
		return invalidSensitiveSnapshot(
			cli.NewCommand("root").Argument(
				cli.Positional("credential").
					Parser(cli.PossibleValuesParser("accepted")).
					Sensitive(),
			),
			[]string{"bad"},
			nil,
		)
	case "nonsensitive-invalid":
		return invalidSensitiveSnapshot(
			cli.NewCommand("root").Option(
				cli.ValueOption("mode").
					Long("mode").
					Parser(cli.PossibleValuesParser("safe", "fast")),
			),
			[]string{"--mode", "bad"},
			nil,
		)
	case "raw-access":
		command := cli.NewCommand("root").Option(
			cli.ValueOption("token").
				Long("token").
				Parser(cli.StringParser()).
				Sensitive(),
		)
		result, err := command.Parse([]string{"--token", "s3cr3t"})
		if err != nil {
			panic(err)
		}
		parsed := result.Invocation().ParsedValues("token")[0]
		return fmt.Sprintf(
			"raw=%s;typed=%s;sensitive=%t;visible=%t;scope=%t;source=%s",
			encodinghex.EncodeToString([]byte(parsed.Raw())),
			parsed.Typed().(string),
			parsed.IsSensitive(),
			result.Invocation().ValueIsSensitive("token"),
			result.Invocation().CurrentScope().ValueIsSensitive("token"),
			valueSourceName(parsed.Source()),
		)
	case "completion-option":
		return sensitiveOptionCompletionSnapshot()
	case "completion-argument":
		return sensitiveArgumentCompletionSnapshot()
	case "completion-partial":
		return sensitivePartialCompletionSnapshot()
	case "validator-target":
		command := cli.NewCommand("root").
			Option(
				cli.ValueOption("token").
					Long("token").
					Parser(cli.StringParser()).
					Sensitive(),
			).
			Validator(func(*cli.Invocation) *cli.Diagnostic {
				return cli.NewDiagnostic(cli.CodeValidation, "rejected").
					WithTarget(cli.OptionTarget("token"))
			})
		_, err := command.Parse([]string{"--token", "s3cr3t"})
		diagnostic := sensitiveDiagnostic(err)
		return fmt.Sprintf("%t", diagnostic.Targets()[0].IsSensitive())
	case "inherited-validator-target":
		return fmt.Sprintf("%t", inheritedValidatorTargetSnapshot())
	case "handler-target":
		return sensitiveHandlerTargetSnapshot()
	case "unmatched-target":
		return fmt.Sprintf(
			"unknown=%t;mismatched=%t",
			validatorTargetSnapshot(
				cli.OptionTarget("token").WithCommandIDPath("unknown"),
			),
			validatorTargetSnapshot(cli.ArgumentTarget("token")),
		)
	case "invalid-flag":
		err := cli.NewCommand("root").
			Option(cli.Flag("verbose").Long("verbose").Sensitive()).
			Validate()
		return string(sensitiveDiagnostic(err).Code())
	case "invalid-count":
		err := cli.NewCommand("root").
			Option(cli.Count("verbose").Long("verbose").Sensitive()).
			Validate()
		return string(sensitiveDiagnostic(err).Code())
	default:
		panic("unknown Sensitive arrangement " + arrangement)
	}
}

func inheritedValidatorTargetSnapshot() bool {
	command := cli.NewCommand("root").
		Option(
			cli.ValueOption("token").
				Long("token").
				Parser(cli.StringParser()).
				Sensitive().
				Inherited(),
		).
		Subcommand(
			cli.NewCommand("run").Validator(func(*cli.Invocation) *cli.Diagnostic {
				return cli.NewDiagnostic(cli.CodeValidation, "rejected").
					WithTarget(cli.OptionTarget("token").WithCommandIDPath("root"))
			}),
		)
	_, err := command.Parse([]string{"run", "--token", "s3cr3t"})
	return sensitiveDiagnostic(err).Targets()[0].IsSensitive()
}

func validatorTargetSnapshot(target cli.DiagnosticTarget) bool {
	command := cli.NewCommand("root").
		Option(
			cli.ValueOption("token").
				Long("token").
				Parser(cli.StringParser()).
				Sensitive(),
		).
		Validator(func(*cli.Invocation) *cli.Diagnostic {
			return cli.NewDiagnostic(cli.CodeValidation, "rejected").WithTarget(target)
		})
	_, err := command.Parse([]string{"--token", "s3cr3t"})
	return sensitiveDiagnostic(err).Targets()[0].IsSensitive()
}

type sensitiveTargetRenderer struct {
	state *atomic.Int32
}

func (r sensitiveTargetRenderer) RenderDiagnostic(diagnostic *cli.Diagnostic) string {
	value := int32(1)
	if diagnostic.Targets()[0].IsSensitive() {
		value = 2
	}
	r.state.Store(value)
	return ""
}

func sensitiveHandlerTargetSnapshot() string {
	command := cli.NewCommand("root").
		Option(
			cli.ValueOption("token").
				Long("token").
				Parser(cli.StringParser()).
				Sensitive(),
		).
		Handle(func(*cli.Context, *cli.Invocation) (cli.Outcome, error) {
			return cli.Outcome{}, cli.NewDiagnostic(cli.CodeHandlerError, "rejected").
				WithTarget(cli.OptionTarget("token"))
		})
	result, err := command.Parse([]string{"--token", "s3cr3t"})
	if err != nil {
		panic(err)
	}
	var state atomic.Int32
	policy := cli.DefaultRuntimePolicy().WithDiagnosticRenderer(
		sensitiveTargetRenderer{state: &state},
	)
	context := cli.NewContext(
		bytes.NewReader(nil),
		&bytes.Buffer{},
		&bytes.Buffer{},
		nil,
		".",
	)
	if _, err := command.RunInvocationWithPolicy(context, result.Invocation(), policy); err != nil {
		panic(err)
	}
	switch state.Load() {
	case 1:
		return "false"
	case 2:
		return "true"
	default:
		panic("renderer did not observe a handler Diagnostic")
	}
}

func sensitiveToken() *cli.OptionSpec {
	return cli.ValueOption("token").
		Long("token").
		Parser(cli.PossibleValuesParser("accepted")).
		Sensitive()
}

func sensitiveHelpSnapshot(inherited bool) string {
	token := cli.ValueOption("token").
		Long("token").
		Help("Token").
		Parser(cli.PossibleValuesParser("alpha", "beta")).
		Environment("NAGI_TOKEN").
		Default("s3cr3t").
		Sensitive()
	var command *cli.Command
	var path []string
	if inherited {
		command = cli.NewCommand("root").
			Option(token.Inherited()).
			Subcommand(cli.NewCommand("run"))
		path = []string{"root", "run"}
	} else {
		command = cli.NewCommand("root").
			Option(token).
			Option(
				cli.ValueOption("mode").
					Long("mode").
					Help("Mode").
					Parser(cli.PossibleValuesParser("safe", "fast")).
					Default("safe"),
			).
			Argument(cli.Positional("credential").Help("Credential").Sensitive())
		path = []string{"root"}
	}
	document, err := command.HelpDocument(path)
	if err != nil {
		panic(err)
	}
	if inherited {
		entry := document.InheritedOptions()[0]
		return fmt.Sprintf("%t:%s", entry.IsSensitive(), entry.Description)
	}
	entry := func(entries []cli.HelpEntry, id string) cli.HelpEntry {
		for _, entry := range entries {
			if entry.ID == id {
				return entry
			}
		}
		panic("missing Help entry " + id)
	}
	tokenEntry := entry(document.Options(), "token")
	argumentEntry := entry(document.Arguments(), "credential")
	plainEntry := entry(document.Options(), "mode")
	return fmt.Sprintf(
		"option=%t:%s;argument=%t:%s;plain=%t:%s",
		tokenEntry.IsSensitive(),
		tokenEntry.Description,
		argumentEntry.IsSensitive(),
		argumentEntry.Description,
		plainEntry.IsSensitive(),
		plainEntry.Description,
	)
}

func invalidSensitiveSnapshot(
	command *cli.Command,
	arguments []string,
	environment map[string]string,
) string {
	_, err := command.ParseWithEnvironment(arguments, environment)
	diagnostic := sensitiveDiagnostic(err)
	return fmt.Sprintf(
		"message=%s;target=%t",
		diagnostic.Message(),
		diagnostic.Targets()[0].IsSensitive(),
	)
}

func sensitiveOptionCompletionSnapshot() string {
	var calls atomic.Int64
	command := cli.NewCommand("root").Option(
		sensitiveToken().CompletionProvider(
			func(context.Context, cli.CompletionRequest) ([]cli.CompletionCandidate, error) {
				calls.Add(1)
				return []cli.CompletionCandidate{cli.NewCompletionCandidate("accepted")}, nil
			},
		),
	)
	engine, err := cli.NewCompletionEngine(command)
	if err != nil {
		panic(err)
	}
	result, err := engine.Complete(
		context.Background(),
		cli.NewCompletionInput([]string{"--token"}, ""),
	)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf(
		"target=%t;candidates=%d;provider=%d",
		result.Request().Target().IsSensitive(),
		len(result.Candidates()),
		calls.Load(),
	)
}

func sensitiveArgumentCompletionSnapshot() string {
	var calls atomic.Int64
	command := cli.NewCommand("root").Argument(
		cli.Positional("credential").
			Parser(cli.PossibleValuesParser("accepted")).
			Sensitive().
			CompletionProvider(
				func(context.Context, cli.CompletionRequest) ([]cli.CompletionCandidate, error) {
					calls.Add(1)
					return []cli.CompletionCandidate{cli.NewCompletionCandidate("accepted")}, nil
				},
			),
	)
	engine, err := cli.NewCompletionEngine(command)
	if err != nil {
		panic(err)
	}
	result, err := engine.Complete(
		context.Background(),
		cli.NewCompletionInput(nil, "a"),
	)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf(
		"target=%t;candidates=%d;provider=%d",
		result.Request().Target().IsSensitive(),
		len(result.Candidates()),
		calls.Load(),
	)
}

func sensitivePartialCompletionSnapshot() string {
	var calls atomic.Int64
	command := cli.NewCommand("root").
		Option(cli.ValueOption("token").Long("token").Sensitive()).
		Option(
			cli.ValueOption("resource").
				Long("resource").
				CompletionProvider(
					func(_ context.Context, request cli.CompletionRequest) ([]cli.CompletionCandidate, error) {
						occurrence := request.PartialOccurrences()[0]
						raw, present := occurrence.Raw()
						if !present || raw != "s3cr3t" || !occurrence.Target().IsSensitive() {
							panic("provider did not receive the Sensitive partial occurrence")
						}
						calls.Add(1)
						return nil, nil
					},
				),
		)
	engine, err := cli.NewCompletionEngine(command)
	if err != nil {
		panic(err)
	}
	result, err := engine.Complete(
		context.Background(),
		cli.NewCompletionInput([]string{"--token", "s3cr3t", "--resource"}, ""),
	)
	if err != nil {
		panic(err)
	}
	request := result.Request()
	occurrence := request.PartialOccurrences()[0]
	raw, present := occurrence.Raw()
	if !present {
		panic("Sensitive partial occurrence has no raw value")
	}
	return fmt.Sprintf(
		"target=%t;raw=%s;partial-sensitive=%t;provider=%d",
		request.Target().IsSensitive(),
		raw,
		occurrence.Target().IsSensitive(),
		calls.Load(),
	)
}

func sensitiveDiagnostic(err error) *cli.Diagnostic {
	var diagnostic *cli.Diagnostic
	if !errors.As(err, &diagnostic) {
		panic(fmt.Sprintf("error = %v, want Diagnostic", err))
	}
	return diagnostic
}

func valueSourceName(source cli.ValueSource) string {
	switch source {
	case cli.SourceCommandLine:
		return "command-line"
	case cli.SourceEnvironment:
		return "environment"
	case cli.SourceDefault:
		return "default"
	case cli.SourceExternal:
		return "external"
	default:
		panic(fmt.Sprintf("unknown ValueSource %d", source))
	}
}
