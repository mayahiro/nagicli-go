package cli

import (
	"fmt"
	"strings"
	"unicode/utf8"

	nagitext "github.com/mayahiro/nagi-go/text"
)

// OptionKind controls option storage and parsing
type OptionKind uint8

const (
	// OptionFlag stores Boolean presence
	OptionFlag OptionKind = iota
	// OptionCount stores an occurrence count
	OptionCount
	// OptionValue stores parser-produced values
	OptionValue
)

// OptionSpec defines one named command option
type OptionSpec struct {
	id          string
	long        string
	short       byte
	kind        OptionKind
	parser      ValueParser
	help        string
	required    bool
	repeated    bool
	environment string
	defaultSet  bool
	defaultVal  string
	requires    []string
	conflicts   []string
}

// Flag constructs a Boolean option
func Flag(id string) *OptionSpec {
	return &OptionSpec{id: id, kind: OptionFlag, parser: RawParser()}
}

// Count constructs an occurrence counter
func Count(id string) *OptionSpec {
	return &OptionSpec{id: id, kind: OptionCount, parser: RawParser()}
}

// ValueOption constructs a raw platform-value option
func ValueOption(id string) *OptionSpec {
	return &OptionSpec{id: id, kind: OptionValue, parser: RawParser()}
}

// Long sets the long option name without leading hyphens
func (o *OptionSpec) Long(name string) *OptionSpec { o.long = name; return o }

// Short sets the one-byte short option name
func (o *OptionSpec) Short(name byte) *OptionSpec { o.short = name; return o }

// Help sets the option description
func (o *OptionSpec) Help(help string) *OptionSpec { o.help = help; return o }

// Required requires this option after source resolution
func (o *OptionSpec) Required() *OptionSpec { o.required = true; return o }

// Repeated allows a Value option to appear multiple times
func (o *OptionSpec) Repeated() *OptionSpec { o.repeated = true; return o }

// Parser sets the typed Value Parser
func (o *OptionSpec) Parser(parser ValueParser) *OptionSpec { o.parser = parser; return o }

// Environment sets an injected environment fallback
func (o *OptionSpec) Environment(name string) *OptionSpec { o.environment = name; return o }

// Default sets a command-definition fallback
func (o *OptionSpec) Default(value string) *OptionSpec {
	o.defaultSet = true
	o.defaultVal = value
	return o
}

// Requires adds an option requirement
func (o *OptionSpec) Requires(id string) *OptionSpec {
	o.requires = append(o.requires, id)
	return o
}

// Conflicts adds a conflicting option
func (o *OptionSpec) Conflicts(id string) *OptionSpec {
	o.conflicts = append(o.conflicts, id)
	return o
}

// ID returns the stable value identifier
func (o *OptionSpec) ID() string { return o.id }

// Kind returns the option kind
func (o *OptionSpec) Kind() OptionKind { return o.kind }

// Argument defines one positional command value
type Argument struct {
	id       string
	parser   ValueParser
	help     string
	required bool
	repeated bool
}

// Positional constructs a raw platform-value positional argument
func Positional(id string) *Argument {
	return &Argument{id: id, parser: RawParser()}
}

// Parser sets the typed Value Parser
func (a *Argument) Parser(parser ValueParser) *Argument { a.parser = parser; return a }

// Help sets the positional description
func (a *Argument) Help(help string) *Argument { a.help = help; return a }

// Required requires this positional argument
func (a *Argument) Required() *Argument { a.required = true; return a }

// Repeated allows this final positional to consume remaining values
func (a *Argument) Repeated() *Argument { a.repeated = true; return a }

// ID returns the stable value identifier
func (a *Argument) ID() string { return a.id }

// Command is one validated node in a Command Graph
type Command struct {
	id                 string
	name               string
	aliases            []string
	about              string
	version            string
	options            []OptionSpec
	arguments          []Argument
	subcommands        []*Command
	subcommandRequired bool
	handler            Handler
}

// NewCommand constructs a command whose stable ID matches its name
func NewCommand(name string) *Command {
	return &Command{id: name, name: name}
}

// ID sets the stable identity independently of the command name
func (c *Command) ID(id string) *Command { c.id = id; return c }

// Alias adds one child-command spelling
func (c *Command) Alias(alias string) *Command {
	c.aliases = append(c.aliases, alias)
	return c
}

// About sets the short command description
func (c *Command) About(about string) *Command { c.about = about; return c }

// Version sets the root version used by the built-in version action
func (c *Command) Version(version string) *Command { c.version = version; return c }

// Option appends an option in help-definition order
func (c *Command) Option(option *OptionSpec) *Command {
	copy := *option
	copy.requires = append([]string(nil), option.requires...)
	copy.conflicts = append([]string(nil), option.conflicts...)
	c.options = append(c.options, copy)
	return c
}

// Argument appends a positional in consumption order
func (c *Command) Argument(argument *Argument) *Command {
	c.arguments = append(c.arguments, *argument)
	return c
}

// Subcommand appends a child command in help-definition order
func (c *Command) Subcommand(command *Command) *Command {
	c.subcommands = append(c.subcommands, command)
	return c
}

// RequireSubcommand requires one child command to be selected
func (c *Command) RequireSubcommand() *Command {
	c.subcommandRequired = true
	return c
}

// Handle sets the language-native command handler
func (c *Command) Handle(handler Handler) *Command { c.handler = handler; return c }

// StableID returns the command identity
func (c *Command) StableID() string { return c.id }

// Name returns the canonical command name
func (c *Command) Name() string { return c.name }

// Description returns the short description
func (c *Command) Description() string { return c.about }

// Validate checks the entire Command Graph before argv is consumed
func (c *Command) Validate() error {
	return validateCommand(c, true, map[string]struct{}{})
}

// RenderHelp renders deterministic help for a canonical command path
func (c *Command) RenderHelp(path []string) (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	command := c.commandAtPath(path)
	if command == nil {
		return "", NewDiagnostic(CodeInvalidSpecification, "help path does not identify a command")
	}
	var output strings.Builder
	if command.about != "" {
		output.WriteString(command.about)
		output.WriteString("\n\n")
	}
	output.WriteString("Usage:\n  ")
	output.WriteString(usageLine(command, path))
	output.WriteByte('\n')
	if len(command.subcommands) > 0 && !command.subcommandRequired {
		fmt.Fprintf(&output, "  %s [OPTIONS] <COMMAND>\n", strings.Join(path, " "))
	}
	if len(command.subcommands) > 0 {
		output.WriteString("\nCommands:\n")
		entries := make([]helpEntry, 0, len(command.subcommands))
		for _, child := range command.subcommands {
			entries = append(entries, helpEntry{child.name, child.about})
		}
		renderEntries(&output, entries)
	}
	if len(command.arguments) > 0 {
		output.WriteString("\nArguments:\n")
		entries := make([]helpEntry, 0, len(command.arguments))
		for i := range command.arguments {
			argument := &command.arguments[i]
			entries = append(entries, helpEntry{argumentLabel(argument), argument.help})
		}
		renderEntries(&output, entries)
	}
	output.WriteString("\nOptions:\n")
	entries := make([]helpEntry, 0, len(command.options)+2)
	for i := range command.options {
		option := &command.options[i]
		entries = append(entries, helpEntry{optionLabel(option), optionDescription(option)})
	}
	entries = append(entries, helpEntry{"-h, --help", "Print help"})
	if c.version != "" {
		entries = append(entries, helpEntry{"-V, --version", "Print version"})
	}
	renderEntries(&output, entries)
	return output.String(), nil
}

func validateCommand(command *Command, root bool, pathIDs map[string]struct{}) error {
	if !validID(command.id) {
		return invalidSpec("invalid command ID %q", command.id)
	}
	if !validName(command.name) || reservedLong(command.name) {
		return invalidSpec("invalid or reserved command name %q", command.name)
	}
	if !utf8.ValidString(command.about) {
		return invalidSpec("command %q has an invalid UTF-8 description", command.name)
	}
	if !root && command.version != "" {
		return invalidSpec("child command %q declares a version", command.name)
	}
	for _, alias := range command.aliases {
		if !validName(alias) || reservedLong(alias) {
			return invalidSpec("invalid or reserved command alias %q", alias)
		}
	}
	for index, argument := range command.arguments {
		if argument.repeated && index+1 != len(command.arguments) {
			return invalidSpec("command %q has a non-final repeated positional", command.name)
		}
	}

	ids := cloneSet(pathIDs)
	localIDs := map[string]struct{}{}
	longs := map[string]struct{}{}
	shorts := map[byte]struct{}{}
	for i := range command.options {
		option := &command.options[i]
		if !validID(option.id) || has(ids, option.id) {
			return invalidSpec("duplicate or invalid value ID %q", option.id)
		}
		ids[option.id] = struct{}{}
		localIDs[option.id] = struct{}{}
		if option.long == "" && option.short == 0 {
			return invalidSpec("option %q has no spelling", option.id)
		}
		if option.long != "" {
			if !validName(option.long) || reservedLong(option.long) || has(longs, option.long) {
				return invalidSpec("duplicate, invalid, or reserved long option %q", option.long)
			}
			longs[option.long] = struct{}{}
		}
		if option.short != 0 {
			if !asciiAlphanumeric(option.short) || reservedShort(option.short) {
				return invalidSpec("invalid or reserved short option %q", option.short)
			}
			if _, duplicate := shorts[option.short]; duplicate {
				return invalidSpec("duplicate short option %q", option.short)
			}
			shorts[option.short] = struct{}{}
		}
		if option.parser == nil {
			return invalidSpec("option %q has no Value Parser", option.id)
		}
		if option.kind != OptionValue && (option.repeated || option.environment != "" || option.defaultSet) {
			return invalidSpec("non-value option %q has value-only configuration", option.id)
		}
	}
	for i := range command.arguments {
		argument := &command.arguments[i]
		if !validID(argument.id) || has(ids, argument.id) {
			return invalidSpec("duplicate or invalid value ID %q", argument.id)
		}
		ids[argument.id] = struct{}{}
		if argument.parser == nil {
			return invalidSpec("argument %q has no Value Parser", argument.id)
		}
	}
	for i := range command.options {
		option := &command.options[i]
		for _, relation := range append(append([]string(nil), option.requires...), option.conflicts...) {
			if !has(localIDs, relation) {
				return invalidSpec("option %q references unknown option %q", option.id, relation)
			}
		}
	}
	childSpellings := map[string]struct{}{}
	for _, child := range command.subcommands {
		spellings := append([]string{child.name}, child.aliases...)
		for _, spelling := range spellings {
			if has(childSpellings, spelling) {
				return invalidSpec("command %q has duplicate child spelling %q", command.name, spelling)
			}
			childSpellings[spelling] = struct{}{}
		}
		if err := validateCommand(child, false, cloneSet(ids)); err != nil {
			return err
		}
	}
	return nil
}

func (c *Command) commandAtPath(path []string) *Command {
	if len(path) == 0 || path[0] != c.name {
		return nil
	}
	command := c
	for _, name := range path[1:] {
		var child *Command
		for _, candidate := range command.subcommands {
			if candidate.name == name {
				child = candidate
				break
			}
		}
		if child == nil {
			return nil
		}
		command = child
	}
	return command
}

func (c *Command) usageForPath(path []string) string {
	return usageLine(c.commandAtPath(path), path)
}

func usageLine(command *Command, path []string) string {
	usage := strings.Join(path, " ") + " [OPTIONS]"
	for i := range command.arguments {
		usage += " " + argumentLabel(&command.arguments[i])
	}
	if command.subcommandRequired {
		usage += " <COMMAND>"
	}
	return usage
}

func argumentLabel(argument *Argument) string {
	name := strings.ToUpper(strings.ReplaceAll(argument.id, "_", "-"))
	label := "[" + name + "]"
	if argument.required {
		label = "<" + name + ">"
	}
	if argument.repeated {
		label += "..."
	}
	return label
}

func optionLabel(option *OptionSpec) string {
	var label string
	switch {
	case option.short != 0 && option.long != "":
		label = fmt.Sprintf("-%c, --%s", option.short, option.long)
	case option.short != 0:
		label = fmt.Sprintf("-%c", option.short)
	default:
		label = "    --" + option.long
	}
	if option.kind == OptionValue {
		label += " <" + option.parser.Metavar() + ">"
		if option.repeated {
			label += "..."
		}
	}
	return label
}

func optionDescription(option *OptionSpec) string {
	description := option.help
	if option.required {
		description = appendNote(description, "required")
	}
	if option.environment != "" {
		description = appendNote(description, "env: "+option.environment)
	}
	if option.defaultSet {
		description = appendNote(description, "default: "+displayValue(option.defaultVal))
	}
	if values := option.parser.PossibleValues(); len(values) > 0 {
		description = appendNote(description, "possible: "+joinComma(values))
	}
	return description
}

func appendNote(description, note string) string {
	if description != "" {
		description += " "
	}
	return description + "[" + note + "]"
}

type helpEntry struct {
	label       string
	description string
}

func renderEntries(output *strings.Builder, entries []helpEntry) {
	width := 0
	for _, entry := range entries {
		if candidate := nagitext.Width(entry.label, nagitext.ModernWidth()); candidate > width {
			width = candidate
		}
	}
	for _, entry := range entries {
		output.WriteString("  ")
		output.WriteString(entry.label)
		labelWidth := nagitext.Width(entry.label, nagitext.ModernWidth())
		output.WriteString(strings.Repeat(" ", width-labelWidth+2))
		output.WriteString(entry.description)
		output.WriteByte('\n')
	}
}

func optionDisplay(option *OptionSpec) string {
	if option.long != "" {
		return "--" + option.long
	}
	if option.short != 0 {
		return fmt.Sprintf("-%c", option.short)
	}
	return option.id
}

func invalidSpec(format string, arguments ...any) error {
	return NewDiagnostic(CodeInvalidSpecification, fmt.Sprintf(format, arguments...))
}

func validID(value string) bool {
	if value == "" || !asciiLetter(value[0]) {
		return false
	}
	for index := 1; index < len(value); index++ {
		byteValue := value[index]
		if !asciiAlphanumeric(byteValue) && byteValue != '-' && byteValue != '_' {
			return false
		}
	}
	return true
}

func validName(value string) bool {
	if value == "" || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for index := 1; index < len(value); index++ {
		byteValue := value[index]
		if (byteValue < 'a' || byteValue > 'z') && (byteValue < '0' || byteValue > '9') && byteValue != '-' {
			return false
		}
	}
	return true
}

func asciiLetter(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func asciiAlphanumeric(value byte) bool {
	return asciiLetter(value) || value >= '0' && value <= '9'
}

func reservedLong(value string) bool { return value == "help" || value == "version" }
func reservedShort(value byte) bool  { return value == 'h' || value == 'V' }

func cloneSet(source map[string]struct{}) map[string]struct{} {
	clone := make(map[string]struct{}, len(source))
	for value := range source {
		clone[value] = struct{}{}
	}
	return clone
}

func has(set map[string]struct{}, value string) bool {
	_, ok := set[value]
	return ok
}
