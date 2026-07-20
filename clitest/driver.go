package clitest

import (
	"bytes"
	"context"
	"errors"

	cli "github.com/mayahiro/nagicli-go"
)

// Driver configures one process-free command application run
type Driver struct {
	command          *cli.Command
	arguments        []string
	stdin            []byte
	environment      map[string]string
	currentDirectory string
	cancelled        bool
}

// New constructs a Driver with empty input and / as its current directory
func New(command *cli.Command) *Driver {
	return &Driver{
		command:          command,
		environment:      map[string]string{},
		currentDirectory: "/",
	}
}

// Arguments sets arguments after the program name
func (d *Driver) Arguments(arguments ...string) *Driver {
	d.arguments = append([]string(nil), arguments...)
	return d
}

// Stdin sets standard input bytes
func (d *Driver) Stdin(input []byte) *Driver {
	d.stdin = append([]byte(nil), input...)
	return d
}

// Environment adds one environment value
func (d *Driver) Environment(name, value string) *Driver {
	d.environment[name] = value
	return d
}

// CurrentDirectory sets the injected current directory
func (d *Driver) CurrentDirectory(path string) *Driver {
	d.currentDirectory = path
	return d
}

// Cancelled requests cancellation before the handler runs
func (d *Driver) Cancelled(cancelled bool) *Driver {
	d.cancelled = cancelled
	return d
}

// Run executes the application without a child process or signal handler
func (d *Driver) Run() (Result, error) {
	if d.command == nil {
		return Result{}, errors.New("nagi cli test: nil Command")
	}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cancellation, cancel := context.WithCancel(context.Background())
	defer cancel()
	if d.cancelled {
		cancel()
	}
	runtime := cli.NewContextWithCancellation(
		bytes.NewReader(d.stdin),
		stdout,
		stderr,
		d.environment,
		d.currentDirectory,
		cancellation,
	)
	outcome, err := d.command.Run(runtime, d.arguments)
	if err != nil {
		return Result{}, err
	}
	return Result{
		status: outcome.Status(),
		stdout: append([]byte(nil), stdout.Bytes()...),
		stderr: append([]byte(nil), stderr.Bytes()...),
	}, nil
}

// Result contains captured output and the explicit Exit Status
type Result struct {
	status cli.ExitStatus
	stdout []byte
	stderr []byte
}

// Status returns the explicit Exit Status
func (r Result) Status() cli.ExitStatus { return r.status }

// Stdout returns a copy of captured standard output bytes
func (r Result) Stdout() []byte { return append([]byte(nil), r.stdout...) }

// Stderr returns a copy of captured standard error bytes
func (r Result) Stderr() []byte { return append([]byte(nil), r.stderr...) }
