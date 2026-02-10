# Console Emitter Pipeline

## Purpose

This subsystem is responsible for formatting log records as console lines in plain and colored variants, including static field payload reuse, runtime key/value emission, and mode-specific hot-path selection.

It is not responsible for logger construction policy, env parsing, or JSON emission.

## Architecture and Programmatic Details

### Entry Points

- Logger constructors: `newConsolePlainLogger` (`console_plain.go:19`) and `newConsoleColorLogger` (`console_color.go:23`).
- Emit dispatch selection: `selectConsolePlainEmit` (`console_plain.go:257`) and `selectConsoleColorEmit` (`console_color.go:279`).
- Runtime field emission: `writeRuntimeConsolePlain*` (`console_plain.go:165`) and `writeRuntimeConsoleColor*` (`console_color.go:182`).

### Core Types and Interfaces

- `consolePlainLogger` and `consoleColorLogger` contain:
  - shared `loggerBase` config/fields,
  - cached static payload bytes (`baseBytes`),
  - `lineHint` estimate for preallocation,
  - selected `emit` function pointer (`console_plain.go:11`, `console_color.go:15`).
- Runtime helpers split into fast and slow paths for key/value encoding (`console_plain.go:172`, `console_plain.go:212`, `console_color.go:189`, `console_color.go:225`).

### Control and Data Flow

1. `log` checks level and adds optional caller field.
2. It acquires pooled `lineWriter`, disables auto flush, optionally preallocates from `lineHint`, and invokes the selected emitter.
3. Emitter writes timestamp/level/message/static fields/runtime fields in deterministic order.
4. Logger flushes line and records updated hint (`console_plain.go:49` to `console_plain.go:66`, `console_color.go:83` to `console_color.go:100`).

### Invariants and Error Handling

- Plain and colored pipelines intentionally duplicate mode-specific emitters for lower branching overhead.
- Fast runtime path accepts only `string`/`TrustedString` keys; any non-string key switches entire entry to slow path (`console_plain.go:179` to `console_plain.go:186`, `console_color.go:196` to `console_color.go:203`).
- Writer errors are not surfaced from console emit path because flush ignores them (runtime primitive behavior).

### Test and Observability Coverage

- Console internals and variants are covered by:
  - `console_plain_internal_test.go`,
  - `console_color_internal_test.go`,
  - `console_variants_test.go`,
  - `console_message_escape_test.go`,
  - `logger_variants_test.go`.
- Gap: no differential fuzz/property tests asserting fast-path and slow-path semantic equivalence under arbitrary key/value shapes.

## Quality Improvements (Non-Style, Non-New-Feature)

1. Prevent persistent over-allocation from one-off long lines
   - Problem: `lineHint` in console loggers only grows (max), so a single pathological line can permanently inflate preallocation for later normal lines.
   - Evidence: `recordHint` only stores when `n > current` (`console_plain.go:68`, `console_color.go:102`), and preallocation consumes this hint (`console_plain.go:56`, `console_color.go:90`).
   - Impact: Performance/memory regression under bursty high-cardinality messages.
   - Fix direction: switch to bounded/decaying estimate (for example EWMA or clamp with upper percentile cap).
   - Verification: targeted benchmark/test that logs one huge line followed by many small lines and asserts allocation profile stabilizes.
2. Add parity tests between fast and slow runtime field encoders
   - Problem: Two independent runtime paths encode equivalent conceptual data; drift can introduce mode-dependent behavior.
   - Evidence: separate implementations in `writeRuntimeConsolePlainFast/Slow` and `writeRuntimeConsoleColorFast/Slow` (`console_plain.go:172`, `console_plain.go:212`, `console_color.go:189`, `console_color.go:225`).
   - Impact: Correctness risk (field rendering inconsistencies) and hard-to-debug regressions.
   - Fix direction: differential tests/fuzzers that feed mixed key/value shapes and compare normalized output for parity.
   - Verification: property tests plus fuzz corpus for odd key counts, non-string keys, and escaped values.
3. Add explicit tests for write failure behavior in console mode
   - Problem: Console pipeline currently has no assertions around downstream writer failure behavior.
   - Evidence: log path flushes through `lineWriter.commit` (`console_plain.go:63`, `console_color.go:97`) and `writer.go:287` currently discards write errors.
   - Impact: Silent log loss in production when destination is unavailable.
   - Fix direction: define contract (`best-effort` vs `observable failure`) and pin behavior with tests.
   - Verification: failing-writer test cases in console mode for plain and colored adapters.

## Feature Improvements (Optional, Aligned to Existing Feature Set)

- Add optional console field ordering mode (`static-before-msg` vs current default) only if required by downstream text parsers.

## References

- `console_plain.go:11`
- `console_plain.go:49`
- `console_plain.go:165`
- `console_plain.go:257`
- `console_color.go:15`
- `console_color.go:83`
- `console_color.go:182`
- `console_color.go:279`
- `writer.go:287`
- `console_plain_internal_test.go`
- `console_color_internal_test.go`
- `console_variants_test.go`
- `logger_variants_test.go`
