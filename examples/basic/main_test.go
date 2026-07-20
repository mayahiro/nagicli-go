package main

import (
	"testing"

	cli "github.com/mayahiro/nagicli-go"
	"github.com/mayahiro/nagicli-go/clitest"
)

func TestApplication(t *testing.T) {
	result, err := clitest.New(application()).Arguments("Nagi").Run()
	if err != nil {
		t.Fatal(err)
	}
	if result.Status() != cli.StatusSuccess {
		t.Fatalf("status = %d, want %d", result.Status(), cli.StatusSuccess)
	}
	if got := string(result.Stdout()); got != "Hello, Nagi!\n" {
		t.Fatalf("stdout = %q", got)
	}
	if got := result.Stderr(); len(got) != 0 {
		t.Fatalf("stderr = %q", got)
	}
}
