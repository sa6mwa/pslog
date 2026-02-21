package pslog

import (
	"context"
	"io"
	"sync/atomic"
	"time"

	"pkt.systems/pslog/ansi"
)

type jsonColorEmitFunc func(l *jsonColorLogger, lw *lineWriter, level Level, msg string, keyvals []any)

type jsonColorLogger struct {
	base           loggerBase
	palette        *ansi.Palette
	tsKeyData      []byte
	lvlKeyData     []byte
	msgKeyData     []byte
	basePayload    []byte
	hasBasePayload bool
	logLevelKey    []byte
	lineHint       *atomic.Int64
	floatPolicy    NonFiniteFloatPolicy
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

func newJSONColorLogger(ctx context.Context, cfg coreConfig, opts Options) *jsonColorLogger {
	tsKey := "ts"
	lvlKey := "lvl"
	msgKey := "msg"
	if opts.VerboseFields {
		tsKey = "time"
		lvlKey = "level"
		msgKey = "message"
	}
	configureJSONEscapeFromOptions(opts)
	palette := resolvePaletteOption(opts.Palette)
	logger := &jsonColorLogger{
		palette:      palette,
		base:         newLoggerBase(cfg, nil),
		tsKeyData:    makeColoredKey(tsKey, palette.Key, false),
		lvlKeyData:   makeColoredKey(lvlKey, palette.Key, true),
		msgKeyData:   makeColoredKey(msgKey, palette.MessageKey, true),
		logLevelKey:  makeColoredKey("loglevel", palette.Key, true),
		floatPolicy:  normalizeNonFiniteFloatPolicy(opts.NonFiniteFloatPolicy),
		lineHint:     new(atomic.Int64),
		verboseField: opts.VerboseFields,
	}
	owner := ownerToken(logger)
	claimTimeCacheOwnership(cfg.timeCache, owner)
	claimContextCancellation(ctx, cfg.writer, cfg.timeCache, owner)
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
	if l.floatPolicy != NonFiniteFloatAsString {
		lw.floatPolicy = l.floatPolicy
	}
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

func (l *jsonColorLogger) Close() error {
	return closeLoggerRuntime(l.base.cfg.writer, l.base.cfg.timeCache, ownerToken(l))
}

func (l *jsonColorLogger) rebuildBasePayload() {
	l.basePayload = encodeBaseJSONColor(l.base.fields, l.palette, l.floatPolicy)
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
	levelColor := colorForLevel(level, l.palette)
	levelLabel := LevelString(level)
	estimate := 2 + len(l.basePayload) + len(l.lvlKeyData) + len(levelLabel) +
		len(levelColor) + len(ansi.Reset) +
		len(l.tsKeyData) + len(timestamp) + len(l.palette.Timestamp) + len(ansi.Reset) +
		len(l.logLevelKey) + len(l.base.cfg.logLevelValue) + len(l.palette.String) + len(ansi.Reset)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg) + len(l.palette.Message) + len(ansi.Reset)
	}
	if n := len(keyvals); n > 0 {
		estimate += n*8 + n*(len(l.palette.Key)+len(ansi.Reset))
	}
	lw.reserve(estimate)
	lw.writeByte('{')
	first := true
	writeColoredJSONStringField(lw, &first, l.tsKeyData, timestamp, l.palette.Timestamp, l.base.cfg.timestampTrusted)
	writeColoredJSONStringField(lw, &first, l.lvlKeyData, levelLabel, levelColor, true)
	if msg != "" {
		appendKeyDataWithFirst(lw, &first, l.msgKeyData)
		writeColoredJSONString(lw, msg, l.palette.Message)
	}
	lw.writeBytes(l.basePayload)
	first = false
	writeRuntimeJSONFieldsColor(lw, &first, keyvals, l.palette)
	writeColoredJSONStringField(lw, &first, l.logLevelKey, l.base.cfg.logLevelValue, l.palette.String, true)
	lw.writeByte('}')
}

func emitJSONColorTimestampLogLevelNoStaticFields(l *jsonColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	timestamp := l.base.cfg.timestamp()
	levelColor := colorForLevel(level, l.palette)
	levelLabel := LevelString(level)
	estimate := 2 + len(l.lvlKeyData) + len(levelLabel) +
		len(levelColor) + len(ansi.Reset) +
		len(l.tsKeyData) + len(timestamp) + len(l.palette.Timestamp) + len(ansi.Reset) +
		len(l.logLevelKey) + len(l.base.cfg.logLevelValue) + len(l.palette.String) + len(ansi.Reset)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg) + len(l.palette.Message) + len(ansi.Reset)
	}
	if n := len(keyvals); n > 0 {
		estimate += n*8 + n*(len(l.palette.Key)+len(ansi.Reset))
	}
	lw.reserve(estimate)
	lw.writeByte('{')
	first := true
	writeColoredJSONStringField(lw, &first, l.tsKeyData, timestamp, l.palette.Timestamp, l.base.cfg.timestampTrusted)
	writeColoredJSONStringField(lw, &first, l.lvlKeyData, levelLabel, levelColor, true)
	if msg != "" {
		appendKeyDataWithFirst(lw, &first, l.msgKeyData)
		writeColoredJSONString(lw, msg, l.palette.Message)
	}
	writeRuntimeJSONFieldsColor(lw, &first, keyvals, l.palette)
	writeColoredJSONStringField(lw, &first, l.logLevelKey, l.base.cfg.logLevelValue, l.palette.String, true)
	lw.writeByte('}')
}

func emitJSONColorTimestampWithStaticFields(l *jsonColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	timestamp := l.base.cfg.timestamp()
	levelColor := colorForLevel(level, l.palette)
	levelLabel := LevelString(level)
	estimate := 2 + len(l.basePayload) + len(l.lvlKeyData) + len(levelLabel) +
		len(levelColor) + len(ansi.Reset) +
		len(l.tsKeyData) + len(timestamp) + len(l.palette.Timestamp) + len(ansi.Reset)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg) + len(l.palette.Message) + len(ansi.Reset)
	}
	if n := len(keyvals); n > 0 {
		estimate += n*8 + n*(len(l.palette.Key)+len(ansi.Reset))
	}
	lw.reserve(estimate)
	lw.writeByte('{')
	first := true
	writeColoredJSONStringField(lw, &first, l.tsKeyData, timestamp, l.palette.Timestamp, l.base.cfg.timestampTrusted)
	writeColoredJSONStringField(lw, &first, l.lvlKeyData, levelLabel, levelColor, true)
	if msg != "" {
		appendKeyDataWithFirst(lw, &first, l.msgKeyData)
		writeColoredJSONString(lw, msg, l.palette.Message)
	}
	lw.writeBytes(l.basePayload)
	first = false
	writeRuntimeJSONFieldsColor(lw, &first, keyvals, l.palette)
	lw.writeByte('}')
}

func emitJSONColorTimestampNoStaticFields(l *jsonColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	timestamp := l.base.cfg.timestamp()
	levelColor := colorForLevel(level, l.palette)
	levelLabel := LevelString(level)
	estimate := 2 + len(l.lvlKeyData) + len(levelLabel) +
		len(levelColor) + len(ansi.Reset) +
		len(l.tsKeyData) + len(timestamp) + len(l.palette.Timestamp) + len(ansi.Reset)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg) + len(l.palette.Message) + len(ansi.Reset)
	}
	if n := len(keyvals); n > 0 {
		estimate += n*8 + n*(len(l.palette.Key)+len(ansi.Reset))
	}
	lw.reserve(estimate)
	lw.writeByte('{')
	first := true
	writeColoredJSONStringField(lw, &first, l.tsKeyData, timestamp, l.palette.Timestamp, l.base.cfg.timestampTrusted)
	writeColoredJSONStringField(lw, &first, l.lvlKeyData, levelLabel, levelColor, true)
	if msg != "" {
		appendKeyDataWithFirst(lw, &first, l.msgKeyData)
		writeColoredJSONString(lw, msg, l.palette.Message)
	}
	writeRuntimeJSONFieldsColor(lw, &first, keyvals, l.palette)
	lw.writeByte('}')
}

func emitJSONColorLogLevelWithStaticFields(l *jsonColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	levelColor := colorForLevel(level, l.palette)
	levelLabel := LevelString(level)
	estimate := 2 + len(l.basePayload) + len(l.lvlKeyData) + len(levelLabel) +
		len(levelColor) + len(ansi.Reset) +
		len(l.logLevelKey) + len(l.base.cfg.logLevelValue) + len(l.palette.String) + len(ansi.Reset)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg) + len(l.palette.Message) + len(ansi.Reset)
	}
	if n := len(keyvals); n > 0 {
		estimate += n*8 + n*(len(l.palette.Key)+len(ansi.Reset))
	}
	lw.reserve(estimate)
	lw.writeByte('{')
	first := true
	writeColoredJSONStringField(lw, &first, l.lvlKeyData, levelLabel, levelColor, true)
	if msg != "" {
		appendKeyDataWithFirst(lw, &first, l.msgKeyData)
		writeColoredJSONString(lw, msg, l.palette.Message)
	}
	lw.writeBytes(l.basePayload)
	first = false
	writeRuntimeJSONFieldsColor(lw, &first, keyvals, l.palette)
	writeColoredJSONStringField(lw, &first, l.logLevelKey, l.base.cfg.logLevelValue, l.palette.String, true)
	lw.writeByte('}')
}

func emitJSONColorLogLevelNoStaticFields(l *jsonColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	levelColor := colorForLevel(level, l.palette)
	levelLabel := LevelString(level)
	estimate := 2 + len(l.lvlKeyData) + len(levelLabel) +
		len(levelColor) + len(ansi.Reset) +
		len(l.logLevelKey) + len(l.base.cfg.logLevelValue) + len(l.palette.String) + len(ansi.Reset)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg) + len(l.palette.Message) + len(ansi.Reset)
	}
	if n := len(keyvals); n > 0 {
		estimate += n*8 + n*(len(l.palette.Key)+len(ansi.Reset))
	}
	lw.reserve(estimate)
	lw.writeByte('{')
	first := true
	writeColoredJSONStringField(lw, &first, l.lvlKeyData, levelLabel, levelColor, true)
	if msg != "" {
		appendKeyDataWithFirst(lw, &first, l.msgKeyData)
		writeColoredJSONString(lw, msg, l.palette.Message)
	}
	writeRuntimeJSONFieldsColor(lw, &first, keyvals, l.palette)
	writeColoredJSONStringField(lw, &first, l.logLevelKey, l.base.cfg.logLevelValue, l.palette.String, true)
	lw.writeByte('}')
}

func emitJSONColorBaseWithStaticFields(l *jsonColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	levelColor := colorForLevel(level, l.palette)
	levelLabel := LevelString(level)
	estimate := 2 + len(l.basePayload) + len(l.lvlKeyData) + len(levelLabel) +
		len(levelColor) + len(ansi.Reset)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg) + len(l.palette.Message) + len(ansi.Reset)
	}
	if n := len(keyvals); n > 0 {
		estimate += n*8 + n*(len(l.palette.Key)+len(ansi.Reset))
	}
	lw.reserve(estimate)
	lw.writeByte('{')
	first := true
	writeColoredJSONStringField(lw, &first, l.lvlKeyData, levelLabel, levelColor, true)
	if msg != "" {
		appendKeyDataWithFirst(lw, &first, l.msgKeyData)
		writeColoredJSONString(lw, msg, l.palette.Message)
	}
	lw.writeBytes(l.basePayload)
	first = false
	writeRuntimeJSONFieldsColor(lw, &first, keyvals, l.palette)
	lw.writeByte('}')
}

func emitJSONColorBaseNoStaticFields(l *jsonColorLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	levelColor := colorForLevel(level, l.palette)
	levelLabel := LevelString(level)
	estimate := 2 + len(l.lvlKeyData) + len(levelLabel) +
		len(levelColor) + len(ansi.Reset)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg) + len(l.palette.Message) + len(ansi.Reset)
	}
	if n := len(keyvals); n > 0 {
		estimate += n*8 + n*(len(l.palette.Key)+len(ansi.Reset))
	}
	lw.reserve(estimate)
	lw.writeByte('{')
	first := true
	writeColoredJSONStringField(lw, &first, l.lvlKeyData, levelLabel, levelColor, true)
	if msg != "" {
		appendKeyDataWithFirst(lw, &first, l.msgKeyData)
		writeColoredJSONString(lw, msg, l.palette.Message)
	}
	writeRuntimeJSONFieldsColor(lw, &first, keyvals, l.palette)
	lw.writeByte('}')
}

func writeRuntimeJSONFieldsColor(lw *lineWriter, first *bool, keyvals []any, palette *ansi.Palette) {
	startLen := len(lw.buf)
	startFirst := *first
	if writeRuntimeJSONFieldsColorFast(lw, first, keyvals, palette) {
		return
	}
	lw.buf = lw.buf[:startLen]
	*first = startFirst
	writeRuntimeJSONFieldsColorSlow(lw, first, keyvals, palette)
}

func writeRuntimeJSONFieldsColorFast(lw *lineWriter, first *bool, keyvals []any, palette *ansi.Palette) bool {
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
		writeColoredKey(lw, key, palette.Key, trusted)
		lw.writeByte(':')
		value := keyvals[i+1]
		writeRuntimeJSONValueColor(lw, value, palette)
		pairIndex++
	}
	if n&1 == 1 {
		value := keyvals[n-1]
		if *first {
			*first = false
		} else {
			lw.writeByte(',')
		}
		writeColoredKey(lw, argKeyName(pairIndex), palette.Key, false)
		lw.writeByte(':')
		writeRuntimeJSONValueColor(lw, value, palette)
	}
	return true
}

func writeRuntimeJSONFieldsColorSlow(lw *lineWriter, first *bool, keyvals []any, palette *ansi.Palette) {
	n := len(keyvals)
	if n == 0 {
		return
	}

	pairIndex := 0
	if n >= 2 {
		pairIndex = writeRuntimeJSONPairColor(lw, first, keyvals[0], keyvals[1], pairIndex, palette)
		if n >= 4 {
			pairIndex = writeRuntimeJSONPairColor(lw, first, keyvals[2], keyvals[3], pairIndex, palette)
			for i := 4; i+1 < n; i += 2 {
				pairIndex = writeRuntimeJSONPairColor(lw, first, keyvals[i], keyvals[i+1], pairIndex, palette)
			}
			if n&1 == 1 {
				writeRuntimeJSONOddColor(lw, first, keyvals[n-1], pairIndex, palette)
			}
			return
		}
		if n == 3 {
			writeRuntimeJSONOddColor(lw, first, keyvals[2], pairIndex, palette)
		}
		return
	}

	writeRuntimeJSONOddColor(lw, first, keyvals[0], pairIndex, palette)
}

func writeRuntimeJSONPairColor(lw *lineWriter, first *bool, key any, value any, pairIndex int, palette *ansi.Palette) int {
	k, trusted := runtimeKeyFromValue(key, pairIndex)
	if k != "" {
		if *first {
			*first = false
		} else {
			lw.writeByte(',')
		}
		writeColoredKey(lw, k, palette.Key, trusted)
		lw.writeByte(':')
		writeRuntimeJSONValueColor(lw, value, palette)
	}
	return pairIndex + 1
}

func writeRuntimeJSONOddColor(lw *lineWriter, first *bool, value any, pairIndex int, palette *ansi.Palette) {
	if *first {
		*first = false
	} else {
		lw.writeByte(',')
	}
	writeColoredKey(lw, argKeyName(pairIndex), palette.Key, false)
	lw.writeByte(':')
	writeRuntimeJSONValueColor(lw, value, palette)
}

func writeColoredKey(lw *lineWriter, key string, color string, trusted bool) {
	if trusted {
		writePTJSONStringTrustedColored(lw, color, key)
		return
	}
	writePTJSONStringColored(lw, color, key)
}

func encodeBaseJSONColor(fields []field, palette *ansi.Palette, floatPolicy NonFiniteFloatPolicy) []byte {
	if len(fields) == 0 {
		return nil
	}
	lw := acquireLineWriter(io.Discard)
	lw.autoFlush = false
	lw.floatPolicy = floatPolicy
	for _, f := range fields {
		if f.key == "" {
			continue
		}
		lw.writeByte(',')
		writeColoredKey(lw, f.key, palette.Key, f.trustedKey)
		lw.writeByte(':')
		writeRuntimeJSONValueColor(lw, f.value, palette)
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

func writeRuntimeJSONValueColor(lw *lineWriter, value any, palette *ansi.Palette) {
	switch v := value.(type) {
	case TrustedString:
		writePTJSONStringTrustedColored(lw, palette.String, string(v))
	case string:
		writePTJSONStringColored(lw, palette.String, v)
	case bool:
		writeJSONBoolColored(lw, v, palette.Bool)
	case int:
		writeJSONNumberColored(lw, int64(v), palette.Num)
	case int8:
		writeJSONNumberColored(lw, int64(v), palette.Num)
	case int16:
		writeJSONNumberColored(lw, int64(v), palette.Num)
	case int32:
		writeJSONNumberColored(lw, int64(v), palette.Num)
	case int64:
		writeJSONNumberColored(lw, v, palette.Num)
	case uint:
		writeJSONUintColored(lw, uint64(v), palette.Num)
	case uint8:
		writeJSONUintColored(lw, uint64(v), palette.Num)
	case uint16:
		writeJSONUintColored(lw, uint64(v), palette.Num)
	case uint32:
		writeJSONUintColored(lw, uint64(v), palette.Num)
	case uint64:
		writeJSONUintColored(lw, v, palette.Num)
	case uintptr:
		writeJSONUintColored(lw, uint64(v), palette.Num)
	case float32:
		writeJSONFloatColored(lw, float64(v), palette.Num)
	case float64:
		writeJSONFloatColored(lw, v, palette.Num)
	case []byte:
		s := string(v)
		if stringTrustedASCII(s) {
			writePTJSONStringTrustedColored(lw, palette.String, s)
		} else {
			writePTJSONStringColored(lw, palette.String, s)
		}
	case time.Time:
		writePTJSONStringTrustedColored(lw, palette.Timestamp, lw.formatTimeRFC3339(v))
	case time.Duration:
		writePTJSONStringTrustedColored(lw, palette.String, lw.formatDuration(v))
	case stringer:
		s := v.String()
		color := palette.String
		if _, isError := value.(error); isError {
			color = palette.Error
		}
		if stringTrustedASCII(s) {
			writePTJSONStringTrustedColored(lw, color, s)
		} else {
			writePTJSONStringColored(lw, color, s)
		}
	case error:
		s := v.Error()
		if stringTrustedASCII(s) {
			writePTJSONStringTrustedColored(lw, palette.Error, s)
		} else {
			writePTJSONStringColored(lw, palette.Error, s)
		}
	case nil:
		if palette.Nil == "" {
			lw.writeString("null")
			return
		}
		lw.reserve(len(palette.Nil) + len("null") + len(ansi.Reset))
		lw.buf = append(lw.buf, palette.Nil...)
		lw.buf = append(lw.buf, 'n', 'u', 'l', 'l')
		lw.buf = append(lw.buf, ansi.Reset...)
		lw.maybeFlush()
	default:
		// Preserve previous behaviour for unknown/runtime-marshal values: no color.
		writePTLogValue(lw, value)
	}
}

func colorForLevel(level Level, palette *ansi.Palette) string {
	switch level {
	case TraceLevel:
		return palette.Trace
	case DebugLevel:
		return palette.Debug
	case InfoLevel:
		return palette.Info
	case WarnLevel:
		return palette.Warn
	case ErrorLevel:
		return palette.Error
	case FatalLevel, PanicLevel:
		return palette.Fatal
	case NoLevel:
		return palette.NoLevel
	default:
		return palette.Info
	}
}
