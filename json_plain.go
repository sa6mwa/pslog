package pslog

import (
	"io"
	"sync/atomic"
)

type jsonPlainEmitFunc func(l *jsonPlainLogger, lw *lineWriter, level Level, msg string, keyvals []any)

type jsonPlainLogger struct {
	base           loggerBase
	tsKeyData      []byte
	lvlKeyData     []byte
	msgKeyData     []byte
	basePayload    []byte
	hasBasePayload bool
	logLevelKey    []byte
	lineHint       *atomic.Int64
	verboseField   bool
	emit           jsonPlainEmitFunc
}

func appendKeyDataWithFirst(lw *lineWriter, first *bool, keyData []byte) {
	if *first {
		*first = false
		if len(keyData) > 0 && keyData[0] == ',' {
			lw.buf = append(lw.buf, keyData[1:]...)
			return
		}
	}
	lw.buf = append(lw.buf, keyData...)
}

func writeJSONStringField(lw *lineWriter, first *bool, keyData []byte, value string, trusted bool) {
	appendKeyDataWithFirst(lw, first, keyData)
	if trusted {
		writePTJSONStringTrusted(lw, value)
		return
	}
	writePTJSONString(lw, value)
}

func newJSONPlainLogger(cfg coreConfig, opts Options) *jsonPlainLogger {
	tsKey := "ts"
	lvlKey := "lvl"
	msgKey := "msg"
	if opts.VerboseFields {
		tsKey = "time"
		lvlKey = "level"
		msgKey = "message"
	}
	configureJSONEscapeFromOptions(opts)
	logger := &jsonPlainLogger{
		base:         newLoggerBase(cfg, nil),
		tsKeyData:    makeKeyData(tsKey, false),
		lvlKeyData:   makeKeyData(lvlKey, true),
		msgKeyData:   makeKeyData(msgKey, true),
		logLevelKey:  makeKeyData("loglevel", true),
		verboseField: opts.VerboseFields,
		lineHint:     new(atomic.Int64),
	}
	logger.rebuildBasePayload()
	return logger
}

func (l *jsonPlainLogger) Trace(msg string, keyvals ...any) { l.log(TraceLevel, msg, keyvals...) }
func (l *jsonPlainLogger) Debug(msg string, keyvals ...any) { l.log(DebugLevel, msg, keyvals...) }
func (l *jsonPlainLogger) Info(msg string, keyvals ...any)  { l.log(InfoLevel, msg, keyvals...) }
func (l *jsonPlainLogger) Warn(msg string, keyvals ...any)  { l.log(WarnLevel, msg, keyvals...) }
func (l *jsonPlainLogger) Error(msg string, keyvals ...any) { l.log(ErrorLevel, msg, keyvals...) }

func (l *jsonPlainLogger) Fatal(msg string, keyvals ...any) {
	l.log(FatalLevel, msg, keyvals...)
	exitProcess()
}

func (l *jsonPlainLogger) Panic(msg string, keyvals ...any) {
	l.log(PanicLevel, msg, keyvals...)
	panic(msg)
}

func (l *jsonPlainLogger) Log(level Level, msg string, keyvals ...any) {
	l.log(level, msg, keyvals...)
}

func (l *jsonPlainLogger) log(level Level, msg string, keyvals ...any) {
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

func (l *jsonPlainLogger) With(keyvals ...any) Logger {
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

func (l *jsonPlainLogger) WithLogLevel() Logger {
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

func (l *jsonPlainLogger) LogLevel(level Level) Logger {
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

func (l *jsonPlainLogger) LogLevelFromEnv(key string) Logger {
	if level, ok := LevelFromEnv(key); ok {
		return l.LogLevel(level)
	}
	return l
}

func (l *jsonPlainLogger) Close() error {
	return closeOutput(l.base.cfg.writer)
}

func (l *jsonPlainLogger) rebuildBasePayload() {
	l.basePayload = encodeBaseJSONPlain(l.base.fields)
	l.hasBasePayload = len(l.basePayload) > 0
	if l.base.cfg.includeLogLevel {
		l.base.cfg.logLevelValue = LevelString(l.base.cfg.currentLevel())
	}
	l.emit = selectJSONPlainEmit(l.base.cfg, l.hasBasePayload)
}

func selectJSONPlainEmit(cfg coreConfig, hasStaticFields bool) jsonPlainEmitFunc {
	switch {
	case cfg.includeTimestamp && cfg.includeLogLevel:
		if hasStaticFields {
			return emitJSONPlainTimestampLogLevelWithStaticFields
		}
		return emitJSONPlainTimestampLogLevelNoStaticFields
	case cfg.includeTimestamp:
		if hasStaticFields {
			return emitJSONPlainTimestampWithStaticFields
		}
		return emitJSONPlainTimestampNoStaticFields
	case cfg.includeLogLevel:
		if hasStaticFields {
			return emitJSONPlainLogLevelWithStaticFields
		}
		return emitJSONPlainLogLevelNoStaticFields
	default:
		if hasStaticFields {
			return emitJSONPlainBaseWithStaticFields
		}
		return emitJSONPlainBaseNoStaticFields
	}
}

func emitJSONPlainTimestampLogLevelWithStaticFields(l *jsonPlainLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	timestamp := l.base.cfg.timestamp()
	levelLabel := LevelString(level)
	estimate := 2 + len(l.basePayload) + len(keyvals)*8 +
		len(l.tsKeyData) + len(timestamp) +
		len(l.lvlKeyData) + len(levelLabel) +
		len(l.logLevelKey) + len(l.base.cfg.logLevelValue)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg)
	}
	lw.reserve(estimate)
	lw.writeByte('{')
	first := true
	writeJSONStringField(lw, &first, l.tsKeyData, timestamp, l.base.cfg.timestampTrusted)
	writeJSONStringField(lw, &first, l.lvlKeyData, levelLabel, true)
	if msg != "" {
		writeJSONStringField(lw, &first, l.msgKeyData, msg, false)
	}
	lw.writeBytes(l.basePayload)
	first = false
	writeRuntimeJSONFieldsPlain(lw, &first, keyvals)
	writeJSONStringField(lw, &first, l.logLevelKey, l.base.cfg.logLevelValue, true)
	lw.writeByte('}')
}

func emitJSONPlainTimestampLogLevelNoStaticFields(l *jsonPlainLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	timestamp := l.base.cfg.timestamp()
	levelLabel := LevelString(level)
	estimate := 2 + len(keyvals)*8 +
		len(l.tsKeyData) + len(timestamp) +
		len(l.lvlKeyData) + len(levelLabel) +
		len(l.logLevelKey) + len(l.base.cfg.logLevelValue)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg)
	}
	lw.reserve(estimate)
	lw.writeByte('{')
	first := true
	writeJSONStringField(lw, &first, l.tsKeyData, timestamp, l.base.cfg.timestampTrusted)
	writeJSONStringField(lw, &first, l.lvlKeyData, levelLabel, true)
	if msg != "" {
		writeJSONStringField(lw, &first, l.msgKeyData, msg, false)
	}
	writeRuntimeJSONFieldsPlain(lw, &first, keyvals)
	writeJSONStringField(lw, &first, l.logLevelKey, l.base.cfg.logLevelValue, true)
	lw.writeByte('}')
}

func emitJSONPlainTimestampWithStaticFields(l *jsonPlainLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	timestamp := l.base.cfg.timestamp()
	levelLabel := LevelString(level)
	estimate := 2 + len(l.basePayload) + len(keyvals)*8 +
		len(l.tsKeyData) + len(timestamp) +
		len(l.lvlKeyData) + len(levelLabel)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg)
	}
	lw.reserve(estimate)
	lw.writeByte('{')
	first := true
	writeJSONStringField(lw, &first, l.tsKeyData, timestamp, l.base.cfg.timestampTrusted)
	writeJSONStringField(lw, &first, l.lvlKeyData, levelLabel, true)
	if msg != "" {
		writeJSONStringField(lw, &first, l.msgKeyData, msg, false)
	}
	lw.writeBytes(l.basePayload)
	first = false
	writeRuntimeJSONFieldsPlain(lw, &first, keyvals)
	lw.writeByte('}')
}

func emitJSONPlainTimestampNoStaticFields(l *jsonPlainLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	timestamp := l.base.cfg.timestamp()
	levelLabel := LevelString(level)
	estimate := 2 + len(keyvals)*8 +
		len(l.tsKeyData) + len(timestamp) +
		len(l.lvlKeyData) + len(levelLabel)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg)
	}
	lw.reserve(estimate)
	lw.writeByte('{')
	first := true
	writeJSONStringField(lw, &first, l.tsKeyData, timestamp, l.base.cfg.timestampTrusted)
	writeJSONStringField(lw, &first, l.lvlKeyData, levelLabel, true)
	if msg != "" {
		writeJSONStringField(lw, &first, l.msgKeyData, msg, false)
	}
	writeRuntimeJSONFieldsPlain(lw, &first, keyvals)
	lw.writeByte('}')
}

func emitJSONPlainLogLevelWithStaticFields(l *jsonPlainLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	levelLabel := LevelString(level)
	estimate := 2 + len(l.basePayload) + len(keyvals)*8 +
		len(l.lvlKeyData) + len(levelLabel) +
		len(l.logLevelKey) + len(l.base.cfg.logLevelValue)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg)
	}
	lw.reserve(estimate)
	lw.writeByte('{')
	first := true
	writeJSONStringField(lw, &first, l.lvlKeyData, levelLabel, true)
	if msg != "" {
		writeJSONStringField(lw, &first, l.msgKeyData, msg, false)
	}
	lw.writeBytes(l.basePayload)
	first = false
	writeRuntimeJSONFieldsPlain(lw, &first, keyvals)
	writeJSONStringField(lw, &first, l.logLevelKey, l.base.cfg.logLevelValue, true)
	lw.writeByte('}')
}

func emitJSONPlainLogLevelNoStaticFields(l *jsonPlainLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	levelLabel := LevelString(level)
	estimate := 2 + len(keyvals)*8 +
		len(l.lvlKeyData) + len(levelLabel) +
		len(l.logLevelKey) + len(l.base.cfg.logLevelValue)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg)
	}
	lw.reserve(estimate)
	lw.writeByte('{')
	first := true
	writeJSONStringField(lw, &first, l.lvlKeyData, levelLabel, true)
	if msg != "" {
		writeJSONStringField(lw, &first, l.msgKeyData, msg, false)
	}
	writeRuntimeJSONFieldsPlain(lw, &first, keyvals)
	writeJSONStringField(lw, &first, l.logLevelKey, l.base.cfg.logLevelValue, true)
	lw.writeByte('}')
}

func emitJSONPlainBaseWithStaticFields(l *jsonPlainLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	levelLabel := LevelString(level)
	estimate := 2 + len(l.basePayload) + len(keyvals)*8 +
		len(l.lvlKeyData) + len(levelLabel)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg)
	}
	lw.reserve(estimate)
	lw.writeByte('{')
	first := true
	writeJSONStringField(lw, &first, l.lvlKeyData, levelLabel, true)
	if msg != "" {
		writeJSONStringField(lw, &first, l.msgKeyData, msg, false)
	}
	lw.writeBytes(l.basePayload)
	first = false
	writeRuntimeJSONFieldsPlain(lw, &first, keyvals)
	lw.writeByte('}')
}

func emitJSONPlainBaseNoStaticFields(l *jsonPlainLogger, lw *lineWriter, level Level, msg string, keyvals []any) {
	levelLabel := LevelString(level)
	estimate := 2 + len(keyvals)*8 +
		len(l.lvlKeyData) + len(levelLabel)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg)
	}
	lw.reserve(estimate)
	lw.writeByte('{')
	first := true
	writeJSONStringField(lw, &first, l.lvlKeyData, levelLabel, true)
	if msg != "" {
		writeJSONStringField(lw, &first, l.msgKeyData, msg, false)
	}
	writeRuntimeJSONFieldsPlain(lw, &first, keyvals)
	lw.writeByte('}')
}

func encodeBaseJSONPlain(fields []field) []byte {
	if len(fields) == 0 {
		return nil
	}
	var buf []byte
	for _, f := range fields {
		if f.key == "" {
			continue
		}
		if buf == nil {
			buf = make([]byte, 0, len(fields)*24)
		}
		buf = append(buf, ',')
		if f.trustedKey {
			buf = append(buf, '"')
			buf = append(buf, f.key...)
			buf = append(buf, '"', ':')
		} else {
			buf = appendEscapedKey(buf, f.key)
		}
		buf = appendJSONValuePlain(buf, f.value)
	}
	return buf
}

func writeRuntimeJSONFieldsPlain(lw *lineWriter, first *bool, keyvals []any) {
	if writeRuntimeJSONFieldsPlainFast(lw, first, keyvals) {
		return
	}
	writeRuntimeJSONFieldsPlainSlow(lw, first, keyvals)
}

func writeRuntimeJSONFieldsPlainFast(lw *lineWriter, first *bool, keyvals []any) bool {
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
			lw.buf = append(lw.buf, ',')
		}
		if trusted {
			lw.reserve(len(key) + 3)
			lw.buf = append(lw.buf, '"')
			lw.buf = append(lw.buf, key...)
			lw.buf = append(lw.buf, '"', ':')
		} else {
			lw.reserve(len(key)*6 + 3)
			lw.buf = append(lw.buf, '"')
			appendEscapedStringContent(lw, key)
			lw.buf = append(lw.buf, '"', ':')
		}
		value := keyvals[i+1]
		if !writeRuntimeValuePlainInline(lw, value) {
			if !writeRuntimeValuePlain(lw, value) {
				writePTLogValue(lw, value)
			}
		}
		pairIndex++
	}
	if n&1 == 1 {
		writeRuntimeJSONOddPlain(lw, first, keyvals[n-1], pairIndex)
	}
	return true
}

func writeRuntimeJSONFieldsPlainSlow(lw *lineWriter, first *bool, keyvals []any) {
	n := len(keyvals)
	if n == 0 {
		return
	}

	pairIndex := 0
	if n >= 2 {
		pairIndex = writeRuntimeJSONPairPlain(lw, first, keyvals[0], keyvals[1], pairIndex)
		if n >= 4 {
			pairIndex = writeRuntimeJSONPairPlain(lw, first, keyvals[2], keyvals[3], pairIndex)
			for i := 4; i+1 < n; i += 2 {
				pairIndex = writeRuntimeJSONPairPlain(lw, first, keyvals[i], keyvals[i+1], pairIndex)
			}
			if n&1 == 1 {
				writeRuntimeJSONOddPlain(lw, first, keyvals[n-1], pairIndex)
			}
			return
		}
		if n == 3 {
			writeRuntimeJSONOddPlain(lw, first, keyvals[2], pairIndex)
		}
		return
	}

	writeRuntimeJSONOddPlain(lw, first, keyvals[0], pairIndex)
}

func writeRuntimeJSONPairPlain(lw *lineWriter, first *bool, key any, value any, pairIndex int) int {
	k, trusted := runtimeKeyFromValue(key, pairIndex)
	if k != "" {
		if *first {
			*first = false
		} else {
			lw.buf = append(lw.buf, ',')
		}
		if trusted {
			writeTrustedKeyColon(lw, k)
		} else {
			writePTJSONStringWithColon(lw, k)
		}
		if !writeRuntimeValuePlain(lw, value) {
			writePTLogValue(lw, value)
		}
	}
	return pairIndex + 1
}

func writeRuntimeJSONOddPlain(lw *lineWriter, first *bool, value any, pairIndex int) {
	if *first {
		*first = false
	} else {
		lw.buf = append(lw.buf, ',')
	}
	writePTJSONStringWithColon(lw, argKeyName(pairIndex))
	if !writeRuntimeValuePlainInline(lw, value) {
		if !writeRuntimeValuePlain(lw, value) {
			writePTLogValue(lw, value)
		}
	}
}

func appendJSONValuePlain(buf []byte, value any) []byte {
	lw := acquireLineWriter(io.Discard)
	lw.autoFlush = false
	writePTLogValue(lw, value)
	buf = append(buf, lw.buf...)
	releaseLineWriter(lw)
	return buf
}

func appendEscapedKey(dst []byte, key string) []byte {
	dst = append(dst, '"')
	start := 0
	const hex = "0123456789abcdef"
	for i := 0; i < len(key); i++ {
		if !jsonNeedsEscape[key[i]] {
			continue
		}
		if start < i {
			dst = append(dst, key[start:i]...)
		}
		switch c := key[i]; c {
		case '\\', '"':
			dst = append(dst, '\\', c)
		case '\b':
			dst = append(dst, '\\', 'b')
		case '\f':
			dst = append(dst, '\\', 'f')
		case '\n':
			dst = append(dst, '\\', 'n')
		case '\r':
			dst = append(dst, '\\', 'r')
		case '\t':
			dst = append(dst, '\\', 't')
		default:
			dst = append(dst, '\\', 'u', '0', '0', hex[c>>4], hex[c&0x0f])
		}
		start = i + 1
	}
	if start < len(key) {
		dst = append(dst, key[start:]...)
	}
	dst = append(dst, '"', ':')
	return dst
}
