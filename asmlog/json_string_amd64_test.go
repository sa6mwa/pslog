//go:build amd64

package asmlog

import (
	"strconv"
	"testing"
)

func TestAppendJSONStringASMAndFallback(t *testing.T) {
	cases := []string{
		"",
		"plain",
		"needs \"quotes\" and \\slashes\\",
		"controls \n\r\t\b\f",
		"value\x7fdel",
		"\x1b",
		"emoji 😃",
	}

	tests := []struct {
		name    string
		enabled bool
	}{
		{"fallback", false},
		{"asm", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := setJSONStringASM(tt.enabled && asmJSONStringAvailable())
			for _, input := range cases {
				out := appendJSONString(nil, input)
				want := []byte(strconv.Quote(input))
				if string(out) != string(want) {
					t.Fatalf("mismatch (asm=%v) for %q: got %q want %q", tt.enabled, input, string(out), string(want))
				}
			}
			restore()
		})
	}
}
