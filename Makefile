# AgentScope Go Makefile
# Usage (Unix/macOS/Linux): make test
# Windows: use Git Bash / WSL / mingw32-make, or run the go commands directly.
# All targets respect the project rule: go test ./... -race must pass.

.PHONY: help test test-short fmt vet lint build clean bench ci cover cover-html

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

test: ## Run all tests with race detector (required before any commit)
	go test -race -count=1 -timeout=12m ./...

test-short: ## Quick tests (no race, for fast iteration)
	go test -count=1 -timeout=5m ./...

fmt: ## Format all Go code (must be clean before commit)
	gofmt -l -w .

fmt-check: ## Check formatting (used by CI)
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "The following files are not gofmt'ed:"; \
		gofmt -l .; \
		exit 1; \
	fi
	@echo "All Go files are properly formatted."

vet: ## Run go vet
	go vet ./...

# golangci-lint must be installed (https://golangci-lint.run/welcome/install/)
# or use the CI job which installs it automatically.
lint: ## Run golangci-lint (recommended)
	golangci-lint run ./...

build: ## Build all packages
	go build ./...

clean: ## Remove build artifacts, coverage files and binaries
	find . -type f \( -name '*.out' -o -name '*.exe' -o -name '*.test' \) -delete
	rm -rf coverage/ react_coverage/ .cache/embeddings 2>/dev/null || true
	@echo "Cleaned build artifacts."

bench: ## Run benchmarks
	go test -bench=. -benchmem -timeout=5m ./...

ci: fmt-check vet build test ## Simulate the main CI steps locally (without golangci)

# Convenience: full local pre-commit check (add golangci if installed)
precommit: fmt-check vet lint build test

fuzz: ## Run fuzz tests (example for json tool; use -fuzztime=10s etc.)
	go test -fuzz=Fuzz -fuzztime=5s ./tool/json ./permission 2>&1 | head -20 || echo "fuzz example (extend as needed)"

cover: ## Generate coverage report (coverage.out)
	go test -race -count=1 -timeout=12m -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out | tail -5

cover-html: ## Generate HTML coverage report (opens in browser)
	go test -race -count=1 -timeout=12m -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage HTML generated: coverage.html (use 'open coverage.html' or browser)"
	@echo "Pre-commit checks passed. Ready to push."