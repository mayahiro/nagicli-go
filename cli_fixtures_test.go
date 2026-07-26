package cli_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"testing"

	cli "github.com/mayahiro/nagicli-go"
	"github.com/mayahiro/nagicli-go/internal/conformance"
)

func TestParsingMatchesSharedFixtures(t *testing.T) {
	records := loadFixtures(t, "cli/parsing.txt", "cli-parsing", "argv", "env", "expected")
	command := fixtureCommand()
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			result, err := command.ParseWithEnvironment(
				arguments(record.Bytes("argv")),
				environment(record.Bytes("env")),
			)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := snapshotParse(result), record.Field("expected"); got != want {
				t.Fatalf("snapshot = %q, want %q", got, want)
			}
		})
	}
}

func TestErrorsMatchSharedFixtures(t *testing.T) {
	records := loadFixtures(
		t,
		"cli/errors.txt",
		"cli-errors",
		"argv",
		"env",
		"expected",
		"category",
	)
	command := fixtureCommand()
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			_, err := command.ParseWithEnvironment(
				arguments(record.Bytes("argv")),
				environment(record.Bytes("env")),
			)
			var diagnostic *cli.Diagnostic
			if !errors.As(err, &diagnostic) {
				t.Fatalf("error = %v, want Diagnostic", err)
			}
			if got, want := string(diagnostic.Code()), record.Field("expected"); got != want {
				t.Fatalf("code = %q, want %q", got, want)
			}
			if got, want := string(diagnostic.Category()), record.Field("category"); got != want {
				t.Fatalf("category = %q, want %q", got, want)
			}
		})
	}
}

func TestHelpMatchesSharedFixtures(t *testing.T) {
	records := loadFixtures(t, "cli/help.txt", "cli-help", "path", "expected")
	command := fixtureCommand()
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			path := []string{"nagi"}
			if value := record.Field("path"); value != "" {
				path = append(path, strings.Split(value, "/")...)
			}
			got, err := command.RenderHelp(path)
			if err != nil {
				t.Fatal(err)
			}
			if want := record.Text("expected"); got != want {
				t.Fatalf("help = %q, want %q", got, want)
			}
		})
	}
}

func TestRuntimeMatchesSharedFixtures(t *testing.T) {
	records := loadFixtures(
		t,
		"cli/runtime.txt",
		"cli-runtime",
		"argv",
		"env",
		"stdin",
		"cwd",
		"cancelled",
		"usage-status",
		"error-prefix",
		"show-usage",
		"status",
		"stdout",
		"stderr",
	)
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			cancellation, cancel := context.WithCancel(context.Background())
			defer cancel()
			if record.Field("cancelled") == "true" {
				cancel()
			}
			runtime := cli.NewContextWithCancellation(
				bytes.NewReader(record.Bytes("stdin")),
				stdout,
				stderr,
				environment(record.Bytes("env")),
				record.Field("cwd"),
				cancellation,
			)
			usageStatus, err := strconv.ParseUint(record.Field("usage-status"), 10, 8)
			if err != nil {
				t.Fatal(err)
			}
			exitCodes := cli.DefaultExitCodePolicy().WithStatus(
				cli.CategoryUsage,
				cli.ExitStatus(usageStatus),
			)
			renderer := cli.DefaultPlainDiagnosticRenderer().
				WithPrefix(record.Field("error-prefix")).
				WithUsage(record.Field("show-usage") == "true")
			policy := cli.DefaultRuntimePolicy().
				WithExitCodePolicy(exitCodes).
				WithDiagnosticRenderer(renderer)
			outcome, err := runtimeCommand().RunWithPolicy(
				runtime,
				arguments(record.Bytes("argv")),
				policy,
			)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := fmt.Sprint(outcome.Status()), record.Field("status"); got != want {
				t.Fatalf("status = %s, want %s", got, want)
			}
			if got, want := stdout.Bytes(), record.Bytes("stdout"); !bytes.Equal(got, want) {
				t.Fatalf("stdout = %q, want %q", got, want)
			}
			if got, want := stderr.Bytes(), record.Bytes("stderr"); !bytes.Equal(got, want) {
				t.Fatalf("stderr = %q, want %q", got, want)
			}
		})
	}
}

func TestDefinitionValidationAndTypedValuesArePublic(t *testing.T) {
	invalid := cli.NewCommand("root").
		Option(cli.Flag("first").Long("same")).
		Option(cli.Flag("second").Long("same"))
	err := invalid.Validate()
	var diagnostic *cli.Diagnostic
	if !errors.As(err, &diagnostic) || diagnostic.Code() != cli.CodeInvalidSpecification {
		t.Fatalf("Validate error = %v", err)
	}
	if diagnostic.Category() != cli.CategorySpecification {
		t.Fatalf("Validate category = %q", diagnostic.Category())
	}

	result, err := fixtureCommand().Parse([]string{"serve", "--mode", "http", "--port", "42", "host"})
	if err != nil {
		t.Fatal(err)
	}
	value, ok := cli.ValueAs[int64](result.Invocation(), "port")
	if !ok || value != 42 {
		t.Fatalf("port = %d, %t", value, ok)
	}
}

func fixtureCommand() *cli.Command {
	return cli.NewCommand("nagi").
		About("Nagi fixture command").
		Version("1.2.3").
		Option(cli.Count("verbose").Long("verbose").Short('v').Help("Increase verbosity")).
		Option(cli.ValueOption("output").Long("output").Short('o').Parser(cli.CustomParser("PATH", func(value string) (string, error) {
			return value, nil
		})).Help("Output path")).
		Option(cli.ValueOption("color").Long("color").Parser(cli.PossibleValuesParser("auto", "always", "never")).Default("auto").Help("Color mode")).
		Option(cli.ValueOption("config").Long("config").Short('c').Environment("NAGI_CONFIG").Help("Config path")).
		Option(cli.ValueOption("tag").Long("tag").Short('t').Repeated().Help("Tag value")).
		Option(cli.Flag("force").Long("force").Short('f').Conflicts("dry-run").Help("Force operation")).
		Option(cli.Flag("dry-run").Long("dry-run").Short('n').Help("Dry run")).
		Option(cli.ValueOption("token").Long("token").Requires("config").Help("Token value")).
		OptionGroup(cli.AtMostOne("output-mode", "color", "output")).
		Argument(cli.Positional("input").Help("Input value")).
		Argument(cli.Positional("extra").Repeated().Help("Extra values")).
		Example("basic", "nagi file").
		Note("Values use command line, environment, then default precedence").
		Link("guide", "https://github.com/mayahiro/nagi/blob/main/docs/CLI_API.md").
		HelpSection(
			cli.NewHelpSection("output-formats", "Output formats").
				Entry("plain", "Default deterministic text").
				Paragraph("Custom renderers consume the same Help Document"),
		).
		Validator(func(invocation *cli.Invocation) error {
			input, _ := invocation.RawValue("input")
			if input == "blocked" {
				return cli.NewDiagnostic(cli.CodeValidation, "input 'blocked' is not allowed")
			}
			return nil
		}).
		Subcommand(
			cli.NewCommand("serve").
				Alias("s").
				About("Serve files").
				Option(cli.ValueOption("port").Long("port").Short('p').Parser(cli.IntegerParser()).Default("8080").Help("Port")).
				Option(cli.ValueOption("mode").Long("mode").Short('m').Parser(cli.PossibleValuesParser("http", "https")).Required().Help("Mode")).
				Option(cli.ValueOption("header").Long("header").Short('H').Repeated().Help("Header value")).
				Argument(cli.Positional("host").Required().Help("Host name")),
		)
}

func runtimeCommand() *cli.Command {
	return cli.NewCommand("nagi").
		About("Nagi fixture command").
		Version("1.2.3").
		Option(cli.Count("verbose").Long("verbose").Short('v').Help("Increase verbosity")).
		Option(cli.ValueOption("output").Long("output").Short('o').Parser(cli.CustomParser("PATH", func(value string) (string, error) {
			return value, nil
		})).Help("Output path")).
		Argument(cli.Positional("input").Help("Input value")).
		Argument(cli.Positional("extra").Repeated().Help("Extra values")).
		Handle(func(context *cli.Context, invocation *cli.Invocation) (cli.Outcome, error) {
			input, _ := invocation.RawValue("input")
			if input == "fail" {
				return cli.Outcome{}, cli.NewDiagnostic(cli.CodeHandlerError, "requested failure")
			}
			stdin, err := io.ReadAll(context.Stdin())
			if err != nil {
				return cli.Outcome{}, cli.NewDiagnostic(cli.CodeIOError, err.Error())
			}
			environment, _ := context.Environment("NAGI_TEST")
			_, err = fmt.Fprintf(
				context.Stdout(),
				"command=%s\ncwd=%s\nenv=%s\nstdin=%s\n",
				strings.Join(invocation.CommandPath(), "/"),
				context.CurrentDirectory(),
				environment,
				stdin,
			)
			if err != nil {
				return cli.Outcome{}, cli.NewDiagnostic(cli.CodeIOError, err.Error())
			}
			return cli.Success(), nil
		})
}

func loadFixtures(t *testing.T, relative, suite string, fields ...string) []conformance.Record {
	t.Helper()
	records, err := conformance.Load(relative, suite, fields...)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	return records
}

func arguments(input []byte) []string {
	if len(input) == 0 {
		return nil
	}
	parts := bytes.Split(input, []byte{'|'})
	arguments := make([]string, len(parts))
	for index, part := range parts {
		arguments[index] = string(part)
	}
	return arguments
}

func environment(input []byte) map[string]string {
	values := map[string]string{}
	if len(input) == 0 {
		return values
	}
	for _, entry := range bytes.Split(input, []byte{'|'}) {
		name, value, found := bytes.Cut(entry, []byte{'='})
		if !found {
			panic("fixture environment entry has no equals sign")
		}
		values[string(name)] = string(value)
	}
	return values
}

func snapshotParse(result cli.ParseResult) string {
	switch result.Kind() {
	case cli.ParseHelp:
		return "action;kind=help;command=" + strings.Join(result.CommandPath(), "/")
	case cli.ParseVersion:
		return "action;kind=version;value=" + result.Version()
	case cli.ParseInvocation:
		return snapshotInvocation(result.Invocation())
	default:
		panic("unknown parse result")
	}
}

func snapshotInvocation(invocation *cli.Invocation) string {
	ids := invocation.ValueIDs()
	sort.Strings(ids)
	values := make([]string, 0, len(ids))
	for _, id := range ids {
		if value, ok := invocation.Flag(id); ok {
			values = append(values, fmt.Sprintf("%s=flag:%t", id, value))
			continue
		}
		if value, ok := invocation.Count(id); ok {
			values = append(values, fmt.Sprintf("%s=count:%d", id, value))
			continue
		}
		parsed := invocation.ParsedValues(id)
		if !invocation.IsRepeated(id) {
			values = append(values, fmt.Sprintf("%s=value:%s:%s", id, source(parsed[0].Source()), hex(parsed[0].Raw())))
			continue
		}
		items := make([]string, 0, len(parsed))
		for _, value := range parsed {
			items = append(items, source(value.Source())+":"+hex(value.Raw()))
		}
		values = append(values, id+"=values:"+strings.Join(items, "+"))
	}
	return "ok;command=" + strings.Join(invocation.CommandPath(), "/") + ";values=" + strings.Join(values, ",")
}

func source(value cli.ValueSource) string {
	switch value {
	case cli.SourceCommandLine:
		return "cli"
	case cli.SourceEnvironment:
		return "env"
	case cli.SourceDefault:
		return "default"
	default:
		panic("unknown value source")
	}
}

func hex(value string) string {
	var result strings.Builder
	for _, byteValue := range []byte(value) {
		fmt.Fprintf(&result, "%02X", byteValue)
	}
	return result.String()
}
