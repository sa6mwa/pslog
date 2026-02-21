package pslog_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	pslog "pkt.systems/pslog"
	"pkt.systems/pslog/ansi"
)

func TestLoggerFromEnvOverridesOptions(t *testing.T) {
	t.Setenv("PSLOG_TEST_LEVEL", "info")
	t.Setenv("PSLOG_TEST_MODE", "json")

	var buf bytes.Buffer
	logger := pslog.LoggerFromEnv(nil,
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
	logger := pslog.LoggerFromEnv(nil,
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
	logger := pslog.LoggerFromEnv(nil,
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

func TestLoggerFromEnvNoColorDisablesColor(t *testing.T) {
	t.Setenv("PSLOG_TEST_NO_COLOR", "true")

	out := captureTTYOutput(t, func(w io.Writer) {
		logger := pslog.LoggerFromEnv(nil,
			pslog.WithEnvPrefix("PSLOG_TEST_"),
			pslog.WithEnvWriter(w),
			pslog.WithEnvOptions(pslog.Options{
				Mode:             pslog.ModeStructured,
				DisableTimestamp: true,
				ForceColor:       true,
			}),
		)
		logger.Info("env_no_color")
	})

	if hasANSI(out) {
		t.Fatalf("expected LoggerFromEnv NO_COLOR to disable color, got %q", out)
	}
}

func TestLoggerFromEnvCallerKey(t *testing.T) {
	t.Setenv("PSLOG_TEST_CALLER_KEYVAL", "true")
	t.Setenv("PSLOG_TEST_CALLER_KEY", "caller")

	var buf bytes.Buffer
	logger := pslog.LoggerFromEnv(nil,
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
	logger := pslog.LoggerFromEnv(nil,
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
	logger := pslog.LoggerFromEnv(nil,
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

func TestLoggerFromEnvPalette(t *testing.T) {
	t.Setenv("PSLOG_TEST_PALETTE", "one-dark")

	var buf bytes.Buffer
	logger := pslog.LoggerFromEnv(nil,
		pslog.WithEnvPrefix("PSLOG_TEST_"),
		pslog.WithEnvWriter(&buf),
		pslog.WithEnvOptions(pslog.Options{
			Mode:             pslog.ModeStructured,
			DisableTimestamp: true,
			ForceColor:       true,
		}),
	)
	logger.Info("palette")

	line := strings.TrimSpace(buf.String())
	if !strings.Contains(line, ansi.PaletteOneDark.Message+"\"palette\"") {
		t.Fatalf("expected one-dark message color, got %q", line)
	}
}

func TestLoggerFromEnvPaletteAliasCompatibility(t *testing.T) {
	t.Setenv("PSLOG_TEST_PALETTE", "doom-nord")

	var buf bytes.Buffer
	logger := pslog.LoggerFromEnv(nil,
		pslog.WithEnvPrefix("PSLOG_TEST_"),
		pslog.WithEnvWriter(&buf),
		pslog.WithEnvOptions(pslog.Options{
			Mode:             pslog.ModeStructured,
			DisableTimestamp: true,
			ForceColor:       true,
		}),
	)
	logger.Info("alias")

	line := strings.TrimSpace(buf.String())
	if !strings.Contains(line, ansi.PaletteNord.Message+"\"alias\"") {
		t.Fatalf("expected doom alias to resolve to nord, got %q", line)
	}
}

func TestLoggerFromEnvInvalidPaletteFallsBackToDefault(t *testing.T) {
	t.Setenv("PSLOG_TEST_PALETTE", "not-a-palette")

	var buf bytes.Buffer
	seededPalette := ansi.Palette{
		Key:        "[KEY]",
		String:     "[STR]",
		Num:        "[NUM]",
		Bool:       "[BOOL]",
		Nil:        "[NIL]",
		Trace:      "[TRC]",
		Debug:      "[DBG]",
		Info:       "[INF]",
		Warn:       "[WRN]",
		Error:      "[ERR]",
		Fatal:      "[FTL]",
		Panic:      "[PNC]",
		NoLevel:    "[NOL]",
		Timestamp:  "[TS]",
		MessageKey: "[MSGKEY]",
		Message:    "[MSG]",
	}
	logger := pslog.LoggerFromEnv(nil,
		pslog.WithEnvPrefix("PSLOG_TEST_"),
		pslog.WithEnvWriter(&buf),
		pslog.WithEnvOptions(pslog.Options{
			Mode:             pslog.ModeStructured,
			DisableTimestamp: true,
			ForceColor:       true,
			Palette:          &seededPalette,
		}),
	)
	logger.Info("seed")

	line := strings.TrimSpace(buf.String())
	if strings.Contains(line, "[MSG]\"seed\"") {
		t.Fatalf("expected invalid env palette to fall back to default palette, got %q", line)
	}
	if !strings.Contains(line, ansi.PaletteDefault.Message+"\"seed\"") {
		t.Fatalf("expected invalid env palette to resolve to default palette, got %q", line)
	}
}

func TestLoggerFromEnvTimeFormat(t *testing.T) {
	t.Setenv("PSLOG_TEST_TIME_FORMAT", "2006-01-02")

	var buf bytes.Buffer
	logger := pslog.LoggerFromEnv(nil,
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
	logger := pslog.LoggerFromEnv(nil,
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

func TestLoggerFromEnvOutputFileModeDefault(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file mode checks are platform-dependent on Windows")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "app-default.log")
	t.Setenv("PSLOG_TEST_OUTPUT", path)

	var buf bytes.Buffer
	logger := pslog.LoggerFromEnv(nil,
		pslog.WithEnvPrefix("PSLOG_TEST_"),
		pslog.WithEnvWriter(&buf),
		pslog.WithEnvOptions(pslog.Options{Mode: pslog.ModeStructured, DisableTimestamp: true, NoColor: true}),
	)

	logger.Info("file_only")
	closeLogger(t, logger)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	const secureDefaultMode = 0o600
	if got := info.Mode().Perm(); got&^secureDefaultMode != 0 {
		t.Fatalf("unexpected default file mode: %o", got)
	}
}

func TestLoggerFromEnvOutputFileModeOverride(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file mode checks are platform-dependent on Windows")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	t.Setenv("PSLOG_TEST_OUTPUT", path)
	t.Setenv("PSLOG_TEST_OUTPUT_FILE_MODE", "0644")

	var buf bytes.Buffer
	logger := pslog.LoggerFromEnv(nil,
		pslog.WithEnvPrefix("PSLOG_TEST_"),
		pslog.WithEnvWriter(&buf),
		pslog.WithEnvOptions(pslog.Options{Mode: pslog.ModeStructured, DisableTimestamp: true, NoColor: true}),
	)

	logger.Info("file_only")
	closeLogger(t, logger)

	if got := strings.TrimSpace(buf.String()); got != "" {
		t.Fatalf("expected buffer empty, got %q", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	const overrideMode = 0o644
	if got := info.Mode().Perm(); got&^overrideMode != 0 {
		t.Fatalf("expected file mode masked by umask from %o, got %o", overrideMode, got)
	}
}

func TestLoggerFromEnvOutputDefaultKeepsSeed(t *testing.T) {
	t.Setenv("PSLOG_TEST_OUTPUT", "default")

	var buf bytes.Buffer
	logger := pslog.LoggerFromEnv(nil,
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
	logger := pslog.LoggerFromEnv(nil,
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
	logger := pslog.LoggerFromEnv(nil,
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
	logger := pslog.LoggerFromEnv(nil,
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

func TestLoggerFromEnvOutputFileModeInvalid(t *testing.T) {
	t.Setenv("PSLOG_TEST_OUTPUT_FILE_MODE", "0x1ff")

	var buf bytes.Buffer
	logger := pslog.LoggerFromEnv(nil,
		pslog.WithEnvPrefix("PSLOG_TEST_"),
		pslog.WithEnvWriter(&buf),
		pslog.WithEnvOptions(pslog.Options{Mode: pslog.ModeStructured, DisableTimestamp: true, NoColor: true}),
	)

	logger.Info("after")

	lines := collectLines(&buf)
	foundInvalid := false
	foundAfter := false
	for _, line := range lines {
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			t.Fatalf("invalid json output %q: %v", line, err)
		}
		msg, _ := payload["msg"].(string)
		if msg == "logger.output.file_mode.invalid" {
			foundInvalid = true
			if payload["output_file_mode"] != "0x1ff" {
				t.Fatalf("expected output_file_mode to be recorded, got %v", payload["output_file_mode"])
			}
			if _, ok := payload["error"]; !ok {
				t.Fatalf("expected error field in %v", payload)
			}
		}
		if msg == "after" {
			foundAfter = true
		}
	}
	if !foundInvalid {
		t.Fatalf("expected invalid output file mode to be logged")
	}
	if !foundAfter {
		t.Fatalf("expected subsequent entries after invalid output file mode")
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
