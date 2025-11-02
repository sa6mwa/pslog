package benchmark_test

import (
	"errors"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	plog "github.com/phuslu/log"
	"github.com/rs/zerolog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	pslog "pkt.systems/pslog"
)

const productionDatasetLimit = 2048

var (
	productionOnce    sync.Once
	productionEntries []productionEntry
	productionLoadErr error
)

func BenchmarkPSLogProduction(b *testing.B) {
	entries := loadProductionEntries()
	if len(entries) == 0 {
		b.Fatal("no production entries loaded")
	}

	sink := newBenchmarkSink()
	prevFormat := zerolog.TimeFieldFormat
	prevField := zerolog.TimestampFieldName
	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.TimestampFieldName = "ts"
	b.Cleanup(func() {
		zerolog.TimeFieldFormat = prevFormat
		zerolog.TimestampFieldName = prevField
	})

	withArgs, staticKeySet := productionStaticWithArgs(entries)
	if len(withArgs) > 0 {
		withArgs = pslog.Keyvals(withArgs...)
	}
	dynamicEntries := entries
	if len(staticKeySet) > 0 {
		dynamicEntries = productionEntriesWithout(entries, staticKeySet)
	}
	keyvalsEntries := productionEntriesWithKeyvals(dynamicEntries)

	run := func(name string, opts pslog.Options, useWith bool, useKeyvals bool) {
		b.Run(name, func(b *testing.B) {
			sink.resetCount()
			opts.MinLevel = pslog.TraceLevel
			logger := pslog.NewWithOptions(sink, opts)
			activeEntries := entries
			if useWith {
				if len(withArgs) > 0 {
					logger = logger.With(withArgs...)
				}
				activeEntries = dynamicEntries
			}
			if useKeyvals {
				activeEntries = keyvalsEntries
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				entry := activeEntries[i%len(activeEntries)]
				entry.log(logger)
			}
			if sink.bytesWritten() == 0 {
				b.Fatalf("%s wrote zero bytes", name)
			}
			reportBytesPerOp(b, sink)
		})
	}

	run("pslog/production/json", pslog.Options{Mode: pslog.ModeStructured}, false, false)
	run("pslog/production/json+with", pslog.Options{Mode: pslog.ModeStructured}, true, false)
	run("pslog/production/json+keyvals", pslog.Options{Mode: pslog.ModeStructured}, true, true)
	run("pslog/production/jsoncolor", pslog.Options{Mode: pslog.ModeStructured, ForceColor: true}, false, false)
	run("pslog/production/jsoncolor+with", pslog.Options{Mode: pslog.ModeStructured, ForceColor: true}, true, false)
	run("pslog/production/jsoncolor+keyvals", pslog.Options{Mode: pslog.ModeStructured, ForceColor: true}, true, true)
	run("pslog/production/console", pslog.Options{Mode: pslog.ModeConsole, NoColor: true}, false, false)
	run("pslog/production/consolecolor", pslog.Options{Mode: pslog.ModeConsole, ForceColor: true}, false, false)

	zerologJSON := func() zerolog.Logger {
		return zerolog.New(sink).With().Timestamp().Logger().Level(zerolog.TraceLevel)
	}
	zerologConsole := func(color bool) zerolog.Logger {
		writer := zerolog.ConsoleWriter{Out: sink, NoColor: !color, TimeFormat: time.RFC3339}
		return zerolog.New(writer).With().Timestamp().Logger().Level(zerolog.TraceLevel)
	}
	runZerolog := func(name string, factory func() zerolog.Logger) {
		b.Run(name, func(b *testing.B) {
			sink.resetCount()
			logger := factory()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				entry := entries[i%len(entries)]
				lvl := zerologLevelFromPSLog(entry.level)
				e := logger.WithLevel(lvl)
				entry.applyZerolog(e).Msg(entry.message)
			}
			if sink.bytesWritten() == 0 {
				b.Fatalf("%s wrote zero bytes", name)
			}
			reportBytesPerOp(b, sink)
		})
	}

	runZerolog("zerolog/production/json", zerologJSON)
	runZerolog("zerolog/production/console", func() zerolog.Logger { return zerologConsole(false) })

	runPhuslu := func(name string, writer plog.Writer) {
		b.Run(name, func(b *testing.B) {
			sink.resetCount()
			logger := &plog.Logger{
				Level:  plog.TraceLevel,
				Writer: writer,
			}
			logger.TimeFormat = time.RFC3339
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				entry := entries[i%len(entries)]
				pe := phusluEntryForLevel(logger, entry.level)
				entry.applyPhuslu(pe)
				pe.Msg(entry.message)
			}
			if sink.bytesWritten() == 0 {
				b.Fatalf("%s wrote zero bytes", name)
			}
			reportBytesPerOp(b, sink)
		})
	}

	runPhuslu("phuslu/production/json", plog.IOWriter{Writer: sink})

	runZap := func(name string, encoder zapcore.Encoder) {
		b.Run(name, func(b *testing.B) {
			sink.resetCount()
			core := zapcore.NewCore(encoder, zapcore.AddSync(sink), zapcore.DebugLevel)
			logger := zap.New(core, zap.WithCaller(false))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				entry := entries[i%len(entries)]
				lvl := zapLevelFromPSLog(entry.level)
				if ce := logger.Check(lvl, entry.message); ce != nil {
					ce.Write(entry.zapFieldsSlice()...)
				}
			}
			if sink.bytesWritten() == 0 {
				b.Fatalf("%s wrote zero bytes", name)
			}
			reportBytesPerOp(b, sink)
		})
	}

	jsonEncoderCfg := zap.NewProductionEncoderConfig()
	jsonEncoderCfg.TimeKey = ""
	jsonEncoderCfg.CallerKey = ""
	jsonEncoderCfg.StacktraceKey = ""
	runZap("zap/production/json", zapcore.NewJSONEncoder(jsonEncoderCfg))

	consoleEncoderCfg := zap.NewDevelopmentEncoderConfig()
	consoleEncoderCfg.TimeKey = ""
	consoleEncoderCfg.CallerKey = ""
	consoleEncoderCfg.StacktraceKey = ""
	consoleEncoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	runZap("zap/production/console", zapcore.NewConsoleEncoder(consoleEncoderCfg))
}

func loadProductionEntries() []productionEntry {
	productionOnce.Do(func() {
		entries, err := parseProductionDataset(productionDatasetLimit)
		if err != nil {
			productionLoadErr = err
			return
		}
		filtered := make([]productionEntry, 0, len(entries))
		for _, entry := range entries {
			if entry.level == pslog.Disabled {
				continue
			}
			filtered = append(filtered, entry)
		}
		if len(filtered) == 0 {
			productionLoadErr = errors.New("no non-disabled production entries parsed")
			return
		}
		productionEntries = filtered
	})
	if productionLoadErr != nil {
		panic(productionLoadErr)
	}
	return productionEntries
}

func parseProductionDataset(limit int) ([]productionEntry, error) {
	max := len(embeddedProductionDataset)
	if max == 0 {
		return nil, errors.New("embedded production dataset empty")
	}
	if limit <= 0 || limit > max {
		limit = max
	}
	entries := make([]productionEntry, 0, limit)
	for i := 0; i < limit; i++ {
		line := embeddedProductionDataset[i]
		entry, err := parseProductionLine(line)
		if err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return nil, errors.New("no production log entries parsed")
	}
	return entries, nil
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
	return withArgs, staticKeys
}

func productionEntriesWithKeyvals(entries []productionEntry) []productionEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]productionEntry, len(entries))
	for i, entry := range entries {
		converted := entry
		converted.keyvals = pslog.Keyvals(entry.keyvals...)
		out[i] = converted
	}
	return out
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
		keyStr, ok := key.(string)
		if !ok {
			if ts, ok2 := key.(pslog.TrustedString); ok2 {
				keyStr = string(ts)
				ok = true
			}
		}
		if ok {
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
