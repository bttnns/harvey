# Basics. Every target assumes a Go toolchain; on a host without one, run the
# toolchain targets through harv itself (the dogfood move):
#   harv make check
#   harv 'make install GOOS=darwin GOARCH=arm64'   # cross-compile the host binary
# e2e talks to the container runtime, so it always runs on the host.

VERSION := $(shell git describe --tags --always --dirty)
LDFLAGS := -s -w -X github.com/bttnns/harvey/cmd.Version=$(VERSION)
BIN     := harv
PREFIX  ?= $(HOME)/.local

.PHONY: build install test vet lint fmt check e2e clean

build: ## build ./harv, version-stamped from git describe
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) .

install: ## build and install to $(PREFIX)/bin (override GOOS/GOARCH to cross-compile)
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(PREFIX)/bin/$(BIN) .

test:
	go test ./...

vet:
	go vet ./...

lint:
	golangci-lint run

fmt:
	gofmt -w .

check: vet test lint ## everything CI checks, runtime-free

e2e: ## full end-to-end gate against every usable runtime on this machine
	./scripts/e2e-all.sh

clean:
	rm -f $(BIN)
