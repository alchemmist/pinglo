BIN_DIR := ./bin
INTEGRATIONS_DIR := ./integrations
GOBIN := $(shell go env GOPATH)/bin
GOFILES := ./cmd/pinglo ./cmd/pinglod ./...

.PHONY: all build test clean run-daemon install run-integration-template run-codex-integration

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

run-integration-template:
	$(INTEGRATIONS_DIR)/templates/integration-template.sh

run-codex-integration:
	$(INTEGRATIONS_DIR)/codex/pinglo-codex-chat.sh exec

install: build
	mkdir -p $(GOBIN)
	GOBIN=$(GOBIN) go install ./cmd/pinglo ./cmd/pinglod
