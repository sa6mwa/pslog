package pslog

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"pkt.systems/pslog/ansi"
)

// LoggerFromEnvOption customizes LoggerFromEnv behavior.
type LoggerFromEnvOption func(*loggerFromEnvConfig)

type loggerFromEnvConfig struct {
	prefix  string
	options Options
	writer  io.Writer
}

// WithEnvPrefix overrides the environment variable prefix used by LoggerFromEnv.
func WithEnvPrefix(prefix string) LoggerFromEnvOption {
	return func(cfg *loggerFromEnvConfig) {
		cfg.prefix = prefix
	}
}

// WithEnvOptions seeds LoggerFromEnv with explicit Options values.
func WithEnvOptions(opts Options) LoggerFromEnvOption {
	return func(cfg *loggerFromEnvConfig) {
		cfg.options = opts
	}
}

// WithEnvWriter seeds LoggerFromEnv with a default output writer.
func WithEnvWriter(w io.Writer) LoggerFromEnvOption {
	return func(cfg *loggerFromEnvConfig) {
		cfg.writer = w
	}
}

// LoggerFromEnv builds a logger from environment variables, allowing optional
// seeded options and writers. Environment values override supplied options.
//
// Recognised variables are: {prefix}LEVEL, VERBOSE_FIELDS, CALLER_KEYVAL,
// CALLER_KEY, MODE (console|structured|json), TIME_FORMAT, DISABLE_TIMESTAMP,
// NO_COLOR, FORCE_COLOR, PALETTE, UTC, and OUTPUT. OUTPUT accepts stdout, stderr,
// default, a file path, or stdout+/stderr+/default+<path> to tee.
func LoggerFromEnv(opts ...LoggerFromEnvOption) Logger {
	cfg := loggerFromEnvConfig{prefix: "LOG_"}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	resolvedOpts := cfg.options
	baseWriter := cfg.writer
	if baseWriter == nil {
		baseWriter = os.Stdout
	}
	prefix := cfg.prefix
	if value, ok := lookupEnv(prefix, "LEVEL"); ok {
		if level, ok := ParseLevel(value); ok {
			resolvedOpts.MinLevel = level
		}
	}
	if value, ok := lookupEnv(prefix, "VERBOSE_FIELDS"); ok {
		if parsed, ok := parseEnvBool(value); ok {
			resolvedOpts.VerboseFields = parsed
		}
	}
	if value, ok := lookupEnv(prefix, "CALLER_KEYVAL"); ok {
		if parsed, ok := parseEnvBool(value); ok {
			resolvedOpts.CallerKeyval = parsed
		}
	}
	if value, ok := lookupEnv(prefix, "CALLER_KEY"); ok {
		if parsed := strings.TrimSpace(value); parsed != "" {
			resolvedOpts.CallerKey = parsed
		}
	}
	if value, ok := lookupEnv(prefix, "MODE"); ok {
		if parsed, ok := parseEnvMode(value); ok {
			resolvedOpts.Mode = parsed
		}
	}
	if value, ok := lookupEnv(prefix, "TIME_FORMAT"); ok {
		if parsed := strings.TrimSpace(value); parsed != "" {
			resolvedOpts.TimeFormat = parsed
		}
	}
	if value, ok := lookupEnv(prefix, "DISABLE_TIMESTAMP"); ok {
		if parsed, ok := parseEnvBool(value); ok {
			resolvedOpts.DisableTimestamp = parsed
		}
	}
	if value, ok := lookupEnv(prefix, "NO_COLOR"); ok {
		if parsed, ok := parseEnvBool(value); ok {
			resolvedOpts.NoColor = parsed
		}
	}
	if value, ok := lookupEnv(prefix, "FORCE_COLOR"); ok {
		if parsed, ok := parseEnvBool(value); ok {
			resolvedOpts.ForceColor = parsed
		}
	}
	if value, ok := lookupEnv(prefix, "PALETTE"); ok {
		resolvedOpts.Palette = ansi.PaletteByName(value)
	}
	if value, ok := lookupEnv(prefix, "UTC"); ok {
		if parsed, ok := parseEnvBool(value); ok {
			resolvedOpts.UTC = parsed
		}
	}
	outputValue, hasOutput := lookupEnv(prefix, "OUTPUT")
	writer := baseWriter
	var outputErr error
	if hasOutput {
		if resolved, err := writerFromEnvOutput(outputValue, baseWriter); err != nil {
			outputErr = err
			writer = baseWriter
		} else {
			writer = resolved
		}
	}
	logger := NewWithOptions(writer, resolvedOpts)
	if outputErr != nil {
		logger.With(outputErr).Error("logger.output.open.failed", "output", strings.TrimSpace(outputValue))
	}
	return logger
}

func lookupEnv(prefix, key string) (string, bool) {
	if prefix == "" {
		return os.LookupEnv(key)
	}
	return os.LookupEnv(prefix + key)
}

func parseEnvBool(value string) (bool, bool) {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false, false
	}
	return parsed, true
}

func parseEnvMode(value string) (Mode, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "console":
		return ModeConsole, true
	case "structured", "json":
		return ModeStructured, true
	default:
		return ModeConsole, false
	}
}

func writerFromEnvOutput(value string, base io.Writer) (io.Writer, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return base, nil
	}
	if base == nil {
		base = io.Discard
	}
	lowered := strings.ToLower(trimmed)
	switch lowered {
	case "stdout":
		return os.Stdout, nil
	case "stderr":
		return os.Stderr, nil
	case "default":
		return base, nil
	}
	const (
		stdoutPrefix  = "stdout+"
		stderrPrefix  = "stderr+"
		defaultPrefix = "default+"
	)
	switch {
	case strings.HasPrefix(lowered, stdoutPrefix):
		path := strings.TrimSpace(trimmed[len(stdoutPrefix):])
		if path == "" {
			return os.Stdout, nil
		}
		fileWriter, err := openLogOutputFile(path)
		if err != nil {
			return base, err
		}
		return newOwnedOutput(newTeeWriter(os.Stdout, fileWriter), fileWriter), nil
	case strings.HasPrefix(lowered, stderrPrefix):
		path := strings.TrimSpace(trimmed[len(stderrPrefix):])
		if path == "" {
			return os.Stderr, nil
		}
		fileWriter, err := openLogOutputFile(path)
		if err != nil {
			return base, err
		}
		return newOwnedOutput(newTeeWriter(os.Stderr, fileWriter), fileWriter), nil
	case strings.HasPrefix(lowered, defaultPrefix):
		path := strings.TrimSpace(trimmed[len(defaultPrefix):])
		if path == "" {
			return base, nil
		}
		fileWriter, err := openLogOutputFile(path)
		if err != nil {
			return base, err
		}
		return newOwnedOutput(newTeeWriter(base, fileWriter), fileWriter), nil
	default:
		fileWriter, err := openLogOutputFile(trimmed)
		if err != nil {
			return base, err
		}
		return newOwnedOutput(fileWriter, fileWriter), nil
	}
}

func openLogOutputFile(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log output %q: %w", path, err)
	}
	return file, nil
}
