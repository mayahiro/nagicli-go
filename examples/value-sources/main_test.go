package main

import (
	"testing"

	cli "github.com/mayahiro/nagicli-go"
	"github.com/mayahiro/nagicli-go/clitest"
)

func TestValueSourceApplication(t *testing.T) {
	result, err := clitest.New(application()).ValueResolver(projectConfig).Run()
	if err != nil {
		t.Fatal(err)
	}
	if result.Status() != cli.StatusSuccess {
		t.Fatalf("status = %d, want %d", result.Status(), cli.StatusSuccess)
	}
	want := "profile=workspace:external(project-config)\n" +
		"tags=configured:external(project-config),baseline:default\n"
	if got := string(result.Stdout()); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}
