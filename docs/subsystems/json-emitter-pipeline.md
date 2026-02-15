# JSON Emitter Pipeline

## Purpose

This subsystem emits structured JSON log lines in plain and colorized variants with precomputed key payloads, optimized runtime type handling, and specialized emit functions for timestamp/loglevel/static-field combinations.

It is not responsible for environment resolution, output writer lifecycle policy, or CLI conversion.

## Architecture and Programmatic Details

### Entry Points

- Logger constructors: `newJSONPlainLogger` (`json_plain.go:43`) and `newJSONColorLogger` (`json_color.go:35`).
- Emit function dispatch: `selectJSONPlainEmit` (`json_plain.go:177`) and `selectJSONColorEmit` (`json_color.go:169`).
- Runtime value writers and fast paths: `writeRuntimeValue*Inline` and `writePTLogValue*` in `json_runtime.go:52`.
- Escape engine: `appendEscapedStringContent` in `json_escape.go:8`.

### Core Types and Interfaces

- `jsonPlainLogger` and `jsonColorLogger` store:
  - base config/fields,
  - prebuilt key byte payloads (`ts/lvl/msg/loglevel`),
  - cached static payload bytes,
  - `lineHint`,
  - selected `emit` function pointer (`json_plain.go:10`, `json_color.go:13`).
- `TrustedString` allows bypassing escape scans for known-safe data (`json_runtime.go:23`).

### Control and Data Flow

1. Constructor resolves key names (`ts/lvl/msg` vs verbose names) and precomputes key payload bytes.
2. `log` checks level, appends caller field when configured, acquires pooled writer, preallocates from hint, and invokes selected emit function.
3. Emit function writes envelope (`{...}`), static payload, and runtime fields.
4. Runtime value writers handle common primitives inline, fallback to generic JSON marshaling for uncommon types.

### Invariants and Error Handling

- Field order is stable: timestamp/level/message first, then static fields, runtime fields, optional `loglevel`.
- Non-finite floats (`NaN`/`Inf`) are policy-driven (`Options.NonFiniteFloatPolicy`):
  - default: JSON strings (`"NaN"`, `"+Inf"`, `"-Inf"`),
  - optional: `null` (`json_values.go`, `non_finite_float_policy.go`).
- Colorized JSON shares logical schema but embeds ANSI color escape sequences around keys/values.

### Test and Observability Coverage

- Coverage exists for JSON emission/runtime/escape/parity:
  - `json_emit_test.go`,
  - `json_runtime_test.go`,
  - `json_escape_test.go`,
  - `json_parity_test.go`,
  - `json_keys_test.go`,
  - `json_policy_parity_test.go`,
  - `json_runtime_parity_test.go`,
  - `alloc_regression_test.go`.
- Gap: no fuzz/property suite that exhaustively randomizes all emitter variant combinations.

## Quality Improvements (Non-Style, Non-New-Feature)

1. Add differential parity tests across duplicated emitters
   - Status: Done (targeted parity coverage).
   - Behavior: parity tests now compare plain/color behavior across runtime and emitter variants, including slow-path cases.
   - Verification: `json_policy_parity_test.go`, `json_runtime_parity_test.go`.
2. Make non-finite float policy explicit and testable
   - Status: Done.
   - Behavior: `Options.NonFiniteFloatPolicy` controls serialization for base and runtime fields in plain/color JSON emitters.
   - Verification: `json_policy_parity_test.go` plus allocation guardrails in `alloc_regression_test.go`.
3. Remove or implement the currently no-op JSON escape configuration hook
   - Problem: JSON constructors call `configureJSONEscapeFromOptions`, but the function is empty.
   - Evidence: calls in `json_plain.go:52`, `json_color.go:44`; no-op implementation in `json_escape_config.go:1`.
   - Impact: Latent correctness/operability risk if callers assume options affect escape behavior.
   - Fix direction: either implement a real option-driven behavior or remove the hook to eliminate false affordance.
   - Verification: constructor-level tests proving configured behavior (or compile-time/behavioral simplification if removed).

## Feature Improvements (Optional, Aligned to Existing Feature Set)

- Add optional typed-object fast path for common map payloads (`map[string]any`) when callers pass prevalidated structured values.

## References

- `json_plain.go:10`
- `json_plain.go:43`
- `json_plain.go:86`
- `json_plain.go:177`
- `json_color.go:13`
- `json_color.go:35`
- `json_color.go:78`
- `json_color.go:169`
- `json_runtime.go:23`
- `json_runtime.go:52`
- `json_escape.go:8`
- `json_values.go:43`
- `json_escape_config.go:1`
- `json_emit_test.go`
- `json_parity_test.go`
