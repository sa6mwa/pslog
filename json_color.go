package pslog

import (
	"io"
	"sync/atomic"
	"time"

	"pkt.systems/pslog/ansi"
)

type jsonColorEmitFunc func(l *jsonColorLogger, lw *lineWriter, level Level, msg string, keyvals []any)

type jsonColorLogger struct {
	base           loggerBase
	tsKeyData      []byte
	lvlKeyData     []byte
	msgKeyData     []byte
	basePayload    []byte
	hasBasePayload bool
	logLevelKey    []byte
	lineHint       *atomic.Int64
	verboseField   bool
	emit           jsonColorEmitFunc
}

func writeColoredJSONStringField(lw *lineWriter, first *bool, keyData []byte, value string, color string, trusted bool) {
	appendKeyDataWithFirst(lw, first, keyData)
	if trusted {
		writePTJSONStringTrustedColored(lw, color, value)
		return
	}
	writePTJSONStringColored(lw, color, value)
}

func newJSONColorLogger(cfg coreConfig, opts Options) *jsonColorLogger {
	tsKey := "ts"
	lvlKey := "lvl"
	msgKey := "msg"
	if opts.VerboseFields {
		tsKey = "time"
		lvlKey = "level"
		msgKey = "message"
	}
	configureJSONEscapeFromOptions(opts)
	logger := &jsonColorLogger{
		base:         newLoggerBase(cfg, nil),
		tsKeyData:    makeColoredKey(tsKey, ansi.Key, false),
		lvlKeyData:   makeColoredKey(lvlKey, ansi.Key, true),
		msgKeyData:   makeColoredKey(msgKey, ansi.MessageKey, true),
		logLevelKey:  makeColoredKey("loglevel", ansi.Key, true),
		lineHint:     new(atomic.Int64),
		verboseField: opts.VerboseFields,
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
	if l.lineHint != nil {
		l.lineHint.Store(int64(lw.lastLineLength()))
	}
	releaseLineWriter(lw)
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
	l.hasBasePayload = len(l.basePayload) > 0
	if l.base.cfg.includeLogLevel {
		l.base.cfg.logLevelValue = LevelString(l.base.cfg.currentLevel())
	}
	l.emit = selectJSONColorEmit(l.base.cfg, l.hasBasePayload)
}

func selectJSONColorEmit(cfg coreConfig, hasStaticFields bool) jsonColorEmitFunc {
	switch {
	case cfg.includeTimestamp && cfg.includeLogLevel:
		if hasStaticFields {
			return emitJSONColorTimestampLogLevelWithStaticFields
		}
		return emitJSONColorTimestampLogLevelNoStaticFields
	case cfg.includeTimestamp:
		if hasStaticFields {
			return emitJSONColorTimestampWithStaticFields
		}
		return emitJSONColorTimestampNoStaticFields
	case cfg.includeLogLevel:
		if hasStaticFields {
			return emitJSONColorLogLevelWithStaticFields
		}
		return emitJSONColorLogLevelNoStaticFields
	default:
		if hasStaticFields {
			return emitJSONColorBaseWithStaticFields
		}
		return emitJSONColorBaseNoStaticFields
	}
}

func emitJSONColorTimestampLogLevelWithStaticFields(l *jsonColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	timestamp := l.base.cfg.timestamp()
	levelColor := colorForLevel(level)
	levelLabel := LevelString(level)
	estimate := 2 + len(l.basePayload) + len(l.lvlKeyData) + len(levelLabel) +
		len(levelColor) + len(ansi.Reset) +
		len(l.tsKeyData) + len(timestamp) + len(ansi.Timestamp) + len(ansi.Reset) +
		len(l.logLevelKey) + len(l.base.cfg.logLevelValue) + len(ansi.String) + len(ansi.Reset)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg) + len(ansi.Message) + len(ansi.Reset)
	}
	if n := len(keyvals); n > 0 {
		estimate += n*8 + n*(len(ansi.Key)+len(ansi.Reset))
	}
	lw.reserve(estimate)
	lw.writeByte('{')
	first := true
	writeColoredJSONStringField(lw, &first, l.tsKeyData, timestamp, ansi.Timestamp, l.base.cfg.timestampTrusted)
	writeColoredJSONStringField(lw, &first, l.lvlKeyData, levelLabel, levelColor, true)
	if msg != "" {
		appendKeyDataWithFirst(lw, &first, l.msgKeyData)
		writeColoredJSONString(lw, msg, ansi.Message)
	}
	lw.writeBytes(l.basePayload)
	first = false
	writeRuntimeJSONFieldsColor(lw, &first, keyvals)
	writeColoredJSONStringField(lw, &first, l.logLevelKey, l.base.cfg.logLevelValue, ansi.String, true)
	lw.writeByte('}')
}

func emitJSONColorTimestampLogLevelNoStaticFields(l *jsonColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	timestamp := l.base.cfg.timestamp()
	levelColor := colorForLevel(level)
	levelLabel := LevelString(level)
	estimate := 2 + len(l.lvlKeyData) + len(levelLabel) +
		len(levelColor) + len(ansi.Reset) +
		len(l.tsKeyData) + len(timestamp) + len(ansi.Timestamp) + len(ansi.Reset) +
		len(l.logLevelKey) + len(l.base.cfg.logLevelValue) + len(ansi.String) + len(ansi.Reset)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg) + len(ansi.Message) + len(ansi.Reset)
	}
	if n := len(keyvals); n > 0 {
		estimate += n*8 + n*(len(ansi.Key)+len(ansi.Reset))
	}
	lw.reserve(estimate)
	lw.writeByte('{')
	first := true
	writeColoredJSONStringField(lw, &first, l.tsKeyData, timestamp, ansi.Timestamp, l.base.cfg.timestampTrusted)
	writeColoredJSONStringField(lw, &first, l.lvlKeyData, levelLabel, levelColor, true)
	if msg != "" {
		appendKeyDataWithFirst(lw, &first, l.msgKeyData)
		writeColoredJSONString(lw, msg, ansi.Message)
	}
	writeRuntimeJSONFieldsColor(lw, &first, keyvals)
	writeColoredJSONStringField(lw, &first, l.logLevelKey, l.base.cfg.logLevelValue, ansi.String, true)
	lw.writeByte('}')
}

func emitJSONColorTimestampWithStaticFields(l *jsonColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	timestamp := l.base.cfg.timestamp()
	levelColor := colorForLevel(level)
	levelLabel := LevelString(level)
	estimate := 2 + len(l.basePayload) + len(l.lvlKeyData) + len(levelLabel) +
		len(levelColor) + len(ansi.Reset) +
		len(l.tsKeyData) + len(timestamp) + len(ansi.Timestamp) + len(ansi.Reset)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg) + len(ansi.Message) + len(ansi.Reset)
	}
	if n := len(keyvals); n > 0 {
		estimate += n*8 + n*(len(ansi.Key)+len(ansi.Reset))
	}
	lw.reserve(estimate)
	lw.writeByte('{')
	first := true
	writeColoredJSONStringField(lw, &first, l.tsKeyData, timestamp, ansi.Timestamp, l.base.cfg.timestampTrusted)
	writeColoredJSONStringField(lw, &first, l.lvlKeyData, levelLabel, levelColor, true)
	if msg != "" {
		appendKeyDataWithFirst(lw, &first, l.msgKeyData)
		writeColoredJSONString(lw, msg, ansi.Message)
	}
	lw.writeBytes(l.basePayload)
	first = false
	writeRuntimeJSONFieldsColor(lw, &first, keyvals)
	lw.writeByte('}')
}

func emitJSONColorTimestampNoStaticFields(l *jsonColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	timestamp := l.base.cfg.timestamp()
	levelColor := colorForLevel(level)
	levelLabel := LevelString(level)
	estimate := 2 + len(l.lvlKeyData) + len(levelLabel) +
		len(levelColor) + len(ansi.Reset) +
		len(l.tsKeyData) + len(timestamp) + len(ansi.Timestamp) + len(ansi.Reset)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg) + len(ansi.Message) + len(ansi.Reset)
	}
	if n := len(keyvals); n > 0 {
		estimate += n*8 + n*(len(ansi.Key)+len(ansi.Reset))
	}
	lw.reserve(estimate)
	lw.writeByte('{')
	first := true
	writeColoredJSONStringField(lw, &first, l.tsKeyData, timestamp, ansi.Timestamp, l.base.cfg.timestampTrusted)
	writeColoredJSONStringField(lw, &first, l.lvlKeyData, levelLabel, levelColor, true)
	if msg != "" {
		appendKeyDataWithFirst(lw, &first, l.msgKeyData)
		writeColoredJSONString(lw, msg, ansi.Message)
	}
	writeRuntimeJSONFieldsColor(lw, &first, keyvals)
	lw.writeByte('}')
}

func emitJSONColorLogLevelWithStaticFields(l *jsonColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	levelColor := colorForLevel(level)
	levelLabel := LevelString(level)
	estimate := 2 + len(l.basePayload) + len(l.lvlKeyData) + len(levelLabel) +
		len(levelColor) + len(ansi.Reset) +
		len(l.logLevelKey) + len(l.base.cfg.logLevelValue) + len(ansi.String) + len(ansi.Reset)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg) + len(ansi.Message) + len(ansi.Reset)
	}
	if n := len(keyvals); n > 0 {
		estimate += n*8 + n*(len(ansi.Key)+len(ansi.Reset))
	}
	lw.reserve(estimate)
	lw.writeByte('{')
	first := true
	writeColoredJSONStringField(lw, &first, l.lvlKeyData, levelLabel, levelColor, true)
	if msg != "" {
		appendKeyDataWithFirst(lw, &first, l.msgKeyData)
		writeColoredJSONString(lw, msg, ansi.Message)
	}
	lw.writeBytes(l.basePayload)
	first = false
	writeRuntimeJSONFieldsColor(lw, &first, keyvals)
	writeColoredJSONStringField(lw, &first, l.logLevelKey, l.base.cfg.logLevelValue, ansi.String, true)
	lw.writeByte('}')
}

func emitJSONColorLogLevelNoStaticFields(l *jsonColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	levelColor := colorForLevel(level)
	levelLabel := LevelString(level)
	estimate := 2 + len(l.lvlKeyData) + len(levelLabel) +
		len(levelColor) + len(ansi.Reset) +
		len(l.logLevelKey) + len(l.base.cfg.logLevelValue) + len(ansi.String) + len(ansi.Reset)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg) + len(ansi.Message) + len(ansi.Reset)
	}
	if n := len(keyvals); n > 0 {
		estimate += n*8 + n*(len(ansi.Key)+len(ansi.Reset))
	}
	lw.reserve(estimate)
	lw.writeByte('{')
	first := true
	writeColoredJSONStringField(lw, &first, l.lvlKeyData, levelLabel, levelColor, true)
	if msg != "" {
		appendKeyDataWithFirst(lw, &first, l.msgKeyData)
		writeColoredJSONString(lw, msg, ansi.Message)
	}
	writeRuntimeJSONFieldsColor(lw, &first, keyvals)
	writeColoredJSONStringField(lw, &first, l.logLevelKey, l.base.cfg.logLevelValue, ansi.String, true)
	lw.writeByte('}')
}

func emitJSONColorBaseWithStaticFields(l *jsonColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	levelColor := colorForLevel(level)
	levelLabel := LevelString(level)
	estimate := 2 + len(l.basePayload) + len(l.lvlKeyData) + len(levelLabel) +
		len(levelColor) + len(ansi.Reset)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg) + len(ansi.Message) + len(ansi.Reset)
	}
	if n := len(keyvals); n > 0 {
		estimate += n*8 + n*(len(ansi.Key)+len(ansi.Reset))
	}
	lw.reserve(estimate)
	lw.writeByte('{')
	first := true
	writeColoredJSONStringField(lw, &first, l.lvlKeyData, levelLabel, levelColor, true)
	if msg != "" {
		appendKeyDataWithFirst(lw, &first, l.msgKeyData)
		writeColoredJSONString(lw, msg, ansi.Message)
	}
	lw.writeBytes(l.basePayload)
	first = false
	writeRuntimeJSONFieldsColor(lw, &first, keyvals)
	lw.writeByte('}')
}

func emitJSONColorBaseNoStaticFields(l *jsonColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	levelColor := colorForLevel(level)
	levelLabel := LevelString(level)
	estimate := 2 + len(l.lvlKeyData) + len(levelLabel) +
		len(levelColor) + len(ansi.Reset)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg) + len(ansi.Message) + len(ansi.Reset)
	}
	if n := len(keyvals); n > 0 {
		estimate += n*8 + n*(len(ansi.Key)+len(ansi.Reset))
	}
	lw.reserve(estimate)
	lw.writeByte('{')
	first := true
	writeColoredJSONStringField(lw, &first, l.lvlKeyData, levelLabel, levelColor, true)
	if msg != "" {
		appendKeyDataWithFirst(lw, &first, l.msgKeyData)
		writeColoredJSONString(lw, msg, ansi.Message)
	}
	writeRuntimeJSONFieldsColor(lw, &first, keyvals)
	lw.writeByte('}')
}

func writeRuntimeJSONFieldsColor(lw *lineWriter, first *bool, keyvals []any) {
	if writeRuntimeJSONFieldsColorFast(lw, first, keyvals) {
		return
	}
	writeRuntimeJSONFieldsColorSlow(lw, first, keyvals)
}

func writeRuntimeJSONFieldsColorFast(lw *lineWriter, first *bool, keyvals []any) bool {
	n := len(keyvals)
	if n == 0 {
		return true
	}
	pairIndex := 0
	for i := 0; i+1 < n; i += 2 {
		var key string
		var trusted bool
		switch k := keyvals[i].(type) {
		case TrustedString:
			key = string(k)
			trusted = true
		case string:
			key = k
			trusted = stringTrustedASCII(key)
		default:
			return false
		}
		if key == "" {
			pairIndex++
			continue
		}
		if *first {
			*first = false
		} else {
			lw.writeByte(',')
		}
		writeColoredKey(lw, key, ansi.Key, trusted)
		lw.writeByte(':')
		value := keyvals[i+1]
		color := jsonColorForValue(value)
		if !writeRuntimeValueColorInline(lw, value, color) {
			if !writeRuntimeValueColor(lw, value, color) {
				writeColoredValue(lw, value, color)
			}
		}
		pairIndex++
	}
	if n&1 == 1 {
		value := keyvals[n-1]
		if *first {
			*first = false
		} else {
			lw.writeByte(',')
		}
		writeColoredKey(lw, argKeyName(pairIndex), ansi.Key, false)
		lw.writeByte(':')
		color := jsonColorForValue(value)
		if !writeRuntimeValueColor(lw, value, color) {
			writeColoredValue(lw, value, color)
		}
	}
	return true
}

func writeRuntimeJSONFieldsColorSlow(lw *lineWriter, first *bool, keyvals []any) {
	n := len(keyvals)
	if n == 0 {
		return
	}

	pairIndex := 0
	if n >= 2 {
		pairIndex = writeRuntimeJSONPairColor(lw, first, keyvals[0], keyvals[1], pairIndex)
		if n >= 4 {
			pairIndex = writeRuntimeJSONPairColor(lw, first, keyvals[2], keyvals[3], pairIndex)
			for i := 4; i+1 < n; i += 2 {
				pairIndex = writeRuntimeJSONPairColor(lw, first, keyvals[i], keyvals[i+1], pairIndex)
			}
			if n&1 == 1 {
				writeRuntimeJSONOddColor(lw, first, keyvals[n-1], pairIndex)
			}
			return
		}
		if n == 3 {
			writeRuntimeJSONOddColor(lw, first, keyvals[2], pairIndex)
		}
		return
	}

	writeRuntimeJSONOddColor(lw, first, keyvals[0], pairIndex)
}

func writeRuntimeJSONPairColor(lw *lineWriter, first *bool, key any, value any, pairIndex int) int {
	k, trusted := runtimeKeyFromValue(key, pairIndex)
	if k != "" {
		if *first {
			*first = false
		} else {
			lw.writeByte(',')
		}
		writeColoredKey(lw, k, ansi.Key, trusted)
		lw.writeByte(':')
		color := jsonColorForValue(value)
		if !writeRuntimeValueColor(lw, value, color) {
			writeColoredValue(lw, value, color)
		}
	}
	return pairIndex + 1
}

func writeRuntimeJSONOddColor(lw *lineWriter, first *bool, value any, pairIndex int) {
	if *first {
		*first = false
	} else {
		lw.writeByte(',')
	}
	writeColoredKey(lw, argKeyName(pairIndex), ansi.Key, false)
	lw.writeByte(':')
	color := jsonColorForValue(value)
	if !writeRuntimeValueColorInline(lw, value, color) {
		if !writeRuntimeValueColor(lw, value, color) {
			writeColoredValue(lw, value, color)
		}
	}
}

func writeColoredKey(lw *lineWriter, key string, color string, trusted bool) {
	if trusted {
		writePTJSONStringTrustedColored(lw, color, key)
		return
	}
	writePTJSONStringColored(lw, color, key)
}

func writeColoredValue(lw *lineWriter, value any, color string) {
	if !writePTLogValueColoredFast(lw, value, color) {
		writePTLogValueColored(lw, value, color)
	}
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
		writeColoredKey(lw, f.key, ansi.Key, f.trustedKey)
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
