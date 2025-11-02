package pslog

import "testing"

func TestMakeKeyDataRespectsTrust(t *testing.T) {
	safe := makeKeyData("safe", false)
	if got := string(safe); got != "\"safe\":" {
		t.Fatalf("safe key mismatch: %q", got)
	}

	withComma := makeKeyData("safe", true)
	if got := string(withComma); got != ",\"safe\":" {
		t.Fatalf("comma variant mismatch: %q", got)
	}

	escaped := makeKeyData("needs\n", false)
	if got := string(escaped); got != "\"needs\\n\":" {
		t.Fatalf("escaped key mismatch: %q", got)
	}
}

func TestAppendKeyDataWithFirst(t *testing.T) {
	lw := acquireLineWriter(nil)
	lw.autoFlush = false
	defer releaseLineWriter(lw)

	first := true
	appendKeyDataWithFirst(lw, &first, makeKeyData("first", false))
	appendKeyDataWithFirst(lw, &first, makeKeyData("second", true))

	if got := string(lw.buf); got != "\"first\":,\"second\":" {
		t.Fatalf("unexpected concatenation: %q", got)
	}
}
