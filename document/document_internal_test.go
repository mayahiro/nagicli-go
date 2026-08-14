package document

import (
	"testing"
	"unicode/utf8"
)

func TestNormalizeTextReplacesInvalidUTF8AndControls(t *testing.T) {
	got := normalizeText("bad\xff\tvalue\r\nnext\rline")
	want := "bad\uFFFD\uFFFDvalue\nnext\nline"
	if !utf8.ValidString(got) || got != want {
		t.Fatalf("normalizeText() = %q, want %q", got, want)
	}
}
