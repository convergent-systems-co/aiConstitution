# Defaults
GO        ?= go
GOFLAGS   ?= -trimpath
APP       ?= ai

.PHONY: help build test lint fmt tidy docs clean

help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the binary into dist/
	mkdir -p dist
	$(GO) build $(GOFLAGS) -o dist/$(APP) ./src/cmd/$(APP)

test: ## Run unit tests
	$(GO) test ./... -race

lint: ## Run golangci-lint
	golangci-lint run ./...

fmt: ## Run gofmt
	gofmt -s -w .

tidy: ## go mod tidy
	$(GO) mod tidy

docs: ## Regenerate README command table from registered commands
	go run ./src/cmd/ai/cmd/gen_docs.go > /tmp/cmd-table.md
	@echo "Command table written to /tmp/cmd-table.md"
	@echo "Review and paste into README.md ## What it does section"

clean: ## Remove build artifacts
	rm -rf dist/
