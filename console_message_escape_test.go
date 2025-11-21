package pslog

import (
	"strings"
	"testing"
)

// Regression: console emitters must escape control/ANSI bytes in the unquoted
// message while leaving printable unicode untouched.
func TestConsoleMessageEscape(t *testing.T) {
	msg := "line\nwith\tesc" + string([]byte{0x1b}) + "[31mred"
	want := "line\\nwith\\tesc\\x1b[31mred"

	cases := []struct {
		name string
		opts Options
		strip func(string) string
	}{
		{name: "console_plain", opts: Options{Mode: ModeConsole, DisableTimestamp: true, NoColor: true}},
		{name: "console_color", opts: Options{Mode: ModeConsole, DisableTimestamp: true, ForceColor: true}, strip: stripANSIString},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf strings.Builder
			logger := NewWithOptions(&buf, tc.opts)
			logger.Info(msg)

			out := strings.TrimSpace(buf.String())
			if tc.strip != nil {
				out = tc.strip(out)
			}
			if !strings.Contains(out, want) {
				t.Fatalf("got %q, want message containing %q", out, want)
			}
			if strings.Contains(out, "\n") || strings.Contains(out, "\t") || strings.Contains(out, "\x1b") {
				t.Fatalf("raw control bytes leaked: %q", out)
			}
		})
	}
}
