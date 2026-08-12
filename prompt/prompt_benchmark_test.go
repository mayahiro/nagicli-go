package prompt

import (
	"context"
	"io"
	"testing"
)

func BenchmarkPromptConfirm(b *testing.B) {
	promptIO := &benchmarkIO{response: []byte("yes")}
	request := NewConfirm("Proceed?")
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		value, err := New(promptIO).Confirm(ctx, request)
		if err != nil {
			b.Fatal(err)
		}
		if !value {
			b.Fatal("Confirm result = false")
		}
	}
}

func BenchmarkPromptInput64KiB(b *testing.B) {
	promptIO := &benchmarkIO{response: make([]byte, DefaultMaxInputBytes)}
	for index := range promptIO.response {
		promptIO.response[index] = 'a'
	}
	request := NewInput("Value")
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		value, err := New(promptIO).Input(ctx, request)
		if err != nil {
			b.Fatal(err)
		}
		if len(value) != DefaultMaxInputBytes {
			b.Fatalf("value bytes = %d", len(value))
		}
	}
}

type benchmarkIO struct {
	response []byte
}

func (*benchmarkIO) Write(value []byte) (int, error) { return len(value), nil }

func (*benchmarkIO) Flush() error { return nil }

func (*benchmarkIO) IsTerminal() bool { return true }

func (b *benchmarkIO) ReadLine(context.Context, InputMode, int) (ReadResult, error) {
	return NewLineResult(b.response), nil
}

var _ io.Writer = (*benchmarkIO)(nil)
