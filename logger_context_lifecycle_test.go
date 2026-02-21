package pslog

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func cacheFromLogger(tb testing.TB, logger Logger) *timeCache {
	tb.Helper()
	switch l := logger.(type) {
	case *consolePlainLogger:
		return l.base.cfg.timeCache
	case *consoleColorLogger:
		return l.base.cfg.timeCache
	case *jsonPlainLogger:
		return l.base.cfg.timeCache
	case *jsonColorLogger:
		return l.base.cfg.timeCache
	default:
		tb.Fatalf("unsupported logger type %T", logger)
		return nil
	}
}

func ownerFromLogger(tb testing.TB, logger Logger) uintptr {
	tb.Helper()
	switch l := logger.(type) {
	case *consolePlainLogger:
		return ownerToken(l)
	case *consoleColorLogger:
		return ownerToken(l)
	case *jsonPlainLogger:
		return ownerToken(l)
	case *jsonColorLogger:
		return ownerToken(l)
	default:
		tb.Fatalf("unsupported logger type %T", logger)
		return 0
	}
}

func TestLoggerContextCancelStopsTimeCacheAllVariants(t *testing.T) {
	variants := []struct {
		name string
		opts Options
	}{
		{name: "console_plain", opts: Options{Mode: ModeConsole, NoColor: true, TimeFormat: time.RFC3339}},
		{name: "console_color", opts: Options{Mode: ModeConsole, ForceColor: true, TimeFormat: time.RFC3339}},
		{name: "json_plain", opts: Options{Mode: ModeStructured, NoColor: true, TimeFormat: time.RFC3339}},
		{name: "json_color", opts: Options{Mode: ModeStructured, ForceColor: true, TimeFormat: time.RFC3339}},
	}

	for _, tc := range variants {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			logger := NewWithOptions(ctx, io.Discard, tc.opts)
			cache := cacheFromLogger(t, logger)
			if cache == nil {
				t.Fatalf("expected time cache")
			}
			owner := ownerFromLogger(t, logger)
			if owner == 0 {
				t.Fatalf("expected non-zero owner")
			}

			cancel()

			if !cache.waitStopped(2 * time.Second) {
				t.Fatalf("expected cache goroutine to terminate on context cancel")
			}
			if !cache.isStopped() {
				t.Fatalf("expected cache marked stopped")
			}
			if _, owned := cacheOwners.Load(cache); owned {
				t.Fatalf("expected cache owner entry to be removed")
			}
			if _, pending := contextCancelOwners.Load(owner); pending {
				t.Fatalf("expected context cancellation hook to be removed")
			}
		})
	}
}

func TestLoggerContextCancelStressManyLoggersStopsAllCaches(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	variants := []Options{
		{Mode: ModeConsole, NoColor: true, TimeFormat: time.RFC3339},
		{Mode: ModeConsole, ForceColor: true, TimeFormat: time.RFC3339},
		{Mode: ModeStructured, NoColor: true, TimeFormat: time.RFC3339},
		{Mode: ModeStructured, ForceColor: true, TimeFormat: time.RFC3339},
	}

	const perVariant = 64
	caches := make([]*timeCache, 0, len(variants)*perVariant)
	owners := make([]uintptr, 0, len(variants)*perVariant)
	for _, opts := range variants {
		for i := 0; i < perVariant; i++ {
			logger := NewWithOptions(ctx, io.Discard, opts)
			cache := cacheFromLogger(t, logger)
			if cache == nil {
				t.Fatalf("expected time cache")
			}
			caches = append(caches, cache)
			owners = append(owners, ownerFromLogger(t, logger))
		}
	}

	cancel()

	for i, cache := range caches {
		if !cache.waitStopped(3 * time.Second) {
			t.Fatalf("cache[%d] did not terminate", i)
		}
		if _, owned := cacheOwners.Load(cache); owned {
			t.Fatalf("cache[%d] still has owner entry", i)
		}
	}
	for i, owner := range owners {
		if _, pending := contextCancelOwners.Load(owner); pending {
			t.Fatalf("owner[%d] still has context cancellation hook", i)
		}
	}
}

func TestLoggerContextCancelWithAlreadyCanceledContextStopsImmediately(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	logger := NewWithOptions(ctx, io.Discard, Options{
		Mode:       ModeStructured,
		NoColor:    true,
		TimeFormat: time.RFC3339,
	})
	cache := cacheFromLogger(t, logger)
	if cache == nil {
		t.Fatalf("expected time cache")
	}
	if !cache.waitStopped(2 * time.Second) {
		t.Fatalf("expected cache to terminate for already-canceled context")
	}
}

func TestLoggerContextCancelDoesNotCloseUserWriter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	writer := &closeTrackingWriter{}
	logger := NewWithOptions(ctx, writer, Options{
		Mode:       ModeStructured,
		NoColor:    true,
		TimeFormat: time.RFC3339,
	})
	cache := cacheFromLogger(t, logger)
	if cache == nil {
		t.Fatalf("expected time cache")
	}

	cancel()

	if !cache.waitStopped(2 * time.Second) {
		t.Fatalf("expected cache to terminate")
	}
	if writer.closed.Load() {
		t.Fatalf("expected user writer to remain open")
	}
	logger.Info("still-writes")
	if writer.writes.Load() == 0 {
		t.Fatalf("expected writes to continue after context cancel")
	}
}

func TestLoggerContextCancelWithoutOwnedResourcesSkipsHook(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := NewWithOptions(ctx, io.Discard, Options{
		Mode:             ModeStructured,
		NoColor:          true,
		DisableTimestamp: true,
	})
	owner := ownerFromLogger(t, logger)
	if owner == 0 {
		t.Fatalf("expected non-zero owner")
	}
	if _, pending := contextCancelOwners.Load(owner); pending {
		t.Fatalf("expected no context hook when logger has no owned resources")
	}
}

func TestLoggerFromEnvContextCancelClosesOwnedOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "context-owned.log")
	t.Setenv("PSLOG_CTX_OUTPUT", path)

	ctx, cancel := context.WithCancel(context.Background())
	logger := LoggerFromEnv(ctx,
		WithEnvPrefix("PSLOG_CTX_"),
		WithEnvOptions(Options{
			Mode:       ModeStructured,
			NoColor:    true,
			TimeFormat: time.RFC3339,
		}),
	)

	logger.Info("before_cancel")
	cache := cacheFromLogger(t, logger)
	if cache == nil {
		t.Fatalf("expected time cache")
	}

	cancel()
	if !cache.waitStopped(2 * time.Second) {
		t.Fatalf("expected cache to terminate")
	}

	logger.Info("after_cancel")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read owned log: %v", err)
	}
	output := string(data)
	if !strings.Contains(output, "before_cancel") {
		t.Fatalf("expected pre-cancel line in file, got %q", output)
	}
	if strings.Contains(output, "after_cancel") {
		t.Fatalf("expected post-cancel line to be dropped, got %q", output)
	}
}

func TestLoggerSharedContextCancelStopsRootCacheWithConcurrentClones(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	root := NewWithOptions(ctx, io.Discard, Options{
		Mode:       ModeStructured,
		NoColor:    true,
		TimeFormat: time.RFC3339,
	}).With("root", true)

	cache := cacheFromLogger(t, root)
	if cache == nil {
		t.Fatalf("expected time cache")
	}

	const cloneCount = 128
	clones := make([]Logger, 0, cloneCount)
	for i := 0; i < cloneCount; i++ {
		clones = append(clones, root.With("clone", i))
	}

	var wg sync.WaitGroup
	for _, clone := range clones {
		clone := clone
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 8; i++ {
				clone.Info("inflight", "i", i)
			}
		}()
	}

	cancel()
	wg.Wait()

	if !cache.waitStopped(2 * time.Second) {
		t.Fatalf("expected cache to terminate")
	}
	if _, owned := cacheOwners.Load(cache); owned {
		t.Fatalf("expected cache owner entry to be removed")
	}
}
