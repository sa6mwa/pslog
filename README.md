# pslog

pslog is a zero-allocation logging toolkit for Go that targets fast, structured
output without sacrificing ergonomics. It ships compact JSON and console
encoders, colourised variants, and a palette system that can be adjusted at
runtime. The project now centres on a minimal-branch design: every logger
variant carries its own hand-inlined hot path so the CPU executes as few
instructions as possible.

![Elevator pitch](elevatorpitch/elevatorpitch.gif)

## Design philosophy

Beside being colorful...

![Demo](examples/demo/demo.gif)

The latest implementation is guided by a few pragmatic rules:

- **Do the work at construction** – adapters precompute everything they can.
  Field metadata (including whether keys are trusted) is cached once, colour
  emitters are selected up-front, and the timestamp formatter is resolved during
  logger creation.
- **Inline the hot path** – JSON, JSON+colour, console, and console+colour each
  have their own copy of the emission logic. That duplication is intentional;
  avoiding shared helpers removes branches and keeps the CPU on a single-track
  code path.
- **Chunk-aware scanning** – JSON escapes scan 16-byte blocks at a time,
  copying safe spans in one go before emitting escapes. Console quoting now uses
  the same guard.
- **`[]any` compatibility** – pslog keeps the familiar variadic API (`logger.Log
  (level, msg, "k", "v", ...)`). The encoder specialises the most common types
  (strings, numbers, bools, time) so the dynamic type switch collapses to a few
  fast cases, but the interface stays ergonomic.
- **Trusted strings, not caches** – once a key or value is known safe it is
  wrapped in `pslog.TrustedString`, letting the emitters bypass the escape loop
  entirely. There is no global cache: each caller controls when to promote data.

These choices mirror the obvious path to reduce cycles (inline, avoid switches,
avoid shared helpers). They mean the code base contains deliberate duplication,
but the payoff is simpler instruction streams.

### Time handling

Formatted timestamps are cached per layout using the `timeCache`. Each adapter
decides once whether the layout is trusted; if it is, timestamps are emitted
without additional scanning. This keeps readable RFC3339 logs at the same cost
as unix-epoch timestamps elsewhere.

### Differences from other loggers

While the shape resembles other high-performance toolkits, pslog keeps a few
distinct traits:

- **Chunked escaper** – the escape guard scans in 16-byte chunks. Some loggers
  walk byte-by-byte; pslog jumps over entire safe spans.
- **`[]any` first** – pslog keeps the variadic logging API. There is no builder
  type required to reach the fast path, although you can pre-promote fields if
  you want to.
- **Inline duplication** – every encoder (JSON/plain/colour, console/plain/
  colour) carries its own hot path. There is no runtime branch between modes.

## Structured logging helpers

- `logger.With(...)` remains the preferred way to attach static fields. The
  elevator pitch benchmark now uses this pattern for pslog’s JSON encoders,
  matching real-world deployments.
- `pslog.Keyvals(...)` is available for performance-conscious code that wants to
  pre-promote runtime keyvals before calling `Log/Info/Debug/...`. It returns a
  slice of key/value pairs with trusted strings already tagged.

> ⚠️ **Fairness note:** Benchmarks labelled `json+keyvals` or `jsoncolor+keyvals`
> pre-promote *all* key/value pairs before the run. That eliminates the escape
> scans the other loggers still perform, so only the standard `json`,
> `json+with`, and their colour counterparts should be used for apples-to-apples
> comparisons.

## Write-failure observability (opt-in)

By default, pslog keeps write-failure handling out of the hot path. If you need
explicit observability for downstream writer failures, wrap your destination
with `ObservedWriter`.

```go
obs := pslog.NewObservedWriter(os.Stdout, func(f pslog.WriteFailure) {
	// Hook this into metrics/alerts.
	// f.Err, f.Written, f.Attempted
})

logger := pslog.NewWithOptions(context.Background(), obs, pslog.Options{Mode: pslog.ModeStructured})
logger.Info("ready")

stats := obs.Stats() // cumulative Failures + ShortWrites
_ = stats
```

This behavior is intentionally opt-in so default logger construction keeps the
lowest overhead profile.

## Benchmarking

The benchmark suite lives under the `benchmark/` module. Typical commands:

```bash
go test ./benchmark -bench=. -run=^$ -benchmem
```

Observed-writer comparison helpers:

```bash
# JSON-only A/B: json vs json+observed
env -u NO_COLOR RUNS=4 BENCHTIME=700ms GOMAXPROCS_VALUE=1 \
  ./benchmark/compare_json_observed.sh

# Full A/B across all pslog production variants
env -u NO_COLOR RUNS=4 BENCHTIME=500ms GOMAXPROCS_VALUE=1 \
  ./benchmark/compare_pslog_observed_ab.sh

# Baseline comparison with host-noise guard + CPU pinning
RUNS=5 BENCHTIME=700ms PIN_CORE=6 \
  MAX_NORMALIZED_LOAD=0.35 MAX_TOP_CPU=75 \
  ./benchmark/compare_pslog_baseline_guarded.sh
```

`compare_pslog_baseline_guarded.sh` refuses to run when host load is too noisy,
then delegates to `compare_pslog_baseline.sh` once the machine is within the
configured thresholds.

Performance gate:

```bash
BASE_COMMIT=97012d4 RUNS=6 BENCHTIME=700ms PIN_CORE=6 \
  MAX_MEDIAN_DELTA_PERCENT=1.00 \
  ./benchmark/perf_gate.sh
```

Policy for core production benchmarks (`BenchmarkPSLogProduction/pslog/production/*`):

- Use commit-vs-commit AB comparisons, not only baseline snapshots.
- Pin benchmark execution to one CPU core (default is the last core when
  `taskset` is available; set `PIN_CORE` explicitly for reproducibility).
- Gate on `median_delta_percent` from `compare_pslog_commits_ab.sh`.
- Treat any benchmark above `+1.00%` median regression as a fail.

The suite logs both `ns/op` and `cycles/op` in `benchmark/results/pslog_json_perf
.csv` so regressions are easier to spot. Additional scripts (including
benchorder and the elevator pitch visualiser) live in the same module.

There is also a benchmark-sorting helper if you want an at-a-glance ranking:

```bash
go run ./cmd/benchorder/ -benchtime 100ms -bench PSLogProduction
=== cpupower frequency-info ===
analyzing CPU 2:
  driver: intel_pstate
  CPUs which run at the same hardware frequency: 2
  CPUs which need to have their frequency coordinated by software: 2
  energy performance preference: performance
  hardware limits: 400 MHz - 5.00 GHz
  available cpufreq governors: performance powersave
  current policy: frequency should be within 400 MHz and 5.00 GHz.
                  The governor "performance" may decide which speed to use
                  within this range.
  current CPU frequency: 2.98 GHz (asserted by call to kernel)
  boost state support:
    Supported: yes
    Active: yes
===============================
Run 1/1 results
BenchmarkPSLogProduction (13 benchmarks, sorted by ns/op)
Rank  Variant                             Time (ns/op)  Bytes/op  B/op  allocs/op
1     pslog/production/json+keyvals       221.2         371.7     0     0
2     phuslu/production/json              234.3         379.7     0     0
3     pslog/production/json+with          240.9         371.7     0     0
4     pslog/production/console            273.5         290.0     0     0
5     pslog/production/json               283.9         371.7     0     0
6     pslog/production/consolecolor       329.2         514.2     0     0
7     zerolog/production/json             343.7         377.7     4     0
8     pslog/production/jsoncolor+keyvals  344.5         622.5     0     0
9     pslog/production/jsoncolor+with     372.8         622.5     0     0
10    pslog/production/jsoncolor          456.2         622.5     0     0
11    zap/production/json                 522.1         340.7     3     0
12    zap/production/console              629.5         350.6     19    1
13    zerolog/production/console          10635         307.8     4310  132
```

> ⚠️ Treat json+keyvals as unfair against the other loggers (see the note above
> regarding pre-promotion). json+with is a realistic configuration because it
> matches how pslog is typically deployed and is a much more fair
> comparison. However, strictly speaking, pure json benchmarks should be used
> for apples-to-apples comparisons.

## Environment configuration

`LoggerFromEnv` builds a logger from environment variables. It applies the
values on top of seeded options and uses the same defaults as `NewWithOptions`
when variables are missing or invalid.

Recognised variables (default prefix `LOG_`):

- `LOG_LEVEL` (`trace|debug|info|warn|error|fatal|panic|no|disabled`)
- `LOG_MODE` (`console|structured|json`)
- `LOG_TIME_FORMAT`
- `LOG_DISABLE_TIMESTAMP` (bool)
- `LOG_NO_COLOR` (bool)
- `LOG_FORCE_COLOR` (bool)
- `LOG_PALETTE` (for example `one-dark`, `synthwave-84`, `doom-nord`)
- `LOG_VERBOSE_FIELDS` (bool)
- `LOG_UTC` (bool)
- `LOG_CALLER_KEYVAL` (bool)
- `LOG_CALLER_KEY`
- `LOG_OUTPUT` (`stdout|stderr|default|/path/to/file.log|stdout+/path|stderr+/path|default+/path`)
- `LOG_OUTPUT_FILE_MODE` (octal permissions for newly-created output files, default `0600`; accepted range `0000`-`0777` with optional `0o` prefix, invalid values fall back to `0600` and emit `logger.output.file_mode.invalid`)

Example:

```go
logger := pslog.LoggerFromEnv(context.Background(),
	pslog.WithEnvOptions(pslog.Options{Mode: pslog.ModeStructured}),
)
logger.Info("ready")
```

Programmatic palette selection uses a pointer:

```go
logger := pslog.NewWithOptions(context.Background(), os.Stdout, pslog.Options{
	Mode:    pslog.ModeStructured,
	Palette: &ansi.PaletteOneDark,
})
logger.Info("ready")
```

JSON non-finite float handling is configurable:

```go
logger := pslog.NewWithOptions(context.Background(), os.Stdout, pslog.Options{
	Mode:                 pslog.ModeStructured,
	NonFiniteFloatPolicy: pslog.NonFiniteFloatAsNull, // or NonFiniteFloatAsString
})
```

## Testing

Run the main suite on the host OS:

```bash
make test
```

Run host tests plus Windows tests under wine (no submodules):

```bash
make test-cross
```

You can override the wine command and verbosity:

```bash
WINE=wine64 WINEDEBUG=-all make test-cross
```

## Credits

pslog is maintained by [sa6mwa](https://github.com/sa6mwa). Contributions are
welcome; feel free to open issues or pull requests.
