package asmlog

import (
	"fmt"
	"io"
	"sync"
	"time"

	"pkt.systems/pslog"
)

// Logger is an early prototype for a specialised JSON logger that satisfies pslog.Base.
type Logger struct {
	dst       io.Writer
	mu        sync.Mutex
	bufPool   sync.Pool
	timeCache *timeCache
}

var _ pslog.Base = (*Logger)(nil)

// New returns a Logger that emits structured JSON to dst. dst defaults to io.Discard
// when nil. The prototype focuses on the Msg/Level hot path to enable low-level
// experimentation (including assembly backends) without impacting pslog directly.
func New(dst io.Writer) *Logger {
	if dst == nil {
		dst = io.Discard
	}
	tc := newTimeCache(time.RFC3339, true, nil)
	logger := &Logger{
		dst:       dst,
		timeCache: tc,
		bufPool: sync.Pool{
			New: func() any { return newLineBuffer() },
		},
	}
	return logger
}

// Trace implements pslog.Base.
func (l *Logger) Trace(msg string, keyvals ...any) { l.log(pslog.TraceLevel, msg, keyvals...) }

// Debug implements pslog.Base.
func (l *Logger) Debug(msg string, keyvals ...any) { l.log(pslog.DebugLevel, msg, keyvals...) }

// Info implements pslog.Base.
func (l *Logger) Info(msg string, keyvals ...any) { l.log(pslog.InfoLevel, msg, keyvals...) }

// Warn implements pslog.Base.
func (l *Logger) Warn(msg string, keyvals ...any) { l.log(pslog.WarnLevel, msg, keyvals...) }

// Error implements pslog.Base.
func (l *Logger) Error(msg string, keyvals ...any) { l.log(pslog.ErrorLevel, msg, keyvals...) }

func (l *Logger) log(level pslog.Level, msg string, keyvals ...any) {
	buf := l.acquireBuffer()
	defer l.releaseBuffer(buf)

	buf.reset()
	buf.writeByte('{')
	l.appendField(buf, "ts", l.timeCache.Current(), true)
	l.appendField(buf, "lvl", pslog.LevelString(level), false)
	l.appendField(buf, "msg", msg, false)
	l.appendKeyvals(buf, keyvals)
	buf.writeByte('}')
	buf.writeByte('\n')

	l.mu.Lock()
	_, _ = l.dst.Write(buf.bytes())
	l.mu.Unlock()
}

func (l *Logger) appendField(buf *lineBuffer, key, value string, first bool) {
	if !first {
		buf.writeByte(',')
	}
	buf.appendQuoted(key)
	buf.writeByte(':')
	buf.appendQuoted(value)
}

func (l *Logger) appendKeyvals(buf *lineBuffer, keyvals []any) {
	count := len(keyvals)
	for i := 0; i+1 < count; i += 2 {
		key := stringifyKey(keyvals[i])
		buf.writeByte(',')
		buf.appendQuoted(key)
		buf.writeByte(':')
		appendValue(buf, keyvals[i+1])
	}
	if count%2 == 1 {
		argKey := fmt.Sprintf("arg%d", count/2)
		buf.writeByte(',')
		buf.appendQuoted(argKey)
		buf.writeByte(':')
		appendValue(buf, keyvals[count-1])
	}
}

func (l *Logger) acquireBuffer() *lineBuffer {
	return l.bufPool.Get().(*lineBuffer)
}

func (l *Logger) releaseBuffer(buf *lineBuffer) {
	l.bufPool.Put(buf)
}

func stringifyKey(key any) string {
	switch k := key.(type) {
	case string:
		if k == "" {
			return "key"
		}
		return k
	case fmt.Stringer:
		return k.String()
	default:
		return fmt.Sprint(k)
	}
}
