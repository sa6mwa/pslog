package pslog

import (
	"sync"
	"sync/atomic"
	"time"
)

var (
	cacheableLayouts sync.Map
	nonCacheLayouts  sync.Map
)

func init() {
	for _, layout := range []string{
		DTGTimeFormat,
		time.ANSIC,
		time.UnixDate,
		time.RubyDate,
		time.RFC822,
		time.RFC822Z,
		time.RFC850,
		time.RFC1123,
		time.RFC1123Z,
		time.RFC3339,
		time.Kitchen,
		time.Stamp,
		time.DateTime,
		time.DateOnly,
		time.TimeOnly,
	} {
		cacheableLayouts.Store(layout, struct{}{})
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.StampMilli,
		time.StampMicro,
		time.StampNano,
	} {
		nonCacheLayouts.Store(layout, struct{}{})
	}
}

type timeCache struct {
	layout    string
	utc       bool
	value     atomic.Value
	now       func() time.Time
	newTicker func(time.Duration) tickerControl
	formatter func(time.Time) string

	stopCh   chan struct{}
	doneCh   chan struct{}
	stopOnce sync.Once
	stopped  atomic.Bool
}

type tickerControl struct {
	C    <-chan time.Time
	Stop func()
}

func (t tickerControl) stop() {
	if t.Stop != nil {
		t.Stop()
	}
}

func newTimeCache(layout string, utc bool, formatter func(time.Time) string) *timeCache {
	cache := &timeCache{
		layout:    layout,
		utc:       utc,
		now:       time.Now,
		newTicker: defaultTicker,
		formatter: formatter,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
	cache.start()
	return cache
}

func newStandaloneTimeCache(layout string, utc bool, formatter func(time.Time) string, now func() time.Time, newTicker func(time.Duration) tickerControl) *timeCache {
	cache := &timeCache{
		layout:    layout,
		utc:       utc,
		now:       now,
		newTicker: newTicker,
		formatter: formatter,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
	cache.start()
	return cache
}

func defaultTicker(d time.Duration) tickerControl {
	t := time.NewTicker(d)
	return tickerControl{
		C:    t.C,
		Stop: t.Stop,
	}
}

func (c *timeCache) start() {
	if c == nil {
		return
	}
	c.value.Store(c.formatTime(c.nowTime()))
	ticker := c.makeTicker(time.Second)
	if ticker.C == nil {
		close(c.doneCh)
		return
	}
	go c.refresh(ticker)
}

func (c *timeCache) Current() string {
	if c == nil {
		return ""
	}
	return c.value.Load().(string)
}

func (c *timeCache) refresh(ticker tickerControl) {
	defer ticker.stop()
	defer close(c.doneCh)
	for {
		select {
		case <-c.stopCh:
			return
		case now, ok := <-ticker.C:
			if !ok {
				return
			}
			c.value.Store(c.formatTime(now))
		}
	}
}

func (c *timeCache) nowTime() time.Time {
	nowFunc := c.now
	if nowFunc == nil {
		nowFunc = time.Now
	}
	now := nowFunc()
	if c.utc {
		return now.UTC()
	}
	return now
}

func (c *timeCache) formatTime(t time.Time) string {
	if c.utc {
		t = t.UTC()
	}
	if c.formatter != nil {
		return c.formatter(t)
	}
	return t.Format(c.layout)
}

func (c *timeCache) makeTicker(d time.Duration) tickerControl {
	if c.newTicker != nil {
		if ticker := c.newTicker(d); ticker.C != nil {
			return ticker
		}
	}
	return defaultTicker(d)
}

func (c *timeCache) Close() {
	if c == nil {
		return
	}
	c.stopped.Store(true)
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
}

func (c *timeCache) isStopped() bool {
	if c == nil {
		return true
	}
	return c.stopped.Load()
}

func (c *timeCache) waitStopped(timeout time.Duration) bool {
	if c == nil || c.doneCh == nil {
		return true
	}
	if timeout <= 0 {
		<-c.doneCh
		return true
	}
	select {
	case <-c.doneCh:
		return true
	case <-time.After(timeout):
		return false
	}
}

func isCacheableLayout(layout string) bool {
	if _, ok := cacheableLayouts.Load(layout); ok {
		return true
	}
	if _, ok := nonCacheLayouts.Load(layout); ok {
		return false
	}
	if hasSubSecondPrecision(layout) {
		nonCacheLayouts.Store(layout, struct{}{})
		return false
	}
	cacheableLayouts.Store(layout, struct{}{})
	return true
}

func hasSubSecondPrecision(layout string) bool {
	base := time.Date(2024, time.January, 2, 15, 4, 5, 0, time.UTC)
	// If formatting changes within the same second, layout depends on sub-second precision.
	return base.Format(layout) != base.Add(time.Millisecond).Format(layout)
}
