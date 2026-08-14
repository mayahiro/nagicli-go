package cli

import (
	"strings"

	nagitext "github.com/mayahiro/nagi-go/text"
)

// JSONDiagnosticSchema is the stable schema identifier emitted by JSONDiagnosticRenderer
const JSONDiagnosticSchema = "nagi.cli.diagnostic.v1"

// JSONDiagnosticRenderer renders one structured Diagnostic as a stable
// newline-delimited JSON object
//
// Its zero value is ready to use. Object member order, string escaping,
// nullability, and the final newline are part of the public format contract
type JSONDiagnosticRenderer struct{}

// RenderDiagnostic renders one compact JSON object with one final newline
func (JSONDiagnosticRenderer) RenderDiagnostic(diagnostic *Diagnostic) string {
	var output strings.Builder
	output.Grow(estimatedJSONDiagnosticCapacity(diagnostic))
	output.WriteString(`{"schema":`)
	writeJSONString(&output, JSONDiagnosticSchema)
	output.WriteString(`,"code":`)
	writeJSONString(&output, string(diagnostic.code))
	output.WriteString(`,"category":`)
	writeJSONString(&output, string(diagnostic.category))
	output.WriteString(`,"message":`)
	writeJSONString(&output, diagnostic.message)
	output.WriteString(`,"command_path":`)
	writeJSONStringArray(&output, diagnostic.commandPath)
	output.WriteString(`,"usage":`)
	if diagnostic.usageSet {
		writeJSONString(&output, diagnostic.usage)
	} else {
		output.WriteString("null")
	}
	output.WriteString(`,"targets":[`)
	for index, target := range diagnostic.targets {
		if index != 0 {
			output.WriteByte(',')
		}
		writeJSONDiagnosticTarget(&output, target)
	}
	output.WriteString(`],"hints":`)
	writeJSONStringArray(&output, diagnostic.hints)
	output.WriteString("}\n")
	return output.String()
}

func writeJSONDiagnosticTarget(output *strings.Builder, target DiagnosticTarget) {
	output.WriteString(`{"kind":`)
	writeJSONString(output, string(target.kind))
	output.WriteString(`,"command_id_path":`)
	writeJSONStringArray(output, target.commandIDPath)
	output.WriteString(`,"value_id":`)
	writeJSONString(output, target.valueID)
	output.WriteByte('}')
}

func writeJSONStringArray(output *strings.Builder, values []string) {
	output.WriteByte('[')
	for index, value := range values {
		if index != 0 {
			output.WriteByte(',')
		}
		writeJSONString(output, value)
	}
	output.WriteByte(']')
}

func writeJSONString(output *strings.Builder, value string) {
	value = nagitext.NormalizeUTF8(value)
	output.WriteByte('"')
	copied := 0
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character > 0x1f && character != '"' && character != '\\' {
			continue
		}
		output.WriteString(value[copied:index])
		switch character {
		case '"':
			output.WriteString(`\"`)
		case '\\':
			output.WriteString(`\\`)
		case '\b':
			output.WriteString(`\b`)
		case '\t':
			output.WriteString(`\t`)
		case '\n':
			output.WriteString(`\n`)
		case '\f':
			output.WriteString(`\f`)
		case '\r':
			output.WriteString(`\r`)
		default:
			const lowerHex = "0123456789abcdef"
			output.WriteString(`\u00`)
			output.WriteByte(lowerHex[character>>4])
			output.WriteByte(lowerHex[character&0x0f])
		}
		copied = index + 1
	}
	output.WriteString(value[copied:])
	output.WriteByte('"')
}

func estimatedJSONDiagnosticCapacity(diagnostic *Diagnostic) int {
	const structure = 160
	capacity := saturatingAddJSONCapacity(
		structure,
		len(JSONDiagnosticSchema),
		len(diagnostic.code),
		len(diagnostic.category),
		len(diagnostic.message),
	)
	for _, value := range diagnostic.commandPath {
		capacity = saturatingAddJSONCapacity(capacity, len(value), 3)
	}
	if diagnostic.usageSet {
		capacity = saturatingAddJSONCapacity(capacity, len(diagnostic.usage), 2)
	}
	for _, target := range diagnostic.targets {
		capacity = saturatingAddJSONCapacity(capacity, len(target.valueID), 64)
		for _, value := range target.commandIDPath {
			capacity = saturatingAddJSONCapacity(capacity, len(value), 3)
		}
	}
	for _, hint := range diagnostic.hints {
		capacity = saturatingAddJSONCapacity(capacity, len(hint), 3)
	}
	return capacity
}

func saturatingAddJSONCapacity(values ...int) int {
	const maxInt = int(^uint(0) >> 1)
	result := 0
	for _, value := range values {
		if value > maxInt-result {
			return maxInt
		}
		result += value
	}
	return result
}

var _ DiagnosticRenderer = JSONDiagnosticRenderer{}
