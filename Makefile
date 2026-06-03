# Defaults
GO        ?= go
GOFLAGS   ?= -trimpath
APP       ?= ai

.PHONY: help build test lint fmt tidy docs clean guard install-git-hooks worktree build-pr

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

guard: ## Check branch/worktree/dirty-state governance before edits
	$(GO) run ./src/cmd/ai guard

install-git-hooks: ## Install repo-managed git hooks into .git/hooks
	$(GO) run ./scripts/install_git_hooks

worktree: ## Create a canonical repo-local worktree: make worktree BRANCH=feature-name
	$(GO) run ./scripts/create_worktree -branch "$(BRANCH)"

build-pr: guard test ## Safe PR-build entrypoint; runs guard and tests before explicit commit/push/PR steps
	@echo "build-pr gate passed. Commit, push, and PR creation still require explicit principal approval or make-build."
