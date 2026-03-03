BIN_DIR := ./bin
INTEGRATIONS_DIR := ./integrations
GOBIN := $(shell go env GOPATH)/bin
GOFILES := ./cmd/pinglo ./cmd/pinglod ./...
SHELL_SCRIPTS := $(shell find . -type f -name '*.sh' -not -path './.git/*' | sort)

.PHONY: all build test clean run-daemon install run-integration-template run-codex-integration lint-shellcheck

all: build

build: build-pinglo build-pinglod

build-pinglo:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/pinglo ./cmd/pinglo

build-pinglod:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/pinglod ./cmd/pinglod

test:
	@GOCACHE=/tmp/go-build go test ./...

clean:
	rm -rf $(BIN_DIR)
	rm -rf /tmp/go-build

run-daemon:
	$(BIN_DIR)/pinglod

run-integration-template: build-pinglo
	PINGLO_BIN=$(BIN_DIR)/pinglo $(INTEGRATIONS_DIR)/templates/integration-template.sh

run-codex-integration: build-pinglo
	PINGLO_BIN=$(BIN_DIR)/pinglo $(INTEGRATIONS_DIR)/codex/codex-with-pinglo.sh

lint-shellcheck:
	shellcheck $(SHELL_SCRIPTS)

install: build
	mkdir -p $(GOBIN)
	GOBIN=$(GOBIN) go install ./cmd/pinglo ./cmd/pinglod
