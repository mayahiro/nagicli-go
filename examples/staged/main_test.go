package main

import (
	"bytes"
	"testing"

	cli "github.com/mayahiro/nagicli-go"
)

func TestStagedApplication(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	context := cli.NewContext(&bytes.Buffer{}, stdout, stderr, nil, "/")
	status, err := run([]string{"inspect", "page"}, context)
	if err != nil {
		t.Fatal(err)
	}
	if status != cli.StatusSuccess || stdout.String() != "legacy inspect: page\n" {
		t.Fatalf("status = %d, stdout = %q", status, stdout.String())
	}
}

func TestStagedApplicationRendersParserDiagnostic(t *testing.T) {
	stderr := &bytes.Buffer{}
	context := cli.NewContext(&bytes.Buffer{}, &bytes.Buffer{}, stderr, nil, "/")
	status, err := run([]string{"inspect", "blocked"}, context)
	if err != nil {
		t.Fatal(err)
	}
	if status != cli.StatusFailure {
		t.Fatalf("status = %d", status)
	}
	if got := stderr.String(); got != "error[target-blocked]: target 'blocked' cannot be inspected\nhint: choose another target\nusage: tool inspect [OPTIONS] <TARGET>\n" {
		t.Fatalf("stderr = %q", got)
	}
}
