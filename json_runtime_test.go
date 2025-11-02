package pslog

import (
	"encoding/json"
	"io"
	"strconv"
	"testing"
	"time"

	"pkt.systems/pslog/ansi"
)

type stubStringer struct {
	value string
}

func (s stubStringer) String() string {
	return s.value
}

type stubError struct {
	value string
}

func (s stubError) Error() string {
	return s.value
}

type fastPathMarshaler struct{}

func (fastPathMarshaler) MarshalJSON() ([]byte, error) {
	return []byte(`"ok"`), nil
}

var _ json.Marshaler = fastPathMarshaler{}

func captureInlinePlain(t *testing.T, value any) (string, bool) {
	t.Helper()
	lw := acquireLineWriter(io.Discard)
	lw.autoFlush = false
	ok := writeRuntimeValuePlainInline(lw, value)
	out := string(lw.buf)
	releaseLineWriter(lw)
	return out, ok
}

func capturePlainFallback(t *testing.T, value any) string {
	t.Helper()
	lw := acquireLineWriter(io.Discard)
	lw.autoFlush = false
	writeJSONValuePlain(lw, value)
	out := string(lw.buf)
	releaseLineWriter(lw)
	return out
}

func captureInlineColor(t *testing.T, value any, color string) (string, bool) {
	t.Helper()
	lw := acquireLineWriter(io.Discard)
	lw.autoFlush = false
	ok := writeRuntimeValueColorInline(lw, value, color)
	out := string(lw.buf)
	releaseLineWriter(lw)
	return out, ok
}

func captureColorFallback(t *testing.T, value any, color string) string {
	t.Helper()
	lw := acquireLineWriter(io.Discard)
	lw.autoFlush = false
	writeJSONValueColored(lw, value, color)
	out := string(lw.buf)
	releaseLineWriter(lw)
	return out
}

func TestWriteRuntimeValuePlainInlineMatchesFallback(t *testing.T) {
	loc := time.FixedZone("UTC+2", 2*60*60)
	now := time.Date(2025, time.November, 1, 12, 34, 56, 789_123_000, loc)
	duration := 1500 * time.Millisecond
	tests := []struct {
		name   string
		value  any
		expect func() string
	}{
		{
			name:  "Time",
			value: now,
			expect: func() string {
				return strconv.Quote(now.Format(time.RFC3339Nano))
			},
		},
		{name: "Duration", value: duration},
		{name: "TrustedString", value: TrustedString("already-safe")},
		{name: "StringNeedsEscape", value: "line\nbreak"},
		{name: "BoolTrue", value: true},
		{name: "Int", value: int64(-99)},
		{name: "Float", value: 3.1415},
		{name: "ByteSliceASCII", value: []byte("hello world")},
		{name: "ByteSliceNeedsEscape", value: []byte("line\nbreak")},
		{name: "StringerASCII", value: stubStringer{"ok-value"}},
		{name: "StringerNeedsEscape", value: stubStringer{"line\nbreak"}},
		{name: "ErrorASCII", value: stubError{"boom"}},
		{name: "ErrorNeedsEscape", value: stubError{"boom\nagain"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inline, ok := captureInlinePlain(t, tt.value)
			if !ok {
				t.Fatalf("writeRuntimeValuePlainInline returned false for %T", tt.value)
			}
			want := ""
			if tt.expect != nil {
				want = tt.expect()
			} else {
				want = capturePlainFallback(t, tt.value)
			}
			if inline != want {
				t.Fatalf("inline output %q != expected %q", inline, want)
			}
		})
	}
}

func TestWriteRuntimeValueColorInlineMatchesFallback(t *testing.T) {
	color := ansi.String
	loc := time.FixedZone("UTC-7", -7*60*60)
	now := time.Date(2025, time.November, 1, 1, 2, 3, 456_789_000, loc)
	tests := []struct {
		name   string
		value  any
		expect func() string
	}{
		{
			name:  "Time",
			value: now,
			expect: func() string {
				quoted := strconv.Quote(now.Format(time.RFC3339Nano))
				return color + quoted + ansi.Reset
			},
		},
		{name: "TrustedString", value: TrustedString("already-safe")},
		{name: "StringNeedsEscape", value: "colored\nvalue"},
		{name: "BoolFalse", value: false},
		{name: "Uint", value: uint64(42)},
		{name: "Float", value: 1.5},
		{name: "ByteSliceASCII", value: []byte("colored")},
		{name: "ByteSliceNeedsEscape", value: []byte("colored\nvalue")},
		{name: "StringerASCII", value: stubStringer{"colored-string"}},
		{name: "StringerNeedsEscape", value: stubStringer{"colored\nstring"}},
		{name: "ErrorASCII", value: stubError{"colored-error"}},
		{name: "ErrorNeedsEscape", value: stubError{"colored\nerror"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inline, ok := captureInlineColor(t, tt.value, color)
			if !ok {
				t.Fatalf("writeRuntimeValueColorInline returned false for %T", tt.value)
			}
			want := ""
			if tt.expect != nil {
				want = tt.expect()
			} else {
				want = captureColorFallback(t, tt.value, color)
			}
			if inline != want {
				t.Fatalf("inline output %q != expected %q", inline, want)
			}
		})
	}
}

func TestWriteRuntimeValuePlainInlineUnsupportedTypes(t *testing.T) {
	lw := acquireLineWriter(io.Discard)
	lw.autoFlush = false
	defer releaseLineWriter(lw)

	cases := []any{
		fastPathMarshaler{},
		[]any{"nested"},
		struct{ X int }{X: 1},
	}
	for _, value := range cases {
		lw.buf = lw.buf[:0]
		if writeRuntimeValuePlainInline(lw, value) {
			t.Fatalf("expected inline fast path to reject %T", value)
		}
		lw.buf = lw.buf[:0]
	}
}

func TestWritePTLogArrayNested(t *testing.T) {
	lw := acquireLineWriter(io.Discard)
	lw.autoFlush = false
	defer releaseLineWriter(lw)

	writePTLogArray(lw, []any{
		"simple",
		[]any{"nested", TrustedString("safe")},
		true,
	})
	got := string(lw.buf)
	want := `["simple",["nested","safe"],true]`
	if got != want {
		t.Fatalf("writePTLogArray mismatch: got %q want %q", got, want)
	}
}
