package cli_test

import (
	"bytes"
	encodinghex "encoding/hex"
	"fmt"
	"io"
	"strings"
	"testing"

	cli "github.com/mayahiro/nagicli-go"
)

func TestResponseFileExpansionMatchesSharedFixtures(t *testing.T) {
	records := loadFixtures(
		t,
		"cli/response-file.txt",
		"cli-response-file",
		"argv", "files", "stdin", "options", "limits", "expected", "reads",
	)
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			reader := &responseFileMemoryReader{files: parseResponseFileFiles(t, record.Field("files"))}
			arguments, err := cli.ExpandResponseFiles(
				parseResponseFileArguments(t, record.Field("argv")),
				"/work",
				parseResponseFileOptions(t, record.Field("options"), record.Field("limits")),
				reader,
				bytes.NewReader(mustResponseFileHex(t, record.Field("stdin"))),
			)
			if got, want := snapshotResponseFile(arguments, err), record.Field("expected"); got != want {
				t.Fatalf("snapshot = %q, want %q", got, want)
			}
			if got, want := strings.Join(reader.reads, ","), record.Field("reads"); got != want {
				t.Fatalf("reads = %q, want %q", got, want)
			}
		})
	}
}

func TestResponseFileDefaultsZeroAndInvalidLimits(t *testing.T) {
	limits := (cli.ResponseFileOptions{}).Limits()
	if limits.MaxDepth() != 16 || limits.MaxSources() != 64 ||
		limits.MaxSourceBytes() != 8*1024*1024 || limits.MaxTokens() != 65_536 ||
		limits.MaxTokenBytes() != 8*1024*1024 {
		t.Fatalf("default limits = %+v", limits)
	}
	if (cli.ResponseFileOptions{}).StandardInputEnabled() {
		t.Fatal("zero options enabled standard input")
	}
	zero := (cli.ResponseFileLimits{}).
		WithMaxDepth(0).
		WithMaxSources(0).
		WithMaxSourceBytes(0).
		WithMaxTokens(0).
		WithMaxTokenBytes(0)
	if zero.MaxDepth() != 0 || zero.MaxSources() != 0 || zero.MaxSourceBytes() != 0 ||
		zero.MaxTokens() != 0 || zero.MaxTokenBytes() != 0 {
		t.Fatalf("explicit zero limits = %+v", zero)
	}
	_, err := cli.ExpandResponseFiles(
		nil,
		"/work",
		(cli.ResponseFileOptions{}).WithLimits(
			(cli.ResponseFileLimits{}).WithMaxDepth(-1),
		),
		nil,
		bytes.NewReader(nil),
	)
	diagnostic, ok := err.(*cli.Diagnostic)
	if !ok || diagnostic.Code() != cli.CodeInvalidSpecification {
		t.Fatalf("negative limit error = %T %v", err, err)
	}
}

func TestOrdinaryParserPreservesLeadingAtWithoutOptIn(t *testing.T) {
	command := cli.NewCommand("root").Argument(cli.Positional("value").Required())
	result, err := command.Parse([]string{"@literal"})
	if err != nil {
		t.Fatal(err)
	}
	invocation := result.Invocation()
	if invocation == nil {
		t.Fatal("parse result is not an Invocation")
	}
	value, present := invocation.RawValue("value")
	if !present || value != "@literal" {
		t.Fatalf("value = %q, %t", value, present)
	}
}

func TestResponseFileReaderIsLazyAndBounded(t *testing.T) {
	calls := 0
	reader := cli.ResponseFileReaderFunc(func(request cli.ResponseFileReadRequest) ([]byte, *cli.Diagnostic) {
		calls++
		if request.Path() != "/work/args" || request.ReadLimit() != 4 {
			t.Fatalf("request = %q, %d", request.Path(), request.ReadLimit())
		}
		return []byte("four"), nil
	})
	_, err := cli.ExpandResponseFiles(
		[]string{"plain", "@args"},
		"/work",
		(cli.ResponseFileOptions{}).WithLimits(
			(cli.ResponseFileLimits{}).WithMaxSourceBytes(3),
		),
		reader,
		bytes.NewReader(nil),
	)
	diagnostic, ok := err.(*cli.Diagnostic)
	if !ok || diagnostic.Code() != cli.CodeResponseFileLimit || calls != 1 {
		t.Fatalf("bounded read = %T %v, calls %d", err, err, calls)
	}

	arguments, err := cli.ExpandResponseFiles(
		[]string{"plain"},
		"/work",
		cli.ResponseFileOptions{},
		nil,
		bytes.NewReader(nil),
	)
	if err != nil || len(arguments) != 1 || arguments[0] != "plain" {
		t.Fatalf("literal expansion = %v, %v", arguments, err)
	}

	_, err = cli.ExpandResponseFiles(
		[]string{"@args"},
		"/work",
		cli.ResponseFileOptions{},
		nil,
		bytes.NewReader(nil),
	)
	diagnostic, ok = err.(*cli.Diagnostic)
	if !ok || diagnostic.Code() != cli.CodeInvalidSpecification {
		t.Fatalf("nil file reader error = %T %v", err, err)
	}
}

func TestResponseFileTargetUsesStableJSONShape(t *testing.T) {
	target := cli.ResponseFileTarget("args.txt").WithCommandIDPath("must-not-apply")
	if len(target.CommandIDPath()) != 0 {
		t.Fatalf("Response File target accepted a command path")
	}
	diagnostic := cli.NewDiagnostic(
		cli.CodeResponseFileSyntax,
		"response file has invalid syntax",
	).WithTarget(target)
	if diagnostic.Category() != cli.CategoryUsage || diagnostic.Targets()[0].Kind() != cli.TargetResponseFile {
		t.Fatalf("diagnostic = %+v", diagnostic)
	}
	want := "{\"schema\":\"nagi.cli.diagnostic.v1\",\"code\":\"response-file-syntax\",\"category\":\"usage\",\"message\":\"response file has invalid syntax\",\"command_path\":[],\"usage\":null,\"targets\":[{\"kind\":\"response-file\",\"command_id_path\":[],\"value_id\":\"args.txt\"}],\"hints\":[]}\n"
	if got := (cli.JSONDiagnosticRenderer{}).RenderDiagnostic(diagnostic); got != want {
		t.Fatalf("JSON = %q, want %q", got, want)
	}
}

func TestRuntimeValidatesGraphBeforeResponseFileIO(t *testing.T) {
	command := cli.NewCommand("root").
		Option(cli.Flag("one").Long("same")).
		Option(cli.Flag("two").Long("same"))
	calls := 0
	runtime := cli.NewContext(
		bytes.NewReader(nil),
		io.Discard,
		io.Discard,
		nil,
		"/work",
	).WithResponseFiles(
		cli.ResponseFileOptions{},
		cli.ResponseFileReaderFunc(func(cli.ResponseFileReadRequest) ([]byte, *cli.Diagnostic) {
			calls++
			return nil, nil
		}),
	)
	outcome, err := command.Run(runtime, []string{"@args"})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status() != cli.StatusUsage || calls != 0 {
		t.Fatalf("outcome = %d, reader calls = %d", outcome.Status(), calls)
	}
}

func TestProcessOptionsComposeResponseFilesAndValueResolution(t *testing.T) {
	responseOptions := (cli.ResponseFileOptions{}).WithStandardInput(true)
	options := cli.DefaultProcessOptions().
		WithResponseFiles(responseOptions).
		WithValueResolver(func(cli.ValueResolutionRequest) (cli.ValueResolution, *cli.Diagnostic) {
			return cli.ValueResolution{}, nil
		})
	configured, enabled := options.ResponseFileOptions()
	if !enabled || !configured.StandardInputEnabled() || options.ValueResolver() == nil {
		t.Fatalf("options did not retain services")
	}
	if options.Policy().RenderDiagnostic(cli.NewDiagnostic(cli.CodeHandlerError, "failed")) == "" {
		t.Fatal("options lost the Runtime Policy")
	}
	options = options.WithoutResponseFiles().WithoutValueResolver()
	if _, enabled := options.ResponseFileOptions(); enabled || options.ValueResolver() != nil {
		t.Fatal("without methods retained services")
	}
}

type responseFileMemoryReader struct {
	files map[string][]byte
	reads []string
}

func (r *responseFileMemoryReader) ReadResponseFile(request cli.ResponseFileReadRequest) ([]byte, *cli.Diagnostic) {
	r.reads = append(r.reads, request.Path())
	if request.Path() == "/work/custom.args" {
		return nil, cli.NewDiagnostic(
			cli.DiagnosticCode("source-read"),
			"custom reader failed",
		)
	}
	contents, present := r.files[request.Path()]
	if !present {
		return nil, cli.NewDiagnostic(
			cli.CodeResponseFileIO,
			"could not read response file",
		)
	}
	length := len(contents)
	if length > request.ReadLimit() {
		length = request.ReadLimit()
	}
	return append([]byte(nil), contents[:length]...), nil
}

func parseResponseFileOptions(t *testing.T, options, limit string) cli.ResponseFileOptions {
	t.Helper()
	var result cli.ResponseFileOptions
	switch options {
	case "default":
	case "stdin":
		result = result.WithStandardInput(true)
	default:
		t.Fatalf("unknown Response File options %q", options)
	}
	var limits cli.ResponseFileLimits
	if limit != "default" {
		name, value, found := strings.Cut(limit, "=")
		if !found {
			t.Fatalf("invalid Response File limit %q", limit)
		}
		var number int
		if _, err := fmt.Sscanf(value, "%d", &number); err != nil {
			t.Fatalf("invalid Response File limit %q", limit)
		}
		switch name {
		case "depth":
			limits = limits.WithMaxDepth(number)
		case "sources":
			limits = limits.WithMaxSources(number)
		case "source-bytes":
			limits = limits.WithMaxSourceBytes(number)
		case "tokens":
			limits = limits.WithMaxTokens(number)
		case "token-bytes":
			limits = limits.WithMaxTokenBytes(number)
		default:
			t.Fatalf("unknown Response File limit %q", name)
		}
	}
	return result.WithLimits(limits)
}

func parseResponseFileFiles(t *testing.T, value string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	if value == "" {
		return files
	}
	for _, entry := range strings.Split(value, ",") {
		path, contents, found := strings.Cut(entry, ":")
		if !found {
			t.Fatalf("invalid fixture file %q", entry)
		}
		files[string(mustResponseFileHex(t, path))] = mustResponseFileHex(t, contents)
	}
	return files
}

func parseResponseFileArguments(t *testing.T, value string) []string {
	t.Helper()
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	arguments := make([]string, len(parts))
	for index, part := range parts {
		arguments[index] = string(mustResponseFileHex(t, part))
	}
	return arguments
}

func snapshotResponseFile(arguments []string, err error) string {
	if err == nil {
		encoded := make([]string, len(arguments))
		for index, argument := range arguments {
			if argument == "" {
				encoded[index] = "-"
			} else {
				encoded[index] = strings.ToUpper(encodinghex.EncodeToString([]byte(argument)))
			}
		}
		return "ok;argv=" + strings.Join(encoded, ",")
	}
	diagnostic, ok := err.(*cli.Diagnostic)
	if !ok {
		panic(fmt.Sprintf("unexpected Response File error %T", err))
	}
	target := "none"
	if targets := diagnostic.Targets(); len(targets) > 0 {
		if targets[0].Kind() != cli.TargetResponseFile || len(targets[0].CommandIDPath()) != 0 ||
			targets[0].IsSensitive() {
			panic(fmt.Sprintf("invalid Response File target %+v", targets[0]))
		}
		if _, present := targets[0].ValueOrigin(); present {
			panic("Response File target has a Value Origin")
		}
		target = targets[0].ValueID()
	}
	return fmt.Sprintf(
		"error;code=%s;target=%s;message=%s",
		diagnostic.Code(),
		target,
		diagnostic.Message(),
	)
}

func mustResponseFileHex(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := encodinghex.DecodeString(value)
	if err != nil {
		t.Fatalf("invalid fixture hex %q: %v", value, err)
	}
	return decoded
}
