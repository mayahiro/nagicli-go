package main

import (
	"strings"
	"testing"

	cli "github.com/mayahiro/nagicli-go"
	"github.com/mayahiro/nagicli-go/clitest"
)

func TestSensitiveValueApplication(t *testing.T) {
	help, err := application().RenderHelp([]string{"sensitive-values"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(help, "demo-token") || !strings.Contains(help, cli.RedactedValue) {
		t.Fatalf("Help did not redact the configured default: %q", help)
	}

	result, err := clitest.New(application()).Arguments("--token", "test-token").Run()
	if err != nil {
		t.Fatal(err)
	}
	if result.Status() != cli.StatusSuccess {
		t.Fatalf("status = %d, want %d", result.Status(), cli.StatusSuccess)
	}
	if output := string(result.Stdout()); strings.Contains(output, "test-token") {
		t.Fatalf("handler output exposed the token: %q", output)
	}
}
