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
2. For cacheable layouts, logger construction creates a `timeCache` and starts its refresher loop.
3. Colored emitters use logger-held palette pointers resolved at construction (`Options.Palette` or `ansi.PaletteDefault`), avoiding global ANSI lookups on hot write paths.
4. Close behavior routes through runtime ownership helpers so shared caches are shutdown by owners and owned outputs are closed once.

### Invariants and Error Handling

- `lineWriter` buffer capacity is bounded by reset logic (`writer.go:78` to `writer.go:82`) but write failures are currently ignored.
- `timeCache` supports explicit shutdown via `Close`; shutdown is idempotent and wired to logger close ownership rules.
- ANSI package-level palette values remain mutable global state, but active loggers use explicit palette references and `ansi.SetPalette` writes are lock-protected.

### Test and Observability Coverage

- Timestamp cache behavior and cacheability are tested in `timecache_test.go:9`.
- Terminal probing has OS-specific tests in `internal/istty/*_test.go`.
- Cache lifecycle and owner-close semantics are covered in `timecache_test.go` and `close_ownership_test.go`.
- Concurrent palette swap under active logging is covered in `palette_race_test.go`.
- Gaps:
  - no tests asserting behavior under writer failures.

## Quality Improvements (Non-Style, Non-New-Feature)

1. Add a stoppable lifecycle for `timeCache` refresh goroutines
   - Status: Done.
   - Behavior: `timeCache` has explicit stop channel + idempotent `Close`, and logger runtime close wires owner-only cache shutdown.
   - Verification: `timecache_test.go` lifecycle and concurrent-close coverage.
2. Make ANSI palette access concurrency-safe
   - Status: Done.
   - Behavior: `ansi.SetPalette` uses locking and loggers resolve/store palette pointers at construction, so runtime emission avoids global palette mutation reads.
   - Verification: concurrent swap test in `palette_race_test.go`.
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
