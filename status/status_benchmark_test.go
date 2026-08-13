package status

import (
	"io"
	"testing"
)

func TestStatusWarmedUpdatesDoNotAllocateOrGrow(t *testing.T) {
	for _, terminal := range []bool{false, true} {
		statusIO := &benchmarkIO{terminal: terminal, width: 80}
		reporter := New(statusIO)
		for tick := uint64(0); tick < 8; tick++ {
			if _, err := reporter.Update(NewSpinner(tick, "waiting")); err != nil {
				t.Fatal(err)
			}
		}
		capacity := cap(reporter.lastTerminal) + cap(reporter.lastLog) + cap(reporter.scratch)
		var tick uint64
		allocations := testing.AllocsPerRun(1_000, func() {
			tick++
			if _, err := reporter.Update(NewSpinner(tick, "waiting")); err != nil {
				t.Fatal(err)
			}
		})
		if allocations != 0 {
			t.Fatalf("terminal=%t allocations = %f", terminal, allocations)
		}
		if got := cap(reporter.lastTerminal) + cap(reporter.lastLog) + cap(reporter.scratch); got != capacity {
			t.Fatalf("terminal=%t retained capacity = %d, want %d", terminal, got, capacity)
		}
	}
}

func BenchmarkStatusTerminalUpdate(b *testing.B) {
	statusIO := &benchmarkIO{terminal: true, width: 80}
	reporter := New(statusIO)
	if _, err := reporter.Update(NewStatus("warming")); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for tick := range b.N {
		if _, err := reporter.Update(NewSpinner(uint64(tick), "waiting")); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStatusLogCoalesced(b *testing.B) {
	statusIO := &benchmarkIO{}
	reporter := New(statusIO)
	if _, err := reporter.Update(NewStatus("waiting")); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := reporter.Update(NewSpinner(0, "waiting")); err != nil {
			b.Fatal(err)
		}
	}
}

type benchmarkIO struct {
	terminal bool
	width    int
	bytes    uint64
}

func (b *benchmarkIO) Write(value []byte) (int, error) {
	b.bytes += uint64(len(value))
	return len(value), nil
}

func (*benchmarkIO) Flush() error { return nil }

func (b *benchmarkIO) IsTerminal() bool { return b.terminal }

func (b *benchmarkIO) TerminalWidth() (int, bool) { return b.width, b.width > 0 }

var _ IO = (*benchmarkIO)(nil)
var _ io.Writer = (*benchmarkIO)(nil)
