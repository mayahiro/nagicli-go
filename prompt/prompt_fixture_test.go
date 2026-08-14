package prompt_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/mayahiro/nagicli-go/internal/conformance"
	"github.com/mayahiro/nagicli-go/prompt"
)

func TestPromptMatchesSharedFixtures(t *testing.T) {
	records, err := conformance.Load(
		"cli/prompt.txt",
		"cli-prompt",
		"kind", "terminal", "message", "default", "required", "choices", "input", "limit",
		"expected", "output", "modes",
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
			limit, err := strconv.Atoi(record.Field("limit"))
			if err != nil {
				t.Fatal(err)
			}
			promptIO := &memoryIO{
				input:    record.Bytes("input"),
				terminal: record.Field("terminal") == "yes",
			}
			prompter := prompt.New(promptIO).WithLimits(prompt.Limits{}.WithMaxInputBytes(limit))
			result := runFixture(prompter, record)
			if got, want := result, record.Field("expected"); got != want {
				t.Fatalf("result = %q, want %q", got, want)
			}
			if got, want := promptIO.output.Bytes(), record.Bytes("output"); !bytes.Equal(got, want) {
				t.Fatalf("output = %q, want %q", got, want)
			}
			if got, want := snapshotModes(promptIO.modes), record.Field("modes"); got != want {
				t.Fatalf("modes = %q, want %q", got, want)
			}
		})
	}
}

func runFixture(prompter *prompt.Prompter, record conformance.Record) string {
	var value string
	var err error
	switch record.Field("kind") {
	case "confirm":
		request := prompt.NewConfirm(record.Text("message"))
		switch record.Field("default") {
		case "yes":
			request = request.WithDefault(true)
		case "no":
			request = request.WithDefault(false)
		case "-":
		default:
			panic("unknown Confirm default")
		}
		var result bool
		result, err = prompter.Confirm(context.Background(), request)
		value = "bool:" + strconv.FormatBool(result)
	case "select":
		choices := []string(nil)
		if choicesField := record.Text("choices"); choicesField != "" {
			choices = strings.Split(choicesField, "|")
		}
		request := prompt.NewSelect(record.Text("message"), choices...)
		if record.Field("default") != "-" {
			index, parseErr := strconv.Atoi(record.Field("default"))
			if parseErr != nil {
				panic(parseErr)
			}
			request = request.WithDefault(index)
		}
		var result int
		result, err = prompter.Select(context.Background(), request)
		value = "index:" + strconv.Itoa(result)
	case "input":
		request := prompt.NewInput(record.Text("message"))
		if record.Field("default") != "-" {
			request = request.WithDefault(record.Text("default"))
		}
		if record.Field("required") == "yes" {
			request = request.Required()
		}
		var result string
		result, err = prompter.Input(context.Background(), request)
		value = "text:" + result
	case "secret":
		request := prompt.NewSecret(record.Text("message"))
		if record.Field("required") == "yes" {
			request = request.Required()
		}
		var result string
		result, err = prompter.Secret(context.Background(), request)
		value = "text:" + result
	default:
		panic("unknown fixture prompt kind")
	}
	if err == nil {
		return value
	}
	var promptErr *prompt.Error
	if !errors.As(err, &promptErr) {
		panic(err)
	}
	return "error:" + snapshotErrorKind(promptErr.Kind())
}

func snapshotErrorKind(kind prompt.ErrorKind) string {
	switch kind {
	case prompt.ErrorCanceled:
		return "cancelled"
	case prompt.ErrorNotTerminal:
		return "not-terminal"
	case prompt.ErrorInvalidRequest:
		return "invalid-request"
	case prompt.ErrorInputTooLong:
		return "input-too-long"
	case prompt.ErrorIO:
		return "io"
	default:
		panic("unknown prompt ErrorKind")
	}
}

func snapshotModes(modes []prompt.InputMode) string {
	values := make([]string, len(modes))
	for index, mode := range modes {
		switch mode {
		case prompt.InputVisible:
			values[index] = "visible"
		case prompt.InputSecret:
			values[index] = "secret"
		default:
			panic("unknown InputMode")
		}
	}
	return strings.Join(values, ",")
}

type memoryIO struct {
	input    []byte
	position int
	output   bytes.Buffer
	terminal bool
	modes    []prompt.InputMode
}

func (m *memoryIO) Write(value []byte) (int, error) { return m.output.Write(value) }

func (*memoryIO) Flush() error { return nil }

func (m *memoryIO) IsTerminal() bool { return m.terminal }

func (m *memoryIO) ReadLine(
	_ context.Context,
	mode prompt.InputMode,
	maxBytes int,
) (prompt.ReadResult, error) {
	m.modes = append(m.modes, mode)
	if m.position == len(m.input) {
		return prompt.NewReadResult(prompt.ReadEndOfFile), nil
	}
	remaining := m.input[m.position:]
	end := bytes.IndexByte(remaining, '\n')
	if end < 0 {
		end = len(remaining)
	} else {
		end++
	}
	m.position += end
	line := append([]byte(nil), remaining[:end]...)
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
	}
	if len(line) > maxBytes {
		return prompt.NewReadResult(prompt.ReadInputTooLong), nil
	}
	return prompt.NewLineResult(line), nil
}

var _ prompt.IO = (*memoryIO)(nil)
var _ io.Writer = (*memoryIO)(nil)
