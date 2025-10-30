package pslog_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	pslog "pkt.systems/pslog"
)

func TestConsolePlainQuoting(t *testing.T) {
	var buf bytes.Buffer
	logger := pslog.NewWithOptions(&buf, pslog.Options{
		Mode:             pslog.ModeConsole,
		DisableTimestamp: true,
		NoColor:          true,
	})

	logger.Info("event", "user name", "alice bob", "err", errors.New("disk full"))
	line := strings.TrimSpace(buf.String())
	if !strings.Contains(line, "user name=\"alice bob\"") {
		t.Fatalf("expected quoted console field, got %q", line)
	}
	if !strings.Contains(line, "err=\"disk full\"") {
		t.Fatalf("expected quoted error field, got %q", line)
	}
}

func TestConsoleColorQuoting(t *testing.T) {
	var buf bytes.Buffer
	logger := pslog.NewWithOptions(&buf, pslog.Options{
		Mode:             pslog.ModeConsole,
		ForceColor:       true,
		DisableTimestamp: true,
	})

	logger.Info("event", "err", errors.New("disk full"))
	line := buf.String()
	if !strings.Contains(line, "\x1b") {
		t.Fatalf("expected ansi colored output, got %q", line)
	}
	if !strings.Contains(line, "\x1b[36merr=\x1b[0m\x1b[1;31m\"disk full\"\x1b[0m") {
		t.Fatalf("expected quoted colored error field, got %q", line)
	}
}

func TestConsoleColorMatchesPlain(t *testing.T) {
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

	plain := pslog.NewWithOptions(&plainBuf, pslog.Options{Mode: pslog.ModeConsole, DisableTimestamp: true, NoColor: true})
	color := pslog.NewWithOptions(&colorBuf, pslog.Options{Mode: pslog.ModeConsole, DisableTimestamp: true, ForceColor: true})

	plain.Info("event", fields...)
	color.Info("event", fields...)

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

func TestConsoleQuotingDelAndHighBit(t *testing.T) {
	cases := []struct {
		name  string
		value string
		sub   string
	}{
		{"del", "has\x7fdel", "value=\"has\\x7fdel\""},
		{"highbit", string([]byte{'h', 'i', 0x80, 'g', 'h'}), "value=\"hi\\x80gh\""},
	}

	for _, tc := range cases {
		var plainBuf, colorBuf bytes.Buffer
		plain := pslog.NewWithOptions(&plainBuf, pslog.Options{Mode: pslog.ModeConsole, DisableTimestamp: true, NoColor: true})
		color := pslog.NewWithOptions(&colorBuf, pslog.Options{Mode: pslog.ModeConsole, DisableTimestamp: true, ForceColor: true})

		plain.Info("event", "value", tc.value)
		color.Info("event", "value", tc.value)

		plainLine := strings.TrimSpace(plainBuf.String())
		if !strings.Contains(plainLine, tc.sub) {
			t.Fatalf("%s plain output missing %q: %q", tc.name, tc.sub, plainLine)
		}

		colorLine := colorBuf.String()
		if !strings.Contains(colorLine, "\x1b") {
			t.Fatalf("%s color output missing ansi: %q", tc.name, colorLine)
		}
		stripped := stripANSI(colorLine)
		if !strings.Contains(stripped, tc.sub) {
			t.Fatalf("%s color output missing %q after strip: %q", tc.name, tc.sub, stripped)
		}
	}
}
