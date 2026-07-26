package cli

import (
	"fmt"
	"strings"
	"unicode/utf8"
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

// PresenceBasis selects resolved or command-line presence for validation
type PresenceBasis uint8

const (
	// PresenceResolved counts values from command line, environment, or default
	PresenceResolved PresenceBasis = iota
	// PresenceCommandLine counts only values supplied in argv
	PresenceCommandLine
)

// OptionGroupKind controls how many group members may be present
type OptionGroupKind uint8

const (
	// GroupAtMostOne accepts zero or one present option
	GroupAtMostOne OptionGroupKind = iota
	// GroupExactlyOne accepts exactly one present option
	GroupExactlyOne
	// GroupAtLeastOne accepts one or more present options
	GroupAtLeastOne
	// GroupAllOrNone accepts zero options or every option
	GroupAllOrNone
)

type optionRelation struct {
	id       string
	presence PresenceBasis
}

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
	requires    []optionRelation
	conflicts   []optionRelation
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
	return o.RequiresWithPresence(id, PresenceResolved)
}

// RequiresSupplied requires another command-line-supplied option
func (o *OptionSpec) RequiresSupplied(id string) *OptionSpec {
	return o.RequiresWithPresence(id, PresenceCommandLine)
}

// RequiresWithPresence adds an option requirement with an explicit presence basis
func (o *OptionSpec) RequiresWithPresence(id string, presence PresenceBasis) *OptionSpec {
	o.requires = append(o.requires, optionRelation{id: id, presence: presence})
	return o
}

// Conflicts adds a conflicting option
func (o *OptionSpec) Conflicts(id string) *OptionSpec {
	return o.ConflictsWithPresence(id, PresenceResolved)
}

// ConflictsSupplied conflicts with another command-line-supplied option
func (o *OptionSpec) ConflictsSupplied(id string) *OptionSpec {
	return o.ConflictsWithPresence(id, PresenceCommandLine)
}

// ConflictsWithPresence adds a conflict with an explicit presence basis
func (o *OptionSpec) ConflictsWithPresence(id string, presence PresenceBasis) *OptionSpec {
	o.conflicts = append(o.conflicts, optionRelation{id: id, presence: presence})
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

// OptionGroup applies one cardinality rule to local command options
type OptionGroup struct {
	id       string
	kind     OptionGroupKind
	presence PresenceBasis
	options  []string
}

// AtMostOne constructs an optional mutually exclusive option group
func AtMostOne(id string, optionIDs ...string) *OptionGroup {
	return newOptionGroup(id, GroupAtMostOne, optionIDs)
}

// ExactlyOne constructs a required mutually exclusive option group
func ExactlyOne(id string, optionIDs ...string) *OptionGroup {
	return newOptionGroup(id, GroupExactlyOne, optionIDs)
}

// AtLeastOne constructs a group requiring one or more options
func AtLeastOne(id string, optionIDs ...string) *OptionGroup {
	return newOptionGroup(id, GroupAtLeastOne, optionIDs)
}

// AllOrNone constructs a group whose options must occur together
func AllOrNone(id string, optionIDs ...string) *OptionGroup {
	return newOptionGroup(id, GroupAllOrNone, optionIDs)
}

func newOptionGroup(id string, kind OptionGroupKind, optionIDs []string) *OptionGroup {
	return &OptionGroup{
		id:       id,
		kind:     kind,
		presence: PresenceCommandLine,
		options:  append([]string(nil), optionIDs...),
	}
}

// Presence changes how this group determines whether an option is present
func (g *OptionGroup) Presence(presence PresenceBasis) *OptionGroup {
	g.presence = presence
	return g
}

// ID returns the stable group identifier
func (g *OptionGroup) ID() string { return g.id }

// Kind returns the group cardinality rule
func (g *OptionGroup) Kind() OptionGroupKind { return g.kind }

// PresenceBasis returns the group's presence basis
func (g *OptionGroup) PresenceBasis() PresenceBasis { return g.presence }

// OptionIDs returns a copy of group members in definition order
func (g *OptionGroup) OptionIDs() []string {
	return append([]string(nil), g.options...)
}

// InvocationValidator returns a structured application validation failure
//
// A nil result accepts the Invocation. The Invocation current scope is the
// Command that declared the validator
type InvocationValidator func(invocation *Invocation) *Diagnostic

// SubcommandUsageMode selects how parent Help presents subcommand syntax
type SubcommandUsageMode uint8

const (
	// SubcommandUsageAuto generates one generic optional-subcommand usage line
	SubcommandUsageAuto SubcommandUsageMode = iota
	// SubcommandUsageHidden omits generated optional-subcommand usage
	SubcommandUsageHidden
	// SubcommandUsageExpanded expands each immediate child's direct variants without recursion
	SubcommandUsageExpanded
)

// Command is one validated node in a Command Graph
type Command struct {
	id                 string
	name               string
	aliases            []string
	about              string
	version            string
	options            []OptionSpec
	arguments          []Argument
	optionGroups       []OptionGroup
	subcommands        []*Command
	subcommandRequired bool
	subcommandUsage    SubcommandUsageMode
	usageVariants      []usageVariant
	examples           []HelpExample
	notes              []string
	links              []HelpLink
	helpSections       []HelpSection
	validators         []InvocationValidator
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
	copy.requires = append([]optionRelation(nil), option.requires...)
	copy.conflicts = append([]optionRelation(nil), option.conflicts...)
	c.options = append(c.options, copy)
	return c
}

// Argument appends a positional in consumption order
func (c *Command) Argument(argument *Argument) *Command {
	c.arguments = append(c.arguments, *argument)
	return c
}

// OptionGroup appends one portable option-group constraint
func (c *Command) OptionGroup(group *OptionGroup) *Command {
	copy := *group
	copy.options = append([]string(nil), group.options...)
	c.optionGroups = append(c.optionGroups, copy)
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

// SubcommandUsage selects generic, hidden, or expanded child usage in Help
//
// It changes Help presentation only and does not change parsing, validation,
// Diagnostics, or Invocation values
func (c *Command) SubcommandUsage(mode SubcommandUsageMode) *Command {
	c.subcommandUsage = mode
	return c
}

// UsageVariant appends one Help-only invocation syntax with a stable ID
//
// The syntax is a non-empty suffix relative to the canonical command path
// and does not change argv parsing, Invocation validation, Diagnostic usage,
// or Invocation values
func (c *Command) UsageVariant(id, syntax string) *Command {
	c.usageVariants = append(c.usageVariants, usageVariant{id: id, syntax: syntax})
	return c
}

// Example appends one named command-line example
func (c *Command) Example(name, invocation string) *Command {
	c.examples = append(c.examples, HelpExample{Name: name, Invocation: invocation})
	return c
}

// Note appends one structured Help note
func (c *Command) Note(note string) *Command {
	c.notes = append(c.notes, note)
	return c
}

// Link appends one labeled documentation link
func (c *Command) Link(label, url string) *Command {
	c.links = append(c.links, HelpLink{Label: label, URL: url})
	return c
}

// HelpSection appends one application-defined structured Help section
func (c *Command) HelpSection(section *HelpSection) *Command {
	c.helpSections = append(c.helpSections, cloneHelpSection(*section))
	return c
}

// Validator appends one language-native typed Invocation validator
func (c *Command) Validator(validator InvocationValidator) *Command {
	c.validators = append(c.validators, validator)
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
	return validateCommand(c, true)
}

func validateCommand(command *Command, root bool) error {
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

	ids := map[string]struct{}{}
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
		for _, relation := range append(append([]optionRelation(nil), option.requires...), option.conflicts...) {
			if !validPresence(relation.presence) {
				return invalidSpec("option %q has an invalid relation presence", option.id)
			}
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
		for _, relation := range append(append([]optionRelation(nil), option.requires...), option.conflicts...) {
			if !has(localIDs, relation.id) {
				return invalidSpec("option %q references unknown option %q", option.id, relation.id)
			}
		}
	}
	groupIDs := map[string]struct{}{}
	for i := range command.optionGroups {
		group := &command.optionGroups[i]
		if !validID(group.id) || has(groupIDs, group.id) {
			return invalidSpec("duplicate or invalid option group ID %q", group.id)
		}
		groupIDs[group.id] = struct{}{}
		if !validGroupKind(group.kind) || !validPresence(group.presence) {
			return invalidSpec("option group %q has an invalid kind or presence", group.id)
		}
		if len(group.options) < 2 {
			return invalidSpec("option group %q has fewer than two options", group.id)
		}
		members := map[string]struct{}{}
		for _, id := range group.options {
			if !has(localIDs, id) || has(members, id) {
				return invalidSpec("option group %q references duplicate or unknown option %q", group.id, id)
			}
			members[id] = struct{}{}
		}
	}
	if err := validateHelp(command); err != nil {
		return err
	}
	for _, validator := range command.validators {
		if validator == nil {
			return invalidSpec("command %q has a nil Invocation validator", command.name)
		}
	}
	childSpellings := map[string]struct{}{}
	childIDs := map[string]struct{}{}
	for _, child := range command.subcommands {
		if has(childIDs, child.id) {
			return invalidSpec("command %q has duplicate child ID %q", command.name, child.id)
		}
		childIDs[child.id] = struct{}{}
		spellings := append([]string{child.name}, child.aliases...)
		for _, spelling := range spellings {
			if has(childSpellings, spelling) {
				return invalidSpec("command %q has duplicate child spelling %q", command.name, spelling)
			}
			childSpellings[spelling] = struct{}{}
		}
		if err := validateCommand(child, false); err != nil {
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

func (c *Command) commandIDPathAtPath(path []string) []string {
	if len(path) == 0 || path[0] != c.name {
		return nil
	}
	command := c
	ids := []string{c.id}
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
		ids = append(ids, command.id)
	}
	return ids
}

func (c *Command) usageForPath(path []string) string {
	return usageLine(c.commandAtPath(path), path)
}

func (c *Command) optionByID(id string) *OptionSpec {
	for index := range c.options {
		if c.options[index].id == id {
			return &c.options[index]
		}
	}
	return nil
}

func usageLine(command *Command, path []string) string {
	return usageCommandLine(path, generatedUsageSyntax(command))
}

func generatedUsageSyntax(command *Command) string {
	syntax := "[OPTIONS]"
	for i := range command.arguments {
		syntax += " " + argumentLabel(&command.arguments[i])
	}
	if command.subcommandRequired {
		syntax += " <COMMAND>"
	}
	return syntax
}

func usageCommandLine(path []string, syntax string) string {
	return strings.Join(path, " ") + " " + syntax
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

func optionDisplay(option *OptionSpec) string {
	if option == nil {
		return ""
	}
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

func validPresence(value PresenceBasis) bool {
	return value == PresenceResolved || value == PresenceCommandLine
}

func validGroupKind(value OptionGroupKind) bool {
	return value >= GroupAtMostOne && value <= GroupAllOrNone
}

func validateHelp(command *Command) error {
	if command.subcommandUsage > SubcommandUsageExpanded {
		return invalidSpec(
			"command %q has an invalid subcommand usage mode",
			command.name,
		)
	}
	if len(command.usageVariants) > 0 && command.subcommandRequired {
		return invalidSpec(
			"command %q declares Help usage variants while requiring a subcommand",
			command.name,
		)
	}
	usageIDs := map[string]struct{}{}
	for _, variant := range command.usageVariants {
		if !validID(variant.id) || has(usageIDs, variant.id) || !validUsageSyntax(variant.syntax) {
			return invalidSpec("command %q has an invalid Help usage variant %q", command.name, variant.id)
		}
		if command.subcommandUsage == SubcommandUsageAuto &&
			len(command.subcommands) > 0 &&
			variant.id == "subcommand" {
			return invalidSpec(
				"command %q Help usage variant %q conflicts with a generated variant",
				command.name,
				variant.id,
			)
		}
		usageIDs[variant.id] = struct{}{}
	}
	for _, example := range command.examples {
		if example.Name == "" || example.Invocation == "" ||
			!utf8.ValidString(example.Name) || !utf8.ValidString(example.Invocation) {
			return invalidSpec("command %q has an invalid Help example", command.name)
		}
	}
	for _, note := range command.notes {
		if note == "" || !utf8.ValidString(note) {
			return invalidSpec("command %q has an invalid Help note", command.name)
		}
	}
	for _, link := range command.links {
		if link.Label == "" || link.URL == "" ||
			!utf8.ValidString(link.Label) || !utf8.ValidString(link.URL) {
			return invalidSpec("command %q has an invalid Help link", command.name)
		}
	}
	sectionIDs := map[string]struct{}{}
	for _, section := range command.helpSections {
		if !validID(section.id) || has(sectionIDs, section.id) ||
			section.heading == "" || !utf8.ValidString(section.heading) ||
			len(section.blocks) == 0 {
			return invalidSpec("command %q has an invalid Help section %q", command.name, section.id)
		}
		sectionIDs[section.id] = struct{}{}
		for _, block := range section.blocks {
			if block.Kind != HelpBlockParagraph && block.Kind != HelpBlockEntry {
				return invalidSpec("Help section %q has an invalid block kind", section.id)
			}
			if block.Text == "" || !utf8.ValidString(block.Text) ||
				(block.Kind == HelpBlockEntry && (block.Label == "" || !utf8.ValidString(block.Label))) {
				return invalidSpec("Help section %q has an invalid block", section.id)
			}
		}
	}
	return nil
}

func validUsageSyntax(syntax string) bool {
	if syntax == "" || !utf8.ValidString(syntax) ||
		syntax[0] == ' ' || syntax[len(syntax)-1] == ' ' {
		return false
	}
	for index := 0; index < len(syntax); index++ {
		if syntax[index] < 0x20 || syntax[index] == 0x7f {
			return false
		}
	}
	return true
}

func has(set map[string]struct{}, value string) bool {
	_, ok := set[value]
	return ok
}
