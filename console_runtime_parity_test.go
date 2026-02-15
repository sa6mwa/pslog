package pslog

import (
	"io"
	"strings"
	"testing"
	"time"

	"pkt.systems/pslog/ansi"
)

func TestConsolePlainRuntimeParity(t *testing.T) {
	cases := []struct {
		name    string
		keyvals []any
	}{
		{name: "empty", keyvals: nil},
		{name: "strings", keyvals: []any{"k", "v", "n", int64(-5)}},
		{name: "trusted_keys", keyvals: []any{TrustedString("k"), "v", TrustedString("n"), uint(7)}},
		{name: "odd_tail", keyvals: []any{"k", "v", 99}},
		{
			name: "mixed_values",
			keyvals: []any{
				"str", "value",
				"bool", true,
				"int", int64(-5),
				"uint", uint64(7),
				"float", 3.5,
				"bytes", []byte("raw"),
				"dur", 1500 * time.Millisecond,
				"time", time.Unix(1700000000, 0).UTC(),
				"map", map[string]any{"k": "v"},
				"nil", nil,
			},
		},
		{name: "empty_key_skipped", keyvals: []any{"", "skip", "ok", "value"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fastOut, ok := renderRuntimePlainFast(tc.keyvals)
			if !ok {
				t.Fatalf("fast path unexpectedly rejected case")
			}
			slowOut := renderRuntimePlainSlow(tc.keyvals)
			if fastOut != slowOut {
				t.Fatalf("fast/slow mismatch\nfast=%q\nslow=%q", fastOut, slowOut)
			}
		})
	}
}

func TestConsoleColorRuntimeParity(t *testing.T) {
	palette := &ansi.PaletteDefault
	cases := []struct {
		name    string
		keyvals []any
	}{
		{name: "empty", keyvals: nil},
		{name: "strings", keyvals: []any{"k", "v", "n", int64(-5)}},
		{name: "trusted_keys", keyvals: []any{TrustedString("k"), "v", TrustedString("n"), uint(7)}},
		{name: "odd_tail", keyvals: []any{"k", "v", 99}},
		{
			name: "mixed_values",
			keyvals: []any{
				"str", "value",
				"bool", true,
				"int", int64(-5),
				"uint", uint64(7),
				"float", 3.5,
				"bytes", []byte("raw"),
				"dur", 1500 * time.Millisecond,
				"time", time.Unix(1700000000, 0).UTC(),
				"map", map[string]any{"k": "v"},
				"nil", nil,
			},
		},
		{name: "empty_key_skipped", keyvals: []any{"", "skip", "ok", "value"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fastOut, ok := renderRuntimeColorFast(tc.keyvals, palette)
			if !ok {
				t.Fatalf("fast path unexpectedly rejected case")
			}
			slowOut := renderRuntimeColorSlow(tc.keyvals, palette)
			if fastOut != slowOut {
				t.Fatalf("fast/slow mismatch\nfast=%q\nslow=%q", fastOut, slowOut)
			}
		})
	}
}

func TestConsolePlainRuntimeFallbackParity(t *testing.T) {
	keyvals := []any{"first", "ok", 123, "v", "tail", true}
	wrapperOut := renderRuntimePlain(keyvals)
	slowOut := renderRuntimePlainSlow(keyvals)
	if wrapperOut != slowOut {
		t.Fatalf("wrapper/slow mismatch\nwrapper=%q\nslow=%q", wrapperOut, slowOut)
	}
	if strings.Count(wrapperOut, "first=") != 1 {
		t.Fatalf("expected first field once, got %q", wrapperOut)
	}
}

func TestConsoleColorRuntimeFallbackParity(t *testing.T) {
	palette := &ansi.PaletteDefault
	keyvals := []any{"first", "ok", 123, "v", "tail", true}
	wrapperOut := stripANSIString(renderRuntimeColor(keyvals, palette))
	slowOut := stripANSIString(renderRuntimeColorSlow(keyvals, palette))
	if wrapperOut != slowOut {
		t.Fatalf("wrapper/slow mismatch\nwrapper=%q\nslow=%q", wrapperOut, slowOut)
	}
	if strings.Count(wrapperOut, "first=") != 1 {
		t.Fatalf("expected first field once, got %q", wrapperOut)
	}
}

func renderRuntimePlain(keyvals []any) string {
	lw := acquireLineWriter(io.Discard)
	lw.autoFlush = false
	defer releaseLineWriter(lw)
	writeRuntimeConsolePlain(lw, keyvals)
	return string(lw.buf)
}

func renderRuntimePlainFast(keyvals []any) (string, bool) {
	lw := acquireLineWriter(io.Discard)
	lw.autoFlush = false
	defer releaseLineWriter(lw)
	ok := writeRuntimeConsolePlainFast(lw, keyvals)
	return string(lw.buf), ok
}

func renderRuntimePlainSlow(keyvals []any) string {
	lw := acquireLineWriter(io.Discard)
	lw.autoFlush = false
	defer releaseLineWriter(lw)
	writeRuntimeConsolePlainSlow(lw, keyvals)
	return string(lw.buf)
}

func renderRuntimeColor(keyvals []any, palette *ansi.Palette) string {
	lw := acquireLineWriter(io.Discard)
	lw.autoFlush = false
	defer releaseLineWriter(lw)
	writeRuntimeConsoleColor(lw, keyvals, palette)
	return string(lw.buf)
}

func renderRuntimeColorFast(keyvals []any, palette *ansi.Palette) (string, bool) {
	lw := acquireLineWriter(io.Discard)
	lw.autoFlush = false
	defer releaseLineWriter(lw)
	ok := writeRuntimeConsoleColorFast(lw, keyvals, palette)
	return string(lw.buf), ok
}

func renderRuntimeColorSlow(keyvals []any, palette *ansi.Palette) string {
	lw := acquireLineWriter(io.Discard)
	lw.autoFlush = false
	defer releaseLineWriter(lw)
	writeRuntimeConsoleColorSlow(lw, keyvals, palette)
	return string(lw.buf)
}
