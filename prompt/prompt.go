// Package prompt provides lightweight line-oriented interactive prompts above
// Nagi CLI Core
package prompt

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	nagitext "github.com/mayahiro/nagi-go/text"
)

const (
	// DefaultMaxInputBytes is the default response byte limit before a line ending
	DefaultMaxInputBytes = 65_536
	// DefaultMaxChoices is the default maximum Select choice count
	DefaultMaxChoices = 1_000
)

// InputMode says whether a line read is visible or must hide terminal echo
type InputMode uint8

const (
	// InputVisible leaves terminal echo unchanged
	InputVisible InputMode = iota
	// InputSecret hides terminal echo for the read
	InputSecret
)

// ReadResultKind classifies one injected bounded line read
type ReadResultKind uint8

const (
	// ReadLine contains a complete line without its line ending
	ReadLine ReadResultKind = iota
	// ReadEndOfFile means no byte was available before input ended
	ReadEndOfFile
	// ReadInputTooLong means a line exceeded its limit and was drained
	ReadInputTooLong
	// ReadCanceled means caller or terminal cancellation interrupted the read
	ReadCanceled
)

// ReadResult is one result returned by injected prompt I/O
type ReadResult struct {
	kind ReadResultKind
	line []byte
}

// NewLineResult constructs a complete line result and copies its bytes
func NewLineResult(line []byte) ReadResult {
	return ReadResult{kind: ReadLine, line: append([]byte(nil), line...)}
}

// NewReadResult constructs a result without line bytes
func NewReadResult(kind ReadResultKind) ReadResult { return ReadResult{kind: kind} }

// Kind returns the line read outcome
func (r ReadResult) Kind() ReadResultKind { return r.kind }

// Bytes returns a copy of complete line bytes
func (r ReadResult) Bytes() []byte { return append([]byte(nil), r.line...) }

// IO is the injected terminal and stream boundary used by Prompter
//
// ReadLine removes one trailing LF and an immediately preceding CR. It must
// not retain ctx after returning. Prompter always passes a positive maxBytes
type IO interface {
	io.Writer
	Flush() error
	IsTerminal() bool
	ReadLine(ctx context.Context, mode InputMode, maxBytes int) (ReadResult, error)
}

// TerminalPolicy controls whether visible prompts may use non-terminal I/O
type TerminalPolicy uint8

const (
	// RequireTerminal requires terminal input and output before prompt output
	RequireTerminal TerminalPolicy = iota
	// AllowNonTerminal permits Confirm, Select, and Input on injected streams
	AllowNonTerminal
)

// Limits bounds response bytes and Select choices
//
// The zero value uses DefaultMaxInputBytes and DefaultMaxChoices
type Limits struct {
	maxInputBytes int
	maxChoices    int
}

// MaxInputBytes returns the normalized response byte limit
func (l Limits) MaxInputBytes() int { return l.normalized().maxInputBytes }

// MaxChoices returns the normalized Select choice limit
func (l Limits) MaxChoices() int { return l.normalized().maxChoices }

// WithMaxInputBytes replaces the response byte limit
//
// A negative value is rejected when a request is executed. Zero selects the
// default because Limits has a useful zero value
func (l Limits) WithMaxInputBytes(value int) Limits {
	l.maxInputBytes = value
	return l
}

// WithMaxChoices replaces the Select choice limit
//
// A negative value is rejected when a request is executed. Zero selects the
// default because Limits has a useful zero value
func (l Limits) WithMaxChoices(value int) Limits {
	l.maxChoices = value
	return l
}

func (l Limits) normalized() Limits {
	if l.maxInputBytes == 0 {
		l.maxInputBytes = DefaultMaxInputBytes
	}
	if l.maxChoices == 0 {
		l.maxChoices = DefaultMaxChoices
	}
	return l
}

// Confirm is a yes-or-no prompt request
type Confirm struct {
	message      string
	defaultValue bool
	hasDefault   bool
}

// NewConfirm constructs a Confirm request without a default answer
func NewConfirm(message string) Confirm { return Confirm{message: message} }

// WithDefault sets the answer selected by an empty response
func (c Confirm) WithDefault(value bool) Confirm {
	c.defaultValue = value
	c.hasDefault = true
	return c
}

// Message returns the prompt message
func (c Confirm) Message() string { return c.message }

// DefaultAnswer returns the explicit default answer
func (c Confirm) DefaultAnswer() (bool, bool) { return c.defaultValue, c.hasDefault }

// Select is a numbered choice prompt request
type Select struct {
	message      string
	choices      []string
	defaultIndex int
	hasDefault   bool
}

// NewSelect constructs a Select request without a default choice
func NewSelect(message string, choices ...string) Select {
	return Select{message: message, choices: append([]string(nil), choices...)}
}

// WithDefault sets the zero-based choice selected by an empty response
func (s Select) WithDefault(index int) Select {
	s.defaultIndex = index
	s.hasDefault = true
	return s
}

// Message returns the prompt message
func (s Select) Message() string { return s.message }

// Choices returns a copy of the ordered choice labels
func (s Select) Choices() []string { return append([]string(nil), s.choices...) }

// DefaultChoice returns the explicit zero-based default choice
func (s Select) DefaultChoice() (int, bool) { return s.defaultIndex, s.hasDefault }

// Input is a visible text input request
type Input struct {
	message      string
	defaultValue string
	hasDefault   bool
	required     bool
}

// NewInput constructs an Input request that permits an empty value
func NewInput(message string) Input { return Input{message: message} }

// WithDefault sets the value selected by an empty response
func (i Input) WithDefault(value string) Input {
	i.defaultValue = value
	i.hasDefault = true
	return i
}

// Required requires a non-empty response when no default is configured
func (i Input) Required() Input {
	i.required = true
	return i
}

// Message returns the prompt message
func (i Input) Message() string { return i.message }

// DefaultValue returns the explicit default value
func (i Input) DefaultValue() (string, bool) { return i.defaultValue, i.hasDefault }

// IsRequired reports whether an empty response must be retried
func (i Input) IsRequired() bool { return i.required }

// Secret is a text input request whose process read hides terminal echo
type Secret struct {
	message  string
	required bool
}

// NewSecret constructs a Secret request that permits an empty value
func NewSecret(message string) Secret { return Secret{message: message} }

// Required requires a non-empty response
func (s Secret) Required() Secret {
	s.required = true
	return s
}

// Message returns the prompt message
func (s Secret) Message() string { return s.message }

// IsRequired reports whether an empty response must be retried
func (s Secret) IsRequired() bool { return s.required }

// ErrorKind classifies a prompt failure independently of its display text
type ErrorKind uint8

const (
	// ErrorCanceled means the caller, terminal, or end of input canceled the request
	ErrorCanceled ErrorKind = iota
	// ErrorNotTerminal means required input or output is not a terminal
	ErrorNotTerminal
	// ErrorInvalidRequest means request metadata or limits violate the contract
	ErrorInvalidRequest
	// ErrorInputTooLong means a response exceeded its configured byte limit
	ErrorInputTooLong
	// ErrorIO means an injected or process I/O operation failed
	ErrorIO
)

// Error is a structured prompt failure
type Error struct {
	kind    ErrorKind
	message string
	cause   error
}

// Kind returns the stable failure category
func (e *Error) Kind() ErrorKind { return e.kind }

// Error implements error
func (e *Error) Error() string { return "prompt failed: " + e.message }

// Unwrap exposes an I/O or cancellation cause
func (e *Error) Unwrap() error { return e.cause }

// Prompter executes prompt requests through one injected I/O implementation
//
// A Prompter executes one request at a time and is not safe for concurrent use
type Prompter struct {
	io             IO
	limits         Limits
	terminalPolicy TerminalPolicy
}

// New constructs a Prompter with default resource and terminal policies
func New(promptIO IO) *Prompter { return &Prompter{io: promptIO} }

// NewProcess constructs a Prompter using standard input and standard error
func NewProcess() *Prompter { return New(NewProcessIO(nil, nil)) }

// WithLimits replaces the resource limits
func (p *Prompter) WithLimits(limits Limits) *Prompter {
	p.limits = limits
	return p
}

// WithTerminalPolicy replaces the visible-prompt terminal policy
func (p *Prompter) WithTerminalPolicy(policy TerminalPolicy) *Prompter {
	p.terminalPolicy = policy
	return p
}

// Confirm prompts until a valid yes-or-no answer is read
func (p *Prompter) Confirm(ctx context.Context, request Confirm) (bool, error) {
	ctx = nonNilContext(ctx)
	if err := p.validateCommon(request.message); err != nil {
		return false, err
	}
	if err := p.prepare(ctx, false); err != nil {
		return false, err
	}
	suffix := " [y/n] "
	if request.hasDefault && request.defaultValue {
		suffix = " [Y/n] "
	} else if request.hasDefault {
		suffix = " [y/N] "
	}
	for {
		if err := p.writePrompt(ctx, request.message+suffix); err != nil {
			return false, err
		}
		line, err := p.readValue(ctx, InputVisible)
		if err != nil {
			return false, err
		}
		answer := strings.Trim(line, " \t")
		if answer == "" && request.hasDefault {
			return request.defaultValue, nil
		}
		switch {
		case asciiEqualFold(answer, "y"), asciiEqualFold(answer, "yes"):
			return true, nil
		case asciiEqualFold(answer, "n"), asciiEqualFold(answer, "no"):
			return false, nil
		}
		if err := p.writePrompt(ctx, "Enter yes or no\n"); err != nil {
			return false, err
		}
	}
}

// Select prompts until a valid zero-based choice index is read
func (p *Prompter) Select(ctx context.Context, request Select) (int, error) {
	ctx = nonNilContext(ctx)
	if err := p.validateCommon(request.message); err != nil {
		return 0, err
	}
	limits, err := p.normalizedLimits()
	if err != nil {
		return 0, err
	}
	if len(request.choices) == 0 {
		return 0, promptError(ErrorInvalidRequest, "Select has no choices", nil)
	}
	if len(request.choices) > limits.maxChoices {
		return 0, promptError(ErrorInvalidRequest, "Select exceeds the configured choice limit", nil)
	}
	for _, choice := range request.choices {
		if err := validateMetadata(choice, "Select choice"); err != nil {
			return 0, err
		}
	}
	if request.hasDefault && (request.defaultIndex < 0 || request.defaultIndex >= len(request.choices)) {
		return 0, promptError(ErrorInvalidRequest, "Select default is outside the choice list", nil)
	}
	if err := p.prepare(ctx, false); err != nil {
		return 0, err
	}
	var menu strings.Builder
	fmt.Fprintf(&menu, "%s\n", request.message)
	for index, choice := range request.choices {
		fmt.Fprintf(&menu, "  %d) %s\n", index+1, choice)
	}
	if err := p.writePrompt(ctx, menu.String()); err != nil {
		return 0, err
	}
	for {
		linePrompt := fmt.Sprintf("Select [1-%d]: ", len(request.choices))
		if request.hasDefault {
			linePrompt = fmt.Sprintf("Select [1-%d, default %d]: ", len(request.choices), request.defaultIndex+1)
		}
		if err := p.writePrompt(ctx, linePrompt); err != nil {
			return 0, err
		}
		line, err := p.readValue(ctx, InputVisible)
		if err != nil {
			return 0, err
		}
		answer := strings.Trim(line, " \t")
		if answer == "" && request.hasDefault {
			return request.defaultIndex, nil
		}
		value, parseErr := strconv.Atoi(answer)
		if asciiDigits(answer) && parseErr == nil && value >= 1 && value <= len(request.choices) {
			return value - 1, nil
		}
		if err := p.writePrompt(ctx, fmt.Sprintf("Enter a number from 1 to %d\n", len(request.choices))); err != nil {
			return 0, err
		}
	}
}

// Input reads one visible text value
func (p *Prompter) Input(ctx context.Context, request Input) (string, error) {
	ctx = nonNilContext(ctx)
	if err := p.validateCommon(request.message); err != nil {
		return "", err
	}
	if request.hasDefault {
		if err := validateMetadata(request.defaultValue, "Input default"); err != nil {
			return "", err
		}
	}
	if err := p.prepare(ctx, false); err != nil {
		return "", err
	}
	for {
		linePrompt := request.message + ": "
		if request.hasDefault {
			linePrompt = fmt.Sprintf("%s [%s]: ", request.message, request.defaultValue)
		}
		if err := p.writePrompt(ctx, linePrompt); err != nil {
			return "", err
		}
		value, err := p.readValue(ctx, InputVisible)
		if err != nil {
			return "", err
		}
		if value == "" && request.hasDefault {
			return request.defaultValue, nil
		}
		if value == "" && request.required {
			if err := p.writePrompt(ctx, "A value is required\n"); err != nil {
				return "", err
			}
			continue
		}
		return value, nil
	}
}

// Secret reads one text value while process terminal echo is disabled
func (p *Prompter) Secret(ctx context.Context, request Secret) (string, error) {
	ctx = nonNilContext(ctx)
	if err := p.validateCommon(request.message); err != nil {
		return "", err
	}
	if err := p.prepare(ctx, true); err != nil {
		return "", err
	}
	for {
		if err := p.writePrompt(ctx, request.message+": "); err != nil {
			return "", err
		}
		value, err := p.readValue(ctx, InputSecret)
		if err != nil {
			return "", err
		}
		if value == "" && request.required {
			if err := p.writePrompt(ctx, "A value is required\n"); err != nil {
				return "", err
			}
			continue
		}
		return value, nil
	}
}

func (p *Prompter) validateCommon(message string) error {
	if p == nil || p.io == nil {
		return promptError(ErrorInvalidRequest, "nil prompt I/O", nil)
	}
	if p.terminalPolicy != RequireTerminal && p.terminalPolicy != AllowNonTerminal {
		return promptError(ErrorInvalidRequest, "unknown terminal policy", nil)
	}
	if _, err := p.normalizedLimits(); err != nil {
		return err
	}
	return validateMetadata(message, "prompt message")
}

func (p *Prompter) normalizedLimits() (Limits, error) {
	limits := p.limits.normalized()
	if limits.maxInputBytes < 0 || limits.maxChoices < 0 {
		return Limits{}, promptError(ErrorInvalidRequest, "prompt limits must be non-negative", nil)
	}
	return limits, nil
}

func (p *Prompter) prepare(ctx context.Context, secret bool) error {
	if err := ctx.Err(); err != nil {
		return canceled(ctx)
	}
	if !p.io.IsTerminal() && (secret || p.terminalPolicy == RequireTerminal) {
		return promptError(ErrorNotTerminal, "input and output are not interactive terminals", nil)
	}
	return nil
}

func (p *Prompter) writePrompt(ctx context.Context, value string) error {
	if err := ctx.Err(); err != nil {
		return canceled(ctx)
	}
	if err := writeAll(p.io, []byte(value)); err != nil {
		return promptError(ErrorIO, "write: "+err.Error(), err)
	}
	if err := p.io.Flush(); err != nil {
		return promptError(ErrorIO, "flush: "+err.Error(), err)
	}
	if err := ctx.Err(); err != nil {
		return canceled(ctx)
	}
	return nil
}

func (p *Prompter) readValue(ctx context.Context, mode InputMode) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", canceled(ctx)
	}
	limits, err := p.normalizedLimits()
	if err != nil {
		return "", err
	}
	result, readErr := p.io.ReadLine(ctx, mode, limits.maxInputBytes)
	if mode == InputSecret {
		if newlineErr := writeAll(p.io, []byte{'\n'}); newlineErr != nil {
			return "", promptError(ErrorIO, "write secret line ending: "+newlineErr.Error(), newlineErr)
		}
		if flushErr := p.io.Flush(); flushErr != nil {
			return "", promptError(ErrorIO, "flush secret line ending: "+flushErr.Error(), flushErr)
		}
	}
	if readErr != nil {
		if errors.Is(readErr, context.Canceled) || errors.Is(readErr, context.DeadlineExceeded) {
			return "", canceled(ctx)
		}
		return "", promptError(ErrorIO, "read: "+readErr.Error(), readErr)
	}
	if ctx.Err() != nil || result.kind == ReadCanceled {
		return "", canceled(ctx)
	}
	switch result.kind {
	case ReadLine:
		if len(result.line) > limits.maxInputBytes {
			return "", tooLong(limits.maxInputBytes)
		}
		if len(result.line) == 1 && (result.line[0] == 0x03 || result.line[0] == 0x1b) {
			return "", canceled(ctx)
		}
		return nagitext.NormalizeUTF8(string(result.line)), nil
	case ReadEndOfFile, ReadCanceled:
		return "", canceled(ctx)
	case ReadInputTooLong:
		return "", tooLong(limits.maxInputBytes)
	default:
		return "", promptError(ErrorIO, "read returned an unknown result", nil)
	}
}

func validateMetadata(value, field string) error {
	if value == "" {
		return promptError(ErrorInvalidRequest, field+" is empty", nil)
	}
	if !utf8.ValidString(value) {
		return promptError(ErrorInvalidRequest, field+" is not valid UTF-8", nil)
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return promptError(ErrorInvalidRequest, field+" contains a control character", nil)
		}
	}
	return nil
}

func asciiEqualFold(value, expected string) bool {
	if len(value) != len(expected) {
		return false
	}
	for index := range len(value) {
		character := value[index]
		if character >= 'A' && character <= 'Z' {
			character += 'a' - 'A'
		}
		if character != expected[index] {
			return false
		}
	}
	return true
}

func asciiDigits(value string) bool {
	if value == "" {
		return false
	}
	for index := range len(value) {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

func writeAll(writer io.Writer, value []byte) error {
	for len(value) > 0 {
		written, err := writer.Write(value)
		if err != nil {
			return err
		}
		if written <= 0 || written > len(value) {
			return io.ErrShortWrite
		}
		value = value[written:]
	}
	return nil
}

func nonNilContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func promptError(kind ErrorKind, message string, cause error) error {
	return &Error{kind: kind, message: message, cause: cause}
}

func canceled(ctx context.Context) error {
	var cause error
	if ctx != nil {
		cause = context.Cause(ctx)
	}
	return promptError(ErrorCanceled, "canceled", cause)
}

func tooLong(limit int) error {
	return promptError(ErrorInputTooLong, fmt.Sprintf("input exceeds %d bytes", limit), nil)
}
