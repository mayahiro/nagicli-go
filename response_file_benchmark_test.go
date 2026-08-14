package cli_test

import (
	"bytes"
	"fmt"
	"testing"

	cli "github.com/mayahiro/nagicli-go"
)

const benchmarkResponseFileArgumentCount = 1_000

func benchmarkResponseFileSource(tokens int) []byte {
	var output bytes.Buffer
	for index := range tokens {
		if index != 0 {
			output.WriteByte(' ')
		}
		fmt.Fprintf(&output, "value-%d", index)
	}
	return output.Bytes()
}

func benchmarkResponseFileExpansion(b *testing.B, tokens int) {
	source := benchmarkResponseFileSource(tokens)
	reader := cli.ResponseFileReaderFunc(func(request cli.ResponseFileReadRequest) ([]byte, *cli.Diagnostic) {
		length := len(source)
		if length > request.ReadLimit() {
			length = request.ReadLimit()
		}
		return append([]byte(nil), source[:length]...), nil
	})
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		arguments, err := cli.ExpandResponseFiles(
			[]string{"@args.txt"},
			"/work",
			cli.ResponseFileOptions{},
			reader,
			nil,
		)
		if err != nil {
			b.Fatal(err)
		}
		if len(arguments) != tokens {
			b.Fatalf("arguments = %d, want %d", len(arguments), tokens)
		}
	}
}

func BenchmarkResponseFileSingle(b *testing.B) {
	benchmarkResponseFileExpansion(b, 1)
}

func BenchmarkResponseFile1000Tokens(b *testing.B) {
	benchmarkResponseFileExpansion(b, benchmarkResponseFileArgumentCount)
}

func BenchmarkResponseFile1000LiteralArguments(b *testing.B) {
	arguments := make([]string, benchmarkResponseFileArgumentCount)
	for index := range arguments {
		arguments[index] = fmt.Sprintf("value-%d", index)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		expanded, err := cli.ExpandResponseFiles(
			arguments,
			"/work",
			cli.ResponseFileOptions{},
			nil,
			nil,
		)
		if err != nil {
			b.Fatal(err)
		}
		if len(expanded) != len(arguments) {
			b.Fatalf("arguments = %d, want %d", len(expanded), len(arguments))
		}
	}
}
