// Package cli provides a validated command graph, scoped typed Invocations,
// structured Help and Diagnostics, and an injected policy-controlled runtime
// for native Go command applications.
//
// Options are local unless Inherited makes them visible in selected
// descendants. Every value remains in its declaration scope. Parent and child
// Commands may reuse local value IDs. Invocation access starts at a documented
// current scope, while Scope selects one exact stable command-ID path.
// RequireValueAs provides fallible schema-required typed access.
//
// Help-only Usage Variants and SubcommandUsage control presentation without
// changing parsing. InvocationValidator returns a structured Diagnostic with
// application codes, option or argument targets, and remediation hints.
// VisitHelpDocuments validates once and streams structured Help for every
// visible command. Package document owns optional deterministic Markdown and
// man rendering.
//
// OptionSpec.Sensitive and Argument.Sensitive attach generic presentation
// metadata. Framework Help, parser Diagnostics, formatting, and completion
// redact or suppress those values while explicit Invocation access preserves
// the original raw and typed data.
//
// ValueResolver adapts already loaded application configuration into selected
// Value Option fallbacks without giving Nagi ownership of its schema or I/O.
// Fixed precedence remains command line, environment, external resolver, then
// command-definition default.
//
// CompletionEngine snapshots the validated graph without handlers and resolves
// static candidates plus only the active Option or Argument provider. Package
// completion owns shell-specific generation and its reserved protocol.
//
// Parse, RunParsedWithPolicy, RunInvocationWithPolicy, and RuntimePolicy's pure
// rendering and status helpers support command-by-command adoption in an
// existing CLI. RunProcess remains the complete process integration.
package cli
