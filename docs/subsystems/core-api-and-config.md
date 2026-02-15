# Core API and Config

## Purpose

This subsystem defines the public `pslog` API surface (`Logger`, `Base`, constructors, context helpers, stdlib adapters) and the configuration path that builds concrete logger variants from explicit options or environment variables.

It is responsible for API semantics, option resolution, and output target selection.
It is not responsible for low-level encoding internals (console/json emission loops) or runtime primitive internals (buffer pool lifecycle, timestamp refresher lifecycle).

## Architecture and Programmatic Details

### Entry Points

- Constructors: `New`, `NewStructured`, `NewStructuredNoColor`, `NewWithOptions` in `pslog.go:197`.
- Env-based construction: `LoggerFromEnv` and option functions in `pslog_fromenv.go:11`.
- Context bridge: `ContextWithLogger`, `LoggerFromContext`, `Ctx` in `pslog.go:231`.
- stdlib bridge: `LogLogger`, `LogLoggerWithLevel` in `pslog.go:290`.

### Core Types and Interfaces

- `Logger` and `Base` interfaces define call contracts for applications and libraries (`pslog.go:16`, `pslog.go:30`).
- `Options` defines construction-time behavior (mode, levels, timestamps, caller fields, color) (`pslog.go:162`).
- `coreConfig` and `loggerBase` carry resolved config and inherited fields for concrete logger implementations (`logger_core.go:93`, `logger_core.go:164`).
- `teeWriter` multiplexes output for env `OUTPUT` tee forms (`output_writer.go:5`).

### Control and Data Flow

1. User calls constructor or `LoggerFromEnv`.
2. `LoggerFromEnv` overlays env values on seeded `Options`, resolves writer (stdout/stderr/file/tee), and logs fallback errors when file opening fails (`pslog_fromenv.go:48`, `pslog_fromenv.go:111`, `pslog_fromenv.go:123`).
3. `buildAdapter` resolves mode/defaults, color enablement, timestamp strategy, and caller metadata, then dispatches to one of 4 concrete emitters (`pslog.go:306` to `pslog.go:379`).
4. `With`/`WithLogLevel`/`LogLevel` on concrete loggers clone config and static fields rather than mutating the receiver (for example `json_plain.go:107`, `console_plain.go:78`).

### Invariants and Error Handling

- Missing/invalid env values generally preserve seeded options (for example parse checks in `pslog_fromenv.go:61`, `pslog_fromenv.go:81`, `pslog_fromenv.go:91`).
- `LoggerFromEnv` falls back to base writer on output open failure and emits a structured error event (`pslog_fromenv.go:115`, `pslog_fromenv.go:123`).
- Writer close behavior uses explicit ownership semantics:
  - user-provided writers are not closed by logger `Close`,
  - env-opened outputs are wrapped as owned outputs and closed once,
  - shared runtime components (for example `timeCache`) are closed by owning logger only (`logger_close.go`, `owned_output.go`).

### Test and Observability Coverage

- Strong env configuration coverage in `logger_from_env_test.go:15` and `pslog_fromenv_internal_test.go:12`.
- Close ownership and shared-runtime close semantics are covered in:
  - `close_ownership_test.go`,
  - `observed_writer_test.go`,
  - `timecache_test.go`.
- Gaps:
  - No explicit tests for write-failure observability in runtime logging paths.

## Quality Improvements (Non-Style, Non-New-Feature)

1. Make downstream write failures observable
   - Problem: Emission currently swallows writer errors, so callers cannot detect log loss.
   - Evidence: `lineWriter.flush` ignores write result and error in `writer.go:287`.
   - Impact: Reliability/operability risk, especially for file/socket/pipe outputs.
   - Fix direction: propagate `Write` errors through `commit`/`log` path (or emit to an explicit error hook/metric) and make behavior deterministic under failure.
   - Verification: add failing-writer unit tests (for each logger mode) asserting failures are surfaced and do not panic.
2. Add explicit output ownership semantics for `Close`
   - Status: Done.
   - Behavior:
     - `closeLoggerRuntime` enforces owner-only `timeCache` shutdown.
     - owned outputs use a `sync.Once`-guarded closer.
     - `Close` on clone no longer tears down shared runtime owned by root logger.
   - Verification: `close_ownership_test.go`, `observed_writer_test.go`, `timecache_test.go`.
3. Tighten default file permissions for env-selected output files
   - Status: Done. `LoggerFromEnv` now defaults new output files to mode `0600`.
   - Behavior: `OUTPUT_FILE_MODE` controls file creation mode for env-output paths.
   - Guardrail: invalid `OUTPUT_FILE_MODE` values fall back to `0600` and emit `logger.output.file_mode.invalid`.
   - Verification: unit tests in `pslog_fromenv_internal_test.go` and `logger_from_env_test.go` validate default and override behavior.

## Feature Improvements (Optional, Aligned to Existing Feature Set)

- Add an explicit constructor option for write-failure policy (`drop`, `report`, `panic`) so service owners can choose strictness without changing call sites.

## References

- `pslog.go:16`
- `pslog.go:162`
- `pslog.go:197`
- `pslog.go:306`
- `logger_core.go:93`
- `pslog_fromenv.go:48`
- `pslog_fromenv.go:155`
- `output_writer.go:5`
- `logger_close.go:8`
- `logger_from_env_test.go:15`
- `pslog_fromenv_internal_test.go:12`
