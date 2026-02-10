package pslog

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type legacyTimeCache struct {
	layout    string
	utc       bool
	once      sync.Once
	value     atomic.Value
	now       func() time.Time
	newTicker func(time.Duration) tickerControl
	formatter func(time.Time) string
}

func newLegacyTimeCache(layout string, utc bool, formatter func(time.Time) string) *legacyTimeCache {
	return &legacyTimeCache{
		layout:    layout,
		utc:       utc,
		now:       time.Now,
		newTicker: defaultTicker,
		formatter: formatter,
	}
}

func (c *legacyTimeCache) Current() string {
	c.once.Do(func() {
		now := c.nowTime()
		c.value.Store(c.formatTime(now))
		go c.refresh()
	})
	if v := c.value.Load(); v != nil {
		return v.(string)
	}
	return c.formatTime(c.nowTime())
}

func (c *legacyTimeCache) refresh() {
	ticker := c.makeTicker(time.Second)
	if ticker.C == nil {
		return
	}
	defer ticker.stop()
	for now := range ticker.C {
		c.value.Store(c.formatTime(now))
	}
}

func (c *legacyTimeCache) nowTime() time.Time {
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

func (c *legacyTimeCache) formatTime(t time.Time) string {
	if c.utc {
		t = t.UTC()
	}
	if c.formatter != nil {
		return c.formatter(t)
	}
	return t.Format(c.layout)
}

func (c *legacyTimeCache) makeTicker(d time.Duration) tickerControl {
	if c.newTicker != nil {
		if ticker := c.newTicker(d); ticker.C != nil {
			return ticker
		}
	}
	return defaultTicker(d)
}

func BenchmarkTimeCacheCurrentAB(b *testing.B) {
	b.Run("legacy", func(b *testing.B) {
		cache := newLegacyTimeCache(time.RFC3339, false, nil)
		_ = cache.Current()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = cache.Current()
		}
	})

	b.Run("current", func(b *testing.B) {
		cache := newTimeCache(time.RFC3339, false, nil)
		_ = cache.Current()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = cache.Current()
		}
	})
}
