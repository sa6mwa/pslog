package benchmark_test

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"

	apexlog "github.com/apex/log"
	apexjson "github.com/apex/log/handlers/json"
	apextext "github.com/apex/log/handlers/text"
	charm "github.com/charmbracelet/log"
	"github.com/cihub/seelog"
	onelog "github.com/francoispqt/onelog"
	kitlog "github.com/go-kit/log"
	"github.com/go-logr/logr/funcr"
	"github.com/golang/glog"
	"github.com/inconshreveable/log15"
	"github.com/lmittmann/tint"
	plog "github.com/phuslu/log"
	"github.com/rs/zerolog"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	klog "k8s.io/klog/v2"

	pslog "pkt.systems/pslog"
)

// timestampStyle determines how a logger renders timestamps in the benchmark.
type timestampStyle int

const (
	timestampFormatted timestampStyle = iota
	timestampUnix
)

// nativeBenchConfig wires which timestamp style applies to pslog vs the rest.
type nativeBenchConfig struct {
	pslogStyle        timestampStyle
	othersStyle       timestampStyle
	requireUnixLogger bool
}

type nativeLogger struct {
	name         string
	isPSLog      bool
	supportsUnix bool
	run          func(b *testing.B, sink *lockedDiscard, style timestampStyle)
}

var glogSetup sync.Once

var benchmarkEntries = mustLoadBenchmarkEntries()

var nativeBenchLogFiles = flag.Bool("native.bench.logfiles", false, "write native benchmark output to <logger>.log files")

func mustLoadBenchmarkEntries() []productionEntry {
	entries, err := loadEmbeddedProductionDataset(200)
	if err != nil {
		panic(err)
	}
	return entries
}

func sanitizeBenchmarkName(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		r = unicode.ToLower(r)
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "benchmark"
	}
	return b.String()
}

func applyZerologFields(ev *zerolog.Event, entry productionEntry) *zerolog.Event {
	return entry.applyZerolog(ev)
}

func applyPhusluFields(e *plog.Entry, entry productionEntry) {
	entry.applyPhuslu(e)
}

func applyOnelogFields(dst onelog.Entry, entry productionEntry) {
	entry.forEachField(func(key string, value any) {
		switch v := value.(type) {
		case string:
			dst.String(key, v)
		case bool:
			dst.Bool(key, v)
		case int:
			dst.Int(key, v)
		case int64:
			dst.Int64(key, v)
		case uint64:
			dst.Int64(key, int64(v))
		case float64:
			dst.Float(key, v)
		default:
			dst.String(key, fmt.Sprintf("%v", v))
		}
	})
}

func seelogFieldFormat(entry productionEntry) string {
	var b strings.Builder
	entry.forEachField(func(key string, value any) {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(fmt.Sprintf("%v", value))
	})
	return b.String()
}

func logrusFields(entry productionEntry) logrus.Fields {
	fields := logrus.Fields{}
	entry.forEachField(func(key string, value any) {
		fields[key] = value
	})
	return fields
}

func apexFields(entry productionEntry) apexlog.Fields {
	fields := apexlog.Fields{}
	entry.forEachField(func(key string, value any) {
		fields[key] = value
	})
	return fields
}

func BenchmarkLoggerFormattedTime(b *testing.B) {
	runNativeLoggerBenchmarks(b, nativeBenchConfig{
		pslogStyle:  timestampFormatted,
		othersStyle: timestampFormatted,
	})
}

func BenchmarkLoggerFormattedVsUnixEpoch(b *testing.B) {
	runNativeLoggerBenchmarks(b, nativeBenchConfig{
		pslogStyle:        timestampFormatted,
		othersStyle:       timestampUnix,
		requireUnixLogger: true,
	})
}

func newPSLogLogger(w io.Writer, opts pslog.Options) pslog.Logger {
	opts.MinLevel = pslog.TraceLevel
	return pslog.NewWithOptions(w, opts)
}

func runPSLogKVAny(b *testing.B, logger pslog.Logger) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entry := benchmarkEntries[i%len(benchmarkEntries)]
		logger.Log(entry.level, entry.message, entry.keyvals...)
	}
}

func BenchmarkPSLogVariants(b *testing.B) {
	sink := newBenchmarkSink()
	types := []struct {
		name       string
		mode       pslog.Mode
		forceColor bool
		noColor    bool
	}{
		{"json", pslog.ModeStructured, false, false},
		{"jsoncolor", pslog.ModeStructured, true, false},
		{"console", pslog.ModeConsole, false, true},
		{"consolecolor", pslog.ModeConsole, true, false},
	}
	for _, t := range types {
		name := fmt.Sprintf("pslog/kvany/%s", t.name)
		b.Run(name, func(b *testing.B) {
			sink.resetCount()
			logger := newPSLogLogger(sink, pslog.Options{
				Mode:       t.mode,
				TimeFormat: time.RFC3339,
				ForceColor: t.forceColor,
				NoColor:    t.noColor,
			})
			runPSLogKVAny(b, logger)
			if sink.bytesWritten() == 0 {
				b.Fatalf("%s wrote zero bytes", name)
			}
			reportBytesPerOp(b, sink)
		})
	}
}

func runNativeLoggerBenchmarks(b *testing.B, cfg nativeBenchConfig) {
	sink := newBenchmarkSink()

	for _, bench := range nativeLoggers() {
		bench := bench

		if cfg.requireUnixLogger && !bench.supportsUnix && !bench.isPSLog {
			continue
		}

		style := cfg.othersStyle
		if bench.isPSLog {
			style = cfg.pslogStyle
		} else if cfg.requireUnixLogger && !bench.supportsUnix && style == timestampUnix {
			continue
		}

		filename := sanitizeBenchmarkName(bench.name) + ".log"
		b.Run(bench.name, func(b *testing.B) {
			sink.resetCount()
			sink.setTee(nil)

			if *nativeBenchLogFiles {
				file, err := os.Create(filename)
				if err != nil {
					b.Fatalf("create log file: %v", err)
				}
				writer := bufio.NewWriter(file)
				sink.setTee(writer)
				b.Cleanup(func() {
					sink.setTee(nil)
					writer.Flush()
					file.Close()
				})
			}

			bench.run(b, sink, style)
			if sink.bytesWritten() == 0 {
				b.Fatalf("%s wrote zero bytes; check benchmark setup", bench.name)
			}
			reportBytesPerOp(b, sink)
		})
	}
}

func nativeLoggers() []nativeLogger {
	return []nativeLogger{
		{
			name:    "pslog/console",
			isPSLog: true,
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				opts := pslog.Options{
					Mode:       pslog.ModeConsole,
					NoColor:    true,
					TimeFormat: time.RFC3339,
				}
				logger := pslog.NewWithOptions(sink, opts)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					entry := benchmarkEntries[i%len(benchmarkEntries)]
					logger.Log(entry.level, entry.message, entry.keyvals...)
				}
			},
		},
		{
			name:    "pslog/consolecolor",
			isPSLog: true,
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				opts := pslog.Options{
					Mode:       pslog.ModeConsole,
					ForceColor: true,
					TimeFormat: time.RFC3339,
				}
				logger := pslog.NewWithOptions(sink, opts)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					entry := benchmarkEntries[i%len(benchmarkEntries)]
					logger.Log(entry.level, entry.message, entry.keyvals...)
				}
			},
		},
		{
			name:    "pslog/json",
			isPSLog: true,
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				opts := pslog.Options{
					Mode:       pslog.ModeStructured,
					TimeFormat: time.RFC3339,
				}
				logger := pslog.NewWithOptions(sink, opts)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					entry := benchmarkEntries[i%len(benchmarkEntries)]
					logger.Info(entry.message, entry.keyvals...)
				}
			},
		},
		{
			name:    "pslog/jsoncolor",
			isPSLog: true,
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				opts := pslog.Options{
					Mode:       pslog.ModeStructured,
					TimeFormat: time.RFC3339,
					ForceColor: true,
				}
				logger := pslog.NewWithOptions(sink, opts)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					entry := benchmarkEntries[i%len(benchmarkEntries)]
					logger.Info(entry.message, entry.keyvals...)
				}
			},
		},
		{
			name:         "zerolog/json",
			supportsUnix: true,
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				prevFormat := zerolog.TimeFieldFormat
				b.Cleanup(func() { zerolog.TimeFieldFormat = prevFormat })

				switch style {
				case timestampFormatted:
					zerolog.TimeFieldFormat = time.RFC3339
				case timestampUnix:
					zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
				}

				builder := zerolog.New(sink).With()
				if style == timestampFormatted || style == timestampUnix {
					builder = builder.Timestamp()
				}
				logger := builder.Logger()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					entry := benchmarkEntries[i%len(benchmarkEntries)]
					applyZerologFields(logger.Info(), entry).Msg(entry.message)
				}
			},
		},
		{
			name: "zerolog/console",
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				writer := zerolog.ConsoleWriter{
					Out:        sink,
					NoColor:    true,
					TimeFormat: time.RFC3339,
				}
				logger := zerolog.New(writer).With().Timestamp().Logger()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					entry := benchmarkEntries[i%len(benchmarkEntries)]
					applyZerologFields(logger.Info(), entry).Msg(entry.message)
				}
			},
		},
		{
			name:         "zerolog/global",
			supportsUnix: true,
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				prevFormat := zerolog.TimeFieldFormat
				b.Cleanup(func() { zerolog.TimeFieldFormat = prevFormat })
				switch style {
				case timestampFormatted:
					zerolog.TimeFieldFormat = time.RFC3339
				case timestampUnix:
					zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
				}
				logger := zerolog.New(sink)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					entry := benchmarkEntries[i%len(benchmarkEntries)]
					applyZerologFields(logger.Info().Timestamp(), entry).Msg(entry.message)
				}
			},
		},
		{
			name: "charm/console",
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				opts := charm.Options{
					ReportTimestamp: true,
					TimeFormat:      time.RFC3339,
				}
				logger := charm.NewWithOptions(sink, opts)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					entry := benchmarkEntries[i%len(benchmarkEntries)]
					logger.Info(entry.message, entry.keyvals...)
				}
			},
		},
		{
			name: "charm/json",
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				opts := charm.Options{
					Formatter:       charm.JSONFormatter,
					ReportTimestamp: true,
					TimeFormat:      time.RFC3339,
				}
				logger := charm.NewWithOptions(sink, opts)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					entry := benchmarkEntries[i%len(benchmarkEntries)]
					logger.Info(entry.message, entry.keyvals...)
				}
			},
		},
		{
			name: "slog/json",
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				opts := slog.HandlerOptions{}
				if style == timestampUnix {
					opts.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
						if a.Key == slog.TimeKey && a.Value.Kind() == slog.KindTime {
							return slog.Int64("ts", a.Value.Time().UnixNano())
						}
						return a
					}
				}
				handler := slog.NewJSONHandler(sink, &opts)
				logger := slog.New(handler)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					entry := benchmarkEntries[i%len(benchmarkEntries)]
					logger.Info(entry.message, entry.keyvals...)
				}
			},
			supportsUnix: true,
		},
		{
			name:         "zap/json",
			supportsUnix: true,
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				encoderCfg := zap.NewProductionEncoderConfig()
				if style == timestampFormatted {
					encoderCfg.EncodeTime = zapcore.RFC3339TimeEncoder
				} else {
					encoderCfg.EncodeTime = zapcore.EpochMillisTimeEncoder
				}
				core := zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), zapcore.AddSync(sink), zapcore.InfoLevel)
				logger := zap.New(core, zap.WithCaller(false))
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					entry := benchmarkEntries[i%len(benchmarkEntries)]
					logger.Info(entry.message, entry.zapFieldsSlice()...)
				}
			},
		},
		{
			name:         "phuslu/json",
			supportsUnix: true,
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				logger := &plog.Logger{
					Level:  plog.InfoLevel,
					Writer: plog.IOWriter{Writer: sink},
				}
				if style == timestampFormatted {
					logger.TimeFormat = time.RFC3339
				} else {
					logger.TimeFormat = plog.TimeFormatUnix
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					logEntry := logger.Info()
					entry := benchmarkEntries[i%len(benchmarkEntries)]
					applyPhusluFields(logEntry, entry)
					logEntry.Msg(entry.message)
				}
			},
		},
		{
			name:         "onelog/json",
			supportsUnix: true,
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				logger := onelog.New(sink, onelog.ALL)
				if style == timestampFormatted {
					logger.Hook(func(e onelog.Entry) {
						e.String("ts", time.Now().UTC().Format(time.RFC3339))
					})
				} else {
					logger.Hook(func(e onelog.Entry) {
						e.Int64("ts", time.Now().UTC().UnixNano())
					})
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					entry := benchmarkEntries[i%len(benchmarkEntries)]
					logger.InfoWithFields(entry.message, func(en onelog.Entry) {
						applyOnelogFields(en, entry)
					})
				}
			},
		},
		{
			name:         "kitlog/json",
			supportsUnix: true,
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				logger := kitlog.NewJSONLogger(sink)
				logger = kitlog.With(logger, "level", "info")
				if style == timestampFormatted {
					logger = kitlog.With(logger, "ts", kitlog.TimestampFormat(func() time.Time {
						return time.Now().UTC()
					}, time.RFC3339))
				} else {
					logger = kitlog.With(logger, "ts", kitlog.Valuer(func() interface{} {
						return time.Now().UTC().UnixNano()
					}))
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					entry := benchmarkEntries[i%len(benchmarkEntries)]
					msgFields := append([]interface{}{"msg", entry.message}, entry.keyvals...)
					_ = logger.Log(msgFields...)
				}
			},
		},
		{
			name: "logr/funcr",
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				opts := funcr.Options{
					LogTimestamp:    true,
					TimestampFormat: time.RFC3339,
				}
				logger := funcr.NewJSON(func(obj string) {
					sink.Write([]byte(obj))
				}, opts)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					entry := benchmarkEntries[i%len(benchmarkEntries)]
					logger.Info(entry.message, entry.keyvals...)
				}
			},
		},
		{
			name:         "zap/console",
			supportsUnix: true,
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				encoderCfg := zap.NewDevelopmentEncoderConfig()
				encoderCfg.EncodeLevel = zapcore.CapitalLevelEncoder
				if style == timestampFormatted {
					encoderCfg.EncodeTime = zapcore.RFC3339TimeEncoder
				} else {
					encoderCfg.EncodeTime = zapcore.EpochMillisTimeEncoder
				}
				core := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderCfg), zapcore.AddSync(sink), zapcore.InfoLevel)
				logger := zap.New(core, zap.WithCaller(false))
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					entry := benchmarkEntries[i%len(benchmarkEntries)]
					logger.Info(entry.message, entry.zapFieldsSlice()...)
				}
			},
		},
		{
			name:         "zap/sugar",
			supportsUnix: true,
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				encoderCfg := zap.NewProductionEncoderConfig()
				if style == timestampFormatted {
					encoderCfg.EncodeTime = zapcore.RFC3339TimeEncoder
				} else {
					encoderCfg.EncodeTime = zapcore.EpochMillisTimeEncoder
				}
				core := zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), zapcore.AddSync(sink), zapcore.InfoLevel)
				logger := zap.New(core, zap.WithCaller(false)).Sugar()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					entry := benchmarkEntries[i%len(benchmarkEntries)]
					logger.Infow(entry.message, entry.keyvals...)
				}
			},
		},
		{
			name: "logrus/text",
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				logger := logrus.New()
				logger.SetOutput(sink)
				logger.SetLevel(logrus.InfoLevel)
				formatter := &logrus.TextFormatter{
					DisableColors:   true,
					FullTimestamp:   true,
					TimestampFormat: time.RFC3339,
				}
				if style == timestampUnix {
					// fall back to formatted output for unsupported mode
					style = timestampFormatted
				}
				logger.SetFormatter(formatter)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					entry := benchmarkEntries[i%len(benchmarkEntries)]
					logger.WithFields(logrusFields(entry)).Info(entry.message)
				}
			},
		},
		{
			name: "logrus/json",
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				logger := logrus.New()
				logger.SetOutput(sink)
				logger.SetLevel(logrus.InfoLevel)
				formatter := &logrus.JSONFormatter{
					TimestampFormat: time.RFC3339,
				}
				if style == timestampUnix {
					style = timestampFormatted
				}
				logger.SetFormatter(formatter)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					entry := benchmarkEntries[i%len(benchmarkEntries)]
					logger.WithFields(logrusFields(entry)).Info(entry.message)
				}
			},
		},
		{
			name: "apex/text",
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				_ = style
				handler := apextext.New(sink)
				logger := &apexlog.Logger{
					Handler: handler,
					Level:   apexlog.InfoLevel,
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					entry := benchmarkEntries[i%len(benchmarkEntries)]
					logger.WithFields(apexFields(entry)).Info(entry.message)
				}
			},
		},
		{
			name: "apex/json",
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				_ = style
				handler := apexjson.New(sink)
				logger := &apexlog.Logger{
					Handler: handler,
					Level:   apexlog.InfoLevel,
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					entry := benchmarkEntries[i%len(benchmarkEntries)]
					logger.WithFields(apexFields(entry)).Info(entry.message)
				}
			},
		},
		{
			name: "log15/logfmt",
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				_ = style
				handler := log15.StreamHandler(sink, log15.LogfmtFormat())
				logger := log15.New()
				logger.SetHandler(handler)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					entry := benchmarkEntries[i%len(benchmarkEntries)]
					logger.Info(entry.message, entry.keyvals...)
				}
			},
		},
		{
			name: "seelog/custom",
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				_ = style
				logger := newSeelogLogger(b, sink)
				defer logger.Flush()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					entry := benchmarkEntries[i%len(benchmarkEntries)]
					logger.Infof("%s %s", entry.message, seelogFieldFormat(entry))
				}
			},
		},
		{
			name: "klog/text",
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				_ = style
				klog.LogToStderr(false)
				defer klog.LogToStderr(true)
				klog.SetOutput(sink)
				defer klog.SetOutput(io.Discard)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					entry := benchmarkEntries[i%len(benchmarkEntries)]
					klog.InfoS(entry.message, entry.keyvals...)
				}
				klog.Flush()
			},
		},
		{
			name: "glog/text",
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				_ = style
				glogSetup.Do(func() {
					_ = flag.Set("logtostderr", "true")
					_ = flag.Set("alsologtostderr", "false")
					_ = flag.Set("log_dir", "")
				})
				withRedirectedStderr(b, sink, func() {
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						entry := benchmarkEntries[i%len(benchmarkEntries)]
						glog.Infof("%s %s", entry.message, seelogFieldFormat(entry))
					}
					glog.Flush()
				})
			},
		},
		{
			name: "tint/console",
			run: func(b *testing.B, sink *lockedDiscard, style timestampStyle) {
				if style == timestampUnix {
					style = timestampFormatted
				}
				handler := tint.NewHandler(sink, &tint.Options{
					NoColor:    true,
					TimeFormat: time.RFC3339,
				})
				logger := slog.New(handler)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					entry := benchmarkEntries[i%len(benchmarkEntries)]
					logger.Info(entry.message, entry.keyvals...)
				}
			},
		},
	}
}

type discardReceiver struct {
	sink *lockedDiscard
}

func (r *discardReceiver) ReceiveMessage(message string, _ seelog.LogLevel, _ seelog.LogContextInterface) error {
	if message == "" {
		return nil
	}
	if _, err := r.sink.Write([]byte(message)); err != nil {
		return err
	}
	_, err := r.sink.Write([]byte{'\n'})
	return err
}

func (r *discardReceiver) AfterParse(seelog.CustomReceiverInitArgs) error { return nil }
func (r *discardReceiver) Flush()                                         {}
func (r *discardReceiver) Close() error                                   { return nil }

func newSeelogLogger(b *testing.B, sink *lockedDiscard) seelog.LoggerInterface {
	logger, err := seelog.LoggerFromCustomReceiver(&discardReceiver{sink: sink})
	if err != nil {
		b.Fatalf("failed to create seelog custom receiver: %v", err)
	}
	return logger
}

func withRedirectedStderr(b *testing.B, sink *lockedDiscard, fn func()) {
	r, w, err := os.Pipe()
	if err != nil {
		b.Fatalf("stderr redirection failed: %v", err)
	}
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(sink, r)
		close(done)
	}()
	orig := os.Stderr
	os.Stderr = w
	defer func() {
		_ = w.Close()
		<-done
		_ = r.Close()
		os.Stderr = orig
	}()
	fn()
}
