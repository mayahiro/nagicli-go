package cli

import (
	"io"
	"os"
	"path/filepath"
	"unicode/utf8"
)

const (
	defaultResponseFileMaxDepth       = 16
	defaultResponseFileMaxSources     = 64
	defaultResponseFileMaxSourceBytes = 8 * 1024 * 1024
	defaultResponseFileMaxTokens      = 65_536
	defaultResponseFileMaxTokenBytes  = 8 * 1024 * 1024
)

// ResponseFileLimits bounds one Response File expansion
//
// Its zero value selects the portable defaults
type ResponseFileLimits struct {
	maxDepth          int
	maxDepthSet       bool
	maxSources        int
	maxSourcesSet     bool
	maxSourceBytes    int
	maxSourceBytesSet bool
	maxTokens         int
	maxTokensSet      bool
	maxTokenBytes     int
	maxTokenBytesSet  bool
}

// WithMaxDepth returns a copy with the active include-depth limit replaced
func (l ResponseFileLimits) WithMaxDepth(value int) ResponseFileLimits {
	l.maxDepth, l.maxDepthSet = value, true
	return l
}

// WithMaxSources returns a copy with the source-read count limit replaced
func (l ResponseFileLimits) WithMaxSources(value int) ResponseFileLimits {
	l.maxSources, l.maxSourcesSet = value, true
	return l
}

// WithMaxSourceBytes returns a copy with the aggregate source-byte limit replaced
func (l ResponseFileLimits) WithMaxSourceBytes(value int) ResponseFileLimits {
	l.maxSourceBytes, l.maxSourceBytesSet = value, true
	return l
}

// WithMaxTokens returns a copy with the examined-token count limit replaced
func (l ResponseFileLimits) WithMaxTokens(value int) ResponseFileLimits {
	l.maxTokens, l.maxTokensSet = value, true
	return l
}

// WithMaxTokenBytes returns a copy with the aggregate token-byte limit replaced
func (l ResponseFileLimits) WithMaxTokenBytes(value int) ResponseFileLimits {
	l.maxTokenBytes, l.maxTokenBytesSet = value, true
	return l
}

// MaxDepth returns the active include-depth limit
func (l ResponseFileLimits) MaxDepth() int { return l.normalized().maxDepth }

// MaxSources returns the source-read count limit
func (l ResponseFileLimits) MaxSources() int { return l.normalized().maxSources }

// MaxSourceBytes returns the aggregate source-byte limit
func (l ResponseFileLimits) MaxSourceBytes() int { return l.normalized().maxSourceBytes }

// MaxTokens returns the examined-token count limit
func (l ResponseFileLimits) MaxTokens() int { return l.normalized().maxTokens }

// MaxTokenBytes returns the aggregate token-byte limit
func (l ResponseFileLimits) MaxTokenBytes() int { return l.normalized().maxTokenBytes }

func (l ResponseFileLimits) normalized() ResponseFileLimits {
	if !l.maxDepthSet {
		l.maxDepth = defaultResponseFileMaxDepth
	}
	if !l.maxSourcesSet {
		l.maxSources = defaultResponseFileMaxSources
	}
	if !l.maxSourceBytesSet {
		l.maxSourceBytes = defaultResponseFileMaxSourceBytes
	}
	if !l.maxTokensSet {
		l.maxTokens = defaultResponseFileMaxTokens
	}
	if !l.maxTokenBytesSet {
		l.maxTokenBytes = defaultResponseFileMaxTokenBytes
	}
	return l
}

func (l ResponseFileLimits) validate() *Diagnostic {
	l = l.normalized()
	if l.maxDepth < 0 || l.maxSources < 0 || l.maxSourceBytes < 0 || l.maxTokens < 0 || l.maxTokenBytes < 0 {
		return NewDiagnostic(
			CodeInvalidSpecification,
			"response file limits must be non-negative",
		)
	}
	return nil
}

// ResponseFileOptions selects opt-in behavior for one expansion
//
// Its zero value uses the default limits and disables exact @- expansion
type ResponseFileOptions struct {
	limits        ResponseFileLimits
	standardInput bool
}

// WithLimits returns a copy using the provided resource limits
func (o ResponseFileOptions) WithLimits(limits ResponseFileLimits) ResponseFileOptions {
	o.limits = limits
	return o
}

// WithStandardInput returns a copy that enables or disables exact @- expansion
func (o ResponseFileOptions) WithStandardInput(enabled bool) ResponseFileOptions {
	o.standardInput = enabled
	return o
}

// Limits returns the configured normalized resource limits
func (o ResponseFileOptions) Limits() ResponseFileLimits { return o.limits.normalized() }

// StandardInputEnabled reports whether exact @- expansion is enabled
func (o ResponseFileOptions) StandardInputEnabled() bool { return o.standardInput }

// ResponseFileReadRequest is one bounded request sent to an injected reader
type ResponseFileReadRequest struct {
	path      string
	readLimit int
}

// Path returns the lexically resolved platform-native file path
func (r ResponseFileReadRequest) Path() string { return r.path }

// ReadLimit returns the maximum bytes needed by the expander
//
// The value is one greater than the remaining accepted byte count when
// representable, allowing a reader to report a limit crossing without loading
// the rest of the source
func (r ResponseFileReadRequest) ReadLimit() int { return r.readLimit }

// ResponseFileReader reads Response File bytes from an injected or real filesystem
type ResponseFileReader interface {
	// ReadResponseFile reads at most the requested bytes or returns a Diagnostic
	ReadResponseFile(ResponseFileReadRequest) ([]byte, *Diagnostic)
}

// ResponseFileReaderFunc adapts a function into a ResponseFileReader
type ResponseFileReaderFunc func(ResponseFileReadRequest) ([]byte, *Diagnostic)

// ReadResponseFile calls the adapted function
func (f ResponseFileReaderFunc) ReadResponseFile(request ResponseFileReadRequest) ([]byte, *Diagnostic) {
	return f(request)
}

// FilesystemResponseFileReader is a stateless process-filesystem reader
type FilesystemResponseFileReader struct{}

// ReadResponseFile reads one bounded file from the process filesystem
func (FilesystemResponseFileReader) ReadResponseFile(request ResponseFileReadRequest) ([]byte, *Diagnostic) {
	file, err := os.Open(request.Path())
	if err != nil {
		return nil, responseFileIO()
	}
	bytes, readErr := readResponseFileBounded(file, request.ReadLimit())
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return nil, responseFileIO()
	}
	return bytes, nil
}

// ExpandResponseFiles expands opt-in @file arguments through injected file and
// standard-input readers
//
// Expansion is independent of a Command Graph. Returned strings retain their
// platform bytes and can be passed to Command.Parse or Command.Run
func ExpandResponseFiles(
	arguments []string,
	baseDirectory string,
	options ResponseFileOptions,
	reader ResponseFileReader,
	standardInput io.Reader,
) ([]string, error) {
	if diagnostic := options.limits.validate(); diagnostic != nil {
		return nil, diagnostic
	}
	expansion := responseFileExpansion{
		options:       options,
		reader:        reader,
		standardInput: standardInput,
		initialBase:   filepath.Clean(baseDirectory),
		activeFiles:   map[string]struct{}{},
	}
	return expansion.expand(arguments)
}

type responseFileExpansion struct {
	options               ResponseFileOptions
	reader                ResponseFileReader
	standardInput         io.Reader
	initialBase           string
	activeFiles           map[string]struct{}
	standardInputConsumed bool
	sourceCount           int
	sourceBytes           int
	tokenCount            int
	tokenBytes            int
}

type responseFileWork struct {
	endFile         bool
	path            string
	value           string
	base            string
	sourceDepth     int
	sourceReference string
	hasSource       bool
	budgetCounted   bool
}

func (e *responseFileExpansion) expand(arguments []string) ([]string, error) {
	work := make([]responseFileWork, 0, len(arguments))
	for index := len(arguments) - 1; index >= 0; index-- {
		work = append(work, responseFileWork{value: arguments[index], base: e.initialBase})
	}
	output := make([]string, 0, len(arguments))
	for len(work) > 0 {
		last := len(work) - 1
		item := work[last]
		work = work[:last]
		if item.endFile {
			delete(e.activeFiles, item.path)
			continue
		}
		if !item.budgetCounted {
			if diagnostic := e.recordToken(len(item.value), item.sourceReference, item.hasSource); diagnostic != nil {
				return nil, diagnostic
			}
		}
		switch {
		case len(item.value) >= 2 && item.value[:2] == "@@":
			output = append(output, item.value[1:])
		case item.value == "@" || len(item.value) == 0 || item.value[0] != '@':
			output = append(output, item.value)
		default:
			if diagnostic := e.include(item.value[1:], item.base, item.sourceDepth, &work); diagnostic != nil {
				return nil, diagnostic
			}
		}
	}
	return output, nil
}

func (e *responseFileExpansion) include(
	reference string,
	base string,
	sourceDepth int,
	work *[]responseFileWork,
) *Diagnostic {
	referenceDisplay := displayValue(reference)
	depth := sourceDepth + 1
	limits := e.options.limits.normalized()
	if depth > limits.maxDepth {
		return responseFileError(
			CodeResponseFileLimit,
			"response file include depth exceeds limit",
			referenceDisplay,
		)
	}

	if reference == "-" {
		if !e.options.standardInput {
			return responseFileError(
				CodeResponseFileStdin,
				"response file standard input is disabled",
				referenceDisplay,
			)
		}
		if e.standardInputConsumed {
			return responseFileError(
				CodeResponseFileStdin,
				"response file standard input was already consumed",
				referenceDisplay,
			)
		}
		e.standardInputConsumed = true
		if diagnostic := e.reserveSource(referenceDisplay); diagnostic != nil {
			return diagnostic
		}
		bytes, diagnostic := e.readStandardInput(referenceDisplay)
		if diagnostic != nil {
			return diagnostic
		}
		tokens, diagnostic := e.tokenizeSource(bytes, referenceDisplay)
		if diagnostic != nil {
			return diagnostic
		}
		e.pushTokens(tokens, e.initialBase, depth, referenceDisplay, work)
		return nil
	}

	path := resolveResponseFilePath(base, reference)
	if _, active := e.activeFiles[path]; active {
		return responseFileError(
			CodeResponseFileCycle,
			"response file include cycle detected",
			referenceDisplay,
		)
	}
	if diagnostic := e.reserveSource(referenceDisplay); diagnostic != nil {
		return diagnostic
	}
	bytes, diagnostic := e.readFile(path, referenceDisplay)
	if diagnostic != nil {
		return diagnostic
	}
	tokens, diagnostic := e.tokenizeSource(bytes, referenceDisplay)
	if diagnostic != nil {
		return diagnostic
	}
	e.activeFiles[path] = struct{}{}
	*work = append(*work, responseFileWork{endFile: true, path: path})
	e.pushTokens(tokens, filepath.Dir(path), depth, referenceDisplay, work)
	return nil
}

func (e *responseFileExpansion) reserveSource(reference string) *Diagnostic {
	if e.sourceCount >= e.options.limits.normalized().maxSources {
		return responseFileError(
			CodeResponseFileLimit,
			"response file source count exceeds limit",
			reference,
		)
	}
	e.sourceCount++
	return nil
}

func (e *responseFileExpansion) readFile(path, reference string) ([]byte, *Diagnostic) {
	if e.reader == nil {
		return nil, responseFileError(
			CodeInvalidSpecification,
			"response file reader is nil",
			reference,
		)
	}
	request := ResponseFileReadRequest{path: path, readLimit: e.nextReadLimit()}
	bytes, diagnostic := e.reader.ReadResponseFile(request)
	if diagnostic != nil {
		if len(diagnostic.targets) == 0 {
			diagnostic.WithTarget(ResponseFileTarget(reference))
		}
		return nil, diagnostic
	}
	return e.acceptSourceBytes(bytes, reference)
}

func (e *responseFileExpansion) readStandardInput(reference string) ([]byte, *Diagnostic) {
	if e.standardInput == nil {
		return nil, responseFileError(
			CodeResponseFileIO,
			"could not read response file",
			reference,
		)
	}
	bytes, err := readResponseFileBounded(e.standardInput, e.nextReadLimit())
	if err != nil {
		return nil, responseFileError(
			CodeResponseFileIO,
			"could not read response file",
			reference,
		)
	}
	return e.acceptSourceBytes(bytes, reference)
}

func (e *responseFileExpansion) nextReadLimit() int {
	remaining := e.options.limits.normalized().maxSourceBytes - e.sourceBytes
	maxInt := int(^uint(0) >> 1)
	if remaining < maxInt {
		return remaining + 1
	}
	return remaining
}

func (e *responseFileExpansion) acceptSourceBytes(bytes []byte, reference string) ([]byte, *Diagnostic) {
	remaining := e.options.limits.normalized().maxSourceBytes - e.sourceBytes
	if len(bytes) > remaining {
		return nil, responseFileError(
			CodeResponseFileLimit,
			"response file source bytes exceed limit",
			reference,
		)
	}
	e.sourceBytes += len(bytes)
	return bytes, nil
}

func (e *responseFileExpansion) tokenizeSource(bytes []byte, reference string) ([]string, *Diagnostic) {
	if len(bytes) >= 3 && bytes[0] == 0xef && bytes[1] == 0xbb && bytes[2] == 0xbf {
		bytes = bytes[3:]
	}
	if !utf8.Valid(bytes) {
		return nil, responseFileError(
			CodeResponseFileEncoding,
			"response file is not valid UTF-8",
			reference,
		)
	}
	return tokenizeResponseFile(
		string(bytes),
		reference,
		&e.tokenCount,
		&e.tokenBytes,
		e.options.limits.normalized(),
	)
}

func (e *responseFileExpansion) pushTokens(
	tokens []string,
	base string,
	sourceDepth int,
	sourceReference string,
	work *[]responseFileWork,
) {
	for index := len(tokens) - 1; index >= 0; index-- {
		*work = append(*work, responseFileWork{
			value:           tokens[index],
			base:            base,
			sourceDepth:     sourceDepth,
			sourceReference: sourceReference,
			hasSource:       true,
			budgetCounted:   true,
		})
	}
}

func (e *responseFileExpansion) recordToken(byteCount int, reference string, hasReference bool) *Diagnostic {
	return recordResponseFileTokenBudget(
		byteCount,
		reference,
		hasReference,
		&e.tokenCount,
		&e.tokenBytes,
		e.options.limits.normalized(),
	)
}

type responseFileQuote uint8

const (
	responseFileSingleQuote responseFileQuote = iota + 1
	responseFileDoubleQuote
)

func tokenizeResponseFile(
	text string,
	reference string,
	tokenCount *int,
	tokenBytes *int,
	limits ResponseFileLimits,
) ([]string, *Diagnostic) {
	cursor := responseFileTextCursor{text: text, line: 1, column: 1}
	var tokens []string
	var token []byte
	started := false
	var quote responseFileQuote
	quoteLine, quoteColumn := 0, 0

	for {
		character, line, column, present := cursor.next()
		if !present {
			break
		}
		if quote != 0 {
			switch {
			case quote == responseFileSingleQuote && character == '\'':
				quote = 0
			case quote == responseFileDoubleQuote && character == '"':
				quote = 0
			case quote == responseFileDoubleQuote && character == '\\':
				escaped, escapeLine, escapeColumn, ok := cursor.next()
				if !ok {
					return nil, responseFileSyntaxError(
						reference,
						"response file has a trailing escape",
						line,
						column,
					)
				}
				var diagnostic *Diagnostic
				token, diagnostic = appendResponseFileCharacter(
					token, escaped, reference, escapeLine, escapeColumn, *tokenBytes, limits,
				)
				if diagnostic != nil {
					return nil, diagnostic
				}
			default:
				var diagnostic *Diagnostic
				token, diagnostic = appendResponseFileCharacter(
					token, character, reference, line, column, *tokenBytes, limits,
				)
				if diagnostic != nil {
					return nil, diagnostic
				}
			}
			continue
		}

		if responseFileASCIISeparator(character) {
			if started {
				var diagnostic *Diagnostic
				tokens, diagnostic = appendResponseFileToken(tokens, token, reference, tokenCount, tokenBytes, limits)
				if diagnostic != nil {
					return nil, diagnostic
				}
				token = nil
				started = false
			}
			continue
		}
		if character == '#' && !started {
			for {
				comment, commentLine, commentColumn, ok := cursor.next()
				if ok && comment == 0 {
					return nil, responseFileSyntaxError(
						reference,
						"response file contains U+0000",
						commentLine,
						commentColumn,
					)
				}
				if !ok || comment == '\n' {
					break
				}
			}
			continue
		}
		switch character {
		case '\'':
			if diagnostic := ensureResponseFileTokenSlot(started, reference, *tokenCount, limits); diagnostic != nil {
				return nil, diagnostic
			}
			started = true
			quote = responseFileSingleQuote
			quoteLine, quoteColumn = line, column
		case '"':
			if diagnostic := ensureResponseFileTokenSlot(started, reference, *tokenCount, limits); diagnostic != nil {
				return nil, diagnostic
			}
			started = true
			quote = responseFileDoubleQuote
			quoteLine, quoteColumn = line, column
		case '\\':
			if diagnostic := ensureResponseFileTokenSlot(started, reference, *tokenCount, limits); diagnostic != nil {
				return nil, diagnostic
			}
			started = true
			escaped, escapeLine, escapeColumn, ok := cursor.next()
			if !ok {
				return nil, responseFileSyntaxError(
					reference,
					"response file has a trailing escape",
					line,
					column,
				)
			}
			var diagnostic *Diagnostic
			token, diagnostic = appendResponseFileCharacter(
				token, escaped, reference, escapeLine, escapeColumn, *tokenBytes, limits,
			)
			if diagnostic != nil {
				return nil, diagnostic
			}
		default:
			if diagnostic := ensureResponseFileTokenSlot(started, reference, *tokenCount, limits); diagnostic != nil {
				return nil, diagnostic
			}
			started = true
			var diagnostic *Diagnostic
			token, diagnostic = appendResponseFileCharacter(
				token, character, reference, line, column, *tokenBytes, limits,
			)
			if diagnostic != nil {
				return nil, diagnostic
			}
		}
	}

	if quote != 0 {
		message := "response file has an unterminated single quote"
		if quote == responseFileDoubleQuote {
			message = "response file has an unterminated double quote"
		}
		return nil, responseFileSyntaxError(reference, message, quoteLine, quoteColumn)
	}
	if started {
		var diagnostic *Diagnostic
		tokens, diagnostic = appendResponseFileToken(tokens, token, reference, tokenCount, tokenBytes, limits)
		if diagnostic != nil {
			return nil, diagnostic
		}
	}
	return tokens, nil
}

func appendResponseFileToken(
	tokens []string,
	token []byte,
	reference string,
	tokenCount *int,
	tokenBytes *int,
	limits ResponseFileLimits,
) ([]string, *Diagnostic) {
	if diagnostic := recordResponseFileTokenBudget(
		len(token),
		reference,
		true,
		tokenCount,
		tokenBytes,
		limits,
	); diagnostic != nil {
		return nil, diagnostic
	}
	return append(tokens, string(token)), nil
}

func recordResponseFileTokenBudget(
	byteCount int,
	reference string,
	hasReference bool,
	tokenCount *int,
	tokenBytes *int,
	limits ResponseFileLimits,
) *Diagnostic {
	if *tokenCount >= limits.maxTokens {
		return responseFileLimitError(
			"response file token count exceeds limit",
			reference,
			hasReference,
		)
	}
	if byteCount > limits.maxTokenBytes-*tokenBytes {
		return responseFileLimitError(
			"response file token bytes exceed limit",
			reference,
			hasReference,
		)
	}
	*tokenCount++
	*tokenBytes += byteCount
	return nil
}

func appendResponseFileCharacter(
	token []byte,
	character rune,
	reference string,
	line int,
	column int,
	completedTokenBytes int,
	limits ResponseFileLimits,
) ([]byte, *Diagnostic) {
	if character == 0 {
		return nil, responseFileSyntaxError(
			reference,
			"response file contains U+0000",
			line,
			column,
		)
	}
	encodedLength := utf8.RuneLen(character)
	remaining := limits.maxTokenBytes - completedTokenBytes - len(token)
	if encodedLength > remaining {
		return nil, responseFileError(
			CodeResponseFileLimit,
			"response file token bytes exceed limit",
			reference,
		)
	}
	return utf8.AppendRune(token, character), nil
}

func ensureResponseFileTokenSlot(
	started bool,
	reference string,
	tokenCount int,
	limits ResponseFileLimits,
) *Diagnostic {
	if !started && tokenCount >= limits.maxTokens {
		return responseFileError(
			CodeResponseFileLimit,
			"response file token count exceeds limit",
			reference,
		)
	}
	return nil
}

type responseFileTextCursor struct {
	text   string
	index  int
	line   int
	column int
}

func (c *responseFileTextCursor) next() (rune, int, int, bool) {
	if c.index >= len(c.text) {
		return 0, 0, 0, false
	}
	character, size := utf8.DecodeRuneInString(c.text[c.index:])
	line, column := c.line, c.column
	c.index += size
	if character == '\n' {
		c.line++
		c.column = 1
	} else {
		c.column++
	}
	return character, line, column, true
}

func responseFileASCIISeparator(character rune) bool {
	switch character {
	case ' ', '\t', '\r', '\n', '\v', '\f':
		return true
	default:
		return false
	}
}

func responseFileSyntaxError(reference, message string, line, column int) *Diagnostic {
	return responseFileError(
		CodeResponseFileSyntax,
		message+" at line "+itoa(line)+", column "+itoa(column),
		reference,
	)
}

func responseFileLimitError(message, reference string, hasReference bool) *Diagnostic {
	diagnostic := NewDiagnostic(CodeResponseFileLimit, message)
	if hasReference {
		diagnostic.WithTarget(ResponseFileTarget(reference))
	}
	return diagnostic
}

func responseFileError(code DiagnosticCode, message, reference string) *Diagnostic {
	return NewDiagnostic(code, message).WithTarget(ResponseFileTarget(reference))
}

func responseFileIO() *Diagnostic {
	return NewDiagnostic(CodeResponseFileIO, "could not read response file")
}

func readResponseFileBounded(reader io.Reader, limit int) ([]byte, error) {
	return io.ReadAll(io.LimitReader(reader, int64(limit)))
}

func resolveResponseFilePath(base, reference string) string {
	if filepath.IsAbs(reference) {
		return filepath.Clean(reference)
	}
	return filepath.Clean(filepath.Join(base, reference))
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte(value%10) + '0'
		value /= 10
	}
	return string(digits[index:])
}
