package pslog_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"pkt.systems/pslog"
)

type recordingBase struct {
	seen []string
}

func (r *recordingBase) record(level, msg string, _ ...any) {
	r.seen = append(r.seen, level+":"+msg)
}

func (r *recordingBase) Trace(msg string, kv ...any) { r.record("trace", msg, kv...) }
func (r *recordingBase) Debug(msg string, kv ...any) { r.record("debug", msg, kv...) }
func (r *recordingBase) Info(msg string, kv ...any)  { r.record("info", msg, kv...) }
func (r *recordingBase) Warn(msg string, kv ...any)  { r.record("warn", msg, kv...) }
func (r *recordingBase) Error(msg string, kv ...any) { r.record("error", msg, kv...) }

func isNoopLogger(tb testing.TB, logger any) bool {
	tb.Helper()
	return strings.HasSuffix(fmt.Sprintf("%T", logger), ".noopLogger")
}

func TestContextWithLoggerRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	logger := pslog.NewWithOptions(&buf, pslog.Options{Mode: pslog.ModeStructured, DisableTimestamp: true})

	ctx := pslog.ContextWithLogger(nil, logger)
	if ctx == nil {
		t.Fatalf("expected non-nil context")
	}
	if got := pslog.LoggerFromContext(ctx); got != logger {
		t.Fatalf("logger round trip failed: got %T want %T", got, logger)
	}
	if got := pslog.Ctx(ctx); got != logger {
		t.Fatalf("Ctx alias did not match logger")
	}
	if got := pslog.BaseLoggerFromContext(ctx); got != logger {
		t.Fatalf("BaseLoggerFromContext did not return stored logger")
	}
	if got := pslog.BCtx(ctx); got != logger {
		t.Fatalf("BCtx alias did not return stored logger")
	}

	pslog.Ctx(ctx).Info("from logger")
	pslog.BCtx(ctx).Info("from base")
	out := buf.String()
	if !strings.Contains(out, "from logger") {
		t.Fatalf("expected output from logger usage, got %q", out)
	}
	if !strings.Contains(out, "from base") {
		t.Fatalf("expected output from base usage, got %q", out)
	}
}

func TestContextWithLoggerNilLoggerPreservesContext(t *testing.T) {
	baseCtx := context.WithValue(context.Background(), "k", "v")

	ctx := pslog.ContextWithLogger(baseCtx, nil)
	if ctx != baseCtx {
		t.Fatalf("expected original context when logger is nil")
	}
	if got := pslog.LoggerFromContext(ctx); !isNoopLogger(t, got) {
		t.Fatalf("expected noop logger when none stored, got %T", got)
	}
}

func TestLoggerFromContextNilContextIsNoop(t *testing.T) {
	logger := pslog.LoggerFromContext(nil)
	if !isNoopLogger(t, logger) {
		t.Fatalf("expected noop logger from nil context, got %T", logger)
	}
	logger.Info("nil-safe") // should not panic
}

func TestContextWithBaseLogger_StoresBaseOnly(t *testing.T) {
	rec := &recordingBase{}
	ctx := pslog.ContextWithBaseLogger(context.Background(), rec)

	if got := pslog.BaseLoggerFromContext(ctx); got != rec {
		t.Fatalf("expected stored base logger back, got %T", got)
	}
	if got := pslog.BCtx(ctx); got != rec {
		t.Fatalf("BCtx alias did not return stored base logger")
	}
	pslog.BCtx(ctx).Info("hello")
	if len(rec.seen) != 1 || rec.seen[0] != "info:hello" {
		t.Fatalf("expected base logger to record message, got %v", rec.seen)
	}

	if got := pslog.LoggerFromContext(ctx); !isNoopLogger(t, got) {
		t.Fatalf("expected LoggerFromContext to fall back to noop when only Base is stored, got %T", got)
	}
}

func TestContextWithBaseLoggerWithPslogLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := pslog.NewWithOptions(&buf, pslog.Options{Mode: pslog.ModeStructured, DisableTimestamp: true})

	ctx := pslog.ContextWithBaseLogger(nil, logger)
	if ctx == nil {
		t.Fatalf("expected non-nil context")
	}
	if got := pslog.BaseLoggerFromContext(ctx); got != logger {
		t.Fatalf("BaseLoggerFromContext did not return stored logger")
	}
	if got := pslog.LoggerFromContext(ctx); got != logger {
		t.Fatalf("LoggerFromContext should also succeed when value implements Logger, got %T", got)
	}

	pslog.LoggerFromContext(ctx).Info("through logger")
	pslog.BaseLoggerFromContext(ctx).Info("through base")
	out := buf.String()
	if !strings.Contains(out, "through logger") || !strings.Contains(out, "through base") {
		t.Fatalf("expected both log entries in output, got %q", out)
	}
}

func TestContextWithBaseLoggerNilLoggerPreservesContext(t *testing.T) {
	baseCtx := context.WithValue(context.Background(), "k", "v")

	ctx := pslog.ContextWithBaseLogger(baseCtx, nil)
	if ctx != baseCtx {
		t.Fatalf("expected original context when base logger is nil")
	}
	if got := pslog.BaseLoggerFromContext(ctx); !isNoopLogger(t, got) {
		t.Fatalf("expected noop base logger when none stored, got %T", got)
	}
}
