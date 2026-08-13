package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	cli "github.com/mayahiro/nagicli-go"
	"github.com/mayahiro/nagicli-go/internal/conformance"
)

func TestJSONDiagnosticRendererMatchesSharedFixtures(t *testing.T) {
	records, err := conformance.Load(
		"cli/diagnostic-json.txt",
		"cli-diagnostic-json",
		"arrangement",
		"expected",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		record := record
		t.Run(record.ID, func(t *testing.T) {
			got := (cli.JSONDiagnosticRenderer{}).RenderDiagnostic(
				diagnosticJSONArrangement(t, record.Field("arrangement")),
			)
			if want := record.Text("expected"); got != want {
				t.Fatalf("JSON = %q, want %q", got, want)
			}
			if !json.Valid([]byte(strings.TrimSuffix(got, "\n"))) {
				t.Fatalf("invalid JSON = %q", got)
			}
		})
	}
}

func TestJSONDiagnosticRendererNormalizesInvalidUTF8(t *testing.T) {
	diagnostic := cli.NewDiagnostic(cli.DiagnosticCode("bad"), string([]byte{0xff, 0xfe, 'A'})).
		WithCommandPath([]string{string([]byte{0xff}), "root"}).
		WithHint(string([]byte{0xff, 0xfe}))
	got := (cli.JSONDiagnosticRenderer{}).RenderDiagnostic(diagnostic)
	if !json.Valid([]byte(strings.TrimSuffix(got, "\n"))) {
		t.Fatalf("invalid JSON = %q", got)
	}
	if strings.Count(got, "�") != 3 {
		t.Fatalf("invalid UTF-8 normalization = %q", got)
	}
}

func TestRuntimePolicyWritesJSONWithoutChangingStatusMeaning(t *testing.T) {
	command := cli.NewCommand("root").Argument(cli.Positional("value").Required())
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	runtime := cli.NewContextWithCancellation(
		bytes.NewReader(nil),
		stdout,
		stderr,
		nil,
		".",
		context.Background(),
	)
	policy := cli.DefaultRuntimePolicy().WithDiagnosticRenderer(cli.JSONDiagnosticRenderer{})
	outcome, err := command.RunWithPolicy(runtime, nil, policy)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status() != cli.StatusUsage {
		t.Fatalf("status = %d", outcome.Status())
	}
	var document struct {
		Schema      string   `json:"schema"`
		Code        string   `json:"code"`
		CommandPath []string `json:"command_path"`
	}
	if err := json.Unmarshal(bytes.TrimSuffix(stderr.Bytes(), []byte{'\n'}), &document); err != nil {
		t.Fatal(err)
	}
	if document.Schema != cli.JSONDiagnosticSchema ||
		document.Code != string(cli.CodeMissingRequired) ||
		len(document.CommandPath) != 1 ||
		document.CommandPath[0] != "root" {
		t.Fatalf("document = %+v", document)
	}
}

func TestUsageValueDistinguishesAbsentAndPresentEmpty(t *testing.T) {
	diagnostic := cli.NewDiagnostic(cli.CodeHandlerError, "failed")
	if usage, ok := diagnostic.UsageValue(); ok || usage != "" {
		t.Fatalf("unset usage = %q, %t", usage, ok)
	}
	diagnostic.WithUsage("")
	if usage, ok := diagnostic.UsageValue(); !ok || usage != "" {
		t.Fatalf("present empty usage = %q, %t", usage, ok)
	}
	if got := cli.DefaultPlainDiagnosticRenderer().RenderDiagnostic(diagnostic); !strings.HasSuffix(got, "usage: \n") {
		t.Fatalf("plain usage presence parity = %q", got)
	}
}

func diagnosticJSONArrangement(t *testing.T, name string) *cli.Diagnostic {
	t.Helper()
	switch name {
	case "minimal":
		return cli.NewDiagnostic(cli.CodeHandlerError, "failed")
	case "complete":
		return cli.NewDiagnostic(cli.DiagnosticCode("profile-blocked"), `profile "prod" blocked`).
			WithCategory(cli.CategoryUsage).
			WithCommandPath([]string{"nagi", "deploy"}).
			WithUsage("nagi deploy --profile <PROFILE>").
			WithTarget(cli.OptionTarget("profile").WithCommandIDPath("root", "deploy")).
			WithTarget(cli.ArgumentTarget("target").WithCommandIDPath("root", "deploy")).
			WithHint("choose staging").
			WithHint("see https://example.com/a?x=1&y=2")
	case "escaping":
		return cli.NewDiagnostic(
			cli.CodeHandlerError,
			"line\n\t\"\\\b\f\r\x00<>&/\u2028",
		).
			WithCommandPath([]string{"a\\b", "日本"}).
			WithUsage("").
			WithTarget(cli.OptionTarget("x\n")).
			WithHint("\x1f")
	default:
		t.Fatalf("unknown arrangement %q", name)
		return nil
	}
}
