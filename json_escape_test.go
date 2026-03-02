package pslog

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
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

func TestWritePTJSONStringLeavesSingleQuoteAndLessThanUnescaped(t *testing.T) {
	lw := acquireLineWriter(io.Discard)
	writePTJSONString(lw, "can't <tag>")
	out := string(lw.buf)
	releaseLineWriter(lw)

	const want = "\"can't <tag>\""
	if out != want {
		t.Fatalf("expected single quote and less-than to stay unescaped: got %q want %q", out, want)
	}
}

func TestWritePTJSONStringSingleQuoteChunkPath(t *testing.T) {
	input := "aaaaaaaa'bbbbbbbbbbbbbbbbbbbbbbbb"
	out := renderJSONStringForTest(input)
	want := `"` + input + `"`
	if out != want {
		t.Fatalf("single quote chunk path mismatch: got %q want %q", out, want)
	}
}

func TestWritePTJSONStringLessThanChunkPath(t *testing.T) {
	input := "aaaaaaaa<bbbbbbbbbbbbbbbbbbbbbbbb"
	out := renderJSONStringForTest(input)
	want := `"` + input + `"`
	if out != want {
		t.Fatalf("less-than chunk path mismatch: got %q want %q", out, want)
	}
}

func TestWritePTJSONStringChunkVsTailParity(t *testing.T) {
	cases := []struct {
		name string
		char byte
	}{
		{name: "single_quote", char: '\''},
		{name: "less_than", char: '<'},
	}

	const totalLen = 34
	positions := []int{0, 7, 8, 15, 16, 23, 24, 31, 33}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, pos := range positions {
				b := bytes.Repeat([]byte{'a'}, totalLen)
				b[pos] = tc.char
				out := renderJSONStringForTest(string(b))
				if strings.Count(out, string(tc.char)) != 1 {
					t.Fatalf("expected one raw %q at pos %d, got %q", tc.char, pos, out)
				}
				if strings.Contains(out, `\u0027`) || strings.Contains(out, `\u003c`) {
					t.Fatalf("unexpected html-style escape at pos %d: %q", pos, out)
				}
			}
		})
	}
}

func renderJSONStringForTest(input string) string {
	lw := acquireLineWriter(io.Discard)
	writePTJSONString(lw, input)
	out := string(lw.buf)
	releaseLineWriter(lw)
	return out
}
