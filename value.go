package cli

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// RedactedValue is the stable marker used when a framework projection hides a
// Sensitive value
const RedactedValue = "<redacted>"

// ValueSource identifies where a parsed value came from
type ValueSource uint8

const (
	// SourceCommandLine indicates an argv value
	SourceCommandLine ValueSource = iota
	// SourceEnvironment indicates an injected environment fallback
	SourceEnvironment
	// SourceDefault indicates a command-definition fallback
	SourceDefault
)

// ParsedValue stores one raw value, its source, and its typed parser result
type ParsedValue struct {
	raw       string
	source    ValueSource
	typed     any
	sensitive bool
}

// Raw returns the platform argument bytes as a Go string
func (v ParsedValue) Raw() string {
	return v.raw
}

// Source returns where this value came from
func (v ParsedValue) Source() ValueSource {
	return v.source
}

// IsSensitive reports whether framework-controlled display must redact this
// value
func (v ParsedValue) IsSensitive() bool { return v.sensitive }

// Typed returns the language-native parser result
func (v ParsedValue) Typed() any {
	return v.typed
}

// Format implements fmt.Formatter without exposing a Sensitive raw value or
// any typed parser result
func (v ParsedValue) Format(state fmt.State, _ rune) {
	raw := v.raw
	if v.sensitive {
		raw = RedactedValue
	}
	fmt.Fprintf(
		state,
		"ParsedValue{raw:%q source:%d sensitive:%t}",
		raw,
		v.source,
		v.sensitive,
	)
}

// ValueParser parses one raw option or positional value
type ValueParser interface {
	// Parse returns a typed value or a human-readable validation reason
	Parse(raw string) (any, error)
	// Metavar returns the help placeholder without angle brackets
	Metavar() string
	// PossibleValues returns a documented finite value set when one exists
	PossibleValues() []string
}

type rawValueParser struct{}

func (rawValueParser) Parse(raw string) (any, error) { return raw, nil }
func (rawValueParser) Metavar() string               { return "VALUE" }
func (rawValueParser) PossibleValues() []string      { return nil }

type stringValueParser struct{}

func (stringValueParser) Parse(raw string) (any, error) {
	if !utf8.ValidString(raw) {
		return nil, fmt.Errorf("value is not valid UTF-8")
	}
	return raw, nil
}
func (stringValueParser) Metavar() string          { return "VALUE" }
func (stringValueParser) PossibleValues() []string { return nil }

type integerValueParser struct{}

func (integerValueParser) Parse(raw string) (any, error) {
	if !utf8.ValidString(raw) {
		return nil, fmt.Errorf("integer is not valid UTF-8")
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("value is not a signed 64-bit integer")
	}
	return value, nil
}
func (integerValueParser) Metavar() string          { return "INTEGER" }
func (integerValueParser) PossibleValues() []string { return nil }

type possibleValueParser struct {
	values []string
}

func (p possibleValueParser) Parse(raw string) (any, error) {
	if !utf8.ValidString(raw) {
		return nil, fmt.Errorf("value is not valid UTF-8")
	}
	for _, candidate := range p.values {
		if raw == candidate {
			return raw, nil
		}
	}
	return nil, fmt.Errorf("expected one of %s", joinComma(p.values))
}
func (p possibleValueParser) Metavar() string { return "VALUE" }
func (p possibleValueParser) PossibleValues() []string {
	return append([]string(nil), p.values...)
}

type customValueParser[T any] struct {
	metavar string
	parse   func(string) (T, error)
}

func (p customValueParser[T]) Parse(raw string) (any, error) { return p.parse(raw) }
func (p customValueParser[T]) Metavar() string               { return p.metavar }
func (p customValueParser[T]) PossibleValues() []string      { return nil }

// RawParser returns a parser that preserves arbitrary bytes
func RawParser() ValueParser {
	return rawValueParser{}
}

// StringParser returns a parser that requires valid UTF-8
func StringParser() ValueParser {
	return stringValueParser{}
}

// IntegerParser returns a parser for signed 64-bit decimal integers
func IntegerParser() ValueParser {
	return integerValueParser{}
}

// PossibleValuesParser returns a parser that accepts an exact finite value set
func PossibleValuesParser(values ...string) ValueParser {
	return possibleValueParser{values: append([]string(nil), values...)}
}

// CustomParser adapts a typed Go function into a ValueParser
func CustomParser[T any](metavar string, parse func(string) (T, error)) ValueParser {
	return customValueParser[T]{metavar: metavar, parse: parse}
}

// ValueLookup provides typed value access for an Invocation or exact scope
type ValueLookup interface {
	// ValueScopeIDPath returns the stable command-ID path used by this lookup
	ValueScopeIDPath() []string
	// ParsedValues returns parsed values and sources for one ID
	ParsedValues(id string) []ParsedValue
}

type directValueLookup interface {
	parsedValuesForLookup(id string) []ParsedValue
}

// ValueAccessErrorKind distinguishes missing values and parser type mismatches
type ValueAccessErrorKind uint8

const (
	// ValueMissing means no resolved value exists for the requested ID
	ValueMissing ValueAccessErrorKind = iota
	// ValueTypeMismatch means the parser result does not have the requested type
	ValueTypeMismatch
)

// ValueAccessError reports why required typed value access failed
type ValueAccessError struct {
	kind          ValueAccessErrorKind
	commandIDPath []string
	valueID       string
}

// Kind returns whether the value was missing or had another dynamic type
func (e *ValueAccessError) Kind() ValueAccessErrorKind { return e.kind }

// CommandIDPath returns the stable scope path used for the lookup
func (e *ValueAccessError) CommandIDPath() []string {
	return append([]string(nil), e.commandIDPath...)
}

// ValueID returns the requested command-local value ID
func (e *ValueAccessError) ValueID() string { return e.valueID }

// Error implements error
func (e *ValueAccessError) Error() string {
	reason := "is missing"
	if e.kind == ValueTypeMismatch {
		reason = "has an unexpected parser result type"
	}
	path := strings.Join(e.commandIDPath, "/")
	if path == "" {
		return fmt.Sprintf("value %q %s", e.valueID, reason)
	}
	return fmt.Sprintf("value %q in command scope %q %s", e.valueID, path, reason)
}

// ValueAs returns the first parser result when its dynamic type is T
func ValueAs[T any](values ValueLookup, id string) (T, bool) {
	var zero T
	parsed := lookupParsedValues(values, id)
	if len(parsed) == 0 {
		return zero, false
	}
	value, ok := parsed[0].typed.(T)
	return value, ok
}

// RequireValueAs returns the first parser result or a structured access error
//
// Environment and default values are already resolved before this lookup
func RequireValueAs[T any](values ValueLookup, id string) (T, *ValueAccessError) {
	var zero T
	parsed := lookupParsedValues(values, id)
	if len(parsed) == 0 {
		return zero, &ValueAccessError{
			kind:          ValueMissing,
			commandIDPath: values.ValueScopeIDPath(),
			valueID:       id,
		}
	}
	value, ok := parsed[0].typed.(T)
	if !ok {
		return zero, &ValueAccessError{
			kind:          ValueTypeMismatch,
			commandIDPath: values.ValueScopeIDPath(),
			valueID:       id,
		}
	}
	return value, nil
}

func lookupParsedValues(values ValueLookup, id string) []ParsedValue {
	if direct, ok := values.(directValueLookup); ok {
		return direct.parsedValuesForLookup(id)
	}
	return values.ParsedValues(id)
}

func joinComma(values []string) string {
	if len(values) == 0 {
		return ""
	}
	result := values[0]
	for _, value := range values[1:] {
		result += ", " + value
	}
	return result
}
