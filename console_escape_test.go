package pslog

import (
	"strings"
	"testing"
)

func TestFirstConsoleUnsafeIndexUnsafeBytes(t *testing.T) {
	unsafeBytes := []byte{0x00, 0x1f, 0x20, '"', '\\', 0x7f, 0x80}
	for _, b := range unsafeBytes {
		t.Run(hexByteName(b), func(t *testing.T) {
			got := firstConsoleUnsafeIndex(string([]byte{b}))
			if got != 0 {
				t.Fatalf("firstConsoleUnsafeIndex(0x%02x) = %d, want 0", b, got)
			}
		})
	}

	safe := "console-safe"
	if got := firstConsoleUnsafeIndex(safe); got != len(safe) {
		t.Fatalf("firstConsoleUnsafeIndex(%q) = %d, want %d", safe, got, len(safe))
	}

	composite := strings.Repeat("a", 17) + "\x7f" + "tail"
	if got := firstConsoleUnsafeIndex(composite); got != 17 {
		t.Fatalf("firstConsoleUnsafeIndex composite = %d, want 17", got)
	}
}

func hexByteName(b byte) string {
	const hexdigits = "0123456789abcdef"
	return "byte_0x" + string([]byte{hexdigits[b>>4], hexdigits[b&0x0f]})
}
