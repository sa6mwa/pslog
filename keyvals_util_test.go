package pslog

import (
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"
)

type testStringer struct{ value string }

func (t testStringer) String() string { return t.value }

type customMarshaler struct{ payload any }

func (c customMarshaler) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.payload)
}

type errorMarshaler struct{}

func (errorMarshaler) MarshalJSON() ([]byte, error) {
	return nil, errors.New("marshal failure")
}

func TestKeyFromAnyScalars(t *testing.T) {
	now := time.Unix(1700000000, 123456789).UTC()
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{"string", "key", "key"},
		{"stringer", testStringer{"value"}, "value"},
		{"error", errors.New("boom"), "boom"},
		{"bytes", []byte("raw"), "raw"},
		{"boolTrue", true, "true"},
		{"boolFalse", false, "false"},
		{"int", int(-42), "-42"},
		{"uint", uint(42), "42"},
		{"float", 3.25, strconv.FormatFloat(3.25, 'g', -1, 64)},
		{"duration", 1500 * time.Millisecond, (1500 * time.Millisecond).String()},
		{"time", now, now.Format(time.RFC3339Nano)},
		{"jsonNumber", json.Number("99.5"), "99.5"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := keyFromAny(tc.input)
			if !ok {
				t.Fatalf("expected ok for %s", tc.name)
			}
			if got != tc.want {
				t.Fatalf("key mismatch: got %q want %q", got, tc.want)
			}
		})
	}

	if _, ok := keyFromAny(struct{}{}); ok {
		t.Fatalf("expected struct{} to be unsupported")
	}
}

func TestStringFromAnyFallbacks(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	if got := stringFromAny(nil); got != "null" {
		t.Fatalf("nil mismatch: %q", got)
	}

	if got := stringFromAny(customMarshaler{payload: map[string]int{"a": 1}}); got != `{"a":1}` {
		t.Fatalf("custom marshaler mismatch: %q", got)
	}

	if got := stringFromAny(errorMarshaler{}); got != "marshal failure" {
		t.Fatalf("error marshaler mismatch: %q", got)
	}

	type payload struct {
		Time time.Time `json:"time"`
		Num  int       `json:"num"`
	}
	wantStruct := payload{Time: now, Num: 7}
	got := stringFromAny(wantStruct)
	expected := `{"time":"` + now.Format(time.RFC3339Nano) + `","num":7}`
	if got != expected {
		t.Fatalf("struct fallback mismatch: got %q want %q", got, expected)
	}
}

func TestArgKeyName(t *testing.T) {
	for i := range 5 {
		if want := "arg" + strconv.Itoa(i); argKeyName(i) != want {
			t.Fatalf("unexpected arg key for %d", i)
		}
	}
}

func TestKeyvalsPromotesTrustedKeys(t *testing.T) {
	payload := Keyvals(
		"ascii", "value",
		"needs\nescape", "value2",
		testStringer{"key"}, "stringer-value",
		[]byte("bin"), "bytes",
		123, "number",
		TrustedString("already"), "trusted",
	)

	if _, ok := payload[0].(TrustedString); !ok {
		t.Fatalf("expected ascii key to be promoted to TrustedString: %#v", payload[0])
	}
	if _, ok := payload[2].(TrustedString); ok {
		t.Fatalf("expected newline key to remain plain string")
	}
	if keyStr, ok := payload[4].(TrustedString); !ok || string(keyStr) != "key" {
		t.Fatalf("stringer key mismatch: %q", keyStr)
	}
	if keyStr, ok := payload[6].(TrustedString); !ok || string(keyStr) != "bin" {
		t.Fatalf("expected []byte key to be trusted 'bin', got %#v", payload[6])
	}
	if keyStr, ok := payload[8].(TrustedString); !ok || string(keyStr) != "123" {
		t.Fatalf("numeric key should be trusted string '123', got %#v", payload[8])
	}
	if keyStr, ok := payload[10].(TrustedString); !ok || string(keyStr) != "already" {
		t.Fatalf("TrustedString should be preserved, got %#v", payload[10])
	}
	// Uneven final value should be kept as-is for readability (arg name will be generated later).
	lone := Keyvals("orphan")
	if len(lone) != 1 || lone[0] != "orphan" {
		t.Fatalf("unexpected result for single argument: %#v", lone)
	}
}
