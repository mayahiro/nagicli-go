// Package document renders Nagi CLI Help Documents as Markdown or man pages
// without performing filesystem I/O
package document

import (
	"strings"
	"unicode"
	"unicode/utf8"

	nagitext "github.com/mayahiro/nagi-go/text"
	cli "github.com/mayahiro/nagicli-go"
)

// MarkdownRenderer renders deterministic CommonMark from one Help Document
type MarkdownRenderer struct{}

// Render renders one Help Document with exactly one final newline
func (MarkdownRenderer) Render(document cli.HelpDocument) string {
	return renderMarkdown(document)
}

// RenderHelp implements cli.HelpRenderer
func (MarkdownRenderer) RenderHelp(document cli.HelpDocument) string {
	return renderMarkdown(document)
}

// ManRenderer renders one section 1 page with the portable man macro subset
type ManRenderer struct{}

// Render renders one Help Document with exactly one final newline
func (ManRenderer) Render(document cli.HelpDocument) string {
	return renderMan(document)
}

// RenderHelp implements cli.HelpRenderer
func (ManRenderer) RenderHelp(document cli.HelpDocument) string {
	return renderMan(document)
}

func renderMarkdown(document cli.HelpDocument) string {
	var output strings.Builder
	output.WriteString("# ")
	pushMarkdownInline(&output, strings.Join(document.CommandPath(), " "))
	output.WriteString("\n\n")

	if document.Description() != "" {
		pushMarkdownText(&output, document.Description(), "\n")
		output.WriteString("\n\n")
	}
	if deprecation, ok := document.Deprecation(); ok {
		output.WriteString("**Deprecated:** use ")
		pushMarkdownInline(&output, deprecation.Replacement())
		output.WriteString("\n\n")
	}

	output.WriteString("## Usage\n\n")
	for _, usage := range document.Usage() {
		pushMarkdownCodeBlock(&output, usage)
	}
	pushMarkdownEntries(&output, "Commands", document.Commands())
	pushMarkdownEntries(&output, "Arguments", document.Arguments())
	pushMarkdownEntries(&output, "Options", document.Options())

	if options := document.InheritedOptions(); len(options) > 0 {
		output.WriteString("## Inherited Options\n\n")
		for _, option := range options {
			description := option.Description
			if description != "" {
				description += " "
			}
			description += "[from " + strings.Join(option.CommandPath, " ") + "]"
			var replacement string
			if deprecation, ok := option.Deprecation(); ok {
				replacement = deprecation.Replacement()
			}
			pushMarkdownEntry(&output, option.Label, description, replacement)
		}
		output.WriteByte('\n')
	}

	relations := document.OptionRelations()
	groups := document.OptionGroups()
	if len(relations) > 0 || len(groups) > 0 {
		output.WriteString("## Constraints\n\n")
		for _, relation := range relations {
			pushMarkdownEntry(
				&output,
				relation.SourceLabel,
				optionRelationDescription(relation),
				"",
			)
		}
		for _, group := range groups {
			pushMarkdownEntry(&output, group.ID, optionGroupDescription(group), "")
		}
		output.WriteByte('\n')
	}

	if examples := document.Examples(); len(examples) > 0 {
		output.WriteString("## Examples\n\n")
		for _, example := range examples {
			output.WriteString("### ")
			pushMarkdownInline(&output, example.Name)
			output.WriteString("\n\n")
			pushMarkdownCodeBlock(&output, example.Invocation)
		}
	}
	if notes := document.Notes(); len(notes) > 0 {
		output.WriteString("## Notes\n\n")
		for _, note := range notes {
			pushMarkdownText(&output, note, "\n")
			output.WriteString("\n\n")
		}
	}
	if links := document.Links(); len(links) > 0 {
		output.WriteString("## Links\n\n")
		for _, link := range links {
			output.WriteString("- [")
			pushMarkdownInline(&output, link.Label)
			output.WriteString("](<")
			pushMarkdownDestination(&output, link.URL)
			output.WriteString(">)\n")
		}
		output.WriteByte('\n')
	}
	for _, section := range document.Sections() {
		output.WriteString("## ")
		pushMarkdownInline(&output, section.Heading())
		output.WriteString("\n\n")
		for _, block := range section.Blocks() {
			switch block.Kind {
			case cli.HelpBlockParagraph:
				pushMarkdownText(&output, block.Text, "\n")
				output.WriteString("\n\n")
			case cli.HelpBlockEntry:
				pushMarkdownEntry(&output, block.Label, block.Text, "")
			}
		}
		output.WriteByte('\n')
	}

	return finishOutput(output.String())
}

func pushMarkdownEntries(output *strings.Builder, heading string, entries []cli.HelpEntry) {
	if len(entries) == 0 {
		return
	}
	output.WriteString("## ")
	output.WriteString(heading)
	output.WriteString("\n\n")
	for _, entry := range entries {
		var replacement string
		if deprecation, ok := entry.Deprecation(); ok {
			replacement = deprecation.Replacement()
		}
		pushMarkdownEntry(output, entry.Label, entry.Description, replacement)
	}
	output.WriteByte('\n')
}

func pushMarkdownEntry(output *strings.Builder, label, description, replacement string) {
	output.WriteString("- **")
	pushMarkdownInline(output, label)
	output.WriteString("**")
	if description != "" {
		output.WriteString(": ")
		pushMarkdownText(output, description, "  \n  ")
	}
	if replacement != "" {
		output.WriteString(" [deprecated: use ")
		pushMarkdownInline(output, replacement)
		output.WriteByte(']')
	}
	output.WriteByte('\n')
}

func pushMarkdownCodeBlock(output *strings.Builder, text string) {
	output.WriteString("    ")
	forNormalizedRunes(text, func(character rune) {
		if character == '\n' {
			output.WriteString("\n    ")
		} else {
			output.WriteRune(character)
		}
	})
	output.WriteString("\n\n")
}

func pushMarkdownText(output *strings.Builder, text, newline string) {
	lineStart := true
	forNormalizedRunes(text, func(character rune) {
		if character == '\n' {
			output.WriteString(newline)
			lineStart = true
		} else if lineStart && character == ' ' {
			output.WriteString("&#32;")
			lineStart = false
		} else {
			pushMarkdownCharacter(output, character)
			lineStart = false
		}
	})
}

func pushMarkdownInline(output *strings.Builder, text string) {
	forNormalizedRunes(text, func(character rune) {
		if character == '\n' {
			character = '\uFFFD'
		}
		pushMarkdownCharacter(output, character)
	})
}

func pushMarkdownCharacter(output *strings.Builder, character rune) {
	if character <= unicode.MaxASCII && strings.ContainsRune(
		"!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~",
		character,
	) {
		output.WriteByte('\\')
	}
	output.WriteRune(character)
}

func pushMarkdownDestination(output *strings.Builder, value string) {
	forNormalizedRunes(value, func(character rune) {
		if unicode.IsSpace(character) || unicode.IsControl(character) ||
			character == '<' || character == '>' || character == '\\' {
			var bytes [utf8.UTFMax]byte
			length := utf8.EncodeRune(bytes[:], character)
			for _, value := range bytes[:length] {
				const hex = "0123456789ABCDEF"
				output.WriteByte('%')
				output.WriteByte(hex[value>>4])
				output.WriteByte(hex[value&0x0f])
			}
		} else {
			output.WriteRune(character)
		}
	})
}

func renderMan(document cli.HelpDocument) string {
	path := strings.Join(document.CommandPath(), " ")
	var output strings.Builder
	output.WriteString(".\\\" Generated by Nagi\n.TH \"")
	pushRoffArgument(&output, strings.ToUpper(path))
	output.WriteString("\" \"1\"\n.SH NAME\n")
	pushRoffInline(&output, path)
	if document.Description() != "" {
		output.WriteString(" \\- ")
		pushRoffInline(&output, document.Description())
	}
	output.WriteByte('\n')

	output.WriteString(".SH SYNOPSIS\n.nf\n")
	for _, usage := range document.Usage() {
		pushRoffText(&output, usage)
		output.WriteByte('\n')
	}
	output.WriteString(".fi\n")
	if document.Description() != "" {
		output.WriteString(".SH DESCRIPTION\n")
		pushRoffText(&output, document.Description())
		output.WriteByte('\n')
	}
	if deprecation, ok := document.Deprecation(); ok {
		output.WriteString(".SH DEPRECATED\n")
		pushRoffText(&output, "Use ")
		pushRoffText(&output, deprecation.Replacement())
		output.WriteString(" instead\n")
	}

	pushManEntries(&output, "COMMANDS", document.Commands())
	pushManEntries(&output, "ARGUMENTS", document.Arguments())
	pushManEntries(&output, "OPTIONS", document.Options())
	if options := document.InheritedOptions(); len(options) > 0 {
		output.WriteString(".SH \"INHERITED OPTIONS\"\n")
		for _, option := range options {
			description := option.Description
			if description != "" {
				description += " "
			}
			description += "[from " + strings.Join(option.CommandPath, " ") + "]"
			var replacement string
			if deprecation, ok := option.Deprecation(); ok {
				replacement = deprecation.Replacement()
			}
			pushManEntry(&output, option.Label, description, replacement)
		}
	}

	relations := document.OptionRelations()
	groups := document.OptionGroups()
	if len(relations) > 0 || len(groups) > 0 {
		output.WriteString(".SH CONSTRAINTS\n")
		for _, relation := range relations {
			pushManEntry(
				&output,
				relation.SourceLabel,
				optionRelationDescription(relation),
				"",
			)
		}
		for _, group := range groups {
			pushManEntry(&output, group.ID, optionGroupDescription(group), "")
		}
	}
	if examples := document.Examples(); len(examples) > 0 {
		output.WriteString(".SH EXAMPLES\n")
		for index, example := range examples {
			pushRoffParagraph(&output, example.Name, index != 0)
			output.WriteString(".nf\n")
			pushRoffText(&output, example.Invocation)
			output.WriteString("\n.fi\n")
		}
	}
	if notes := document.Notes(); len(notes) > 0 {
		output.WriteString(".SH NOTES\n")
		for index, note := range notes {
			pushRoffParagraph(&output, note, index != 0)
		}
	}
	if links := document.Links(); len(links) > 0 {
		output.WriteString(".SH LINKS\n")
		for _, link := range links {
			pushManEntry(&output, link.Label, link.URL, "")
		}
	}
	for _, section := range document.Sections() {
		output.WriteString(".SH \"")
		pushRoffArgument(&output, section.Heading())
		output.WriteString("\"\n")
		for index, block := range section.Blocks() {
			switch block.Kind {
			case cli.HelpBlockParagraph:
				pushRoffParagraph(&output, block.Text, index != 0)
			case cli.HelpBlockEntry:
				pushManEntry(&output, block.Label, block.Text, "")
			}
		}
	}
	return finishOutput(output.String())
}

func pushManEntries(output *strings.Builder, heading string, entries []cli.HelpEntry) {
	if len(entries) == 0 {
		return
	}
	output.WriteString(".SH ")
	output.WriteString(heading)
	output.WriteByte('\n')
	for _, entry := range entries {
		var replacement string
		if deprecation, ok := entry.Deprecation(); ok {
			replacement = deprecation.Replacement()
		}
		pushManEntry(output, entry.Label, entry.Description, replacement)
	}
}

func pushManEntry(output *strings.Builder, label, description, replacement string) {
	output.WriteString(".TP\n\\fB")
	pushRoffText(output, label)
	output.WriteString("\\fP\n")
	if description != "" {
		pushRoffText(output, description)
	}
	if replacement != "" {
		if description != "" {
			output.WriteByte(' ')
		}
		pushRoffText(output, "[deprecated: use ")
		pushRoffText(output, replacement)
		pushRoffText(output, "]")
	}
	output.WriteByte('\n')
}

func pushRoffParagraph(output *strings.Builder, text string, separated bool) {
	if separated {
		output.WriteString(".PP\n")
	}
	pushRoffText(output, text)
	output.WriteByte('\n')
}

func pushRoffText(output *strings.Builder, text string) {
	lineStart := true
	forNormalizedRunes(text, func(character rune) {
		if character == '\n' {
			output.WriteByte('\n')
			lineStart = true
			return
		}
		if lineStart && (character == '.' || character == '\'') {
			output.WriteString("\\&")
		}
		lineStart = false
		switch character {
		case '\\':
			output.WriteString("\\e")
		case '-':
			output.WriteString("\\-")
		default:
			output.WriteRune(character)
		}
	})
}

func pushRoffInline(output *strings.Builder, text string) {
	forNormalizedRunes(text, func(character rune) {
		switch character {
		case '\n':
			output.WriteRune('\uFFFD')
		case '\\':
			output.WriteString("\\e")
		case '-':
			output.WriteString("\\-")
		default:
			output.WriteRune(character)
		}
	})
}

func pushRoffArgument(output *strings.Builder, text string) {
	forNormalizedRunes(text, func(character rune) {
		switch character {
		case '\n':
			output.WriteRune('\uFFFD')
		case '\\':
			output.WriteString("\\e")
		case '-':
			output.WriteString("\\-")
		case '"':
			output.WriteString("\\(dq")
		default:
			output.WriteRune(character)
		}
	})
}

func normalizeText(text string) string {
	var output strings.Builder
	output.Grow(len(text))
	forNormalizedRunes(text, func(character rune) { output.WriteRune(character) })
	return output.String()
}

func forNormalizedRunes(text string, visitor func(rune)) {
	text = nagitext.NormalizeUTF8(text)
	for index := 0; index < len(text); {
		character, size := utf8.DecodeRuneInString(text[index:])
		index += size
		switch character {
		case '\r':
			if index < len(text) && text[index] == '\n' {
				index++
			}
			visitor('\n')
		case '\n':
			visitor('\n')
		default:
			if unicode.IsControl(character) {
				visitor('\uFFFD')
			} else {
				visitor(character)
			}
		}
	}
}

func optionGroupDescription(group cli.HelpOptionGroup) string {
	var rule string
	switch group.Kind {
	case cli.GroupAtMostOne:
		rule = "at most one of "
	case cli.GroupExactlyOne:
		rule = "exactly one of "
	case cli.GroupAtLeastOne:
		rule = "at least one of "
	case cli.GroupAllOrNone:
		rule = "all or none of "
	}
	return rule + strings.Join(group.OptionLabels, ", ") + presenceDescription(group.Presence)
}

func optionRelationDescription(relation cli.HelpOptionRelation) string {
	rule := "conflicts with "
	if relation.Kind == cli.HelpRelationRequires {
		rule = "requires "
	}
	return rule + relation.TargetLabel + presenceDescription(relation.Presence)
}

func presenceDescription(presence cli.PresenceBasis) string {
	if presence == cli.PresenceCommandLine {
		return " [command line]"
	}
	return " [resolved]"
}

func finishOutput(output string) string {
	return strings.TrimRight(output, "\n") + "\n"
}
