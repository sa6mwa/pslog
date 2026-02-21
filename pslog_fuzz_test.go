package pslog_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"pkt.systems/pslog"
)

var jsonEscapingSeeds = []struct {
	name  string
	msg   string
	key   string
	value string
}{
	{"plain", "hello", "key", "value"},
	{"quotes", `"quoted" message`, `quote"key`, `value"with"quotes`},
	{"braces", "ends } braces", "brace}", `{"evil":1}`},
	{"controls", "line\nfeed\tand\\slash", "new\nline", "tab\tvalue"},
	{"unicode", "emoji 😃", "control" + string(rune(0)), "snowman ☃"},
	{"del", "include\x7fdel", "key\x7f", "value\x7f"},
}

func TestStructuredJSONEscaping(t *testing.T) {
	for _, tc := range jsonEscapingSeeds {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := pslog.NewWithOptions(nil, &buf, pslog.Options{Mode: pslog.ModeStructured, DisableTimestamp: true, NoColor: true})
			logger = logger.With("seed", tc.name)
			logger.Info(tc.msg, tc.key, tc.value)

			line := strings.TrimSpace(buf.String())
			if line == "" {
				t.Fatalf("empty structured output")
			}
			if !strings.HasPrefix(line, "{") {
				t.Fatalf("expected json object, got %q", line)
			}

			var payload map[string]any
			if err := json.Unmarshal([]byte(line), &payload); err != nil {
				t.Fatalf("invalid json output: %v for line %q", err, line)
			}
			if payload["msg"] != tc.msg {
				t.Fatalf("message mismatch: got %q want %q", payload["msg"], tc.msg)
			}
		})
	}
}

func FuzzLogVariants(f *testing.F) {
	for _, seed := range jsonEscapingSeeds {
		f.Add(seed.msg, seed.key, seed.value)
	}
	f.Add("", "", "")

	f.Fuzz(func(t *testing.T, msg, key, value string) {
		safeKey := sanitizeKey(key)

		var (
			structuredPlainBuf bytes.Buffer
			structuredColorBuf bytes.Buffer
			consolePlainBuf    bytes.Buffer
			consoleColorBuf    bytes.Buffer
		)

		plainStructured := pslog.NewWithOptions(nil, &structuredPlainBuf, pslog.Options{Mode: pslog.ModeStructured, DisableTimestamp: true, NoColor: true, MinLevel: pslog.DebugLevel}).With("origin", "fuzz")
		colorStructured := pslog.NewWithOptions(nil, &structuredColorBuf, pslog.Options{Mode: pslog.ModeStructured, DisableTimestamp: true, ForceColor: true, MinLevel: pslog.DebugLevel}).With("origin", "fuzz")
		plainConsole := pslog.NewWithOptions(nil, &consolePlainBuf, pslog.Options{Mode: pslog.ModeConsole, DisableTimestamp: true, NoColor: true, MinLevel: pslog.DebugLevel}).With("origin", "fuzz")
		colorConsole := pslog.NewWithOptions(nil, &consoleColorBuf, pslog.Options{Mode: pslog.ModeConsole, DisableTimestamp: true, ForceColor: true, MinLevel: pslog.DebugLevel}).With("origin", "fuzz")

		type pair struct {
			key any
			val any
		}
		var (
			fields []any
			pairs  []pair
		)
		addField := func(k, v any) {
			fields = append(fields, k, v)
			pairs = append(pairs, pair{key: k, val: v})
		}

		addField(safeKey, value)
		if trusted, ok := pslog.NewTrustedString(value); ok {
			addField("value_trusted", trusted)
		} else {
			addField("value_trusted", value)
		}
		addField("int", len(value))
		addField("float", float64(len(msg))+0.5)
		addField("bool", len(value)%2 == 0)
		addField("nil_field", nil)
		addField("bytes", []byte(value))
		addField("map", map[string]any{"value": value, "len": len(value)})
		addField("slice", []any{value, len(value)})
		addField("err", errors.New("disk full: "+value))
		addField("duration", time.Duration(len(value))*time.Millisecond)
		addField("time", time.Unix(int64(len(value)), int64(len(msg))*1e6).UTC())
		addField(pslog.TrustedString("trusted_key"), pslog.TrustedString("trusted value"))
		addField("stringer", sampleStringer{value})

		plainStructured.Info(msg, fields...)
		colorStructured.Info(msg, fields...)
		plainConsole.Info(msg, fields...)
		colorConsole.Info(msg, fields...)

		plainJSON := strings.TrimSpace(structuredPlainBuf.String())
		colorJSON := strings.TrimSpace(stripANSI(structuredColorBuf.String()))
		if plainJSON == "" || colorJSON == "" {
			t.Fatalf("empty structured output plain=%q color=%q", plainJSON, colorJSON)
		}
		if plainJSON != colorJSON {
			t.Fatalf("structured color output mismatch:\nplain=%s\ncolor=%s", plainJSON, colorJSON)
		}

		var gotPlain map[string]any
		if err := json.Unmarshal([]byte(plainJSON), &gotPlain); err != nil {
			t.Fatalf("invalid json output (plain): %v for %q", err, plainJSON)
		}
		var gotColor map[string]any
		if err := json.Unmarshal([]byte(colorJSON), &gotColor); err != nil {
			t.Fatalf("invalid json output (color): %v for %q", err, colorJSON)
		}
		if !reflect.DeepEqual(gotPlain, gotColor) {
			t.Fatalf("structured plain/color mismatch:\nplain=%v\ncolor=%v", gotPlain, gotColor)
		}

		expected := map[string]any{"lvl": "info", "origin": "fuzz"}
		if msg != "" {
			expected["msg"] = msg
		}
		for _, p := range pairs {
			expected[normalizeKey(p.key)] = normalizeValue(p.val)
		}
		refJSON, err := json.Marshal(expected)
		if err != nil {
			t.Fatalf("failed to marshal reference json: %v", err)
		}
		var want map[string]any
		if err := json.Unmarshal(refJSON, &want); err != nil {
			t.Fatalf("failed to decode reference json: %v", err)
		}
		if !reflect.DeepEqual(gotPlain, want) {
			t.Fatalf("structured json parity mismatch:\n got %v\nwant %v\nline=%s\nref=%s", gotPlain, want, plainJSON, string(refJSON))
		}

		plainConsoleLine := strings.TrimSpace(stripANSI(consolePlainBuf.String()))
		colorConsoleLine := strings.TrimSpace(stripANSI(consoleColorBuf.String()))
		if plainConsoleLine == "" || colorConsoleLine == "" {
			t.Fatalf("empty console output plain=%q color=%q", plainConsoleLine, colorConsoleLine)
		}
		if plainConsoleLine != colorConsoleLine {
			t.Fatalf("console color output mismatch:\nplain=%s\ncolor=%s", plainConsoleLine, colorConsoleLine)
		}
	})
}

func FuzzLoggerContextCancellation(f *testing.F) {
	f.Add(uint8(0), "hello", "key", "value")
	f.Add(uint8(3), "line\nfeed", "key\x7f", `{"a":1}`)
	f.Add(uint8(255), "", "", "")

	f.Fuzz(func(t *testing.T, mask uint8, msg, key, value string) {
		variants := []pslog.Options{
			{Mode: pslog.ModeStructured, NoColor: true, TimeFormat: time.RFC3339, MinLevel: pslog.DebugLevel},
			{Mode: pslog.ModeStructured, ForceColor: true, TimeFormat: time.RFC3339, MinLevel: pslog.DebugLevel},
			{Mode: pslog.ModeConsole, NoColor: true, TimeFormat: time.RFC3339, MinLevel: pslog.DebugLevel},
			{Mode: pslog.ModeConsole, ForceColor: true, TimeFormat: time.RFC3339, MinLevel: pslog.DebugLevel},
		}

		for i, opts := range variants {
			ctx, cancel := context.WithCancel(context.Background())
			var buf bytes.Buffer
			logger := pslog.NewWithOptions(ctx, &buf, opts).With("variant", i)
			if mask&(1<<uint(i)) != 0 {
				cancel()
			}

			logger.Info(msg, sanitizeKey(key), value)
			if mask&(1<<uint(i+4)) != 0 {
				cancel()
			}
			logger.Warn("post", "k", value)
			if closer, ok := logger.(interface{ Close() error }); ok && mask&(1<<uint(i+8)) != 0 {
				_ = closer.Close()
			}
			cancel()

			if opts.Mode != pslog.ModeStructured {
				continue
			}
			output := strings.TrimSpace(stripANSI(buf.String()))
			if output == "" {
				continue
			}
			for _, line := range strings.Split(output, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				var payload map[string]any
				if err := json.Unmarshal([]byte(line), &payload); err != nil {
					t.Fatalf("invalid structured output for variant %d: %v (%q)", i, err, line)
				}
			}
		}
	})
}

type sampleStringer struct {
	value string
}

func (s sampleStringer) String() string { return "stringer:" + s.value }

func sanitizeKey(k string) string {
	if k == "" {
		return "key"
	}
	var b strings.Builder
	b.Grow(len(k))
	for _, r := range k {
		if r < 0x20 || r == '=' || r == ' ' || r == '\\' || r == '"' || r == '\t' || r == '\n' || r == '\r' || r == 0x1b {
			b.WriteByte('_')
			continue
		}
		b.WriteRune(r)
	}
	if b.Len() == 0 {
		return "key"
	}
	return b.String()
}

func normalizeKey(k any) string {
	switch key := k.(type) {
	case string:
		return sanitizeKey(key)
	case pslog.TrustedString:
		return sanitizeKey(string(key))
	default:
		return fmt.Sprint(key)
	}
}

func normalizeValue(v any) any {
	switch val := v.(type) {
	case pslog.TrustedString:
		return string(val)
	case []byte:
		return string(val)
	case error:
		return val.Error()
	case time.Time:
		return val.Format(time.RFC3339Nano)
	case time.Duration:
		return val.String()
	case sampleStringer:
		return val.String()
	case interface{ String() string }:
		return val.String()
	default:
		return val
	}
}
