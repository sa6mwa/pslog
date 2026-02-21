package pslog_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	pslog "pkt.systems/pslog"
)

func TestStructuredJSONParityAllTypes(t *testing.T) {
	var buf bytes.Buffer
	logger := pslog.NewWithOptions(nil, &buf, pslog.Options{
		Mode:             pslog.ModeStructured,
		DisableTimestamp: true,
		NoColor:          true,
		MinLevel:         pslog.DebugLevel,
	})

	tsISO := time.Date(2025, time.October, 29, 16, 0, 34, 897754025, time.FixedZone("CET", 3600)).Format(time.RFC3339Nano)
	errVal := errors.New("disk full")
	nested := map[string]any{
		"nested": "value",
		"slice":  []any{"alpha", 42},
	}

	logger.Info("hello",
		"ts_iso", tsISO,
		"user", "alice",
		"attempts", 3,
		"latency_ms", 12.34,
		"ok", true,
		"status", nil,
		"err", errVal,
		"payload", nested,
		"bytes", []byte("payload"),
		pslog.TrustedString("trusted_key"), pslog.TrustedString("trusted value"),
	)

	line := strings.TrimSpace(buf.String())
	if line == "" || line[0] != '{' {
		t.Fatalf("expected json object, got %q", line)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("invalid json output: %v for line %q", err, line)
	}

	expected := map[string]any{
		"lvl":         "info",
		"msg":         "hello",
		"ts_iso":      tsISO,
		"user":        "alice",
		"attempts":    3,
		"latency_ms":  12.34,
		"ok":          true,
		"status":      nil,
		"err":         errVal.Error(),
		"payload":     nested,
		"bytes":       "payload",
		"trusted_key": "trusted value",
	}

	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("failed to marshal reference json: %v", err)
	}

	var want map[string]any
	if err := json.Unmarshal(expectedJSON, &want); err != nil {
		t.Fatalf("failed to decode reference json: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("json mismatch:\n got  %v\n want %v\n line=%s\n ref=%s", got, want, line, string(expectedJSON))
	}
}

func TestStructuredJSONColorErrorQuoting(t *testing.T) {
	var buf bytes.Buffer
	logger := pslog.NewWithOptions(nil, &buf, pslog.Options{
		Mode:             pslog.ModeStructured,
		ForceColor:       true,
		DisableTimestamp: true,
		MinLevel:         pslog.DebugLevel,
	})

	logger.Info("hello", "err", errors.New("disk full"))
	line := buf.String()
	if !strings.Contains(line, "\x1b") {
		t.Fatalf("expected ansi colored output, got %q", line)
	}
	if !strings.Contains(line, "\x1b[1;31m\"disk full\"\x1b[0m") {
		t.Fatalf("colored error not quoted correctly: %q", line)
	}
}

func TestStructuredJSONColorMatchesPlain(t *testing.T) {
	var plainBuf, colorBuf bytes.Buffer
	fields := []any{
		"user", "alice",
		"attempts", 3,
		"latency_ms", 12.34,
		"ok", true,
		"status", nil,
		"err", errors.New("disk full"),
		"payload", map[string]any{"nested": "value", "slice": []any{"alpha", 42}},
		"bytes", []byte("payload"),
		pslog.TrustedString("trusted_key"), pslog.TrustedString("trusted value"),
	}

	plain := pslog.NewWithOptions(nil, &plainBuf, pslog.Options{Mode: pslog.ModeStructured, DisableTimestamp: true, NoColor: true})
	color := pslog.NewWithOptions(nil, &colorBuf, pslog.Options{Mode: pslog.ModeStructured, DisableTimestamp: true, ForceColor: true})

	plain.Info("hello", fields...)
	color.Info("hello", fields...)

	plainLine := strings.TrimSpace(plainBuf.String())
	colorLine := strings.TrimSpace(colorBuf.String())
	if plainLine == "" || colorLine == "" {
		t.Fatalf("empty output plain=%q color=%q", plainLine, colorLine)
	}
	stripped := stripANSI(colorLine)
	if plainLine != stripped {
		t.Fatalf("color output mismatch after stripping ansi:\nplain=%s\ncolor=%s\nstripped=%s", plainLine, colorLine, stripped)
	}
}
