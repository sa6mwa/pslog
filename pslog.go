package pslog

import (
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"pkt.systems/pslog/ansi"
)

// Base defines the smallest set of convenience methods that library authors can
// require when they want consumers to plug in their own logger. Callers can
// satisfy this interface with a bespoke implementation or by reusing a Logger.
type Base interface {
	// Trace logs msg at TraceLevel (below DebugLevel).
	Trace(msg string, keyvals ...any)
	// Debug logs msg at DebugLevel.
	Debug(msg string, keyvals ...any)
	// Info logs msg at InfoLevel.
	Info(msg string, keyvals ...any)
	// Warn logs msg at WarnLevel.
	Warn(msg string, keyvals ...any)
	// Error logs msg at ErrorLevel.
	Error(msg string, keyvals ...any)
}

// Logger is the main interface of pslog.
type Logger interface {
	Base
	// Fatal logs msg at FatalLevel and terminates the process when the backend supports it.
	Fatal(msg string, keyvals ...any)
	// Panic logs msg at PanicLevel and panics when the backend supports it.
	Panic(msg string, keyvals ...any)
	// Log emits msg at the supplied pslog level.
	Log(level Level, msg string, keyvals ...any)

	// With returns a logger that includes the supplied key/value pairs on every
	// subsequent log entry. The receiver remains untouched.
	With(keyvals ...any) Logger

	// WithLogLevel returns a logger that carries a `loglevel` field describing
	// the logger's effective severity.
	WithLogLevel() Logger

	// LogLevel returns a logger derived from the receiver whose minimum level is
	// set to level. The receiver itself is not modified.
	LogLevel(Level) Logger

	// LogLevelFromEnv configures the logger's level using the value of key in the
	// environment. Recognised values are the same as ParseLevel. Missing or
	// invalid values leave the logger unchanged.
	LogLevelFromEnv(key string) Logger
}

// Mode controls how pslog renders log entries.
type Mode int

const (
	// ModeConsole emits zerolog-style console lines (colour aware, minimal allocations).
	ModeConsole Mode = iota
	// ModeStructured emits compact JSON (optionally colourful) suitable for ingestion.
	ModeStructured
)

// Level defines log levels.
type Level int8

const (
	// DebugLevel defines debug log level.
	DebugLevel Level = iota
	// InfoLevel defines info log level.
	InfoLevel
	// WarnLevel defines warn log level.
	WarnLevel
	// ErrorLevel defines error log level.
	ErrorLevel
	// FatalLevel defines fatal log level.
	FatalLevel
	// PanicLevel defines panic log level.
	PanicLevel
	// NoLevel defines an absent log level.
	NoLevel
	// Disabled disables the logger.
	Disabled
	// TraceLevel defines trace log level.
	TraceLevel Level = -1
)

// ParseLevel converts a textual level into a Level value. It accepts values
// such as "trace", "debug", "info", "warn", "warning", "error",
// "fatal", "panic", "no", "nolevel", "disabled", and "off" (case
// insensitive).
func ParseLevel(value string) (Level, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "trace":
		return TraceLevel, true
	case "debug":
		return DebugLevel, true
	case "info":
		return InfoLevel, true
	case "warn", "warning":
		return WarnLevel, true
	case "error":
		return ErrorLevel, true
	case "fatal":
		return FatalLevel, true
	case "panic":
		return PanicLevel, true
	case "no", "nolevel", "none":
		return NoLevel, true
	case "disabled", "disable", "off":
		return Disabled, true
	default:
		return InfoLevel, false
	}
}

// LevelString returns the canonical string representation of a Level.
func LevelString(level Level) string {
	switch level {
	case TraceLevel:
		return "trace"
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case WarnLevel:
		return "warn"
	case ErrorLevel:
		return "error"
	case FatalLevel:
		return "fatal"
	case PanicLevel:
		return "panic"
	case NoLevel:
		return "nolevel"
	case Disabled:
		return "disabled"
	default:
		return "info"
	}
}

// LevelFromEnv looks up key in the environment and parses it into a Level.
func LevelFromEnv(key string) (Level, bool) {
	if key == "" {
		return InfoLevel, false
	}
	value, ok := os.LookupEnv(key)
	if !ok {
		return InfoLevel, false
	}
	return ParseLevel(value)
}

// DTGTimeFormat is the default Date Time Group format (DDHHMM) for the console
// logger.
var DTGTimeFormat = "021504"

// Options controls how the pslog adapter formats and filters output.
type Options struct {
	// Mode selects console (default) or structured JSON rendering.
	Mode Mode

	// TimeFormat overrides the timestamp layout. When empty, pslog uses
	// DTGTimeFormat for console output and time.RFC3339 for JSON.
	TimeFormat string

	// DisableTimestamp drops the timestamp entirely.
	DisableTimestamp bool

	// NoColor forces colour escape codes off regardless of terminal detection.
	NoColor bool

	// ForceColor bypasses terminal detection and emits colour even when the
	// destination is not a TTY. Useful for tests and forced-colour logs.
	ForceColor bool

	// Palette overrides the ANSI palette for colorized console/JSON loggers.
	// When nil, pslog uses ansi.PaletteDefault.
	Palette *ansi.Palette

	// MinLevel sets the minimum level the adapter will emit. Defaults to Debug.
	MinLevel Level

	// VerboseFields switches JSON keys from ts/lvl/msg to time/level/message.
	VerboseFields bool

	// UTC forces timestamps to be rendered in UTC.
	UTC bool

	// CallerKeyval emits the calling function name on every log entry.
	CallerKeyval bool

	// CallerKey sets the key used when CallerKeyval is enabled. Defaults to "fn".
	CallerKey string
}

// New constructs a pslog adapter configured for console output.
func New(w io.Writer) Logger {
	return NewWithOptions(w, Options{Mode: ModeConsole})
}

// NewStructured returns a pslog adapter in structured JSON mode.
func NewStructured(w io.Writer) Logger {
	return NewWithOptions(w, Options{Mode: ModeStructured})
}

// NewStructuredNoColor returns a pslog adapter that always emits plain JSON.
func NewStructuredNoColor(w io.Writer) Logger {
	return NewWithOptions(w, Options{Mode: ModeStructured, NoColor: true})
}

// NewWithOptions builds a pslog adapter with explicit settings.
func NewWithOptions(w io.Writer, opts Options) Logger {
	return buildAdapter(w, opts)
}

// NewWithPalette builds a logger in mode using palette for colorized output.
func NewWithPalette(w io.Writer, mode Mode, palette *ansi.Palette) Logger {
	return NewWithOptions(w, Options{Mode: mode, Palette: palette})
}

// NewBaseLogger returns a Base implementation writing to w with default
// options.
func NewBaseLogger(w io.Writer) Base {
	return buildAdapter(w, Options{Mode: ModeStructured})
}

// NewBaseLoggerWithOptions returns a Base implementation using the supplied
// options.
func NewBaseLoggerWithOptions(w io.Writer, opts Options) Base {
	return buildAdapter(w, opts)
}

type loggerContextKey struct{}

// ContextWithLogger returns a child context carrying the supplied pslog.Logger
// implementation.
func ContextWithLogger(ctx context.Context, logger Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerContextKey{}, logger)
}

// ContextWithBaseLogger returns a child context carrying the supplied
// pslog.Base logger implementation.
func ContextWithBaseLogger(ctx context.Context, logger Base) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerContextKey{}, logger)
}

// LoggerFromContext extracts a pslog.Logger implementation from context if
// present or returns a NoopLogger.
func LoggerFromContext(ctx context.Context) Logger {
	if ctx == nil {
		return noopLogger{}
	}
	if logger, ok := ctx.Value(loggerContextKey{}).(Logger); ok && logger != nil {
		return logger
	}
	return noopLogger{}
}

// BaseLoggerFromContext extract a pslog.Base logger implementation from context
// if present or returns a NoopLogger.
func BaseLoggerFromContext(ctx context.Context) Base {
	if ctx == nil {
		return noopLogger{}
	}
	if logger, ok := ctx.Value(loggerContextKey{}).(Base); ok && logger != nil {
		return logger
	}
	return noopLogger{}
}

// Ctx extracts a pslog logger from context if present or return a NoopLogger.
func Ctx(ctx context.Context) Logger {
	return LoggerFromContext(ctx)
}

// BCtx extracts a pslog logger from context if present or a NoopLogger and
// returns a pslog.Base logger implementation.
func BCtx(ctx context.Context) Base {
	return BaseLoggerFromContext(ctx)
}

// LogLogger wraps a Logger implementation into a stdlib *log.Logger.
func LogLogger(logger Logger) *log.Logger {
	if logger == nil {
		logger = noopLogger{}
	}
	return log.New(loggerWriter{logger: logger}, "", 0)
}

// LogLoggerWithLevel wraps a Logger implementation into a stdlib *log.Logger that pins every emitted entry to level.
func LogLoggerWithLevel(logger Logger, level Level) *log.Logger {
	if logger == nil {
		logger = noopLogger{}
	}
	return log.New(levelPinnedWriter{logger: logger, level: level}, "", 0)
}

func buildAdapter(w io.Writer, opts Options) Logger {
	if w == nil {
		w = io.Discard
	}
	mode := opts.Mode
	if mode != ModeStructured {
		mode = ModeConsole
	}
	minLevel := opts.MinLevel
	timeFormat := opts.TimeFormat
	if timeFormat == "" {
		if mode == ModeConsole {
			timeFormat = DTGTimeFormat
		} else {
			timeFormat = time.RFC3339
		}
	}
	formatter := formatterForLayout(timeFormat)
	includeTimestamp := !opts.DisableTimestamp
	useUTC := opts.UTC
	var cache *timeCache
	if includeTimestamp && isCacheableLayout(timeFormat) {
		cache = newTimeCache(timeFormat, useUTC, formatter)
	}
	timestampTrusted := false
	if includeTimestamp {
		sample := time.Date(2024, time.January, 2, 15, 4, 5, 0, time.UTC)
		if !useUTC {
			sample = sample.Local()
		}
		var formatted string
		switch {
		case cache != nil:
			formatted = cache.formatTime(sample)
		case formatter != nil:
			formatted = formatter(sample)
		default:
			formatted = sample.Format(timeFormat)
		}
		timestampTrusted = promoteTrustedValueString(formatted)
	}
	disableColor := opts.NoColor
	colorEnabled := !disableColor && (opts.ForceColor || isTerminal(w))

	callerKey := opts.CallerKey
	if callerKey == "" {
		callerKey = "fn"
	}

	cfg := coreConfig{
		writer:           w,
		minLevel:         minLevel,
		includeTimestamp: includeTimestamp,
		timeLayout:       timeFormat,
		useUTC:           useUTC,
		timeCache:        cache,
		timeFormatter:    formatter,
		logLevelValue:    LevelString(minLevel),
		timestampTrusted: timestampTrusted,
		includeCaller:    opts.CallerKeyval,
		callerKey:        callerKey,
		callerKeyTrusted: stringTrustedASCII(callerKey),
	}

	if mode == ModeStructured {
		if colorEnabled {
			return newJSONColorLogger(cfg, opts)
		}
		return newJSONPlainLogger(cfg, opts)
	}
	if colorEnabled {
		return newConsoleColorLogger(cfg, opts)
	}
	return newConsolePlainLogger(cfg, opts)
}

func classifyLineLevel(line string) (Level, string) {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "[") {
		if end := strings.IndexRune(trimmed, ']'); end > 1 {
			candidate := trimmed[1:end]
			if lvl, ok := ParseLevel(candidate); ok {
				msg := strings.TrimSpace(trimmed[end+1:])
				return lvl, msg
			}
		}
	}
	lowered := strings.ToLower(trimmed)
	trimTail := func(prefixLen int) string {
		tail := strings.TrimSpace(trimmed[prefixLen:])
		tail = strings.TrimLeft(tail, ":- ")
		return strings.TrimSpace(tail)
	}
	switch {
	case strings.HasPrefix(lowered, "trace"):
		return TraceLevel, trimTail(len("trace"))
	case strings.HasPrefix(lowered, "debug"):
		return DebugLevel, trimTail(len("debug"))
	case strings.HasPrefix(lowered, "info"):
		return InfoLevel, trimTail(len("info"))
	case strings.HasPrefix(lowered, "warn"):
		return WarnLevel, trimTail(len("warn"))
	case strings.HasPrefix(lowered, "error"):
		return ErrorLevel, trimTail(len("error"))
	case strings.HasPrefix(lowered, "fatal"):
		return FatalLevel, trimTail(len("fatal"))
	case strings.HasPrefix(lowered, "panic"):
		return PanicLevel, trimTail(len("panic"))
	default:
		return InfoLevel, trimmed
	}
}

type loggerWriter struct {
	logger Logger
}

func (w loggerWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if w.logger == nil {
		return len(p), nil
	}
	lines := bytes.SplitSeq(p, []byte{'\n'})
	for line := range lines {
		line = bytes.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(string(line))
		if trimmed == "" {
			continue
		}
		level, msg := classifyLineLevel(trimmed)
		w.logger.Log(level, msg)
	}
	return len(p), nil
}

type levelPinnedWriter struct {
	logger Logger
	level  Level
}

func (w levelPinnedWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if w.logger == nil {
		return len(p), nil
	}
	lines := bytes.SplitSeq(p, []byte{'\n'})
	for line := range lines {
		line = bytes.TrimSpace(bytes.TrimSuffix(line, []byte{'\r'}))
		if len(line) == 0 {
			continue
		}
		w.logger.Log(w.level, string(line))
	}
	return len(p), nil
}
