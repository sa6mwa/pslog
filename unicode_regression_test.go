package pslog

import (
	"strings"
	"testing"
)

// Regression: printable Unicode code points should flow through all emitters
// without being hex- or \u-escaped.
func TestUnicodePrintableAllModes(t *testing.T) {
	samples := []struct {
		name  string
		value string
	}{
		{"micro_sign", "µ"},   // U+00B5
		{"greek_mu", "μ"},     // U+03BC
		{"degree", "°"},       // U+00B0
		{"plus_minus", "±"},   // U+00B1
		{"multiplication", "×"}, // U+00D7
		{"division", "÷"},     // U+00F7
		{"bullet", "•"},       // U+2022
		{"endash", "–"},       // U+2013
		{"emdash", "—"},       // U+2014
		{"ellipsis", "…"},     // U+2026
		{"arrow_left", "←"},   // U+2190
		{"arrow_right", "→"},  // U+2192
		{"arrow_up", "↑"},     // U+2191
		{"arrow_down", "↓"},   // U+2193
		{"euro", "€"},         // U+20AC
		{"pound", "£"},        // U+00A3
		{"yen", "¥"},          // U+00A5
		{"omega", "Ω"},        // U+03A9
		{"emoji_check", "✅"},  // U+2705 (4-byte UTF-8)
		{"emoji_rocket", "🚀"}, // U+1F680 (4-byte UTF-8)
	}

	modes := []struct {
		name   string
		opts   Options
		expect func(string) string
		strip  func(string) string
	}{
		{
			name: "console_plain",
			opts: Options{Mode: ModeConsole, DisableTimestamp: true, NoColor: true},
			expect: func(v string) string { return "value=" + v },
		},
		{
			name:  "console_color",
			opts:  Options{Mode: ModeConsole, DisableTimestamp: true, ForceColor: true},
			strip: stripANSIString,
			expect: func(v string) string { return "value=" + v },
		},
		{
			name: "json_plain",
			opts: Options{Mode: ModeStructured, DisableTimestamp: true, NoColor: true},
			expect: func(v string) string { return `"value":"` + v + `"` },
		},
		{
			name:  "json_color",
			opts:  Options{Mode: ModeStructured, DisableTimestamp: true, ForceColor: true},
			strip: stripANSIString,
			expect: func(v string) string { return `"value":"` + v + `"` },
		},
	}

	for _, sample := range samples {
		for _, mode := range modes {
			t.Run(sample.name+"_"+mode.name, func(t *testing.T) {
				var buf strings.Builder
				logger := NewWithOptions(&buf, mode.opts)
				logger.Info("event", "value", sample.value)

				out := strings.TrimSpace(buf.String())
				if mode.strip != nil {
					out = mode.strip(out)
				}

				if want := mode.expect(sample.value); !strings.Contains(out, want) {
					t.Fatalf("output missing %q: %q", want, out)
				}
				if strings.Contains(out, "\\x") || strings.Contains(out, "\\u00") || strings.Contains(out, "\\ud8") {
					t.Fatalf("output still contains escape sequences: %q", out)
				}
			})
		}
	}
}
