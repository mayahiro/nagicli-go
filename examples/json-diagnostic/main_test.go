package main

import (
	"bytes"
	"encoding/json"
	"testing"

	cli "github.com/mayahiro/nagicli-go"
)

func TestRunWritesOneSchemaRecord(t *testing.T) {
	var output bytes.Buffer
	if err := run(&output); err != nil {
		t.Fatal(err)
	}
	var document struct {
		Schema string `json:"schema"`
		Code   string `json:"code"`
	}
	if err := json.Unmarshal(bytes.TrimSuffix(output.Bytes(), []byte{'\n'}), &document); err != nil {
		t.Fatal(err)
	}
	if document.Schema != cli.JSONDiagnosticSchema || document.Code != "profile-blocked" {
		t.Fatalf("document = %+v", document)
	}
}
