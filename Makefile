GO ?= go
PKG := pkt.systems/pslog
TESTBIN := .testbin
ARM64_TEST := $(TESTBIN)/pslog_arm64.test
I386_TEST := $(TESTBIN)/pslog_386.test
QEMU_AARCH64 ?= qemu-aarch64
QEMU_I386 ?= qemu-i386

.PHONY: test test-native test-arm64 test-386 clean-testbin clean

test: test-native test-arm64 test-386

test-native:
	$(GO) test $(PKG)

test-arm64: $(ARM64_TEST)
	$(QEMU_AARCH64) $< -test.v

test-386: $(I386_TEST)
	$(QEMU_I386) $< -test.v

$(ARM64_TEST): $(TESTBIN)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GO) test $(PKG) -c -o $@

$(I386_TEST): $(TESTBIN)
	GOOS=linux GOARCH=386 GO386=softfloat CGO_ENABLED=0 $(GO) test $(PKG) -c -o $@

$(TESTBIN):
	mkdir -p $(TESTBIN)

clean-testbin:
	rm -rf $(TESTBIN)

clean: clean-testbin
