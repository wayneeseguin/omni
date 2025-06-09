# Testing Guide

This document describes how to run tests for the Omni logging library.

## Test Types

### Unit Tests

Unit tests run quickly without requiring external services. They are the default when running tests. The Omni project uses a custom `testing.Unit()` helper to detect when running in unit test mode.

```bash
# Run all unit tests
make test

# Run unit tests with verbose output
make test-verbose

# Run unit tests with race detector
make test-race
```

### Integration Tests

Integration tests require external services like syslog servers. They are skipped by default in unit test runs.

```bash
# Run all integration tests with Docker
make test-integration

# Run only syslog integration tests
make integration-syslog

# Run integration tests manually with environment setup
OMNI_RUN_INTEGRATION_TESTS=true go test -v ./...
```

## Syslog Tests

Syslog tests are designed to work in multiple scenarios:

1. **Unit Test Mode (default)**: Tests that require a syslog server are skipped
2. **Integration Test Mode**: Tests attempt to connect to a syslog server
3. **Docker Integration Mode**: A syslog container is started for testing

### Running Syslog Tests

```bash
# Unit tests (syslog tests are skipped)
OMNI_UNIT_TESTS_ONLY=true go test -v ./pkg/backends/
OMNI_UNIT_TESTS_ONLY=true go test -v ./pkg/omni/

# Or use the -short flag (backward compatible)
go test -v -short ./pkg/backends/
go test -v -short ./pkg/omni/

# Integration tests with manual syslog server
OMNI_RUN_INTEGRATION_TESTS=true go test -v ./pkg/backends/ -run TestSyslog

# Integration tests with Docker (recommended)
make integration-syslog
```

### Environment Variables

- `OMNI_UNIT_TESTS_ONLY`: Set to "true" to run only unit tests (skip integration tests)
- `OMNI_RUN_INTEGRATION_TESTS`: Set to "true" to enable integration tests
- `OMNI_SYSLOG_TEST_ADDR`: Address of syslog server for testing (e.g., "localhost:514")
- `OMNI_SYSLOG_TEST_PROTO`: Protocol for syslog connection ("tcp" or "udp")

### Test Mode Detection

The testing framework uses the following logic to determine test mode:
1. If `-short` flag is set, runs in unit test mode (backward compatibility)
2. If `OMNI_UNIT_TESTS_ONLY=true`, runs in unit test mode
3. If `OMNI_RUN_INTEGRATION_TESTS=false`, runs in unit test mode
4. If `OMNI_RUN_INTEGRATION_TESTS=true`, runs in integration test mode
5. Default is unit test mode

## Test Organization

Tests are organized to minimize disruption:

- Connection failure tests expect and handle errors gracefully
- Tests that require external services check for their availability
- Integration tests are clearly separated from unit tests
- Tests use `testhelpers.SkipIfUnit()` to skip integration tests in unit mode
- Tests use `testhelpers.SkipIfIntegration()` to skip unit-only tests in integration mode

## Examples

### Run all tests excluding integration
```bash
make test
```

### Run syslog tests with a local syslog server
```bash
# Start your syslog server on port 514
OMNI_RUN_INTEGRATION_TESTS=true OMNI_SYSLOG_TEST_ADDR=localhost:514 go test -v ./pkg/backends/ -run TestSyslog
```

### Run all integration tests with Docker
```bash
make integration
```

### Keep containers running after tests
```bash
./scripts/integration --keep-containers
```

## Troubleshooting

### Syslog Connection Errors

If you see errors like:
- `dial unix /dev/log: connect: no such file or directory`
- `dial tcp [::1]:514: connect: connection refused`

These are expected when running integration tests without a syslog server. Either:
1. Run tests in short mode: `go test -short ./...`
2. Set up a syslog server and provide its address
3. Use Docker integration: `make integration-syslog`

### Docker Issues

If Docker tests fail:
1. Ensure Docker is installed and running
2. Check that ports 4222 (NATS), 8200 (Vault), and 5514 (syslog) are available
3. Run with verbose output: `./scripts/integration --verbose`