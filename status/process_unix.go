//go:build (linux || darwin) && (amd64 || arm64)

package status

import (
	"errors"
	"os"
)

// ProcessIO is Unix status I/O backed by a configured process file
//
// ProcessIO does not close its file and is not safe for concurrent use
type ProcessIO struct {
	output   *os.File
	terminal bool
	access   terminalAccess
}

type terminalAccess interface {
	isTerminal(fd int) bool
	width(fd int) (uint16, error)
}

type unixTerminalAccess struct{}

// NewProcessIO constructs process I/O from an output file
//
// A nil output selects standard error. The caller retains ownership of a
// non-nil file
func NewProcessIO(output *os.File) *ProcessIO {
	if output == nil {
		output = os.Stderr
	}
	return newProcessIO(output, unixTerminalAccess{})
}

func newProcessIO(output *os.File, access terminalAccess) *ProcessIO {
	return &ProcessIO{
		output:   output,
		terminal: access.isTerminal(int(output.Fd())),
		access:   access,
	}
}

// NewProcess constructs a Reporter backed by standard error
func NewProcess() *Reporter { return New(NewProcessIO(nil)) }

// Write writes status output to the configured file
func (p *ProcessIO) Write(value []byte) (int, error) {
	if p == nil || p.output == nil {
		return 0, errors.New("nagi status: nil process output")
	}
	return p.output.Write(value)
}

// Flush completes pending status output
//
// Process files are unbuffered, so Flush has no additional operation
func (*ProcessIO) Flush() error { return nil }

// IsTerminal reports whether the configured output is a terminal
func (p *ProcessIO) IsTerminal() bool {
	return p != nil && p.output != nil && p.terminal
}

// TerminalWidth returns current positive terminal columns when available
func (p *ProcessIO) TerminalWidth() (int, bool) {
	if p == nil || p.output == nil || p.access == nil || !p.terminal {
		return 0, false
	}
	width, err := p.access.width(int(p.output.Fd()))
	return int(width), err == nil && width != 0
}

var _ IO = (*ProcessIO)(nil)
