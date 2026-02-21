package pslog

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"pkt.systems/pslog/ansi"
)

func TestConsoleColorEmitVariants(t *testing.T) {
	keyvals := []any{
		"str", TrustedString("value"),
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
		name           string
		opts           Options
		withBase       bool
		withLogLevel   bool
		expectContains []string
	}{
		{
			name:           "timestamp_loglevel_with_base",
			opts:           Options{Mode: ModeConsole, ForceColor: true, MinLevel: TraceLevel},
			withBase:       true,
			withLogLevel:   true,
			expectContains: []string{"app=demo", "loglevel="},
		},
		{
			name:           "timestamp_loglevel_no_base",
			opts:           Options{Mode: ModeConsole, ForceColor: true, MinLevel: TraceLevel},
			withLogLevel:   true,
			expectContains: []string{"loglevel="},
		},
		{
			name:           "timestamp_only_with_base",
			opts:           Options{Mode: ModeConsole, ForceColor: true, MinLevel: TraceLevel},
			withBase:       true,
			expectContains: []string{"app=demo"},
		},
		{
			name:           "loglevel_only_with_base",
			opts:           Options{Mode: ModeConsole, DisableTimestamp: true, ForceColor: true, MinLevel: TraceLevel},
			withBase:       true,
			withLogLevel:   true,
			expectContains: []string{"app=demo", "loglevel="},
		},
		{
			name:           "loglevel_only_no_base",
			opts:           Options{Mode: ModeConsole, DisableTimestamp: true, ForceColor: true, MinLevel: TraceLevel},
			withLogLevel:   true,
			expectContains: []string{"loglevel="},
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
			out := stripANSIString(buf.String())
			if out == "" {
				t.Fatalf("expected output for combo %s", combo.name)
			}
			for _, want := range combo.expectContains {
				if !strings.Contains(out, want) {
					t.Fatalf("output missing %q: %s", want, out)
				}
			}
		})
	}
}

func TestConsoleColorRuntimeSlowPath(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithOptions(nil, &buf, Options{Mode: ModeConsole, ForceColor: true, DisableTimestamp: true})
	logger.Info("slow", 123, "value", "msg", "ok")
	out := stripANSIString(buf.String())
	if !strings.Contains(out, "123=") {
		t.Fatalf("expected numeric key for integer: %s", out)
	}
}

func TestConsoleColorValueEncoders(t *testing.T) {
	palette := &ansi.PaletteDefault
	lw := acquireLineWriter(io.Discard)
	lw.autoFlush = false
	defer releaseLineWriter(lw)

	values := []any{
		TrustedString("trusted"),
		"plain",
		true,
		int64(-9),
		uint64(9),
		float64(1.25),
		[]byte("bytes"),
		time.Second,
		time.Unix(1700000000, 0).UTC(),
		stubStringer{"stringer"},
		stubError{"err"},
		nil,
	}

	for _, value := range values {
		lw.buf = lw.buf[:0]
		if !writeConsoleValueColorFast(lw, value, palette) {
			t.Fatalf("fast path rejected %T", value)
		}
	}

	for _, value := range values {
		lw.buf = lw.buf[:0]
		if !writeConsoleValueColorInline(lw, value, palette) {
			t.Fatalf("inline path rejected %T", value)
		}
	}

	for _, value := range append(values, map[string]any{"k": "v"}) {
		lw.buf = lw.buf[:0]
		writeConsoleValueColor(lw, value, palette)
		if len(lw.buf) == 0 {
			t.Fatalf("no output for %T", value)
		}
	}
}

func TestAppendConsoleValueColorHelpers(t *testing.T) {
	palette := &ansi.PaletteDefault
	buf := make([]byte, 0, 128)
	buf = appendConsoleValueColor(buf, "value", palette)
	buf = appendConsoleValueColor(buf, true, palette)
	buf = appendConsoleValueColor(buf, int64(-3), palette)
	buf = appendConsoleValueColor(buf, uint64(3), palette)
	buf = appendConsoleValueColor(buf, 1.5, palette)
	buf = appendConsoleValueColor(buf, []byte("bytes"), palette)
	buf = appendColoredLiteral(buf, "true", palette.Bool)
	buf = appendColoredInt(buf, -4, palette.Num)
	buf = appendColoredUint(buf, 4, palette.Num)
	buf = appendColoredFloat(buf, 2.5, palette.Num)

	if !strings.Contains(string(buf), "true") {
		t.Fatalf("appended buffer missing literal data")
	}
}

func TestConsoleLevelColorAllBranches(t *testing.T) {
	palette := &ansi.PaletteDefault
	levels := []Level{TraceLevel, DebugLevel, InfoLevel, WarnLevel, ErrorLevel, FatalLevel, PanicLevel, NoLevel, Level(99)}
	for _, lvl := range levels {
		color, label := consoleLevelColor(lvl, palette)
		if color == "" || label == "" {
			t.Fatalf("empty result for level %v", lvl)
		}
	}
}

func TestConfigureConsoleScannerInvoked(t *testing.T) {
	configureConsoleScannerFromOptions(Options{ForceColor: true})
}
