package benchmark_test

import (
	"errors"
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

	run := func(name string, opts pslog.Options) {
		b.Run(name, func(b *testing.B) {
			sink.resetCount()
			opts.MinLevel = pslog.TraceLevel
			logger := pslog.NewWithOptions(sink, opts)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				entry := entries[i%len(entries)]
				entry.log(logger)
			}
			if sink.bytesWritten() == 0 {
				b.Fatalf("%s wrote zero bytes", name)
			}
			reportBytesPerOp(b, sink)
		})
	}

	run("pslog/production/json", pslog.Options{Mode: pslog.ModeStructured})
	run("pslog/production/jsoncolor", pslog.Options{Mode: pslog.ModeStructured, ForceColor: true})
	run("pslog/production/console", pslog.Options{Mode: pslog.ModeConsole, NoColor: true})
	run("pslog/production/consolecolor", pslog.Options{Mode: pslog.ModeConsole, ForceColor: true})

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
