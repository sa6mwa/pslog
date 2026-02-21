package pslog

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

type closeTrackingWriter struct {
	closed atomic.Bool
	writes atomic.Int64
}

func (w *closeTrackingWriter) Write(p []byte) (int, error) {
	w.writes.Add(1)
	return len(p), nil
}

func (w *closeTrackingWriter) Close() error {
	w.closed.Store(true)
	return nil
}

func TestLoggerCloseDoesNotCloseUserProvidedWriter(t *testing.T) {
	writer := &closeTrackingWriter{}
	logger := NewWithOptions(nil, writer, Options{
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

	if writer.closed.Load() {
		t.Fatalf("expected user-provided writer to stay open")
	}

	logger.Info("after_close")
	if writer.writes.Load() < 2 {
		t.Fatalf("expected writes to continue after Close, got %d", writer.writes.Load())
	}
}

func TestLoggerFromEnvCloseClosesOwnedOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "owned.log")
	t.Setenv("PSLOG_OBS_OUTPUT", path)

	logger := LoggerFromEnv(nil,
		WithEnvPrefix("PSLOG_OBS_"),
		WithEnvOptions(Options{
			Mode:             ModeStructured,
			NoColor:          true,
			DisableTimestamp: true,
		}),
	)

	logger.Info("before_close")
	closer, ok := logger.(interface{ Close() error })
	if !ok {
		t.Fatalf("expected close-capable logger")
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("close returned error: %v", err)
	}

	logger.Info("after_close")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read owned log: %v", err)
	}
	output := string(data)
	if !strings.Contains(output, "before_close") {
		t.Fatalf("expected pre-close line in file, got %q", output)
	}
	if strings.Contains(output, "after_close") {
		t.Fatalf("expected no post-close line in file, got %q", output)
	}
}
