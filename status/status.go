// Package status provides synchronous TTY-aware status reporting above Nagi
// CLI Core
package status

import (
	"fmt"
	"io"
	"math/bits"
	"strconv"
	"unicode"
	"unicode/utf8"

	nagitext "github.com/mayahiro/nagi-go/text"
)

const (
	// DefaultMaxMessageBytes is the default UTF-8 byte limit for one message
	DefaultMaxMessageBytes = 65_536
	// DefaultFallbackTerminalWidth is used when a terminal does not report its columns
	DefaultFallbackTerminalWidth = 80
	// DefaultProgressWidth is the default number of cells in a progress bar
	DefaultProgressWidth = 20
	// MaxProgressWidth is the maximum configurable progress-bar width
	MaxProgressWidth = 1_024
)

var spinnerFrames = [...]string{"-", "\\", "|", "/"}

var clearLine = []byte("\r\x1b[2K")

// SpinnerFrameCount is the number of stable ASCII spinner frames
const SpinnerFrameCount = len(spinnerFrames)

// SnapshotKind identifies the semantic kind of one Snapshot
type SnapshotKind uint8

const (
	// SnapshotStatus is a plain status message
	SnapshotStatus SnapshotKind = iota
	// SnapshotSpinner is an indeterminate application-driven spinner
	SnapshotSpinner
	// SnapshotProgress is determinate current and total progress
	SnapshotProgress
)

// Snapshot is one immutable status-line value supplied by an application
type Snapshot struct {
	kind    SnapshotKind
	message string
	tick    uint64
	current uint64
	total   uint64
}

// NewStatus constructs a plain status message
func NewStatus(message string) Snapshot {
	return Snapshot{kind: SnapshotStatus, message: message}
}

// NewSpinner constructs an application-driven spinner snapshot
func NewSpinner(tick uint64, message string) Snapshot {
	return Snapshot{kind: SnapshotSpinner, message: message, tick: tick}
}

// NewProgress constructs a determinate progress snapshot
func NewProgress(current, total uint64, message string) Snapshot {
	return Snapshot{kind: SnapshotProgress, message: message, current: current, total: total}
}

// Kind returns the semantic snapshot kind
func (s Snapshot) Kind() SnapshotKind { return s.kind }

// Message returns the human-readable message
func (s Snapshot) Message() string { return s.message }

// Tick returns the spinner tick and whether this is a Spinner snapshot
func (s Snapshot) Tick() (uint64, bool) { return s.tick, s.kind == SnapshotSpinner }

// Progress returns current and total units and whether this is a Progress snapshot
func (s Snapshot) Progress() (uint64, uint64, bool) {
	return s.current, s.total, s.kind == SnapshotProgress
}

// IO is the injected terminal and stream boundary used by Reporter
//
// TerminalWidth returns current positive columns and true, or false when the
// width is unknown. Terminal availability should remain stable for one
// Reporter lifetime
type IO interface {
	io.Writer
	Flush() error
	IsTerminal() bool
	TerminalWidth() (int, bool)
}

// Options contains immutable rendering and resource options
//
// The zero value uses portable defaults
type Options struct {
	maxMessageBytes       int
	fallbackTerminalWidth int
	progressWidth         int
	widthProfile          nagitext.WidthProfile
	widthProfileExplicit  bool
}

// MaxMessageBytes returns the normalized maximum UTF-8 message bytes
func (o Options) MaxMessageBytes() int { return o.normalized().maxMessageBytes }

// FallbackTerminalWidth returns normalized fallback terminal columns
func (o Options) FallbackTerminalWidth() int {
	return o.normalized().fallbackTerminalWidth
}

// ProgressWidth returns the normalized progress-bar width
func (o Options) ProgressWidth() int { return o.normalized().progressWidth }

// WidthProfile returns the terminal cell-width policy
func (o Options) WidthProfile() nagitext.WidthProfile { return o.widthProfile }

// WithMaxMessageBytes replaces the message byte limit
//
// A negative value is rejected when a Reporter is constructed. Zero selects
// the default because Options has a useful zero value
func (o Options) WithMaxMessageBytes(value int) Options {
	o.maxMessageBytes = value
	return o
}

// WithFallbackTerminalWidth replaces fallback terminal columns
//
// A negative value is rejected when a Reporter is constructed. Zero selects
// the default because Options has a useful zero value
func (o Options) WithFallbackTerminalWidth(value int) Options {
	o.fallbackTerminalWidth = value
	return o
}

// WithProgressWidth replaces the progress-bar width
//
// A negative value or a value above MaxProgressWidth is rejected when a
// Reporter is constructed. Zero selects the default
func (o Options) WithProgressWidth(value int) Options {
	o.progressWidth = value
	return o
}

// WithWidthProfile replaces the terminal cell-width policy
func (o Options) WithWidthProfile(value nagitext.WidthProfile) Options {
	o.widthProfile = value
	o.widthProfileExplicit = true
	return o
}

func (o Options) normalized() Options {
	if o.maxMessageBytes == 0 {
		o.maxMessageBytes = DefaultMaxMessageBytes
	}
	if o.fallbackTerminalWidth == 0 {
		o.fallbackTerminalWidth = DefaultFallbackTerminalWidth
	}
	if o.progressWidth == 0 {
		o.progressWidth = DefaultProgressWidth
	}
	return o
}

// ErrorKind classifies a status-reporting failure independently of display text
type ErrorKind uint8

const (
	// ErrorInvalidStatus means options or message metadata violate the contract
	ErrorInvalidStatus ErrorKind = iota
	// ErrorIO means injected or process output failed
	ErrorIO
)

// Error is a structured status-reporting failure
type Error struct {
	kind    ErrorKind
	message string
	cause   error
}

// Kind returns the stable failure kind
func (e *Error) Kind() ErrorKind { return e.kind }

// Error implements error
func (e *Error) Error() string { return "status reporting failed: " + e.message }

// Unwrap returns an injected I/O cause when present
func (e *Error) Unwrap() error { return e.cause }

func statusError(kind ErrorKind, message string, cause error) error {
	return &Error{kind: kind, message: message, cause: cause}
}

// Reporter is a synchronous TTY-aware reporter with bounded retained buffers
//
// Reporter creates no goroutine or timer. The application owns spinner ticks,
// progress updates, and serialization with any other writer using the same
// output. Reporter is not safe for concurrent use
type Reporter struct {
	output       IO
	options      Options
	active       bool
	lastTerminal []byte
	lastLog      []byte
	scratch      []byte
}

// New constructs a Reporter with portable default options
func New(output IO) *Reporter {
	reporter, _ := NewWithOptions(output, Options{})
	return reporter
}

// NewWithOptions constructs a Reporter after validating explicit options
func NewWithOptions(output IO, options Options) (*Reporter, error) {
	if output == nil {
		return nil, statusError(ErrorInvalidStatus, "reporter has no output", nil)
	}
	options = options.normalized()
	if options.maxMessageBytes < 0 {
		return nil, statusError(ErrorInvalidStatus, "maximum message bytes must not be negative", nil)
	}
	if options.fallbackTerminalWidth < 0 {
		return nil, statusError(ErrorInvalidStatus, "fallback terminal width must not be negative", nil)
	}
	if options.progressWidth < 0 || options.progressWidth > MaxProgressWidth {
		return nil, statusError(
			ErrorInvalidStatus,
			fmt.Sprintf("progress width must be between 1 and %d", MaxProgressWidth),
			nil,
		)
	}
	return &Reporter{output: output, options: options}, nil
}

// Options returns immutable normalized rendering options
func (r *Reporter) Options() Options {
	if r == nil {
		return Options{}.normalized()
	}
	return r.options
}

// IsActive reports whether a transient terminal line is currently active
func (r *Reporter) IsActive() bool { return r != nil && r.active }

// IO returns injected I/O
func (r *Reporter) IO() IO {
	if r == nil {
		return nil
	}
	return r.output
}

// Update changes the transient line or emits a plain non-terminal log record
//
// The result is true only when bytes were written. Repeated identical
// terminal lines and repeated identical non-terminal records are coalesced
func (r *Reporter) Update(snapshot Snapshot) (bool, error) {
	if err := r.validate(snapshot); err != nil {
		return false, err
	}
	if r.output.IsTerminal() {
		return r.updateTerminal(snapshot)
	}
	return r.updateLog(snapshot)
}

// Finish commits a final terminal line or final plain fallback record
//
// Finishing resets coalescing state so a later identical Update starts a new
// reporting lifecycle
func (r *Reporter) Finish(snapshot Snapshot) (bool, error) {
	if err := r.validate(snapshot); err != nil {
		return false, err
	}
	if r.output.IsTerminal() {
		return r.finishTerminal(snapshot)
	}
	emitted, err := r.updateLog(snapshot)
	if err != nil {
		return false, err
	}
	r.lastLog = r.lastLog[:0]
	r.active = false
	return emitted, nil
}

// Clear removes an active terminal line and resets coalescing state
//
// Non-terminal output is never erased. The result reports whether bytes were
// written
func (r *Reporter) Clear() (bool, error) {
	if err := r.validReporter(); err != nil {
		return false, err
	}
	emitted := false
	if r.active && r.output.IsTerminal() {
		if err := writeAndFlush(r.output, clearLine); err != nil {
			return false, err
		}
		emitted = true
	}
	r.active = false
	r.lastTerminal = r.lastTerminal[:0]
	r.lastLog = r.lastLog[:0]
	return emitted, nil
}

// Log writes one permanent plain line without losing an active status
//
// On a terminal the transient line is erased, the log is written, and the
// previous status line is repainted in one flushed operation
func (r *Reporter) Log(message string) error {
	if err := r.validateMessage(message); err != nil {
		return err
	}
	if r.active && r.output.IsTerminal() {
		return writeAndFlush(
			r.output,
			clearLine,
			[]byte(message),
			[]byte{'\n'},
			clearLine,
			r.lastTerminal,
		)
	}
	return writeAndFlush(r.output, []byte(message), []byte{'\n'})
}

func (r *Reporter) updateTerminal(snapshot Snapshot) (bool, error) {
	r.renderTerminal(snapshot)
	if r.active && stringEqual(r.lastTerminal, r.scratch) {
		return false, nil
	}
	if err := writeAndFlush(r.output, clearLine, r.scratch); err != nil {
		return false, err
	}
	r.lastTerminal, r.scratch = r.scratch, r.lastTerminal[:0]
	r.active = true
	return true, nil
}

func (r *Reporter) finishTerminal(snapshot Snapshot) (bool, error) {
	r.renderTerminal(snapshot)
	var err error
	if r.active && stringEqual(r.lastTerminal, r.scratch) {
		err = writeAndFlush(r.output, []byte{'\n'})
	} else {
		err = writeAndFlush(r.output, clearLine, r.scratch, []byte{'\n'})
	}
	if err != nil {
		return false, err
	}
	r.active = false
	r.lastTerminal = r.lastTerminal[:0]
	r.lastLog = r.lastLog[:0]
	return true, nil
}

func (r *Reporter) updateLog(snapshot Snapshot) (bool, error) {
	r.renderFallback(snapshot)
	if len(r.scratch) == 0 || stringEqual(r.lastLog, r.scratch) {
		return false, nil
	}
	if err := writeAndFlush(r.output, r.scratch, []byte{'\n'}); err != nil {
		return false, err
	}
	r.lastLog, r.scratch = r.scratch, r.lastLog[:0]
	r.active = false
	return true, nil
}

func (r *Reporter) renderTerminal(snapshot Snapshot) {
	r.scratch = r.scratch[:0]
	width, ok := r.output.TerminalWidth()
	if !ok || width <= 0 {
		width = r.options.fallbackTerminalWidth
	}
	usable := max(width-1, 0)
	if r.options.widthProfileExplicit {
		r.renderTerminalProfiled(snapshot, usable)
		return
	}
	switch snapshot.kind {
	case SnapshotStatus:
		r.appendMessage(snapshot.message, usable)
	case SnapshotSpinner:
		r.appendASCII(spinnerFrames[snapshot.tick%uint64(len(spinnerFrames))], usable)
		r.appendSeparatedMessage(snapshot.message, usable)
	case SnapshotProgress:
		current := normalizedCurrent(snapshot.current, snapshot.total)
		complete := completedCells(current, snapshot.total, r.options.progressWidth)
		r.appendASCII("[", usable)
		r.appendRepeated('#', complete, usable)
		r.appendRepeated('-', r.options.progressWidth-complete, usable)
		r.appendASCII("] ", usable)
		r.appendUint(current, usable)
		r.appendASCII("/", usable)
		r.appendUint(snapshot.total, usable)
		r.appendSeparatedMessage(snapshot.message, usable)
	}
}

func (r *Reporter) renderTerminalProfiled(snapshot Snapshot, usable int) {
	switch snapshot.kind {
	case SnapshotStatus:
		r.scratch = append(r.scratch, snapshot.message...)
	case SnapshotSpinner:
		r.scratch = append(r.scratch, spinnerFrames[snapshot.tick%uint64(len(spinnerFrames))]...)
		if snapshot.message != "" {
			r.scratch = append(r.scratch, ' ')
			r.scratch = append(r.scratch, snapshot.message...)
		}
	case SnapshotProgress:
		current := normalizedCurrent(snapshot.current, snapshot.total)
		complete := completedCells(current, snapshot.total, r.options.progressWidth)
		r.scratch = append(r.scratch, '[')
		for range complete {
			r.scratch = append(r.scratch, '#')
		}
		for range r.options.progressWidth - complete {
			r.scratch = append(r.scratch, '-')
		}
		r.scratch = append(r.scratch, ']', ' ')
		r.scratch = strconv.AppendUint(r.scratch, current, 10)
		r.scratch = append(r.scratch, '/')
		r.scratch = strconv.AppendUint(r.scratch, snapshot.total, 10)
		if snapshot.message != "" {
			r.scratch = append(r.scratch, ' ')
			r.scratch = append(r.scratch, snapshot.message...)
		}
	}
	rendered := nagitext.Truncate(string(r.scratch), usable, r.options.widthProfile)
	r.scratch = r.scratch[:len(rendered)]
}

func (r *Reporter) appendMessage(message string, usable int) {
	remaining := usable - len(r.scratch)
	if remaining <= 0 {
		return
	}
	if isASCII(message) {
		if len(message) > remaining {
			message = message[:remaining]
		}
		r.scratch = append(r.scratch, message...)
		return
	}
	r.scratch = append(r.scratch, nagitext.Truncate(message, remaining, r.options.widthProfile)...)
}

func (r *Reporter) appendSeparatedMessage(message string, usable int) {
	if message == "" || len(r.scratch) >= usable {
		return
	}
	r.scratch = append(r.scratch, ' ')
	r.appendMessage(message, usable)
}

func (r *Reporter) appendASCII(value string, usable int) {
	remaining := usable - len(r.scratch)
	if remaining <= 0 {
		return
	}
	if len(value) > remaining {
		value = value[:remaining]
	}
	r.scratch = append(r.scratch, value...)
}

func (r *Reporter) appendRepeated(value byte, count, usable int) {
	for range min(count, max(usable-len(r.scratch), 0)) {
		r.scratch = append(r.scratch, value)
	}
}

func (r *Reporter) appendUint(value uint64, usable int) {
	start := len(r.scratch)
	r.scratch = strconv.AppendUint(r.scratch, value, 10)
	if len(r.scratch) > usable {
		r.scratch = r.scratch[:max(usable, start)]
	}
}

func (r *Reporter) renderFallback(snapshot Snapshot) {
	r.scratch = r.scratch[:0]
	switch snapshot.kind {
	case SnapshotStatus, SnapshotSpinner:
		r.scratch = append(r.scratch, snapshot.message...)
	case SnapshotProgress:
		current := normalizedCurrent(snapshot.current, snapshot.total)
		r.scratch = strconv.AppendUint(r.scratch, current, 10)
		r.scratch = append(r.scratch, '/')
		r.scratch = strconv.AppendUint(r.scratch, snapshot.total, 10)
		if snapshot.message != "" {
			r.scratch = append(r.scratch, ' ')
			r.scratch = append(r.scratch, snapshot.message...)
		}
	}
}

func (r *Reporter) validate(snapshot Snapshot) error {
	if err := r.validateMessage(snapshot.message); err != nil {
		return err
	}
	if snapshot.kind > SnapshotProgress {
		return statusError(ErrorInvalidStatus, "unknown snapshot kind", nil)
	}
	return nil
}

func (r *Reporter) validateMessage(message string) error {
	if err := r.validReporter(); err != nil {
		return err
	}
	if len(message) > r.options.maxMessageBytes {
		return statusError(ErrorInvalidStatus, "message exceeds its byte limit", nil)
	}
	if !utf8.ValidString(message) {
		return statusError(ErrorInvalidStatus, "message is not valid UTF-8", nil)
	}
	for _, value := range message {
		if unicode.IsControl(value) {
			return statusError(ErrorInvalidStatus, "message contains a control character", nil)
		}
	}
	return nil
}

func (r *Reporter) validReporter() error {
	if r == nil || r.output == nil {
		return statusError(ErrorInvalidStatus, "reporter has no output", nil)
	}
	return nil
}

func normalizedCurrent(current, total uint64) uint64 {
	if total == 0 {
		return 0
	}
	return min(current, total)
}

func completedCells(current, total uint64, width int) int {
	if total == 0 {
		return 0
	}
	high, low := bits.Mul64(current, uint64(width))
	quotient, _ := bits.Div64(high, low, total)
	return int(quotient)
}

func stringEqual(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func isASCII(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func writeAndFlush(output IO, parts ...[]byte) error {
	for _, part := range parts {
		for len(part) > 0 {
			count, err := output.Write(part)
			if err != nil {
				return statusError(ErrorIO, "could not write output", err)
			}
			if count <= 0 || count > len(part) {
				return statusError(ErrorIO, "could not write output", io.ErrShortWrite)
			}
			part = part[count:]
		}
	}
	if err := output.Flush(); err != nil {
		return statusError(ErrorIO, "could not flush output", err)
	}
	return nil
}

var _ error = (*Error)(nil)
var _ interface{ Unwrap() error } = (*Error)(nil)
