package pslog

import (
	"io"
	"sync/atomic"
)

type jsonPlainLogger struct {
	base         loggerBase
	tsKeyData    []byte
	lvlKeyData   []byte
	msgKeyData   []byte
	basePayload  []byte
	logLevelKey  []byte
	lineHint     *atomic.Int64
	verboseField bool
}

func newJSONPlainLogger(cfg coreConfig, verbose bool) *jsonPlainLogger {
	tsKey := "ts"
	lvlKey := "lvl"
	msgKey := "msg"
	if verbose {
		tsKey = "time"
		lvlKey = "level"
		msgKey = "message"
	}
	logger := &jsonPlainLogger{
		base:         newLoggerBase(cfg, nil),
		tsKeyData:    makeKeyData(tsKey, false),
		lvlKeyData:   makeKeyData(lvlKey, true),
		msgKeyData:   makeKeyData(msgKey, true),
		logLevelKey:  makeKeyData("loglevel", true),
		verboseField: verbose,
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
	msgTrusted := msg == "" || promoteTrustedValueString(msg)
	l.writeLine(lw, level, msg, msgTrusted, timestamp, keyvals)
	lw.finishLine()
	lw.commit()
	if l.lineHint != nil {
		l.lineHint.Store(int64(lw.lastLineLength()))
	}
	releaseLineWriter(lw)
}

func (l *jsonPlainLogger) writeLine(lw *lineWriter, level Level, msg string, msgTrusted bool, timestamp string, keyvals []any) {
	levelLabel := LevelString(level)
	estimate := 2 + len(l.basePayload) + len(keyvals)*8 + len(l.lvlKeyData) + len(levelLabel)
	if msg != "" {
		estimate += len(l.msgKeyData) + len(msg)
	}
	if l.base.cfg.includeTimestamp {
		estimate += len(l.tsKeyData) + len(timestamp)
	}
	if l.base.cfg.includeLogLevel {
		estimate += len(l.logLevelKey) + len(l.base.cfg.logLevelValue)
	}
	lw.reserve(estimate)

	lw.writeByte('{')
	first := true
	if l.base.cfg.includeTimestamp {
		lw.buf = appendKeyData(lw.buf, l.tsKeyData, first)
		writePTJSONStringTrusted(lw, timestamp)
		first = false
	}
	lw.buf = appendKeyData(lw.buf, l.lvlKeyData, first)
	writePTJSONStringTrusted(lw, levelLabel)
	if msg != "" {
		lw.buf = appendKeyData(lw.buf, l.msgKeyData, false)
		writePTJSONStringMaybeTrusted(lw, msg, msgTrusted)
	}
	if len(l.basePayload) > 0 {
		lw.writeBytes(l.basePayload)
	}
	writeRuntimeJSONFieldsPlain(lw, keyvals)
	if l.base.cfg.includeLogLevel {
		lw.writeBytes(l.logLevelKey)
		writePTJSONStringTrusted(lw, l.base.cfg.logLevelValue)
	}
	lw.writeByte('}')
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

func (l *jsonPlainLogger) rebuildBasePayload() {
	l.basePayload = encodeBaseJSONPlain(l.base.fields)
	if l.base.cfg.includeLogLevel {
		l.base.cfg.logLevelValue = LevelString(l.base.cfg.currentLevel())
	}
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
		if promoteTrustedKey(f.key) {
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

func writeRuntimeJSONFieldsPlain(lw *lineWriter, keyvals []any) {
	if len(keyvals) == 0 {
		return
	}
	first := false
	for i := 0; i+1 < len(keyvals); i += 2 {
		key, keyTrusted := extractKeyFast(keyvals[i])
		if key == "" {
			continue
		}
		if !keyTrusted && promoteTrustedKey(key) {
			keyTrusted = true
		}
		writePTFieldPrefix(lw, &first, key, keyTrusted)
		writePTLogValue(lw, keyvals[i+1])
	}
	if len(keyvals)%2 != 0 {
		writePTFieldPrefix(lw, &first, argKeyName(len(keyvals)/2), false)
		writePTLogValue(lw, keyvals[len(keyvals)-1])
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

func appendKeyData(dst []byte, keyData []byte, first bool) []byte {
	if first && len(keyData) > 0 && keyData[0] == ',' {
		return append(dst, keyData[1:]...)
	}
	return append(dst, keyData...)
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
