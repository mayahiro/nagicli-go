package status_test

import (
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"

	nagitext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagicli-go/internal/conformance"
	"github.com/mayahiro/nagicli-go/status"
)

func TestStatusReporterMatchesSharedFixtures(t *testing.T) {
	records, err := conformance.Load(
		"cli/status.txt",
		"cli-status",
		"terminal",
		"width",
		"profile",
		"progress-width",
		"kind",
		"tick",
		"current",
		"total",
		"message",
		"repeat",
		"log",
		"end",
		"emitted",
		"output",
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
			statusIO := &memoryIO{terminal: record.Field("terminal") == "yes"}
			if record.Field("width") != "-" {
				statusIO.width = fixtureInt(t, record.Field("width"))
				statusIO.hasWidth = true
			}
			profile := nagitext.ModernWidth()
			switch record.Field("profile") {
			case "modern":
			case "cjk":
				profile = nagitext.CJKWidth()
			default:
				t.Fatal("unknown width profile")
			}
			options := status.Options{}.
				WithProgressWidth(fixtureInt(t, record.Field("progress-width"))).
				WithWidthProfile(profile)
			reporter, err := status.NewWithOptions(statusIO, options)
			if err != nil {
				t.Fatal(err)
			}
			var emitted []bool
			value := fixtureSnapshot(t, record)
			result, err := reporter.Update(value)
			if err != nil {
				t.Fatal(err)
			}
			emitted = append(emitted, result)
			if record.Field("repeat") == "yes" {
				result, err = reporter.Update(value)
				if err != nil {
					t.Fatal(err)
				}
				emitted = append(emitted, result)
			}
			if record.Field("log") != "-" {
				if err := reporter.Log(record.Text("log")); err != nil {
					t.Fatal(err)
				}
			}
			switch record.Field("end") {
			case "none":
			case "clear":
				result, err = reporter.Clear()
				emitted = append(emitted, result)
			case "finish":
				result, err = reporter.Finish(value)
				emitted = append(emitted, result)
			default:
				t.Fatal("unknown fixture end")
			}
			if err != nil {
				t.Fatal(err)
			}
			if got, want := emittedSnapshot(emitted), record.Field("emitted"); got != want {
				t.Fatalf("emitted = %q, want %q", got, want)
			}
			if got, want := statusIO.output.Bytes(), record.Bytes("output"); !bytes.Equal(got, want) {
				t.Fatalf("output = %q, want %q", got, want)
			}
		})
	}
}

func fixtureSnapshot(t *testing.T, record conformance.Record) status.Snapshot {
	t.Helper()
	message := record.Text("message")
	switch record.Field("kind") {
	case "status":
		return status.NewStatus(message)
	case "spinner":
		return status.NewSpinner(fixtureUint64(t, record.Field("tick")), message)
	case "progress":
		return status.NewProgress(
			fixtureUint64(t, record.Field("current")),
			fixtureUint64(t, record.Field("total")),
			message,
		)
	default:
		t.Fatal("unknown snapshot kind")
		return status.Snapshot{}
	}
}

func fixtureInt(t *testing.T, value string) int {
	t.Helper()
	parsed, err := strconv.Atoi(value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func fixtureUint64(t *testing.T, value string) uint64 {
	t.Helper()
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func emittedSnapshot(values []bool) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = strconv.FormatBool(value)
	}
	return strings.Join(parts, ",")
}

type memoryIO struct {
	output   bytes.Buffer
	terminal bool
	width    int
	hasWidth bool
}

func (m *memoryIO) Write(value []byte) (int, error) { return m.output.Write(value) }

func (*memoryIO) Flush() error { return nil }

func (m *memoryIO) IsTerminal() bool { return m.terminal }

func (m *memoryIO) TerminalWidth() (int, bool) { return m.width, m.hasWidth }

var _ status.IO = (*memoryIO)(nil)
var _ io.Writer = (*memoryIO)(nil)
