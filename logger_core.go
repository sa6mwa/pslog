package pslog

import (
	"io"
	"time"
)

type field struct {
	key        string
	value      any
	trustedKey bool
}

func collectFields(keyvals []any) []field {
	if len(keyvals) == 0 {
		return nil
	}
	fields := make([]field, 0, (len(keyvals)+1)/2)
	pair := 0
	for i := 0; i < len(keyvals); {
		if i+1 < len(keyvals) {
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
				key = keyFromValue(keyvals[i], pair)
				if key != "" {
					trusted = stringTrustedASCII(key)
				}
			}
			fields = append(fields, field{key: key, value: keyvals[i+1], trustedKey: trusted})
			i += 2
			pair++
			continue
		}
		key := argKeyName(pair)
		fields = append(fields, field{key: key, value: keyvals[i], trustedKey: stringTrustedASCII(key)})
		i++
		pair++
	}
	return fields
}

func keyFromValue(v any, pair int) string {
	if v == nil {
		return argKeyName(pair)
	}
	if key, ok := v.(string); ok {
		return key
	}
	if key, ok := v.(TrustedString); ok {
		return string(key)
	}
	if key, ok := keyFromAny(v); ok && key != "" {
		return key
	}
	return stringFromAny(v)
}

func cloneFields(src []field) []field {
	if len(src) == 0 {
		return nil
	}
	dst := make([]field, len(src))
	copy(dst, src)
	return dst
}

type coreConfig struct {
	writer           io.Writer
	minLevel         Level
	forcedLevel      *Level
	includeLogLevel  bool
	logLevelValue    string
	includeTimestamp bool
	timeLayout       string
	useUTC           bool
	timeCache        *timeCache
	timeFormatter    func(time.Time) string
	timestampTrusted bool
}

func (c coreConfig) clone() coreConfig {
	clone := c
	if c.forcedLevel != nil {
		value := *c.forcedLevel
		clone.forcedLevel = &value
	}
	return clone
}

func (c coreConfig) shouldLog(level Level) bool {
	if c.writer == nil {
		return false
	}
	effective := level
	if c.forcedLevel != nil {
		switch *c.forcedLevel {
		case Disabled:
			return false
		case NoLevel:
			effective = InfoLevel
		default:
			effective = *c.forcedLevel
		}
	}
	if effective == Disabled {
		return false
	}
	return effective >= c.minLevel
}

func (c coreConfig) currentLevel() Level {
	if c.forcedLevel != nil {
		return *c.forcedLevel
	}
	return c.minLevel
}

func (c coreConfig) timestamp() string {
	if !c.includeTimestamp {
		return ""
	}
	if c.timeCache != nil {
		return c.timeCache.Current()
	}
	now := time.Now()
	if c.useUTC {
		now = now.UTC()
	}
	if c.timeFormatter != nil {
		return c.timeFormatter(now)
	}
	return now.Format(c.timeLayout)
}

type loggerBase struct {
	cfg    coreConfig
	fields []field
}

func newLoggerBase(cfg coreConfig, fields []field) loggerBase {
	return loggerBase{cfg: cfg, fields: fields}
}

func (b loggerBase) clone() loggerBase {
	return loggerBase{
		cfg:    b.cfg.clone(),
		fields: cloneFields(b.fields),
	}
}

func (b *loggerBase) withFields(additional []field) {
	if len(additional) == 0 {
		return
	}
	if len(b.fields) == 0 {
		b.fields = cloneFields(additional)
		return
	}
	fields := make([]field, 0, len(b.fields)+len(additional))
	fields = append(fields, b.fields...)
	fields = append(fields, additional...)
	b.fields = fields
}

func (b *loggerBase) withLogLevelField() {
	if b.cfg.includeLogLevel {
		return
	}
	b.cfg.includeLogLevel = true
	b.cfg.logLevelValue = LevelString(b.cfg.currentLevel())
}

func (b *loggerBase) withMinLevel(level Level) {
	if level == NoLevel {
		value := level
		b.cfg.forcedLevel = &value
		b.cfg.logLevelValue = LevelString(b.cfg.currentLevel())
		return
	}
	b.cfg.minLevel = level
	b.cfg.forcedLevel = nil
	b.cfg.logLevelValue = LevelString(b.cfg.currentLevel())
}

func (b *loggerBase) withForcedLevel(level Level) {
	value := level
	b.cfg.forcedLevel = &value
	b.cfg.logLevelValue = LevelString(b.cfg.currentLevel())
}
