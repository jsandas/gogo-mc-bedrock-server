.PHONY: test test-integration test-unit build clean docker-up docker-down

run-wrapper: ./cmd/minecraft-server-wrapper/main.go
	AUTH_KEY=supersecret go run ./cmd/minecraft-server-wrapper

run-center: ./cmd/minecraft-server-center/main.go
	AUTH_KEY=supersecret go run ./cmd/minecraft-server-center

run-docker:
# 	docker build --platform linux/amd64 -t gogo-mc-bedrock-wrapper -f build/minecraft-server-wrapper/Dockerfile .
	@docker run --name minecraft-server-wrapper -it --rm -p 8080:8080 -p 19132:19132/udp \
		-e EULA_ACCEPT=true -e AUTH_KEY=supersecret \
		jsandas/minecraft-server-wrapper:latest-amd64

# Default target
all: build

# Build the artifacts
build:
	goreleaser release --clean --snapshot

# Run all tests and quality checks
test: quality test-integration

# Run unit tests only
test-unit:
	@go test -v ./...

# Run integration tests (requires docker compose)
test-integration:
	go test -v -tags integration ./...

# Run linting with golangci-lint
lint:
	golangci-lint run --timeout=5m

# Run linting with golangci-lint (install if not present)
lint-install:
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	golangci-lint run --timeout=5m

# Check code formatting
fmt-check:
	@if [ "$(shell gofmt -s -l . | wc -l)" -gt 0 ]; then \
		echo "Code is not formatted. Please run 'gofmt -s -w .'"; \
		gofmt -s -l .; \
		exit 1; \
	fi
	@echo "Code formatting is correct"

go-mod-tidy:
	@go mod tidy
	@if [ -n "$(shell git status --porcelain | egrep '(go.mod|go.sum)')" ]; then \
		echo "go.mod or go.sum is not tidy. Please run 'go mod tidy'"; \
		exit 1; \
	fi
	@echo "go.mod and go.sum are tidy"

# Format code
fmt:
	gofmt -s -w .

# Run all code quality checks
quality: fmt-check go-mod-tidy lint
	@echo "All code quality checks passed!"

# Show help
help:
	@echo "Available targets:"
	@echo "  build              - Build the packages"
	@echo "  test               - Run all tests (integration tests)"
	@echo "  test-unit          - Show unit test status"
	@echo "  test-integration   - Run integration tests"
	@echo "  docker-up          - Start docker services"
	@echo "  docker-down        - Stop docker services"
	@echo "  clean              - Clean build artifacts"
	@echo ""
	@echo "Code Quality:"
	@echo "  quality            - Run all code quality checks"
	@echo "  lint               - Run golangci-lint (requires golangci-lint)"
	@echo "  lint-install       - Install golangci-lint and run linting"
	@echo "  fmt-check          - Check code formatting"
	@echo "  fmt                - Format code with gofmt"
	@echo "  mod-check          - Check if go.mod is tidy"
	@echo ""
	@echo "Other:"
	@echo "  example            - Run the example"
	@echo "  help               - Show this help"
