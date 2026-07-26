package cli

import (
	"errors"
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

// Invocation is one canonical parsed command path and its typed values
type Invocation struct {
	commandPath []string
	values      map[string]invocationValue
}

// CommandPath returns a copy of the canonical root-to-leaf path
func (i *Invocation) CommandPath() []string {
	return append([]string(nil), i.commandPath...)
}

// Contains reports whether an ID has a value
func (i *Invocation) Contains(id string) bool {
	_, ok := i.values[id]
	return ok
}

// Supplied reports whether an ID was present in argv
func (i *Invocation) Supplied(id string) bool {
	value, ok := i.values[id]
	return ok && value.supplied
}

// Flag returns Boolean presence when the ID denotes a flag
func (i *Invocation) Flag(id string) (bool, bool) {
	value, ok := i.values[id]
	if !ok || value.kind != OptionFlag {
		return false, false
	}
	return value.flag, true
}

// Count returns occurrences when the ID denotes a count option
func (i *Invocation) Count(id string) (uint64, bool) {
	value, ok := i.values[id]
	if !ok || value.kind != OptionCount {
		return 0, false
	}
	return value.count, true
}

// ParsedValues returns a copy of all values and sources for an ID
func (i *Invocation) ParsedValues(id string) []ParsedValue {
	value, ok := i.values[id]
	if !ok || value.kind != OptionValue {
		return nil
	}
	return append([]ParsedValue(nil), value.values...)
}

// RawValue returns the first raw value
func (i *Invocation) RawValue(id string) (string, bool) {
	values := i.ParsedValues(id)
	if len(values) == 0 {
		return "", false
	}
	return values[0].raw, true
}

// IsRepeated reports whether a value ID was declared as repeatable
func (i *Invocation) IsRepeated(id string) bool {
	value, ok := i.values[id]
	return ok && value.kind == OptionValue && value.repeated
}

// ValueIDs returns sorted IDs that have a value
func (i *Invocation) ValueIDs() []string {
	ids := make([]string, 0, len(i.values))
	for id := range i.values {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
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
	kind        ParseResultKind
	invocation  *Invocation
	commandPath []string
	version     string
}

// Kind returns the parse result category
func (r ParseResult) Kind() ParseResultKind { return r.kind }

// Invocation returns the validated invocation when Kind is ParseInvocation
func (r ParseResult) Invocation() *Invocation { return r.invocation }

// CommandPath returns the help target when Kind is ParseHelp
func (r ParseResult) CommandPath() []string {
	return append([]string(nil), r.commandPath...)
}

// Version returns the configured version when Kind is ParseVersion
func (r ParseResult) Version() string { return r.version }

// Parse parses arguments after the program name with an empty environment
func (c *Command) Parse(arguments []string) (ParseResult, error) {
	return c.ParseWithEnvironment(arguments, nil)
}

// ParseWithEnvironment parses argv and injected environment values
func (c *Command) ParseWithEnvironment(arguments []string, environment map[string]string) (ParseResult, error) {
	if err := c.Validate(); err != nil {
		return ParseResult{}, err
	}
	copyEnvironment := make(map[string]string, len(environment))
	for key, value := range environment {
		copyEnvironment[key] = value
	}
	parser := &argumentParser{
		root:           c,
		arguments:      append([]string(nil), arguments...),
		environment:    copyEnvironment,
		commands:       []*Command{c},
		commandPath:    []string{c.name},
		values:         map[string]invocationValue{},
		optionsEnabled: true,
	}
	return parser.parse()
}

type argumentParser struct {
	root              *Command
	arguments         []string
	environment       map[string]string
	index             int
	commands          []*Command
	commandPath       []string
	values            map[string]invocationValue
	positionalIndex   int
	positionalStarted bool
	optionsEnabled    bool
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
	if err := p.resolveFallbacks(); err != nil {
		return ParseResult{}, err
	}
	if err := p.validateValues(); err != nil {
		return ParseResult{}, err
	}
	invocation := &Invocation{
		commandPath: append([]string(nil), p.commandPath...),
		values:      p.values,
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

func (p *argumentParser) parseHelpCommand() (ParseResult, bool, error) {
	if len(p.arguments) == 0 || p.arguments[0] != "help" {
		return ParseResult{}, false, nil
	}
	command := p.root
	path := []string{p.root.name}
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
	}
	return ParseResult{kind: ParseHelp, commandPath: path}, true, nil
}

func (p *argumentParser) parseLong(argument string) (ParseResult, bool, error) {
	body := argument[2:]
	name, attached, hasAttached := strings.Cut(body, "=")
	if name == "help" {
		if hasAttached {
			return ParseResult{}, false, p.diag(CodeUnexpectedOptionValue, "option '--help' does not take a value")
		}
		return ParseResult{kind: ParseHelp, commandPath: append([]string(nil), p.commandPath...)}, true, nil
	}
	if name == "version" && p.root.version != "" {
		if hasAttached {
			return ParseResult{}, false, p.diag(CodeUnexpectedOptionValue, "option '--version' does not take a value")
		}
		return ParseResult{kind: ParseVersion, version: p.root.version}, true, nil
	}
	var option *OptionSpec
	for index := range p.active().options {
		candidate := &p.active().options[index]
		if candidate.long == name {
			option = candidate
			break
		}
	}
	if option == nil {
		return ParseResult{}, false, p.diag(CodeUnknownOption, "unknown option "+quoteValue(argument))
	}
	p.index++
	var value *string
	if hasAttached {
		value = &attached
	}
	if err := p.applyOption(option, value); err != nil {
		return ParseResult{}, false, err
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
			return ParseResult{kind: ParseHelp, commandPath: append([]string(nil), p.commandPath...)}, true, nil
		}
		if short == 'V' && p.root.version != "" {
			return ParseResult{kind: ParseVersion, version: p.root.version}, true, nil
		}
		var option *OptionSpec
		for index := range p.active().options {
			candidate := &p.active().options[index]
			if candidate.short == short {
				option = candidate
				break
			}
		}
		if option == nil {
			return ParseResult{}, false, p.diag(CodeUnknownOption, fmt.Sprintf("unknown option '-%c'", short))
		}
		if option.kind == OptionValue {
			var attached *string
			if offset+1 < len(argument) {
				value := argument[offset+1:]
				attached = &value
			}
			if err := p.applyOption(option, attached); err != nil {
				return ParseResult{}, false, err
			}
			return ParseResult{}, false, nil
		}
		if err := p.applyOption(option, nil); err != nil {
			return ParseResult{}, false, err
		}
	}
	return ParseResult{}, false, nil
}

func (p *argumentParser) applyOption(option *OptionSpec, attached *string) error {
	switch option.kind {
	case OptionFlag:
		if attached != nil {
			return p.diag(CodeUnexpectedOptionValue, fmt.Sprintf("option %q does not take a value", optionDisplay(option)))
		}
		if _, duplicate := p.values[option.id]; duplicate {
			return p.diag(CodeDuplicateOption, fmt.Sprintf("option %q was provided more than once", optionDisplay(option)))
		}
		p.values[option.id] = invocationValue{kind: OptionFlag, flag: true, supplied: true}
	case OptionCount:
		if attached != nil {
			return p.diag(CodeUnexpectedOptionValue, fmt.Sprintf("option %q does not take a value", optionDisplay(option)))
		}
		value := p.values[option.id]
		value.kind = OptionCount
		value.supplied = true
		if value.count != math.MaxUint64 {
			value.count++
		}
		p.values[option.id] = value
	case OptionValue:
		var raw string
		if attached != nil {
			raw = *attached
		} else {
			if p.index >= len(p.arguments) {
				return p.diag(CodeMissingOptionValue, fmt.Sprintf("option %q requires a value", optionDisplay(option)))
			}
			raw = p.arguments[p.index]
			p.index++
		}
		if _, duplicate := p.values[option.id]; duplicate && !option.repeated {
			return p.diag(CodeDuplicateOption, fmt.Sprintf("option %q was provided more than once", optionDisplay(option)))
		}
		value, err := p.parseValue(option.id, option.parser, raw, SourceCommandLine)
		if err != nil {
			return err
		}
		p.pushValue(option.id, value, option.repeated)
	}
	return nil
}

func (p *argumentParser) selectSubcommand(argument string) bool {
	for _, command := range p.active().subcommands {
		if command.name == argument {
			p.commands = append(p.commands, command)
			p.commandPath = append(p.commandPath, command.name)
			p.positionalIndex = 0
			p.positionalStarted = false
			return true
		}
		for _, alias := range command.aliases {
			if alias == argument {
				p.commands = append(p.commands, command)
				p.commandPath = append(p.commandPath, command.name)
				p.positionalIndex = 0
				p.positionalStarted = false
				return true
			}
		}
	}
	return false
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
	value, err := p.parseValue(argument.id, argument.parser, raw, SourceCommandLine)
	if err != nil {
		return err
	}
	p.pushValue(argument.id, value, argument.repeated)
	if !argument.repeated {
		p.positionalIndex++
	}
	return nil
}

func (p *argumentParser) resolveFallbacks() error {
	for _, command := range p.commands {
		for index := range command.options {
			option := &command.options[index]
			if option.kind != OptionValue {
				continue
			}
			if _, present := p.values[option.id]; present {
				continue
			}
			raw, source, present := "", SourceDefault, false
			if option.environment != "" {
				raw, present = p.environment[option.environment]
				if present {
					source = SourceEnvironment
				}
			}
			if !present && option.defaultSet {
				raw, source, present = option.defaultVal, SourceDefault, true
			}
			if !present {
				continue
			}
			value, err := p.parseValue(option.id, option.parser, raw, source)
			if err != nil {
				return err
			}
			p.pushValue(option.id, value, option.repeated)
		}
	}
	return nil
}

func (p *argumentParser) validateValues() error {
	for commandIndex, command := range p.commands {
		if command.subcommandRequired && commandIndex+1 == len(p.commands) {
			return p.diag(CodeMissingSubcommand, fmt.Sprintf("command %q requires a subcommand", command.name))
		}
		for index := range command.options {
			option := &command.options[index]
			_, present := p.values[option.id]
			if option.required && !present {
				return p.diag(CodeMissingRequired, fmt.Sprintf("required option %q is missing", optionDisplay(option)))
			}
			if !present {
				continue
			}
			for _, required := range option.requires {
				if p.present(option.id, required.presence) && !p.present(required.id, required.presence) {
					return p.diag(CodeRequires, fmt.Sprintf("option %q requires %q", optionDisplay(option), required.id))
				}
			}
			for _, conflict := range option.conflicts {
				if p.present(option.id, conflict.presence) && p.present(conflict.id, conflict.presence) {
					return p.diag(CodeConflicts, fmt.Sprintf("option %q conflicts with %q", optionDisplay(option), conflict.id))
				}
			}
		}
		for index := range command.arguments {
			argument := &command.arguments[index]
			if _, present := p.values[argument.id]; argument.required && !present {
				return p.diag(CodeMissingRequired, fmt.Sprintf("required argument %q is missing", argument.id))
			}
		}
		for index := range command.optionGroups {
			group := &command.optionGroups[index]
			count := 0
			for _, id := range group.options {
				if p.present(id, group.presence) {
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
				return p.diag(
					CodeOptionGroup,
					fmt.Sprintf("option group %q is not satisfied", group.id),
				)
			}
		}
	}
	return nil
}

func (p *argumentParser) present(id string, basis PresenceBasis) bool {
	value, ok := p.values[id]
	if !ok {
		return false
	}
	return basis == PresenceResolved || value.supplied
}

func (p *argumentParser) runValidators(invocation *Invocation) error {
	for _, command := range p.commands {
		for _, validator := range command.validators {
			err := validator(invocation)
			if err == nil {
				continue
			}
			var diagnostic *Diagnostic
			if !errors.As(err, &diagnostic) {
				diagnostic = NewDiagnostic(CodeValidation, err.Error())
			}
			return diagnostic.withCommand(p.commandPath, p.root.usageForPath(p.commandPath))
		}
	}
	return nil
}

func (p *argumentParser) parseValue(id string, parser ValueParser, raw string, source ValueSource) (ParsedValue, error) {
	typed, err := parser.Parse(raw)
	if err != nil {
		return ParsedValue{}, p.diag(CodeInvalidValue, fmt.Sprintf("invalid value %s for %q: %s", quoteValue(raw), id, err))
	}
	return ParsedValue{raw: raw, source: source, typed: typed}, nil
}

func (p *argumentParser) pushValue(id string, parsed ParsedValue, repeated bool) {
	value := p.values[id]
	value.kind = OptionValue
	value.repeated = repeated
	if parsed.Source() == SourceCommandLine {
		value.supplied = true
	}
	value.values = append(value.values, parsed)
	p.values[id] = value
}

func (p *argumentParser) diag(code DiagnosticCode, message string) error {
	return NewDiagnostic(code, message).withCommand(p.commandPath, p.root.usageForPath(p.commandPath))
}
