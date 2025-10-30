package pslog

import (
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"strings"
	"time"
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

// DTGTimeFormat matches the legacy DTG layout used by the console logger.
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

	// MinLevel sets the minimum level the adapter will emit. Defaults to Debug.
	MinLevel Level

	// VerboseFields switches JSON keys from ts/lvl/msg to time/level/message.
	VerboseFields bool

	// UTC forces timestamps to be rendered in UTC.
	UTC bool
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

// NewBaseLogger returns a Base implementation writing to w with default options.
func NewBaseLogger(w io.Writer) Base {
	return buildAdapter(w, Options{Mode: ModeStructured})
}

// NewBaseLoggerWithOptions returns a Base implementation using the supplied options.
func NewBaseLoggerWithOptions(w io.Writer, opts Options) Base {
	return buildAdapter(w, opts)
}

type loggerContextKey struct{}

// ContextWithLogger returns a child context carrying the supplied logger implementation.
func ContextWithLogger(ctx context.Context, logger Logger) context.Context {
	if logger == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerContextKey{}, logger)
}

// LoggerFromContext extracts a logger implementation from context if present or returns a NoopLogger.
func LoggerFromContext(ctx context.Context) Logger {
	if ctx == nil {
		return noopLogger{}
	}
	if logger, ok := ctx.Value(loggerContextKey{}).(Logger); ok && logger != nil {
		return logger
	}
	return noopLogger{}
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
	disableColor := opts.NoColor || os.Getenv("NO_COLOR") != ""
	colorEnabled := !disableColor && (opts.ForceColor || isTerminal(w))

	cfg := coreConfig{
		writer:           w,
		minLevel:         minLevel,
		includeTimestamp: includeTimestamp,
		timeLayout:       timeFormat,
		useUTC:           useUTC,
		timeCache:        cache,
		timeFormatter:    formatter,
		logLevelValue:    LevelString(minLevel),
	}

	if mode == ModeStructured {
		if colorEnabled {
			return newJSONColorLogger(cfg, opts.VerboseFields)
		}
		return newJSONPlainLogger(cfg, opts.VerboseFields)
	}
	if colorEnabled {
		return newConsoleColorLogger(cfg)
	}
	return newConsolePlainLogger(cfg)
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
