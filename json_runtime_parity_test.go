package pslog

import (
	"io"
	"strings"
	"testing"

	"pkt.systems/pslog/ansi"
)

func TestJSONRuntimePlainFallbackParity(t *testing.T) {
	keyvals := []any{"first", "ok", 123, "value", "tail", true}
	wrapper := renderRuntimeJSONPlain(keyvals)
	slow := renderRuntimeJSONPlainSlow(keyvals)
	if wrapper != slow {
		t.Fatalf("wrapper/slow mismatch\nwrapper=%q\nslow=%q", wrapper, slow)
	}
	if strings.Count(wrapper, `"first"`) != 1 {
		t.Fatalf("expected first key once, got %q", wrapper)
	}
}

func TestJSONRuntimeColorFallbackParity(t *testing.T) {
	keyvals := []any{"first", "ok", 123, "value", "tail", true}
	palette := &ansi.PaletteDefault
	wrapper := stripANSIString(renderRuntimeJSONColor(keyvals, palette))
	slow := stripANSIString(renderRuntimeJSONColorSlow(keyvals, palette))
	if wrapper != slow {
		t.Fatalf("wrapper/slow mismatch\nwrapper=%q\nslow=%q", wrapper, slow)
	}
	if strings.Count(wrapper, `"first"`) != 1 {
		t.Fatalf("expected first key once, got %q", wrapper)
	}
}

func renderRuntimeJSONPlain(keyvals []any) string {
	lw := acquireLineWriter(io.Discard)
	lw.autoFlush = false
	defer releaseLineWriter(lw)
	lw.writeByte('{')
	first := true
	writeRuntimeJSONFieldsPlain(lw, &first, keyvals)
	lw.writeByte('}')
	return string(lw.buf)
}

func renderRuntimeJSONPlainSlow(keyvals []any) string {
	lw := acquireLineWriter(io.Discard)
	lw.autoFlush = false
	defer releaseLineWriter(lw)
	lw.writeByte('{')
	first := true
	writeRuntimeJSONFieldsPlainSlow(lw, &first, keyvals)
	lw.writeByte('}')
	return string(lw.buf)
}

func renderRuntimeJSONColor(keyvals []any, palette *ansi.Palette) string {
	lw := acquireLineWriter(io.Discard)
	lw.autoFlush = false
	defer releaseLineWriter(lw)
	lw.writeByte('{')
	first := true
	writeRuntimeJSONFieldsColor(lw, &first, keyvals, palette)
	lw.writeByte('}')
	return string(lw.buf)
}

func renderRuntimeJSONColorSlow(keyvals []any, palette *ansi.Palette) string {
	lw := acquireLineWriter(io.Discard)
	lw.autoFlush = false
	defer releaseLineWriter(lw)
	lw.writeByte('{')
	first := true
	writeRuntimeJSONFieldsColorSlow(lw, &first, keyvals, palette)
	lw.writeByte('}')
	return string(lw.buf)
}
