package pslog

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestJSONPlainEmitVariants(t *testing.T) {
	keyvals := []any{
		"str", "value",
		"bool", true,
		"int", int64(-5),
		"uint", uint64(7),
		"float", 3.5,
		"bytes", []byte("raw"),
		"dur", 1500 * time.Millisecond,
		"time", time.Unix(1700000000, 0).UTC(),
		"stringer", stubStringer{"stringer"},
		"error", stubError{"boom"},
		"nil", nil,
	}

	combos := []struct {
		name     string
		opts     Options
		withBase bool
		withLvl  bool
	}{
		{"timestamp_loglevel_with_base", Options{Mode: ModeStructured, NoColor: true, MinLevel: TraceLevel}, true, true},
		{"timestamp_loglevel_no_base", Options{Mode: ModeStructured, NoColor: true, MinLevel: TraceLevel}, false, true},
		{"timestamp_with_base", Options{Mode: ModeStructured, NoColor: true, MinLevel: TraceLevel}, true, false},
		{"loglevel_with_base", Options{Mode: ModeStructured, NoColor: true, MinLevel: TraceLevel, DisableTimestamp: true}, true, true},
		{"loglevel_no_base", Options{Mode: ModeStructured, NoColor: true, MinLevel: TraceLevel, DisableTimestamp: true}, false, true},
	}

	for _, combo := range combos {
		t.Run(combo.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewWithOptions(nil, &buf, combo.opts)
			if combo.withBase {
				logger = logger.With("app", "demo")
			}
			if combo.withLvl {
				logger = logger.WithLogLevel()
			}
			logger.Log(InfoLevel, "hello", keyvals...)
			out := strings.TrimSpace(buf.String())
			if out == "" {
				t.Fatalf("expected output")
			}
			if !json.Valid([]byte(out)) {
				t.Fatalf("invalid JSON: %s", out)
			}
			if combo.withBase && !strings.Contains(out, "\"app\":\"demo\"") {
				t.Fatalf("missing base field: %s", out)
			}
			if combo.withLvl && !strings.Contains(out, "\"loglevel\"") {
				t.Fatalf("missing loglevel field: %s", out)
			}
		})
	}
}

func TestJSONPlainSlowPath(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithOptions(nil, &buf, Options{Mode: ModeStructured, NoColor: true, DisableTimestamp: true})
	logger.Info("slow", 123, "value", "odd")
	out := strings.TrimSpace(buf.String())
	if !json.Valid([]byte(out)) {
		t.Fatalf("invalid JSON: %s", out)
	}
	if !strings.Contains(out, "\"123\"") || !strings.Contains(out, "\"arg1\"") {
		t.Fatalf("expected numeric and synthesized keys: %s", out)
	}
}

func TestJSONColorEmitVariants(t *testing.T) {
	keyvals := []any{
		"str", "value",
		"bool", true,
		"int", int64(-5),
		"uint", uint64(7),
		"float", 3.5,
		"bytes", []byte("raw"),
		"dur", 1500 * time.Millisecond,
		"time", time.Unix(1700000000, 0).UTC(),
		"stringer", stubStringer{"stringer"},
		"error", stubError{"boom"},
		"nil", nil,
	}

	combos := []struct {
		name     string
		opts     Options
		withBase bool
		withLvl  bool
	}{
		{"timestamp_loglevel_with_base", Options{Mode: ModeStructured, ForceColor: true, MinLevel: TraceLevel}, true, true},
		{"timestamp_loglevel_no_base", Options{Mode: ModeStructured, ForceColor: true, MinLevel: TraceLevel}, false, true},
		{"timestamp_with_base", Options{Mode: ModeStructured, ForceColor: true, MinLevel: TraceLevel}, true, false},
		{"loglevel_with_base", Options{Mode: ModeStructured, ForceColor: true, MinLevel: TraceLevel, DisableTimestamp: true}, true, true},
		{"loglevel_no_base", Options{Mode: ModeStructured, ForceColor: true, MinLevel: TraceLevel, DisableTimestamp: true}, false, true},
	}

	for _, combo := range combos {
		t.Run(combo.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewWithOptions(nil, &buf, combo.opts)
			if combo.withBase {
				logger = logger.With("app", "demo")
			}
			if combo.withLvl {
				logger = logger.WithLogLevel()
			}
			logger.Log(InfoLevel, "hello", keyvals...)
			raw := buf.String()
			if raw == "" {
				t.Fatalf("expected output")
			}
			out := stripANSIString(raw)
			if combo.withBase && !strings.Contains(out, `"app"`) {
				t.Fatalf("missing base field: %s", out)
			}
			if combo.withLvl && !strings.Contains(out, `"loglevel"`) {
				t.Fatalf("missing loglevel field: %s", out)
			}
		})
	}
}

func TestJSONColorSlowPath(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithOptions(nil, &buf, Options{Mode: ModeStructured, ForceColor: true, DisableTimestamp: true})
	logger.Info("slow", 123, "value", "odd")
	out := stripANSIString(buf.String())
	if !strings.Contains(out, `"123"`) || !strings.Contains(out, `"arg1"`) {
		t.Fatalf("expected numeric and synthesized keys: %s", out)
	}
}
