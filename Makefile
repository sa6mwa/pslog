GO ?= go
QEMU_AARCH64 ?= qemu-aarch64
QEMU_I386 ?= qemu-i386
WINE ?= wine
WINEDEBUG ?= -all
GOVER ?= 1.25

# Modules that depend on the root via replace directives.
BENCHMARK_DIR    := benchmark
EXAMPLES_DIR     := examples
ELEVATOR_DIR     := elevatorpitch
CONTOJSON_DIR    := pslogconsole2json

.PHONY: all check test test-cross test-benchmark test-examples test-elevatorpitch \
        build-examples build-elevatorpitch bench benchorder benchorder-cross \
        benchorder-arm64 benchorder-386 benchorder-all fuzz tidy download upgrade clean \
        cross-build

all: check

# Core module tests
test:
	$(GO) test -cover -count=1 -race ./...

test-cross: test
	WINEDEBUG=$(WINEDEBUG) GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
		$(GO) test -count=1 -exec "$(WINE)" ./...

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

benchorder-all: benchorder benchorder-cross

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
	cd $(CONTOJSON_DIR) && $(GO) mod tidy

download:
	$(GO) mod download
	cd $(BENCHMARK_DIR) && $(GO) mod download
	cd $(EXAMPLES_DIR) && $(GO) mod download
	cd $(ELEVATOR_DIR) && $(GO) mod download
	cd $(CONTOJSON_DIR) && $(GO) mod download

# Upgrade dependencies across root and submodules, then tidy all.
upgrade:
	$(GO) get -u all
	cd $(BENCHMARK_DIR) && $(GO) get -u all
	cd $(EXAMPLES_DIR) && $(GO) get -u all
	cd $(ELEVATOR_DIR) && $(GO) get -u all
	cd $(CONTOJSON_DIR) && $(GO) get -u all
	$(MAKE) tidy

setgoversion:
	$(GO) mod edit -go=$(GOVER)
	cd $(BENCHMARK_DIR) && $(GO) mod edit -go=$(GOVER)
	cd $(EXAMPLES_DIR) && $(GO) mod edit -go=$(GOVER)
	cd $(ELEVATOR_DIR) && $(GO) mod edit -go=$(GOVER)
	cd $(CONTOJSON_DIR) && $(GO) mod edit -go=$(GOVER)

# Optional: compile-only cross-builds to catch GOARCH portability issues.
cross-build:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GO) build ./...
	GOOS=linux GOARCH=386  CGO_ENABLED=0 $(GO) build ./...

clean:
	$(GO) clean ./...
	cd $(BENCHMARK_DIR) && $(GO) clean ./...
	cd $(EXAMPLES_DIR) && $(GO) clean ./...
	cd $(ELEVATOR_DIR) && $(GO) clean ./...
	cd $(CONTOJSON_DIR) && $(GO) clean ./...
	rm -f $(EXAMPLES_DIR)/demo/demo $(ELEVATOR_DIR)/elevatorpitch
