package pslog

import (
	"sync/atomic"
	"testing"
)

func TestConsoleLineHintCap(t *testing.T) {
	var hint atomic.Int64
	updateLineHint(&hint, lineHintMaxPrealloc*4)
	if got := hint.Load(); got != int64(lineHintMaxPrealloc) {
		t.Fatalf("expected capped hint %d, got %d", lineHintMaxPrealloc, got)
	}
}

func TestConsoleLineHintDecay(t *testing.T) {
	var hint atomic.Int64
	updateLineHint(&hint, lineHintMaxPrealloc)
	for i := 0; i < 32; i++ {
		updateLineHint(&hint, 128)
	}
	got := hint.Load()
	if got >= int64(lineHintMaxPrealloc) {
		t.Fatalf("expected decay below cap, got %d", got)
	}
	if got < 128 {
		t.Fatalf("expected hint to stay >= current line, got %d", got)
	}
	if got > 1024 {
		t.Fatalf("expected substantial decay, got %d", got)
	}
}

func TestConsoleLineHintHysteresis(t *testing.T) {
	var hint atomic.Int64
	hint.Store(512)
	updateLineHint(&hint, 480) // delta below threshold: ignore
	if got := hint.Load(); got != 512 {
		t.Fatalf("expected hint to stay stable for small delta, got %d", got)
	}
	updateLineHint(&hint, 200) // substantial drop: decay
	if got := hint.Load(); got >= 512 {
		t.Fatalf("expected decayed hint below 512, got %d", got)
	}
}

func TestConsoleLineHintRecordersUseEstimator(t *testing.T) {
	plain := &consolePlainLogger{lineHint: new(atomic.Int64)}
	color := &consoleColorLogger{lineHint: new(atomic.Int64)}
	plain.recordHint(lineHintMaxPrealloc * 8)
	color.recordHint(lineHintMaxPrealloc * 8)
	if got := plain.lineHint.Load(); got != int64(lineHintMaxPrealloc) {
		t.Fatalf("plain hint should be capped, got %d", got)
	}
	if got := color.lineHint.Load(); got != int64(lineHintMaxPrealloc) {
		t.Fatalf("color hint should be capped, got %d", got)
	}
}
