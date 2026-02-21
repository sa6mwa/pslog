package pslog

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTimeCacheCloseStopsRefreshLoop(t *testing.T) {
	layout := time.RFC3339
	start := time.Date(2025, time.October, 12, 12, 0, 0, 0, time.UTC)
	tickCh := make(chan time.Time, 1)
	stopped := make(chan struct{}, 1)
	var closeTickOnce sync.Once

	cache := newStandaloneTimeCache(
		layout,
		false,
		nil,
		func() time.Time { return start },
		func(time.Duration) tickerControl {
			return tickerControl{
				C: tickCh,
				Stop: func() {
					select {
					case stopped <- struct{}{}:
					default:
					}
					closeTickOnce.Do(func() {
						close(tickCh)
					})
				},
			}
		},
	)

	cache.Close()
	select {
	case <-stopped:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected ticker Stop to be called on cache close")
	}

	if !cache.isStopped() {
		t.Fatalf("expected cache to be marked stopped")
	}
}

func TestLoggerCloseStopsOwnedTimeCache(t *testing.T) {
	var buf strings.Builder
	logger := NewWithOptions(nil, &buf, Options{
		Mode:       ModeStructured,
		NoColor:    true,
		TimeFormat: time.RFC3339,
	})

	plain, ok := logger.(*jsonPlainLogger)
	if !ok {
		t.Fatalf("expected json plain logger, got %T", logger)
	}
	cache := plain.base.cfg.timeCache
	if cache == nil {
		t.Fatalf("expected logger to create cacheable timeCache")
	}

	closer, ok := logger.(interface{ Close() error })
	if !ok {
		t.Fatalf("expected close-capable logger")
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("close returned error: %v", err)
	}

	if !cache.isStopped() {
		t.Fatalf("expected owned cache to be stopped after close")
	}
}

func TestLoggerCloneCloseDoesNotStopSharedTimeCache(t *testing.T) {
	var buf strings.Builder
	logger := NewWithOptions(nil, &buf, Options{
		Mode:       ModeStructured,
		NoColor:    true,
		TimeFormat: time.RFC3339,
	})
	plain, ok := logger.(*jsonPlainLogger)
	if !ok {
		t.Fatalf("expected json plain logger, got %T", logger)
	}
	cache := plain.base.cfg.timeCache
	if cache == nil {
		t.Fatalf("expected time cache")
	}

	clone := logger.With("k", "v")
	clonePlain, ok := clone.(*jsonPlainLogger)
	if !ok {
		t.Fatalf("expected json plain clone, got %T", clone)
	}
	if clonePlain.base.cfg.timeCache != cache {
		t.Fatalf("expected clone to share same cache pointer")
	}

	cloneCloser, ok := clone.(interface{ Close() error })
	if !ok {
		t.Fatalf("expected clone to be close-capable")
	}
	if err := cloneCloser.Close(); err != nil {
		t.Fatalf("clone close returned error: %v", err)
	}
	if cache.isStopped() {
		t.Fatalf("expected clone close to keep shared cache running")
	}

	rootCloser, ok := logger.(interface{ Close() error })
	if !ok {
		t.Fatalf("expected root to be close-capable")
	}
	if err := rootCloser.Close(); err != nil {
		t.Fatalf("root close returned error: %v", err)
	}
	if !cache.isStopped() {
		t.Fatalf("expected root close to stop owned cache")
	}
}

func TestTimeCacheCloseIsIdempotent(t *testing.T) {
	cache := newTimeCache(time.RFC3339, false, nil)
	cache.Close()
	cache.Close()
	if !cache.isStopped() {
		t.Fatalf("expected cache to remain stopped")
	}
}

func TestTimeCacheCloseReturnsQuicklyUnderTickerBackpressure(t *testing.T) {
	layout := time.RFC3339
	start := time.Date(2025, time.October, 12, 12, 0, 0, 0, time.UTC)
	sourceTicks := make(chan time.Time, 3)
	relayTicks := make(chan time.Time, 1)
	relayDone := make(chan struct{})
	stopCalled := make(chan struct{}, 1)
	releaseFormatter := make(chan struct{})
	formatterEntered := make(chan struct{}, 1)
	var stopOnce sync.Once
	var closeSourceOnce sync.Once
	var releaseOnce sync.Once
	var formatCalls atomic.Int32

	t.Cleanup(func() {
		stopOnce.Do(func() { close(relayDone) })
		closeSourceOnce.Do(func() { close(sourceTicks) })
		releaseOnce.Do(func() { close(releaseFormatter) })
	})

	go func() {
		defer close(relayTicks)
		for {
			select {
			case <-relayDone:
				return
			case tick, ok := <-sourceTicks:
				if !ok {
					return
				}
				select {
				case relayTicks <- tick:
				case <-relayDone:
					return
				}
			}
		}
	}()

	cache := newStandaloneTimeCache(
		layout,
		false,
		func(ts time.Time) string {
			call := formatCalls.Add(1)
			if call == 2 {
				select {
				case formatterEntered <- struct{}{}:
				default:
				}
				<-releaseFormatter
			}
			return ts.Format(layout)
		},
		func() time.Time { return start },
		func(time.Duration) tickerControl {
			return tickerControl{
				C: relayTicks,
				Stop: func() {
					stopOnce.Do(func() {
						close(relayDone)
						select {
						case stopCalled <- struct{}{}:
						default:
						}
					})
				},
			}
		},
	)

	sourceTicks <- start.Add(time.Second)
	sourceTicks <- start.Add(2 * time.Second)
	sourceTicks <- start.Add(3 * time.Second)

	select {
	case <-formatterEntered:
	case <-time.After(300 * time.Millisecond):
		t.Fatalf("expected formatter to block while refresh loop is active")
	}

	closeStart := time.Now()
	cache.Close()
	closeDuration := time.Since(closeStart)
	if closeDuration > 50*time.Millisecond {
		t.Fatalf("expected cache.Close() to return quickly, took %s", closeDuration)
	}

	if !cache.isStopped() {
		t.Fatalf("expected cache to be marked stopped")
	}

	releaseOnce.Do(func() { close(releaseFormatter) })
	closeSourceOnce.Do(func() { close(sourceTicks) })

	select {
	case <-stopCalled:
	case <-time.After(300 * time.Millisecond):
		t.Fatalf("expected ticker Stop callback once blocked formatter work was released")
	}
}

func TestLoggerConcurrentCloseAcrossClones(t *testing.T) {
	var buf strings.Builder
	root := NewWithOptions(nil, &buf, Options{
		Mode:       ModeStructured,
		NoColor:    true,
		TimeFormat: time.RFC3339,
	})

	plain, ok := root.(*jsonPlainLogger)
	if !ok {
		t.Fatalf("expected json plain logger, got %T", root)
	}
	cache := plain.base.cfg.timeCache
	if cache == nil {
		t.Fatalf("expected time cache")
	}

	closers := make([]interface{ Close() error }, 0, 33)
	rootCloser, ok := root.(interface{ Close() error })
	if !ok {
		t.Fatalf("expected root to be close-capable")
	}
	closers = append(closers, rootCloser)
	for i := 0; i < 32; i++ {
		clone := root.With("clone", i)
		cloneCloser, ok := clone.(interface{ Close() error })
		if !ok {
			t.Fatalf("expected clone to be close-capable")
		}
		closers = append(closers, cloneCloser)
	}

	const repeats = 20
	var wg sync.WaitGroup
	errs := make(chan error, len(closers)*repeats)
	for _, closer := range closers {
		closer := closer
		for i := 0; i < repeats; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := closer.Close(); err != nil {
					errs <- err
				}
			}()
		}
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("concurrent logger close calls did not complete in time")
	}

	close(errs)
	for err := range errs {
		t.Fatalf("expected no close errors, got %v", err)
	}

	if !cache.isStopped() {
		t.Fatalf("expected shared cache to be stopped after root close")
	}
	if _, owned := cacheOwners.Load(cache); owned {
		t.Fatalf("expected cache owner entry to be removed after shutdown")
	}
}

func TestTimeCacheCachesWithinTick(t *testing.T) {
	layout := time.RFC3339
	start := time.Date(2025, time.October, 12, 12, 0, 0, 0, time.FixedZone("CEST", 2*3600))
	tickCh := make(chan time.Time, 1)

	cache := newStandaloneTimeCache(
		layout,
		false,
		nil,
		func() time.Time { return start },
		func(time.Duration) tickerControl { return tickerControl{C: tickCh} },
	)

	first := cache.Current()
	wantFirst := start.Format(layout)
	if first != wantFirst {
		t.Fatalf("initial cache value mismatch: got %q want %q", first, wantFirst)
	}

	next := start.Add(500 * time.Millisecond)
	cache.now = func() time.Time { return next }
	second := cache.Current()
	if second != first {
		t.Fatalf("cache should return cached value before tick: got %q want %q", second, first)
	}

	advance := start.Add(time.Second)
	tickCh <- advance
	close(tickCh)

	wantAdvance := advance.Format(layout)
	deadline := time.After(200 * time.Millisecond)
	for {
		current := cache.Current()
		if current == wantAdvance {
			break
		}
		select {
		case <-time.After(5 * time.Millisecond):
		case <-deadline:
			t.Fatalf("cache did not update after tick; last value %q, want %q", current, wantAdvance)
		}
	}
}

func TestTimeCacheHonoursUTC(t *testing.T) {
	layout := time.RFC3339
	start := time.Date(2025, time.July, 4, 15, 30, 0, 0, time.FixedZone("PDT", -7*3600))
	tickCh := make(chan time.Time, 1)

	cache := newStandaloneTimeCache(
		layout,
		true,
		nil,
		func() time.Time { return start },
		func(time.Duration) tickerControl {
			return tickerControl{C: tickCh}
		},
	)

	initial := cache.Current()
	if initial != start.UTC().Format(layout) {
		t.Fatalf("expected UTC formatting on initial value: got %q want %q", initial, start.UTC().Format(layout))
	}

	next := start.Add(2 * time.Second)
	tickCh <- next
	close(tickCh)

	want := next.UTC().Format(layout)
	deadline := time.After(200 * time.Millisecond)
	for {
		current := cache.Current()
		if current == want {
			break
		}
		select {
		case <-time.After(5 * time.Millisecond):
		case <-deadline:
			t.Fatalf("expected UTC formatting after tick; last %q want %q", current, want)
		}
	}
}

func TestIsCacheableLayoutRejectsSubSecondLayouts(t *testing.T) {
	if isCacheableLayout(time.RFC3339Nano) {
		t.Fatalf("time.RFC3339Nano should not be cacheable")
	}
	custom := "2006-01-02T15:04:05.000000"
	if isCacheableLayout(custom) {
		t.Fatalf("%q should not be cacheable", custom)
	}
	if !isCacheableLayout(time.RFC3339) {
		t.Fatalf("time.RFC3339 should be cacheable")
	}
	if !isCacheableLayout(DTGTimeFormat) {
		t.Fatalf("DTGTimeFormat should be cacheable")
	}
}

func TestTimeCacheDisabledForSubSecondLayouts(t *testing.T) {
	var firstBuf, secondBuf strings.Builder
	logger := NewWithOptions(nil, &firstBuf, Options{Mode: ModeStructured, TimeFormat: time.RFC3339Nano})
	logger.Info("first")
	firstLine := strings.TrimSuffix(firstBuf.String(), "\n")
	time.Sleep(time.Millisecond)
	logger = NewWithOptions(nil, &secondBuf, Options{Mode: ModeStructured, TimeFormat: time.RFC3339Nano})
	logger.Info("second")
	secondLine := strings.TrimSuffix(secondBuf.String(), "\n")
	if firstLine == secondLine {
		t.Fatalf("expected timestamps to differ for RFC3339Nano, got identical lines %q", firstLine)
	}
}
