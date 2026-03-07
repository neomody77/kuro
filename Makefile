.PHONY: build build-recovery build-all ui ui-install dev dev-go dev-ui test lint clean help

BIN_DIR := bin

## Build

build: ui
	go build -o $(BIN_DIR)/kuro ./cmd/kuro

build-cli:
	go build -o $(BIN_DIR)/kuro-cli ./cmd/kuro-cli

build-recovery:
	go build -o $(BIN_DIR)/kuro-recovery ./cmd/kuro-recovery

build-all: build build-cli build-recovery

## UI

ui-install:
	cd ui && npm install

ui: ui-install
	cd ui && npm run build
	rm -rf cmd/kuro/ui
	cp -r ui/dist cmd/kuro/ui

## Development

dev:
	@echo "Starting Kuro dev (Go + UI)..."
	@trap 'kill 0' EXIT; \
	cd ui && npm run dev &\
	go run ./cmd/kuro & \
	wait

dev-go:
	go run ./cmd/kuro

dev-ui:
	cd ui && npm run dev

## Quality

test:
	go test ./...

lint:
	golangci-lint run ./...

vet:
	go vet ./...

check: vet lint test

## Cleanup

clean:
	rm -rf $(BIN_DIR) cmd/kuro/ui

## Help

help:
	@echo "Kuro build targets:"
	@echo "  make build          Build engine binary (includes UI)"
	@echo "  make build-cli      Build CLI binary"
	@echo "  make build-recovery Build recovery binary"
	@echo "  make build-all      Build all binaries"
	@echo "  make ui             Build frontend assets"
	@echo "  make dev            Run Go backend + Vite dev server"
	@echo "  make dev-go         Run Go backend only"
	@echo "  make dev-ui         Run Vite dev server only"
	@echo "  make test           Run Go tests"
	@echo "  make lint           Run golangci-lint"
	@echo "  make vet            Run go vet"
	@echo "  make check          Run vet + lint + test"
	@echo "  make clean          Remove build artifacts"
