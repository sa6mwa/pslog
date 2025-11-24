package pslog

import (
	"errors"
	"testing"
)

func TestCollectFieldsTracksTrustedKeys(t *testing.T) {
	fields := collectFields([]any{
		"ascii", 1,
		"needs\n", 2,
		TrustedString("trusted"), 3,
		123, 4,
		"lonely",
	})

	if len(fields) != 5 {
		t.Fatalf("expected 5 fields, got %d", len(fields))
	}

	tests := []struct {
		i       int
		key     string
		trusted bool
		value   any
	}{
		{0, "ascii", true, 1},
		{1, "needs\n", false, 2},
		{2, "trusted", true, 3},
		{3, "123", true, 4},
		{4, "arg4", true, "lonely"},
	}

	for _, tt := range tests {
		field := fields[tt.i]
		if field.key != tt.key {
			t.Fatalf("field %d key mismatch: got %q want %q", tt.i, field.key, tt.key)
		}
		if field.trustedKey != tt.trusted {
			t.Fatalf("field %d trusted mismatch: got %v want %v", tt.i, field.trustedKey, tt.trusted)
		}
		if field.value != tt.value {
			t.Fatalf("field %d value mismatch: got %#v want %#v", tt.i, field.value, tt.value)
		}
	}
}

func TestCollectFieldsNilAndEmpty(t *testing.T) {
	if fields := collectFields(nil); fields != nil {
		t.Fatalf("expected nil input to return nil slice")
	}
	if fields := collectFields([]any{}); fields != nil {
		t.Fatalf("expected empty input to return nil slice")
	}
}

func TestCollectFieldsSingleError(t *testing.T) {
	err := errors.New("boom")

	fields := collectFields([]any{err})
	if len(fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(fields))
	}

	field := fields[0]
	if field.key != "error" {
		t.Fatalf("expected key \"error\", got %q", field.key)
	}
	if !field.trustedKey {
		t.Fatalf("expected key to be trusted")
	}
	errValue, ok := field.value.(error)
	if !ok {
		t.Fatalf("expected value to implement error, got %T", field.value)
	}
	if got := errValue.Error(); got != err.Error() {
		t.Fatalf("expected error message %q, got %q", err.Error(), got)
	}
}
