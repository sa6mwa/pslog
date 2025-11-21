package pslog

import (
	"io"
	"testing"
)

// Regression: hot path logging should allocate 0 bytes for all emitter variants
// when given pre-built keyvals (to avoid variadic slice creation) and no
// timestamps.
func TestLoggersAllocateZero(t *testing.T) {
	keyvals := []any{"key", "value", "n", 123, "b", true}

	cases := []struct {
		name string
		opts Options
	}{
		{"console_plain", Options{Mode: ModeConsole, DisableTimestamp: true, NoColor: true}},
		{"console_color", Options{Mode: ModeConsole, DisableTimestamp: true, ForceColor: true}},
		{"json_plain", Options{Mode: ModeStructured, DisableTimestamp: true, NoColor: true}},
		{"json_color", Options{Mode: ModeStructured, DisableTimestamp: true, ForceColor: true}},
	}

	for _, tc := range cases {
		logger := NewWithOptions(io.Discard, tc.opts)

		// Warm caches (duration/time/string/float) so the measured run is steady-state.
		logger.Info("warm", keyvals...)

		allocs := testing.AllocsPerRun(1000, func() {
			logger.Info("msg", keyvals...)
		})
		if allocs != 0 {
			t.Fatalf("%s: expected 0 allocs/log, got %.2f", tc.name, allocs)
		}
	}
}
