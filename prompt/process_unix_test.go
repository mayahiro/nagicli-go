//go:build linux || darwin

package prompt

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestProcessSecretReadRestoresCompleteTerminalState(t *testing.T) {
	input, writer := pipeWithInput(t, []byte("secret\n"))
	defer input.Close()
	defer writer.Close()
	output, outputWriter := pipeWithInput(t, nil)
	defer output.Close()
	defer outputWriter.Close()
	initial := terminalState{Lflag: syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG}
	terminal := &fakeTerminal{state: initial, ready: true}
	promptIO := &ProcessIO{input: input, output: outputWriter, terminal: terminal}
	result, err := promptIO.ReadLine(context.Background(), InputSecret, DefaultMaxInputBytes)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(result.line); got != "secret" {
		t.Fatalf("line = %q, want secret", got)
	}
	if len(terminal.sets) != 2 {
		t.Fatalf("set calls = %d, want 2", len(terminal.sets))
	}
	if terminal.sets[0].Lflag&(syscall.ECHO|syscall.ECHONL) != 0 {
		t.Fatal("secret state retained echo flags")
	}
	if terminal.state != initial {
		t.Fatal("terminal state was not completely restored")
	}
}

func TestProcessSecretReadRestoresStateAfterPanic(t *testing.T) {
	input, writer := pipeWithInput(t, nil)
	defer input.Close()
	defer writer.Close()
	initial := terminalState{Lflag: syscall.ECHO | syscall.ECHONL | syscall.ICANON}
	terminal := &fakeTerminal{state: initial, panicWait: true}
	promptIO := &ProcessIO{input: input, output: writer, terminal: terminal}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("ReadLine did not panic")
			}
		}()
		_, _ = promptIO.ReadLine(context.Background(), InputSecret, DefaultMaxInputBytes)
	}()
	if terminal.state != initial {
		t.Fatal("terminal state was not restored during panic")
	}
}

func TestProcessReadObservesCancellationWithoutInput(t *testing.T) {
	input, writer := pipeWithInput(t, nil)
	defer input.Close()
	defer writer.Close()
	terminal := &fakeTerminal{state: terminalState{}, wait: true}
	promptIO := &ProcessIO{input: input, output: writer, terminal: terminal}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	started := time.Now()
	result, err := promptIO.ReadLine(ctx, InputVisible, DefaultMaxInputBytes)
	if err != nil {
		t.Fatal(err)
	}
	if result.kind != ReadCanceled {
		t.Fatalf("kind = %v, want ReadCanceled", result.kind)
	}
	if elapsed := time.Since(started); elapsed >= 500*time.Millisecond {
		t.Fatalf("cancellation latency = %v", elapsed)
	}
}

func TestProcessReadDrainsOversizedLine(t *testing.T) {
	input, writer := pipeWithInput(t, []byte("abcd\nok\n"))
	defer input.Close()
	defer writer.Close()
	terminal := &fakeTerminal{ready: true}
	promptIO := &ProcessIO{input: input, output: writer, terminal: terminal}
	first, err := promptIO.ReadLine(context.Background(), InputVisible, 3)
	if err != nil {
		t.Fatal(err)
	}
	if first.kind != ReadInputTooLong {
		t.Fatalf("first kind = %v, want ReadInputTooLong", first.kind)
	}
	second, err := promptIO.ReadLine(context.Background(), InputVisible, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(second.line); got != "ok" {
		t.Fatalf("second line = %q, want ok", got)
	}
}

func TestProcessSecretRestoreFailureIsReturned(t *testing.T) {
	input, writer := pipeWithInput(t, []byte("secret\n"))
	defer input.Close()
	defer writer.Close()
	restoreErr := errors.New("restore failed")
	terminal := &fakeTerminal{
		state:     terminalState{Lflag: syscall.ECHO},
		ready:     true,
		failSetAt: 2,
		setError:  restoreErr,
	}
	promptIO := &ProcessIO{input: input, output: writer, terminal: terminal}
	_, err := promptIO.ReadLine(context.Background(), InputSecret, DefaultMaxInputBytes)
	if !errors.Is(err, restoreErr) {
		t.Fatalf("error = %v, want restore failure", err)
	}
}

func TestProcessReadRejectsInvalidModeAndLimit(t *testing.T) {
	input, writer := pipeWithInput(t, nil)
	defer input.Close()
	defer writer.Close()
	promptIO := &ProcessIO{input: input, output: writer, terminal: &fakeTerminal{}}
	if _, err := promptIO.ReadLine(context.Background(), InputVisible, 0); err == nil {
		t.Fatal("zero input limit succeeded")
	}
	if _, err := promptIO.ReadLine(context.Background(), InputMode(255), 1); err == nil {
		t.Fatal("unknown input mode succeeded")
	}
}

type fakeTerminal struct {
	state     terminalState
	sets      []terminalState
	ready     bool
	wait      bool
	panicWait bool
	failSetAt int
	setError  error
}

func (f *fakeTerminal) getState(int) (terminalState, error) { return f.state, nil }

func (f *fakeTerminal) setState(_ int, state *terminalState) error {
	f.sets = append(f.sets, *state)
	if f.failSetAt == len(f.sets) {
		return f.setError
	}
	f.state = *state
	return nil
}

func (f *fakeTerminal) waitReadable(_ int, timeout time.Duration) (bool, error) {
	if f.panicWait {
		panic("wait panic")
	}
	if f.wait {
		time.Sleep(timeout)
	}
	return f.ready, nil
}

func pipeWithInput(t *testing.T, input []byte) (*os.File, *os.File) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if len(input) > 0 {
		if _, err := writer.Write(input); err != nil {
			t.Fatal(err)
		}
	}
	return reader, writer
}
