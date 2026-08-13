//go:build (linux || darwin) && (amd64 || arm64)

package status

import (
	"errors"
	"os"
	"testing"
)

func TestProcessIOCachesTerminalAndReadsCurrentWidth(t *testing.T) {
	file, err := os.OpenFile("/dev/null", os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	access := &fakeTerminalAccess{terminal: true, columns: 120}
	processIO := newProcessIO(file, access)
	if !processIO.IsTerminal() || access.terminalCalls != 1 {
		t.Fatalf("terminal = %t, calls = %d", processIO.IsTerminal(), access.terminalCalls)
	}
	if width, ok := processIO.TerminalWidth(); !ok || width != 120 {
		t.Fatalf("width = %d, %t", width, ok)
	}
	access.columns = 90
	if width, ok := processIO.TerminalWidth(); !ok || width != 90 {
		t.Fatalf("resized width = %d, %t", width, ok)
	}
	if access.terminalCalls != 1 || access.widthCalls != 2 {
		t.Fatalf("terminal/width calls = %d/%d", access.terminalCalls, access.widthCalls)
	}
}

func TestProcessIOWidthFailureKeepsTerminalClassification(t *testing.T) {
	file, err := os.OpenFile("/dev/null", os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	access := &fakeTerminalAccess{terminal: true, widthErr: errors.New("size failed")}
	processIO := newProcessIO(file, access)
	if width, ok := processIO.TerminalWidth(); ok || width != 0 {
		t.Fatalf("width = %d, %t", width, ok)
	}
	if !processIO.IsTerminal() {
		t.Fatal("width failure changed terminal classification")
	}
}

type fakeTerminalAccess struct {
	terminal      bool
	columns       uint16
	widthErr      error
	terminalCalls int
	widthCalls    int
}

func (f *fakeTerminalAccess) isTerminal(int) bool {
	f.terminalCalls++
	return f.terminal
}

func (f *fakeTerminalAccess) width(int) (uint16, error) {
	f.widthCalls++
	return f.columns, f.widthErr
}
