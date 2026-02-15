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
  - `logger_variants_test.go`,
  - `console_runtime_parity_test.go`.
- Gap: no fuzz/property suite that randomizes fast/slow path parity across arbitrary key/value shapes.

## Quality Improvements (Non-Style, Non-New-Feature)

1. Prevent persistent over-allocation from one-off long lines
   - Status: Done.
   - Behavior: `lineHint` uses bounded/decaying updates (`updateLineHint`) instead of max-only growth, preventing persistent preallocation inflation after outliers.
   - Verification: `console_hint_test.go`.
2. Add parity tests between fast and slow runtime field encoders
   - Status: Done (targeted parity coverage).
   - Behavior: explicit parity tests validate wrapper/fast/slow output equivalence for plain and color runtime field encoders.
   - Verification: `console_runtime_parity_test.go`.
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
