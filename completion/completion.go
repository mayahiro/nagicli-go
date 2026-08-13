// Package completion generates shell integrations for a Nagi CLI CompletionEngine
// and handles their reserved runtime protocol before normal command dispatch
package completion

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	cli "github.com/mayahiro/nagicli-go"
)

// ProtocolToken is the reserved argv token used by generated scripts
//
// It is outside the portable command-name grammar and should be intercepted
// before ordinary Command Graph parsing
const ProtocolToken = "__nagi_complete"

// Shell identifies one supported generated completion format
type Shell uint8

const (
	// Bash generates GNU Bash programmable completion
	Bash Shell = iota
	// Zsh generates Zsh compsys completion
	Zsh
	// Fish generates Fish command completion
	Fish
	// PowerShell generates PowerShell native argument completion
	PowerShell
)

// String returns the stable protocol spelling
func (s Shell) String() string {
	switch s {
	case Bash:
		return "bash"
	case Zsh:
		return "zsh"
	case Fish:
		return "fish"
	case PowerShell:
		return "powershell"
	default:
		return ""
	}
}

// ProtocolErrorKind classifies a completion protocol failure
type ProtocolErrorKind uint8

const (
	// ProtocolErrorInvalidRequest means protocol arguments are missing or invalid
	ProtocolErrorInvalidRequest ProtocolErrorKind = iota
	// ProtocolErrorCompletion means Core completion resolution failed
	ProtocolErrorCompletion
	// ProtocolErrorIO means candidate output could not be written
	ProtocolErrorIO
)

// ProtocolError is a protocol parsing, resolution, or output failure
type ProtocolError struct {
	kind    ProtocolErrorKind
	message string
	cause   error
}

// Kind returns the failure category
func (e *ProtocolError) Kind() ProtocolErrorKind { return e.kind }

// Error implements error
func (e *ProtocolError) Error() string { return "completion protocol failed: " + e.message }

// Unwrap exposes completion or I/O failures
func (e *ProtocolError) Unwrap() error { return e.cause }

// Generate returns one deterministic shell completion script
func Generate(shell Shell, engine *cli.CompletionEngine) (string, error) {
	if engine == nil || engine.RootName() == "" {
		return "", errors.New("nagi completion: nil CompletionEngine")
	}
	command := engine.RootName()
	function := strings.ReplaceAll(command, "-", "_")
	switch shell {
	case Bash:
		return generateBash(command, function), nil
	case Zsh:
		return generateZsh(command, function), nil
	case Fish:
		return generateFish(command, function), nil
	case PowerShell:
		return generatePowerShell(command), nil
	default:
		return "", fmt.Errorf("nagi completion: unsupported Shell %d", shell)
	}
}

// Handle handles a generated-script request before normal Command Graph parsing
//
// Arguments exclude the program name. A false handled result means the
// reserved protocol token was absent and ordinary dispatch should continue
func Handle(
	ctx context.Context,
	engine *cli.CompletionEngine,
	arguments []string,
	output io.Writer,
) (handled bool, err error) {
	if len(arguments) == 0 || arguments[0] != ProtocolToken {
		return false, nil
	}
	if len(arguments) < 3 {
		return true, protocolError(ProtocolErrorInvalidRequest, "expected shell and current-token arguments", nil)
	}
	shell, ok := parseShell(arguments[1])
	if !ok {
		return true, protocolError(ProtocolErrorInvalidRequest, "unsupported shell identifier", nil)
	}
	if output == nil {
		return true, protocolError(ProtocolErrorIO, "nil output Writer", nil)
	}
	result, completionErr := engine.Complete(
		ctx,
		cli.NewCompletionInput(arguments[3:], arguments[2]),
	)
	if completionErr != nil {
		return true, protocolError(ProtocolErrorCompletion, completionErr.Error(), completionErr)
	}
	if writeErr := writeProtocol(shell, result.Candidates(), output); writeErr != nil {
		return true, protocolError(ProtocolErrorIO, writeErr.Error(), writeErr)
	}
	return true, nil
}

func parseShell(value string) (Shell, bool) {
	switch value {
	case "bash":
		return Bash, true
	case "zsh":
		return Zsh, true
	case "fish":
		return Fish, true
	case "powershell":
		return PowerShell, true
	default:
		return 0, false
	}
}

func writeProtocol(shell Shell, candidates []cli.CompletionCandidate, output io.Writer) error {
	for _, candidate := range candidates {
		description := candidate.Description()
		if description == "" {
			description = candidate.DisplayLabel()
		}
		if deprecation, ok := candidate.Deprecation(); ok {
			description += " [deprecated: use " + deprecation.Replacement() + "]"
		}
		if shell == Fish {
			if _, err := fmt.Fprintf(output, "%s\t%s\n", candidate.Value(), description); err != nil {
				return err
			}
			continue
		}
		appendPolicy := "none"
		if candidate.AppendSpace() {
			appendPolicy = "space"
		}
		if _, err := fmt.Fprintf(
			output,
			"%s\t%s\t%s\t%s\t%s\n",
			candidate.Value(),
			candidate.DisplayLabel(),
			description,
			candidateKind(candidate.Kind()),
			appendPolicy,
		); err != nil {
			return err
		}
	}
	return nil
}

func candidateKind(kind cli.CompletionCandidateKind) string {
	switch kind {
	case cli.CompletionCandidateCommand:
		return "command"
	case cli.CompletionCandidateOption:
		return "option"
	case cli.CompletionCandidateValue:
		return "value"
	default:
		return "value"
	}
}

func protocolError(kind ProtocolErrorKind, message string, cause error) *ProtocolError {
	return &ProtocolError{kind: kind, message: message, cause: cause}
}

func generateBash(command, function string) string {
	return fmt.Sprintf(`# Nagi completion for %[1]s
_nagi_completion_dequote_%[2]s() {
    local LC_ALL=C
    local input="$1"
    local output=""
    local quote=""
    local character next
    local index=0
    local length=${#input}

    while (( index < length )); do
        character="${input:index:1}"
        if [[ -z "$quote" ]]; then
            case "$character" in
                "'") quote="single" ;;
                '"') quote="double" ;;
                \\)
                    if (( index + 1 < length )); then
                        ((index++))
                        output+="${input:index:1}"
                    else
                        output+="$character"
                    fi
                    ;;
                *) output+="$character" ;;
            esac
        elif [[ "$quote" == "single" ]]; then
            if [[ "$character" == "'" ]]; then
                quote=""
            else
                output+="$character"
            fi
        elif [[ "$character" == '"' ]]; then
            quote=""
        elif [[ "$character" == \\ ]] && (( index + 1 < length )); then
            next="${input:index+1:1}"
            if [[ "$next" == '$' || "$next" == $'\x60' || "$next" == '"' || "$next" == \\ ]]; then
                output+="$next"
                ((index++))
            elif [[ "$next" == $'\n' ]]; then
                ((index++))
            else
                output+="$character"
            fi
        else
            output+="$character"
        fi
        ((index++))
    done
    REPLY="$output"
    NAGI_COMPLETION_QUOTE_REPLY="$quote"
}

_nagi_completion_escape_%[2]s() {
    local input="$1"
    local quote="$2"
    local output=""
    local character
    local index=0

    if [[ -z "$quote" ]]; then
        printf -v output '%%q' "$input"
    else
        while (( index < ${#input} )); do
            character="${input:index:1}"
            if [[ "$quote" == "double" && ( "$character" == '$' || "$character" == $'\x60' || "$character" == '"' || "$character" == \\ ) ]]; then
                output+="\\$character"
            elif [[ "$quote" == "single" && "$character" == "'" ]]; then
                output+="'\\''"
            else
                output+="$character"
            fi
            ((index++))
        done
    fi

    REPLY="$output"
}

_nagi_completion_current_%[2]s() {
    local LC_ALL=C
    local full="${COMP_WORDS[COMP_CWORD]}"
    local length=${#full}
    local point=$COMP_POINT
    local start=$(( point - length ))
    local raw="$full"

    (( start < 0 )) && start=0
    while (( start <= point )); do
        if (( point <= start + length )) && [[ "${COMP_LINE:start:length}" == "$full" ]]; then
            raw="${COMP_LINE:start:point-start}"
            break
        fi
        ((start++))
    done
    _nagi_completion_dequote_%[2]s "$raw"
}

_nagi_completion_%[2]s() {
    local current
    local -a completed=()
    local value label description kind append escaped
    local index
    local REPLY
    local NAGI_COMPLETION_QUOTE_REPLY
    local current_quote

    _nagi_completion_current_%[2]s
    current="$REPLY"
    current_quote="$NAGI_COMPLETION_QUOTE_REPLY"
    for (( index = 1; index < COMP_CWORD; index++ )); do
        _nagi_completion_dequote_%[2]s "${COMP_WORDS[index]}"
        completed+=("$REPLY")
    done

    COMPREPLY=()
    while IFS=$'\t' read -r value label description kind append; do
        [[ -z "$value" ]] && continue
        _nagi_completion_escape_%[2]s "$value" "$current_quote"
        escaped="$REPLY"
        COMPREPLY+=("$escaped")
    done < <(command %[1]s %[3]s bash "$current" "${completed[@]}" 2>/dev/null)
}
complete -o nospace -F _nagi_completion_%[2]s %[1]s
`, command, function, ProtocolToken)
}

func generateZsh(command, function string) string {
	return fmt.Sprintf(`#compdef %[1]s
# Nagi completion for %[1]s
_nagi_completion_%[2]s() {
    local current="$PREFIX"
    local -a completed=()
    local value label description kind append

    if (( CURRENT > 2 )); then
        completed=("${(@)words[2,CURRENT-1]}")
    fi

    while IFS=$'\t' read -r value label description kind append; do
        [[ -z "$value" ]] && continue
        if [[ "$append" == "space" ]]; then
            compadd -S ' ' -- "$value"
        else
            compadd -S '' -- "$value"
        fi
    done < <(command %[1]s %[3]s zsh "$current" "${completed[@]}" 2>/dev/null)
}
compdef _nagi_completion_%[2]s %[1]s
`, command, function, ProtocolToken)
}

func generateFish(command, function string) string {
	return fmt.Sprintf(`# Nagi completion for %[1]s
function __nagi_completion_%[2]s
    set -l tokens (commandline -xpc)
    set -l current (commandline -ct)
    if test (count $tokens) -gt 0
        set -e tokens[1]
    end
    command %[1]s %[3]s fish "$current" $tokens 2>/dev/null
end
complete -c %[1]s -f -a '(__nagi_completion_%[2]s)'
`, command, function, ProtocolToken)
}

func generatePowerShell(command string) string {
	return fmt.Sprintf(`# Nagi completion for %[1]s
Register-ArgumentCompleter -Native -CommandName '%[1]s' -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)

    $completed = @()
    $elements = @($commandAst.CommandElements)
    for ($index = 1; $index -lt $elements.Count; $index++) {
        $element = $elements[$index]
        if ($element.Extent.StartOffset -ge $cursorPosition) { break }
        if ($element.Extent.EndOffset -gt $cursorPosition) { break }
        if ($wordToComplete -ne '' -and $element.Extent.EndOffset -eq $cursorPosition) { break }
        if ($element -is [System.Management.Automation.Language.StringConstantExpressionAst]) {
            $completed += [string]$element.Value
        } else {
            $completed += $element.Extent.Text
        }
    }

    $lines = & '%[1]s' '%[2]s' 'powershell' $wordToComplete @completed 2>$null
    foreach ($line in $lines) {
        $fields = $line -split "%[3]ct", 5
        if ($fields.Count -ne 5 -or $fields[0] -eq '') { continue }
        $completionText = $fields[0]
        if ($completionText -notmatch '^[a-zA-Z0-9_./:=+@%%,-]+$') {
            $completionText = "'" + $completionText.Replace("'", "''") + "'"
        }
        if ($fields[4] -eq 'space') { $completionText += ' ' }
        $listItem = if ($fields[1] -ne '') { $fields[1] } else { $fields[0] }
        $toolTip = if ($fields[2] -ne '') { $fields[2] } else { $listItem }
        $resultType = switch ($fields[3]) {
            'command' { 'Command' }
            'option' { 'ParameterName' }
            default { 'ParameterValue' }
        }
        [System.Management.Automation.CompletionResult]::new(
            $completionText, $listItem, $resultType, $toolTip
        )
    }
}
`, command, ProtocolToken, '`')
}
