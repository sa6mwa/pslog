package pslog_test

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
	"pkt.systems/pslog"
)

func TestConsoleOutputMatchesFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := pslog.NewWithOptions(&buf, pslog.Options{Mode: pslog.ModeConsole, DisableTimestamp: true, NoColor: true})
	logger.Info("ready", "foo", "bar", "greeting", "hello world")

	got := strings.TrimSpace(buf.String())
	expected := "INF ready foo=bar greeting=\"hello world\""
	if got != expected {
		t.Fatalf("unexpected output: got %q want %q", got, expected)
	}
}

func TestStructuredOutputJSON(t *testing.T) {
	var buf bytes.Buffer
	logger := pslog.NewWithOptions(&buf, pslog.Options{Mode: pslog.ModeStructured, DisableTimestamp: true})
	logger.Warn("boom", "count", 3)

	line := strings.TrimSpace(buf.String())
	if strings.Contains(line, "\x1b") {
		t.Fatalf("unexpected color codes in JSON: %q", line)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
	if payload["msg"] != "boom" {
		t.Fatalf("expected msg boom, got %v", payload["msg"])
	}
	lvl, ok := payload["lvl"]
	if !ok {
		t.Fatalf("expected lvl field, payload=%v", payload)
	}
	if lvl != "warn" {
		t.Fatalf("expected lvl warn, got %v", lvl)
	}
	if payload["count"] != float64(3) {
		t.Fatalf("expected count 3, got %v", payload["count"])
	}
}

func TestStructuredVerboseFields(t *testing.T) {
	var buf bytes.Buffer
	logger := pslog.NewWithOptions(&buf, pslog.Options{Mode: pslog.ModeStructured, DisableTimestamp: true, VerboseFields: true})
	logger.Info("hello")

	line := strings.TrimSpace(buf.String())
	var payload map[string]any
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
	if payload["message"] != "hello" {
		t.Fatalf("expected message hello, got %v", payload["message"])
	}
	if payload["level"] != "info" {
		t.Fatalf("expected level info, got %v", payload["level"])
	}
	if _, ok := payload["msg"]; ok {
		t.Fatalf("unexpected short field present, payload=%v", payload)
	}
	if _, ok := payload["ts"]; ok {
		t.Fatalf("unexpected ts field when timestamps disabled, payload=%v", payload)
	}
}

func TestStructuredNoColorOnNonTerminal(t *testing.T) {
	var buf bytes.Buffer
	logger := pslog.NewWithOptions(&buf, pslog.Options{Mode: pslog.ModeStructured, DisableTimestamp: true})
	logger.Info("msg")
	if hasANSI(buf.String()) {
		t.Fatalf("expected no colors on non-terminal writer, got %q", buf.String())
	}
}

func TestStructuredForceColor(t *testing.T) {
	var buf bytes.Buffer
	logger := pslog.NewWithOptions(&buf, pslog.Options{Mode: pslog.ModeStructured, DisableTimestamp: true, ForceColor: true})
	logger.Info("msg", "foo", "bar")
	if !hasANSI(buf.String()) {
		t.Fatalf("expected forced color output, got %q", buf.String())
	}
}

func TestConsoleColorAutoDetectWithTTY(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	out := captureTTYOutput(t, func(w io.Writer) {
		logger := pslog.NewWithOptions(w, pslog.Options{Mode: pslog.ModeConsole})
		logger.Info("color")
	})
	if !hasANSI(out) {
		t.Fatalf("expected ANSI sequences when terminal detected, got %q", out)
	}
}

func TestConsoleNoColor(t *testing.T) {
	out := captureTTYOutput(t, func(w io.Writer) {
		logger := pslog.NewWithOptions(w, pslog.Options{Mode: pslog.ModeConsole, NoColor: true})
		logger.Info("plain")
	})
	if hasANSI(out) {
		t.Fatalf("unexpected ANSI sequences when NoColor set: %q", out)
	}
}

func TestConsoleForceColorNoTTY(t *testing.T) {
	var buf bytes.Buffer
	logger := pslog.NewWithOptions(&buf, pslog.Options{Mode: pslog.ModeConsole, ForceColor: true})
	logger.Info("forced")
	if !hasANSI(buf.String()) {
		t.Fatalf("expected ANSI sequences with ForceColor, got %q", buf.String())
	}
}

func TestStructuredColorAutoDetectWithTTY(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	out := captureTTYOutput(t, func(w io.Writer) {
		logger := pslog.NewWithOptions(w, pslog.Options{Mode: pslog.ModeStructured})
		logger.Info("msg", "foo", "bar")
	})
	if !hasANSI(out) {
		t.Fatalf("expected colored output with terminal, got %q", out)
	}
}

func TestStructuredNoColorOnTTY(t *testing.T) {
	out := captureTTYOutput(t, func(w io.Writer) {
		logger := pslog.NewWithOptions(w, pslog.Options{Mode: pslog.ModeStructured, NoColor: true})
		logger.Info("msg", "foo", "bar")
	})
	if hasANSI(out) {
		t.Fatalf("did not expect ANSI sequences when NoColor set, got %q", out)
	}
}

func captureTTYOutput(t *testing.T, fn func(io.Writer)) string {
	t.Helper()
	master, slave, err := pty.Open()
	if err != nil {
		t.Skipf("pty unavailable: %v", err)
	}
	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, master)
		close(done)
	}()
	fn(slave)
	_ = slave.Close()
	<-done
	_ = master.Close()
	return buf.String()
}

func hasANSI(s string) bool {
	return strings.Contains(s, "\x1b[")
}

func TestWithAndMinimalSubset(t *testing.T) {
	var buf bytes.Buffer
	logger := pslog.NewWithOptions(&buf, pslog.Options{Mode: pslog.ModeConsole, DisableTimestamp: true, NoColor: true}).With("app", "demo")
	base := any(logger).(pslog.Base)
	base.Info("up")
	got := strings.TrimSpace(buf.String())
	if !strings.Contains(got, "app=demo") {
		t.Fatalf("expected base field in output, got %q", got)
	}
}

func TestLogLoggerBridgePSL(t *testing.T) {
	var buf bytes.Buffer
	std := pslog.LogLogger(pslog.NewWithOptions(&buf, pslog.Options{Mode: pslog.ModeConsole, DisableTimestamp: true, NoColor: true}))
	std.Println("[INFO] bridge")
	if !strings.Contains(buf.String(), "bridge") {
		t.Fatalf("bridge output missing message: %q", buf.String())
	}
}

func TestConsoleUTCOption(t *testing.T) {
	var buf bytes.Buffer
	logger := pslog.NewWithOptions(&buf, pslog.Options{
		Mode:       pslog.ModeConsole,
		TimeFormat: time.RFC3339,
		NoColor:    true,
		UTC:        true,
	})
	logger.Info("utc-test")

	line := strings.TrimSpace(buf.String())
	parts := strings.SplitN(line, " ", 2)
	if len(parts) == 0 {
		t.Fatalf("expected timestamp in output, got %q", line)
	}
	ts := parts[0]
	parsed, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t.Fatalf("failed to parse timestamp %q: %v", ts, err)
	}
	if parsed.Location().String() != "UTC" {
		t.Fatalf("expected UTC timestamp, got %q (location=%s)", ts, parsed.Location())
	}
}
