# pslogconsole2json CLI

## Purpose

This subsystem converts pslog console output (plain or colored) into NDJSON with optional LQL filtering and optional file output.

It is responsible for parsing console tokens, timestamp/level normalization, heuristic value typing, and JSON re-emission order.
It is not responsible for generating logs or maintaining the core `pslog` library runtime.

## Architecture and Programmatic Details

### Entry Points

- Process entrypoint and flag parsing: `main` in `pslogconsole2json/main.go:286`.
- Reader pipeline: `processReader` in `pslogconsole2json/main.go:384`.
- Parse pipeline: `parseConsoleLine` in `pslogconsole2json/main.go:412`.
- Timestamp parsing: `parseTimestampPrefix`, `parseEpoch`, `parseDTG` in `pslogconsole2json/main.go:711`.
- Output re-encoding: `writeOrderedJSON` in `pslogconsole2json/main.go:852`.

### Core Types and Interfaces

- `typeTracker` remembers per-key inferred kind for typed parsing (`pslogconsole2json/main.go:45`).
- `selectorFilter` wraps LQL selector logic and runtime filter matching (`pslogconsole2json/main.go:155`).
- `fieldPair` carries parsed key/value pairs preserving parse order (`pslogconsole2json/main.go:532`).

### Control and Data Flow

1. Parse flags, validate combinations (`-i`, `-o`, `-outdir`, `-l`, `-or`).
2. Build selector filter and initialize type tracker.
3. For each line:
   - strip ANSI,
   - parse timestamp prefix (epoch/DTG/layout table),
   - parse level prefix,
   - split message vs field suffix,
   - parse fields and typed values,
   - apply filter,
   - emit ordered JSON object (`ts`, `lvl`, `msg`, fields...).

### Invariants and Error Handling

- Invalid lines and parse errors are warned to stderr and skipped (`pslogconsole2json/main.go:392` to `pslogconsole2json/main.go:395`).
- Parser is permissive: malformed tail content in `parseFields` can end parsing without explicit error (`pslogconsole2json/main.go:551` to `pslogconsole2json/main.go:553`).
- Type inference currently persists for all processed input paths in a single invocation (`pslogconsole2json/main.go:325`, `pslogconsole2json/main.go:340`).

### Test and Observability Coverage

- There are no tests in this module currently (`pslogconsole2json/` contains `main.go`, `README.md`, `go.mod`, `go.sum` only).
- Observability is stderr warnings via `warnf` (`pslogconsole2json/main.go:902`).

## Quality Improvements (Non-Style, Non-New-Feature)

1. Isolate type inference state per input stream
   - Problem: a single `typeTracker` instance is reused for all input files, so field types observed in one file influence parsing in subsequent files.
   - Evidence: tracker initialized once (`pslogconsole2json/main.go:325`) and passed to each `processFile` call in loop (`pslogconsole2json/main.go:335`, `pslogconsole2json/main.go:340`).
   - Impact: Data consistency risk when heterogeneous logs are processed together.
   - Fix direction: create per-file tracker (or optional global-tracker mode) and document behavior.
   - Verification: multi-file tests with conflicting key types asserting independent outputs.
2. Correct DTG date normalization semantics
   - Problem: DTG parser constructs date with current year/month and token day; invalid day/month combinations silently roll over via `time.Date`.
   - Evidence: `parseDTG` checks day `1..31` only, then builds with `time.Date` (`pslogconsole2json/main.go:793` to `pslogconsole2json/main.go:801`).
   - Impact: Correctness risk (wrong timestamp month/day) near month boundaries.
   - Fix direction: validate day against actual month length and introduce bounded rollover strategy for operational logs crossing month transitions.
   - Verification: unit tests for short-month invalid days and month-boundary rollover expectations.
3. Fail loud on malformed field tails instead of silent truncation
   - Problem: malformed field tokens currently terminate parsing without explicit error in several branches.
   - Evidence: `parseFields` `break` paths at `pslogconsole2json/main.go:551`, `pslogconsole2json/main.go:557`.
   - Impact: Reliability risk from silent partial ingestion.
   - Fix direction: return explicit parse errors for malformed key/value segments and include line/offset in warning output.
   - Verification: parser unit tests with malformed tokens asserting warning/error behavior and predictable partial output policy.
4. Build comprehensive tests for parser and pipeline
   - Problem: module has no executable verification signal today.
   - Evidence: no `*_test.go` files under `pslogconsole2json/`.
   - Impact: High regression risk for parsing and filter semantics.
   - Fix direction: add unit tests for tokenizer/parsers and integration tests for end-to-end conversion/filtering.
   - Verification: `cd pslogconsole2json && go test ./...` plus fixture-based smoke tests in CI.

## Feature Improvements (Optional, Aligned to Existing Feature Set)

- Add `--strict` mode that exits non-zero on first parse error instead of warn-and-continue, for ingestion pipelines requiring fail-fast behavior.

## References

- `pslogconsole2json/main.go:286`
- `pslogconsole2json/main.go:325`
- `pslogconsole2json/main.go:340`
- `pslogconsole2json/main.go:384`
- `pslogconsole2json/main.go:412`
- `pslogconsole2json/main.go:537`
- `pslogconsole2json/main.go:711`
- `pslogconsole2json/main.go:781`
- `pslogconsole2json/main.go:852`
- `pslogconsole2json/main.go:902`
- `pslogconsole2json/README.md`
