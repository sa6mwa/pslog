package pslog_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	pslog "pkt.systems/pslog"
)

func collectLines(buf *bytes.Buffer) []string {
	raw := strings.Split(strings.TrimSpace(buf.String()), "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func TestLoggerVariantsCoverage(t *testing.T) {
	largeValue := strings.Repeat("x", 10*1024)
	variants := []struct {
		name  string
		opts  pslog.Options
		strip func(string) string
		parse func(*testing.T, []string)
	}{
		{
			name:  "structured_plain",
			opts:  pslog.Options{Mode: pslog.ModeStructured, DisableTimestamp: false, NoColor: true, MinLevel: pslog.TraceLevel},
			strip: func(s string) string { return s },
			parse: parseStructuredLines,
		},
		{
			name:  "structured_color",
			opts:  pslog.Options{Mode: pslog.ModeStructured, DisableTimestamp: false, ForceColor: true, MinLevel: pslog.TraceLevel},
			strip: stripANSI,
			parse: parseStructuredLines,
		},
		{
			name:  "console_plain",
			opts:  pslog.Options{Mode: pslog.ModeConsole, DisableTimestamp: false, NoColor: true, MinLevel: pslog.TraceLevel},
			strip: func(s string) string { return s },
			parse: parseConsoleLines,
		},
		{
			name:  "console_color",
			opts:  pslog.Options{Mode: pslog.ModeConsole, DisableTimestamp: false, ForceColor: true, MinLevel: pslog.TraceLevel},
			strip: stripANSI,
			parse: parseConsoleLines,
		},
	}

	outputs := make(map[string][]string)

	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := pslog.NewWithOptions(nil, &buf, variant.opts)

			if fmt.Sprintf("%p", logger.LogLevelFromEnv("_UNSET")) != fmt.Sprintf("%p", logger) {
				t.Fatalf("expected LogLevelFromEnv without env to return same logger")
			}

			logger = logger.With("app", "demo")
			logger = logger.WithLogLevel()
			if fmt.Sprintf("%p", logger.WithLogLevel()) != fmt.Sprintf("%p", logger) {
				t.Fatalf("WithLogLevel should return same when already included")
			}

			logger.Trace("trace", "value_trusted", pslog.TrustedString("trusted"))
			logger.Debug("debug", "bytes", []byte("b"))

			if err := os.Setenv("PSLOG_LEVEL", "info"); err != nil {
				t.Fatalf("Setenv failed: %v", err)
			}
			defer func() {
				if err := os.Unsetenv("PSLOG_LEVEL"); err != nil {
					t.Fatalf("Unsetenv failed: %v", err)
				}
			}()
			logger = logger.LogLevelFromEnv("PSLOG_LEVEL")
			logger.Info("info", "map", map[string]any{"k": "v"})
			logger.Warn("warn", "slice", []any{"a", 1}, "large", largeValue)

			logger = logger.LogLevel(pslog.ErrorLevel)
			logger.Info("info_suppressed")
			logger.Error("error", "err", errors.New("boom"))

			lines := collectLines(&buf)
			if len(lines) != 5 {
				t.Fatalf("expected 5 log lines, got %d: %v", len(lines), lines)
			}

			stripped := make([]string, len(lines))
			for i, line := range lines {
				stripped[i] = variant.strip(line)
			}

			outputs[variant.name] = stripped
			variant.parse(t, stripped)
		})
	}

	if !reflect.DeepEqual(outputs["structured_plain"], outputs["structured_color"]) {
		t.Fatalf("structured plain/color mismatch:\nplain=%v\ncolor=%v", outputs["structured_plain"], outputs["structured_color"])
	}
	if !reflect.DeepEqual(outputs["console_plain"], outputs["console_color"]) {
		t.Fatalf("console plain/color mismatch:\nplain=%v\ncolor=%v", outputs["console_plain"], outputs["console_color"])
	}
}

func parseStructuredLines(t *testing.T, lines []string) {
	loglevelCount := 0
	for _, line := range lines {
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			t.Fatalf("invalid structured output %q: %v", line, err)
		}
		if _, ok := payload["app"]; !ok {
			t.Fatalf("missing base field in %q", line)
		}
		if _, ok := payload["loglevel"]; ok {
			loglevelCount++
		}
	}
	if loglevelCount == 0 {
		t.Fatalf("expected at least one line with loglevel field")
	}
}

func parseConsoleLines(t *testing.T, lines []string) {
	for _, line := range lines {
		if !strings.Contains(line, "app=demo") {
			t.Fatalf("console line missing base field: %s", line)
		}
	}
	if strings.Contains(lines[len(lines)-2], "info_suppressed") {
		t.Fatalf("info message should have been suppressed")
	}
}
