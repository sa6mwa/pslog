// Package pslog provides a zero-allocation logging toolkit that favours minimal
// branching and precomputation. Structured and console adapters share no
// hot-path code: each variant inlines its own encoder, trading deliberate
// duplication for fewer instructions at runtime.
//
// # Design overview
//
//   - Construction-time setup: options resolve colour emitters, time layouts,
//     and field metadata (including whether keys are trusted) once during
//     logger creation.
//   - Inline emitters: JSON, JSON+colour, console, and console+colour each have
//     dedicated escape loops and value writers, so the inner log call executes
//     without mode checks.
//   - Chunk-aware scans: JSON and console escaping walk 16-byte blocks, copying
//     entire safe spans before emitting characters that require escaping.
//   - []any compatibility: the variadic API is retained; encoders specialise
//     common types (strings, numbers, bools, time) so the type switch is shallow
//     and branch prediction remains friendly.
//   - Time cache: formatted timestamps are cached per layout. When trusted, a
//     timestamp string is copied directly without re-validating in the hot
//     path.
//
// # Logging patterns
//
// Use Logger.With to attach static key/value pairs (the production benchmarks
// use this pattern). For dynamic slices, pslog.Keyvals returns a copy of the
// arguments with trusted strings promoted up front; this helps the
// performance-conscious path but is optional. When comparing with other
// loggers, remember that json+keyvals style benchmarks are intentionally unfair
// because every key skips the escape loop.
//
// # Usage
//
//	logger := pslog.NewStructured(os.Stdout).With("service", "checkout", "env", "prod")
//	logger.Info("ready", "port", 8080)
//
// The console adapter mirrors the API:
//
//	logger := pslog.New(os.Stdout)
//	logger.Warn("cache bust", "key", pslog.NewTrustedString("user:42"))
//
// When you want to pre-promote runtime arguments, pslog.Keyvals returns a copy
// with trusted strings tagged in advance:
//
//	dynamic := pslog.Keyvals(
//	"user", "alice",
//		"attempts", 3,
//		"latency_ms", 12.34,
//	)
//	logger.Info("login", dynamic...)
//
// Additional examples (context helpers, palette switching, etc.) live in the
// examples/ directory of the repository.
//
// # Integration notes
//
//   - Use Logger.LogLevel to derive loggers with different minimum levels.
//   - The ansi subpackage exposes palette controls (ansi.SetPalette).
//   - pslog.LogLogger bridges to the standard library by returning a *log.Logger
//     that feeds through to pslog.
//
// Benchmarks, tooling, and the elevator pitch visualiser are under the
// benchmark/ and elevatorpitch/ directories respectively.
package pslog
