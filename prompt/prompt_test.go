package prompt_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/mayahiro/nagicli-go/prompt"
)

func TestPreexistingCancellationWritesNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	promptIO := &memoryIO{input: []byte("yes\n"), terminal: true}
	_, err := prompt.New(promptIO).Confirm(ctx, prompt.NewConfirm("Proceed?"))
	assertErrorKind(t, err, prompt.ErrorCanceled)
	if promptIO.output.Len() != 0 || len(promptIO.modes) != 0 {
		t.Fatal("canceled prompt performed I/O")
	}
}

func TestVisiblePromptCanExplicitlyAllowNonTerminalIO(t *testing.T) {
	promptIO := &memoryIO{input: []byte("yes\n")}
	result, err := prompt.New(promptIO).
		WithTerminalPolicy(prompt.AllowNonTerminal).
		Confirm(context.Background(), prompt.NewConfirm("Proceed?"))
	if err != nil {
		t.Fatal(err)
	}
	if !result {
		t.Fatal("Confirm result = false, want true")
	}
}

func TestSecretAlwaysRejectsNonTerminalIO(t *testing.T) {
	promptIO := &memoryIO{input: []byte("secret\n")}
	_, err := prompt.New(promptIO).
		WithTerminalPolicy(prompt.AllowNonTerminal).
		Secret(context.Background(), prompt.NewSecret("Token"))
	assertErrorKind(t, err, prompt.ErrorNotTerminal)
	if promptIO.output.Len() != 0 {
		t.Fatal("rejected Secret wrote output")
	}
}

func TestInvalidMetadataAndLimitsFailBeforeOutput(t *testing.T) {
	for _, message := range []string{"", "unsafe\nmessage", string([]byte{0xff})} {
		promptIO := &memoryIO{input: []byte("yes\n"), terminal: true}
		_, err := prompt.New(promptIO).Confirm(context.Background(), prompt.NewConfirm(message))
		assertErrorKind(t, err, prompt.ErrorInvalidRequest)
		if promptIO.output.Len() != 0 {
			t.Fatalf("invalid message %q wrote output", message)
		}
	}

	promptIO := &memoryIO{input: []byte("yes\n"), terminal: true}
	_, err := prompt.New(promptIO).
		WithLimits(prompt.Limits{}.WithMaxInputBytes(-1)).
		Confirm(context.Background(), prompt.NewConfirm("Proceed?"))
	assertErrorKind(t, err, prompt.ErrorInvalidRequest)
}

func TestInvalidSelectDefinitionFailsBeforeOutput(t *testing.T) {
	requests := []prompt.Select{
		prompt.NewSelect("Profile"),
		prompt.NewSelect("Profile", "Local").WithDefault(1),
		prompt.NewSelect("Profile", "bad\nchoice"),
	}
	for _, request := range requests {
		promptIO := &memoryIO{input: []byte("1\n"), terminal: true}
		_, err := prompt.New(promptIO).Select(context.Background(), request)
		assertErrorKind(t, err, prompt.ErrorInvalidRequest)
		if promptIO.output.Len() != 0 {
			t.Fatal("invalid Select wrote output")
		}
	}
}

func TestInjectedIOCannotReturnLineBeyondLimit(t *testing.T) {
	promptIO := &memoryIO{input: []byte("abcd\n"), terminal: true}
	_, err := prompt.New(promptIO).
		WithLimits(prompt.Limits{}.WithMaxInputBytes(3)).
		Confirm(context.Background(), prompt.NewConfirm("Proceed?"))
	assertErrorKind(t, err, prompt.ErrorInputTooLong)
}

func TestSecretReadFailureEmitsLineEndingAndPreservesCause(t *testing.T) {
	readErr := errors.New("read failed")
	promptIO := &failingSecretIO{readErr: readErr}
	_, err := prompt.New(promptIO).Secret(context.Background(), prompt.NewSecret("Token"))
	assertErrorKind(t, err, prompt.ErrorIO)
	if !errors.Is(err, readErr) {
		t.Fatalf("error = %v, want read cause", err)
	}
	if got, want := promptIO.output.String(), "Token: \n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestRequestAccessorsReturnDefensiveCopies(t *testing.T) {
	choices := []string{"Local", "Production"}
	request := prompt.NewSelect("Profile", choices...)
	choices[0] = "changed"
	returned := request.Choices()
	returned[1] = "changed"
	if got, want := request.Choices(), []string{"Local", "Production"}; !equalStrings(got, want) {
		t.Fatalf("choices = %v, want %v", got, want)
	}
}

func TestNilPrompterIOIsStructuredError(t *testing.T) {
	_, err := prompt.New(nil).Confirm(context.Background(), prompt.NewConfirm("Proceed?"))
	assertErrorKind(t, err, prompt.ErrorInvalidRequest)
}

func TestUnknownTerminalPolicyIsRejected(t *testing.T) {
	promptIO := &memoryIO{input: []byte("yes\n"), terminal: true}
	_, err := prompt.New(promptIO).
		WithTerminalPolicy(prompt.TerminalPolicy(255)).
		Confirm(context.Background(), prompt.NewConfirm("Proceed?"))
	assertErrorKind(t, err, prompt.ErrorInvalidRequest)
}

func assertErrorKind(t *testing.T, err error, want prompt.ErrorKind) {
	t.Helper()
	var promptErr *prompt.Error
	if !errors.As(err, &promptErr) {
		t.Fatalf("error = %v, want *prompt.Error", err)
	}
	if promptErr.Kind() != want {
		t.Fatalf("kind = %v, want %v", promptErr.Kind(), want)
	}
}

func equalStrings(left, right []string) bool {
	return bytes.Equal([]byte(stringsJoin(left)), []byte(stringsJoin(right)))
}

func stringsJoin(values []string) string {
	var output bytes.Buffer
	for _, value := range values {
		output.WriteString(value)
		output.WriteByte(0)
	}
	return output.String()
}

type failingSecretIO struct {
	output  bytes.Buffer
	readErr error
}

func (f *failingSecretIO) Write(value []byte) (int, error) { return f.output.Write(value) }

func (*failingSecretIO) Flush() error { return nil }

func (*failingSecretIO) IsTerminal() bool { return true }

func (f *failingSecretIO) ReadLine(
	context.Context,
	prompt.InputMode,
	int,
) (prompt.ReadResult, error) {
	return prompt.ReadResult{}, f.readErr
}
