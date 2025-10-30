package pslog

import (
	"io"
	"sync/atomic"
	"time"

	"pkt.systems/pslog/ansi"
)

type jsonColorLogger struct {
	base         loggerBase
	tsKeyData    []byte
	lvlKeyData   []byte
	msgKeyData   []byte
	basePayload  []byte
	logLevelKey  []byte
	lineHint     *atomic.Int64
	verboseField bool
}

func newJSONColorLogger(cfg coreConfig, verbose bool) *jsonColorLogger {
	tsKey := "ts"
	lvlKey := "lvl"
	msgKey := "msg"
	if verbose {
		tsKey = "time"
		lvlKey = "level"
		msgKey = "message"
	}
	logger := &jsonColorLogger{
		base:         newLoggerBase(cfg, nil),
		tsKeyData:    makeColoredKey(tsKey, ansi.Key, false),
		lvlKeyData:   makeColoredKey(lvlKey, ansi.Key, true),
		msgKeyData:   makeColoredKey(msgKey, ansi.MessageKey, true),
		logLevelKey:  makeColoredKey("loglevel", ansi.Key, true),
		lineHint:     new(atomic.Int64),
		verboseField: verbose,
	}
	logger.rebuildBasePayload()
	return logger
}

func (l *jsonColorLogger) Trace(msg string, keyvals ...any) { l.log(TraceLevel, msg, keyvals...) }
func (l *jsonColorLogger) Debug(msg string, keyvals ...any) { l.log(DebugLevel, msg, keyvals...) }
func (l *jsonColorLogger) Info(msg string, keyvals ...any)  { l.log(InfoLevel, msg, keyvals...) }
func (l *jsonColorLogger) Warn(msg string, keyvals ...any)  { l.log(WarnLevel, msg, keyvals...) }
func (l *jsonColorLogger) Error(msg string, keyvals ...any) { l.log(ErrorLevel, msg, keyvals...) }

func (l *jsonColorLogger) Fatal(msg string, keyvals ...any) {
	l.log(FatalLevel, msg, keyvals...)
	exitProcess()
}

func (l *jsonColorLogger) Panic(msg string, keyvals ...any) {
	l.log(PanicLevel, msg, keyvals...)
	panic(msg)
}

func (l *jsonColorLogger) Log(level Level, msg string, keyvals ...any) {
	l.log(level, msg, keyvals...)
}

func (l *jsonColorLogger) log(level Level, msg string, keyvals ...any) {
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
	l.writeLine(lw, level, msg, timestamp, keyvals)
	lw.finishLine()
	lw.commit()
	if l.lineHint != nil {
		l.lineHint.Store(int64(lw.lastLineLength()))
	}
	releaseLineWriter(lw)
}

func (l *jsonColorLogger) writeLine(lw *lineWriter, level Level, msg string, timestamp string, keyvals []any) {
	levelColor := colorForLevel(level)
	estimate := 2 + len(l.basePayload) + len(l.lvlKeyData) + len(LevelString(level)) + len(levelColor) + len(ansi.Reset)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg) + len(ansi.Message) + len(ansi.Reset)
	}
	if l.base.cfg.includeTimestamp {
		estimate += len(l.tsKeyData) + len(timestamp) + len(ansi.Timestamp) + len(ansi.Reset)
	}
	if l.base.cfg.includeLogLevel {
		estimate += len(l.logLevelKey) + len(l.base.cfg.logLevelValue) + len(ansi.String) + len(ansi.Reset)
	}
	if n := len(keyvals); n > 0 {
		estimate += n*8 + n*(len(ansi.Key)+len(ansi.Reset))
	}
	lw.reserve(estimate)

	lw.writeByte('{')
	first := true
	if l.base.cfg.includeTimestamp {
		lw.buf = appendColoredKeyData(lw.buf, l.tsKeyData, first)
		writePTJSONStringTrustedColored(lw, ansi.Timestamp, timestamp)
		first = false
	}
	lw.buf = appendColoredKeyData(lw.buf, l.lvlKeyData, first)
	writePTJSONStringTrustedColored(lw, levelColor, LevelString(level))
	first = false
	if msg != "" {
		lw.buf = appendColoredKeyData(lw.buf, l.msgKeyData, first)
		safe := promoteTrustedValueString(msg)
		if safe {
			writePTJSONStringTrustedColored(lw, ansi.Message, msg)
		} else {
			writeColoredJSONString(lw, msg, ansi.Message)
		}
	}
	if len(l.basePayload) > 0 {
		lw.writeBytes(l.basePayload)
	}
	writeRuntimeJSONFieldsColor(lw, keyvals)
	if l.base.cfg.includeLogLevel {
		lw.writeBytes(l.logLevelKey)
		writePTJSONStringTrustedColored(lw, ansi.String, l.base.cfg.logLevelValue)
	}
	lw.writeByte('}')
}

func (l *jsonColorLogger) With(keyvals ...any) Logger {
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
	clone.rebuildBasePayload()
	return &clone
}

func (l *jsonColorLogger) WithLogLevel() Logger {
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
	clone.rebuildBasePayload()
	return &clone
}

func (l *jsonColorLogger) LogLevel(level Level) Logger {
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
	clone.rebuildBasePayload()
	return &clone
}

func (l *jsonColorLogger) LogLevelFromEnv(key string) Logger {
	if level, ok := LevelFromEnv(key); ok {
		return l.LogLevel(level)
	}
	return l
}

func (l *jsonColorLogger) rebuildBasePayload() {
	l.basePayload = encodeBaseJSONColor(l.base.fields)
	if l.base.cfg.includeLogLevel {
		l.base.cfg.logLevelValue = LevelString(l.base.cfg.currentLevel())
	}
}

func writeRuntimeJSONFieldsColor(lw *lineWriter, keyvals []any) {
	if len(keyvals) == 0 {
		return
	}
	first := false
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
		if key == "" {
			continue
		}
		trusted := promoteTrustedKey(key)
		writeColoredField(lw, &first, key, value, jsonColorForValue(value), trusted)
	}
}

func writeColoredField(lw *lineWriter, first *bool, key string, value any, color string, keyTrusted bool) {
	if *first {
		*first = false
	} else {
		lw.writeByte(',')
	}
	writeColoredKey(lw, key, ansi.Key, keyTrusted)
	lw.writeByte(':')
	writeColoredValue(lw, value, color)
}

func writeColoredKey(lw *lineWriter, key string, color string, trusted bool) {
	if trusted {
		writePTJSONStringTrustedColored(lw, color, key)
		return
	}
	writePTJSONStringColored(lw, color, key)
}

func writeColoredValue(lw *lineWriter, value any, color string) {
	writePTLogValueColored(lw, value, color)
}

func encodeBaseJSONColor(fields []field) []byte {
	if len(fields) == 0 {
		return nil
	}
	lw := acquireLineWriter(io.Discard)
	lw.autoFlush = false
	for _, f := range fields {
		if f.key == "" {
			continue
		}
		lw.writeByte(',')
		writeColoredKey(lw, f.key, ansi.Key, promoteTrustedKey(f.key))
		lw.writeByte(':')
		writeColoredValue(lw, f.value, jsonColorForValue(f.value))
	}
	data := append([]byte(nil), lw.buf...)
	releaseLineWriter(lw)
	return data
}

func makeColoredKey(key string, color string, leadingComma bool) []byte {
	lw := acquireLineWriter(io.Discard)
	lw.autoFlush = false
	if leadingComma {
		lw.writeByte(',')
	}
	writeColoredKey(lw, key, color, promoteTrustedKey(key))
	lw.writeByte(':')
	data := append([]byte(nil), lw.buf...)
	releaseLineWriter(lw)
	return data
}

func appendColoredKeyData(dst []byte, keyData []byte, first bool) []byte {
	if first && len(keyData) > 0 && keyData[0] == ',' {
		return append(dst, keyData[1:]...)
	}
	return append(dst, keyData...)
}

func writeColoredJSONString(lw *lineWriter, value string, color string) {
	writePTJSONStringColored(lw, color, value)
}

func jsonColorForValue(value any) string {
	switch value.(type) {
	case error:
		return ansi.Error
	case TrustedString, string, time.Duration, []byte:
		return ansi.String
	case time.Time:
		return ansi.Timestamp
	case stringer:
		return ansi.String
	case bool:
		return ansi.Bool
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, uintptr,
		float32, float64:
		return ansi.Num
	case nil:
		return ansi.Nil
	default:
		return ""
	}
}

func colorForLevel(level Level) string {
	switch level {
	case TraceLevel:
		return ansi.Trace
	case DebugLevel:
		return ansi.Debug
	case InfoLevel:
		return ansi.Info
	case WarnLevel:
		return ansi.Warn
	case ErrorLevel:
		return ansi.Error
	case FatalLevel, PanicLevel:
		return ansi.Fatal
	case NoLevel:
		return ansi.NoLevel
	default:
		return ansi.Info
	}
}
