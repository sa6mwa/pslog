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

func TestWritePTJSONStringEscapesSingleQuoteChunkPath(t *testing.T) {
	input := "aaaaaaaa'bbbbbbbbbbbbbbbbbbbbbbbb"
	out := renderJSONStringForTest(input)
	want := `"` + strings.ReplaceAll(input, "'", `\u0027`) + `"`
	if out != want {
		t.Fatalf("single quote chunk path mismatch: got %q want %q", out, want)
	}
}

func TestWritePTJSONStringEscapesLessThanChunkPath(t *testing.T) {
	input := "aaaaaaaa<bbbbbbbbbbbbbbbbbbbbbbbb"
	out := renderJSONStringForTest(input)
	want := `"` + strings.ReplaceAll(input, "<", `\u003c`) + `"`
	if out != want {
		t.Fatalf("less-than chunk path mismatch: got %q want %q", out, want)
	}
}

func TestWritePTJSONStringChunkVsTailParity(t *testing.T) {
	cases := []struct {
		name string
		char byte
		esc  string
	}{
		{name: "single_quote", char: '\'', esc: `\u0027`},
		{name: "less_than", char: '<', esc: `\u003c`},
	}

	const totalLen = 34
	positions := []int{0, 7, 8, 15, 16, 23, 24, 31, 33}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, pos := range positions {
				b := bytes.Repeat([]byte{'a'}, totalLen)
				b[pos] = tc.char
				out := renderJSONStringForTest(string(b))
				if strings.ContainsRune(out, rune(tc.char)) {
					t.Fatalf("raw %q leaked at pos %d in %q", tc.char, pos, out)
				}
				if strings.Count(out, tc.esc) != 1 {
					t.Fatalf("expected one %s escape at pos %d, got %q", tc.esc, pos, out)
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
