.PHONY: all ci build test test-verbose test-race test-integration bench bench-full lint clean coverage coverage-text fmt deps security check build-examples run-examples install-tools help

# Default target - show help
all: help

# Run full CI pipeline
ci: lint test build
	@echo "CI pipeline completed successfully!"

# Build the library
build:
	@echo "Building omni..."
	@go build -v ./...

# Run tests
test:
	@echo "Running tests..."
	@CGO_ENABLED=0 go test -v -coverprofile=coverage.out ./...

# Run tests with verbose output
test-verbose:
	@echo "Running tests with verbose output..."
	@CGO_ENABLED=0 go test -v -coverprofile=coverage.out -covermode=atomic ./...

# Run tests with race detector (may show linker warnings on macOS)
test-race:
	@echo "Running tests with race detector..."
	@go test -v -race -coverprofile=coverage.out ./...

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	@go clean -testcache
	@go test -v -tags=integration -timeout=10m ./...

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	@go test -run=^$$ -bench=. -benchmem ./...

# Run comprehensive benchmarks
bench-full:
	@echo "Running comprehensive benchmarks..."
	@go test -run=^$$ -bench=. -benchmem -benchtime=10s ./...

# Run linter
lint:
	@echo "Running linter..."
	@golangci-lint run ./...

# Generate test coverage report
coverage:
	@echo "Generating coverage report..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Show coverage in terminal
coverage-text:
	@echo "Test coverage summary:"
	@go test -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f coverage.out coverage.html
	@rm -f *.test
	@rm -f *.log
	@find . -name "*.log.*" -delete
	@go clean -cache

# Format code
fmt:
	@echo "Formatting code..."
	@gofmt -l -w .
	@go mod tidy

# Run go mod tidy and vendor
deps:
	@echo "Tidying dependencies..."
	@go mod tidy
	@go mod verify

# Check for security vulnerabilities
security:
	@echo "Running security scan..."
	@gosec -quiet ./...

# Run all quality checks
check: lint test security
	@echo "All checks passed!"

# Build examples
build-examples:
	@echo "Building examples..."
	@for dir in examples/*/; do \
		echo "Building $$dir..."; \
		(cd $$dir && go build -v .); \
	done

# Run examples
run-examples:
	@echo "Running examples..."
	@for dir in examples/*/; do \
		echo "Running $$dir..."; \
		(cd $$dir && go run .); \
	done

# Install development tools
install-tools:
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/securego/gosec/v2/cmd/gosec@latest
	@go install golang.org/x/tools/cmd/goimports@latest

# Help target
help:
	@echo "Available targets:"
	@echo "  help             - Show this help message (default)"
	@echo "  ci               - Run full CI pipeline (lint, test, build)"
	@echo "  build            - Build the library"
	@echo "  test             - Run tests (CGO disabled, no linker warnings)"
	@echo "  test-verbose     - Run tests with verbose output"
	@echo "  test-race        - Run tests with race detector (may show warnings)"
	@echo "  test-integration - Run integration tests"
	@echo "  bench            - Run benchmarks"
	@echo "  bench-full       - Run comprehensive benchmarks"
	@echo "  lint             - Run golangci-lint"
	@echo "  coverage         - Generate HTML coverage report"
	@echo "  coverage-text    - Show coverage in terminal"
	@echo "  clean            - Remove build artifacts"
	@echo "  fmt              - Format code"
	@echo "  deps             - Tidy and verify dependencies"
	@echo "  security         - Run security scan"
	@echo "  check            - Run all quality checks"
	@echo "  build-examples   - Build all examples"
	@echo "  run-examples     - Run all examples"
	@echo "  install-tools    - Install development tools"
	@echo "  help             - Show this help message"