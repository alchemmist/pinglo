.DEFAULT_GOAL := help

SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules

BIN_DIR := ./bin
INTEGRATIONS_DIR := ./integrations
GO_PACKAGES := ./...
GOFMT_PATHS := ./cmd ./internal
GOBIN := $(shell go env GOPATH)/bin
STATICCHECK := $(GOBIN)/staticcheck
GOLANGCI_LINT := $(GOBIN)/golangci-lint

SHELL_SCRIPTS := $(shell find . -type f -name '*.sh' -not -path './.git/*' | sort)
YAML_FILES := $(shell find . -type f \( -name '*.yml' -o -name '*.yaml' \) -not -path './.git/*' | sort)

.PHONY: all help check build build-pinglo build-pinglod test test-race fmt fmt-check fmt-go fmt-go-check fmt-sh fmt-sh-check vet staticcheck golangci-lint lint lint-shellcheck lint-yaml lint-makefile run-daemon run-integration-template run-codex-integration install clean

all: check

check: fmt-check lint test build

build: build-pinglo build-pinglod

build-pinglo:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/pinglo ./cmd/pinglo

build-pinglod:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/pinglod ./cmd/pinglod

test:
	GOCACHE=/tmp/go-build go test $(GO_PACKAGES)

test-race:
	GOCACHE=/tmp/go-build go test -race $(GO_PACKAGES)

fmt: fmt-go fmt-sh

fmt-check: fmt-go-check fmt-sh-check

fmt-go:
	gofmt -w $(GOFMT_PATHS)

fmt-go-check:
	@unformatted="$$(gofmt -l $(GOFMT_PATHS))"; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt required for:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

fmt-sh:
	shfmt -w $(SHELL_SCRIPTS)

fmt-sh-check:
	shfmt -d $(SHELL_SCRIPTS)

vet:
	go vet $(GO_PACKAGES)

staticcheck:
	go install honnef.co/go/tools/cmd/staticcheck@latest
	$(STATICCHECK) $(GO_PACKAGES)

golangci-lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOLANGCI_LINT) run ./...

lint: vet staticcheck golangci-lint lint-shellcheck lint-yaml lint-makefile

lint-shellcheck:
	shellcheck $(SHELL_SCRIPTS)

lint-yaml:
	yamllint -c .yamllint.yml $(YAML_FILES)

lint-makefile:
	checkmake Makefile

run-daemon: build-pinglod
	$(BIN_DIR)/pinglod

run-integration-template: build-pinglo
	PINGLO_BIN=$(BIN_DIR)/pinglo $(INTEGRATIONS_DIR)/templates/integration-template.sh

run-codex-integration: build-pinglo
	PINGLO_BIN=$(BIN_DIR)/pinglo $(INTEGRATIONS_DIR)/codex/codex-with-pinglo.sh

install: build
	mkdir -p $(GOBIN)
	GOBIN=$(GOBIN) go install ./cmd/pinglo ./cmd/pinglod

clean:
	rm -rf $(BIN_DIR) /tmp/go-build

help:
	@printf "Available targets:\n\n"
	@printf "  check               - run fmt-check, lint, test, build\n"
	@printf "  build               - build pinglo and pinglod into ./bin\n"
	@printf "  test                - run go test ./...\n"
	@printf "  test-race           - run go test -race ./...\n"
	@printf "  fmt                 - format go and shell files\n"
	@printf "  fmt-check           - verify go and shell files are formatted\n"
	@printf "  vet                 - run go vet\n"
	@printf "  staticcheck         - run staticcheck\n"
	@printf "  golangci-lint       - run golangci-lint\n"
	@printf "  lint-shellcheck     - run shellcheck on .sh files\n"
	@printf "  lint-yaml           - run yamllint on YAML files\n"
	@printf "  lint-makefile       - lint Makefile via checkmake\n"
	@printf "  lint                - run all linters\n"
	@printf "  run-daemon          - build and run pinglod\n"
	@printf "  run-codex-integration - run codex TUI wrapper with local pinglo\n"
	@printf "  install             - install pinglo and pinglod with go install\n"
	@printf "  clean               - remove build artifacts\n"
