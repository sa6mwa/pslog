package pslog

import (
	"io"
	"sync/atomic"
	"time"
)

type consolePlainLogger struct {
	base      loggerBase
	baseBytes []byte
	lineHint  *atomic.Int64
}

func newConsolePlainLogger(cfg coreConfig) *consolePlainLogger {
	logger := &consolePlainLogger{
		base:     newLoggerBase(cfg, nil),
		lineHint: new(atomic.Int64),
	}
	logger.rebuildBaseBytes()
	return logger
}

func (l *consolePlainLogger) Trace(msg string, keyvals ...any) { l.log(TraceLevel, msg, keyvals...) }
func (l *consolePlainLogger) Debug(msg string, keyvals ...any) { l.log(DebugLevel, msg, keyvals...) }
func (l *consolePlainLogger) Info(msg string, keyvals ...any)  { l.log(InfoLevel, msg, keyvals...) }
func (l *consolePlainLogger) Warn(msg string, keyvals ...any)  { l.log(WarnLevel, msg, keyvals...) }
func (l *consolePlainLogger) Error(msg string, keyvals ...any) { l.log(ErrorLevel, msg, keyvals...) }

func (l *consolePlainLogger) Fatal(msg string, keyvals ...any) {
	l.log(FatalLevel, msg, keyvals...)
	exitProcess()
}

func (l *consolePlainLogger) Panic(msg string, keyvals ...any) {
	l.log(PanicLevel, msg, keyvals...)
	panic(msg)
}

func (l *consolePlainLogger) Log(level Level, msg string, keyvals ...any) {
	l.log(level, msg, keyvals...)
}

func (l *consolePlainLogger) log(level Level, msg string, keyvals ...any) {
	if !l.base.cfg.shouldLog(level) {
		return
	}
	lw := acquireLineWriter(l.base.cfg.writer)
	lw.autoFlush = false
	if l.lineHint != nil {
		if hint := l.lineHint.Load(); hint > 0 {
			lw.preallocate(int(hint))
		}
	}
	timestamp := ""
	if l.base.cfg.includeTimestamp {
		timestamp = l.base.cfg.timestamp()
	}
	levelLabel := consoleLevelPlain(level)
	estimate := len(levelLabel) + len(l.baseBytes) + len(keyvals)*16 + 4
	if timestamp != "" {
		estimate += len(timestamp) + 1
	}
	if msg != "" {
		estimate += len(msg) + 1
	}
	if l.base.cfg.includeLogLevel {
		estimate += len(" loglevel=") + len(l.base.cfg.logLevelValue)
	}
	lw.reserve(estimate)
	if l.base.cfg.includeTimestamp {
		writeConsoleTimestampPlain(lw, timestamp)
		lw.writeByte(' ')
	}
	lw.writeString(levelLabel)
	if msg != "" {
		lw.writeByte(' ')
		lw.writeString(msg)
	}
	if len(l.baseBytes) > 0 {
		lw.writeBytes(l.baseBytes)
	}
	writeRuntimeConsolePlain(lw, keyvals)
	if l.base.cfg.includeLogLevel {
		writeConsoleFieldPlain(lw, "loglevel", l.base.cfg.logLevelValue)
	}
	lw.finishLine()
	lw.commit()
	l.recordHint(lw.lastLineLength())
	releaseLineWriter(lw)
}

func (l *consolePlainLogger) recordHint(n int) {
	if n <= 0 || l.lineHint == nil {
		return
	}
	current := l.lineHint.Load()
	if n > int(current) {
		l.lineHint.Store(int64(n))
	}
}

func (l *consolePlainLogger) With(keyvals ...any) Logger {
	fields := collectFields(keyvals)
	if len(fields) == 0 {
		return l
	}
	clone := *l
	if l.lineHint != nil {
		hint := l.lineHint.Load()
		clone.lineHint = new(atomic.Int64)
		clone.lineHint.Store(hint)
	}
	clone.base = l.base.clone()
	clone.base.withFields(fields)
	clone.rebuildBaseBytes()
	return &clone
}

func (l *consolePlainLogger) WithLogLevel() Logger {
	if l.base.cfg.includeLogLevel {
		return l
	}
	clone := *l
	if l.lineHint != nil {
		hint := l.lineHint.Load()
		clone.lineHint = new(atomic.Int64)
		clone.lineHint.Store(hint)
	}
	clone.base = l.base.clone()
	clone.base.withLogLevelField()
	clone.rebuildBaseBytes()
	return &clone
}

func (l *consolePlainLogger) LogLevel(level Level) Logger {
	clone := *l
	if l.lineHint != nil {
		hint := l.lineHint.Load()
		clone.lineHint = new(atomic.Int64)
		clone.lineHint.Store(hint)
	}
	clone.base = l.base.clone()
	if level == NoLevel {
		clone.base.withForcedLevel(level)
	} else {
		clone.base.withMinLevel(level)
	}
	clone.rebuildBaseBytes()
	return &clone
}

func (l *consolePlainLogger) LogLevelFromEnv(key string) Logger {
	if level, ok := LevelFromEnv(key); ok {
		return l.LogLevel(level)
	}
	return l
}

func (l *consolePlainLogger) rebuildBaseBytes() {
	l.baseBytes = encodeConsoleFieldsPlain(l.base.fields)
	if l.base.cfg.includeLogLevel {
		l.base.cfg.logLevelValue = LevelString(l.base.cfg.currentLevel())
	}
}

func encodeConsoleFieldsPlain(fields []field) []byte {
	if len(fields) == 0 {
		return nil
	}
	buf := make([]byte, 0, len(fields)*16)
	for _, f := range fields {
		if f.key == "" {
			continue
		}
		buf = append(buf, ' ')
		buf = append(buf, f.key...)
		buf = append(buf, '=')
		buf = appendConsoleValuePlain(buf, f.value)
	}
	return buf
}

func writeRuntimeConsolePlain(lw *lineWriter, keyvals []any) {
	if len(keyvals) == 0 {
		return
	}
	pair := 0
	for i := 0; i < len(keyvals); {
		var key string
		var value any
		if i+1 < len(keyvals) {
			key = keyFromValue(keyvals[i], pair)
			value = keyvals[i+1]
			i += 2
		} else {
			key = argKeyName(pair)
			value = keyvals[i]
			i++
		}
		pair++
		writeConsoleFieldPlain(lw, key, value)
	}
}

func writeConsoleFieldPlain(lw *lineWriter, key string, value any) {
	if key == "" {
		return
	}
	lw.writeByte(' ')
	lw.writeString(key)
	lw.writeByte('=')
	writeConsoleValuePlain(lw, value)
}

func writeConsoleTimestampPlain(lw *lineWriter, ts string) {
	lw.writeString(ts)
}

func consoleLevelPlain(level Level) string {
	switch level {
	case TraceLevel:
		return "TRC"
	case DebugLevel:
		return "DBG"
	case InfoLevel:
		return "INF"
	case WarnLevel:
		return "WRN"
	case ErrorLevel:
		return "ERR"
	case FatalLevel:
		return "FTL"
	case PanicLevel:
		return "PNC"
	case NoLevel:
		return "---"
	default:
		return "INF"
	}
}

func writeConsoleValuePlain(lw *lineWriter, value any) {
	switch v := value.(type) {
	case string:
		writeConsoleStringPlain(lw, v)
	case time.Time:
		writeConsoleStringPlain(lw, lw.formatTimeRFC3339(v))
	case time.Duration:
		writeConsoleStringPlain(lw, lw.formatDuration(v))
	case stringer:
		writeConsoleStringPlain(lw, v.String())
	case error:
		writeConsoleStringPlain(lw, v.Error())
	case bool:
		writeConsoleBoolPlain(lw, v)
	case int:
		writeConsoleIntPlain(lw, int64(v))
	case int8:
		writeConsoleIntPlain(lw, int64(v))
	case int16:
		writeConsoleIntPlain(lw, int64(v))
	case int32:
		writeConsoleIntPlain(lw, int64(v))
	case int64:
		writeConsoleIntPlain(lw, v)
	case uint:
		writeConsoleUintPlain(lw, uint64(v))
	case uint8:
		writeConsoleUintPlain(lw, uint64(v))
	case uint16:
		writeConsoleUintPlain(lw, uint64(v))
	case uint32:
		writeConsoleUintPlain(lw, uint64(v))
	case uint64:
		writeConsoleUintPlain(lw, v)
	case uintptr:
		writeConsoleUintPlain(lw, uint64(v))
	case float32:
		writeConsoleFloatPlain(lw, float64(v))
	case float64:
		writeConsoleFloatPlain(lw, v)
	case []byte:
		writeConsoleStringPlain(lw, string(v))
	case nil:
		writeConsoleStringPlain(lw, "nil")
	default:
		writePTLogValue(lw, v)
	}
}

func writeConsoleStringPlain(lw *lineWriter, value string) {
	if needsQuote(value) {
		lw.writeQuotedString(value)
		return
	}
	lw.writeString(value)
}

func writeConsoleBoolPlain(lw *lineWriter, value bool) {
	if value {
		lw.writeString("true")
		return
	}
	lw.writeString("false")
}

func writeConsoleIntPlain(lw *lineWriter, value int64) {
	lw.writeInt64(value)
}

func writeConsoleUintPlain(lw *lineWriter, value uint64) {
	lw.writeUint64(value)
}

func writeConsoleFloatPlain(lw *lineWriter, value float64) {
	lw.writeFloat64(value)
}

func appendConsoleValuePlain(buf []byte, value any) []byte {
	switch v := value.(type) {
	case string:
		return appendConsoleStringPlain(buf, v)
	case time.Time:
		return appendConsoleStringPlain(buf, v.Format(time.RFC3339))
	case time.Duration:
		return appendConsoleStringPlain(buf, v.String())
	case stringer:
		return appendConsoleStringPlain(buf, v.String())
	case error:
		return appendConsoleStringPlain(buf, v.Error())
	case bool:
		if v {
			return append(buf, "true"...)
		}
		return append(buf, "false"...)
	case int:
		return strconvAppendInt(buf, int64(v))
	case int8:
		return strconvAppendInt(buf, int64(v))
	case int16:
		return strconvAppendInt(buf, int64(v))
	case int32:
		return strconvAppendInt(buf, int64(v))
	case int64:
		return strconvAppendInt(buf, v)
	case uint:
		return strconvAppendUint(buf, uint64(v))
	case uint8:
		return strconvAppendUint(buf, uint64(v))
	case uint16:
		return strconvAppendUint(buf, uint64(v))
	case uint32:
		return strconvAppendUint(buf, uint64(v))
	case uint64:
		return strconvAppendUint(buf, v)
	case uintptr:
		return strconvAppendUint(buf, uint64(v))
	case float32:
		return strconvAppendFloat(buf, float64(v))
	case float64:
		return strconvAppendFloat(buf, v)
	case []byte:
		return appendConsoleStringPlain(buf, string(v))
	case nil:
		return appendConsoleStringPlain(buf, "nil")
	default:
		lw := acquireLineWriter(io.Discard)
		lw.autoFlush = false
		writePTLogValue(lw, v)
		buf = append(buf, lw.buf...)
		releaseLineWriter(lw)
		return buf
	}
}

func appendConsoleStringPlain(buf []byte, value string) []byte {
	if needsQuote(value) {
		return strconvAppendQuoted(buf, value)
	}
	return append(buf, value...)
}
