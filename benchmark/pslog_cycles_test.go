package benchmark

import (
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	pslog "pkt.systems/pslog"
)

type cycleSink struct {
	mu  sync.Mutex
	sum int64
}

func (c *cycleSink) Write(p []byte) (int, error) {
	c.mu.Lock()
	c.sum += int64(len(p))
	c.mu.Unlock()
	return len(p), nil
}

func (c *cycleSink) Sync() error { return nil }

func (c *cycleSink) reset() {
	c.mu.Lock()
	c.sum = 0
	c.mu.Unlock()
}

func (c *cycleSink) bytesWritten() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sum
}

func TestPSLogJSONCycles(t *testing.T) {
	if os.Getenv("PSLOG_CYCLE_TEST") == "" {
		t.Skip("set PSLOG_CYCLE_TEST=1 to run cycle measurement")
	}
	runtime.GOMAXPROCS(1)

	sink := &cycleSink{}
	logger := pslog.NewWithOptions(sink, pslog.Options{Mode: pslog.ModeStructured, MinLevel: pslog.TraceLevel})

	level := pslog.InfoLevel
	message := "lease.acquire"
	keyvals := []any{
		"app", "lockd",
		"sys", "api.http.router",
		"cid", "019a22e0-a900-7bf3-a5e5-e00f4411786d",
		"req_id", "019a22e0-a900-7b72-a852-7908d418dd5d",
		"key", "disk-lifecycle-019a22e0-a8ff-75fc-bdb8-15db481d5956",
		"owner", "disk-worker",
		"ttl_seconds", 45,
		"block_seconds", 0,
		"attempt", 1,
		"network", "tcp",
		"address", "127.0.0.1:37289",
		"enabled", true,
	}

	// Warm-up the logger and buffers.
	for i := 0; i < 2048; i++ {
		logger.Log(level, message, keyvals...)
	}
	sink.reset()

	const iterations = 20000

	start := time.Now()
	for i := 0; i < iterations; i++ {
		logger.Log(level, message, keyvals...)
	}
	elapsed := time.Since(start)

	nsPerOp := float64(elapsed.Nanoseconds()) / float64(iterations)

	bytesPerOp := float64(sink.bytesWritten()) / float64(iterations)

	t.Logf("pslog.Log json path: loops=%d ns/op=%.2f bytes/op=%.1f",
		iterations, nsPerOp, bytesPerOp)
}
