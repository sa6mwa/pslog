package asmlog

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"pkt.systems/pslog"
)

func TestLoggerProducesJSON(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf)
	fixed := time.Unix(0, 123456789).UTC()
	logger.timeCache.setNow(func() time.Time { return fixed })
	logger.timeCache.setTicker(func(time.Duration) tickerControl { return tickerControl{} })

	logger.Info("hello", "user", "alice", "count", 3, "flag", true)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected one line, got %d: %q", len(lines), buf.String())
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	if payload["lvl"] != pslog.LevelString(pslog.InfoLevel) {
		t.Fatalf("unexpected level: %v", payload["lvl"])
	}
	if payload["msg"] != "hello" {
		t.Fatalf("unexpected message: %v", payload["msg"])
	}
	if want := fixed.Format(time.RFC3339); payload["ts"] != want {
		t.Fatalf("unexpected timestamp: got %v want %v", payload["ts"], want)
	}
	if payload["user"] != "alice" {
		t.Fatalf("unexpected user: %v", payload["user"])
	}
	if v, ok := payload["count"].(float64); !ok || v != 3 {
		t.Fatalf("unexpected count: %#v", payload["count"])
	}
	if flag, ok := payload["flag"].(bool); !ok || !flag {
		t.Fatalf("unexpected flag: %#v", payload["flag"])
	}
}

func TestLoggerHandlesOddKeyvals(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf)
	logger.timeCache.setNow(func() time.Time { return time.Unix(0, 0).UTC() })
	logger.timeCache.setTicker(func(time.Duration) tickerControl { return tickerControl{} })

	logger.Debug("msg", "keyOnly")

	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	if payload["arg0"] != "keyOnly" {
		t.Fatalf("unexpected arg0: %v", payload["arg0"])
	}
}
