package pslog

import (
	"io"
	"math"
	"strconv"
	"sync"
	"time"
)

const (
	lineWriterDefaultCap   = 1024
	lineWriterFlushTrigger = 8 << 10 // flush once a line exceeds 8KiB
	lineWriterMaxCap       = 64 << 10
)

type lineWriter struct {
	dst           io.Writer
	buf           []byte
	lastLen       int
	autoFlush     bool
	floatCache    [floatCacheSlots]floatCacheEntry
	durationCache [durationCacheSlots]durationCacheEntry
	timeCache     [timeCacheSlots]timeCacheEntry
	boolCache     [2]literalCacheEntry
	stringCache   [stringCacheSlots]literalCacheEntry
	nullLiteral   literalCacheEntry
}

const (
	floatCacheSlots    = 4
	durationCacheSlots = 4
	timeCacheSlots     = 4
	stringCacheSlots   = 8
)

const literalCacheCapacity = 64

type floatCacheEntry struct {
	bits uint64
	n    int
	buf  [32]byte
}

type durationCacheEntry struct {
	value time.Duration
	str   string
}

type timeCacheEntry struct {
	nano   int64
	offset int
	str    string
}

type literalCacheEntry struct {
	len byte
	buf [literalCacheCapacity]byte
}

var lineWriterPool = sync.Pool{
	New: func() any {
		return &lineWriter{buf: make([]byte, 0, lineWriterDefaultCap)}
	},
}

func acquireLineWriter(dst io.Writer) *lineWriter {
	lw := lineWriterPool.Get().(*lineWriter)
	lw.dst = dst
	lw.buf = lw.buf[:0]
	lw.lastLen = 0
	lw.autoFlush = true
	return lw
}

func releaseLineWriter(lw *lineWriter) {
	lw.dst = nil
	if cap(lw.buf) > lineWriterMaxCap {
		lw.buf = make([]byte, 0, lineWriterDefaultCap)
	} else {
		lw.buf = lw.buf[:0]
	}
	for i := range lw.boolCache {
		lw.boolCache[i] = literalCacheEntry{}
	}
	for i := range lw.stringCache {
		lw.stringCache[i] = literalCacheEntry{}
	}
	lw.nullLiteral = literalCacheEntry{}
	lw.autoFlush = true
	lw.lastLen = 0
	lineWriterPool.Put(lw)
}

func (lw *lineWriter) reserve(n int) {
	if n <= 0 {
		return
	}
	need := len(lw.buf) + n
	if need <= cap(lw.buf) {
		return
	}
	newCap := max(cap(lw.buf)*2+n, need)
	if newCap > lineWriterMaxCap {
		newCap = need
	}
	newBuf := make([]byte, len(lw.buf), newCap)
	copy(newBuf, lw.buf)
	lw.buf = newBuf
}

func (lw *lineWriter) preallocate(n int) {
	if n <= 0 || len(lw.buf) != 0 {
		return
	}
	if n > lineWriterMaxCap {
		n = lineWriterMaxCap
	}
	lw.reserve(n)
}

func (lw *lineWriter) lastLineLength() int {
	return lw.lastLen
}

func (lw *lineWriter) writeByte(b byte) {
	lw.reserve(1)
	lw.buf = append(lw.buf, b)
	lw.maybeFlush()
}

func (lw *lineWriter) writeString(s string) {
	if s == "" {
		return
	}
	lw.reserve(len(s))
	lw.buf = append(lw.buf, s...)
	lw.maybeFlush()
}

func (lw *lineWriter) writeBytes(b []byte) {
	if len(b) == 0 {
		return
	}
	lw.reserve(len(b))
	lw.buf = append(lw.buf, b...)
	lw.maybeFlush()
}

func (lw *lineWriter) writeInt64(n int64) {
	lw.reserve(24)
	lw.buf = strconv.AppendInt(lw.buf, n, 10)
	lw.maybeFlush()
}

func (lw *lineWriter) writeUint64(n uint64) {
	lw.reserve(24)
	lw.buf = strconv.AppendUint(lw.buf, n, 10)
	lw.maybeFlush()
}

func (lw *lineWriter) writeFloat64(f float64) {
	if buf, ok := lw.lookupFloat(f); ok {
		lw.reserve(len(buf))
		lw.buf = append(lw.buf, buf...)
		lw.maybeFlush()
		return
	}
	start := len(lw.buf)
	lw.reserve(32)
	lw.buf = strconv.AppendFloat(lw.buf, f, 'f', -1, 64)
	lw.storeFloat(f, lw.buf[start:])
	lw.maybeFlush()
}

func (lw *lineWriter) writeQuotedString(s string) {
	lw.reserve(len(s) + 2)
	lw.buf = strconv.AppendQuote(lw.buf, s)
	lw.maybeFlush()
}

func (lw *lineWriter) lookupFloat(f float64) ([]byte, bool) {
	bits := math.Float64bits(f)
	for i := range lw.floatCache {
		entry := &lw.floatCache[i]
		if entry.n > 0 && entry.bits == bits {
			return entry.buf[:entry.n], true
		}
	}
	return nil, false
}

func (lw *lineWriter) storeFloat(f float64, data []byte) {
	if len(data) > len(lw.floatCache[0].buf) {
		return
	}
	bits := math.Float64bits(f)
	idx := bits & (floatCacheSlots - 1)
	entry := &lw.floatCache[idx]
	entry.bits = bits
	entry.n = copy(entry.buf[:], data)
}

func (lw *lineWriter) formatDuration(d time.Duration) string {
	for i := range lw.durationCache {
		entry := &lw.durationCache[i]
		if entry.str != "" && entry.value == d {
			return entry.str
		}
	}
	str := d.String()
	idx := int(d) & (durationCacheSlots - 1)
	lw.durationCache[idx] = durationCacheEntry{value: d, str: str}
	return str
}

func (lw *lineWriter) formatTimeRFC3339(t time.Time) string {
	nano := t.UnixNano()
	_, offset := t.Zone()
	for i := range lw.timeCache {
		entry := &lw.timeCache[i]
		if entry.str != "" && entry.nano == nano && entry.offset == offset {
			return entry.str
		}
	}
	str := t.Format(time.RFC3339Nano)
	idx := int(nano) & (timeCacheSlots - 1)
	lw.timeCache[idx] = timeCacheEntry{nano: nano, offset: offset, str: str}
	return str
}

func (lw *lineWriter) writeBool(v bool) {
	lw.writeBoolLiteral(v)
}

func (lw *lineWriter) writeBoolLiteral(v bool) {
	idx := 0
	if v {
		idx = 1
	}
	entry := &lw.boolCache[idx]
	if entry.len != 0 {
		lw.reserve(int(entry.len))
		lw.buf = append(lw.buf, entry.buf[:entry.len]...)
		lw.maybeFlush()
		return
	}
	var literal string
	if v {
		literal = "true"
	} else {
		literal = "false"
	}
	lw.reserve(len(literal))
	start := len(lw.buf)
	lw.buf = append(lw.buf, literal...)
	entry.len = byte(len(literal))
	copy(entry.buf[:entry.len], lw.buf[start:])
	lw.maybeFlush()
}

func (lw *lineWriter) writeNullLiteral() {
	entry := &lw.nullLiteral
	if entry.len != 0 {
		lw.reserve(int(entry.len))
		lw.buf = append(lw.buf, entry.buf[:entry.len]...)
		lw.maybeFlush()
		return
	}
	lw.reserve(4)
	start := len(lw.buf)
	lw.buf = append(lw.buf, 'n', 'u', 'l', 'l')
	entry.len = 4
	copy(entry.buf[:entry.len], lw.buf[start:])
	lw.maybeFlush()
}

func (lw *lineWriter) finishLine() {
	lw.writeByte('\n')
}

func (lw *lineWriter) commit() {
	lw.flush()
}

func (lw *lineWriter) flush() {
	if len(lw.buf) == 0 || lw.dst == nil {
		lw.lastLen = 0
		lw.buf = lw.buf[:0]
		return
	}
	lw.lastLen = len(lw.buf)
	_, _ = lw.dst.Write(lw.buf)
	lw.buf = lw.buf[:0]
}

func (lw *lineWriter) maybeFlush() {
	if !lw.autoFlush {
		return
	}
	if cap(lw.buf) <= lineWriterFlushTrigger {
		return
	}
	if len(lw.buf) >= lineWriterFlushTrigger {
		lw.flush()
	}
}
