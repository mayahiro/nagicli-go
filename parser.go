package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

type invocationValue struct {
	kind     OptionKind
	flag     bool
	count    uint64
	values   []ParsedValue
	repeated bool
	supplied bool
}

type invocationDefinition struct {
	kind      OptionKind
	repeated  bool
	sensitive bool
	argument  bool
}

type invocationScopeData struct {
	definitions map[string]invocationDefinition
	values      map[string]invocationValue
}

// InvocationScope is one exact command-local value scope
type InvocationScope struct {
	invocation *Invocation
	index      int
}

// Invocation is one canonical parsed command path and its command-local typed
// value scopes
//
// Unqualified access starts at the current scope and searches ancestors. The
// nearest declaration shadows an ancestor even when it has no resolved value.
// Validators temporarily use their defining Command as current; handlers and
// returned Invocations use the selected leaf
type Invocation struct {
	commandPath        []string
	commandIDPath      []string
	scopes             []invocationScopeData
	currentScope       int
	deprecationNotices []DeprecationNotice
}

// CommandPath returns a copy of the canonical root-to-leaf path
func (i *Invocation) CommandPath() []string {
	return append([]string(nil), i.commandPath...)
}

// CommandIDPath returns a copy of the stable root-to-leaf command-ID path
func (i *Invocation) CommandIDPath() []string {
	return append([]string(nil), i.commandIDPath...)
}

// DeprecationNotices returns deprecated Command and Option uses in
// deterministic first-use order
//
// A deprecated root Command comes first. Subsequent targets follow their first
// successful argv occurrence. Each stable target occurs at most once.
// Environment, external, and default value resolution do not produce notices
func (i *Invocation) DeprecationNotices() []DeprecationNotice {
	notices := make([]DeprecationNotice, len(i.deprecationNotices))
	for index, notice := range i.deprecationNotices {
		notices[index] = cloneDeprecationNotice(notice)
	}
	return notices
}

// ValueScopeIDPath returns the current stable path where unqualified lookup
// starts
func (i *Invocation) ValueScopeIDPath() []string {
	return append([]string(nil), i.commandIDPath[:i.currentScope+1]...)
}

// CurrentScope returns the exact scope where unqualified lookup starts
func (i *Invocation) CurrentScope() InvocationScope {
	return InvocationScope{invocation: i, index: i.currentScope}
}

// Scope returns an exact local scope selected by stable command-ID path
func (i *Invocation) Scope(commandIDPath ...string) (InvocationScope, bool) {
	for index := range i.scopes {
		if equalPath(i.commandIDPath[:index+1], commandIDPath) {
			return InvocationScope{invocation: i, index: index}, true
		}
	}
	return InvocationScope{}, false
}

// Scopes returns exact command scopes in root-to-leaf order
func (i *Invocation) Scopes() []InvocationScope {
	scopes := make([]InvocationScope, len(i.scopes))
	for index := range i.scopes {
		scopes[index] = InvocationScope{invocation: i, index: index}
	}
	return scopes
}

// Contains reports whether the nearest visible declaration has a value
func (i *Invocation) Contains(id string) bool {
	_, _, _, present := i.lookup(id)
	return present
}

// Supplied reports whether the nearest visible declaration was present in argv
func (i *Invocation) Supplied(id string) bool {
	_, value, _, present := i.lookup(id)
	return present && value.supplied
}

// Flag returns Boolean presence for the nearest visible flag declaration
func (i *Invocation) Flag(id string) (bool, bool) {
	definition, value, declared, present := i.lookup(id)
	if !declared || !present || definition.kind != OptionFlag {
		return false, false
	}
	return value.flag, true
}

// Count returns occurrences for the nearest visible count declaration
func (i *Invocation) Count(id string) (uint64, bool) {
	definition, value, declared, present := i.lookup(id)
	if !declared || !present || definition.kind != OptionCount {
		return 0, false
	}
	return value.count, true
}

// ParsedValues returns values for the nearest visible Value declaration
func (i *Invocation) ParsedValues(id string) []ParsedValue {
	return append([]ParsedValue(nil), i.parsedValuesForLookup(id)...)
}

func (i *Invocation) parsedValuesForLookup(id string) []ParsedValue {
	definition, value, declared, present := i.lookup(id)
	if !declared || !present || definition.kind != OptionValue {
		return nil
	}
	return value.values
}

// RawValue returns the first raw value
func (i *Invocation) RawValue(id string) (string, bool) {
	values := i.parsedValuesForLookup(id)
	if len(values) == 0 {
		return "", false
	}
	return values[0].raw, true
}

// IsRepeated reports whether a value ID was declared as repeatable
func (i *Invocation) IsRepeated(id string) bool {
	definition, _, declared, _ := i.lookup(id)
	return declared && definition.kind == OptionValue && definition.repeated
}

// ValueIsSensitive reports whether the nearest visible Value declaration is
// Sensitive
func (i *Invocation) ValueIsSensitive(id string) bool {
	definition, _, declared, _ := i.lookup(id)
	return declared && definition.kind == OptionValue && definition.sensitive
}

// ValueIDs returns sorted visible IDs that have a value
func (i *Invocation) ValueIDs() []string {
	seen := map[string]struct{}{}
	var ids []string
	for index := i.currentScope; index >= 0; index-- {
		scope := &i.scopes[index]
		for id := range scope.definitions {
			if _, hidden := seen[id]; hidden {
				continue
			}
			seen[id] = struct{}{}
			if _, present := scope.values[id]; present {
				ids = append(ids, id)
			}
		}
	}
	sort.Strings(ids)
	return ids
}

func (i *Invocation) lookup(id string) (invocationDefinition, invocationValue, bool, bool) {
	for index := i.currentScope; index >= 0; index-- {
		scope := &i.scopes[index]
		definition, declared := scope.definitions[id]
		if !declared {
			continue
		}
		value, present := scope.values[id]
		return definition, value, true, present
	}
	return invocationDefinition{}, invocationValue{}, false, false
}

func (i *Invocation) markSensitiveTarget(target DiagnosticTarget) DiagnosticTarget {
	if target.kind == TargetResponseFile {
		return target
	}
	scopeIndex := -1
	if len(target.commandIDPath) == 0 {
		scopeIndex = i.currentScope
	} else {
		for index := range i.scopes {
			if equalPath(i.commandIDPath[:index+1], target.commandIDPath) {
				scopeIndex = index
				break
			}
		}
	}
	if scopeIndex < 0 {
		return target
	}
	definition, declared := i.scopes[scopeIndex].definitions[target.valueID]
	if !declared {
		return target
	}
	targetIsArgument := target.kind == TargetArgument
	return target.withSensitive(
		definition.kind == OptionValue &&
			definition.argument == targetIsArgument &&
			definition.sensitive,
	)
}

// CommandPath returns the canonical path prefix for this exact scope
func (s InvocationScope) CommandPath() []string {
	if !s.valid() {
		return nil
	}
	return append([]string(nil), s.invocation.commandPath[:s.index+1]...)
}

// CommandIDPath returns the stable command-ID path for this exact scope
func (s InvocationScope) CommandIDPath() []string {
	if !s.valid() {
		return nil
	}
	return append([]string(nil), s.invocation.commandIDPath[:s.index+1]...)
}

// ValueScopeIDPath returns the stable path used by this exact value scope
func (s InvocationScope) ValueScopeIDPath() []string {
	return s.CommandIDPath()
}

// Contains reports whether one local declaration has a value
func (s InvocationScope) Contains(id string) bool {
	_, _, _, present := s.lookup(id)
	return present
}

// Supplied reports whether one local declaration was present in argv
func (s InvocationScope) Supplied(id string) bool {
	_, value, _, present := s.lookup(id)
	return present && value.supplied
}

// Flag returns Boolean presence for one local flag declaration
func (s InvocationScope) Flag(id string) (bool, bool) {
	definition, value, declared, present := s.lookup(id)
	if !declared || !present || definition.kind != OptionFlag {
		return false, false
	}
	return value.flag, true
}

// Count returns occurrences for one local count declaration
func (s InvocationScope) Count(id string) (uint64, bool) {
	definition, value, declared, present := s.lookup(id)
	if !declared || !present || definition.kind != OptionCount {
		return 0, false
	}
	return value.count, true
}

// ParsedValues returns a copy of local parsed values and sources
func (s InvocationScope) ParsedValues(id string) []ParsedValue {
	return append([]ParsedValue(nil), s.parsedValuesForLookup(id)...)
}

func (s InvocationScope) parsedValuesForLookup(id string) []ParsedValue {
	definition, value, declared, present := s.lookup(id)
	if !declared || !present || definition.kind != OptionValue {
		return nil
	}
	return value.values
}

// RawValue returns the first local raw value
func (s InvocationScope) RawValue(id string) (string, bool) {
	values := s.parsedValuesForLookup(id)
	if len(values) == 0 {
		return "", false
	}
	return values[0].raw, true
}

// IsRepeated reports whether a local Value declaration is repeatable
func (s InvocationScope) IsRepeated(id string) bool {
	definition, _, declared, _ := s.lookup(id)
	return declared && definition.kind == OptionValue && definition.repeated
}

// ValueIsSensitive reports whether one local Value declaration is Sensitive
func (s InvocationScope) ValueIsSensitive(id string) bool {
	definition, _, declared, _ := s.lookup(id)
	return declared && definition.kind == OptionValue && definition.sensitive
}

// ValueIDs returns sorted local IDs that have a value
func (s InvocationScope) ValueIDs() []string {
	if !s.valid() {
		return nil
	}
	scope := &s.invocation.scopes[s.index]
	ids := make([]string, 0, len(scope.values))
	for id := range scope.values {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (s InvocationScope) valid() bool {
	return s.invocation != nil && s.index >= 0 && s.index < len(s.invocation.scopes)
}

func (s InvocationScope) lookup(id string) (invocationDefinition, invocationValue, bool, bool) {
	if !s.valid() {
		return invocationDefinition{}, invocationValue{}, false, false
	}
	scope := &s.invocation.scopes[s.index]
	definition, declared := scope.definitions[id]
	if !declared {
		return invocationDefinition{}, invocationValue{}, false, false
	}
	value, present := scope.values[id]
	return definition, value, true, present
}

func equalPath(left, right []string) bool {
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

// ParseResultKind distinguishes invocation, help, and version actions
type ParseResultKind uint8

const (
	// ParseInvocation is a validated Invocation
	ParseInvocation ParseResultKind = iota
	// ParseHelp requests help for the selected command
	ParseHelp
	// ParseVersion requests the configured root version
	ParseVersion
)

// ParseResult is the result of argv parsing before handler execution
type ParseResult struct {
	kind          ParseResultKind
	invocation    *Invocation
	commandPath   []string
	commandIDPath []string
	version       string
}

// Kind returns the parse result category
func (r ParseResult) Kind() ParseResultKind { return r.kind }

// Invocation returns the validated invocation when Kind is ParseInvocation
func (r ParseResult) Invocation() *Invocation { return r.invocation }

// CommandPath returns the selected canonical command path
func (r ParseResult) CommandPath() []string {
	if r.invocation != nil {
		return r.invocation.CommandPath()
	}
	return append([]string(nil), r.commandPath...)
}

// CommandIDPath returns the selected stable command-ID path
func (r ParseResult) CommandIDPath() []string {
	if r.invocation != nil {
		return r.invocation.CommandIDPath()
	}
	return append([]string(nil), r.commandIDPath...)
}

// Version returns the configured version when Kind is ParseVersion
func (r ParseResult) Version() string { return r.version }

// Parse parses arguments after the program name with an empty environment
func (c *Command) Parse(arguments []string) (ParseResult, error) {
	return c.ParseWithEnvironment(arguments, nil)
}

// ParseWithEnvironment parses argv and injected environment values
func (c *Command) ParseWithEnvironment(arguments []string, environment map[string]string) (ParseResult, error) {
	return c.parseWithOptionalValueResolver(arguments, environment, nil)
}

// ParseWithValueResolver parses argv, environment, and application-owned fallbacks
//
// The resolver is called only for selected Value Options that have no
// command-line or environment value. A nil resolver behaves like
// ParseWithEnvironment
func (c *Command) ParseWithValueResolver(
	arguments []string,
	environment map[string]string,
	resolver ValueResolver,
) (ParseResult, error) {
	return c.parseWithOptionalValueResolver(arguments, environment, resolver)
}

func (c *Command) parseWithOptionalValueResolver(
	arguments []string,
	environment map[string]string,
	resolver ValueResolver,
) (ParseResult, error) {
	if err := c.Validate(); err != nil {
		return ParseResult{}, err
	}
	return c.parseValidatedWithOptionalValueResolver(arguments, environment, resolver)
}

func (c *Command) parseValidatedWithOptionalValueResolver(
	arguments []string,
	environment map[string]string,
	resolver ValueResolver,
) (ParseResult, error) {
	copyEnvironment := make(map[string]string, len(environment))
	for key, value := range environment {
		copyEnvironment[key] = value
	}
	parser := &argumentParser{
		root:           c,
		arguments:      append([]string(nil), arguments...),
		environment:    copyEnvironment,
		resolver:       resolver,
		commands:       []*Command{c},
		commandPath:    []string{c.name},
		scopes:         []invocationScopeData{newInvocationScopeData(c)},
		optionsEnabled: true,
	}
	if c.deprecation.configured {
		parser.pushDeprecationNotice(commandDeprecationNotice(
			[]string{c.name},
			[]string{c.id},
			c.name,
			c.deprecation,
		))
	}
	return parser.parse()
}

type argumentParser struct {
	root               *Command
	arguments          []string
	environment        map[string]string
	resolver           ValueResolver
	index              int
	commands           []*Command
	commandPath        []string
	scopes             []invocationScopeData
	positionalIndex    int
	positionalStarted  bool
	optionsEnabled     bool
	deprecationNotices []DeprecationNotice
}

func (p *argumentParser) parse() (ParseResult, error) {
	if result, matched, err := p.parseHelpCommand(); matched || err != nil {
		return result, err
	}
	for p.index < len(p.arguments) {
		argument := p.arguments[p.index]
		switch {
		case p.optionsEnabled && argument == "--":
			p.optionsEnabled = false
			p.index++
		case p.optionsEnabled && strings.HasPrefix(argument, "--") && len(argument) > 2:
			result, done, err := p.parseLong(argument)
			if err != nil || done {
				return result, err
			}
		case p.optionsEnabled && strings.HasPrefix(argument, "-") && len(argument) > 1:
			result, done, err := p.parseShort(argument)
			if err != nil || done {
				return result, err
			}
		case p.optionsEnabled && !p.positionalStarted && p.selectSubcommand(argument):
			p.index++
		default:
			if err := p.parsePositional(argument); err != nil {
				return ParseResult{}, err
			}
			p.index++
		}
	}
	selectedCommandIDPath := p.currentCommandIDPath()
	if err := p.resolveFallbacks(selectedCommandIDPath); err != nil {
		return ParseResult{}, err
	}
	if err := p.validateValues(); err != nil {
		return ParseResult{}, err
	}
	invocation := &Invocation{
		commandPath:        append([]string(nil), p.commandPath...),
		commandIDPath:      selectedCommandIDPath,
		scopes:             p.scopes,
		currentScope:       len(p.scopes) - 1,
		deprecationNotices: p.deprecationNotices,
	}
	if err := p.runValidators(invocation); err != nil {
		return ParseResult{}, err
	}
	return ParseResult{
		kind:       ParseInvocation,
		invocation: invocation,
	}, nil
}

func (p *argumentParser) active() *Command { return p.commands[len(p.commands)-1] }

func (p *argumentParser) currentCommandIDPath() []string {
	path := make([]string, len(p.commands))
	for index, command := range p.commands {
		path[index] = command.id
	}
	return path
}

func newInvocationScopeData(command *Command) invocationScopeData {
	scope := invocationScopeData{
		definitions: make(map[string]invocationDefinition, len(command.options)+len(command.arguments)),
		values:      map[string]invocationValue{},
	}
	for index := range command.options {
		option := &command.options[index]
		scope.definitions[option.id] = invocationDefinition{
			kind:      option.kind,
			repeated:  option.repeated,
			sensitive: option.sensitive,
			argument:  false,
		}
	}
	for index := range command.arguments {
		argument := &command.arguments[index]
		scope.definitions[argument.id] = invocationDefinition{
			kind:      OptionValue,
			repeated:  argument.repeated,
			sensitive: argument.sensitive,
			argument:  true,
		}
	}
	return scope
}

func (p *argumentParser) parseHelpCommand() (ParseResult, bool, error) {
	if len(p.arguments) == 0 || p.arguments[0] != "help" {
		return ParseResult{}, false, nil
	}
	command := p.root
	path := []string{p.root.name}
	idPath := []string{p.root.id}
	for _, target := range p.arguments[1:] {
		var selected *Command
		for _, child := range command.subcommands {
			if child.name == target {
				selected = child
				break
			}
			for _, alias := range child.aliases {
				if alias == target {
					selected = child
					break
				}
			}
			if selected != nil {
				break
			}
		}
		if selected == nil {
			p.commandPath = path
			return ParseResult{}, true, p.diag(CodeUnknownCommand, "unknown command "+quoteValue(target))
		}
		command = selected
		path = append(path, command.name)
		idPath = append(idPath, command.id)
	}
	return ParseResult{
		kind:          ParseHelp,
		commandPath:   path,
		commandIDPath: idPath,
	}, true, nil
}

func (p *argumentParser) parseLong(argument string) (ParseResult, bool, error) {
	body := argument[2:]
	name, attached, hasAttached := strings.Cut(body, "=")
	if name == "help" {
		if hasAttached {
			return ParseResult{}, false, p.diag(CodeUnexpectedOptionValue, "option '--help' does not take a value")
		}
		return ParseResult{
			kind:          ParseHelp,
			commandPath:   append([]string(nil), p.commandPath...),
			commandIDPath: p.currentCommandIDPath(),
		}, true, nil
	}
	if name == "version" && p.root.version != "" {
		if hasAttached {
			return ParseResult{}, false, p.diag(CodeUnexpectedOptionValue, "option '--version' does not take a value")
		}
		return ParseResult{
			kind:          ParseVersion,
			commandPath:   []string{p.root.name},
			commandIDPath: []string{p.root.id},
			version:       p.root.version,
		}, true, nil
	}
	scopeIndex, option := p.visibleLongOption(name)
	if option == nil {
		return ParseResult{}, false, p.diag(CodeUnknownOption, "unknown option "+quoteValue(argument))
	}
	p.index++
	var value *string
	if hasAttached {
		value = &attached
	}
	firstOccurrence, err := p.applyOption(scopeIndex, option, value)
	if err != nil {
		return ParseResult{}, false, err
	}
	if firstOccurrence && option.deprecation.configured {
		p.pushDeprecationNotice(optionDeprecationNotice(
			p.commandPath,
			p.commandIDPath(scopeIndex),
			option.id,
			"--"+name,
			option.deprecation,
		))
	}
	return ParseResult{}, false, nil
}

func (p *argumentParser) parseShort(argument string) (ParseResult, bool, error) {
	p.index++
	for offset := 1; offset < len(argument); offset++ {
		short := argument[offset]
		if !asciiAlphanumeric(short) {
			return ParseResult{}, false, p.diag(CodeUnknownOption, "unknown option "+quoteValue(argument))
		}
		if short == 'h' {
			return ParseResult{
				kind:          ParseHelp,
				commandPath:   append([]string(nil), p.commandPath...),
				commandIDPath: p.currentCommandIDPath(),
			}, true, nil
		}
		if short == 'V' && p.root.version != "" {
			return ParseResult{
				kind:          ParseVersion,
				commandPath:   []string{p.root.name},
				commandIDPath: []string{p.root.id},
				version:       p.root.version,
			}, true, nil
		}
		scopeIndex, option := p.visibleShortOption(short)
		if option == nil {
			return ParseResult{}, false, p.diag(CodeUnknownOption, fmt.Sprintf("unknown option '-%c'", short))
		}
		if option.kind == OptionValue {
			var attached *string
			if offset+1 < len(argument) {
				value := argument[offset+1:]
				attached = &value
			}
			firstOccurrence, err := p.applyOption(scopeIndex, option, attached)
			if err != nil {
				return ParseResult{}, false, err
			}
			if firstOccurrence && option.deprecation.configured {
				p.recordOptionDeprecation(scopeIndex, option, fmt.Sprintf("-%c", short))
			}
			return ParseResult{}, false, nil
		}
		firstOccurrence, err := p.applyOption(scopeIndex, option, nil)
		if err != nil {
			return ParseResult{}, false, err
		}
		if firstOccurrence && option.deprecation.configured {
			p.recordOptionDeprecation(scopeIndex, option, fmt.Sprintf("-%c", short))
		}
	}
	return ParseResult{}, false, nil
}

func (p *argumentParser) applyOption(
	scopeIndex int,
	option *OptionSpec,
	attached *string,
) (bool, error) {
	values := p.scopes[scopeIndex].values
	target := func() DiagnosticTarget {
		return OptionTarget(option.id).WithCommandIDPath(p.commandIDPath(scopeIndex)...)
	}
	switch option.kind {
	case OptionFlag:
		if attached != nil {
			return false, p.diagTargets(
				CodeUnexpectedOptionValue,
				fmt.Sprintf("option %q does not take a value", optionDisplay(option)),
				target(),
			)
		}
		if _, duplicate := values[option.id]; duplicate {
			return false, p.diagTargets(
				CodeDuplicateOption,
				fmt.Sprintf("option %q was provided more than once", optionDisplay(option)),
				target(),
			)
		}
		values[option.id] = invocationValue{kind: OptionFlag, flag: true, supplied: true}
		return true, nil
	case OptionCount:
		if attached != nil {
			return false, p.diagTargets(
				CodeUnexpectedOptionValue,
				fmt.Sprintf("option %q does not take a value", optionDisplay(option)),
				target(),
			)
		}
		value, duplicate := values[option.id]
		value.kind = OptionCount
		value.supplied = true
		if value.count != math.MaxUint64 {
			value.count++
		}
		values[option.id] = value
		return !duplicate, nil
	case OptionValue:
		var raw string
		if attached != nil {
			raw = *attached
		} else {
			if p.index >= len(p.arguments) {
				return false, p.diagTargets(
					CodeMissingOptionValue,
					fmt.Sprintf("option %q requires a value", optionDisplay(option)),
					target(),
				)
			}
			raw = p.arguments[p.index]
			p.index++
		}
		if _, duplicate := values[option.id]; duplicate && !option.repeated {
			return false, p.diagTargets(
				CodeDuplicateOption,
				fmt.Sprintf("option %q was provided more than once", optionDisplay(option)),
				target(),
			)
		}
		_, duplicate := values[option.id]
		value, err := p.parseValue(
			option.id,
			option.parser,
			raw,
			commandLineValueOrigin(),
			option.sensitive,
			target(),
		)
		if err != nil {
			return false, err
		}
		p.pushValue(scopeIndex, option.id, value, option.repeated)
		return !duplicate, nil
	}
	return false, nil
}

func (p *argumentParser) visibleLongOption(name string) (int, *OptionSpec) {
	activeIndex := len(p.commands) - 1
	for scopeIndex := activeIndex; scopeIndex >= 0; scopeIndex-- {
		command := p.commands[scopeIndex]
		for optionIndex := range command.options {
			option := &command.options[optionIndex]
			if option.long == name && (scopeIndex == activeIndex || option.inherited) {
				return scopeIndex, option
			}
		}
	}
	return 0, nil
}

func (p *argumentParser) visibleShortOption(short byte) (int, *OptionSpec) {
	activeIndex := len(p.commands) - 1
	for scopeIndex := activeIndex; scopeIndex >= 0; scopeIndex-- {
		command := p.commands[scopeIndex]
		for optionIndex := range command.options {
			option := &command.options[optionIndex]
			if option.short == short && (scopeIndex == activeIndex || option.inherited) {
				return scopeIndex, option
			}
		}
	}
	return 0, nil
}

func (p *argumentParser) selectSubcommand(argument string) bool {
	for _, command := range p.active().subcommands {
		if command.name == argument {
			p.commands = append(p.commands, command)
			p.commandPath = append(p.commandPath, command.name)
			p.scopes = append(p.scopes, newInvocationScopeData(command))
			p.positionalIndex = 0
			p.positionalStarted = false
			p.recordCommandDeprecation(command, argument)
			return true
		}
		for _, alias := range command.aliases {
			if alias == argument {
				p.commands = append(p.commands, command)
				p.commandPath = append(p.commandPath, command.name)
				p.scopes = append(p.scopes, newInvocationScopeData(command))
				p.positionalIndex = 0
				p.positionalStarted = false
				p.recordCommandDeprecation(command, argument)
				return true
			}
		}
	}
	return false
}

func (p *argumentParser) recordCommandDeprecation(command *Command, spelling string) {
	if !command.deprecation.configured {
		return
	}
	p.pushDeprecationNotice(commandDeprecationNotice(
		p.commandPath,
		p.currentCommandIDPath(),
		spelling,
		command.deprecation,
	))
}

func (p *argumentParser) recordOptionDeprecation(
	scopeIndex int,
	option *OptionSpec,
	spelling string,
) {
	if !option.deprecation.configured {
		return
	}
	p.pushDeprecationNotice(optionDeprecationNotice(
		p.commandPath,
		p.commandIDPath(scopeIndex),
		option.id,
		spelling,
		option.deprecation,
	))
}

func (p *argumentParser) pushDeprecationNotice(notice DeprecationNotice) {
	p.deprecationNotices = append(p.deprecationNotices, notice)
}

func cloneDeprecationNotice(notice DeprecationNotice) DeprecationNotice {
	notice.commandPath = append([]string(nil), notice.commandPath...)
	notice.commandIDPath = append([]string(nil), notice.commandIDPath...)
	return notice
}

func (p *argumentParser) parsePositional(raw string) error {
	command := p.active()
	if p.positionalIndex >= len(command.arguments) {
		if !p.positionalStarted && len(command.arguments) == 0 && len(command.subcommands) > 0 {
			return p.diag(CodeUnknownCommand, "unknown command "+quoteValue(raw))
		}
		return p.diag(CodeUnexpectedArgument, "unexpected argument "+quoteValue(raw))
	}
	argument := &command.arguments[p.positionalIndex]
	p.positionalStarted = true
	value, err := p.parseValue(
		argument.id,
		argument.parser,
		raw,
		commandLineValueOrigin(),
		argument.sensitive,
		ArgumentTarget(argument.id),
	)
	if err != nil {
		return err
	}
	p.pushValue(len(p.scopes)-1, argument.id, value, argument.repeated)
	if !argument.repeated {
		p.positionalIndex++
	}
	return nil
}

func (p *argumentParser) resolveFallbacks(selectedCommandIDPath []string) error {
	for commandIndex, command := range p.commands {
		values := p.scopes[commandIndex].values
		for index := range command.options {
			option := &command.options[index]
			if option.kind != OptionValue {
				continue
			}
			if _, present := values[option.id]; present {
				continue
			}
			if option.environment != "" {
				raw, present := p.environment[option.environment]
				if present {
					origin := environmentValueOrigin(option.environment)
					value, err := p.parseValue(
						option.id,
						option.parser,
						raw,
						origin,
						option.sensitive,
						OptionTarget(option.id).
							WithCommandIDPath(p.commandIDPath(commandIndex)...).
							withValueOrigin(origin),
					)
					if err != nil {
						return err
					}
					p.pushValue(commandIndex, option.id, value, option.repeated)
					continue
				}
			}
			if p.resolver != nil {
				resolution, diagnostic := p.resolver(ValueResolutionRequest{
					selectedCommandPath:   p.commandPath,
					selectedCommandIDPath: selectedCommandIDPath,
					commandPath:           p.commandPath[:commandIndex+1],
					commandIDPath:         selectedCommandIDPath[:commandIndex+1],
					valueID:               option.id,
					repeated:              option.repeated,
					sensitive:             option.sensitive,
				})
				if diagnostic != nil {
					return p.resolverDiagnostic(diagnostic, commandIndex, option.id)
				}
				if resolution.resolved {
					if err := p.applyExternalResolution(commandIndex, option, resolution); err != nil {
						return err
					}
					continue
				}
			}
			if option.defaultSet {
				origin := defaultValueOrigin()
				value, err := p.parseValue(
					option.id,
					option.parser,
					option.defaultVal,
					origin,
					option.sensitive,
					OptionTarget(option.id).
						WithCommandIDPath(p.commandIDPath(commandIndex)...).
						withValueOrigin(origin),
				)
				if err != nil {
					return err
				}
				p.pushValue(commandIndex, option.id, value, option.repeated)
			}
		}
	}
	return nil
}

func (p *argumentParser) applyExternalResolution(
	commandIndex int,
	option *OptionSpec,
	resolution ValueResolution,
) error {
	if !validID(resolution.sourceIdentity) {
		return p.diagTargets(
			CodeInvalidSpecification,
			fmt.Sprintf("Value Resolver returned an invalid source identity for %q", option.id),
			OptionTarget(option.id).WithCommandIDPath(p.commandIDPath(commandIndex)...),
		)
	}
	if len(resolution.values) == 0 {
		return p.diagTargets(
			CodeInvalidSpecification,
			fmt.Sprintf("Value Resolver returned an empty resolved result for %q", option.id),
			OptionTarget(option.id).WithCommandIDPath(p.commandIDPath(commandIndex)...),
		)
	}
	if resolution.mode != ValueResolutionReplace && resolution.mode != ValueResolutionMerge {
		return p.diagTargets(
			CodeInvalidSpecification,
			fmt.Sprintf("Value Resolver returned an invalid mode for %q", option.id),
			OptionTarget(option.id).WithCommandIDPath(p.commandIDPath(commandIndex)...),
		)
	}
	if !option.repeated && (len(resolution.values) != 1 || resolution.mode != ValueResolutionReplace) {
		return p.diagTargets(
			CodeInvalidSpecification,
			fmt.Sprintf("Value Resolver returned repeated or merged values for non-repeated %q", option.id),
			OptionTarget(option.id).WithCommandIDPath(p.commandIDPath(commandIndex)...),
		)
	}

	for _, raw := range resolution.values {
		origin := externalValueOrigin(resolution.sourceIdentity)
		value, err := p.parseValue(
			option.id,
			option.parser,
			raw,
			origin,
			option.sensitive,
			OptionTarget(option.id).
				WithCommandIDPath(p.commandIDPath(commandIndex)...).
				withValueOrigin(origin),
		)
		if err != nil {
			return err
		}
		p.pushValue(commandIndex, option.id, value, option.repeated)
	}
	if resolution.mode == ValueResolutionMerge && option.defaultSet {
		origin := defaultValueOrigin()
		value, err := p.parseValue(
			option.id,
			option.parser,
			option.defaultVal,
			origin,
			option.sensitive,
			OptionTarget(option.id).
				WithCommandIDPath(p.commandIDPath(commandIndex)...).
				withValueOrigin(origin),
		)
		if err != nil {
			return err
		}
		p.pushValue(commandIndex, option.id, value, option.repeated)
	}
	return nil
}

func (p *argumentParser) resolverDiagnostic(
	diagnostic *Diagnostic,
	commandIndex int,
	valueID string,
) *Diagnostic {
	if len(diagnostic.targets) == 0 {
		diagnostic.WithTarget(
			OptionTarget(valueID).WithCommandIDPath(p.commandIDPath(commandIndex)...),
		)
	}
	return diagnostic.
		withDefaultTargetPath(p.commandIDPath(commandIndex)).
		mapTargets(p.markSensitiveTarget).
		withCommand(p.commandPath, p.root.usageForPath(p.commandPath))
}

func (p *argumentParser) validateValues() error {
	for commandIndex, command := range p.commands {
		values := p.scopes[commandIndex].values
		if command.subcommandRequired && commandIndex+1 == len(p.commands) {
			return p.diag(CodeMissingSubcommand, fmt.Sprintf("command %q requires a subcommand", command.name))
		}
		for index := range command.options {
			option := &command.options[index]
			_, present := values[option.id]
			if option.required && !present {
				return p.diagTargets(
					CodeMissingRequired,
					fmt.Sprintf("required option %q is missing", optionDisplay(option)),
					OptionTarget(option.id).WithCommandIDPath(p.commandIDPath(commandIndex)...),
				)
			}
			if !present {
				continue
			}
			for _, required := range option.requires {
				if p.present(commandIndex, option.id, required.presence) &&
					!p.present(commandIndex, required.id, required.presence) {
					return p.diagTargets(
						CodeRequires,
						fmt.Sprintf("option %q requires %q", optionDisplay(option), required.id),
						OptionTarget(option.id).WithCommandIDPath(p.commandIDPath(commandIndex)...),
						OptionTarget(required.id).WithCommandIDPath(p.commandIDPath(commandIndex)...),
					)
				}
			}
			for _, conflict := range option.conflicts {
				if p.present(commandIndex, option.id, conflict.presence) &&
					p.present(commandIndex, conflict.id, conflict.presence) {
					return p.diagTargets(
						CodeConflicts,
						fmt.Sprintf("option %q conflicts with %q", optionDisplay(option), conflict.id),
						OptionTarget(option.id).WithCommandIDPath(p.commandIDPath(commandIndex)...),
						OptionTarget(conflict.id).WithCommandIDPath(p.commandIDPath(commandIndex)...),
					)
				}
			}
		}
		for index := range command.arguments {
			argument := &command.arguments[index]
			if _, present := values[argument.id]; argument.required && !present {
				return p.diagTargets(
					CodeMissingRequired,
					fmt.Sprintf("required argument %q is missing", argument.id),
					ArgumentTarget(argument.id).WithCommandIDPath(p.commandIDPath(commandIndex)...),
				)
			}
		}
		for index := range command.optionGroups {
			group := &command.optionGroups[index]
			count := 0
			for _, id := range group.options {
				if p.present(commandIndex, id, group.presence) {
					count++
				}
			}
			valid := false
			switch group.kind {
			case GroupAtMostOne:
				valid = count <= 1
			case GroupExactlyOne:
				valid = count == 1
			case GroupAtLeastOne:
				valid = count >= 1
			case GroupAllOrNone:
				valid = count == 0 || count == len(group.options)
			}
			if !valid {
				targets := make([]DiagnosticTarget, 0, len(group.options))
				for _, id := range group.options {
					targets = append(
						targets,
						OptionTarget(id).WithCommandIDPath(p.commandIDPath(commandIndex)...),
					)
				}
				return p.diagTargets(
					CodeOptionGroup,
					fmt.Sprintf("option group %q is not satisfied", group.id),
					targets...,
				)
			}
		}
	}
	return nil
}

func (p *argumentParser) present(scopeIndex int, id string, basis PresenceBasis) bool {
	value, ok := p.scopes[scopeIndex].values[id]
	if !ok {
		return false
	}
	return basis == PresenceResolved || value.supplied
}

func (p *argumentParser) runValidators(invocation *Invocation) error {
	defer func() {
		invocation.currentScope = len(invocation.scopes) - 1
	}()
	for commandIndex, command := range p.commands {
		invocation.currentScope = commandIndex
		for _, validator := range command.validators {
			diagnostic := validator(invocation)
			if diagnostic == nil {
				continue
			}
			return diagnostic.
				withDefaultTargetPath(invocation.commandIDPath[:commandIndex+1]).
				mapTargets(invocation.markSensitiveTarget).
				withCommand(p.commandPath, p.root.usageForPath(p.commandPath))
		}
	}
	return nil
}

func (p *argumentParser) parseValue(
	id string,
	parser ValueParser,
	raw string,
	origin ValueOrigin,
	sensitive bool,
	target DiagnosticTarget,
) (ParsedValue, error) {
	target = target.withValueOrigin(origin)
	typed, err := parser.Parse(raw)
	if err != nil {
		message := fmt.Sprintf("invalid value %s for '%s': %s", quoteValue(raw), id, err)
		if sensitive {
			message = fmt.Sprintf("invalid value %s for '%s'", RedactedValue, id)
		}
		return ParsedValue{}, p.diagTargets(
			CodeInvalidValue,
			message,
			target,
		)
	}
	return ParsedValue{raw: raw, origin: origin, typed: typed, sensitive: sensitive}, nil
}

func (p *argumentParser) pushValue(scopeIndex int, id string, parsed ParsedValue, repeated bool) {
	value := p.scopes[scopeIndex].values[id]
	value.kind = OptionValue
	value.repeated = repeated
	if parsed.Source() == SourceCommandLine {
		value.supplied = true
	}
	value.values = append(value.values, parsed)
	p.scopes[scopeIndex].values[id] = value
}

func (p *argumentParser) commandIDPath(index int) []string {
	path := make([]string, index+1)
	for commandIndex := range path {
		path[commandIndex] = p.commands[commandIndex].id
	}
	return path
}

func (p *argumentParser) diag(code DiagnosticCode, message string) *Diagnostic {
	return p.diagTargets(code, message)
}

func (p *argumentParser) diagTargets(
	code DiagnosticCode,
	message string,
	targets ...DiagnosticTarget,
) *Diagnostic {
	diagnostic := NewDiagnostic(code, message)
	for _, target := range targets {
		diagnostic.WithTarget(p.markSensitiveTarget(target))
	}
	return diagnostic.
		withDefaultTargetPath(p.currentCommandIDPath()).
		withCommand(p.commandPath, p.root.usageForPath(p.commandPath))
}

func (p *argumentParser) markSensitiveTarget(target DiagnosticTarget) DiagnosticTarget {
	scopeIndex := -1
	if len(target.commandIDPath) == 0 {
		scopeIndex = len(p.commands) - 1
	} else {
		for index := range p.commands {
			if equalPath(p.commandIDPath(index), target.commandIDPath) {
				scopeIndex = index
				break
			}
		}
	}
	if scopeIndex < 0 {
		return target
	}
	command := p.commands[scopeIndex]
	sensitive := false
	switch target.kind {
	case TargetOption:
		for index := range command.options {
			option := &command.options[index]
			if option.id == target.valueID {
				sensitive = option.sensitive
				break
			}
		}
	case TargetArgument:
		for index := range command.arguments {
			argument := &command.arguments[index]
			if argument.id == target.valueID {
				sensitive = argument.sensitive
				break
			}
		}
	case TargetResponseFile:
		return target
	}
	return target.withSensitive(sensitive)
}
