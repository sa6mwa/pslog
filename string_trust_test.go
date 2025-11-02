package pslog

import "testing"

func TestStringHasUnsafe(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		unsafe bool
	}{
		{"empty", "", false},
		{"ascii", "hello-world", false},
		{"emoji", "status ✅", false},
		{"quote", "hello\"world", true},
		{"backslash", "path\\name", true},
		{"control", "line\nbreak", true},
		{"del", string([]byte{0x7f}), true},
		{"high-bit", string([]byte{0x80}), true},
		{"invalid UTF-8", string([]byte{0xff, 0xfe}), true},
	}
	for _, tc := range cases {
		if got := stringHasUnsafe(tc.in); got != tc.unsafe {
			t.Fatalf("%s: stringHasUnsafe(%q) = %v, want %v", tc.name, tc.in, got, tc.unsafe)
		}
	}
}

func TestPromoteTrustedValueString(t *testing.T) {
	if !promoteTrustedValueString("hello") {
		t.Fatal("expected ASCII string to be trusted")
	}
	if promoteTrustedValueString("line\nbreak") {
		t.Fatal("expected control characters to be unsafe")
	}
	if promoteTrustedValueString(string([]byte{0xff})) {
		t.Fatal("expected invalid utf-8 to be unsafe")
	}
}
