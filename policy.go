package cli

import "fmt"

// ExitCodePolicy maps semantic Diagnostic categories to process statuses
type ExitCodePolicy struct {
	configured    bool
	specification ExitStatus
	usage         ExitStatus
	execution     ExitStatus
	cancellation  ExitStatus
	io            ExitStatus
}

// DefaultExitCodePolicy returns the portable Nagi status mapping
func DefaultExitCodePolicy() ExitCodePolicy {
	return ExitCodePolicy{
		configured:    true,
		specification: StatusUsage,
		usage:         StatusUsage,
		execution:     StatusFailure,
		cancellation:  StatusCancelled,
		io:            StatusFailure,
	}
}

// WithStatus returns a copy with one category mapping replaced
func (p ExitCodePolicy) WithStatus(category DiagnosticCategory, status ExitStatus) ExitCodePolicy {
	if !p.configured {
		p = DefaultExitCodePolicy()
	}
	switch category {
	case CategorySpecification:
		p.specification = status
	case CategoryUsage:
		p.usage = status
	case CategoryExecution:
		p.execution = status
	case CategoryCancellation:
		p.cancellation = status
	case CategoryIO:
		p.io = status
	}
	return p
}

// StatusFor returns the process status for a semantic category
func (p ExitCodePolicy) StatusFor(category DiagnosticCategory) ExitStatus {
	if !p.configured {
		p = DefaultExitCodePolicy()
	}
	switch category {
	case CategorySpecification:
		return p.specification
	case CategoryUsage:
		return p.usage
	case CategoryExecution:
		return p.execution
	case CategoryCancellation:
		return p.cancellation
	case CategoryIO:
		return p.io
	default:
		return StatusFailure
	}
}

// DiagnosticRenderer renders one structured Diagnostic
type DiagnosticRenderer interface {
	// RenderDiagnostic returns text with one final newline
	RenderDiagnostic(diagnostic *Diagnostic) string
}

// PlainDiagnosticRenderer renders stable plain Diagnostic text
type PlainDiagnosticRenderer struct {
	prefix    string
	showUsage bool
}

// DefaultPlainDiagnosticRenderer returns the standard Nagi renderer
func DefaultPlainDiagnosticRenderer() PlainDiagnosticRenderer {
	return PlainDiagnosticRenderer{prefix: "error", showUsage: true}
}

// WithPrefix returns a copy using prefix before the diagnostic code
func (r PlainDiagnosticRenderer) WithPrefix(prefix string) PlainDiagnosticRenderer {
	r.prefix = prefix
	return r
}

// WithUsage returns a copy that includes or omits available usage text
func (r PlainDiagnosticRenderer) WithUsage(show bool) PlainDiagnosticRenderer {
	r.showUsage = show
	return r
}

// RenderDiagnostic renders the configured plain format
func (r PlainDiagnosticRenderer) RenderDiagnostic(diagnostic *Diagnostic) string {
	result := fmt.Sprintf("%s[%s]: %s\n", r.prefix, diagnostic.code, diagnostic.message)
	if r.showUsage && diagnostic.usage != "" {
		result += "usage: " + diagnostic.usage + "\n"
	}
	return result
}

// RuntimePolicy selects rendering and category-to-status mapping
type RuntimePolicy struct {
	exitCodes          ExitCodePolicy
	helpRenderer       HelpRenderer
	diagnosticRenderer DiagnosticRenderer
}

// DefaultRuntimePolicy returns deterministic Nagi runtime behavior
func DefaultRuntimePolicy() RuntimePolicy {
	return RuntimePolicy{
		exitCodes:          DefaultExitCodePolicy(),
		helpRenderer:       PlainHelpRenderer{},
		diagnosticRenderer: DefaultPlainDiagnosticRenderer(),
	}
}

// WithExitCodePolicy returns a copy using the provided status mapping
func (p RuntimePolicy) WithExitCodePolicy(policy ExitCodePolicy) RuntimePolicy {
	p.exitCodes = policy
	return p
}

// WithHelpRenderer returns a copy using the provided Help renderer
func (p RuntimePolicy) WithHelpRenderer(renderer HelpRenderer) RuntimePolicy {
	p.helpRenderer = renderer
	return p
}

// WithDiagnosticRenderer returns a copy using the provided Diagnostic renderer
func (p RuntimePolicy) WithDiagnosticRenderer(renderer DiagnosticRenderer) RuntimePolicy {
	p.diagnosticRenderer = renderer
	return p
}

func (p RuntimePolicy) normalized() RuntimePolicy {
	defaults := DefaultRuntimePolicy()
	if !p.exitCodes.configured {
		p.exitCodes = defaults.exitCodes
	}
	if p.helpRenderer == nil {
		p.helpRenderer = defaults.helpRenderer
	}
	if p.diagnosticRenderer == nil {
		p.diagnosticRenderer = defaults.diagnosticRenderer
	}
	return p
}
