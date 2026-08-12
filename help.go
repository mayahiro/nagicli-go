package cli

import (
	"strings"

	nagitext "github.com/mayahiro/nagi-go/text"
)

type usageVariant struct {
	id     string
	syntax string
}

// HelpUsageVariant is one structured invocation syntax in a Help Document
type HelpUsageVariant struct {
	// CommandIDPath identifies the source Command using stable IDs
	CommandIDPath []string
	// ID is the stable variant identifier
	ID string
	// Syntax is the command-path-relative syntax suffix
	Syntax string
	// CommandLine is the complete canonical usage line
	CommandLine string
}

// HelpEntry is one labeled description in a Help Document
type HelpEntry struct {
	// ID is the stable command, argument, option, or generated entry identifier
	ID string
	// Label is the cell-aligned entry label
	Label string
	// Description explains the labeled item
	Description string
}

// HelpInheritedOption is one option inherited from an ancestor in a Help
// Document.
type HelpInheritedOption struct {
	// CommandPath is the source canonical command path.
	CommandPath []string
	// CommandIDPath is the source stable command-ID path.
	CommandIDPath []string
	// ID is the source-local stable option ID.
	ID string
	// Label is the option display label.
	Label string
	// Description excludes the rendered origin note.
	Description string
}

// HelpExample is one named command invocation
type HelpExample struct {
	// Name identifies the purpose of the example
	Name string
	// Invocation is the complete example command line
	Invocation string
}

// HelpLink is one labeled documentation link
type HelpLink struct {
	// Label identifies the linked resource
	Label string
	// URL is the link target
	URL string
}

// HelpBlockKind distinguishes custom-section paragraphs and entries
type HelpBlockKind uint8

const (
	// HelpBlockParagraph is one indented text block
	HelpBlockParagraph HelpBlockKind = iota
	// HelpBlockEntry is one cell-aligned labeled description
	HelpBlockEntry
)

// HelpBlock is one ordered block in a custom Help section
type HelpBlock struct {
	// Kind selects paragraph or labeled-entry rendering
	Kind HelpBlockKind
	// Label is used by HelpBlockEntry
	Label string
	// Text is paragraph text or an entry description
	Text string
}

// HelpSection is one application-defined structured Help section
type HelpSection struct {
	id      string
	heading string
	blocks  []HelpBlock
}

// NewHelpSection constructs an empty custom Help section
func NewHelpSection(id, heading string) *HelpSection {
	return &HelpSection{id: id, heading: heading}
}

// Paragraph appends one text paragraph
func (s *HelpSection) Paragraph(text string) *HelpSection {
	s.blocks = append(s.blocks, HelpBlock{Kind: HelpBlockParagraph, Text: text})
	return s
}

// Entry appends one labeled description
func (s *HelpSection) Entry(label, description string) *HelpSection {
	s.blocks = append(s.blocks, HelpBlock{
		Kind:  HelpBlockEntry,
		Label: label,
		Text:  description,
	})
	return s
}

// ID returns the stable section identifier
func (s HelpSection) ID() string { return s.id }

// Heading returns the rendered section heading
func (s HelpSection) Heading() string { return s.heading }

// Blocks returns a copy of the ordered section blocks
func (s HelpSection) Blocks() []HelpBlock {
	return append([]HelpBlock(nil), s.blocks...)
}

// HelpOptionGroup describes one portable option-group constraint
type HelpOptionGroup struct {
	// ID is the stable group identifier
	ID string
	// Kind is the group validation rule
	Kind OptionGroupKind
	// Presence selects resolved or command-line presence
	Presence PresenceBasis
	// OptionIDs contains stable member IDs in definition order
	OptionIDs []string
	// OptionLabels contains display spellings in definition order
	OptionLabels []string
}

// HelpOptionRelationKind distinguishes requires and conflicts constraints
type HelpOptionRelationKind uint8

const (
	// HelpRelationRequires requires the target when the source is present
	HelpRelationRequires HelpOptionRelationKind = iota
	// HelpRelationConflicts rejects the source and target together
	HelpRelationConflicts
)

// HelpOptionRelation describes one portable pairwise option constraint
type HelpOptionRelation struct {
	// Kind selects requires or conflicts behavior
	Kind HelpOptionRelationKind
	// SourceID is the stable source option ID
	SourceID string
	// SourceLabel is the source option display spelling
	SourceLabel string
	// TargetID is the stable target option ID
	TargetID string
	// TargetLabel is the target option display spelling
	TargetLabel string
	// Presence selects resolved or command-line presence
	Presence PresenceBasis
}

// HelpDocument is the structured, renderer-independent Help representation
type HelpDocument struct {
	commandPath      []string
	description      string
	usage            []string
	usageVariants    []HelpUsageVariant
	commands         []HelpEntry
	arguments        []HelpEntry
	options          []HelpEntry
	inheritedOptions []HelpInheritedOption
	optionRelations  []HelpOptionRelation
	optionGroups     []HelpOptionGroup
	examples         []HelpExample
	notes            []string
	links            []HelpLink
	sections         []HelpSection
}

// CommandPath returns the canonical root-to-target path
func (d HelpDocument) CommandPath() []string {
	return append([]string(nil), d.commandPath...)
}

// Description returns the command description
func (d HelpDocument) Description() string { return d.description }

// Usage returns a copy of rendered usage lines
func (d HelpDocument) Usage() []string { return append([]string(nil), d.usage...) }

// UsageVariants returns a copy of structured usage metadata
func (d HelpDocument) UsageVariants() []HelpUsageVariant {
	variants := make([]HelpUsageVariant, len(d.usageVariants))
	for index, variant := range d.usageVariants {
		variants[index] = variant
		variants[index].CommandIDPath = append([]string(nil), variant.CommandIDPath...)
	}
	return variants
}

// Commands returns a copy of child-command entries
func (d HelpDocument) Commands() []HelpEntry {
	return append([]HelpEntry(nil), d.commands...)
}

// Arguments returns a copy of positional-argument entries
func (d HelpDocument) Arguments() []HelpEntry {
	return append([]HelpEntry(nil), d.arguments...)
}

// Options returns a copy of option entries
func (d HelpDocument) Options() []HelpEntry {
	return append([]HelpEntry(nil), d.options...)
}

// InheritedOptions returns a deep copy in outermost-to-nearest ancestor and
// definition order.
func (d HelpDocument) InheritedOptions() []HelpInheritedOption {
	options := make([]HelpInheritedOption, len(d.inheritedOptions))
	for index, option := range d.inheritedOptions {
		options[index] = option
		options[index].CommandPath = append([]string(nil), option.CommandPath...)
		options[index].CommandIDPath = append([]string(nil), option.CommandIDPath...)
	}
	return options
}

// OptionRelations returns a copy of pairwise option constraints
func (d HelpDocument) OptionRelations() []HelpOptionRelation {
	return append([]HelpOptionRelation(nil), d.optionRelations...)
}

// OptionGroups returns a deep copy of option-group metadata
func (d HelpDocument) OptionGroups() []HelpOptionGroup {
	groups := make([]HelpOptionGroup, len(d.optionGroups))
	for index, group := range d.optionGroups {
		groups[index] = group
		groups[index].OptionIDs = append([]string(nil), group.OptionIDs...)
		groups[index].OptionLabels = append([]string(nil), group.OptionLabels...)
	}
	return groups
}

// Examples returns a copy of named examples
func (d HelpDocument) Examples() []HelpExample {
	return append([]HelpExample(nil), d.examples...)
}

// Notes returns a copy of Help notes
func (d HelpDocument) Notes() []string { return append([]string(nil), d.notes...) }

// Links returns a copy of documentation links
func (d HelpDocument) Links() []HelpLink { return append([]HelpLink(nil), d.links...) }

// Sections returns a deep copy of custom Help sections
func (d HelpDocument) Sections() []HelpSection {
	sections := make([]HelpSection, len(d.sections))
	for index, section := range d.sections {
		sections[index] = cloneHelpSection(section)
	}
	return sections
}

// HelpRenderer renders one structured Help Document
type HelpRenderer interface {
	// RenderHelp returns deterministic text with one final newline
	RenderHelp(document HelpDocument) string
}

// PlainHelpRenderer renders the standard cell-aware plain Help format
type PlainHelpRenderer struct{}

// RenderHelp renders the standard Help section order
func (PlainHelpRenderer) RenderHelp(document HelpDocument) string {
	var output strings.Builder
	if document.description != "" {
		output.WriteString(document.description)
		output.WriteString("\n\n")
	}

	output.WriteString("Usage:\n")
	for _, usage := range document.usage {
		output.WriteString("  ")
		output.WriteString(usage)
		output.WriteByte('\n')
	}
	renderHelpEntrySection(&output, "Commands", document.commands)
	renderHelpEntrySection(&output, "Arguments", document.arguments)
	renderHelpEntrySection(&output, "Options", document.options)
	inheritedOptions := make([]HelpEntry, 0, len(document.inheritedOptions))
	for _, option := range document.inheritedOptions {
		description := option.Description
		if description != "" {
			description += " "
		}
		description += "[from " + strings.Join(option.CommandPath, " ") + "]"
		inheritedOptions = append(inheritedOptions, HelpEntry{
			ID: option.ID, Label: option.Label, Description: description,
		})
	}
	renderHelpEntrySection(&output, "Inherited Options", inheritedOptions)

	if len(document.optionRelations) > 0 || len(document.optionGroups) > 0 {
		entries := make(
			[]HelpEntry,
			0,
			len(document.optionRelations)+len(document.optionGroups),
		)
		for _, relation := range document.optionRelations {
			entries = append(entries, HelpEntry{
				ID:          relation.SourceID,
				Label:       relation.SourceLabel,
				Description: optionRelationDescription(relation),
			})
		}
		for _, group := range document.optionGroups {
			entries = append(entries, HelpEntry{
				ID:          group.ID,
				Label:       group.ID,
				Description: optionGroupDescription(group),
			})
		}
		renderHelpEntrySection(&output, "Constraints", entries)
	}

	if len(document.examples) > 0 {
		entries := make([]HelpEntry, 0, len(document.examples))
		for _, example := range document.examples {
			entries = append(entries, HelpEntry{
				ID:          example.Name,
				Label:       example.Name,
				Description: example.Invocation,
			})
		}
		renderHelpEntrySection(&output, "Examples", entries)
	}

	if len(document.notes) > 0 {
		output.WriteString("\nNotes:\n")
		for _, note := range document.notes {
			renderHelpParagraph(&output, note)
		}
	}

	if len(document.links) > 0 {
		entries := make([]HelpEntry, 0, len(document.links))
		for _, link := range document.links {
			entries = append(entries, HelpEntry{
				ID:          link.Label,
				Label:       link.Label,
				Description: link.URL,
			})
		}
		renderHelpEntrySection(&output, "Links", entries)
	}

	for _, section := range document.sections {
		output.WriteString("\n")
		output.WriteString(section.heading)
		output.WriteString(":\n")
		renderHelpBlocks(&output, section.blocks)
	}
	return output.String()
}

// HelpDocument returns structured Help for a canonical command path
func (c *Command) HelpDocument(path []string) (HelpDocument, error) {
	if err := c.Validate(); err != nil {
		return HelpDocument{}, err
	}
	command := c.commandAtPath(path)
	if command == nil {
		return HelpDocument{}, NewDiagnostic(
			CodeInvalidSpecification,
			"help path does not identify a command",
		)
	}
	commandIDPath := c.commandIDPathAtPath(path)
	commandLineage := c.commandsAtPath(path)

	usageVariants := helpUsageVariants(command, path, commandIDPath)
	usage := make([]string, 0, len(usageVariants))
	for _, variant := range usageVariants {
		usage = append(usage, variant.CommandLine)
	}
	document := HelpDocument{
		commandPath: append([]string(nil), path...),
		description: command.about,
		usage:       usage,
		usageVariants: append(
			[]HelpUsageVariant(nil),
			usageVariants...,
		),
		examples: append([]HelpExample(nil), command.examples...),
		notes:    append([]string(nil), command.notes...),
		links:    append([]HelpLink(nil), command.links...),
	}
	for _, child := range command.subcommands {
		document.commands = append(document.commands, HelpEntry{
			ID:          child.id,
			Label:       child.name,
			Description: child.about,
		})
	}
	if command == c && len(command.subcommands) > 0 {
		document.commands = append(document.commands, HelpEntry{
			ID:          "help",
			Label:       "help",
			Description: "Print this message or the help of the given command",
		})
	}
	for index := range command.arguments {
		argument := &command.arguments[index]
		document.arguments = append(document.arguments, HelpEntry{
			ID:          argument.id,
			Label:       argumentLabel(argument),
			Description: argument.help,
		})
	}
	for index := range command.options {
		option := &command.options[index]
		document.options = append(document.options, HelpEntry{
			ID:          option.id,
			Label:       optionLabel(option),
			Description: optionDescription(option),
		})
		for _, relation := range option.requires {
			document.optionRelations = append(
				document.optionRelations,
				helpOptionRelation(command, option, relation, HelpRelationRequires),
			)
		}
		for _, relation := range option.conflicts {
			document.optionRelations = append(
				document.optionRelations,
				helpOptionRelation(command, option, relation, HelpRelationConflicts),
			)
		}
	}
	document.options = append(document.options, HelpEntry{
		ID:          "help",
		Label:       "-h, --help",
		Description: "Print help",
	})
	if c.version != "" {
		document.options = append(document.options, HelpEntry{
			ID:          "version",
			Label:       "-V, --version",
			Description: "Print version",
		})
	}
	for scopeIndex, ancestor := range commandLineage[:len(commandLineage)-1] {
		for optionIndex := range ancestor.options {
			option := &ancestor.options[optionIndex]
			if !option.inherited {
				continue
			}
			document.inheritedOptions = append(document.inheritedOptions, HelpInheritedOption{
				CommandPath:   append([]string(nil), path[:scopeIndex+1]...),
				CommandIDPath: append([]string(nil), commandIDPath[:scopeIndex+1]...),
				ID:            option.id,
				Label:         optionLabel(option),
				Description:   optionDescription(option),
			})
		}
	}
	for _, group := range command.optionGroups {
		metadata := HelpOptionGroup{
			ID:       group.id,
			Kind:     group.kind,
			Presence: group.presence,
			OptionIDs: append(
				[]string(nil),
				group.options...,
			),
			OptionLabels: make([]string, 0, len(group.options)),
		}
		for _, id := range group.options {
			metadata.OptionLabels = append(
				metadata.OptionLabels,
				optionDisplay(command.optionByID(id)),
			)
		}
		document.optionGroups = append(document.optionGroups, metadata)
	}
	for _, section := range command.helpSections {
		document.sections = append(document.sections, cloneHelpSection(section))
	}
	return document, nil
}

func helpUsageVariants(command *Command, path, commandIDPath []string) []HelpUsageVariant {
	var variants []HelpUsageVariant
	if !(command.subcommandUsage == SubcommandUsageExpanded &&
		command.subcommandRequired &&
		len(command.usageVariants) == 0) {
		variants = directHelpUsageVariants(command, path, commandIDPath, "")
	}
	switch command.subcommandUsage {
	case SubcommandUsageAuto:
		if len(command.subcommands) > 0 && !command.subcommandRequired {
			variants = append(variants, newHelpUsageVariant(
				commandIDPath,
				"subcommand",
				"[OPTIONS] <COMMAND>",
				path,
			))
		}
	case SubcommandUsageExpanded:
		for _, child := range command.subcommands {
			childIDPath := append(append([]string(nil), commandIDPath...), child.id)
			variants = append(
				variants,
				directHelpUsageVariants(child, path, childIDPath, child.name)...,
			)
		}
	}
	return variants
}

func directHelpUsageVariants(
	command *Command,
	helpPath, commandIDPath []string,
	prefix string,
) []HelpUsageVariant {
	if len(command.usageVariants) == 0 {
		return []HelpUsageVariant{newHelpUsageVariant(
			commandIDPath,
			"default",
			prefixedUsage(prefix, generatedUsageSyntax(command)),
			helpPath,
		)}
	}
	variants := make([]HelpUsageVariant, 0, len(command.usageVariants))
	for _, variant := range command.usageVariants {
		variants = append(variants, newHelpUsageVariant(
			commandIDPath,
			variant.id,
			prefixedUsage(prefix, variant.syntax),
			helpPath,
		))
	}
	return variants
}

func prefixedUsage(prefix, syntax string) string {
	if prefix == "" {
		return syntax
	}
	return prefix + " " + syntax
}

func newHelpUsageVariant(commandIDPath []string, id, syntax string, path []string) HelpUsageVariant {
	return HelpUsageVariant{
		CommandIDPath: append([]string(nil), commandIDPath...),
		ID:            id,
		Syntax:        syntax,
		CommandLine:   usageCommandLine(path, syntax),
	}
}

// RenderHelp renders standard Help for a canonical command path
func (c *Command) RenderHelp(path []string) (string, error) {
	document, err := c.HelpDocument(path)
	if err != nil {
		return "", err
	}
	return (PlainHelpRenderer{}).RenderHelp(document), nil
}

func cloneHelpSection(section HelpSection) HelpSection {
	section.blocks = append([]HelpBlock(nil), section.blocks...)
	return section
}

func renderHelpEntrySection(output *strings.Builder, heading string, entries []HelpEntry) {
	if len(entries) == 0 {
		return
	}
	output.WriteString("\n")
	output.WriteString(heading)
	output.WriteString(":\n")
	renderHelpEntries(output, entries)
}

func renderHelpEntries(output *strings.Builder, entries []HelpEntry) {
	width := 0
	for _, entry := range entries {
		if candidate := nagitext.Width(entry.Label, nagitext.ModernWidth()); candidate > width {
			width = candidate
		}
	}
	for _, entry := range entries {
		output.WriteString("  ")
		output.WriteString(entry.Label)
		labelWidth := nagitext.Width(entry.Label, nagitext.ModernWidth())
		output.WriteString(strings.Repeat(" ", width-labelWidth+2))
		output.WriteString(entry.Description)
		output.WriteByte('\n')
	}
}

func renderHelpBlocks(output *strings.Builder, blocks []HelpBlock) {
	entries := make([]HelpEntry, 0, len(blocks))
	for _, block := range blocks {
		if block.Kind == HelpBlockEntry {
			entries = append(entries, HelpEntry{Label: block.Label, Description: block.Text})
		}
	}
	width := 0
	for _, entry := range entries {
		if candidate := nagitext.Width(entry.Label, nagitext.ModernWidth()); candidate > width {
			width = candidate
		}
	}
	for _, block := range blocks {
		switch block.Kind {
		case HelpBlockParagraph:
			renderHelpParagraph(output, block.Text)
		case HelpBlockEntry:
			output.WriteString("  ")
			output.WriteString(block.Label)
			labelWidth := nagitext.Width(block.Label, nagitext.ModernWidth())
			output.WriteString(strings.Repeat(" ", width-labelWidth+2))
			output.WriteString(block.Text)
			output.WriteByte('\n')
		}
	}
}

func renderHelpParagraph(output *strings.Builder, text string) {
	for _, line := range strings.Split(text, "\n") {
		output.WriteString("  ")
		output.WriteString(line)
		output.WriteByte('\n')
	}
}

func optionGroupDescription(group HelpOptionGroup) string {
	var rule string
	switch group.Kind {
	case GroupAtMostOne:
		rule = "at most one of "
	case GroupExactlyOne:
		rule = "exactly one of "
	case GroupAtLeastOne:
		rule = "at least one of "
	case GroupAllOrNone:
		rule = "all or none of "
	}
	description := rule + strings.Join(group.OptionLabels, ", ")
	return description + presenceDescription(group.Presence)
}

func helpOptionRelation(
	command *Command,
	source *OptionSpec,
	relation optionRelation,
	kind HelpOptionRelationKind,
) HelpOptionRelation {
	return HelpOptionRelation{
		Kind:        kind,
		SourceID:    source.id,
		SourceLabel: optionDisplay(source),
		TargetID:    relation.id,
		TargetLabel: optionDisplay(command.optionByID(relation.id)),
		Presence:    relation.presence,
	}
}

func optionRelationDescription(relation HelpOptionRelation) string {
	var description string
	if relation.Kind == HelpRelationRequires {
		description = "requires "
	} else {
		description = "conflicts with "
	}
	return description + relation.TargetLabel + presenceDescription(relation.Presence)
}

func presenceDescription(presence PresenceBasis) string {
	if presence == PresenceCommandLine {
		return " [command line]"
	}
	return " [resolved]"
}
