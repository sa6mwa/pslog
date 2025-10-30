# pslog

pslog is a zero-allocation logging toolkit for Go that focuses on fast, readable structured output.
It provides compact JSON and console encoders, colourised variants for development, and a palette
system that can be customised at runtime.

![Elevator pitch](elevatorpitch/elevatorpitch.gif)

The repository includes an animated “elevatorpitch” TUI that continuously benchmarks pslog alongside
other loggers.

## Highlights

- ⚡ **Tightly optimised encoders** – no runtime branching between colour/plain variants and a single
  buffer per log line keep the hot path under 300 ns on modern CPUs.
- 🕒 **Formatted timestamps without the cost** – a `timeCache` pre-formats RFC3339 timestamps once per
  second, so readable time stamps cost the same as unix-epoch logging elsewhere.
- 🎨 **Console & JSON colour palettes** – ship with synthwave, outrun, tokyo night, gruvbox, nord,
  solarized, catppuccin, and more. You can switch palettes at runtime via `ansi.SetPalette` or by
  using the elevatorpitch palette picker.
- 📦 **Single, focused API** – the `Logger` interface exposes `Log`, `With`, `WithLogLevel`, and
  context helpers, keeping integration simple.
- 🔗 **Bridges for the stdlib** – wrap any pslog logger with `LogLogger` to obtain a `*log.Logger`
  without sacrificing the fast path for structured logging.

## Installation

```bash
go get pkt.systems/pslog@latest
```

## Quick start

```go
package main

import (
	"os"

	"pkt.systems/pslog"
)

func main() {
	logger := pslog.NewStructured(os.Stdout).With("env", "production")
	logger.Log(pslog.InfoLevel, "ready", "port", 8080)
	// Default minimum level is Debug, so this should not print
	logger.Trace("without a trace", "visible", false)
	// But this should show
	logger.Debug("ping", "ready", true)
	// Print FatalLevel message and call os.Exit(1)
	logger.Fatal("Error", "error", fmt.Errorf("out of coffee"))
}
```

For console output with colours:

```go
logger := pslog.New(os.Stdout)
logger = logger.With("service", "checkout")
logger.Log(pslog.WarnLevel, "cache bust", "key", pslog.NewTrustedString("user:42"))
```

More extensive examples live in the [examples/](examples) directory of the repository.

## Key concepts

- **Single buffer line writer** – a `lineWriter` manages the scratch buffer, ensuring that fields are
  emitted in one pass without intermediate allocations.
- **Trusted strings** – once a key or value is marked safe, pslog can bypass the JSON escaper and copy
  the bytes directly into the output.
- **Dedicated colour pipelines** – JSON/plain and colour encoders are different implementations. There
  is no `if colour { … } else { … }` inside the hot path; the selection happens once during
  construction.
- **Time cache** – formatted timestamps are cached per-second (and per-layout), so readable timestamps
  are not a performance penalty.

## Benchmarks

Numbers below were produced with `go test ./benchmark -bench=. -run=^$ -benchmem` on a 13th Gen
Intel® Core™ i7-1355U. See the [benchmark module](benchmark) for the full suite and scripts.

### Production dataset (structured vs console)

| Logger | Variant         | ns/op | bytes/op | allocs |
|--------|-----------------|-------|----------|--------|
| pslog  | JSON            | 247.8 |    372.1 |      0 |
| pslog  | Console         | 273.4 |    306.2 |      0 |
| pslog  | Console (colour)| 309.7 |    529.8 |      0 |
| phuslu | JSON            | 310.6 |    380.1 |      0 |
| zerolog| JSON            | 459.4 |    378.1 |      0 |
| pslog  | JSON (colour)   | 518.7 |    622.2 |      0 |
| zap    | JSON            | 653.2 |    341.1 |      0 |
| zap    | Console         | 765.5 |    351.0 |      1 |
| zerolog| Console         | 17685 |    307.7 |    131 |

### Formatted time vs unix epoch

| Logger | Variant | ns/op | bytes/op | allocs |
|--------|---------|-------|----------|--------|
| pslog  | Console         | 126.7 | 149.9 | 0 |
| pslog  | JSON            | 238.6 | 356.9 | 0 |
| phuslu | JSON            | 307.4 | 364.9 | 0 |
| zerolog| JSON            | 440.5 | 364.9 | 0 |
| zap    | JSON            | 718.7 | 358.9 | 0 |

These benches focus on *formatted* timestamps, which reflects real-world deployments where you want
human-readable time in logs. pslog’s cached formatter keeps the cost on par with unix-epoch logging.
Additional results (collector hot paths, colour variants, etc.) are available in the benchmark
module output.

### Complete benchmark

This is one of the benchmarks available under the `benchmark/` directory.

```console
go run ./cmd/benchorder/ -benchtime 100ms -bench FormattedTime
=== cpupower frequency-info ===
analyzing CPU 10:
  driver: intel_pstate
  CPUs which run at the same hardware frequency: 10
  CPUs which need to have their frequency coordinated by software: 10
  energy performance preference: performance
  hardware limits: 400 MHz - 3.70 GHz
  available cpufreq governors: performance powersave
  current policy: frequency should be within 400 MHz and 3.70 GHz.
                  The governor "performance" may decide which speed to use
                  within this range.
  current CPU frequency: 3.50 GHz (asserted by call to kernel)
  boost state support:
    Supported: yes
    Active: yes
===============================
Run 1/1 results
BenchmarkLoggerFormattedTime (26 benchmarks, sorted by ns/op)
Rank  Variant             Time (ns/op)  Bytes/op  B/op  allocs/op
1     pslog/console       103.2         149.9     0     0
2     pslog/consolecolor  119.3         252.2     0     0
3     pslog/json          182.0         356.9     0     0
4     phuslu/json         222.2         364.9     0     0
5     zerolog/json        340.1         364.9     6     0
6     zerolog/global      352.8         364.9     6     0
7     pslog/jsoncolor     372.6         600.0     0     0
8     zap/json            539.0         358.9     5     0
9     zap/console         669.8         351.9     69    3
10    onelog/json         932.1         357.9     227   9
11    seelog/custom       1913          264.6     2002  24
12    glog/text           2752          320.6     1250  19
13    log15/logfmt        2978          187.5     2018  24
14    klog/text           3084          339.8     1878  19
15    charm/json          3103          360.8     1107  37
16    apex/text           3291          376.0     2401  25
17    zap/sugar           3663          756.7     2390  14
18    apex/json           3667          390.7     2619  33
19    kitlog/json         3869          353.9     2611  45
20    slog/json           3938          561.2     2631  37
21    logrus/text         4387          313.7     3188  36
22    logrus/json         5114          360.9     3863  49
23    logr/funcr          6538          573.7     5532  78
24    tint/console        7044          447.4     3025  69
25    zerolog/console     10477         295.2     4221  129
26    charm/console       34196         296.4     2809  151
```

## Colour and palette support

Colour output is handled by the `ansi` subpackage. You can swap palettes at runtime:

```go
ansi.SetPalette(ansi.PaletteDoomDracula)
logger := pslog.NewStructured(os.Stdout)
logger = logger.WithLogLevel().With("component", "tui")
logger.Log(pslog.InfoLevel, "palette swapped")
```

## Development & testing

- Run unit tests: `go test ./...`
- Run the benchmark suite through benchorder: `cd benchmark && go run ./cmd/benchorder -h`
- Run the benchmark suite: `go test ./benchmark -bench=. -run=^$ -benchmem`
- Run the elevator pitch visualiser: `cd elevatorpitch && go run .`

## Credits

pslog is maintained by [sa6mwa](https://github.com/sa6mwa). Contributions are welcome; please file
issues or pull requests on GitHub.
