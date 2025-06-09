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

.PHONY: all help version build package clean test test-unit test-clean test-verbose test-race test-integration test-integration-verbose test-nats test-vault test-all test-diskfull integration integration-nats integration-vault integration-syslog integration-debug integration-diskfull bench bench-full fmt vet security gosec trivy coverage coverage-text deploy deps check ci build-examples run-examples install-tools

# Show help with version
help:
	@echo "Omni Makefile - Available targets:"
	@echo ""
	@echo "Build & Package:"
	@printf "  \033[36m%-25s\033[0m %s\n" ""                    "Show this help message (default)"
	@printf "  \033[36m%-25s\033[0m %s\n" "build"               "Build the omni library"
	@printf "  \033[36m%-25s\033[0m %s\n" "build-examples"      "Build all example applications"
	@printf "  \033[36m%-25s\033[0m %s\n" "package"             "Clean, build, and prepare for release"
	@printf "  \033[36m%-25s\033[0m %s\n" "clean"               "Remove build artifacts and binaries"
	@echo ""
	@echo "Testing:"
	@printf "  \033[36m%-25s\033[0m %s\n" "test"                "Run all unit tests (alias for test-unit)"
	@printf "  \033[36m%-25s\033[0m %s\n" "test-unit"           "Run Go unit tests with coverage"
	@printf "  \033[36m%-25s\033[0m %s\n" "test-clean"          "Run tests without linker warnings (CGO disabled)"
	@printf "  \033[36m%-25s\033[0m %s\n" "test-verbose"        "Run tests with verbose output"
	@printf "  \033[36m%-25s\033[0m %s\n" "test-race"           "Run tests with Go race detector enabled"
	@printf "  \033[36m%-25s\033[0m %s\n" "test-integration"    "Run Docker-based integration tests"
	@printf "  \033[36m%-25s\033[0m %s\n" "test-nats"           "Run all NATS tests (unit + integration)"
	@printf "  \033[36m%-25s\033[0m %s\n" "test-vault"          "Run all Vault tests (unit + integration)"
	@printf "  \033[36m%-25s\033[0m %s\n" "test-diskfull"       "Run disk full tests with Docker"
	@printf "  \033[36m%-25s\033[0m %s\n" "test-all"            "Run all test targets"
	@echo ""
	@echo "Integration Testing:"
	@printf "  \033[36m%-25s\033[0m %s\n" "integration"         "Run all integration tests with Docker"
	@printf "  \033[36m%-25s\033[0m %s\n" "integration-nats"    "Run NATS integration tests only"
	@printf "  \033[36m%-25s\033[0m %s\n" "integration-vault"   "Run Vault integration tests only"
	@printf "  \033[36m%-25s\033[0m %s\n" "integration-syslog"  "Run Syslog integration tests only"
	@printf "  \033[36m%-25s\033[0m %s\n" "integration-diskfull" "Run disk full integration tests"
	@printf "  \033[36m%-25s\033[0m %s\n" "integration-debug"   "Run integration tests (keep containers)"
	@echo ""
	@echo "Code Quality:"
	@printf "  \033[36m%-25s\033[0m %s\n" "fmt"                 "Format all Go source files using gofmt"
	@printf "  \033[36m%-25s\033[0m %s\n" "vet"                 "Run go vet for static analysis"
	@printf "  \033[36m%-25s\033[0m %s\n" "security"            "Run all security checks (gosec + trivy)"
	@printf "  \033[36m%-25s\033[0m %s\n" "gosec"               "Run gosec security scanner"
	@printf "  \033[36m%-25s\033[0m %s\n" "trivy"               "Run trivy vulnerability scanner"
	@printf "  \033[36m%-25s\033[0m %s\n" "coverage"            "Generate HTML coverage report"
	@printf "  \033[36m%-25s\033[0m %s\n" "coverage-text"       "Show coverage summary in terminal"
	@echo ""
	@echo "Performance:"
	@printf "  \033[36m%-25s\033[0m %s\n" "bench"               "Run benchmarks"
	@printf "  \033[36m%-25s\033[0m %s\n" "bench-full"          "Run comprehensive benchmarks (10s)"
	@echo ""
	@echo "Deployment & Dependencies:"
	@printf "  \033[36m%-25s\033[0m %s\n" "deps"                "Download and verify Go module dependencies"
	@printf "  \033[36m%-25s\033[0m %s\n" "check"               "Verify all build prerequisites are installed"
	@printf "  \033[36m%-25s\033[0m %s\n" "ci"                  "Run full CI pipeline (vet, test, build)"
	@printf "  \033[36m%-25s\033[0m %s\n" "install-tools"       "Install development tools"
	@echo ""
	@echo "Examples:"
	@printf "  \033[36m%-25s\033[0m %s\n" "run-examples"        "Run all example applications"
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
	@OMNI_UNIT_TESTS_ONLY=true CGO_ENABLED=0 $(GOTEST) -v -short -coverprofile=$(COVERAGE_FILE) $(shell go list ./... | grep -v /examples/)

# Run tests without linker warnings (CGO disabled)
test-clean:
	@echo "Running tests without linker warnings (CGO disabled)..."
	@OMNI_UNIT_TESTS_ONLY=true CGO_ENABLED=0 $(GOTEST) -short ./...

# Run tests with verbose output
test-verbose:
	@echo "Running tests with verbose output..."
	@OMNI_UNIT_TESTS_ONLY=true CGO_ENABLED=0 $(GOTEST) -v -short -coverprofile=$(COVERAGE_FILE) -covermode=atomic $(shell go list ./... | grep -v /examples/)

# Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	@echo "Note: LC_DYSYMTAB warnings on macOS are a known Go issue and can be safely ignored"
	@OMNI_UNIT_TESTS_ONLY=true $(GOTEST) -v -short -race -coverprofile=$(COVERAGE_FILE) $(shell go list ./... | grep -v /examples/)

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	@$(GOCLEAN) -testcache
	@OMNI_RUN_INTEGRATION_TESTS=true $(GOTEST) -v -tags=integration -timeout=10m ./...

# Run integration tests with verbose output
test-integration-verbose:
	@echo "Running integration tests with verbose output..."
	@$(GOCLEAN) -testcache
	@VERBOSE=1 OMNI_RUN_INTEGRATION_TESTS=true $(GOTEST) -v -tags=integration -timeout=10m ./...

# Run all NATS tests with verbose output
test-nats:
	@echo "Running all NATS tests with verbose output..."
	@$(GOCLEAN) -testcache
	@echo "Starting NATS container for tests..."
	@docker run -d --name omni-test-nats -p 4222:4222 nats:latest > /dev/null 2>&1 || true
	@sleep 2
	@echo "Running NATS unit tests..."
	@$(GOTEST) -v ./examples/plugins/nats-backend/... || true
	@echo "Running NATS integration tests..."
	@$(GOTEST) -v -tags=integration -timeout=10m ./examples/plugins/nats-backend/... || (docker stop omni-test-nats > /dev/null 2>&1; docker rm omni-test-nats > /dev/null 2>&1; exit 1)
	@echo "Running NATS example tests..."
	@$(GOTEST) -v ./examples/nats-logging/... || true
	@docker stop omni-test-nats > /dev/null 2>&1 || true
	@docker rm omni-test-nats > /dev/null 2>&1 || true
	@echo "All NATS tests completed!"

# Run all Vault tests with verbose output
test-vault:
	@echo "Running all Vault tests with verbose output..."
	@$(GOCLEAN) -testcache
	@echo "Starting Vault container for tests..."
	@docker run -d --name omni-test-vault -p 8200:8200 -e VAULT_DEV_ROOT_TOKEN_ID=test-token hashicorp/vault:latest > /dev/null 2>&1 || true
	@sleep 3
	@echo "Running Vault unit tests..."
	@$(GOTEST) -v ./examples/plugins/vault-backend/... || true
	@echo "Running Vault integration tests..."
	@$(GOTEST) -v -tags=integration -timeout=10m ./examples/plugins/vault-backend/... || (docker stop omni-test-vault > /dev/null 2>&1; docker rm omni-test-vault > /dev/null 2>&1; exit 1)
	@docker stop omni-test-vault > /dev/null 2>&1 || true
	@docker rm omni-test-vault > /dev/null 2>&1 || true
	@echo "All Vault tests completed!"

# Run disk full tests with Docker
test-diskfull:
	@echo "Building disk full test image..."
	@docker build -t omni/diskfull-test:latest -f docker/Dockerfile.diskfull .
	@echo "Running disk full integration tests..."
	@docker run --rm \
		--privileged \
		--cap-add SYS_ADMIN \
		-e OMNI_DISKFULL_TEST_PATH=/test-logs \
		omni/diskfull-test:latest

# Run all test targets
test-all: test-verbose test-race integration test-diskfull
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
	@OMNI_RUN_INTEGRATION_TESTS=true ./scripts/integration --syslog-only

integration-diskfull:
	@echo "Running disk full integration tests..."
	@docker build -t omni/diskfull-test:latest -f docker/Dockerfile.diskfull .
	@docker run --rm \
		--privileged \
		--cap-add SYS_ADMIN \
		-e OMNI_DISKFULL_TEST_PATH=/test-logs \
		omni/diskfull-test:latest

integration-debug:
	@echo "Running integration tests with containers kept alive..."
	@./scripts/integration --keep-containers --verbose


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
	@echo "Opening coverage report in browser..."
	@if command -v open >/dev/null 2>&1; then \
		open $(COVERAGE_HTML); \
	elif command -v xdg-open >/dev/null 2>&1; then \
		xdg-open $(COVERAGE_HTML); \
	elif command -v start >/dev/null 2>&1; then \
		start $(COVERAGE_HTML); \
	else \
		echo "Please open $(COVERAGE_HTML) in your browser manually"; \
	fi

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
