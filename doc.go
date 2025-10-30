// Package pslog provides a zero-allocation logging toolkit that focuses on
// fast, readable structured output. It ships with compact JSON and console
// encoders, as well as colorised variants that make development logs easier to
// scan. The package intentionally exposes a single Logger interface rather than
// mirroring the expansive slog API, allowing the adapter to keep its hot path
// tight and allocation free.
//
// # Design overview
//
// pslog’s structured encoders are built around the lineWriter type. Each log
// call reserves a single growing buffer, emits the time stamp, level token,
// message, and then serialises key/value pairs in one pass. A few performance
// tricks keep the ns/op numbers low:
//
//   - Time caching: formatted timestamps are served from a timeCache. When you
//     use the default RFC3339 layout pslog refreshes the cached string once per
//     second, so formatted output is as cheap as unix-epoch logging in other
//     libraries.
//   - Trusted strings: keys and string values get passed through safecheck
//     promotion helpers. Once a token is known to be ASCII without control
//     characters the encoder can copy it directly into the buffer without
//     running an escaper.
//   - Level and colour renderers are split into two independent implementations
//     (plain and colour).  That avoids runtime branches in the inner loop, so
//     the JSON writer can remain branchless once a logger instance has been
//     constructed.
//   - Per-field collectors resolve types into pre-sized fragments (booleans,
//     numbers, durations, time.Time) before the writer begins flushing. The
//     serialisation code therefore only sees `append` calls with a known
//     capacity.
//
// In structured mode the output is a single JSON object per line; console mode
// emits key/value text similar to zerolog’s console encoder. Both outputs can
// be forced to colour regardless of TTY state by setting the ForceColor option.
//
// # Usage
//
// Basic structured logging:
//
//	logger := pslog.NewStructured(os.Stdout)
//	logger.Log(pslog.InfoLevel, "ready", "port", 8080, "mode", "production")
//
// Console logging with colours:
//
//	logger := pslog.New(os.Stdout)
//	logger.Log(pslog.WarnLevel, "cache bust", "key", pslog.NewTrustedString("user:42"))
//
// Advanced usage (context helpers, per-logger palettes, etc.) is demonstrated in the
// examples/ directory of the repository:
// https://github.com/sa6mwa/pslog/tree/main/examples
//
// # Integration tips
//
//   - Use With to attach a static set of key/value pairs to a logger instance.
//   - Call LogLevel to derive a logger that uses a different minimum level
//     without rebuilding the adapter. Options.MinLevel defaults to DebugLevel.
//   - The package exposes colour control through the ansi subpackage. Call
//     ansi.SetPalette to swap console/JSON colour palettes at runtime.
//   - When bridging pslog to the standard library, wrap a logger with
//     pslog.LogLogger; the helper returns a *log.Logger whose Writer funnels
//     back into pslog.
//
// Benchmarks and additional discussion live in the project README.md and the
// dedicated benchmarking module under benchmark/.
package pslog
