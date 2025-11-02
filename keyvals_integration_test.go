package pslog

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func extractLines(buf *bytes.Buffer) []string {
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

func TestKeyvalsStructuredLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithOptions(&buf, Options{Mode: ModeStructured, DisableTimestamp: true, NoColor: true, MinLevel: DebugLevel}).With("app", "demo")

	kv := Keyvals("user", "alice", "note", "multi\nline")
	logger.Info("hello", kv...)
	logger.Warn("bye", kv...)

	lines := extractLines(&buf)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	for i, line := range lines {
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			t.Fatalf("line %d invalid JSON: %v", i, err)
		}
		if payload["app"] != "demo" {
			t.Fatalf("line %d missing app field: %v", i, payload)
		}
		if payload["user"] != "alice" {
			t.Fatalf("line %d user mismatch: %v", i, payload["user"])
		}
		if payload["note"] != "multi\nline" {
			t.Fatalf("line %d note mismatch: %v", i, payload["note"])
		}
		msgKey := "msg"
		if _, ok := payload[msgKey]; !ok {
			t.Fatalf("line %d missing msg field", i)
		}
	}
}

func TestKeyvalsConsoleLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithOptions(&buf, Options{Mode: ModeConsole, DisableTimestamp: true, NoColor: true, MinLevel: DebugLevel}).With("app", "demo")

	kv := Keyvals("user", "alice", "note", "multi\nline")
	logger.Info("hello", kv...)

	lines := extractLines(&buf)
	if len(lines) != 1 {
		t.Fatalf("expected single console line, got %d", len(lines))
	}
	line := lines[0]
	if !strings.Contains(line, "app=demo") {
		t.Fatalf("console line missing app field: %s", line)
	}
	if !strings.Contains(line, "user=alice") {
		t.Fatalf("console line missing user field: %s", line)
	}
	if !strings.Contains(line, "note=\"multi\\nline\"") {
		t.Fatalf("console line missing escaped note field: %s", line)
	}
}
