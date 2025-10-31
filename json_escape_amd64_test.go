//go:build amd64

package pslog

import (
	"encoding/json"
	"io"
	"testing"

	"golang.org/x/sys/cpu"
)

func TestAppendEscapedStringContentVariants(t *testing.T) {
	cases := []string{
		"",
		"plain ascii text",
		"needs \"quotes\" and \\slashes\\",
		"controls \b\f\n\r\t here",
		"\x1b escape",
		"value\x7fdel",
		"emoji 😃 utf8",
	}

	run := func(name string, enableAVX2 bool) {
		t.Run(name, func(t *testing.T) {
			orig := cpu.X86.HasAVX2
			cpu.X86.HasAVX2 = enableAVX2
			defer func() { cpu.X86.HasAVX2 = orig }()

			for _, input := range cases {
				lw := acquireLineWriter(io.Discard)
				lw.autoFlush = false
				writePTJSONString(lw, input)
				got := string(lw.buf)
				releaseLineWriter(lw)

				wantBytes, err := json.Marshal(input)
				if err != nil {
					t.Fatalf("json.Marshal(%q) failed: %v", input, err)
				}
				want := string(wantBytes)
				if got != want {
					t.Fatalf("writePTJSONString mismatch with HasAVX2=%v for %q: got %q want %q", enableAVX2, input, got, want)
				}
			}
		})
	}

	// Always exercise the non-AVX2 path first.
	run("no_avx2", false)
	if cpu.X86.HasAVX2 {
		run("avx2", true)
	}
}
