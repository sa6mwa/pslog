package pslog_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pslog "pkt.systems/pslog"
)

func TestLoggerFromEnvOverridesOptions(t *testing.T) {
	t.Setenv("PSLOG_TEST_LEVEL", "info")
	t.Setenv("PSLOG_TEST_MODE", "json")

	var buf bytes.Buffer
	logger := pslog.LoggerFromEnv(
		pslog.WithEnvPrefix("PSLOG_TEST_"),
		pslog.WithEnvWriter(&buf),
		pslog.WithEnvOptions(pslog.Options{
			Mode:             pslog.ModeConsole,
			DisableTimestamp: true,
			NoColor:          true,
			MinLevel:         pslog.DebugLevel,
		}),
	)

	logger.Debug("debug_suppressed")
	logger.Info("info_visible")

	lines := collectLines(&buf)
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line, got %d: %v", len(lines), lines)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &payload); err != nil {
		t.Fatalf("invalid json output %q: %v", lines[0], err)
	}
	if payload["msg"] != "info_visible" {
		t.Fatalf("unexpected message payload: %v", payload["msg"])
	}
	if strings.Contains(lines[0], "debug_suppressed") {
		t.Fatalf("unexpected debug output: %s", lines[0])
	}
}

func TestLoggerFromEnvVerboseFields(t *testing.T) {
	t.Setenv("PSLOG_TEST_VERBOSE_FIELDS", "true")

	var buf bytes.Buffer
	logger := pslog.LoggerFromEnv(
		pslog.WithEnvPrefix("PSLOG_TEST_"),
		pslog.WithEnvWriter(&buf),
		pslog.WithEnvOptions(pslog.Options{Mode: pslog.ModeStructured, NoColor: true}),
	)

	logger.Info("verbose")

	lines := collectLines(&buf)
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line, got %d: %v", len(lines), lines)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &payload); err != nil {
		t.Fatalf("invalid json output %q: %v", lines[0], err)
	}
	if _, ok := payload["message"]; !ok {
		t.Fatalf("expected message key in %v", payload)
	}
	if _, ok := payload["msg"]; ok {
		t.Fatalf("unexpected msg key in %v", payload)
	}
	if _, ok := payload["time"]; !ok {
		t.Fatalf("expected time key in %v", payload)
	}
	if _, ok := payload["ts"]; ok {
		t.Fatalf("unexpected ts key in %v", payload)
	}
	if _, ok := payload["level"]; !ok {
		t.Fatalf("expected level key in %v", payload)
	}
	if _, ok := payload["lvl"]; ok {
		t.Fatalf("unexpected lvl key in %v", payload)
	}
}

func TestLoggerFromEnvDisableTimestamp(t *testing.T) {
	t.Setenv("PSLOG_TEST_DISABLE_TIMESTAMP", "true")

	var buf bytes.Buffer
	logger := pslog.LoggerFromEnv(
		pslog.WithEnvPrefix("PSLOG_TEST_"),
		pslog.WithEnvWriter(&buf),
		pslog.WithEnvOptions(pslog.Options{Mode: pslog.ModeStructured, NoColor: true}),
	)

	logger.Info("no_timestamp")

	lines := collectLines(&buf)
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line, got %d: %v", len(lines), lines)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &payload); err != nil {
		t.Fatalf("invalid json output %q: %v", lines[0], err)
	}
	if _, ok := payload["ts"]; ok {
		t.Fatalf("unexpected ts key in %v", payload)
	}
	if _, ok := payload["time"]; ok {
		t.Fatalf("unexpected time key in %v", payload)
	}
}

func TestLoggerFromEnvCallerKey(t *testing.T) {
	t.Setenv("PSLOG_TEST_CALLER_KEYVAL", "true")
	t.Setenv("PSLOG_TEST_CALLER_KEY", "caller")

	var buf bytes.Buffer
	logger := pslog.LoggerFromEnv(
		pslog.WithEnvPrefix("PSLOG_TEST_"),
		pslog.WithEnvWriter(&buf),
		pslog.WithEnvOptions(pslog.Options{Mode: pslog.ModeStructured, DisableTimestamp: true, NoColor: true}),
	)

	logger.Info("caller")

	lines := collectLines(&buf)
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line, got %d: %v", len(lines), lines)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &payload); err != nil {
		t.Fatalf("invalid json output %q: %v", lines[0], err)
	}
	value, ok := payload["caller"].(string)
	if !ok || value == "" {
		t.Fatalf("expected caller key in %v", payload)
	}
}

func TestLoggerFromEnvInvalidLevelKeepsSeed(t *testing.T) {
	t.Setenv("PSLOG_TEST_LEVEL", "notalevel")

	var buf bytes.Buffer
	logger := pslog.LoggerFromEnv(
		pslog.WithEnvPrefix("PSLOG_TEST_"),
		pslog.WithEnvWriter(&buf),
		pslog.WithEnvOptions(pslog.Options{Mode: pslog.ModeStructured, DisableTimestamp: true, NoColor: true, MinLevel: pslog.ErrorLevel}),
	)

	logger.Info("suppressed")
	logger.Error("visible")

	lines := collectLines(&buf)
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line, got %d: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "visible") {
		t.Fatalf("expected error output, got %q", lines[0])
	}
}

func TestLoggerFromEnvInvalidModeKeepsSeed(t *testing.T) {
	t.Setenv("PSLOG_TEST_MODE", "nope")

	var buf bytes.Buffer
	logger := pslog.LoggerFromEnv(
		pslog.WithEnvPrefix("PSLOG_TEST_"),
		pslog.WithEnvWriter(&buf),
		pslog.WithEnvOptions(pslog.Options{Mode: pslog.ModeStructured, DisableTimestamp: true, NoColor: true}),
	)

	logger.Info("json")

	lines := collectLines(&buf)
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line, got %d: %v", len(lines), lines)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &payload); err != nil {
		t.Fatalf("expected json output, got %q: %v", lines[0], err)
	}
}

func TestLoggerFromEnvTimeFormat(t *testing.T) {
	t.Setenv("PSLOG_TEST_TIME_FORMAT", "2006-01-02")

	var buf bytes.Buffer
	logger := pslog.LoggerFromEnv(
		pslog.WithEnvPrefix("PSLOG_TEST_"),
		pslog.WithEnvWriter(&buf),
		pslog.WithEnvOptions(pslog.Options{Mode: pslog.ModeStructured, NoColor: true}),
	)

	logger.Info("timefmt")

	lines := collectLines(&buf)
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line, got %d: %v", len(lines), lines)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &payload); err != nil {
		t.Fatalf("invalid json output %q: %v", lines[0], err)
	}
	ts, ok := payload["ts"].(string)
	if !ok || ts == "" {
		t.Fatalf("expected ts field in %v", payload)
	}
	if _, err := time.Parse("2006-01-02", ts); err != nil {
		t.Fatalf("expected ts to match layout: %v", err)
	}
}

func TestLoggerFromEnvOutputFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	t.Setenv("PSLOG_TEST_OUTPUT", path)

	var buf bytes.Buffer
	logger := pslog.LoggerFromEnv(
		pslog.WithEnvPrefix("PSLOG_TEST_"),
		pslog.WithEnvWriter(&buf),
		pslog.WithEnvOptions(pslog.Options{Mode: pslog.ModeStructured, DisableTimestamp: true, NoColor: true}),
	)

	logger.Info("file_only")
	closeLogger(t, logger)

	if got := strings.TrimSpace(buf.String()); got != "" {
		t.Fatalf("expected buffer empty, got %q", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !strings.Contains(string(data), "file_only") {
		t.Fatalf("expected file log, got %q", string(data))
	}
}

func TestLoggerFromEnvOutputDefaultKeepsSeed(t *testing.T) {
	t.Setenv("PSLOG_TEST_OUTPUT", "default")

	var buf bytes.Buffer
	logger := pslog.LoggerFromEnv(
		pslog.WithEnvPrefix("PSLOG_TEST_"),
		pslog.WithEnvWriter(&buf),
		pslog.WithEnvOptions(pslog.Options{Mode: pslog.ModeStructured, DisableTimestamp: true, NoColor: true}),
	)

	logger.Info("seeded")

	lines := collectLines(&buf)
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line, got %d: %v", len(lines), lines)
	}
}

func TestLoggerFromEnvOutputPathWithPlus(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello+world.log")
	t.Setenv("PSLOG_TEST_OUTPUT", path)

	var buf bytes.Buffer
	logger := pslog.LoggerFromEnv(
		pslog.WithEnvPrefix("PSLOG_TEST_"),
		pslog.WithEnvWriter(&buf),
		pslog.WithEnvOptions(pslog.Options{Mode: pslog.ModeStructured, DisableTimestamp: true, NoColor: true}),
	)

	logger.Info("plus")
	closeLogger(t, logger)

	if got := strings.TrimSpace(buf.String()); got != "" {
		t.Fatalf("expected buffer empty, got %q", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !strings.Contains(string(data), "plus") {
		t.Fatalf("expected file log, got %q", string(data))
	}
}

func TestLoggerFromEnvOutputDefaultTee(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tee.log")
	t.Setenv("PSLOG_TEST_OUTPUT", "default+"+path)

	var buf bytes.Buffer
	logger := pslog.LoggerFromEnv(
		pslog.WithEnvPrefix("PSLOG_TEST_"),
		pslog.WithEnvWriter(&buf),
		pslog.WithEnvOptions(pslog.Options{Mode: pslog.ModeStructured, DisableTimestamp: true, NoColor: true}),
	)

	logger.Info("tee")
	closeLogger(t, logger)

	if !strings.Contains(buf.String(), "tee") {
		t.Fatalf("expected buffer to contain tee output, got %q", buf.String())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !strings.Contains(string(data), "tee") {
		t.Fatalf("expected file tee output, got %q", string(data))
	}
}

func TestLoggerFromEnvOutputFailure(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PSLOG_TEST_OUTPUT", dir)

	var buf bytes.Buffer
	logger := pslog.LoggerFromEnv(
		pslog.WithEnvPrefix("PSLOG_TEST_"),
		pslog.WithEnvWriter(&buf),
		pslog.WithEnvOptions(pslog.Options{Mode: pslog.ModeStructured, DisableTimestamp: true, NoColor: true}),
	)

	logger.Info("after")

	lines := collectLines(&buf)
	foundError := false
	foundAfter := false
	for _, line := range lines {
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			t.Fatalf("invalid json output %q: %v", line, err)
		}
		msg, _ := payload["msg"].(string)
		if msg == "logger.output.open.failed" {
			foundError = true
			if _, ok := payload["error"]; !ok {
				t.Fatalf("expected error field in %v", payload)
			}
		}
		if msg == "after" {
			foundAfter = true
		}
	}
	if !foundError {
		t.Fatalf("expected logger output failure to be logged")
	}
	if !foundAfter {
		t.Fatalf("expected logger to fall back and log subsequent entries")
	}
}

func closeLogger(t *testing.T, logger pslog.Logger) {
	t.Helper()
	if closer, ok := logger.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			t.Fatalf("close logger: %v", err)
		}
	}
}
