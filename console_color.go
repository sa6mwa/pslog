package pslog

import (
	"io"
	"math"
	"strconv"
	"sync/atomic"
	"time"

	"pkt.systems/pslog/ansi"
)

type consoleColorEmitFunc func(*consoleColorLogger, *lineWriter, Level, string, []any)

type consoleColorLogger struct {
	base         loggerBase
	baseBytes    []byte
	hasBaseBytes bool
	lineHint     *atomic.Int64
	emit         consoleColorEmitFunc
}

func newConsoleColorLogger(cfg coreConfig, opts Options) *consoleColorLogger {
	configureConsoleScannerFromOptions(opts)
	logger := &consoleColorLogger{
		base:     newLoggerBase(cfg, nil),
		lineHint: new(atomic.Int64),
	}
	logger.rebuildBaseBytes()
	return logger
}

func appendConsoleKeyColor(buf []byte, key string) []byte {
	buf = append(buf, ' ')
	buf = append(buf, ansi.Key...)
	buf = append(buf, key...)
	buf = append(buf, '=')
	buf = append(buf, ansi.Reset...)
	return buf
}

func writeConsoleKeyColor(lw *lineWriter, key string) {
	if key == "" {
		return
	}
	total := 1 + len(ansi.Key) + len(key) + 1 + len(ansi.Reset)
	lw.reserve(total)
	lw.buf = append(lw.buf, ' ')
	lw.buf = append(lw.buf, ansi.Key...)
	lw.buf = append(lw.buf, key...)
	lw.buf = append(lw.buf, '=')
	lw.buf = append(lw.buf, ansi.Reset...)
}

func writeConsoleColoredLiteral(lw *lineWriter, color string, literal string) {
	lw.reserve(len(color) + len(literal) + len(ansi.Reset))
	lw.buf = append(lw.buf, color...)
	lw.buf = append(lw.buf, literal...)
	lw.buf = append(lw.buf, ansi.Reset...)
	lw.maybeFlush()
}

func (l *consoleColorLogger) Trace(msg string, keyvals ...any) { l.log(TraceLevel, msg, keyvals...) }
func (l *consoleColorLogger) Debug(msg string, keyvals ...any) { l.log(DebugLevel, msg, keyvals...) }
func (l *consoleColorLogger) Info(msg string, keyvals ...any)  { l.log(InfoLevel, msg, keyvals...) }
func (l *consoleColorLogger) Warn(msg string, keyvals ...any)  { l.log(WarnLevel, msg, keyvals...) }
func (l *consoleColorLogger) Error(msg string, keyvals ...any) { l.log(ErrorLevel, msg, keyvals...) }

func (l *consoleColorLogger) Fatal(msg string, keyvals ...any) {
	l.log(FatalLevel, msg, keyvals...)
	exitProcess()
}

func (l *consoleColorLogger) Panic(msg string, keyvals ...any) {
	l.log(PanicLevel, msg, keyvals...)
	panic(msg)
}

func (l *consoleColorLogger) Log(level Level, msg string, keyvals ...any) {
	l.log(level, msg, keyvals...)
}

func (l *consoleColorLogger) log(level Level, msg string, keyvals ...any) {
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
	l.emit(l, lw, level, msg, keyvals)
	lw.finishLine()
	lw.commit()
	l.recordHint(lw.lastLineLength())
	releaseLineWriter(lw)
}

func (l *consoleColorLogger) recordHint(n int) {
	if n <= 0 || l.lineHint == nil {
		return
	}
	current := l.lineHint.Load()
	if n > int(current) {
		l.lineHint.Store(int64(n))
	}
}

func (l *consoleColorLogger) With(keyvals ...any) Logger {
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

func (l *consoleColorLogger) WithLogLevel() Logger {
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

func (l *consoleColorLogger) LogLevel(level Level) Logger {
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

func (l *consoleColorLogger) LogLevelFromEnv(key string) Logger {
	if level, ok := LevelFromEnv(key); ok {
		return l.LogLevel(level)
	}
	return l
}

func (l *consoleColorLogger) rebuildBaseBytes() {
	l.baseBytes = encodeConsoleFieldsColor(l.base.fields)
	l.hasBaseBytes = len(l.baseBytes) > 0
	if l.base.cfg.includeLogLevel {
		l.base.cfg.logLevelValue = LevelString(l.base.cfg.currentLevel())
	}
	l.emit = selectConsoleColorEmit(l.base.cfg, l.hasBaseBytes)
}

func writeRuntimeConsoleColor(lw *lineWriter, keyvals []any) {
	if writeRuntimeConsoleColorFast(lw, keyvals) {
		return
	}
	writeRuntimeConsoleColorSlow(lw, keyvals)
}

func writeRuntimeConsoleColorFast(lw *lineWriter, keyvals []any) bool {
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
		writeConsoleKeyColor(lw, key)
		value := keyvals[i+1]
		if !writeConsoleValueColorInline(lw, value) {
			writeConsoleValueColor(lw, value)
		}
		pair++
	}
	if len(keyvals)%2 != 0 {
		writeConsoleKeyColor(lw, argKeyName(pair))
		value := keyvals[len(keyvals)-1]
		if !writeConsoleValueColorInline(lw, value) {
			writeConsoleValueColor(lw, value)
		}
	}
	return true
}

func writeRuntimeConsoleColorSlow(lw *lineWriter, keyvals []any) {
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
		writeConsoleKeyColor(lw, key)
		value := keyvals[i+1]
		if !writeConsoleValueColorFast(lw, value) {
			writeConsoleValueColor(lw, value)
		}
		pair++
	}
	if len(keyvals)%2 != 0 {
		writeConsoleKeyColor(lw, argKeyName(pair))
		value := keyvals[len(keyvals)-1]
		if !writeConsoleValueColorFast(lw, value) {
			writeConsoleValueColor(lw, value)
		}
	}
}

func encodeConsoleFieldsColor(fields []field) []byte {
	if len(fields) == 0 {
		return nil
	}
	buf := make([]byte, 0, len(fields)*24)
	for _, f := range fields {
		if f.key == "" {
			continue
		}
		buf = appendConsoleKeyColor(buf, f.key)
		buf = appendConsoleValueColor(buf, f.value)
	}
	return buf
}

func writeConsoleFieldColor(lw *lineWriter, key string, value any) {
	if key == "" {
		return
	}
	writeConsoleKeyColor(lw, key)
	writeConsoleValueColor(lw, value)
}

func writeConsoleTimestampColor(lw *lineWriter, ts string) {
	writeConsoleColoredLiteral(lw, ansi.Timestamp, ts)
}

func selectConsoleColorEmit(cfg coreConfig, hasBaseFields bool) consoleColorEmitFunc {
	switch {
	case cfg.includeTimestamp && cfg.includeLogLevel:
		if hasBaseFields {
			return emitConsoleColorTimestampLogLevelWithBaseFields
		}
		return emitConsoleColorTimestampLogLevelNoBaseFields
	case cfg.includeTimestamp:
		if hasBaseFields {
			return emitConsoleColorTimestampWithBaseFields
		}
		return emitConsoleColorTimestampNoBaseFields
	case cfg.includeLogLevel:
		if hasBaseFields {
			return emitConsoleColorLogLevelWithBaseFields
		}
		return emitConsoleColorLogLevelNoBaseFields
	default:
		if hasBaseFields {
			return emitConsoleColorBaseWithBaseFields
		}
		return emitConsoleColorBaseNoBaseFields
	}
}

func emitConsoleColorTimestampLogLevelWithBaseFields(l *consoleColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	timestamp := l.base.cfg.timestamp()
	levelColor, levelLabel := consoleLevelColor(level)
	estimate := len(l.baseBytes) + len(keyvals)*20 + 4
	estimate += len(levelLabel) + len(levelColor) + len(ansi.Reset)
	estimate += len(ansi.Timestamp) + len(timestamp) + len(ansi.Reset) + 1
	estimate += len(ansi.Key) + len("loglevel=") + len(ansi.Reset) + len(ansi.String) + len(l.base.cfg.logLevelValue) + len(ansi.Reset)
	if msg != "" {
		estimate += len(ansi.Message) + len(msg) + len(ansi.Reset) + 1
	}
	lw.reserve(estimate)
	writeConsoleTimestampColor(lw, timestamp)
	lw.writeByte(' ')
	writeConsoleColoredLiteral(lw, levelColor, levelLabel)
	if msg != "" {
		lw.writeByte(' ')
		writeConsoleMessageColor(lw, msg)
	}
	lw.writeBytes(l.baseBytes)
	writeRuntimeConsoleColor(lw, keyvals)
	writeConsoleFieldColor(lw, "loglevel", l.base.cfg.logLevelValue)
}

func emitConsoleColorTimestampLogLevelNoBaseFields(l *consoleColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	timestamp := l.base.cfg.timestamp()
	levelColor, levelLabel := consoleLevelColor(level)
	estimate := len(keyvals)*20 + 4
	estimate += len(levelLabel) + len(levelColor) + len(ansi.Reset)
	estimate += len(ansi.Timestamp) + len(timestamp) + len(ansi.Reset) + 1
	estimate += len(ansi.Key) + len("loglevel=") + len(ansi.Reset) + len(ansi.String) + len(l.base.cfg.logLevelValue) + len(ansi.Reset)
	if msg != "" {
		estimate += len(ansi.Message) + len(msg) + len(ansi.Reset) + 1
	}
	lw.reserve(estimate)
	writeConsoleTimestampColor(lw, timestamp)
	lw.writeByte(' ')
	writeConsoleColoredLiteral(lw, levelColor, levelLabel)
	if msg != "" {
		lw.writeByte(' ')
		writeConsoleMessageColor(lw, msg)
	}
	writeRuntimeConsoleColor(lw, keyvals)
	writeConsoleFieldColor(lw, "loglevel", l.base.cfg.logLevelValue)
}

func emitConsoleColorTimestampWithBaseFields(l *consoleColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	timestamp := l.base.cfg.timestamp()
	levelColor, levelLabel := consoleLevelColor(level)
	estimate := len(l.baseBytes) + len(keyvals)*20 + 4
	estimate += len(levelLabel) + len(levelColor) + len(ansi.Reset)
	estimate += len(ansi.Timestamp) + len(timestamp) + len(ansi.Reset) + 1
	if msg != "" {
		estimate += len(ansi.Message) + len(msg) + len(ansi.Reset) + 1
	}
	lw.reserve(estimate)
	writeConsoleTimestampColor(lw, timestamp)
	lw.writeByte(' ')
	writeConsoleColoredLiteral(lw, levelColor, levelLabel)
	if msg != "" {
		lw.writeByte(' ')
		writeConsoleMessageColor(lw, msg)
	}
	lw.writeBytes(l.baseBytes)
	writeRuntimeConsoleColor(lw, keyvals)
}

func emitConsoleColorTimestampNoBaseFields(l *consoleColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	timestamp := l.base.cfg.timestamp()
	levelColor, levelLabel := consoleLevelColor(level)
	estimate := len(keyvals)*20 + 4
	estimate += len(levelLabel) + len(levelColor) + len(ansi.Reset)
	estimate += len(ansi.Timestamp) + len(timestamp) + len(ansi.Reset) + 1
	if msg != "" {
		estimate += len(ansi.Message) + len(msg) + len(ansi.Reset) + 1
	}
	lw.reserve(estimate)
	writeConsoleTimestampColor(lw, timestamp)
	lw.writeByte(' ')
	writeConsoleColoredLiteral(lw, levelColor, levelLabel)
	if msg != "" {
		lw.writeByte(' ')
		writeConsoleMessageColor(lw, msg)
	}
	writeRuntimeConsoleColor(lw, keyvals)
}

func emitConsoleColorLogLevelWithBaseFields(l *consoleColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	levelColor, levelLabel := consoleLevelColor(level)
	estimate := len(l.baseBytes) + len(keyvals)*20 + 4
	estimate += len(levelLabel) + len(levelColor) + len(ansi.Reset)
	estimate += len(ansi.Key) + len("loglevel=") + len(ansi.Reset) + len(ansi.String) + len(l.base.cfg.logLevelValue) + len(ansi.Reset)
	if msg != "" {
		estimate += len(ansi.Message) + len(msg) + len(ansi.Reset) + 1
	}
	lw.reserve(estimate)
	writeConsoleColoredLiteral(lw, levelColor, levelLabel)
	if msg != "" {
		lw.writeByte(' ')
		writeConsoleMessageColor(lw, msg)
	}
	lw.writeBytes(l.baseBytes)
	writeRuntimeConsoleColor(lw, keyvals)
	writeConsoleFieldColor(lw, "loglevel", l.base.cfg.logLevelValue)
}

func emitConsoleColorLogLevelNoBaseFields(l *consoleColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	levelColor, levelLabel := consoleLevelColor(level)
	estimate := len(keyvals)*20 + 4
	estimate += len(levelLabel) + len(levelColor) + len(ansi.Reset)
	estimate += len(ansi.Key) + len("loglevel=") + len(ansi.Reset) + len(ansi.String) + len(l.base.cfg.logLevelValue) + len(ansi.Reset)
	if msg != "" {
		estimate += len(ansi.Message) + len(msg) + len(ansi.Reset) + 1
	}
	lw.reserve(estimate)
	writeConsoleColoredLiteral(lw, levelColor, levelLabel)
	if msg != "" {
		lw.writeByte(' ')
		writeConsoleMessageColor(lw, msg)
	}
	writeRuntimeConsoleColor(lw, keyvals)
	writeConsoleFieldColor(lw, "loglevel", l.base.cfg.logLevelValue)
}

func emitConsoleColorBaseWithBaseFields(l *consoleColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	levelColor, levelLabel := consoleLevelColor(level)
	estimate := len(l.baseBytes) + len(keyvals)*20 + 4
	estimate += len(levelLabel) + len(levelColor) + len(ansi.Reset)
	if msg != "" {
		estimate += len(ansi.Message) + len(msg) + len(ansi.Reset) + 1
	}
	lw.reserve(estimate)
	writeConsoleColoredLiteral(lw, levelColor, levelLabel)
	if msg != "" {
		lw.writeByte(' ')
		writeConsoleMessageColor(lw, msg)
	}
	lw.writeBytes(l.baseBytes)
	writeRuntimeConsoleColor(lw, keyvals)
}

func emitConsoleColorBaseNoBaseFields(l *consoleColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	levelColor, levelLabel := consoleLevelColor(level)
	estimate := len(keyvals)*20 + 4
	estimate += len(levelLabel) + len(levelColor) + len(ansi.Reset)
	if msg != "" {
		estimate += len(ansi.Message) + len(msg) + len(ansi.Reset) + 1
	}
	lw.reserve(estimate)
	writeConsoleColoredLiteral(lw, levelColor, levelLabel)
	if msg != "" {
		lw.writeByte(' ')
		writeConsoleMessageColor(lw, msg)
	}
	writeRuntimeConsoleColor(lw, keyvals)
}
func consoleLevelColor(level Level) (string, string) {
	switch level {
	case TraceLevel:
		return ansi.Trace, "TRC"
	case DebugLevel:
		return ansi.Debug, "DBG"
	case InfoLevel:
		return ansi.Info, "INF"
	case WarnLevel:
		return ansi.Warn, "WRN"
	case ErrorLevel:
		return ansi.Error, "ERR"
	case FatalLevel:
		return ansi.Fatal, "FTL"
	case PanicLevel:
		return ansi.Panic, "PNC"
	case NoLevel:
		return ansi.NoLevel, "---"
	default:
		return ansi.Info, "INF"
	}
}

func writeConsoleMessageColor(lw *lineWriter, msg string) {
	writeConsoleColoredLiteral(lw, ansi.Message, msg)
}

func writeConsoleValueColorFast(lw *lineWriter, value any) bool {
	switch v := value.(type) {
	case TrustedString:
		writeConsoleStringColor(lw, string(v), ansi.String)
		return true
	case string:
		writeConsoleStringColor(lw, v, ansi.String)
		return true
	case time.Time:
		writeConsoleStringColor(lw, lw.formatTimeRFC3339(v), ansi.Timestamp)
		return true
	case time.Duration:
		writeConsoleStringColor(lw, lw.formatDuration(v), ansi.String)
		return true
	case stringer:
		writeConsoleStringColor(lw, v.String(), ansi.String)
		return true
	case error:
		writeConsoleStringColor(lw, v.Error(), ansi.Error)
		return true
	case bool:
		writeConsoleBoolColor(lw, v)
		return true
	case int:
		writeConsoleIntColor(lw, int64(v))
		return true
	case int8:
		writeConsoleIntColor(lw, int64(v))
		return true
	case int16:
		writeConsoleIntColor(lw, int64(v))
		return true
	case int32:
		writeConsoleIntColor(lw, int64(v))
		return true
	case int64:
		writeConsoleIntColor(lw, v)
		return true
	case uint:
		writeConsoleUintColor(lw, uint64(v))
		return true
	case uint8:
		writeConsoleUintColor(lw, uint64(v))
		return true
	case uint16:
		writeConsoleUintColor(lw, uint64(v))
		return true
	case uint32:
		writeConsoleUintColor(lw, uint64(v))
		return true
	case uint64:
		writeConsoleUintColor(lw, v)
		return true
	case uintptr:
		writeConsoleUintColor(lw, uint64(v))
		return true
	case float32:
		writeConsoleFloatColor(lw, float64(v))
		return true
	case float64:
		writeConsoleFloatColor(lw, v)
		return true
	case []byte:
		writeConsoleStringColor(lw, string(v), ansi.String)
		return true
	case nil:
		writeConsoleStringColor(lw, "nil", ansi.Nil)
		return true
	}
	return false
}

func writeConsoleValueColorInline(lw *lineWriter, value any) bool {
	switch v := value.(type) {
	case TrustedString:
		writeConsoleStringColor(lw, string(v), ansi.String)
		return true
	case string:
		writeConsoleStringColor(lw, v, ansi.String)
		return true
	case bool:
		writeConsoleBoolColor(lw, v)
		return true
	case int:
		writeConsoleIntColor(lw, int64(v))
		return true
	case int8:
		writeConsoleIntColor(lw, int64(v))
		return true
	case int16:
		writeConsoleIntColor(lw, int64(v))
		return true
	case int32:
		writeConsoleIntColor(lw, int64(v))
		return true
	case int64:
		writeConsoleIntColor(lw, v)
		return true
	case uint:
		writeConsoleUintColor(lw, uint64(v))
		return true
	case uint8:
		writeConsoleUintColor(lw, uint64(v))
		return true
	case uint16:
		writeConsoleUintColor(lw, uint64(v))
		return true
	case uint32:
		writeConsoleUintColor(lw, uint64(v))
		return true
	case uint64:
		writeConsoleUintColor(lw, v)
		return true
	case uintptr:
		writeConsoleUintColor(lw, uint64(v))
		return true
	case float32:
		writeConsoleFloatColor(lw, float64(v))
		return true
	case float64:
		writeConsoleFloatColor(lw, v)
		return true
	case time.Time:
		writeConsoleStringColor(lw, lw.formatTimeRFC3339(v), ansi.Timestamp)
		return true
	case time.Duration:
		writeConsoleStringColor(lw, lw.formatDuration(v), ansi.String)
		return true
	case stringer:
		writeConsoleStringColor(lw, v.String(), ansi.String)
		return true
	case error:
		writeConsoleStringColor(lw, v.Error(), ansi.Error)
		return true
	case []byte:
		writeConsoleStringColor(lw, string(v), ansi.String)
		return true
	case nil:
		writeConsoleStringColor(lw, "nil", ansi.Nil)
		return true
	}
	return false
}

func writeConsoleValueColor(lw *lineWriter, value any) {

	switch v := value.(type) {
	case string:
		writeConsoleStringColor(lw, v, ansi.String)
	case time.Time:
		writeConsoleStringColor(lw, lw.formatTimeRFC3339(v), ansi.Timestamp)
	case time.Duration:
		writeConsoleStringColor(lw, lw.formatDuration(v), ansi.String)
	case stringer:
		writeConsoleStringColor(lw, v.String(), ansi.String)
	case error:
		writeConsoleStringColor(lw, v.Error(), ansi.Error)
	case bool:
		writeConsoleBoolColor(lw, v)
	case int:
		writeConsoleIntColor(lw, int64(v))
	case int8:
		writeConsoleIntColor(lw, int64(v))
	case int16:
		writeConsoleIntColor(lw, int64(v))
	case int32:
		writeConsoleIntColor(lw, int64(v))
	case int64:
		writeConsoleIntColor(lw, v)
	case uint:
		writeConsoleUintColor(lw, uint64(v))
	case uint8:
		writeConsoleUintColor(lw, uint64(v))
	case uint16:
		writeConsoleUintColor(lw, uint64(v))
	case uint32:
		writeConsoleUintColor(lw, uint64(v))
	case uint64:
		writeConsoleUintColor(lw, v)
	case uintptr:
		writeConsoleUintColor(lw, uint64(v))
	case float32:
		writeConsoleFloatColor(lw, float64(v))
	case float64:
		writeConsoleFloatColor(lw, v)
	case []byte:
		writeConsoleStringColor(lw, string(v), ansi.String)
	case nil:
		writeConsoleStringColor(lw, "nil", ansi.Nil)
	default:
		writePTLogValueColored(lw, v, ansi.String)
	}
}

func writeConsoleStringColor(lw *lineWriter, value string, color string) {
	if color == "" {
		writeConsoleStringPlain(lw, value)
		return
	}
	lw.reserve(len(color) + len(ansi.Reset) + len(value)*4 + 2)
	lw.buf = append(lw.buf, color...)
	if needsQuote(value) {
		lw.buf = appendConsoleQuotedString(lw.buf, value)
	} else {
		lw.buf = append(lw.buf, value...)
	}
	lw.buf = append(lw.buf, ansi.Reset...)
	lw.maybeFlush()
}

func writeConsoleBoolColor(lw *lineWriter, value bool) {
	literal := "false"
	if value {
		literal = "true"
	}
	writeConsoleColoredLiteral(lw, ansi.Bool, literal)
}

func writeConsoleIntColor(lw *lineWriter, value int64) {
	lw.reserve(len(ansi.Num) + 24 + len(ansi.Reset))
	lw.buf = append(lw.buf, ansi.Num...)
	lw.buf = strconv.AppendInt(lw.buf, value, 10)
	lw.buf = append(lw.buf, ansi.Reset...)
	lw.maybeFlush()
}

func writeConsoleUintColor(lw *lineWriter, value uint64) {
	lw.reserve(len(ansi.Num) + 24 + len(ansi.Reset))
	lw.buf = append(lw.buf, ansi.Num...)
	lw.buf = strconv.AppendUint(lw.buf, value, 10)
	lw.buf = append(lw.buf, ansi.Reset...)
	lw.maybeFlush()
}

func writeConsoleFloatColor(lw *lineWriter, value float64) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		writeConsoleColoredLiteral(lw, ansi.Num, strconv.FormatFloat(value, 'f', -1, 64))
		return
	}
	lw.reserve(len(ansi.Num) + 32 + len(ansi.Reset))
	lw.buf = append(lw.buf, ansi.Num...)
	lw.buf = strconv.AppendFloat(lw.buf, value, 'f', -1, 64)
	lw.buf = append(lw.buf, ansi.Reset...)
	lw.maybeFlush()
}

func appendConsoleValueColor(buf []byte, value any) []byte {
	switch v := value.(type) {
	case string:
		return appendConsoleStringColor(buf, v, ansi.String)
	case time.Time:
		return appendConsoleStringColor(buf, v.Format(time.RFC3339), ansi.Timestamp)
	case time.Duration:
		return appendConsoleStringColor(buf, v.String(), ansi.String)
	case stringer:
		return appendConsoleStringColor(buf, v.String(), ansi.String)
	case error:
		return appendConsoleStringColor(buf, v.Error(), ansi.Error)
	case bool:
		if v {
			return appendColoredLiteral(buf, "true", ansi.Bool)
		}
		return appendColoredLiteral(buf, "false", ansi.Bool)
	case int:
		return appendColoredInt(buf, int64(v))
	case int8:
		return appendColoredInt(buf, int64(v))
	case int16:
		return appendColoredInt(buf, int64(v))
	case int32:
		return appendColoredInt(buf, int64(v))
	case int64:
		return appendColoredInt(buf, v)
	case uint:
		return appendColoredUint(buf, uint64(v))
	case uint8:
		return appendColoredUint(buf, uint64(v))
	case uint16:
		return appendColoredUint(buf, uint64(v))
	case uint32:
		return appendColoredUint(buf, uint64(v))
	case uint64:
		return appendColoredUint(buf, v)
	case uintptr:
		return appendColoredUint(buf, uint64(v))
	case float32:
		return appendColoredFloat(buf, float64(v))
	case float64:
		return appendColoredFloat(buf, v)
	case []byte:
		return appendConsoleStringColor(buf, string(v), ansi.String)
	case nil:
		return appendConsoleStringColor(buf, "nil", ansi.Nil)
	default:
		lw := acquireLineWriter(io.Discard)
		lw.autoFlush = false
		writePTLogValueColored(lw, value, ansi.String)
		buf = append(buf, lw.buf...)
		releaseLineWriter(lw)
		return buf
	}
}

func appendConsoleStringColor(buf []byte, value string, color string) []byte {
	buf = append(buf, color...)
	buf = appendConsoleStringInline(buf, value)
	buf = append(buf, ansi.Reset...)
	return buf
}

func appendColoredLiteral(buf []byte, literal string, color string) []byte {
	buf = append(buf, color...)
	buf = append(buf, literal...)
	buf = append(buf, ansi.Reset...)
	return buf
}

func appendColoredInt(buf []byte, value int64) []byte {
	buf = append(buf, ansi.Num...)
	buf = strconvAppendInt(buf, value)
	buf = append(buf, ansi.Reset...)
	return buf
}

func appendColoredUint(buf []byte, value uint64) []byte {
	buf = append(buf, ansi.Num...)
	buf = strconvAppendUint(buf, value)
	buf = append(buf, ansi.Reset...)
	return buf
}

func appendColoredFloat(buf []byte, value float64) []byte {
	buf = append(buf, ansi.Num...)
	buf = strconv.AppendFloat(buf, value, 'f', -1, 64)
	buf = append(buf, ansi.Reset...)
	return buf
}
