package pslog

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

func TestConsolePlainEmitVariants(t *testing.T) {
	keyvals := []any{
		"str", "value",
		"bool", true,
		"int", int64(-5),
		"uint", uint(7),
		"float", 3.5,
		"bytes", []byte("raw"),
		"dur", 1500 * time.Millisecond,
		"time", time.Unix(1700000000, 0).UTC(),
		"stringer", stubStringer{"stringer"},
		"error", stubError{"boom"},
		"nil", nil,
	}

	combos := []struct {
		name         string
		opts         Options
		withBase     bool
		withLogLevel bool
		expect       []string
	}{
		{
			name:         "timestamp_loglevel_with_base",
			opts:         Options{Mode: ModeConsole, MinLevel: TraceLevel, NoColor: true},
			withBase:     true,
			withLogLevel: true,
			expect:       []string{"app=demo", "loglevel="},
		},
		{
			name:         "timestamp_loglevel_no_base",
			opts:         Options{Mode: ModeConsole, MinLevel: TraceLevel, NoColor: true},
			withLogLevel: true,
			expect:       []string{"loglevel="},
		},
		{
			name:     "timestamp_with_base",
			opts:     Options{Mode: ModeConsole, MinLevel: TraceLevel, NoColor: true},
			withBase: true,
			expect:   []string{"app=demo"},
		},
		{
			name:         "loglevel_with_base",
			opts:         Options{Mode: ModeConsole, MinLevel: TraceLevel, NoColor: true, DisableTimestamp: true},
			withBase:     true,
			withLogLevel: true,
			expect:       []string{"app=demo", "loglevel="},
		},
		{
			name:         "loglevel_no_base",
			opts:         Options{Mode: ModeConsole, MinLevel: TraceLevel, NoColor: true, DisableTimestamp: true},
			withLogLevel: true,
			expect:       []string{"loglevel="},
		},
	}

	for _, combo := range combos {
		t.Run(combo.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewWithOptions(nil, &buf, combo.opts)
			if combo.withBase {
				logger = logger.With("app", "demo")
			}
			if combo.withLogLevel {
				logger = logger.WithLogLevel()
			}
			logger.Log(InfoLevel, "hello", keyvals...)
			out := buf.String()
			if out == "" {
				t.Fatalf("expected output")
			}
			for _, want := range combo.expect {
				if !strings.Contains(out, want) {
					t.Fatalf("output missing %q: %s", want, out)
				}
			}
		})
	}
}

func TestConsolePlainRuntimeSlowPath(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithOptions(nil, &buf, Options{Mode: ModeConsole, NoColor: true, DisableTimestamp: true})
	logger.Info("slow", 123, "value", "msg", "ok")
	out := buf.String()
	if !strings.Contains(out, "123=") {
		t.Fatalf("expected numeric key, got %s", out)
	}
}

func TestConsolePlainValueEncoders(t *testing.T) {
	lw := acquireLineWriter(io.Discard)
	lw.autoFlush = false
	defer releaseLineWriter(lw)

	values := []any{
		TrustedString("trusted"),
		"plain",
		true,
		int64(-4),
		uint64(4),
		float64(1.5),
		[]byte("bytes"),
		time.Second,
		time.Unix(1700000000, 0).UTC(),
		stubStringer{"stringer"},
		stubError{"err"},
		nil,
	}

	for _, value := range values {
		lw.buf = lw.buf[:0]
		if !writeConsoleValueFast(lw, value) {
			t.Fatalf("fast path rejected %T", value)
		}
	}

	for _, value := range values {
		lw.buf = lw.buf[:0]
		if !writeConsoleValueInline(lw, value) {
			t.Fatalf("inline path rejected %T", value)
		}
	}

	for _, value := range append(values, map[string]any{"k": "v"}) {
		lw.buf = lw.buf[:0]
		writeConsoleValuePlain(lw, value)
		if len(lw.buf) == 0 {
			t.Fatalf("no output for %T", value)
		}
	}
}

func TestAppendConsoleValuePlainHelpers(t *testing.T) {
	buf := make([]byte, 0, 128)
	buf = appendConsoleValuePlain(buf, "value")
	buf = appendConsoleValuePlain(buf, true)
	buf = appendConsoleValuePlain(buf, int64(-3))
	buf = appendConsoleValuePlain(buf, uint64(3))
	buf = appendConsoleValuePlain(buf, 1.5)
	buf = appendConsoleValuePlain(buf, []byte("bytes"))

	if !strings.Contains(string(buf), "value") {
		t.Fatalf("append buffer missing content: %s", string(buf))
	}
}
