.PHONY: all ci build test test-verbose test-race test-integration integration integration-nats integration-vault integration-syslog integration-debug debug-nats test-nats-recovery test-nats-monitor test-nats-load bench bench-full vet clean coverage coverage-text fmt deps security check build-examples run-examples install-tools help

# Default target - show help
all: help

# Run full CI pipeline
ci: vet test build
	@echo "CI pipeline completed successfully!"

# Build the library
build:
	@echo "Building omni..."
	@go build -v ./...

# Run tests
test:
	@echo "Running tests..."
	@CGO_ENABLED=0 go test -v -coverprofile=coverage.out $(shell go list ./... | grep -v /examples/)

# Run tests with verbose output
test-verbose:
	@echo "Running tests with verbose output..."
	@CGO_ENABLED=0 go test -v -coverprofile=coverage.out -covermode=atomic $(shell go list ./... | grep -v /examples/)

# Run tests with race detector (may show linker warnings on macOS)
test-race:
	@echo "Running tests with race detector..."
	@go test -v -race -coverprofile=coverage.out $(shell go list ./... | grep -v /examples/)

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	@go clean -testcache
	@go test -v -tags=integration -timeout=10m ./...

# Run integration tests with verbose output
test-integration-verbose:
	@echo "Running integration tests with verbose output..."
	@go clean -testcache
	@VERBOSE=1 go test -v -tags=integration -timeout=10m ./...

# Run integration tests with Docker containers
integration:
	@echo "Running integration tests with Docker containers..."
	@./scripts/integration

# Run NATS integration tests only
integration-nats:
	@echo "Running NATS integration tests..."
	@./scripts/integration --nats-only

# Run Vault integration tests only
integration-vault:
	@echo "Running Vault integration tests..."
	@./scripts/integration --vault-only

# Run Syslog integration tests only
integration-syslog:
	@echo "Running Syslog integration tests..."
	@./scripts/integration --syslog-only

# Run integration tests and keep containers running
integration-debug:
	@echo "Running integration tests with containers kept alive..."
	@./scripts/integration --keep-containers --verbose

# Debug NATS integration tests
debug-nats:
	@echo "Starting NATS debug environment..."
	@./scripts/debug-nats start
	@./scripts/debug-nats status

# Run NATS error recovery tests
test-nats-recovery:
	@echo "Running NATS error recovery tests..."
	@./scripts/debug-nats test TestNATSErrorRecovery

# Run NATS monitoring tests
test-nats-monitor:
	@echo "Running NATS monitoring tests..."
	@./scripts/debug-nats test TestNATSMonitoring

# Run NATS load tests
test-nats-load:
	@echo "Running NATS load tests..."
	@go test -v -tags=integration -run TestNATSLoadTest -timeout=30s ./examples/plugins/nats-backend/...

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	@go test -run=^$$ -bench=. -benchmem ./...

# Run comprehensive benchmarks
bench-full:
	@echo "Running comprehensive benchmarks..."
	@go test -run=^$$ -bench=. -benchmem -benchtime=10s ./...

# Run go vet
vet:
	@echo "Running go vet..."
	@go vet ./...

# Generate test coverage report
coverage:
	@echo "Generating coverage report..."
	@go test -coverprofile=coverage.out $(shell go list ./... | grep -v /examples/)
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Show coverage in terminal
coverage-text:
	@echo "Test coverage summary:"
	@go test -coverprofile=coverage.out $(shell go list ./... | grep -v /examples/)
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
check: vet test security
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
	@echo "  help                     - Show this help message (default)"
	@echo "  ci                       - Run full CI pipeline (lint, test, build)"
	@echo "  build                    - Build the library"
	@echo "  test                     - Run tests (CGO disabled, no linker warnings)"
	@echo "  test-verbose             - Run tests with verbose output"
	@echo "  test-race                - Run tests with race detector (may show warnings)"
	@echo "  test-integration         - Run integration tests"
	@echo "  test-integration-verbose - Run integration tests with verbose output"
	@echo "  integration              - Run all integration tests with Docker containers"
	@echo "  integration-nats         - Run only NATS integration tests"
	@echo "  integration-vault        - Run only Vault integration tests"
	@echo "  integration-syslog       - Run only Syslog integration tests"
	@echo "  integration-debug        - Run integration tests keeping containers alive"
	@echo "  debug-nats               - Start NATS debug environment"
	@echo "  test-nats-recovery       - Run NATS error recovery tests"
	@echo "  test-nats-monitor        - Run NATS monitoring tests"
	@echo "  test-nats-load           - Run NATS load/performance tests"
	@echo "  bench                    - Run benchmarks"
	@echo "  bench-full               - Run comprehensive benchmarks"
	@echo "  vet                      - Run go vet on the codebase"
	@echo "  coverage                 - Generate HTML coverage report"
	@echo "  coverage-text            - Show coverage in terminal"
	@echo "  clean                    - Remove build artifacts"
	@echo "  fmt                      - Format code"
	@echo "  deps                     - Tidy and verify dependencies"
	@echo "  security                 - Run security scan"
	@echo "  check                    - Run all quality checks"
	@echo "  build-examples           - Build all examples"
	@echo "  run-examples             - Run all examples"
	@echo "  install-tools            - Install development tools"
	@echo "  help                     - Show this help message"