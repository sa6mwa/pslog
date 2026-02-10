# Runtime Primitives

## Purpose

This subsystem provides shared low-level components used by all logger variants: pooled line buffering, timestamp caching, caller discovery, terminal detection, output closing, and ANSI palette primitives.

It is responsible for runtime behavior and resource lifecycle.
It is not responsible for API-level option parsing or mode-specific envelope format decisions.

## Architecture and Programmatic Details

### Entry Points

- Buffered writer primitives: `acquireLineWriter`, `releaseLineWriter`, `flush` in `writer.go:67`.
- Timestamp cache: `newTimeCache`, `Current`, `refresh` in `timecache.go:65`.
- Caller extraction: `callerFunctionName` in `currentfn.go:62`.
- Terminal probe: `isTerminal` in `terminal.go:11`.
- Palette mutation: `ansi.SetPalette` in `ansi/ansi.go:86`.
- Output close helper: `closeOutput` in `logger_close.go:8`.

### Core Types and Interfaces

- `lineWriter` encapsulates reusable byte buffer and small literal caches (`writer.go:17`).
- `timeCache` stores per-layout formatted time and a background ticker refresh loop (`timecache.go:44`).
- `tickerControl` abstracts ticker channel and stop function for testing (`timecache.go:54`).
- `ansi.Palette` and global semantic color vars define color lookup state (`ansi/ansi.go:51`, `ansi/ansi.go:30`).

### Control and Data Flow

1. Loggers acquire a pooled `lineWriter` for each entry, encode data into `buf`, then flush and release.
2. For cacheable layouts, timestamp retrieval calls `timeCache.Current`, which lazily starts a refresher goroutine.
3. Colored emitters read ANSI semantic globals at write time (for example `ansi.Key`, `ansi.Message`, `ansi.Timestamp`).
4. Close behavior delegates to `closeOutput`, which closes any non-stdout/stderr writer implementing `Close`.

### Invariants and Error Handling

- `lineWriter` buffer capacity is bounded by reset logic (`writer.go:78` to `writer.go:82`) but write failures are currently ignored.
- `timeCache` refresh loop assumes long-lived process lifecycle and lacks explicit shutdown.
- ANSI palette state is process-global mutable state.

### Test and Observability Coverage

- Timestamp cache behavior and cacheability are tested in `timecache_test.go:9`.
- Terminal probing has OS-specific tests in `internal/istty/*_test.go`.
- Gaps:
  - no goroutine lifecycle test for `timeCache` teardown,
  - no race-oriented test around concurrent palette mutation and emission,
  - no tests asserting behavior under writer failures.

## Quality Improvements (Non-Style, Non-New-Feature)

1. Add a stoppable lifecycle for `timeCache` refresh goroutines
   - Problem: `Current` launches `go c.refresh()`, and `refresh` ranges ticker channel indefinitely.
   - Evidence: goroutine start in `timecache.go:87`; loop in `timecache.go:101`; default ticker stop does not close channel (`timecache.go:75` to `timecache.go:80`).
   - Impact: Reliability/resource leak risk when many short-lived loggers are created.
   - Fix direction: add cancellation (context or done channel), call stop on close, and wire logger close to cache shutdown.
   - Verification: goroutine-count/lifecycle tests plus deterministic fake ticker tests.
2. Make ANSI palette access concurrency-safe
   - Problem: color globals are mutable process-wide vars updated without synchronization.
   - Evidence: mutable globals in `ansi/ansi.go:30`; unsynchronized writes in `ansi/ansi.go:86`.
   - Impact: Data race risk and inconsistent color output under concurrent `SetPalette` + logging.
   - Fix direction: move to immutable palette snapshot with atomic pointer swap, and let loggers capture palette references safely.
   - Verification: `-race` tests with concurrent logging and palette updates.
3. Define and implement write error handling semantics in runtime flush
   - Problem: flush drops write errors.
   - Evidence: `writer.go:287` ignores write error.
   - Impact: Silent log loss with no observability.
   - Fix direction: return/record errors from flush and expose a user-visible policy (report hook/metric/callback).
   - Verification: failing-writer tests asserting deterministic behavior and no panic.

## Feature Improvements (Optional, Aligned to Existing Feature Set)

- Add lightweight internal metrics counters (dropped writes, cache refreshes, pool misses) for operability in high-throughput services.

## References

- `writer.go:17`
- `writer.go:67`
- `writer.go:280`
- `timecache.go:44`
- `timecache.go:83`
- `timecache.go:95`
- `currentfn.go:62`
- `terminal.go:11`
- `logger_close.go:8`
- `ansi/ansi.go:30`
- `ansi/ansi.go:86`
- `timecache_test.go:9`
