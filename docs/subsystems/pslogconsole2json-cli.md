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
2. Build selector filter.
3. For each line:
   - strip ANSI,
   - parse timestamp prefix (epoch/DTG/layout table),
   - parse level prefix,
   - split message vs field suffix,
   - parse fields and typed values,
   - apply filter,
   - emit ordered JSON object (`ts`, `lvl`, `msg`, fields...).

`typeTracker` is scoped per `processReader` stream.

### Invariants and Error Handling

- Invalid lines and parse errors are warned to stderr and skipped (`pslogconsole2json/main.go:392` to `pslogconsole2json/main.go:395`).
- Parser now returns explicit errors for malformed field tails (`parseFields`), and the reader pipeline surfaces those as line warnings.
- Type inference is isolated per input stream (`typeTracker` created in `processReader`).

### Test and Observability Coverage

- Coverage now exists for parser primitives and end-to-end conversion:
  - `main_test.go` unit tests for `typeTracker`, DTG parsing, field parsing, line parsing, filtering, ordered JSON output.
  - fixture-backed integration tests using `testdata/generated_console_matrix.txt`.
- Observability is stderr warnings via `warnf` (`pslogconsole2json/main.go:902`).

## Quality Improvements (Non-Style, Non-New-Feature)

1. Isolate type inference state per input stream
   - Status: Done.
   - Behavior: `typeTracker` is created per `processReader` invocation.
   - Verification: `TestProcessReader` in `main_test.go`.
2. Correct DTG date normalization semantics
   - Status: Done.
   - Behavior: DTG parser validates day against actual month length before `time.Date` construction.
   - Verification: `TestParseDTG` in `main_test.go`.
3. Fail loud on malformed field tails instead of silent truncation
   - Status: Done.
   - Behavior: malformed field tails now return explicit parse errors from `parseFields`.
   - Verification: `TestParseFields` and `TestParseConsoleLine` in `main_test.go`.
4. Build comprehensive tests for parser and pipeline
   - Status: Done.
   - Behavior: module now includes unit + fixture-backed integration coverage.
   - Verification: `pslogconsole2json/main_test.go`.

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
