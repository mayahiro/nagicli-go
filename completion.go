package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// CompletionTargetKind identifies the syntax target being completed
type CompletionTargetKind uint8

const (
	// CompletionTargetCommand identifies child-command or command-level syntax
	CompletionTargetCommand CompletionTargetKind = iota
	// CompletionTargetOption identifies a named option value
	CompletionTargetOption
	// CompletionTargetArgument identifies a positional argument value
	CompletionTargetArgument
)

// CompletionTarget identifies one target by stable Command Graph identity
type CompletionTarget struct {
	kind          CompletionTargetKind
	commandIDPath []string
	valueID       string
}

func commandCompletionTarget(path []string) CompletionTarget {
	return CompletionTarget{kind: CompletionTargetCommand, commandIDPath: append([]string(nil), path...)}
}

func optionCompletionTarget(path []string, valueID string) CompletionTarget {
	return CompletionTarget{kind: CompletionTargetOption, commandIDPath: append([]string(nil), path...), valueID: valueID}
}

func argumentCompletionTarget(path []string, valueID string) CompletionTarget {
	return CompletionTarget{kind: CompletionTargetArgument, commandIDPath: append([]string(nil), path...), valueID: valueID}
}

// Kind returns the target category
func (t CompletionTarget) Kind() CompletionTargetKind { return t.kind }

// CommandIDPath returns the stable path of the command that owns this target
func (t CompletionTarget) CommandIDPath() []string {
	return append([]string(nil), t.commandIDPath...)
}

// ValueID returns the command-local value ID for an Option or Argument target
func (t CompletionTarget) ValueID() string { return t.valueID }

// CompletionOccurrenceKind identifies how one partial argv occurrence was represented
type CompletionOccurrenceKind uint8

const (
	// CompletionOccurrenceFlag identifies a Boolean Flag option occurrence
	CompletionOccurrenceFlag CompletionOccurrenceKind = iota
	// CompletionOccurrenceCount identifies a Count option occurrence
	CompletionOccurrenceCount
	// CompletionOccurrenceValue identifies a Value option or positional occurrence
	CompletionOccurrenceValue
)

// CompletionOccurrence is one recognized raw occurrence before the active token
type CompletionOccurrence struct {
	target CompletionTarget
	kind   CompletionOccurrenceKind
	raw    string
	rawSet bool
}

// Target returns the stable option or argument target
func (o CompletionOccurrence) Target() CompletionTarget { return cloneCompletionTarget(o.target) }

// Kind returns whether this occurrence is a Flag, Count, or raw Value
func (o CompletionOccurrence) Kind() CompletionOccurrenceKind { return o.kind }

// Raw returns the raw value for Value occurrences
func (o CompletionOccurrence) Raw() (string, bool) { return o.raw, o.rawSet }

// CompletionInput is tokenized shell input for one completion request
type CompletionInput struct {
	arguments []string
	current   string
}

// NewCompletionInput constructs input from completed arguments and the token prefix at the cursor
//
// Arguments exclude the program name and the current token
func NewCompletionInput(arguments []string, current string) CompletionInput {
	return CompletionInput{
		arguments: append([]string(nil), arguments...),
		current:   current,
	}
}

// Arguments returns completed arguments before the token at the cursor
func (i CompletionInput) Arguments() []string { return append([]string(nil), i.arguments...) }

// Current returns the token prefix at the cursor
func (i CompletionInput) Current() string { return i.current }

// CompletionRequest is one normalized request passed to a dynamic provider
type CompletionRequest struct {
	arguments   []string
	current     string
	commandPath []string
	commandIDs  []string
	target      CompletionTarget
	prefix      string
	partial     []CompletionOccurrence
}

// Arguments returns completed arguments exactly as supplied by the shell adapter
func (r CompletionRequest) Arguments() []string {
	return append([]string(nil), r.arguments...)
}

// Current returns the complete token prefix at the cursor
func (r CompletionRequest) Current() string { return r.current }

// CommandPath returns the selected canonical command path
func (r CompletionRequest) CommandPath() []string {
	return append([]string(nil), r.commandPath...)
}

// CommandIDPath returns the selected stable command-ID path
func (r CompletionRequest) CommandIDPath() []string {
	return append([]string(nil), r.commandIDs...)
}

// Target returns the active completion target
func (r CompletionRequest) Target() CompletionTarget { return cloneCompletionTarget(r.target) }

// Prefix returns the target-local prefix being completed
//
// Attached long and short option syntax is removed from this prefix
func (r CompletionRequest) Prefix() string { return r.prefix }

// PartialOccurrences returns recognized argv occurrences in original order
//
// Completion does not run Value Parsers, fallbacks, or validators
func (r CompletionRequest) PartialOccurrences() []CompletionOccurrence {
	result := make([]CompletionOccurrence, len(r.partial))
	for index := range r.partial {
		result[index] = cloneCompletionOccurrence(r.partial[index])
	}
	return result
}

// CompletionCandidateKind classifies a candidate for presentation adapters
type CompletionCandidateKind uint8

const (
	// CompletionCandidateCommand identifies a child command name or alias
	CompletionCandidateCommand CompletionCandidateKind = iota
	// CompletionCandidateOption identifies a long or short option spelling
	CompletionCandidateOption
	// CompletionCandidateValue identifies an option or positional value
	CompletionCandidateValue
)

// CompletionCandidate is one portable completion candidate
type CompletionCandidate struct {
	value        string
	displayLabel string
	description  string
	deprecation  Deprecation
	kind         CompletionCandidateKind
	appendSpace  bool
}

// NewCompletionCandidate constructs a Value candidate that appends a space when selected
func NewCompletionCandidate(value string) CompletionCandidate {
	return CompletionCandidate{
		value:       value,
		kind:        CompletionCandidateValue,
		appendSpace: true,
	}
}

// WithDisplayLabel sets text displayed separately from the inserted value
//
// An empty label restores the inserted value as the display label
func (c CompletionCandidate) WithDisplayLabel(label string) CompletionCandidate {
	c.displayLabel = label
	return c
}

// WithDescription sets a short human-readable candidate description
//
// An empty description is treated as absent
func (c CompletionCandidate) WithDescription(description string) CompletionCandidate {
	c.description = description
	return c
}

// WithKind sets the semantic candidate kind
func (c CompletionCandidate) WithKind(kind CompletionCandidateKind) CompletionCandidate {
	c.kind = kind
	return c
}

// WithAppendSpace controls whether adapters append a space after this candidate
func (c CompletionCandidate) WithAppendSpace(appendSpace bool) CompletionCandidate {
	c.appendSpace = appendSpace
	return c
}

// Value returns the complete text inserted for this candidate
func (c CompletionCandidate) Value() string { return c.value }

// DisplayLabel returns the display label, defaulting to the inserted value
func (c CompletionCandidate) DisplayLabel() string {
	if c.displayLabel != "" {
		return c.displayLabel
	}
	return c.value
}

// Description returns the optional short description
func (c CompletionCandidate) Description() string { return c.description }

// Deprecation returns replacement metadata for a deprecated static candidate
func (c CompletionCandidate) Deprecation() (Deprecation, bool) {
	return c.deprecation, c.deprecation.configured
}

// Kind returns the semantic candidate kind
func (c CompletionCandidate) Kind() CompletionCandidateKind { return c.kind }

// AppendSpace reports whether adapters should append a space after insertion
func (c CompletionCandidate) AppendSpace() bool { return c.appendSpace }

func (c CompletionCandidate) withValuePrefix(prefix string) CompletionCandidate {
	if prefix != "" {
		c.value = prefix + c.value
	}
	return c
}

func (c CompletionCandidate) withDeprecation(deprecation Deprecation) CompletionCandidate {
	c.deprecation = deprecation
	return c
}

// CompletionProvider supplies runtime candidates for one active Option or Argument target
type CompletionProvider func(context.Context, CompletionRequest) ([]CompletionCandidate, error)

// CompletionErrorKind classifies a completion resolution failure
type CompletionErrorKind uint8

const (
	// CompletionErrorCancelled means cancellation occurred before completion finished
	CompletionErrorCancelled CompletionErrorKind = iota
	// CompletionErrorProvider means the active dynamic provider returned an error
	CompletionErrorProvider
	// CompletionErrorInvalidCandidate means a candidate is unsafe for shell adapters
	CompletionErrorInvalidCandidate
)

// CompletionError is a completion-specific failure separate from parsing and handlers
type CompletionError struct {
	kind      CompletionErrorKind
	target    CompletionTarget
	targetSet bool
	message   string
	cause     error
}

// Kind returns the failure category
func (e *CompletionError) Kind() CompletionErrorKind { return e.kind }

// Target returns the active target when resolution reached one
func (e *CompletionError) Target() (CompletionTarget, bool) {
	return cloneCompletionTarget(e.target), e.targetSet
}

// Message returns the human-readable failure message
func (e *CompletionError) Message() string { return e.message }

// Error implements error
func (e *CompletionError) Error() string { return "completion failed: " + e.message }

// Unwrap exposes provider or context cancellation causes
func (e *CompletionError) Unwrap() error { return e.cause }

// CompletionResult contains the normalized request and deterministic candidates
type CompletionResult struct {
	request    CompletionRequest
	candidates []CompletionCandidate
}

// Request returns the normalized request used for static and dynamic candidates
func (r CompletionResult) Request() CompletionRequest { return cloneCompletionRequest(r.request) }

// Candidates returns candidates in deterministic source order
func (r CompletionResult) Candidates() []CompletionCandidate {
	return append([]CompletionCandidate(nil), r.candidates...)
}

type completionEngineOption struct {
	id             string
	long           string
	short          byte
	kind           OptionKind
	help           string
	hidden         bool
	deprecation    Deprecation
	inherited      bool
	repeated       bool
	possibleValues []string
	provider       CompletionProvider
}

type completionEngineArgument struct {
	id             string
	repeated       bool
	possibleValues []string
	provider       CompletionProvider
}

type completionEngineCommand struct {
	id          string
	name        string
	aliases     []string
	description string
	hidden      bool
	deprecation Deprecation
	options     []completionEngineOption
	arguments   []completionEngineArgument
	subcommands []*completionEngineCommand
	childLookup map[string]int
	hasVersion  bool
}

// CompletionEngine is an immutable handler-free projection of a validated Command Graph
type CompletionEngine struct {
	root *completionEngineCommand
}

// NewCompletionEngine validates and snapshots a Command Graph for repeated requests
func NewCompletionEngine(command *Command) (*CompletionEngine, error) {
	if command == nil {
		return nil, errors.New("nagi cli: nil Command")
	}
	if err := command.Validate(); err != nil {
		return nil, err
	}
	return &CompletionEngine{root: snapshotCompletionCommand(command, command.version != "")}, nil
}

// RootName returns the canonical program name used by shell generators
func (e *CompletionEngine) RootName() string {
	if e == nil || e.root == nil {
		return ""
	}
	return e.root.name
}

// Complete resolves static candidates and only the active target's dynamic provider
//
// It does not run Value Parsers, fallbacks, validators, or handlers
func (e *CompletionEngine) Complete(ctx context.Context, input CompletionInput) (CompletionResult, error) {
	if e == nil || e.root == nil {
		return CompletionResult{}, errors.New("nagi cli: nil CompletionEngine")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return CompletionResult{}, cancelledCompletionError(CompletionTarget{}, false, err)
	}

	resolution := newCompletionState(e.root, input.arguments).resolve(input.current)
	request := CompletionRequest{
		arguments:   input.arguments,
		current:     input.current,
		commandPath: resolution.commandPath,
		commandIDs:  resolution.commandIDs,
		target:      cloneCompletionTarget(resolution.target),
		prefix:      resolution.prefix,
		partial:     resolution.partial,
	}
	candidates := resolution.candidates
	if resolution.provider != nil {
		if err := ctx.Err(); err != nil {
			return CompletionResult{}, cancelledCompletionError(request.target, true, err)
		}
		provided, err := resolution.provider(ctx, cloneCompletionRequest(request))
		if contextError := ctx.Err(); contextError != nil {
			return CompletionResult{}, cancelledCompletionError(request.target, true, contextError)
		}
		if err != nil {
			return CompletionResult{}, &CompletionError{
				kind:      CompletionErrorProvider,
				target:    cloneCompletionTarget(request.target),
				targetSet: true,
				message:   err.Error(),
				cause:     err,
			}
		}
		for _, candidate := range provided {
			if reason := invalidCompletionCandidate(candidate); reason != "" {
				return CompletionResult{}, invalidCandidateCompletionError(request.target, reason)
			}
			candidates = append(candidates, candidate.withValuePrefix(resolution.valuePrefix))
		}
	}

	seen := make(map[string]struct{}, len(candidates))
	filtered := make([]CompletionCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if reason := invalidCompletionCandidate(candidate); reason != "" {
			return CompletionResult{}, invalidCandidateCompletionError(request.target, reason)
		}
		if !strings.HasPrefix(candidate.value, request.current) {
			continue
		}
		if _, duplicate := seen[candidate.value]; duplicate {
			continue
		}
		seen[candidate.value] = struct{}{}
		filtered = append(filtered, candidate)
	}
	return CompletionResult{request: request, candidates: filtered}, nil
}

func snapshotCompletionCommand(command *Command, hasVersion bool) *completionEngineCommand {
	snapshot := &completionEngineCommand{
		id:          command.id,
		name:        command.name,
		aliases:     append([]string(nil), command.aliases...),
		description: command.about,
		hidden:      command.hidden,
		deprecation: command.deprecation,
		hasVersion:  hasVersion,
		options:     make([]completionEngineOption, len(command.options)),
		arguments:   make([]completionEngineArgument, len(command.arguments)),
		subcommands: make([]*completionEngineCommand, len(command.subcommands)),
		childLookup: make(map[string]int),
	}
	for index := range command.options {
		option := &command.options[index]
		snapshot.options[index] = completionEngineOption{
			id:             option.id,
			long:           option.long,
			short:          option.short,
			kind:           option.kind,
			help:           option.help,
			hidden:         option.hidden,
			deprecation:    option.deprecation,
			inherited:      option.inherited,
			repeated:       option.repeated,
			possibleValues: append([]string(nil), option.parser.PossibleValues()...),
			provider:       option.completion,
		}
	}
	for index := range command.arguments {
		argument := &command.arguments[index]
		snapshot.arguments[index] = completionEngineArgument{
			id:             argument.id,
			repeated:       argument.repeated,
			possibleValues: append([]string(nil), argument.parser.PossibleValues()...),
			provider:       argument.completion,
		}
	}
	for index, child := range command.subcommands {
		snapshot.subcommands[index] = snapshotCompletionCommand(child, hasVersion)
		snapshot.childLookup[child.name] = index
		for _, alias := range child.aliases {
			snapshot.childLookup[alias] = index
		}
	}
	return snapshot
}

type completionPendingValue struct {
	scopeIndex  int
	optionIndex int
}

type completionResolution struct {
	commandPath []string
	commandIDs  []string
	target      CompletionTarget
	prefix      string
	valuePrefix string
	partial     []CompletionOccurrence
	candidates  []CompletionCandidate
	provider    CompletionProvider
}

type completionState struct {
	root              *completionEngineCommand
	commands          []*completionEngineCommand
	commandPath       []string
	positionalIndex   int
	positionalStarted bool
	optionsEnabled    bool
	pending           *completionPendingValue
	partial           []CompletionOccurrence
	helpMode          bool
	blocked           bool
}

func newCompletionState(root *completionEngineCommand, arguments []string) *completionState {
	state := &completionState{
		root:           root,
		commands:       []*completionEngineCommand{root},
		commandPath:    []string{root.name},
		optionsEnabled: true,
	}
	state.consume(arguments)
	return state
}

func (s *completionState) active() *completionEngineCommand {
	return s.commands[len(s.commands)-1]
}

func (s *completionState) commandIDPath() []string {
	path := make([]string, len(s.commands))
	for index, command := range s.commands {
		path[index] = command.id
	}
	return path
}

func (s *completionState) targetPath(scopeIndex int) []string {
	path := make([]string, scopeIndex+1)
	for index := range path {
		path[index] = s.commands[index].id
	}
	return path
}

func (s *completionState) consume(arguments []string) {
	if len(s.root.subcommands) > 0 && len(arguments) > 0 && arguments[0] == "help" {
		s.helpMode = true
		for _, argument := range arguments[1:] {
			if !s.selectSubcommand(argument) {
				s.blocked = true
				break
			}
			s.positionalStarted = false
		}
		return
	}
	for index := 0; index < len(arguments) && !s.blocked; {
		argument := arguments[index]
		switch {
		case s.optionsEnabled && argument == "--":
			s.optionsEnabled = false
			index++
		case s.optionsEnabled && strings.HasPrefix(argument, "--") && len(argument) > 2:
			index += s.consumeLong(arguments, index)
		case s.optionsEnabled && strings.HasPrefix(argument, "-") && !strings.HasPrefix(argument, "--") && len(argument) > 1:
			index += s.consumeShort(arguments, index)
		case s.optionsEnabled && !s.positionalStarted && s.selectSubcommand(argument):
			index++
		default:
			s.consumePositional(argument)
			index++
		}
	}
}

func (s *completionState) consumeLong(arguments []string, index int) int {
	body := arguments[index][2:]
	name, attached, hasAttached := strings.Cut(body, "=")
	if name == "help" || name == "version" {
		s.blocked = true
		return 1
	}
	scopeIndex, optionIndex, found := s.visibleLong(name)
	if !found {
		s.blocked = true
		return 1
	}
	option := &s.commands[scopeIndex].options[optionIndex]
	if hasAttached && option.kind != OptionValue {
		s.blocked = true
		return 1
	}
	switch option.kind {
	case OptionFlag:
		s.pushOptionOccurrence(scopeIndex, option, CompletionOccurrenceFlag, "", false)
	case OptionCount:
		s.pushOptionOccurrence(scopeIndex, option, CompletionOccurrenceCount, "", false)
	case OptionValue:
		if hasAttached {
			s.pushOptionOccurrence(scopeIndex, option, CompletionOccurrenceValue, attached, true)
		} else if index+1 < len(arguments) {
			s.pushOptionOccurrence(scopeIndex, option, CompletionOccurrenceValue, arguments[index+1], true)
			return 2
		} else {
			s.pending = &completionPendingValue{scopeIndex: scopeIndex, optionIndex: optionIndex}
		}
	}
	return 1
}

func (s *completionState) consumeShort(arguments []string, index int) int {
	argument := arguments[index]
	for offset := 1; offset < len(argument); offset++ {
		short := argument[offset]
		if !asciiAlphanumeric(short) || short == 'h' || short == 'V' {
			s.blocked = true
			return 1
		}
		scopeIndex, optionIndex, found := s.visibleShort(short)
		if !found {
			s.blocked = true
			return 1
		}
		option := &s.commands[scopeIndex].options[optionIndex]
		switch option.kind {
		case OptionFlag:
			s.pushOptionOccurrence(scopeIndex, option, CompletionOccurrenceFlag, "", false)
		case OptionCount:
			s.pushOptionOccurrence(scopeIndex, option, CompletionOccurrenceCount, "", false)
		case OptionValue:
			if offset+1 < len(argument) {
				s.pushOptionOccurrence(scopeIndex, option, CompletionOccurrenceValue, argument[offset+1:], true)
			} else if index+1 < len(arguments) {
				s.pushOptionOccurrence(scopeIndex, option, CompletionOccurrenceValue, arguments[index+1], true)
				return 2
			} else {
				s.pending = &completionPendingValue{scopeIndex: scopeIndex, optionIndex: optionIndex}
			}
			return 1
		}
	}
	return 1
}

func (s *completionState) consumePositional(raw string) {
	if s.positionalIndex >= len(s.active().arguments) {
		s.blocked = true
		return
	}
	argument := &s.active().arguments[s.positionalIndex]
	s.positionalStarted = true
	s.partial = append(s.partial, CompletionOccurrence{
		target: argumentCompletionTarget(s.commandIDPath(), argument.id),
		kind:   CompletionOccurrenceValue,
		raw:    raw,
		rawSet: true,
	})
	if !argument.repeated {
		s.positionalIndex++
	}
}

func (s *completionState) pushOptionOccurrence(
	scopeIndex int,
	option *completionEngineOption,
	kind CompletionOccurrenceKind,
	raw string,
	rawSet bool,
) {
	s.partial = append(s.partial, CompletionOccurrence{
		target: optionCompletionTarget(s.targetPath(scopeIndex), option.id),
		kind:   kind,
		raw:    raw,
		rawSet: rawSet,
	})
}

func (s *completionState) selectSubcommand(argument string) bool {
	index, found := s.active().childLookup[argument]
	if !found {
		return false
	}
	command := s.active().subcommands[index]
	s.commands = append(s.commands, command)
	s.commandPath = append(s.commandPath, command.name)
	s.positionalIndex = 0
	s.positionalStarted = false
	return true
}

func (s *completionState) visibleLong(name string) (int, int, bool) {
	active := len(s.commands) - 1
	for scopeIndex := active; scopeIndex >= 0; scopeIndex-- {
		for optionIndex := range s.commands[scopeIndex].options {
			option := &s.commands[scopeIndex].options[optionIndex]
			if option.long == name && (scopeIndex == active || option.inherited) {
				return scopeIndex, optionIndex, true
			}
		}
	}
	return 0, 0, false
}

func (s *completionState) visibleShort(short byte) (int, int, bool) {
	active := len(s.commands) - 1
	for scopeIndex := active; scopeIndex >= 0; scopeIndex-- {
		for optionIndex := range s.commands[scopeIndex].options {
			option := &s.commands[scopeIndex].options[optionIndex]
			if option.short == short && (scopeIndex == active || option.inherited) {
				return scopeIndex, optionIndex, true
			}
		}
	}
	return 0, 0, false
}

func (s *completionState) optionSeen(scopeIndex int, option *completionEngineOption) bool {
	for _, occurrence := range s.partial {
		if occurrence.target.kind == CompletionTargetOption &&
			len(occurrence.target.commandIDPath) == scopeIndex+1 &&
			completionPathMatches(occurrence.target.commandIDPath, s.commands[:scopeIndex+1]) &&
			occurrence.target.valueID == option.id {
			return true
		}
	}
	return false
}

func (s *completionState) resolve(current string) completionResolution {
	commandIDs := s.commandIDPath()
	commandPath := append([]string(nil), s.commandPath...)
	if s.blocked {
		return completionResolution{
			commandPath: commandPath,
			commandIDs:  commandIDs,
			target:      commandCompletionTarget(commandIDs),
			prefix:      current,
			partial:     s.partial,
		}
	}
	if s.helpMode {
		var candidates []CompletionCandidate
		s.pushSubcommands(&candidates, current)
		return completionResolution{
			commandPath: commandPath,
			commandIDs:  commandIDs,
			target:      commandCompletionTarget(commandIDs),
			prefix:      current,
			partial:     s.partial,
			candidates:  candidates,
		}
	}
	if s.pending != nil {
		return s.resolveOptionValue(
			s.pending.scopeIndex,
			s.pending.optionIndex,
			current,
			"",
		)
	}
	if s.optionsEnabled {
		if scopeIndex, optionIndex, prefix, valuePrefix, ok := s.attachedValueContext(current); ok {
			return s.resolveOptionValue(scopeIndex, optionIndex, prefix, valuePrefix)
		}
	}

	var candidates []CompletionCandidate
	completingOption := s.optionsEnabled && strings.HasPrefix(current, "-")
	completingWord := !completingOption
	var valueTarget *completionEngineArgument
	if completingWord && s.optionsEnabled && !s.positionalStarted {
		s.pushSubcommands(&candidates, current)
		if len(s.commands) == 1 && s.root.hasVisibleSubcommand() && strings.HasPrefix("help", current) {
			candidates = append(candidates,
				NewCompletionCandidate("help").
					WithKind(CompletionCandidateCommand).
					WithDescription("Show help for a command"),
			)
		}
	}
	if completingWord && s.positionalIndex < len(s.active().arguments) {
		valueTarget = &s.active().arguments[s.positionalIndex]
		s.pushValues(&candidates, valueTarget.possibleValues, "", current)
	}
	if s.optionsEnabled && (completingOption || current == "") {
		s.pushOptions(&candidates, current)
	}

	target := commandCompletionTarget(commandIDs)
	var provider CompletionProvider
	if valueTarget != nil {
		target = argumentCompletionTarget(commandIDs, valueTarget.id)
		provider = valueTarget.provider
	}
	return completionResolution{
		commandPath: commandPath,
		commandIDs:  commandIDs,
		target:      target,
		prefix:      current,
		partial:     s.partial,
		candidates:  candidates,
		provider:    provider,
	}
}

func (c *completionEngineCommand) hasVisibleSubcommand() bool {
	for _, command := range c.subcommands {
		if !command.hidden {
			return true
		}
	}
	return false
}

func (s *completionState) attachedValueContext(current string) (int, int, string, string, bool) {
	if strings.HasPrefix(current, "--") {
		body := current[2:]
		name, prefix, hasAttached := strings.Cut(body, "=")
		if !hasAttached {
			return 0, 0, "", "", false
		}
		scopeIndex, optionIndex, found := s.visibleLong(name)
		if !found || s.commands[scopeIndex].options[optionIndex].kind != OptionValue {
			return 0, 0, "", "", false
		}
		return scopeIndex, optionIndex, prefix, "--" + name + "=", true
	}
	if !strings.HasPrefix(current, "-") || strings.HasPrefix(current, "--") || len(current) <= 2 {
		return 0, 0, "", "", false
	}
	for offset := 1; offset < len(current); offset++ {
		short := current[offset]
		if !asciiAlphanumeric(short) {
			return 0, 0, "", "", false
		}
		scopeIndex, optionIndex, found := s.visibleShort(short)
		if !found {
			return 0, 0, "", "", false
		}
		if s.commands[scopeIndex].options[optionIndex].kind == OptionValue {
			return scopeIndex, optionIndex, current[offset+1:], current[:offset+1], true
		}
	}
	return 0, 0, "", "", false
}

func (s *completionState) resolveOptionValue(
	scopeIndex int,
	optionIndex int,
	prefix string,
	valuePrefix string,
) completionResolution {
	commandIDs := s.commandIDPath()
	option := &s.commands[scopeIndex].options[optionIndex]
	var candidates []CompletionCandidate
	var provider CompletionProvider
	if !option.hidden {
		s.pushValues(&candidates, option.possibleValues, valuePrefix, prefix)
		provider = option.provider
	}
	return completionResolution{
		commandPath: append([]string(nil), s.commandPath...),
		commandIDs:  commandIDs,
		target:      optionCompletionTarget(s.targetPath(scopeIndex), option.id),
		prefix:      prefix,
		valuePrefix: valuePrefix,
		partial:     s.partial,
		candidates:  candidates,
		provider:    provider,
	}
}

func (s *completionState) pushValues(
	candidates *[]CompletionCandidate,
	values []string,
	valuePrefix string,
	filterPrefix string,
) {
	for _, value := range values {
		if !strings.HasPrefix(value, filterPrefix) {
			continue
		}
		*candidates = append(*candidates,
			NewCompletionCandidate(value).
				WithKind(CompletionCandidateValue).
				withValuePrefix(valuePrefix),
		)
	}
}

func (s *completionState) pushSubcommands(candidates *[]CompletionCandidate, prefix string) {
	for _, command := range s.active().subcommands {
		if command.hidden {
			continue
		}
		if strings.HasPrefix(command.name, prefix) {
			*candidates = append(*candidates,
				NewCompletionCandidate(command.name).
					WithKind(CompletionCandidateCommand).
					WithDescription(command.description).
					withDeprecation(command.deprecation),
			)
		}
		for _, alias := range command.aliases {
			if strings.HasPrefix(alias, prefix) {
				*candidates = append(*candidates,
					NewCompletionCandidate(alias).
						WithKind(CompletionCandidateCommand).
						WithDescription(command.description).
						withDeprecation(command.deprecation),
				)
			}
		}
	}
}

func (s *completionState) pushOptions(candidates *[]CompletionCandidate, prefix string) {
	active := len(s.commands) - 1
	for scopeIndex, command := range s.commands {
		for optionIndex := range command.options {
			option := &command.options[optionIndex]
			if scopeIndex != active && !option.inherited {
				continue
			}
			if option.hidden {
				continue
			}
			if option.kind != OptionCount && !option.repeated && s.optionSeen(scopeIndex, option) {
				continue
			}
			if option.long != "" && completionCandidatePartsStartWith("--", option.long, prefix) {
				*candidates = append(*candidates,
					NewCompletionCandidate("--"+option.long).
						WithKind(CompletionCandidateOption).
						WithDescription(option.help).
						withDeprecation(option.deprecation),
				)
			}
			if option.short != 0 && completionShortStartsWith(option.short, prefix) {
				*candidates = append(*candidates,
					NewCompletionCandidate(fmt.Sprintf("-%c", option.short)).
						WithKind(CompletionCandidateOption).
						WithDescription(option.help).
						withDeprecation(option.deprecation),
				)
			}
		}
	}
	if strings.HasPrefix("--help", prefix) {
		*candidates = append(*candidates,
			NewCompletionCandidate("--help").WithKind(CompletionCandidateOption).WithDescription("Show help"),
		)
	}
	if strings.HasPrefix("-h", prefix) {
		*candidates = append(*candidates,
			NewCompletionCandidate("-h").WithKind(CompletionCandidateOption).WithDescription("Show help"),
		)
	}
	if s.root.hasVersion {
		if strings.HasPrefix("--version", prefix) {
			*candidates = append(*candidates,
				NewCompletionCandidate("--version").WithKind(CompletionCandidateOption).WithDescription("Show version"),
			)
		}
		if strings.HasPrefix("-V", prefix) {
			*candidates = append(*candidates,
				NewCompletionCandidate("-V").WithKind(CompletionCandidateOption).WithDescription("Show version"),
			)
		}
	}
}

func completionPathMatches(path []string, commands []*completionEngineCommand) bool {
	for index, command := range commands {
		if path[index] != command.id {
			return false
		}
	}
	return true
}

func completionCandidatePartsStartWith(head, tail, prefix string) bool {
	if len(prefix) <= len(head) {
		return strings.HasPrefix(head, prefix)
	}
	return strings.HasPrefix(prefix, head) && strings.HasPrefix(tail, prefix[len(head):])
}

func completionShortStartsWith(short byte, prefix string) bool {
	if len(prefix) > 2 {
		return false
	}
	spelling := [2]byte{'-', short}
	for index := range len(prefix) {
		if prefix[index] != spelling[index] {
			return false
		}
	}
	return true
}

func invalidCompletionCandidate(candidate CompletionCandidate) string {
	if candidate.value == "" {
		return "candidate value is empty"
	}
	if candidate.kind > CompletionCandidateValue {
		return "candidate kind is invalid"
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "value", value: candidate.value},
		{name: "display label", value: candidate.displayLabel},
		{name: "description", value: candidate.description},
	} {
		if !utf8.ValidString(field.value) {
			return "candidate " + field.name + " is not valid UTF-8"
		}
		for _, character := range field.value {
			if unicode.IsControl(character) {
				return "candidate " + field.name + " contains a control character"
			}
		}
	}
	return ""
}

func cancelledCompletionError(target CompletionTarget, targetSet bool, cause error) *CompletionError {
	return &CompletionError{
		kind:      CompletionErrorCancelled,
		target:    cloneCompletionTarget(target),
		targetSet: targetSet,
		message:   "completion was cancelled",
		cause:     cause,
	}
}

func invalidCandidateCompletionError(target CompletionTarget, reason string) *CompletionError {
	return &CompletionError{
		kind:      CompletionErrorInvalidCandidate,
		target:    cloneCompletionTarget(target),
		targetSet: true,
		message:   reason,
	}
}

func cloneCompletionTarget(target CompletionTarget) CompletionTarget {
	target.commandIDPath = append([]string(nil), target.commandIDPath...)
	return target
}

func cloneCompletionOccurrence(occurrence CompletionOccurrence) CompletionOccurrence {
	occurrence.target = cloneCompletionTarget(occurrence.target)
	return occurrence
}

func cloneCompletionRequest(request CompletionRequest) CompletionRequest {
	request.arguments = append([]string(nil), request.arguments...)
	request.commandPath = append([]string(nil), request.commandPath...)
	request.commandIDs = append([]string(nil), request.commandIDs...)
	request.target = cloneCompletionTarget(request.target)
	partial := make([]CompletionOccurrence, len(request.partial))
	for index := range request.partial {
		partial[index] = cloneCompletionOccurrence(request.partial[index])
	}
	request.partial = partial
	return request
}
