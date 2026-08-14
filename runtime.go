package cli

import (
	stdcontext "context"
	"errors"
	"io"
	"os"
	"os/signal"
	"strings"
)

// Context contains the process services injected into a command handler
type Context struct {
	stdin            io.Reader
	stdout           io.Writer
	stderr           io.Writer
	environment      map[string]string
	currentDirectory string
	cancellation     stdcontext.Context
	valueResolver    ValueResolver
	responseFiles    bool
	responseOptions  ResponseFileOptions
	responseReader   ResponseFileReader
}

// NewContext constructs an injected Context without cancellation
func NewContext(
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	environment map[string]string,
	currentDirectory string,
) *Context {
	return NewContextWithCancellation(
		stdin,
		stdout,
		stderr,
		environment,
		currentDirectory,
		stdcontext.Background(),
	)
}

// NewContextWithCancellation constructs an injected Context with an explicit cancellation source
func NewContextWithCancellation(
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	environment map[string]string,
	currentDirectory string,
	cancellation stdcontext.Context,
) *Context {
	values := make(map[string]string, len(environment))
	for name, value := range environment {
		values[name] = value
	}
	if cancellation == nil {
		cancellation = stdcontext.Background()
	}
	return &Context{
		stdin:            stdin,
		stdout:           stdout,
		stderr:           stderr,
		environment:      values,
		currentDirectory: currentDirectory,
		cancellation:     cancellation,
	}
}

// Stdin returns standard input access
func (c *Context) Stdin() io.Reader { return c.stdin }

// Stdout returns standard output access
func (c *Context) Stdout() io.Writer { return c.stdout }

// Stderr returns standard error access
func (c *Context) Stderr() io.Writer { return c.stderr }

// Environment returns one injected environment value
func (c *Context) Environment(name string) (string, bool) {
	value, ok := c.environment[name]
	return value, ok
}

// EnvironmentValues returns a copy of the injected environment
func (c *Context) EnvironmentValues() map[string]string {
	values := make(map[string]string, len(c.environment))
	for name, value := range c.environment {
		values[name] = value
	}
	return values
}

// CurrentDirectory returns the injected current directory
func (c *Context) CurrentDirectory() string { return c.currentDirectory }

// Cancellation returns the cooperative cancellation source
func (c *Context) Cancellation() stdcontext.Context { return c.cancellation }

// WithValueResolver configures an application-owned Value Resolver
//
// The resolver receives only selected Value Options that remain unresolved
// after command-line and environment processing. A nil resolver clears it
func (c *Context) WithValueResolver(resolver ValueResolver) *Context {
	c.valueResolver = resolver
	return c
}

// ValueResolver returns the configured application Value Resolver
func (c *Context) ValueResolver() ValueResolver { return c.valueResolver }

// WithResponseFiles configures opt-in Response File expansion
//
// Exact @- expansion, when enabled in options, consumes this Context's standard
// input. A nil reader supports standard-input-only expansion; a file include
// then returns an invalid-specification Diagnostic
func (c *Context) WithResponseFiles(
	options ResponseFileOptions,
	reader ResponseFileReader,
) *Context {
	c.responseFiles = true
	c.responseOptions = options
	c.responseReader = reader
	return c
}

// WithoutResponseFiles disables Response File expansion
func (c *Context) WithoutResponseFiles() *Context {
	c.responseFiles = false
	c.responseOptions = ResponseFileOptions{}
	c.responseReader = nil
	return c
}

// ResponseFileOptions returns configured options and whether expansion is enabled
func (c *Context) ResponseFileOptions() (ResponseFileOptions, bool) {
	return c.responseOptions, c.responseFiles
}

// ProcessOptions composes complete process Runtime services
//
// Its zero value uses the default Runtime Policy without a Value Resolver or
// Response File expansion
type ProcessOptions struct {
	policy          RuntimePolicy
	valueResolver   ValueResolver
	responseFiles   bool
	responseOptions ResponseFileOptions
}

// DefaultProcessOptions returns process integration without optional services
func DefaultProcessOptions() ProcessOptions {
	return ProcessOptions{policy: DefaultRuntimePolicy()}
}

// WithPolicy returns a copy using an explicit Runtime Policy
func (o ProcessOptions) WithPolicy(policy RuntimePolicy) ProcessOptions {
	o.policy = policy
	return o
}

// WithValueResolver returns a copy using an application-owned Value Resolver
func (o ProcessOptions) WithValueResolver(resolver ValueResolver) ProcessOptions {
	o.valueResolver = resolver
	return o
}

// WithoutValueResolver returns a copy without a Value Resolver
func (o ProcessOptions) WithoutValueResolver() ProcessOptions {
	o.valueResolver = nil
	return o
}

// WithResponseFiles returns a copy enabling filesystem-backed Response Files
func (o ProcessOptions) WithResponseFiles(options ResponseFileOptions) ProcessOptions {
	o.responseFiles = true
	o.responseOptions = options
	return o
}

// WithoutResponseFiles returns a copy without Response File expansion
func (o ProcessOptions) WithoutResponseFiles() ProcessOptions {
	o.responseFiles = false
	o.responseOptions = ResponseFileOptions{}
	return o
}

// Policy returns the configured normalized Runtime Policy
func (o ProcessOptions) Policy() RuntimePolicy { return o.policy.normalized() }

// ValueResolver returns the configured application Value Resolver
func (o ProcessOptions) ValueResolver() ValueResolver { return o.valueResolver }

// ResponseFileOptions returns configured options and whether expansion is enabled
func (o ProcessOptions) ResponseFileOptions() (ResponseFileOptions, bool) {
	return o.responseOptions, o.responseFiles
}

// Outcome is the result of one command handler
type Outcome struct {
	status ExitStatus
}

// Success constructs a successful Outcome
func Success() Outcome { return Outcome{status: StatusSuccess} }

// NewOutcome constructs an Outcome with an explicit status
func NewOutcome(status ExitStatus) Outcome { return Outcome{status: status} }

// Status returns the process status
func (o Outcome) Status() ExitStatus { return o.status }

// Handler executes one validated Invocation with the selected leaf as current
// scope
type Handler func(context *Context, invocation *Invocation) (Outcome, error)

// Run parses and executes arguments through an injected Context
func (c *Command) Run(context *Context, arguments []string) (Outcome, error) {
	return c.RunWithPolicy(context, arguments, DefaultRuntimePolicy())
}

// RunWithPolicy parses and executes arguments through an explicit Runtime Policy
func (c *Command) RunWithPolicy(
	context *Context,
	arguments []string,
	policy RuntimePolicy,
) (Outcome, error) {
	if context == nil {
		return Outcome{}, errors.New("nagi cli: nil Context")
	}
	policy = policy.normalized()
	if err := c.Validate(); err != nil {
		return renderError(context.stderr, err, policy)
	}
	expanded := append([]string(nil), arguments...)
	if context.responseFiles {
		var err error
		expanded, err = ExpandResponseFiles(
			expanded,
			context.currentDirectory,
			context.responseOptions,
			context.responseReader,
			context.stdin,
		)
		if err != nil {
			return renderError(context.stderr, err, policy)
		}
	}
	result, err := c.parseValidatedWithOptionalValueResolver(
		expanded,
		context.EnvironmentValues(),
		context.valueResolver,
	)
	if err != nil {
		return renderError(context.stderr, err, policy)
	}
	return c.RunParsedWithPolicy(context, result, policy)
}

// RunParsed executes a result parsed by this Command Graph through the default
// policy
func (c *Command) RunParsed(context *Context, result ParseResult) (Outcome, error) {
	return c.RunParsedWithPolicy(context, result, DefaultRuntimePolicy())
}

// RunParsedWithPolicy executes a result parsed by this Command Graph through a
// policy
//
// This is the staged-adoption bridge between parser-only dispatch and Nagi
// Help, version, or registered Handler execution. A result whose canonical or
// stable path does not identify the same graph is rejected
func (c *Command) RunParsedWithPolicy(
	context *Context,
	result ParseResult,
	policy RuntimePolicy,
) (Outcome, error) {
	if context == nil {
		return Outcome{}, errors.New("nagi cli: nil Context")
	}
	policy = policy.normalized()
	if !equalPath(
		c.commandIDPathAtPath(result.CommandPath()),
		result.CommandIDPath(),
	) {
		return Outcome{}, errors.New("nagi cli: ParseResult does not belong to this Command Graph")
	}
	switch result.Kind() {
	case ParseHelp:
		document, err := c.HelpDocument(result.CommandPath())
		if err != nil {
			return Outcome{}, err
		}
		help := policy.helpRenderer.RenderHelp(document)
		if err := writeString(context.stdout, help); err != nil {
			return Outcome{}, err
		}
		return Success(), nil
	case ParseVersion:
		if err := writeString(context.stdout, c.name+" "+result.Version()+"\n"); err != nil {
			return Outcome{}, err
		}
		return Success(), nil
	case ParseInvocation:
		if result.Invocation() == nil {
			return Outcome{}, errors.New("nagi cli: ParseInvocation has no Invocation")
		}
		return c.RunInvocationWithPolicy(context, result.Invocation(), policy)
	default:
		return Outcome{}, errors.New("nagi cli: unknown parse result")
	}
}

// RunInvocation executes one Invocation validated by this Command Graph
//
// An Invocation whose canonical or stable command path does not identify the
// same graph is rejected
func (c *Command) RunInvocation(
	context *Context,
	invocation *Invocation,
) (Outcome, error) {
	return c.RunInvocationWithPolicy(context, invocation, DefaultRuntimePolicy())
}

// RunInvocationWithPolicy executes one Invocation validated by this Command
// Graph through a policy
//
// An Invocation whose canonical or stable command path does not identify the
// same graph is rejected
func (c *Command) RunInvocationWithPolicy(
	context *Context,
	invocation *Invocation,
	policy RuntimePolicy,
) (Outcome, error) {
	if context == nil {
		return Outcome{}, errors.New("nagi cli: nil Context")
	}
	if invocation == nil {
		return Outcome{}, errors.New("nagi cli: nil Invocation")
	}
	policy = policy.normalized()
	if !equalPath(
		c.commandIDPathAtPath(invocation.CommandPath()),
		invocation.CommandIDPath(),
	) {
		return Outcome{}, errors.New("nagi cli: Invocation does not belong to this Command Graph")
	}
	if context.cancellation.Err() != nil {
		return NewOutcome(policy.exitCodes.StatusFor(CategoryCancellation)), nil
	}
	if policy.deprecationNoticeRenderer != nil {
		for _, notice := range invocation.deprecationNotices {
			if err := writeString(
				context.stderr,
				policy.deprecationNoticeRenderer.RenderDeprecationNotice(notice),
			); err != nil {
				return Outcome{}, err
			}
		}
	}
	command := c.commandAtPath(invocation.CommandPath())
	if command == nil {
		return Outcome{}, errors.New("nagi cli: validated invocation has no command")
	}
	if command.handler == nil {
		diagnostic := NewDiagnostic(
			CodeMissingHandler,
			"command '"+command.name+"' has no handler",
		)
		diagnostic.WithCommandPath(invocation.CommandPath())
		return renderDiagnostic(context.stderr, diagnostic, policy)
	}
	outcome, err := command.handler(context, invocation)
	if err != nil {
		var diagnostic *Diagnostic
		if errors.As(err, &diagnostic) {
			diagnostic.
				withDefaultTargetPath(invocation.ValueScopeIDPath()).
				mapTargets(invocation.markSensitiveTarget).
				WithCommandPath(invocation.CommandPath())
			if diagnostic.Category() == CategoryUsage {
				diagnostic.WithUsage(c.usageForPath(invocation.CommandPath()))
			}
			return renderDiagnostic(context.stderr, diagnostic, policy)
		}
		return renderError(context.stderr, err, policy)
	}
	if context.cancellation.Err() != nil && outcome.Status() == StatusSuccess {
		return NewOutcome(policy.exitCodes.StatusFor(CategoryCancellation)), nil
	}
	return outcome, nil
}

// RunProcess executes this command against the current process and returns its status
func (c *Command) RunProcess() (ExitStatus, error) {
	return c.RunProcessWithOptions(DefaultProcessOptions())
}

// RunProcessWithValueResolver executes this command against the current
// process with an application-owned Value Resolver
func (c *Command) RunProcessWithValueResolver(resolver ValueResolver) (ExitStatus, error) {
	return c.RunProcessWithOptions(DefaultProcessOptions().WithValueResolver(resolver))
}

// RunProcessWithPolicy executes this command with an explicit Runtime Policy
func (c *Command) RunProcessWithPolicy(policy RuntimePolicy) (ExitStatus, error) {
	return c.RunProcessWithOptions(DefaultProcessOptions().WithPolicy(policy))
}

// RunProcessWithPolicyAndValueResolver executes this command against the
// current process with an explicit Runtime Policy and Value Resolver
func (c *Command) RunProcessWithPolicyAndValueResolver(
	policy RuntimePolicy,
	resolver ValueResolver,
) (ExitStatus, error) {
	return c.RunProcessWithOptions(
		DefaultProcessOptions().WithPolicy(policy).WithValueResolver(resolver),
	)
}

// RunProcessWithOptions executes this command against the current process with
// composable Runtime, Value Resolver, and Response File options
func (c *Command) RunProcessWithOptions(options ProcessOptions) (ExitStatus, error) {
	cancellation, stop := signal.NotifyContext(stdcontext.Background(), os.Interrupt)
	defer stop()
	currentDirectory, err := os.Getwd()
	if err != nil {
		return StatusFailure, err
	}
	context := NewContextWithCancellation(
		os.Stdin,
		os.Stdout,
		os.Stderr,
		processEnvironment(os.Environ()),
		currentDirectory,
		cancellation,
	)
	context.WithValueResolver(options.valueResolver)
	if options.responseFiles {
		context.WithResponseFiles(options.responseOptions, FilesystemResponseFileReader{})
	}
	outcome, err := c.RunWithPolicy(context, os.Args[1:], options.policy)
	if err != nil {
		return StatusFailure, err
	}
	return outcome.Status(), nil
}

func renderError(writer io.Writer, err error, policy RuntimePolicy) (Outcome, error) {
	var diagnostic *Diagnostic
	if !errors.As(err, &diagnostic) {
		diagnostic = NewDiagnostic(CodeHandlerError, err.Error())
	}
	return renderDiagnostic(writer, diagnostic, policy)
}

func renderDiagnostic(
	writer io.Writer,
	diagnostic *Diagnostic,
	policy RuntimePolicy,
) (Outcome, error) {
	if err := writeString(writer, policy.diagnosticRenderer.RenderDiagnostic(diagnostic)); err != nil {
		return Outcome{}, err
	}
	return NewOutcome(policy.exitCodes.StatusFor(diagnostic.Category())), nil
}

func writeString(writer io.Writer, value string) error {
	written, err := io.WriteString(writer, value)
	if err != nil {
		return err
	}
	if written != len(value) {
		return io.ErrShortWrite
	}
	return nil
}

func processEnvironment(entries []string) map[string]string {
	environment := make(map[string]string, len(entries))
	for _, entry := range entries {
		name, value, found := strings.Cut(entry, "=")
		if found {
			environment[name] = value
		}
	}
	return environment
}
