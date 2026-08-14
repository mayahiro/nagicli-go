package cli

import "fmt"

const opaqueCompletionInput = "<opaque>"

// Format implements fmt.Formatter without exposing a Sensitive option value
func (o OptionSpec) Format(state fmt.State, _ rune) {
	defaultValue := "<unset>"
	if o.defaultSet {
		defaultValue = RedactedValue
		if !o.sensitive {
			defaultValue = displayValue(o.defaultVal)
		}
	}
	fmt.Fprintf(
		state,
		"OptionSpec{id:%q kind:%d sensitive:%t default:%q}",
		o.id,
		o.kind,
		o.sensitive,
		defaultValue,
	)
}

// Format implements fmt.Formatter without exposing parser or provider state
func (a Argument) Format(state fmt.State, _ rune) {
	fmt.Fprintf(
		state,
		"Argument{id:%q sensitive:%t required:%t repeated:%t}",
		a.id,
		a.sensitive,
		a.required,
		a.repeated,
	)
}

// Format implements fmt.Formatter without recursively exposing Command Graph
// values, parsers, providers, validators, or handlers
func (c Command) Format(state fmt.State, _ rune) {
	fmt.Fprintf(
		state,
		"Command{id:%q name:%q options:%d arguments:%d subcommands:%d}",
		c.id,
		c.name,
		len(c.options),
		len(c.arguments),
		len(c.subcommands),
	)
}

// Format implements fmt.Formatter without exposing parsed values
func (i Invocation) Format(state fmt.State, _ rune) {
	fmt.Fprintf(
		state,
		"Invocation{command_path:%q command_id_path:%q scopes:%d current_scope:%d}",
		i.commandPath,
		i.commandIDPath,
		len(i.scopes),
		i.currentScope,
	)
}

// Format implements fmt.Formatter without exposing parsed values
func (s InvocationScope) Format(state fmt.State, _ rune) {
	if !s.valid() {
		fmt.Fprint(state, "InvocationScope<invalid>")
		return
	}
	fmt.Fprintf(
		state,
		"InvocationScope{command_id_path:%q}",
		s.invocation.commandIDPath[:s.index+1],
	)
}

// Format implements fmt.Formatter without exposing parsed values
func (r ParseResult) Format(state fmt.State, _ rune) {
	fmt.Fprintf(
		state,
		"ParseResult{kind:%d command_path:%q command_id_path:%q}",
		r.kind,
		r.CommandPath(),
		r.CommandIDPath(),
	)
}

// Format implements fmt.Formatter while treating every raw shell token as
// opaque because Completion Input has no Command Graph metadata
func (i CompletionInput) Format(state fmt.State, _ rune) {
	fmt.Fprintf(
		state,
		"CompletionInput{arguments:%q current:%q}",
		opaqueCompletionInput,
		opaqueCompletionInput,
	)
}

// Format implements fmt.Formatter without exposing a Sensitive occurrence
func (o CompletionOccurrence) Format(state fmt.State, _ rune) {
	raw := o.raw
	if o.target.sensitive && o.rawSet {
		raw = RedactedValue
	}
	fmt.Fprintf(
		state,
		"CompletionOccurrence{target:%v kind:%d raw:%q raw_set:%t}",
		o.target,
		o.kind,
		raw,
		o.rawSet,
	)
}

// Format implements fmt.Formatter without exposing Sensitive or unclassified
// shell input
func (r CompletionRequest) Format(state fmt.State, _ rune) {
	current := r.current
	prefix := r.prefix
	if r.target.sensitive {
		current = RedactedValue
		prefix = RedactedValue
	}
	fmt.Fprintf(
		state,
		"CompletionRequest{arguments:%q current:%q command_path:%q command_id_path:%q target:%v prefix:%q partial:%d}",
		opaqueCompletionInput,
		current,
		r.commandPath,
		r.commandIDs,
		r.target,
		prefix,
		len(r.partial),
	)
}

// Format implements fmt.Formatter without exposing request input or candidate
// values
func (r CompletionResult) Format(state fmt.State, _ rune) {
	fmt.Fprintf(
		state,
		"CompletionResult{request:%v candidates:%d}",
		r.request,
		len(r.candidates),
	)
}
