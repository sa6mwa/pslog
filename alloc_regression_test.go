package pslog

import (
	"io"
	"math"
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
		logger := NewWithOptions(nil, io.Discard, tc.opts)

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

// Regression: JSON non-finite float serialization should stay zero-allocation
// in steady-state for both string and null policies.
func TestJSONNonFiniteFloatAllocateZero(t *testing.T) {
	keyvals := []any{
		"nan", math.NaN(),
		"pos_inf", math.Inf(1),
		"neg_inf", math.Inf(-1),
	}

	cases := []struct {
		name string
		opts Options
	}{
		{
			name: "json_plain_string_policy",
			opts: Options{
				Mode:                 ModeStructured,
				DisableTimestamp:     true,
				NoColor:              true,
				NonFiniteFloatPolicy: NonFiniteFloatAsString,
			},
		},
		{
			name: "json_color_string_policy",
			opts: Options{
				Mode:                 ModeStructured,
				DisableTimestamp:     true,
				ForceColor:           true,
				NonFiniteFloatPolicy: NonFiniteFloatAsString,
			},
		},
		{
			name: "json_plain_null_policy",
			opts: Options{
				Mode:                 ModeStructured,
				DisableTimestamp:     true,
				NoColor:              true,
				NonFiniteFloatPolicy: NonFiniteFloatAsNull,
			},
		},
		{
			name: "json_color_null_policy",
			opts: Options{
				Mode:                 ModeStructured,
				DisableTimestamp:     true,
				ForceColor:           true,
				NonFiniteFloatPolicy: NonFiniteFloatAsNull,
			},
		},
	}

	for _, tc := range cases {
		logger := NewWithOptions(nil, io.Discard, tc.opts)

		// Warm internal writer caches before measuring steady-state allocations.
		logger.Info("warm", keyvals...)

		allocs := testing.AllocsPerRun(1000, func() {
			logger.Info("msg", keyvals...)
		})
		if allocs != 0 {
			t.Fatalf("%s: expected 0 allocs/log, got %.2f", tc.name, allocs)
		}
	}
}
