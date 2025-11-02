package main

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"

	plog "github.com/phuslu/log"
	"github.com/rs/zerolog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/sa6mwa/pslog/benchmark"
	pslog "pkt.systems/pslog"
)

type productionEntry struct {
	level   pslog.Level
	message string
	keyvals []any
	fields  []productionKV
	zap     []zap.Field
}

type productionKV struct {
	key   string
	value any
}

func (e productionEntry) logPslog(logger pslog.Logger) {
	logger.Log(e.level, e.message, e.keyvals...)
}

func (e productionEntry) toMap() map[string]any {
	if len(e.fields) == 0 {
		return nil
	}
	m := make(map[string]any, len(e.fields))
	for _, f := range e.fields {
		m[f.key] = f.value
	}
	return m
}

func (e productionEntry) applyZerolog(ev *zerolog.Event) *zerolog.Event {
	for _, field := range e.fields {
		switch v := field.value.(type) {
		case string:
			ev.Str(field.key, v)
		case pslog.TrustedString:
			ev.Str(field.key, string(v))
		case bool:
			ev.Bool(field.key, v)
		case int:
			ev.Int(field.key, v)
		case int64:
			ev.Int64(field.key, v)
		case uint64:
			ev.Uint64(field.key, v)
		case float64:
			ev.Float64(field.key, v)
		case []byte:
			ev.Bytes(field.key, v)
		default:
			ev.Interface(field.key, v)
		}
	}
	return ev
}

func (e productionEntry) applyPhuslu(entry *plog.Entry) {
	for _, field := range e.fields {
		switch v := field.value.(type) {
		case string:
			entry.Str(field.key, v)
		case pslog.TrustedString:
			entry.Str(field.key, string(v))
		case bool:
			entry.Bool(field.key, v)
		case int:
			entry.Int(field.key, v)
		case int64:
			entry.Int64(field.key, v)
		case uint64:
			entry.Uint64(field.key, v)
		case float64:
			entry.Float64(field.key, v)
		case []byte:
			entry.Bytes(field.key, v)
		default:
			entry.Any(field.key, v)
		}
	}
}

func (e productionEntry) zapFieldsSlice() []zap.Field {
	return e.zap
}

func loadProductionEntries(limit int) ([]productionEntry, error) {
	dataset := benchmark.EmbeddedProductionDataset
	if len(dataset) == 0 {
		return nil, errors.New("embedded production dataset empty")
	}
	if limit <= 0 || limit > len(dataset) {
		limit = len(dataset)
	}
	entries := make([]productionEntry, 0, limit)
	for i := 0; i < limit; i++ {
		line := dataset[i]
		entry, err := parseProductionLine(line)
		if err != nil {
			continue
		}
		if entry.level == pslog.Disabled {
			continue
		}
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return nil, errors.New("no production log entries parsed")
	}
	return entries, nil
}

func parseProductionLine(line string) (productionEntry, error) {
	decoder := json.NewDecoder(strings.NewReader(line))
	decoder.UseNumber()
	raw := make(map[string]any)
	if err := decoder.Decode(&raw); err != nil {
		return productionEntry{}, err
	}

	level := pslog.InfoLevel
	if lvl, ok := raw["lvl"].(string); ok {
		if parsed, ok := pslog.ParseLevel(lvl); ok {
			level = parsed
		}
	}
	delete(raw, "lvl")

	message := ""
	if msg, ok := raw["msg"].(string); ok {
		message = msg
	}
	delete(raw, "msg")
	delete(raw, "ts")
	delete(raw, "ts_iso")
	delete(raw, "time")

	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	keyvals := make([]any, 0, len(keys)*2)
	fields := make([]productionKV, 0, len(keys))
	zapFields := make([]zap.Field, 0, len(keys))
	for _, k := range keys {
		value := sanitizeJSONValue(raw[k])
		keyvals = append(keyvals, k, value)
		fields = append(fields, productionKV{key: k, value: value})
		zapFields = append(zapFields, zapFieldFromValue(k, value))
	}

	return productionEntry{
		level:   level,
		message: message,
		keyvals: keyvals,
		fields:  fields,
		zap:     zapFields,
	}, nil
}

func normalizeStaticValue(value any) any {
	switch v := value.(type) {
	case string:
		if trusted, ok := pslog.NewTrustedString(v); ok {
			return trusted
		}
		return v
	case pslog.TrustedString:
		return v
	default:
		return value
	}
}

func productionStaticWithArgs(entries []productionEntry) ([]any, map[string]struct{}) {
	if len(entries) == 0 {
		return nil, nil
	}
	constants := entries[0].toMap()
	for k := range constants {
		constants[k] = normalizeStaticValue(constants[k])
	}
	for _, entry := range entries[1:] {
		entryMap := entry.toMap()
		for key, value := range constants {
			other, ok := entryMap[key]
			if !ok {
				delete(constants, key)
				continue
			}
			if !reflect.DeepEqual(normalizeStaticValue(other), value) {
				delete(constants, key)
			}
		}
		if len(constants) == 0 {
			break
		}
	}
	if len(constants) == 0 {
		return nil, nil
	}
	keys := make([]string, 0, len(constants))
	for k := range constants {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	withArgs := make([]any, 0, len(keys)*2)
	staticKeys := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		staticKeys[key] = struct{}{}
		withArgs = append(withArgs, key, constants[key])
	}
	if len(withArgs) > 0 {
		withArgs = pslog.Keyvals(withArgs...)
	}
	return withArgs, staticKeys
}

func productionEntriesWithout(entries []productionEntry, staticKeys map[string]struct{}) []productionEntry {
	if len(staticKeys) == 0 {
		return entries
	}
	filtered := make([]productionEntry, len(entries))
	for i, entry := range entries {
		filteredEntry := entry
		filteredEntry.keyvals = filterKeyvals(entry.keyvals, staticKeys)
		filtered[i] = filteredEntry
	}
	return filtered
}

func filterKeyvals(keyvals []any, staticKeys map[string]struct{}) []any {
	if len(keyvals) == 0 {
		return nil
	}
	out := make([]any, 0, len(keyvals))
	for i := 0; i < len(keyvals); {
		if i+1 >= len(keyvals) {
			out = append(out, keyvals[i:]...)
			break
		}
		key := keyvals[i]
		value := keyvals[i+1]
		keyStr := ""
		switch v := key.(type) {
		case string:
			keyStr = v
		case pslog.TrustedString:
			keyStr = string(v)
		}
		if keyStr != "" {
			if _, exists := staticKeys[keyStr]; exists {
				i += 2
				continue
			}
		}
		out = append(out, key, value)
		i += 2
	}
	return out
}

func sanitizeJSONValue(v any) any {
	switch val := v.(type) {
	case json.Number:
		s := val.String()
		if !strings.ContainsAny(s, ".eE") {
			if i, err := val.Int64(); err == nil {
				return i
			}
		}
		if f, err := val.Float64(); err == nil {
			return f
		}
		return s
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, vv := range val {
			out[k] = sanitizeJSONValue(vv)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, vv := range val {
			out[i] = sanitizeJSONValue(vv)
		}
		return out
	case string:
		return val
	default:
		return val
	}
}

func zapFieldFromValue(key string, value any) zap.Field {
	switch v := value.(type) {
	case string:
		return zap.String(key, v)
	case pslog.TrustedString:
		return zap.String(key, string(v))
	case bool:
		return zap.Bool(key, v)
	case int:
		return zap.Int(key, v)
	case int64:
		return zap.Int64(key, v)
	case uint64:
		return zap.Uint64(key, v)
	case float64:
		return zap.Float64(key, v)
	case []byte:
		return zap.ByteString(key, v)
	default:
		return zap.Any(key, v)
	}
}

func zerologLevelFromPSLog(level pslog.Level) zerolog.Level {
	switch level {
	case pslog.TraceLevel:
		return zerolog.TraceLevel
	case pslog.DebugLevel:
		return zerolog.DebugLevel
	case pslog.InfoLevel, pslog.NoLevel:
		return zerolog.InfoLevel
	case pslog.WarnLevel:
		return zerolog.WarnLevel
	case pslog.ErrorLevel, pslog.FatalLevel, pslog.PanicLevel:
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}

func zapLevelFromPSLog(level pslog.Level) zapcore.Level {
	switch level {
	case pslog.TraceLevel, pslog.DebugLevel:
		return zapcore.DebugLevel
	case pslog.InfoLevel, pslog.NoLevel:
		return zapcore.InfoLevel
	case pslog.WarnLevel:
		return zapcore.WarnLevel
	case pslog.ErrorLevel, pslog.FatalLevel, pslog.PanicLevel:
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

func phusluEntryForLevel(logger *plog.Logger, level pslog.Level) *plog.Entry {
	switch level {
	case pslog.TraceLevel:
		return logger.Trace()
	case pslog.DebugLevel:
		return logger.Debug()
	case pslog.InfoLevel, pslog.NoLevel:
		return logger.Info()
	case pslog.WarnLevel:
		return logger.Warn()
	case pslog.ErrorLevel, pslog.FatalLevel, pslog.PanicLevel:
		return logger.Error()
	default:
		return logger.Info()
	}
}
