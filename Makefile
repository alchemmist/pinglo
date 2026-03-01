BIN_DIR := ./bin
GOBIN := $(shell go env GOPATH)/bin
GOFILES := ./cmd/pinglo ./cmd/pinglod ./...

.PHONY: all build test clean run-daemon install

all: build

build: build-pinglo build-pinglod

build-pinglo:
	@mkdir -p $(BIN_DIR)
	@echo "building pinglo"
	@go build -o $(BIN_DIR)/pinglo ./cmd/pinglo

build-pinglod:
	@mkdir -p $(BIN_DIR)
	@echo "building pinglod"
	@go build -o $(BIN_DIR)/pinglod ./cmd/pinglod

test:
	@GOCACHE=/tmp/go-build go test ./...

clean:
	@rm -rf $(BIN_DIR)
	@rm -rf /tmp/go-build

run-daemon:
	@$(BIN_DIR)/pinglod

install: build
	@echo "installing pinglo/pinglod into $(GOBIN)"
	@mkdir -p $(GOBIN)
	@GOBIN=$(GOBIN) go install ./cmd/pinglo ./cmd/pinglod
