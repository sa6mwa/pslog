package asmlog

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
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
			New: func() any { return &bytes.Buffer{} },
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

	buf.Reset()
	buf.WriteByte('{')
	l.appendField(buf, "ts", l.timeCache.Current(), true)
	l.appendField(buf, "lvl", pslog.LevelString(level), false)
	l.appendField(buf, "msg", msg, false)
	l.appendKeyvals(buf, keyvals)
	buf.WriteByte('}')
	buf.WriteByte('\n')

	l.mu.Lock()
	_, _ = l.dst.Write(buf.Bytes())
	l.mu.Unlock()
}

func (l *Logger) appendField(buf *bytes.Buffer, key, value string, first bool) {
	if !first {
		buf.WriteByte(',')
	}
	buf.WriteString(strconv.Quote(key))
	buf.WriteByte(':')
	buf.WriteString(strconv.Quote(value))
}

func (l *Logger) appendKeyvals(buf *bytes.Buffer, keyvals []any) {
	count := len(keyvals)
	for i := 0; i+1 < count; i += 2 {
		key := stringifyKey(keyvals[i])
		buf.WriteByte(',')
		buf.WriteString(strconv.Quote(key))
		buf.WriteByte(':')
		appendValue(buf, keyvals[i+1])
	}
	if count%2 == 1 {
		argKey := fmt.Sprintf("arg%d", count/2)
		buf.WriteByte(',')
		buf.WriteString(strconv.Quote(argKey))
		buf.WriteByte(':')
		appendValue(buf, keyvals[count-1])
	}
}

func (l *Logger) acquireBuffer() *bytes.Buffer {
	buf := l.bufPool.Get().(*bytes.Buffer)
	return buf
}

func (l *Logger) releaseBuffer(buf *bytes.Buffer) {
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

func appendValue(buf *bytes.Buffer, value any) {
	switch v := value.(type) {
	case string:
		buf.WriteString(strconv.Quote(v))
	case bool:
		buf.WriteString(strconv.FormatBool(v))
	case int:
		buf.WriteString(strconv.FormatInt(int64(v), 10))
	case int8:
		buf.WriteString(strconv.FormatInt(int64(v), 10))
	case int16:
		buf.WriteString(strconv.FormatInt(int64(v), 10))
	case int32:
		buf.WriteString(strconv.FormatInt(int64(v), 10))
	case int64:
		buf.WriteString(strconv.FormatInt(v, 10))
	case uint:
		buf.WriteString(strconv.FormatUint(uint64(v), 10))
	case uint8:
		buf.WriteString(strconv.FormatUint(uint64(v), 10))
	case uint16:
		buf.WriteString(strconv.FormatUint(uint64(v), 10))
	case uint32:
		buf.WriteString(strconv.FormatUint(uint64(v), 10))
	case uint64:
		buf.WriteString(strconv.FormatUint(v, 10))
	case float32:
		buf.WriteString(strconv.FormatFloat(float64(v), 'f', -1, 32))
	case float64:
		buf.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
	case time.Duration:
		buf.WriteString(strconv.Quote(v.String()))
	case time.Time:
		buf.WriteString(strconv.Quote(v.UTC().Format(time.RFC3339Nano)))
	case error:
		buf.WriteString(strconv.Quote(v.Error()))
	case fmt.Stringer:
		buf.WriteString(strconv.Quote(v.String()))
	case []byte:
		buf.WriteString(strconv.Quote(string(v)))
	default:
		buf.WriteString(strconv.Quote(fmt.Sprint(v)))
	}
}
