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
- Writer close behavior is best-effort via `closeOutput`, with no ownership tracking across cloned loggers (`logger_close.go:8`).

### Test and Observability Coverage

- Strong env configuration coverage in `logger_from_env_test.go:15` and `pslog_fromenv_internal_test.go:12`.
- Gaps:
  - No ownership/lifetime tests for closing shared writers across cloned loggers.
  - No explicit tests for write-failure observability in runtime logging paths.

## Quality Improvements (Non-Style, Non-New-Feature)

1. Make downstream write failures observable
   - Problem: Emission currently swallows writer errors, so callers cannot detect log loss.
   - Evidence: `lineWriter.flush` ignores write result and error in `writer.go:287`.
   - Impact: Reliability/operability risk, especially for file/socket/pipe outputs.
   - Fix direction: propagate `Write` errors through `commit`/`log` path (or emit to an explicit error hook/metric) and make behavior deterministic under failure.
   - Verification: add failing-writer unit tests (for each logger mode) asserting failures are surfaced and do not panic.
2. Add explicit output ownership semantics for `Close`
   - Problem: Cloned loggers share the same underlying writer yet every clone exposes `Close`, allowing accidental early close of shared output.
   - Evidence: cloning keeps writer in copied config (`json_plain.go:107`, `console_plain.go:78`); `Close` delegates directly to `closeOutput` (`json_plain.go:164`, `console_plain.go:135`, `logger_close.go:8`).
   - Impact: Reliability risk (silent write loss after one clone closes shared output).
   - Fix direction: introduce shared output handle with `sync.Once` close and ownership rules (for example root-owning logger only), plus docs for contract.
   - Verification: tests covering clone/close interleavings and post-close write behavior.
3. Tighten default file permissions for env-selected output files
   - Problem: `LoggerFromEnv` creates log files with mode `0644`.
   - Evidence: `openLogOutputFile` uses `os.OpenFile(..., 0o644)` in `pslog_fromenv.go:218`.
   - Impact: Security exposure for logs that may contain sensitive values.
   - Fix direction: default to restrictive permissions (for example `0600`) with explicit opt-out for shared-read deployments.
   - Verification: unit tests asserting created mode bits and backward-compatibility behavior when opt-out is enabled.

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
