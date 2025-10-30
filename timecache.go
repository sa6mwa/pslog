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
	once      sync.Once
	value     atomic.Value
	now       func() time.Time
	newTicker func(time.Duration) tickerControl
	formatter func(time.Time) string
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
	return &timeCache{
		layout:    layout,
		utc:       utc,
		now:       time.Now,
		newTicker: defaultTicker,
		formatter: formatter,
	}
}

func defaultTicker(d time.Duration) tickerControl {
	t := time.NewTicker(d)
	return tickerControl{
		C:    t.C,
		Stop: t.Stop,
	}
}

func (c *timeCache) Current() string {
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

func (c *timeCache) refresh() {
	ticker := c.makeTicker(time.Second)
	if ticker.C == nil {
		return
	}
	defer ticker.stop()
	for now := range ticker.C {
		c.value.Store(c.formatTime(now))
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
