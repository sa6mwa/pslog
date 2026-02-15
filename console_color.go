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
	palette      *ansi.Palette
	baseBytes    []byte
	hasBaseBytes bool
	lineHint     *atomic.Int64
	emit         consoleColorEmitFunc
}

func newConsoleColorLogger(cfg coreConfig, opts Options) *consoleColorLogger {
	configureConsoleScannerFromOptions(opts)
	palette := resolvePaletteOption(opts.Palette)
	logger := &consoleColorLogger{
		palette:  palette,
		base:     newLoggerBase(cfg, nil),
		lineHint: new(atomic.Int64),
	}
	claimTimeCacheOwnership(cfg.timeCache, ownerToken(logger))
	logger.rebuildBaseBytes()
	return logger
}

func appendConsoleKeyColor(buf []byte, key string, palette *ansi.Palette) []byte {
	buf = append(buf, ' ')
	buf = append(buf, palette.Key...)
	buf = append(buf, key...)
	buf = append(buf, '=')
	buf = append(buf, ansi.Reset...)
	return buf
}

func writeConsoleKeyColor(lw *lineWriter, key string, palette *ansi.Palette) {
	if key == "" {
		return
	}
	total := 1 + len(palette.Key) + len(key) + 1 + len(ansi.Reset)
	lw.reserve(total)
	lw.buf = append(lw.buf, ' ')
	lw.buf = append(lw.buf, palette.Key...)
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

func (l *consoleColorLogger) recordHint(n int) {
	updateLineHint(l.lineHint, n)
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

func (l *consoleColorLogger) Close() error {
	return closeLoggerRuntime(l.base.cfg.writer, l.base.cfg.timeCache, ownerToken(l))
}

func (l *consoleColorLogger) rebuildBaseBytes() {
	l.baseBytes = encodeConsoleFieldsColor(l.base.fields, l.palette)
	l.hasBaseBytes = len(l.baseBytes) > 0
	if l.base.cfg.includeLogLevel {
		l.base.cfg.logLevelValue = LevelString(l.base.cfg.currentLevel())
	}
	l.emit = selectConsoleColorEmit(l.base.cfg, l.hasBaseBytes)
}

func writeRuntimeConsoleColor(lw *lineWriter, keyvals []any, palette *ansi.Palette) {
	start := len(lw.buf)
	if writeRuntimeConsoleColorFast(lw, keyvals, palette) {
		return
	}
	lw.buf = lw.buf[:start]
	writeRuntimeConsoleColorSlow(lw, keyvals, palette)
}

func writeRuntimeConsoleColorFast(lw *lineWriter, keyvals []any, palette *ansi.Palette) bool {
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
		writeConsoleKeyColor(lw, key, palette)
		value := keyvals[i+1]
		writeConsoleValueColorInline(lw, value, palette)
		pair++
	}
	if len(keyvals)%2 != 0 {
		writeConsoleKeyColor(lw, argKeyName(pair), palette)
		value := keyvals[len(keyvals)-1]
		writeConsoleValueColorInline(lw, value, palette)
	}
	return true
}

func writeRuntimeConsoleColorSlow(lw *lineWriter, keyvals []any, palette *ansi.Palette) {
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
		writeConsoleKeyColor(lw, key, palette)
		value := keyvals[i+1]
		writeConsoleValueColorInline(lw, value, palette)
		pair++
	}
	if len(keyvals)%2 != 0 {
		writeConsoleKeyColor(lw, argKeyName(pair), palette)
		value := keyvals[len(keyvals)-1]
		writeConsoleValueColorInline(lw, value, palette)
	}
}

func encodeConsoleFieldsColor(fields []field, palette *ansi.Palette) []byte {
	if len(fields) == 0 {
		return nil
	}
	buf := make([]byte, 0, len(fields)*24)
	for _, f := range fields {
		if f.key == "" {
			continue
		}
		buf = appendConsoleKeyColor(buf, f.key, palette)
		buf = appendConsoleValueColor(buf, f.value, palette)
	}
	return buf
}

func writeConsoleFieldColor(lw *lineWriter, key string, value any, palette *ansi.Palette) {
	if key == "" {
		return
	}
	writeConsoleKeyColor(lw, key, palette)
	writeConsoleValueColor(lw, value, palette)
}

func writeConsoleTimestampColor(lw *lineWriter, ts string, palette *ansi.Palette) {
	writeConsoleColoredLiteral(lw, palette.Timestamp, ts)
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
	levelColor, levelLabel := consoleLevelColor(level, l.palette)
	estimate := len(l.baseBytes) + len(keyvals)*20 + 4
	estimate += len(levelLabel) + len(levelColor) + len(ansi.Reset)
	estimate += len(l.palette.Timestamp) + len(timestamp) + len(ansi.Reset) + 1
	estimate += len(l.palette.Key) + len("loglevel=") + len(ansi.Reset) + len(l.palette.String) + len(l.base.cfg.logLevelValue) + len(ansi.Reset)
	if msg != "" {
		estimate += len(l.palette.Message) + len(msg) + len(ansi.Reset) + 1
	}
	lw.reserve(estimate)
	writeConsoleTimestampColor(lw, timestamp, l.palette)
	lw.writeByte(' ')
	writeConsoleColoredLiteral(lw, levelColor, levelLabel)
	if msg != "" {
		lw.writeByte(' ')
		writeConsoleMessageColor(lw, msg, l.palette)
	}
	lw.writeBytes(l.baseBytes)
	writeRuntimeConsoleColor(lw, keyvals, l.palette)
	writeConsoleFieldColor(lw, "loglevel", l.base.cfg.logLevelValue, l.palette)
}

func emitConsoleColorTimestampLogLevelNoBaseFields(l *consoleColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	timestamp := l.base.cfg.timestamp()
	levelColor, levelLabel := consoleLevelColor(level, l.palette)
	estimate := len(keyvals)*20 + 4
	estimate += len(levelLabel) + len(levelColor) + len(ansi.Reset)
	estimate += len(l.palette.Timestamp) + len(timestamp) + len(ansi.Reset) + 1
	estimate += len(l.palette.Key) + len("loglevel=") + len(ansi.Reset) + len(l.palette.String) + len(l.base.cfg.logLevelValue) + len(ansi.Reset)
	if msg != "" {
		estimate += len(l.palette.Message) + len(msg) + len(ansi.Reset) + 1
	}
	lw.reserve(estimate)
	writeConsoleTimestampColor(lw, timestamp, l.palette)
	lw.writeByte(' ')
	writeConsoleColoredLiteral(lw, levelColor, levelLabel)
	if msg != "" {
		lw.writeByte(' ')
		writeConsoleMessageColor(lw, msg, l.palette)
	}
	writeRuntimeConsoleColor(lw, keyvals, l.palette)
	writeConsoleFieldColor(lw, "loglevel", l.base.cfg.logLevelValue, l.palette)
}

func emitConsoleColorTimestampWithBaseFields(l *consoleColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	timestamp := l.base.cfg.timestamp()
	levelColor, levelLabel := consoleLevelColor(level, l.palette)
	estimate := len(l.baseBytes) + len(keyvals)*20 + 4
	estimate += len(levelLabel) + len(levelColor) + len(ansi.Reset)
	estimate += len(l.palette.Timestamp) + len(timestamp) + len(ansi.Reset) + 1
	if msg != "" {
		estimate += len(l.palette.Message) + len(msg) + len(ansi.Reset) + 1
	}
	lw.reserve(estimate)
	writeConsoleTimestampColor(lw, timestamp, l.palette)
	lw.writeByte(' ')
	writeConsoleColoredLiteral(lw, levelColor, levelLabel)
	if msg != "" {
		lw.writeByte(' ')
		writeConsoleMessageColor(lw, msg, l.palette)
	}
	lw.writeBytes(l.baseBytes)
	writeRuntimeConsoleColor(lw, keyvals, l.palette)
}

func emitConsoleColorTimestampNoBaseFields(l *consoleColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	timestamp := l.base.cfg.timestamp()
	levelColor, levelLabel := consoleLevelColor(level, l.palette)
	estimate := len(keyvals)*20 + 4
	estimate += len(levelLabel) + len(levelColor) + len(ansi.Reset)
	estimate += len(l.palette.Timestamp) + len(timestamp) + len(ansi.Reset) + 1
	if msg != "" {
		estimate += len(l.palette.Message) + len(msg) + len(ansi.Reset) + 1
	}
	lw.reserve(estimate)
	writeConsoleTimestampColor(lw, timestamp, l.palette)
	lw.writeByte(' ')
	writeConsoleColoredLiteral(lw, levelColor, levelLabel)
	if msg != "" {
		lw.writeByte(' ')
		writeConsoleMessageColor(lw, msg, l.palette)
	}
	writeRuntimeConsoleColor(lw, keyvals, l.palette)
}

func emitConsoleColorLogLevelWithBaseFields(l *consoleColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	levelColor, levelLabel := consoleLevelColor(level, l.palette)
	estimate := len(l.baseBytes) + len(keyvals)*20 + 4
	estimate += len(levelLabel) + len(levelColor) + len(ansi.Reset)
	estimate += len(l.palette.Key) + len("loglevel=") + len(ansi.Reset) + len(l.palette.String) + len(l.base.cfg.logLevelValue) + len(ansi.Reset)
	if msg != "" {
		estimate += len(l.palette.Message) + len(msg) + len(ansi.Reset) + 1
	}
	lw.reserve(estimate)
	writeConsoleColoredLiteral(lw, levelColor, levelLabel)
	if msg != "" {
		lw.writeByte(' ')
		writeConsoleMessageColor(lw, msg, l.palette)
	}
	lw.writeBytes(l.baseBytes)
	writeRuntimeConsoleColor(lw, keyvals, l.palette)
	writeConsoleFieldColor(lw, "loglevel", l.base.cfg.logLevelValue, l.palette)
}

func emitConsoleColorLogLevelNoBaseFields(l *consoleColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	levelColor, levelLabel := consoleLevelColor(level, l.palette)
	estimate := len(keyvals)*20 + 4
	estimate += len(levelLabel) + len(levelColor) + len(ansi.Reset)
	estimate += len(l.palette.Key) + len("loglevel=") + len(ansi.Reset) + len(l.palette.String) + len(l.base.cfg.logLevelValue) + len(ansi.Reset)
	if msg != "" {
		estimate += len(l.palette.Message) + len(msg) + len(ansi.Reset) + 1
	}
	lw.reserve(estimate)
	writeConsoleColoredLiteral(lw, levelColor, levelLabel)
	if msg != "" {
		lw.writeByte(' ')
		writeConsoleMessageColor(lw, msg, l.palette)
	}
	writeRuntimeConsoleColor(lw, keyvals, l.palette)
	writeConsoleFieldColor(lw, "loglevel", l.base.cfg.logLevelValue, l.palette)
}

func emitConsoleColorBaseWithBaseFields(l *consoleColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	levelColor, levelLabel := consoleLevelColor(level, l.palette)
	estimate := len(l.baseBytes) + len(keyvals)*20 + 4
	estimate += len(levelLabel) + len(levelColor) + len(ansi.Reset)
	if msg != "" {
		estimate += len(l.palette.Message) + len(msg) + len(ansi.Reset) + 1
	}
	lw.reserve(estimate)
	writeConsoleColoredLiteral(lw, levelColor, levelLabel)
	if msg != "" {
		lw.writeByte(' ')
		writeConsoleMessageColor(lw, msg, l.palette)
	}
	lw.writeBytes(l.baseBytes)
	writeRuntimeConsoleColor(lw, keyvals, l.palette)
}

func emitConsoleColorBaseNoBaseFields(l *consoleColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	levelColor, levelLabel := consoleLevelColor(level, l.palette)
	estimate := len(keyvals)*20 + 4
	estimate += len(levelLabel) + len(levelColor) + len(ansi.Reset)
	if msg != "" {
		estimate += len(l.palette.Message) + len(msg) + len(ansi.Reset) + 1
	}
	lw.reserve(estimate)
	writeConsoleColoredLiteral(lw, levelColor, levelLabel)
	if msg != "" {
		lw.writeByte(' ')
		writeConsoleMessageColor(lw, msg, l.palette)
	}
	writeRuntimeConsoleColor(lw, keyvals, l.palette)
}
func consoleLevelColor(level Level, palette *ansi.Palette) (string, string) {
	switch level {
	case TraceLevel:
		return palette.Trace, "TRC"
	case DebugLevel:
		return palette.Debug, "DBG"
	case InfoLevel:
		return palette.Info, "INF"
	case WarnLevel:
		return palette.Warn, "WRN"
	case ErrorLevel:
		return palette.Error, "ERR"
	case FatalLevel:
		return palette.Fatal, "FTL"
	case PanicLevel:
		return palette.Panic, "PNC"
	case NoLevel:
		return palette.NoLevel, "---"
	default:
		return palette.Info, "INF"
	}
}

func writeConsoleMessageColor(lw *lineWriter, msg string, palette *ansi.Palette) {
	if msg == "" {
		return
	}
	lw.reserve(len(palette.Message) + len(msg) + len(ansi.Reset) + 8)
	lw.buf = append(lw.buf, palette.Message...)

	// Share the same escaping as plain messages to block control/ESC, but keep it
	// cheap for the common case.
	unsafePos := -1
	for i := 0; i < len(msg); i++ {
		c := msg[i]
		if c < 0x20 || c == '\\' || c == '"' || c == 0x7f || c == 0x1b {
			unsafePos = i
			break
		}
	}
	if unsafePos == -1 {
		lw.buf = append(lw.buf, msg...)
	} else {
		const hex = "0123456789abcdef"
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
	}

	lw.buf = append(lw.buf, ansi.Reset...)
	lw.maybeFlush()
}

func writeConsoleValueColorFast(lw *lineWriter, value any, palette *ansi.Palette) bool {
	switch v := value.(type) {
	case TrustedString:
		writeConsoleStringColor(lw, string(v), palette.String)
		return true
	case string:
		writeConsoleStringColor(lw, v, palette.String)
		return true
	case time.Time:
		writeConsoleStringColor(lw, lw.formatTimeRFC3339(v), palette.Timestamp)
		return true
	case time.Duration:
		writeConsoleStringColor(lw, lw.formatDuration(v), palette.String)
		return true
	case stringer:
		writeConsoleStringColor(lw, v.String(), palette.String)
		return true
	case error:
		writeConsoleStringColor(lw, v.Error(), palette.Error)
		return true
	case bool:
		writeConsoleBoolColor(lw, v, palette)
		return true
	case int:
		writeConsoleIntColor(lw, int64(v), palette)
		return true
	case int8:
		writeConsoleIntColor(lw, int64(v), palette)
		return true
	case int16:
		writeConsoleIntColor(lw, int64(v), palette)
		return true
	case int32:
		writeConsoleIntColor(lw, int64(v), palette)
		return true
	case int64:
		writeConsoleIntColor(lw, v, palette)
		return true
	case uint:
		writeConsoleUintColor(lw, uint64(v), palette)
		return true
	case uint8:
		writeConsoleUintColor(lw, uint64(v), palette)
		return true
	case uint16:
		writeConsoleUintColor(lw, uint64(v), palette)
		return true
	case uint32:
		writeConsoleUintColor(lw, uint64(v), palette)
		return true
	case uint64:
		writeConsoleUintColor(lw, v, palette)
		return true
	case uintptr:
		writeConsoleUintColor(lw, uint64(v), palette)
		return true
	case float32:
		writeConsoleFloatColor(lw, float64(v), palette)
		return true
	case float64:
		writeConsoleFloatColor(lw, v, palette)
		return true
	case []byte:
		writeConsoleStringColor(lw, string(v), palette.String)
		return true
	case nil:
		writeConsoleStringColor(lw, "nil", palette.Nil)
		return true
	}
	return false
}

func writeConsoleValueColorInline(lw *lineWriter, value any, palette *ansi.Palette) bool {
	switch v := value.(type) {
	case TrustedString:
		writeConsoleStringColor(lw, string(v), palette.String)
		return true
	case string:
		writeConsoleStringColor(lw, v, palette.String)
		return true
	case bool:
		writeConsoleBoolColor(lw, v, palette)
		return true
	case int:
		writeConsoleIntColor(lw, int64(v), palette)
		return true
	case int8:
		writeConsoleIntColor(lw, int64(v), palette)
		return true
	case int16:
		writeConsoleIntColor(lw, int64(v), palette)
		return true
	case int32:
		writeConsoleIntColor(lw, int64(v), palette)
		return true
	case int64:
		writeConsoleIntColor(lw, v, palette)
		return true
	case uint:
		writeConsoleUintColor(lw, uint64(v), palette)
		return true
	case uint8:
		writeConsoleUintColor(lw, uint64(v), palette)
		return true
	case uint16:
		writeConsoleUintColor(lw, uint64(v), palette)
		return true
	case uint32:
		writeConsoleUintColor(lw, uint64(v), palette)
		return true
	case uint64:
		writeConsoleUintColor(lw, v, palette)
		return true
	case uintptr:
		writeConsoleUintColor(lw, uint64(v), palette)
		return true
	case float32:
		writeConsoleFloatColor(lw, float64(v), palette)
		return true
	case float64:
		writeConsoleFloatColor(lw, v, palette)
		return true
	case time.Time:
		writeConsoleStringColor(lw, lw.formatTimeRFC3339(v), palette.Timestamp)
		return true
	case time.Duration:
		writeConsoleStringColor(lw, lw.formatDuration(v), palette.String)
		return true
	case stringer:
		writeConsoleStringColor(lw, v.String(), palette.String)
		return true
	case error:
		writeConsoleStringColor(lw, v.Error(), palette.Error)
		return true
	case []byte:
		writeConsoleStringColor(lw, string(v), palette.String)
		return true
	case nil:
		writeConsoleStringColor(lw, "nil", palette.Nil)
		return true
	default:
		writePTLogValueColored(lw, v, palette.String)
		return true
	}
}

func writeConsoleValueColor(lw *lineWriter, value any, palette *ansi.Palette) {

	switch v := value.(type) {
	case TrustedString:
		writeConsoleStringColor(lw, string(v), palette.String)
	case string:
		writeConsoleStringColor(lw, v, palette.String)
	case time.Time:
		writeConsoleStringColor(lw, lw.formatTimeRFC3339(v), palette.Timestamp)
	case time.Duration:
		writeConsoleStringColor(lw, lw.formatDuration(v), palette.String)
	case stringer:
		writeConsoleStringColor(lw, v.String(), palette.String)
	case error:
		writeConsoleStringColor(lw, v.Error(), palette.Error)
	case bool:
		writeConsoleBoolColor(lw, v, palette)
	case int:
		writeConsoleIntColor(lw, int64(v), palette)
	case int8:
		writeConsoleIntColor(lw, int64(v), palette)
	case int16:
		writeConsoleIntColor(lw, int64(v), palette)
	case int32:
		writeConsoleIntColor(lw, int64(v), palette)
	case int64:
		writeConsoleIntColor(lw, v, palette)
	case uint:
		writeConsoleUintColor(lw, uint64(v), palette)
	case uint8:
		writeConsoleUintColor(lw, uint64(v), palette)
	case uint16:
		writeConsoleUintColor(lw, uint64(v), palette)
	case uint32:
		writeConsoleUintColor(lw, uint64(v), palette)
	case uint64:
		writeConsoleUintColor(lw, v, palette)
	case uintptr:
		writeConsoleUintColor(lw, uint64(v), palette)
	case float32:
		writeConsoleFloatColor(lw, float64(v), palette)
	case float64:
		writeConsoleFloatColor(lw, v, palette)
	case []byte:
		writeConsoleStringColor(lw, string(v), palette.String)
	case nil:
		writeConsoleStringColor(lw, "nil", palette.Nil)
	default:
		writePTLogValueColored(lw, v, palette.String)
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

func writeConsoleBoolColor(lw *lineWriter, value bool, palette *ansi.Palette) {
	literal := "false"
	if value {
		literal = "true"
	}
	writeConsoleColoredLiteral(lw, palette.Bool, literal)
}

func writeConsoleIntColor(lw *lineWriter, value int64, palette *ansi.Palette) {
	lw.reserve(len(palette.Num) + 24 + len(ansi.Reset))
	lw.buf = append(lw.buf, palette.Num...)
	lw.buf = strconv.AppendInt(lw.buf, value, 10)
	lw.buf = append(lw.buf, ansi.Reset...)
	lw.maybeFlush()
}

func writeConsoleUintColor(lw *lineWriter, value uint64, palette *ansi.Palette) {
	lw.reserve(len(palette.Num) + 24 + len(ansi.Reset))
	lw.buf = append(lw.buf, palette.Num...)
	lw.buf = strconv.AppendUint(lw.buf, value, 10)
	lw.buf = append(lw.buf, ansi.Reset...)
	lw.maybeFlush()
}

func writeConsoleFloatColor(lw *lineWriter, value float64, palette *ansi.Palette) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		writeConsoleColoredLiteral(lw, palette.Num, strconv.FormatFloat(value, 'f', -1, 64))
		return
	}
	lw.reserve(len(palette.Num) + 32 + len(ansi.Reset))
	lw.buf = append(lw.buf, palette.Num...)
	lw.buf = strconv.AppendFloat(lw.buf, value, 'f', -1, 64)
	lw.buf = append(lw.buf, ansi.Reset...)
	lw.maybeFlush()
}

func appendConsoleValueColor(buf []byte, value any, palette *ansi.Palette) []byte {
	switch v := value.(type) {
	case string:
		return appendConsoleStringColor(buf, v, palette.String)
	case time.Time:
		return appendConsoleStringColor(buf, v.Format(time.RFC3339), palette.Timestamp)
	case time.Duration:
		return appendConsoleStringColor(buf, v.String(), palette.String)
	case stringer:
		return appendConsoleStringColor(buf, v.String(), palette.String)
	case error:
		return appendConsoleStringColor(buf, v.Error(), palette.Error)
	case bool:
		if v {
			return appendColoredLiteral(buf, "true", palette.Bool)
		}
		return appendColoredLiteral(buf, "false", palette.Bool)
	case int:
		return appendColoredInt(buf, int64(v), palette.Num)
	case int8:
		return appendColoredInt(buf, int64(v), palette.Num)
	case int16:
		return appendColoredInt(buf, int64(v), palette.Num)
	case int32:
		return appendColoredInt(buf, int64(v), palette.Num)
	case int64:
		return appendColoredInt(buf, v, palette.Num)
	case uint:
		return appendColoredUint(buf, uint64(v), palette.Num)
	case uint8:
		return appendColoredUint(buf, uint64(v), palette.Num)
	case uint16:
		return appendColoredUint(buf, uint64(v), palette.Num)
	case uint32:
		return appendColoredUint(buf, uint64(v), palette.Num)
	case uint64:
		return appendColoredUint(buf, v, palette.Num)
	case uintptr:
		return appendColoredUint(buf, uint64(v), palette.Num)
	case float32:
		return appendColoredFloat(buf, float64(v), palette.Num)
	case float64:
		return appendColoredFloat(buf, v, palette.Num)
	case []byte:
		return appendConsoleStringColor(buf, string(v), palette.String)
	case nil:
		return appendConsoleStringColor(buf, "nil", palette.Nil)
	default:
		lw := acquireLineWriter(io.Discard)
		lw.autoFlush = false
		writePTLogValueColored(lw, value, palette.String)
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

func appendColoredInt(buf []byte, value int64, numColor string) []byte {
	buf = append(buf, numColor...)
	buf = strconvAppendInt(buf, value)
	buf = append(buf, ansi.Reset...)
	return buf
}

func appendColoredUint(buf []byte, value uint64, numColor string) []byte {
	buf = append(buf, numColor...)
	buf = strconvAppendUint(buf, value)
	buf = append(buf, ansi.Reset...)
	return buf
}

func appendColoredFloat(buf []byte, value float64, numColor string) []byte {
	buf = append(buf, numColor...)
	buf = strconv.AppendFloat(buf, value, 'f', -1, 64)
	buf = append(buf, ansi.Reset...)
	return buf
}
