GO ?= go
QEMU_AARCH64 ?= qemu-aarch64
QEMU_I386 ?= qemu-i386

# Modules that depend on the root via replace directives.
BENCHMARK_DIR   := benchmark
EXAMPLES_DIR    := examples
ELEVATOR_DIR    := elevatorpitch

.PHONY: all check test test-benchmark test-examples test-elevatorpitch \
        build-examples build-elevatorpitch bench benchorder benchorder-cross \
        benchorder-arm64 benchorder-386 fuzz tidy download upgrade clean cross-build

all: check

# Core module tests
test:
	$(GO) test -cover -count=1 -race ./...

# Submodule checks (honour their replace directives by running in place)
test-benchmark:
	cd $(BENCHMARK_DIR) && $(GO) test ./...

test-examples:
	cd $(EXAMPLES_DIR) && $(GO) test ./...

test-elevatorpitch:
	cd $(ELEVATOR_DIR) && $(GO) test ./...

# Aggregate verifier
check: test test-benchmark test-examples test-elevatorpitch

# Lightweight builds for binaries
build-examples:
	cd $(EXAMPLES_DIR) && $(GO) build .
	cd $(EXAMPLES_DIR)/demo && $(GO) build .

build-elevatorpitch:
	cd $(ELEVATOR_DIR) && $(GO) build ./...

# Optional: run benchmarks in the benchmark module (can be slow)
bench:
	cd $(BENCHMARK_DIR) && $(GO) test -run '^$$' -bench . -count=1

# Run the benchorder helper with a short benchtime
benchorder:
	cd $(BENCHMARK_DIR) && $(GO) run ./cmd/benchorder -benchtime 100ms

# Cross-arch BenchmarkPSLogProduction via qemu (arm64 and 386), benchtime 100ms,
# executed through benchorder so output is sorted/grouped.
benchorder-cross: benchorder-arm64 benchorder-386

benchorder-arm64:
	cd $(BENCHMARK_DIR) && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 GOFLAGS="-exec=$(QEMU_AARCH64)" \
		$(GO) run ./cmd/benchorder -bench '^BenchmarkPSLogProduction' -benchtime 100ms -cpuinfo=false

benchorder-386:
	cd $(BENCHMARK_DIR) && GOOS=linux GOARCH=386 CGO_ENABLED=0 GOFLAGS="-exec=$(QEMU_I386)" \
		$(GO) run ./cmd/benchorder -bench '^BenchmarkPSLogProduction' -benchtime 100ms -cpuinfo=false

# Discover and run all fuzz tests for 60s each.
fuzz:
	@set -e; \
	pkgs=$$($(GO) list ./...); \
	for pkg in $$pkgs; do \
		fuzzes=$$($(GO) test $$pkg -list '^Fuzz' | grep '^Fuzz' || true); \
		for f in $$fuzzes; do \
			echo "==> $$pkg $$f"; \
			$(GO) test $$pkg -run ^$$ -fuzz ^$$f$$ -fuzztime 60s; \
		done; \
	done

# Keep modules tidy/downloaded after dependency bumps
tidy:
	$(GO) mod tidy
	cd $(BENCHMARK_DIR) && $(GO) mod tidy
	cd $(EXAMPLES_DIR) && $(GO) mod tidy
	cd $(ELEVATOR_DIR) && $(GO) mod tidy

download:
	$(GO) mod download
	cd $(BENCHMARK_DIR) && $(GO) mod download
	cd $(EXAMPLES_DIR) && $(GO) mod download
	cd $(ELEVATOR_DIR) && $(GO) mod download

# Upgrade dependencies across root and submodules, then tidy all.
upgrade:
	$(GO) get -u all
	$(MAKE) tidy

# Optional: compile-only cross-builds to catch GOARCH portability issues.
cross-build:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GO) build ./...
	GOOS=linux GOARCH=386  CGO_ENABLED=0 $(GO) build ./...

clean:
	rm -rf $(BENCHMARK_DIR)/bench*.test
