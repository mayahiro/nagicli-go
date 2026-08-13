package status_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	nagitext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagicli-go/status"
)

func TestSnapshotAccessors(t *testing.T) {
	if status.NewStatus("Ready").Kind() != status.SnapshotStatus {
		t.Fatal("status kind mismatch")
	}
	if tick, ok := status.NewSpinner(7, "Wait").Tick(); !ok || tick != 7 {
		t.Fatalf("spinner tick = %d, %t", tick, ok)
	}
	if current, total, ok := status.NewProgress(3, 9, "Build").Progress(); !ok || current != 3 || total != 9 {
		t.Fatalf("progress = %d/%d, %t", current, total, ok)
	}
}

func TestInvalidOptions(t *testing.T) {
	for _, options := range []status.Options{
		status.Options{}.WithMaxMessageBytes(-1),
		status.Options{}.WithFallbackTerminalWidth(-1),
		status.Options{}.WithProgressWidth(-1),
		status.Options{}.WithProgressWidth(status.MaxProgressWidth + 1),
	} {
		_, err := status.NewWithOptions(&memoryIO{}, options)
		assertErrorKind(t, err, status.ErrorInvalidStatus)
	}
}

func TestInvalidMessagesDoNotWrite(t *testing.T) {
	for _, message := range []string{"bad\nline", string([]byte{0xff}), string(bytes.Repeat([]byte{'a'}, status.DefaultMaxMessageBytes+1))} {
		statusIO := &memoryIO{terminal: true, width: 80, hasWidth: true}
		reporter := status.New(statusIO)
		_, err := reporter.Update(status.NewStatus(message))
		assertErrorKind(t, err, status.ErrorInvalidStatus)
		if statusIO.output.Len() != 0 {
			t.Fatalf("output = %q", statusIO.output.Bytes())
		}
	}
}

func TestTerminalLogPreservesStatus(t *testing.T) {
	statusIO := &memoryIO{terminal: true, width: 40, hasWidth: true}
	reporter := status.New(statusIO)
	if _, err := reporter.Update(status.NewStatus("Working")); err != nil {
		t.Fatal(err)
	}
	if err := reporter.Log("downloaded"); err != nil {
		t.Fatal(err)
	}
	if !reporter.IsActive() {
		t.Fatal("status must remain active")
	}
	if got, want := statusIO.output.String(), "\r\x1b[2KWorking\r\x1b[2Kdownloaded\n\r\x1b[2KWorking"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestExplicitCustomWidthAppliesToStatusPrefixes(t *testing.T) {
	profile := nagitext.CustomWidth(nagitext.ModernWidth(), func(grapheme string) (int, bool) {
		return 2, grapheme == "-"
	})
	statusIO := &memoryIO{terminal: true, width: 4, hasWidth: true}
	reporter, err := status.NewWithOptions(
		statusIO,
		status.Options{}.WithWidthProfile(profile),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reporter.Update(status.NewSpinner(0, "A")); err != nil {
		t.Fatal(err)
	}
	if got, want := statusIO.output.String(), "\r\x1b[2K- "; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestWriteFailureIsStructured(t *testing.T) {
	reporter := status.New(failingIO{})
	_, err := reporter.Update(status.NewStatus("Working"))
	assertErrorKind(t, err, status.ErrorIO)
	if !errors.Is(err, errWrite) {
		t.Fatalf("error = %v", err)
	}
	if reporter.IsActive() {
		t.Fatal("failed reporter became active")
	}
}

func TestZeroReporterIsInvalid(t *testing.T) {
	var reporter *status.Reporter
	_, err := reporter.Update(status.NewStatus("Working"))
	assertErrorKind(t, err, status.ErrorInvalidStatus)
}

func TestOrdinaryFileIsNotTerminal(t *testing.T) {
	file, err := os.Open("/dev/null")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	processIO := status.NewProcessIO(file)
	if processIO.IsTerminal() {
		t.Fatal("/dev/null was classified as a terminal")
	}
	if width, ok := processIO.TerminalWidth(); ok || width != 0 {
		t.Fatalf("width = %d, %t", width, ok)
	}
}

func assertErrorKind(t *testing.T, err error, want status.ErrorKind) {
	t.Helper()
	var statusErr *status.Error
	if !errors.As(err, &statusErr) {
		t.Fatalf("error = %v, want *status.Error", err)
	}
	if statusErr.Kind() != want {
		t.Fatalf("kind = %v, want %v", statusErr.Kind(), want)
	}
}

var errWrite = errors.New("write failed")

type failingIO struct{}

func (failingIO) Write([]byte) (int, error) { return 0, errWrite }

func (failingIO) Flush() error { return nil }

func (failingIO) IsTerminal() bool { return true }

func (failingIO) TerminalWidth() (int, bool) { return 80, true }

var _ status.IO = failingIO{}
var _ io.Writer = failingIO{}
