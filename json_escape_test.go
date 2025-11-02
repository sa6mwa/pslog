package pslog

import (
	"encoding/json"
	"io"
	"testing"
)

func TestWritePTJSONStringMatchesEncodingJSON(t *testing.T) {
	cases := []string{
		"value",
		"value\x7fhere",
		"\x1b",
		"line\nbreak",
		"quote\"needed",
	}

	for _, input := range cases {
		lw := acquireLineWriter(io.Discard)
		writePTJSONString(lw, input)
		out := string(lw.buf)
		releaseLineWriter(lw)

		expectedBytes, err := json.Marshal(input)
		if err != nil {
			t.Fatalf("json.Marshal failed for %q: %v", input, err)
		}
		expected := string(expectedBytes)
		if out != expected {
			t.Fatalf("writePTJSONString mismatch for %q: got %q want %q", input, out, expected)
		}
	}
}

func TestWritePTJSONStringEscapesSingleQuote(t *testing.T) {
	lw := acquireLineWriter(io.Discard)
	writePTJSONString(lw, "can't")
	out := string(lw.buf)
	releaseLineWriter(lw)

	const want = "\"can\\u0027t\""
	if out != want {
		t.Fatalf("expected single quote to be escaped: got %q want %q", out, want)
	}
}
