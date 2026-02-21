package pslog

import (
	"fmt"
	"io"
	"testing"
	"time"
)

func newLegacyLikeTimeCache(layout string, utc bool, formatter func(time.Time) string) *timeCache {
	cache := &timeCache{
		layout:    layout,
		utc:       utc,
		now:       time.Now,
		newTicker: defaultTicker,
		formatter: formatter,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
	cache.value.Store(cache.formatTime(cache.nowTime()))
	ticker := cache.makeTicker(time.Second)
	if ticker.C != nil {
		go func() {
			defer close(cache.doneCh)
			for now := range ticker.C {
				cache.value.Store(cache.formatTime(now))
			}
		}()
	} else {
		close(cache.doneCh)
	}
	return cache
}

func injectBenchmarkTimeCache(logger Logger, cache *timeCache) Logger {
	switch l := logger.(type) {
	case *jsonPlainLogger:
		l.base.cfg.includeTimestamp = true
		l.base.cfg.timeLayout = time.RFC3339
		l.base.cfg.timeCache = cache
		l.base.cfg.timeFormatter = nil
		claimTimeCacheOwnership(cache, ownerToken(l))
		return l
	case *jsonColorLogger:
		l.base.cfg.includeTimestamp = true
		l.base.cfg.timeLayout = time.RFC3339
		l.base.cfg.timeCache = cache
		l.base.cfg.timeFormatter = nil
		claimTimeCacheOwnership(cache, ownerToken(l))
		return l
	case *consolePlainLogger:
		l.base.cfg.includeTimestamp = true
		l.base.cfg.timeLayout = time.RFC3339
		l.base.cfg.timeCache = cache
		l.base.cfg.timeFormatter = nil
		claimTimeCacheOwnership(cache, ownerToken(l))
		return l
	case *consoleColorLogger:
		l.base.cfg.includeTimestamp = true
		l.base.cfg.timeLayout = time.RFC3339
		l.base.cfg.timeCache = cache
		l.base.cfg.timeFormatter = nil
		claimTimeCacheOwnership(cache, ownerToken(l))
		return l
	default:
		return logger
	}
}

func BenchmarkLoggerTimeCacheStrategyAB(b *testing.B) {
	type strategy struct {
		name string
		make func() *timeCache
	}
	type variant struct {
		name string
		opts Options
	}

	strategies := []strategy{
		{
			name: "current",
			make: func() *timeCache { return newTimeCache(time.RFC3339, false, nil) },
		},
		{
			name: "legacy",
			make: func() *timeCache { return newLegacyLikeTimeCache(time.RFC3339, false, nil) },
		},
	}
	variants := []variant{
		{
			name: "json",
			opts: Options{
				Mode:             ModeStructured,
				DisableTimestamp: true,
				MinLevel:         TraceLevel,
				NoColor:          true,
			},
		},
		{
			name: "consolecolor",
			opts: Options{
				Mode:             ModeConsole,
				DisableTimestamp: true,
				MinLevel:         TraceLevel,
				ForceColor:       true,
			},
		},
	}

	for _, v := range variants {
		v := v
		b.Run(v.name, func(b *testing.B) {
			for round := 1; round <= 6; round++ {
				ordered := strategies
				if round%2 == 0 {
					ordered = []strategy{strategies[1], strategies[0]}
				}
				for _, s := range ordered {
					s := s
					b.Run(fmt.Sprintf("round%d/%s", round, s.name), func(b *testing.B) {
						cache := s.make()
						logger := injectBenchmarkTimeCache(NewWithOptions(nil, io.Discard, v.opts), cache)
						// Ensure each sub-benchmark tears down runtime resources and does not leak ticker goroutines.
						if closer, ok := logger.(interface{ Close() error }); ok {
							b.Cleanup(func() { _ = closer.Close() })
						} else {
							b.Cleanup(cache.Close)
						}
						// Warm the timestamp cache before measurement to focus on steady-state costs.
						logger.Info("warmup", "k", "v")

						b.ReportAllocs()
						b.ResetTimer()
						for i := 0; i < b.N; i++ {
							logger.Info(
								"request completed",
								"req_id", i,
								"status", 200,
								"duration_ms", 12,
								"path", "/v1/search",
								"user", "user-1234",
							)
						}
					})
				}
			}
		})
	}
}
