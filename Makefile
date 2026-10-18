# CampusNet Unified Makefile

.PHONY: all build clean test run-server run-cli docker-up docker-down

# Build variables
BIN_DIR := bin
SERVER_BIN := $(BIN_DIR)/campusnet-server
CLI_BIN := $(BIN_DIR)/campusnet-cli

all: build

build: build-server build-cli

build-server:
	@echo "==> Building CampusNet Server..."
	go build -o $(SERVER_BIN) ./cmd/server

build-cli:
	@echo "==> Building CampusNet CLI..."
	go build -o $(CLI_BIN) ./cmd/cli

test:
	@echo "==> Running comprehensive test suite..."
	go test -v -race ./pkg/... ./internal/...

run-server:
	@echo "==> Starting CampusNet Server in standalone mode..."
	go run ./cmd/server

run-cli:
	@echo "==> Launching CampusNet Interactive TUI..."
	go run ./cmd/cli

docker-up:
	docker compose up -d

docker-down:
	docker compose down

clean:
	rm -rf $(BIN_DIR)
