package pslog_test

import (
	"bytes"
	"encoding/json"
	"testing"

	pslog "pkt.systems/pslog"
)

//go:noinline
func callerAlpha(logger pslog.Logger) {
	logger.Info("alpha")
}

//go:noinline
func callerBeta(logger pslog.Logger) {
	logger.Info("beta")
}

func decodeJSONLine(t *testing.T, line string) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		t.Fatalf("failed to decode JSON line %q: %v", line, err)
	}
	return payload
}

func TestCallerKeyvalAddsFunctionPerLogEntry(t *testing.T) {
	var buf bytes.Buffer
	logger := pslog.NewWithOptions(nil, &buf, pslog.Options{
		Mode:             pslog.ModeStructured,
		NoColor:          true,
		DisableTimestamp: true,
		CallerKeyval:     true,
	})

	callerAlpha(logger)
	callerBeta(logger)

	lines := collectLines(&buf)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}

	first := decodeJSONLine(t, lines[0])
	second := decodeJSONLine(t, lines[1])

	if got := first["fn"]; got != "callerAlpha" {
		t.Fatalf("first fn field mismatch: got %v", got)
	}
	if got := second["fn"]; got != "callerBeta" {
		t.Fatalf("second fn field mismatch: got %v", got)
	}
}

func TestCallerKeyvalUsesCustomKey(t *testing.T) {
	var buf bytes.Buffer
	logger := pslog.NewWithOptions(nil, &buf, pslog.Options{
		Mode:             pslog.ModeStructured,
		NoColor:          true,
		DisableTimestamp: true,
		CallerKeyval:     true,
		CallerKey:        "caller",
	})

	callerAlpha(logger)

	lines := collectLines(&buf)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %v", len(lines), lines)
	}

	payload := decodeJSONLine(t, lines[0])

	if got := payload["caller"]; got != "callerAlpha" {
		t.Fatalf("custom caller field mismatch: got %v", got)
	}
	if _, ok := payload["fn"]; ok {
		t.Fatalf("default fn field should be absent when custom caller key is set")
	}
}
