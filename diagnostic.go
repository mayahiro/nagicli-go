package cli

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// DiagnosticCode is a stable machine-readable framework or application code
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
	// CodeOptionGroup reports an option-group cardinality violation
	CodeOptionGroup DiagnosticCode = "option-group"
	// CodeValidation reports a language-native Invocation validator rejection
	CodeValidation DiagnosticCode = "validation"
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

// DiagnosticCategory is a stable semantic failure category
type DiagnosticCategory string

const (
	// CategorySpecification reports an invalid Command Graph
	CategorySpecification DiagnosticCategory = "specification"
	// CategoryUsage reports invalid command-line usage
	CategoryUsage DiagnosticCategory = "usage"
	// CategoryExecution reports application execution failure
	CategoryExecution DiagnosticCategory = "execution"
	// CategoryCancellation reports cooperative cancellation
	CategoryCancellation DiagnosticCategory = "cancellation"
	// CategoryIO reports an injected I/O failure
	CategoryIO DiagnosticCategory = "io"
)

// DiagnosticTargetKind identifies an option or positional argument
type DiagnosticTargetKind string

const (
	// TargetOption identifies a command option
	TargetOption DiagnosticTargetKind = "option"
	// TargetArgument identifies a positional argument
	TargetArgument DiagnosticTargetKind = "argument"
)

// DiagnosticTarget identifies one command-local option or argument
type DiagnosticTarget struct {
	kind          DiagnosticTargetKind
	commandIDPath []string
	valueID       string
}

// OptionTarget constructs an option target in the current Invocation scope
func OptionTarget(valueID string) DiagnosticTarget {
	return DiagnosticTarget{kind: TargetOption, valueID: valueID}
}

// ArgumentTarget constructs an argument target in the current Invocation scope
func ArgumentTarget(valueID string) DiagnosticTarget {
	return DiagnosticTarget{kind: TargetArgument, valueID: valueID}
}

// WithCommandIDPath returns a copy with an explicit stable command-ID path
func (t DiagnosticTarget) WithCommandIDPath(path ...string) DiagnosticTarget {
	t.commandIDPath = append([]string(nil), path...)
	return t
}

// Kind returns whether this target identifies an option or argument
func (t DiagnosticTarget) Kind() DiagnosticTargetKind { return t.kind }

// CommandIDPath returns a copy of the stable command-ID path
func (t DiagnosticTarget) CommandIDPath() []string {
	return append([]string(nil), t.commandIDPath...)
}

// ValueID returns the command-local value ID
func (t DiagnosticTarget) ValueID() string { return t.valueID }

// Diagnostic is a structured definition, parser, or handler failure
type Diagnostic struct {
	code        DiagnosticCode
	category    DiagnosticCategory
	message     string
	commandPath []string
	usage       string
	usageSet    bool
	targets     []DiagnosticTarget
	hints       []string
}

// NewDiagnostic constructs a Diagnostic with the semantic category for its code
func NewDiagnostic(code DiagnosticCode, message string) *Diagnostic {
	return &Diagnostic{code: code, category: categoryForCode(code), message: message}
}

// WithCategory overrides the semantic category and returns the receiver
func (d *Diagnostic) WithCategory(category DiagnosticCategory) *Diagnostic {
	d.category = category
	return d
}

// WithCommandPath sets the canonical command path and returns the receiver
func (d *Diagnostic) WithCommandPath(path []string) *Diagnostic {
	d.commandPath = append([]string(nil), path...)
	return d
}

// WithUsage sets one present usage line without the usage prefix and returns
// the receiver. An empty string remains present
func (d *Diagnostic) WithUsage(usage string) *Diagnostic {
	d.usage = usage
	d.usageSet = true
	return d
}

// WithTarget appends one structured option or argument target
func (d *Diagnostic) WithTarget(target DiagnosticTarget) *Diagnostic {
	copy := target
	copy.commandIDPath = append([]string(nil), target.commandIDPath...)
	d.targets = append(d.targets, copy)
	return d
}

// WithHint appends one human-readable remediation hint
func (d *Diagnostic) WithHint(hint string) *Diagnostic {
	d.hints = append(d.hints, hint)
	return d
}

// Code returns the stable diagnostic code
func (d *Diagnostic) Code() DiagnosticCode { return d.code }

// Category returns the semantic failure category
func (d *Diagnostic) Category() DiagnosticCategory { return d.category }

// Message returns the human-readable message
func (d *Diagnostic) Message() string { return d.message }

// CommandPath returns a copy of the canonical command path
func (d *Diagnostic) CommandPath() []string {
	return append([]string(nil), d.commandPath...)
}

// Usage returns one usage line without the prefix. It returns an empty string
// for both absent usage and explicitly present empty usage; UsageValue
// distinguishes those states
func (d *Diagnostic) Usage() string { return d.usage }

// UsageValue returns the usage line and whether it is present
func (d *Diagnostic) UsageValue() (string, bool) { return d.usage, d.usageSet }

// Targets returns structured option and argument targets in insertion order
func (d *Diagnostic) Targets() []DiagnosticTarget {
	targets := make([]DiagnosticTarget, len(d.targets))
	for index, target := range d.targets {
		targets[index] = target
		targets[index].commandIDPath = append([]string(nil), target.commandIDPath...)
	}
	return targets
}

// Hints returns human-readable remediation hints in insertion order
func (d *Diagnostic) Hints() []string {
	return append([]string(nil), d.hints...)
}

// Render returns deterministic plain text with one final newline
func (d *Diagnostic) Render() string {
	return DefaultPlainDiagnosticRenderer().RenderDiagnostic(d)
}

// Error implements error without a trailing newline
func (d *Diagnostic) Error() string {
	return strings.TrimSuffix(d.Render(), "\n")
}

func (d *Diagnostic) withCommand(path []string, usage string) *Diagnostic {
	return d.WithCommandPath(path).WithUsage(usage)
}

func (d *Diagnostic) withDefaultTargetPath(path []string) *Diagnostic {
	for index := range d.targets {
		if len(d.targets[index].commandIDPath) == 0 {
			d.targets[index].commandIDPath = append([]string(nil), path...)
		}
	}
	return d
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

func categoryForCode(code DiagnosticCode) DiagnosticCategory {
	switch code {
	case CodeInvalidSpecification:
		return CategorySpecification
	case CodeUnknownOption, CodeUnexpectedOptionValue, CodeMissingOptionValue,
		CodeDuplicateOption, CodeUnknownCommand, CodeMissingSubcommand,
		CodeUnexpectedArgument, CodeMissingRequired, CodeInvalidValue,
		CodeRequires, CodeConflicts, CodeOptionGroup, CodeValidation:
		return CategoryUsage
	case CodeCancelled:
		return CategoryCancellation
	case CodeIOError:
		return CategoryIO
	default:
		return CategoryExecution
	}
}
