package main

import (
	"strings"
	"testing"
)

func TestHelpExposesReplacementWithoutHiddenSyntax(t *testing.T) {
	help, err := application().RenderHelp([]string{"lifecycle"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(help, "[deprecated: use lifecycle run]") ||
		!strings.Contains(help, "[deprecated: use --verbose]") {
		t.Fatalf("Help lacks replacement metadata: %q", help)
	}
	if strings.Contains(help, "internal") {
		t.Fatalf("Help exposes hidden syntax: %q", help)
	}
}
