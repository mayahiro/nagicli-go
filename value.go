package cli

import (
	"fmt"
	"strconv"
	"unicode/utf8"
)

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
	raw    string
	source ValueSource
	typed  any
}

// Raw returns the platform argument bytes as a Go string
func (v ParsedValue) Raw() string {
	return v.raw
}

// Source returns where this value came from
func (v ParsedValue) Source() ValueSource {
	return v.source
}

// Typed returns the language-native parser result
func (v ParsedValue) Typed() any {
	return v.typed
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

// ValueAs returns the first parser result when its dynamic type is T
func ValueAs[T any](invocation *Invocation, id string) (T, bool) {
	var zero T
	values := invocation.ParsedValues(id)
	if len(values) == 0 {
		return zero, false
	}
	value, ok := values[0].typed.(T)
	return value, ok
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
