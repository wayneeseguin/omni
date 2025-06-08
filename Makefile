# Omni Makefile
# Version info
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.0.1")

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get
GOVET=$(GOCMD) vet

# Directories
EXAMPLES_DIR=examples
COVERAGE_FILE=coverage.out
COVERAGE_HTML=coverage.html

# Default target - show help
.DEFAULT_GOAL := help

.PHONY: all help version build package clean test test-unit test-clean test-verbose test-race test-integration test-integration-verbose test-all integration integration-nats integration-vault integration-syslog integration-debug debug-nats test-nats-recovery test-nats-monitor test-nats-load bench bench-full fmt vet security gosec trivy coverage coverage-text deploy deps check ci build-examples run-examples install-tools

# Show help with version
help:
	@echo "Omni Makefile - Available targets:"
	@echo ""
	@echo "Build & Package:"
	@echo "  make              - Show this help message (default)"
	@echo "  make build        - Build the omni library"
	@echo "  make build-examples - Build all example applications"
	@echo "  make package      - Clean, build, and prepare for release"
	@echo "  make clean        - Remove build artifacts and binaries"
	@echo ""
	@echo "Testing:"
	@echo "  make test         - Run all unit tests (alias for test-unit)"
	@echo "  make test-unit    - Run Go unit tests with coverage"
	@echo "  make test-clean   - Run tests without linker warnings (CGO disabled)"
	@echo "  make test-verbose - Run tests with verbose output"
	@echo "  make test-race    - Run tests with Go race detector enabled"
	@echo "  make test-integration - Run Docker-based integration tests"
	@echo "  make test-all     - Run all test targets"
	@echo ""
	@echo "Integration Testing:"
	@echo "  make integration  - Run all integration tests with Docker"
	@echo "  make integration-nats - Run NATS integration tests only"
	@echo "  make integration-vault - Run Vault integration tests only"
	@echo "  make integration-syslog - Run Syslog integration tests only"
	@echo "  make integration-debug - Run integration tests (keep containers)"
	@echo ""
	@echo "NATS Testing:"
	@echo "  make debug-nats   - Start NATS debug environment"
	@echo "  make test-nats-recovery - Run NATS error recovery tests"
	@echo "  make test-nats-monitor - Run NATS monitoring tests"
	@echo "  make test-nats-load - Run NATS load/performance tests"
	@echo ""
	@echo "Code Quality:"
	@echo "  make fmt          - Format all Go source files using gofmt"
	@echo "  make vet          - Run go vet for static analysis"
	@echo "  make security     - Run all security checks (gosec + trivy)"
	@echo "  make gosec        - Run gosec security scanner"
	@echo "  make trivy        - Run trivy vulnerability scanner"
	@echo "  make coverage     - Generate HTML coverage report"
	@echo "  make coverage-text - Show coverage summary in terminal"
	@echo ""
	@echo "Performance:"
	@echo "  make bench        - Run benchmarks"
	@echo "  make bench-full   - Run comprehensive benchmarks (10s)"
	@echo ""
	@echo "Deployment & Dependencies:"
	@echo "  make deps         - Download and verify Go module dependencies"
	@echo "  make check        - Verify all build prerequisites are installed"
	@echo "  make ci           - Run full CI pipeline (vet, test, build)"
	@echo "  make install-tools - Install development tools"
	@echo ""
	@echo "Examples:"
	@echo "  make run-examples - Run all example applications"
	@echo ""
	@echo "Version: $(VERSION)"

# Show version
version:
	@echo "Omni version: $(VERSION)"

# Build & Package targets
build:
	@echo "Building omni library..."
	@$(GOBUILD) -v ./...

# Build all examples
build-examples:
	@echo "Building examples..."
	@for dir in $(EXAMPLES_DIR)/*/; do \
		echo "Building $$dir..."; \
		(cd $$dir && $(GOBUILD) -v .); \
	done

# Package for release
package: clean build
	@echo "Packaging omni $(VERSION)..."
	@echo "Library packaged successfully!"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)
	@rm -f *.test
	@rm -f *.log
	@find . -name "*.log.*" -delete
	@$(GOCLEAN) -cache

# Testing targets
test: test-unit

# Run unit tests
test-unit:
	@echo "Running unit tests..."
	@CGO_ENABLED=0 $(GOTEST) -v -coverprofile=$(COVERAGE_FILE) $(shell go list ./... | grep -v /examples/)

# Run tests without linker warnings (CGO disabled)
test-clean:
	@echo "Running tests without linker warnings (CGO disabled)..."
	@CGO_ENABLED=0 $(GOTEST) ./...

# Run tests with verbose output
test-verbose:
	@echo "Running tests with verbose output..."
	@CGO_ENABLED=0 $(GOTEST) -v -coverprofile=$(COVERAGE_FILE) -covermode=atomic $(shell go list ./... | grep -v /examples/)

# Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	@echo "Note: LC_DYSYMTAB warnings on macOS are a known Go issue and can be safely ignored"
	@$(GOTEST) -v -race -coverprofile=$(COVERAGE_FILE) $(shell go list ./... | grep -v /examples/)

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	@$(GOCLEAN) -testcache
	@$(GOTEST) -v -tags=integration -timeout=10m ./...

# Run integration tests with verbose output
test-integration-verbose:
	@echo "Running integration tests with verbose output..."
	@$(GOCLEAN) -testcache
	@VERBOSE=1 $(GOTEST) -v -tags=integration -timeout=10m ./...

# Run all test targets
test-all: test-unit test-verbose test-race test-integration test-integration-verbose test-nats-recovery test-nats-monitor test-nats-load
	@echo "All tests completed!"

# Integration testing targets
integration:
	@echo "Running integration tests with Docker containers..."
	@./scripts/integration

integration-nats:
	@echo "Running NATS integration tests..."
	@./scripts/integration --nats-only

integration-vault:
	@echo "Running Vault integration tests..."
	@./scripts/integration --vault-only

integration-syslog:
	@echo "Running Syslog integration tests..."
	@./scripts/integration --syslog-only

integration-debug:
	@echo "Running integration tests with containers kept alive..."
	@./scripts/integration --keep-containers --verbose

# NATS testing targets
debug-nats:
	@echo "Starting NATS debug environment..."
	@./scripts/debug-nats start
	@./scripts/debug-nats status

test-nats-recovery:
	@echo "Running NATS error recovery tests..."
	@./scripts/debug-nats test TestNATSErrorRecovery

test-nats-monitor:
	@echo "Running NATS monitoring tests..."
	@./scripts/debug-nats test TestNATSMonitoring

test-nats-load:
	@echo "Running NATS load tests..."
	@$(GOTEST) -v -tags=integration -run TestNATSLoadTest -timeout=30s ./examples/plugins/nats-backend/...

# Code Quality targets
fmt:
	@echo "Formatting code..."
	@gofmt -l -w .
	@$(GOCMD) mod tidy

vet:
	@echo "Running go vet..."
	@$(GOVET) ./...

# Security scanning targets
gosec:
	@echo "Running gosec security scan..."
	@gosec -quiet ./...

trivy:
	@echo "Running trivy vulnerability scan..."
	@trivy fs . --scanners vuln

security: gosec trivy
	@echo "All security checks completed!"

# Coverage targets
coverage:
	@echo "Generating coverage report..."
	@$(GOTEST) -coverprofile=$(COVERAGE_FILE) $(shell go list ./... | grep -v /examples/)
	@$(GOCMD) tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "Coverage report generated: $(COVERAGE_HTML)"

coverage-text:
	@echo "Test coverage summary:"
	@$(GOTEST) -coverprofile=$(COVERAGE_FILE) $(shell go list ./... | grep -v /examples/)
	@$(GOCMD) tool cover -func=$(COVERAGE_FILE)

# Performance targets
bench:
	@echo "Running benchmarks..."
	@$(GOTEST) -run=^$$ -bench=. -benchmem ./...

bench-full:
	@echo "Running comprehensive benchmarks..."
	@$(GOTEST) -run=^$$ -bench=. -benchmem -benchtime=10s ./...

# Deployment & Dependencies
deps:
	@echo "Tidying dependencies..."
	@$(GOCMD) mod tidy
	@$(GOCMD) mod verify

check: vet test security
	@echo "All checks passed!"

# CI pipeline
ci: vet test build
	@echo "CI pipeline completed successfully!"

# Run examples
run-examples:
	@echo "Running examples..."
	@for dir in $(EXAMPLES_DIR)/*/; do \
		echo "Running $$dir..."; \
		(cd $$dir && $(GOCMD) run .); \
	done

# Install development tools
install-tools:
	@echo "Installing development tools..."
	@$(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@$(GOCMD) install github.com/securego/gosec/v2/cmd/gosec@latest
	@$(GOCMD) install golang.org/x/tools/cmd/goimports@latest