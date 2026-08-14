//go:build linux || darwin

package prompt

import (
	"context"
	"errors"
	"io"
	"os"
	"syscall"
	"time"
)

const processWaitInterval = 100 * time.Millisecond

type terminalAccess interface {
	getState(fd int) (terminalState, error)
	setState(fd int, state *terminalState) error
	waitReadable(fd int, timeout time.Duration) (bool, error)
}

type unixTerminalAccess struct{}

// ProcessIO is Unix prompt I/O backed by configured process files
//
// ProcessIO does not close its files and is not safe for concurrent use
type ProcessIO struct {
	input    *os.File
	output   *os.File
	terminal terminalAccess
	buffer   [4_096]byte
	start    int
	end      int
	eof      bool
}

// NewProcessIO constructs process I/O from input and output files
//
// Nil input and output select standard input and standard error respectively.
// The caller retains ownership of non-nil files
func NewProcessIO(input, output *os.File) *ProcessIO {
	if input == nil {
		input = os.Stdin
	}
	if output == nil {
		output = os.Stderr
	}
	return &ProcessIO{input: input, output: output, terminal: unixTerminalAccess{}}
}

// Write writes prompt output to the configured output file
func (p *ProcessIO) Write(value []byte) (int, error) {
	if p == nil || p.output == nil {
		return 0, errors.New("nagi prompt: nil process output")
	}
	return p.output.Write(value)
}

// Flush completes pending prompt output
//
// Process files are unbuffered, so Flush has no additional operation
func (*ProcessIO) Flush() error { return nil }

// IsTerminal reports whether both configured files are terminals
func (p *ProcessIO) IsTerminal() bool {
	if p == nil || p.input == nil || p.output == nil || p.terminal == nil {
		return false
	}
	if _, err := p.getState(int(p.input.Fd())); err != nil {
		return false
	}
	_, err := p.getState(int(p.output.Fd()))
	return err == nil
}

// ReadLine reads one bounded line while observing context cancellation
func (p *ProcessIO) ReadLine(ctx context.Context, mode InputMode, maxBytes int) (ReadResult, error) {
	if p == nil || p.input == nil || p.terminal == nil {
		return ReadResult{}, errors.New("nagi prompt: nil process input")
	}
	if maxBytes <= 0 {
		return ReadResult{}, errors.New("nagi prompt: input limit must be positive")
	}
	ctx = nonNilContext(ctx)
	switch mode {
	case InputVisible:
		return p.readBoundedLine(ctx, maxBytes)
	case InputSecret:
		return p.readSecretLine(ctx, maxBytes)
	default:
		return ReadResult{}, errors.New("nagi prompt: unknown input mode")
	}
}

func (p *ProcessIO) readSecretLine(ctx context.Context, maxBytes int) (result ReadResult, err error) {
	fd := int(p.input.Fd())
	original, err := p.getState(fd)
	if err != nil {
		return ReadResult{}, err
	}
	hidden := original
	hidden.Lflag &^= syscall.ECHO | syscall.ECHONL
	if err := p.setState(fd, &hidden); err != nil {
		return ReadResult{}, err
	}
	defer func() {
		restoreErr := p.setState(fd, &original)
		if restoreErr != nil {
			result = ReadResult{}
			err = restoreErr
		}
	}()
	return p.readBoundedLine(ctx, maxBytes)
}

func (p *ProcessIO) getState(fd int) (terminalState, error) {
	for {
		state, err := p.terminal.getState(fd)
		if !errors.Is(err, syscall.EINTR) {
			return state, err
		}
	}
}

func (p *ProcessIO) setState(fd int, state *terminalState) error {
	for {
		err := p.terminal.setState(fd, state)
		if !errors.Is(err, syscall.EINTR) {
			return err
		}
	}
}

func (p *ProcessIO) readBoundedLine(ctx context.Context, maxBytes int) (ReadResult, error) {
	line := make([]byte, 0, min(maxBytes, 1_024))
	discarded := false
	for {
		for p.start < p.end {
			value := p.buffer[p.start]
			p.start++
			if value == '\n' {
				if len(line) > 0 && line[len(line)-1] == '\r' {
					line = line[:len(line)-1]
				}
				if discarded || len(line) > maxBytes {
					return NewReadResult(ReadInputTooLong), nil
				}
				return ReadResult{kind: ReadLine, line: line}, nil
			}
			if len(line) <= maxBytes {
				line = append(line, value)
			} else {
				discarded = true
			}
		}
		p.start = 0
		p.end = 0
		if p.eof {
			if len(line) == 0 && !discarded {
				return NewReadResult(ReadEndOfFile), nil
			}
			if discarded || len(line) > maxBytes {
				return NewReadResult(ReadInputTooLong), nil
			}
			return ReadResult{kind: ReadLine, line: line}, nil
		}
		if ctx.Err() != nil {
			return NewReadResult(ReadCanceled), nil
		}
		ready, err := p.terminal.waitReadable(int(p.input.Fd()), waitDuration(ctx))
		if err != nil {
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			return ReadResult{}, err
		}
		if !ready {
			continue
		}
		count, err := p.input.Read(p.buffer[:])
		if err != nil {
			if errors.Is(err, io.EOF) {
				p.eof = true
				continue
			}
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			return ReadResult{}, err
		}
		if count == 0 {
			p.eof = true
		} else {
			p.end = count
		}
	}
}

func waitDuration(ctx context.Context) time.Duration {
	wait := processWaitInterval
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining < wait {
			if remaining < 0 {
				return 0
			}
			return remaining
		}
	}
	return wait
}
