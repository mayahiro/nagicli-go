package cli

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// DiagnosticCode is a stable machine-readable failure category
type DiagnosticCode string

const (
	// CodeInvalidSpecification reports an inconsistent Command Graph
	CodeInvalidSpecification DiagnosticCode = "invalid-specification"
	// CodeUnknownOption reports an unknown long or short option
	CodeUnknownOption DiagnosticCode = "unknown-option"
	// CodeUnexpectedOptionValue reports a value attached to a flag or count
	CodeUnexpectedOptionValue DiagnosticCode = "unexpected-option-value"
	// CodeMissingOptionValue reports a Value option without a value
	CodeMissingOptionValue DiagnosticCode = "missing-option-value"
	// CodeDuplicateOption reports a repeated non-repeatable option
	CodeDuplicateOption DiagnosticCode = "duplicate-option"
	// CodeUnknownCommand reports an unknown child command
	CodeUnknownCommand DiagnosticCode = "unknown-command"
	// CodeMissingSubcommand reports a required child command
	CodeMissingSubcommand DiagnosticCode = "missing-subcommand"
	// CodeUnexpectedArgument reports an extra positional value
	CodeUnexpectedArgument DiagnosticCode = "unexpected-argument"
	// CodeMissingRequired reports a missing required value
	CodeMissingRequired DiagnosticCode = "missing-required"
	// CodeInvalidValue reports a Value Parser rejection
	CodeInvalidValue DiagnosticCode = "invalid-value"
	// CodeRequires reports an unsatisfied option requirement
	CodeRequires DiagnosticCode = "requires"
	// CodeConflicts reports two conflicting options
	CodeConflicts DiagnosticCode = "conflicts"
	// CodeMissingHandler reports a selected command without a handler
	CodeMissingHandler DiagnosticCode = "missing-handler"
	// CodeHandlerError reports an application handler failure
	CodeHandlerError DiagnosticCode = "handler-error"
	// CodeCancelled reports cooperative cancellation
	CodeCancelled DiagnosticCode = "cancelled"
	// CodeIOError reports an injected I/O failure
	CodeIOError DiagnosticCode = "io-error"
)

// ExitStatus is a portable process status from 0 through 255
type ExitStatus uint8

const (
	// StatusSuccess indicates successful execution
	StatusSuccess ExitStatus = 0
	// StatusFailure indicates a general application failure
	StatusFailure ExitStatus = 1
	// StatusUsage indicates command syntax or usage failure
	StatusUsage ExitStatus = 2
	// StatusCancelled indicates SIGINT-compatible cancellation
	StatusCancelled ExitStatus = 130
)

// Diagnostic is a structured definition, parser, or handler failure
type Diagnostic struct {
	code        DiagnosticCode
	message     string
	commandPath []string
	usage       string
	status      ExitStatus
}

// NewDiagnostic constructs a Diagnostic with the default status for its code
func NewDiagnostic(code DiagnosticCode, message string) *Diagnostic {
	status := StatusFailure
	switch code {
	case CodeInvalidSpecification, CodeUnknownOption, CodeUnexpectedOptionValue,
		CodeMissingOptionValue, CodeDuplicateOption, CodeUnknownCommand,
		CodeMissingSubcommand, CodeUnexpectedArgument, CodeMissingRequired,
		CodeInvalidValue, CodeRequires, CodeConflicts:
		status = StatusUsage
	case CodeCancelled:
		status = StatusCancelled
	}
	return &Diagnostic{code: code, message: message, status: status}
}

// WithStatus overrides the outcome status and returns the receiver
func (d *Diagnostic) WithStatus(status ExitStatus) *Diagnostic {
	d.status = status
	return d
}

// WithCommandPath sets the canonical command path and returns the receiver
func (d *Diagnostic) WithCommandPath(path []string) *Diagnostic {
	d.commandPath = append([]string(nil), path...)
	return d
}

// WithUsage sets one usage line without the usage prefix and returns the receiver
func (d *Diagnostic) WithUsage(usage string) *Diagnostic {
	d.usage = usage
	return d
}

// Code returns the stable diagnostic code
func (d *Diagnostic) Code() DiagnosticCode { return d.code }

// Message returns the human-readable message
func (d *Diagnostic) Message() string { return d.message }

// CommandPath returns a copy of the canonical command path
func (d *Diagnostic) CommandPath() []string {
	return append([]string(nil), d.commandPath...)
}

// Usage returns one usage line without the prefix
func (d *Diagnostic) Usage() string { return d.usage }

// Status returns the associated Exit Status
func (d *Diagnostic) Status() ExitStatus { return d.status }

// Render returns deterministic plain text with one final newline
func (d *Diagnostic) Render() string {
	result := fmt.Sprintf("error[%s]: %s\n", d.code, d.message)
	if d.usage != "" {
		result += "usage: " + d.usage + "\n"
	}
	return result
}

// Error implements error without a trailing newline
func (d *Diagnostic) Error() string {
	return strings.TrimSuffix(d.Render(), "\n")
}

func (d *Diagnostic) withCommand(path []string, usage string) *Diagnostic {
	return d.WithCommandPath(path).WithUsage(usage)
}

func displayValue(value string) string {
	var result strings.Builder
	for len(value) > 0 {
		r, size := utf8.DecodeRuneInString(value)
		if r == utf8.RuneError && size == 1 {
			fmt.Fprintf(&result, "\\x%02X", value[0])
			value = value[1:]
			continue
		}
		if unicode.IsControl(r) {
			for _, b := range []byte(value[:size]) {
				fmt.Fprintf(&result, "\\x%02X", b)
			}
		} else {
			result.WriteString(value[:size])
		}
		value = value[size:]
	}
	return result.String()
}

func quoteValue(value string) string {
	return "'" + displayValue(value) + "'"
}
