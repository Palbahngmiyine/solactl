.PHONY: build install install-release test test-race lint setup-hooks clean fuzz

HOOKS_DIR := $(shell git rev-parse --git-path hooks)

build:
	@mkdir -p bin
	go build -o bin/solactl ./cmd/solactl

install:
	go install ./cmd/solactl

install-release:
	bash scripts/install.sh

test:
	go test ./... -count=1

test-race:
	go test ./... -race -count=1

fuzz:
	go test ./pkg/validation/... -fuzz=. -fuzztime=10s

lint:
	golangci-lint run ./...

setup-hooks:
	chmod +x scripts/pre-commit
	ln -sf $(abspath scripts/pre-commit) $(HOOKS_DIR)/pre-commit
	@echo "Pre-commit hook installed at $(HOOKS_DIR)/pre-commit"

clean:
	rm -rf bin/
