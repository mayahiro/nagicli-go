package cli

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

// Deprecation contains replacement metadata for a deprecated Command or
// Option
type Deprecation struct {
	replacement string
	configured  bool
}

func newDeprecation(replacement string) Deprecation {
	return Deprecation{replacement: replacement, configured: true}
}

// Replacement returns the application-provided replacement hint
func (d Deprecation) Replacement() string { return d.replacement }

// DeprecationTargetKind identifies whether a notice targets a Command or
// Option
type DeprecationTargetKind uint8

const (
	// DeprecationTargetCommand identifies a selected deprecated Command
	DeprecationTargetCommand DeprecationTargetKind = iota
	// DeprecationTargetOption identifies a deprecated command-line Option
	DeprecationTargetOption
)

// DeprecationNotice is one non-fatal use of deprecated Command Graph syntax
type DeprecationNotice struct {
	kind          DeprecationTargetKind
	commandPath   []string
	commandIDPath []string
	valueID       string
	spelling      string
	replacement   string
}

func commandDeprecationNotice(
	commandPath []string,
	commandIDPath []string,
	spelling string,
	deprecation Deprecation,
) DeprecationNotice {
	return DeprecationNotice{
		kind:          DeprecationTargetCommand,
		commandPath:   append([]string(nil), commandPath...),
		commandIDPath: append([]string(nil), commandIDPath...),
		spelling:      spelling,
		replacement:   deprecation.replacement,
	}
}

func optionDeprecationNotice(
	commandPath []string,
	commandIDPath []string,
	valueID string,
	spelling string,
	deprecation Deprecation,
) DeprecationNotice {
	return DeprecationNotice{
		kind:          DeprecationTargetOption,
		commandPath:   append([]string(nil), commandPath...),
		commandIDPath: append([]string(nil), commandIDPath...),
		valueID:       valueID,
		spelling:      spelling,
		replacement:   deprecation.replacement,
	}
}

// TargetKind returns whether this notice identifies a Command or Option
func (n DeprecationNotice) TargetKind() DeprecationTargetKind { return n.kind }

// CommandPath returns a copy of the canonical selected path when the syntax
// was used
func (n DeprecationNotice) CommandPath() []string {
	return append([]string(nil), n.commandPath...)
}

// CommandIDPath returns a copy of the stable path of the target declaration
func (n DeprecationNotice) CommandIDPath() []string {
	return append([]string(nil), n.commandIDPath...)
}

// ValueID returns the command-local option ID for an Option notice
func (n DeprecationNotice) ValueID() string { return n.valueID }

// Spelling returns the recognized argv spelling, or the canonical root name
// when the root Command itself is deprecated
func (n DeprecationNotice) Spelling() string { return n.spelling }

// Replacement returns the application-provided replacement hint
func (n DeprecationNotice) Replacement() string { return n.replacement }

// DeprecationNoticeRenderer renders one non-fatal deprecation notice
type DeprecationNoticeRenderer interface {
	// RenderDeprecationNotice returns text with one final newline
	RenderDeprecationNotice(notice DeprecationNotice) string
}

// PlainDeprecationNoticeRenderer renders stable plain deprecation notice text
type PlainDeprecationNoticeRenderer struct{}

// RenderDeprecationNotice renders the stable warning and replacement hint
func (PlainDeprecationNoticeRenderer) RenderDeprecationNotice(notice DeprecationNotice) string {
	target := "option"
	if notice.kind == DeprecationTargetCommand {
		target = "command"
	}
	return fmt.Sprintf(
		"warning[deprecated-%s]: %s '%s' is deprecated\nhint: use %s\n",
		target,
		target,
		notice.spelling,
		notice.replacement,
	)
}

func validDeprecationReplacement(replacement string) bool {
	return replacement != "" && utf8.ValidString(replacement) &&
		!containsUnicodeControl(replacement)
}

func containsUnicodeControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) {
			return true
		}
	}
	return false
}

var _ DeprecationNoticeRenderer = PlainDeprecationNoticeRenderer{}
