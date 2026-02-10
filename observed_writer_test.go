package pslog

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

type testWriterFunc func([]byte) (int, error)

func (fn testWriterFunc) Write(p []byte) (int, error) {
	return fn(p)
}

func TestObservedWriterPassThrough(t *testing.T) {
	var out bytes.Buffer
	callbackCalled := false

	w := NewObservedWriter(&out, func(WriteFailure) {
		callbackCalled = true
	})

	n, err := w.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if n != len("hello") {
		t.Fatalf("write count mismatch: got %d want %d", n, len("hello"))
	}
	if got := out.String(); got != "hello" {
		t.Fatalf("unexpected output: got %q", got)
	}
	if callbackCalled {
		t.Fatalf("callback should not be called on successful writes")
	}

	stats := w.Stats()
	if stats.Failures != 0 || stats.ShortWrites != 0 {
		t.Fatalf("unexpected stats on success: %+v", stats)
	}
}

func TestObservedWriterReportsError(t *testing.T) {
	boom := errors.New("boom")
	var got WriteFailure
	calls := 0

	w := NewObservedWriter(testWriterFunc(func(p []byte) (int, error) {
		return len(p), boom
	}), func(f WriteFailure) {
		calls++
		got = f
	})

	n, err := w.Write([]byte("abc"))
	if n != 3 {
		t.Fatalf("write count mismatch: got %d want %d", n, 3)
	}
	if !errors.Is(err, boom) {
		t.Fatalf("expected boom error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("callback call count mismatch: got %d want 1", calls)
	}
	if !errors.Is(got.Err, boom) {
		t.Fatalf("callback error mismatch: got %v", got.Err)
	}
	if got.Written != 3 || got.Attempted != 3 {
		t.Fatalf("callback byte counts mismatch: %+v", got)
	}

	stats := w.Stats()
	if stats.Failures != 1 || stats.ShortWrites != 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestObservedWriterNormalizesShortWrite(t *testing.T) {
	var got WriteFailure
	calls := 0

	w := NewObservedWriter(testWriterFunc(func(p []byte) (int, error) {
		return len(p) - 1, nil
	}), func(f WriteFailure) {
		calls++
		got = f
	})

	n, err := w.Write([]byte("abcd"))
	if n != 3 {
		t.Fatalf("write count mismatch: got %d want %d", n, 3)
	}
	if !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("expected io.ErrShortWrite, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("callback call count mismatch: got %d want 1", calls)
	}
	if !errors.Is(got.Err, io.ErrShortWrite) {
		t.Fatalf("callback error mismatch: got %v", got.Err)
	}
	if got.Written != 3 || got.Attempted != 4 {
		t.Fatalf("callback byte counts mismatch: %+v", got)
	}

	stats := w.Stats()
	if stats.Failures != 1 || stats.ShortWrites != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestObservedWriterCloseOwnershipSemantics(t *testing.T) {
	userWriter := &closeTrackingWriter{}
	logger := NewWithOptions(NewObservedWriter(userWriter, nil), Options{
		Mode:             ModeStructured,
		NoColor:          true,
		DisableTimestamp: true,
	})

	logger.Info("before_close")
	closer, ok := logger.(interface{ Close() error })
	if !ok {
		t.Fatalf("expected close-capable logger")
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("close returned error: %v", err)
	}
	if userWriter.closed.Load() {
		t.Fatalf("expected user writer to remain open")
	}

	ownedWriter := &closeTrackingWriter{}
	owned := newOwnedOutput(ownedWriter, ownedWriter)
	ownedLogger := NewWithOptions(NewObservedWriter(owned, nil), Options{
		Mode:             ModeStructured,
		NoColor:          true,
		DisableTimestamp: true,
	})
	ownedLogger.Info("before_owned_close")
	ownedCloser, ok := ownedLogger.(interface{ Close() error })
	if !ok {
		t.Fatalf("expected close-capable logger")
	}
	if err := ownedCloser.Close(); err != nil {
		t.Fatalf("owned close returned error: %v", err)
	}
	if !ownedWriter.closed.Load() {
		t.Fatalf("expected owned writer to be closed")
	}
}
