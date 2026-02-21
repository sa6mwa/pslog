package pslog

import (
	"context"
	"io"
	"sync/atomic"
	"time"
)

type consolePlainEmitFunc func(*consolePlainLogger, *lineWriter, Level, string, []any)

type consolePlainLogger struct {
	base         loggerBase
	baseBytes    []byte
	hasBaseBytes bool
	lineHint     *atomic.Int64
	emit         consolePlainEmitFunc
}

func newConsolePlainLogger(ctx context.Context, cfg coreConfig, opts Options) *consolePlainLogger {
	configureConsoleScannerFromOptions(opts)
	logger := &consolePlainLogger{
		base:     newLoggerBase(cfg, nil),
		lineHint: new(atomic.Int64),
	}
	owner := ownerToken(logger)
	claimTimeCacheOwnership(cfg.timeCache, owner)
	claimContextCancellation(ctx, cfg.writer, cfg.timeCache, owner)
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
	keyvals = l.base.maybeAddCaller(keyvals)
	lw := acquireLineWriter(l.base.cfg.writer)
	lw.autoFlush = false
	if l.lineHint != nil {
		if hint := l.lineHint.Load(); hint > 0 {
			lw.preallocate(int(hint))
		}
	}
	l.emit(l, lw, level, msg, keyvals)
	lw.finishLine()
	lw.commit()
	l.recordHint(lw.lastLineLength())
	releaseLineWriter(lw)
}

func (l *consolePlainLogger) recordHint(n int) {
	updateLineHint(l.lineHint, n)
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

func (l *consolePlainLogger) Close() error {
	return closeLoggerRuntime(l.base.cfg.writer, l.base.cfg.timeCache, ownerToken(l))
}

func (l *consolePlainLogger) rebuildBaseBytes() {
	l.baseBytes = encodeConsoleFieldsPlain(l.base.fields)
	l.hasBaseBytes = len(l.baseBytes) > 0
	if l.base.cfg.includeLogLevel {
		l.base.cfg.logLevelValue = LevelString(l.base.cfg.currentLevel())
	}
	l.emit = selectConsolePlainEmit(l.base.cfg, l.hasBaseBytes)
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
	start := len(lw.buf)
	if writeRuntimeConsolePlainFast(lw, keyvals) {
		return
	}
	lw.buf = lw.buf[:start]
	writeRuntimeConsolePlainSlow(lw, keyvals)
}

func writeRuntimeConsolePlainFast(lw *lineWriter, keyvals []any) bool {
	if len(keyvals) == 0 {
		return true
	}
	pair := 0
	for i := 0; i+1 < len(keyvals); i += 2 {
		var key string
		switch k := keyvals[i].(type) {
		case TrustedString:
			key = string(k)
		case string:
			key = k
		default:
			return false
		}
		if key == "" {
			pair++
			continue
		}
		lw.writeByte(' ')
		lw.writeString(key)
		lw.writeByte('=')
		value := keyvals[i+1]
		if !writeConsoleValueInline(lw, value) {
			writeConsoleValuePlain(lw, value)
		}
		pair++
	}
	if len(keyvals)%2 != 0 {
		lw.writeByte(' ')
		lw.writeString(argKeyName(pair))
		lw.writeByte('=')
		value := keyvals[len(keyvals)-1]
		if !writeConsoleValueInline(lw, value) {
			writeConsoleValuePlain(lw, value)
		}
	}
	return true
}

func writeRuntimeConsolePlainSlow(lw *lineWriter, keyvals []any) {
	if len(keyvals) == 0 {
		return
	}
	pair := 0
	for i := 0; i+1 < len(keyvals); i += 2 {
		key := keyFromValue(keyvals[i], pair)
		if key == "" {
			pair++
			continue
		}
		lw.writeByte(' ')
		lw.writeString(key)
		lw.writeByte('=')
		value := keyvals[i+1]
		if !writeConsoleValueFast(lw, value) {
			writeConsoleValuePlain(lw, value)
		}
		pair++
	}
	if len(keyvals)%2 != 0 {
		lw.writeByte(' ')
		lw.writeString(argKeyName(pair))
		lw.writeByte('=')
		value := keyvals[len(keyvals)-1]
		if !writeConsoleValueFast(lw, value) {
			writeConsoleValuePlain(lw, value)
		}
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

func selectConsolePlainEmit(cfg coreConfig, hasBaseFields bool) consolePlainEmitFunc {
	switch {
	case cfg.includeTimestamp && cfg.includeLogLevel:
		if hasBaseFields {
			return emitConsolePlainTimestampLogLevelWithBaseFields
		}
		return emitConsolePlainTimestampLogLevelNoBaseFields
	case cfg.includeTimestamp:
		if hasBaseFields {
			return emitConsolePlainTimestampWithBaseFields
		}
		return emitConsolePlainTimestampNoBaseFields
	case cfg.includeLogLevel:
		if hasBaseFields {
			return emitConsolePlainLogLevelWithBaseFields
		}
		return emitConsolePlainLogLevelNoBaseFields
	default:
		if hasBaseFields {
			return emitConsolePlainBaseWithBaseFields
		}
		return emitConsolePlainBaseNoBaseFields
	}
}

func emitConsolePlainTimestampLogLevelWithBaseFields(l *consolePlainLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	timestamp := l.base.cfg.timestamp()
	levelLabel := consoleLevelPlain(level)
	estimate := len(levelLabel) + len(l.baseBytes) + len(keyvals)*16 + 4
	estimate += len(timestamp) + 1
	estimate += len(" loglevel=") + len(l.base.cfg.logLevelValue)
	if msg != "" {
		estimate += len(msg) + 1
	}
	lw.reserve(estimate)
	writeConsoleTimestampPlain(lw, timestamp)
	lw.writeByte(' ')
	lw.writeString(levelLabel)
	if msg != "" {
		lw.writeByte(' ')
		writeConsoleMessagePlain(lw, msg)
	}
	lw.writeBytes(l.baseBytes)
	writeRuntimeConsolePlain(lw, keyvals)
	writeConsoleFieldPlain(lw, "loglevel", l.base.cfg.logLevelValue)
}

func emitConsolePlainTimestampLogLevelNoBaseFields(l *consolePlainLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	timestamp := l.base.cfg.timestamp()
	levelLabel := consoleLevelPlain(level)
	estimate := len(levelLabel) + len(keyvals)*16 + 4
	estimate += len(timestamp) + 1
	estimate += len(" loglevel=") + len(l.base.cfg.logLevelValue)
	if msg != "" {
		estimate += len(msg) + 1
	}
	lw.reserve(estimate)
	writeConsoleTimestampPlain(lw, timestamp)
	lw.writeByte(' ')
	lw.writeString(levelLabel)
	if msg != "" {
		lw.writeByte(' ')
		writeConsoleMessagePlain(lw, msg)
	}
	writeRuntimeConsolePlain(lw, keyvals)
	writeConsoleFieldPlain(lw, "loglevel", l.base.cfg.logLevelValue)
}

func emitConsolePlainTimestampWithBaseFields(l *consolePlainLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	timestamp := l.base.cfg.timestamp()
	levelLabel := consoleLevelPlain(level)
	estimate := len(levelLabel) + len(l.baseBytes) + len(keyvals)*16 + 4
	estimate += len(timestamp) + 1
	if msg != "" {
		estimate += len(msg) + 1
	}
	lw.reserve(estimate)
	writeConsoleTimestampPlain(lw, timestamp)
	lw.writeByte(' ')
	lw.writeString(levelLabel)
	if msg != "" {
		lw.writeByte(' ')
		writeConsoleMessagePlain(lw, msg)
	}
	lw.writeBytes(l.baseBytes)
	writeRuntimeConsolePlain(lw, keyvals)
}

func emitConsolePlainTimestampNoBaseFields(l *consolePlainLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	timestamp := l.base.cfg.timestamp()
	levelLabel := consoleLevelPlain(level)
	estimate := len(levelLabel) + len(keyvals)*16 + 4
	estimate += len(timestamp) + 1
	if msg != "" {
		estimate += len(msg) + 1
	}
	lw.reserve(estimate)
	writeConsoleTimestampPlain(lw, timestamp)
	lw.writeByte(' ')
	lw.writeString(levelLabel)
	if msg != "" {
		lw.writeByte(' ')
		writeConsoleMessagePlain(lw, msg)
	}
	writeRuntimeConsolePlain(lw, keyvals)
}

func emitConsolePlainLogLevelWithBaseFields(l *consolePlainLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	levelLabel := consoleLevelPlain(level)
	estimate := len(levelLabel) + len(l.baseBytes) + len(keyvals)*16 + 4
	estimate += len(" loglevel=") + len(l.base.cfg.logLevelValue)
	if msg != "" {
		estimate += len(msg) + 1
	}
	lw.reserve(estimate)
	lw.writeString(levelLabel)
	if msg != "" {
		lw.writeByte(' ')
		writeConsoleMessagePlain(lw, msg)
	}
	lw.writeBytes(l.baseBytes)
	writeRuntimeConsolePlain(lw, keyvals)
	writeConsoleFieldPlain(lw, "loglevel", l.base.cfg.logLevelValue)
}

func emitConsolePlainLogLevelNoBaseFields(l *consolePlainLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	levelLabel := consoleLevelPlain(level)
	estimate := len(levelLabel) + len(keyvals)*16 + 4
	estimate += len(" loglevel=") + len(l.base.cfg.logLevelValue)
	if msg != "" {
		estimate += len(msg) + 1
	}
	lw.reserve(estimate)
	lw.writeString(levelLabel)
	if msg != "" {
		lw.writeByte(' ')
		writeConsoleMessagePlain(lw, msg)
	}
	writeRuntimeConsolePlain(lw, keyvals)
	writeConsoleFieldPlain(lw, "loglevel", l.base.cfg.logLevelValue)
}

func emitConsolePlainBaseWithBaseFields(l *consolePlainLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	levelLabel := consoleLevelPlain(level)
	estimate := len(levelLabel) + len(l.baseBytes) + len(keyvals)*16 + 4
	if msg != "" {
		estimate += len(msg) + 1
	}
	lw.reserve(estimate)
	lw.writeString(levelLabel)
	if msg != "" {
		lw.writeByte(' ')
		writeConsoleMessagePlain(lw, msg)
	}
	lw.writeBytes(l.baseBytes)
	writeRuntimeConsolePlain(lw, keyvals)
}

func emitConsolePlainBaseNoBaseFields(l *consolePlainLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	levelLabel := consoleLevelPlain(level)
	estimate := len(levelLabel) + len(keyvals)*16 + 4
	if msg != "" {
		estimate += len(msg) + 1
	}
	lw.reserve(estimate)
	lw.writeString(levelLabel)
	if msg != "" {
		lw.writeByte(' ')
		writeConsoleMessagePlain(lw, msg)
	}
	writeRuntimeConsolePlain(lw, keyvals)
}

// writeConsoleMessagePlain escapes control/quote/backslash in the message to
// prevent accidental ANSI injection while keeping it unquoted for readability.
func writeConsoleMessagePlain(lw *lineWriter, msg string) {
	if msg == "" {
		return
	}
	const hex = "0123456789abcdef"

	// Fast path: scan for unsafe bytes; if none, append directly.
	unsafePos := -1
	for i := 0; i < len(msg); i++ {
		c := msg[i]
		if c < 0x20 || c == '\\' || c == '"' || c == 0x7f || c == 0x1b {
			unsafePos = i
			break
		}
	}
	if unsafePos == -1 {
		lw.reserve(len(msg))
		lw.buf = append(lw.buf, msg...)
		lw.maybeFlush()
		return
	}

	lw.reserve(len(msg) + 8) // small headroom for escapes
	lw.buf = append(lw.buf, msg[:unsafePos]...)
	for i := unsafePos; i < len(msg); i++ {
		switch c := msg[i]; c {
		case '\n':
			lw.buf = append(lw.buf, '\\', 'n')
		case '\r':
			lw.buf = append(lw.buf, '\\', 'r')
		case '\t':
			lw.buf = append(lw.buf, '\\', 't')
		case '\b':
			lw.buf = append(lw.buf, '\\', 'b')
		case '\f':
			lw.buf = append(lw.buf, '\\', 'f')
		case '\\':
			lw.buf = append(lw.buf, '\\', '\\')
		case '"':
			lw.buf = append(lw.buf, '\\', '"')
		case 0x1b:
			lw.buf = append(lw.buf, '\\', 'x', '1', 'b')
		case 0x7f:
			lw.buf = append(lw.buf, '\\', 'x', '7', 'f')
		default:
			if c < 0x20 {
				lw.buf = append(lw.buf, '\\', 'x', hex[c>>4], hex[c&0x0f])
			} else {
				lw.buf = append(lw.buf, c)
			}
		}
	}
	lw.maybeFlush()
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

func writeConsoleValueFast(lw *lineWriter, value any) bool {
	switch v := value.(type) {
	case TrustedString:
		writeConsoleStringPlain(lw, string(v))
		return true
	case string:
		writeConsoleStringPlain(lw, v)
		return true
	case time.Time:
		writeConsoleStringPlain(lw, lw.formatTimeRFC3339(v))
		return true
	case time.Duration:
		writeConsoleStringPlain(lw, lw.formatDuration(v))
		return true
	case stringer:
		writeConsoleStringPlain(lw, v.String())
		return true
	case error:
		writeConsoleStringPlain(lw, v.Error())
		return true
	case bool:
		writeConsoleBoolPlain(lw, v)
		return true
	case int:
		writeConsoleIntPlain(lw, int64(v))
		return true
	case int8:
		writeConsoleIntPlain(lw, int64(v))
		return true
	case int16:
		writeConsoleIntPlain(lw, int64(v))
		return true
	case int32:
		writeConsoleIntPlain(lw, int64(v))
		return true
	case int64:
		writeConsoleIntPlain(lw, v)
		return true
	case uint:
		writeConsoleUintPlain(lw, uint64(v))
		return true
	case uint8:
		writeConsoleUintPlain(lw, uint64(v))
		return true
	case uint16:
		writeConsoleUintPlain(lw, uint64(v))
		return true
	case uint32:
		writeConsoleUintPlain(lw, uint64(v))
		return true
	case uint64:
		writeConsoleUintPlain(lw, v)
		return true
	case uintptr:
		writeConsoleUintPlain(lw, uint64(v))
		return true
	case float32:
		writeConsoleFloatPlain(lw, float64(v))
		return true
	case float64:
		writeConsoleFloatPlain(lw, v)
		return true
	case []byte:
		writeConsoleStringPlain(lw, string(v))
		return true
	case nil:
		writeConsoleStringPlain(lw, "nil")
		return true
	}
	return false
}

func writeConsoleValueInline(lw *lineWriter, value any) bool {
	switch v := value.(type) {
	case TrustedString:
		writeConsoleStringPlain(lw, string(v))
		return true
	case string:
		writeConsoleStringPlain(lw, v)
		return true
	case bool:
		writeConsoleBoolPlain(lw, v)
		return true
	case int:
		writeConsoleIntPlain(lw, int64(v))
		return true
	case int8:
		writeConsoleIntPlain(lw, int64(v))
		return true
	case int16:
		writeConsoleIntPlain(lw, int64(v))
		return true
	case int32:
		writeConsoleIntPlain(lw, int64(v))
		return true
	case int64:
		writeConsoleIntPlain(lw, v)
		return true
	case uint:
		writeConsoleUintPlain(lw, uint64(v))
		return true
	case uint8:
		writeConsoleUintPlain(lw, uint64(v))
		return true
	case uint16:
		writeConsoleUintPlain(lw, uint64(v))
		return true
	case uint32:
		writeConsoleUintPlain(lw, uint64(v))
		return true
	case uint64:
		writeConsoleUintPlain(lw, v)
		return true
	case uintptr:
		writeConsoleUintPlain(lw, uint64(v))
		return true
	case float32:
		writeConsoleFloatPlain(lw, float64(v))
		return true
	case float64:
		writeConsoleFloatPlain(lw, v)
		return true
	case time.Time:
		writeConsoleStringPlain(lw, lw.formatTimeRFC3339(v))
		return true
	case time.Duration:
		writeConsoleStringPlain(lw, lw.formatDuration(v))
		return true
	case stringer:
		writeConsoleStringPlain(lw, v.String())
		return true
	case error:
		writeConsoleStringPlain(lw, v.Error())
		return true
	case []byte:
		writeConsoleStringPlain(lw, string(v))
		return true
	case nil:
		writeConsoleStringPlain(lw, "nil")
		return true
	}
	return false
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
		writeConsoleQuotedString(lw, value)
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
	return appendConsoleStringInline(buf, value)
}
