package pslog

import (
	"strings"
	"testing"
	"time"
)

// Regression: durations containing the micro sign (µ, UTF-8 C2 B5) must not be
// hex-escaped in any emitter (console/plain, console/color, JSON/plain, JSON/color).
func TestDurationMicroSignAllModes(t *testing.T) {
	dur := time.Duration(321678) * time.Nanosecond // 321.678µs

	cases := []struct {
		name   string
		opts   Options
		strip  func(string) string
		expect string
	}{
		{
			name:   "console_plain",
			opts:   Options{Mode: ModeConsole, DisableTimestamp: true, NoColor: true},
			expect: "duration=321.678µs",
		},
		{
			name:   "console_color",
			opts:   Options{Mode: ModeConsole, DisableTimestamp: true, ForceColor: true},
			strip:  stripANSIString,
			expect: "duration=321.678µs",
		},
		{
			name:   "json_plain",
			opts:   Options{Mode: ModeStructured, DisableTimestamp: true, NoColor: true},
			expect: `"duration":"321.678µs"`,
		},
		{
			name:   "json_color",
			opts:   Options{Mode: ModeStructured, DisableTimestamp: true, ForceColor: true},
			strip:  stripANSIString,
			expect: `"duration":"321.678µs"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf strings.Builder
			logger := NewWithOptions(nil, &buf, tc.opts)
			logger.Info("event", "duration", dur)

			out := strings.TrimSpace(buf.String())
			if tc.strip != nil {
				out = tc.strip(out)
			}

			if !strings.Contains(out, tc.expect) {
				t.Fatalf("output missing %q: %q", tc.expect, out)
			}
			if strings.Contains(out, `\\x`) || strings.Contains(out, `\\u00b5`) {
				t.Fatalf("output still has escaped micro sign: %q", out)
			}
		})
	}
}
