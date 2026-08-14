package main

import (
	"testing"

	cli "github.com/mayahiro/nagicli-go"
	"github.com/mayahiro/nagicli-go/clitest"
)

func TestResponseFileApplication(t *testing.T) {
	result, err := clitest.New(application()).
		Arguments("@arguments.txt").
		ResponseFiles(
			cli.ResponseFileOptions{},
			cli.ResponseFileReaderFunc(func(request cli.ResponseFileReadRequest) ([]byte, *cli.Diagnostic) {
				if request.Path() != "/arguments.txt" {
					t.Fatalf("path = %q", request.Path())
				}
				return []byte("--profile workspace --tag alpha --tag \"release candidate\""), nil
			}),
		).
		Run()
	if err != nil {
		t.Fatal(err)
	}
	if result.Status() != cli.StatusSuccess {
		t.Fatalf("status = %d, want %d", result.Status(), cli.StatusSuccess)
	}
	want := "profile=workspace\ntags=alpha|release candidate\n"
	if got := string(result.Stdout()); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}
