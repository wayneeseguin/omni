# NATS Backend Testing Guide

This document describes the testing infrastructure for the NATS backend plugin.

## Test Categories

### 1. Unit Tests
Basic tests that don't require a NATS server connection:
- `TestNATSBackendPlugin` - Plugin initialization and metadata
- `TestNATSBackendCreation` - Backend creation without connection
- `TestNATSBackendURIParsing` - URI parsing and configuration
- `TestNATSBackendBuffering` - Message buffering logic

### 2. Integration Tests
Tests that require a running NATS server:
- `TestNATSBackendIntegration` - Basic connectivity and messaging
- `TestNATSBackendQueueGroup` - Queue group load balancing
- `TestNATSBackendBatching` - Batching and flush intervals
- `TestNATSBackendReconnection` - Connection establishment

### 3. Multi-Destination Tests
- `TestNATSMultiDestination` - Multiple NATS destinations
- `TestNATSRoutingByLevel` - Level-based message routing
- `TestNATSClusteredDestinations` - Clustered NATS setup

### 4. Advanced Tests
- `TestNATSErrorRecovery` - Error scenarios and recovery
- `TestNATSMonitoring` - Message monitoring and statistics
- `TestNATSLoadTest` - Performance and load testing

## Running Tests

### Quick Start
```bash
# Run all NATS integration tests
make integration-nats

# Run with verbose output
./scripts/integration --nats-only --verbose

# Keep containers running for debugging
./scripts/integration --nats-only --keep-containers
```

### Using the Debug Script
```bash
# Check status
./scripts/debug-nats status

# Start NATS container
./scripts/debug-nats start

# Run specific test
./scripts/debug-nats test TestNATSBackendIntegration

# Run debug tests with extra logging
./scripts/debug-nats debug

# Monitor NATS logs in real-time
./scripts/debug-nats monitor

# Enter NATS container shell
./scripts/debug-nats shell
```

### Running Individual Tests
```bash
# Run unit tests only
go test -v ./examples/plugins/nats-backend/...

# Run integration tests with specific test
go test -v -tags=integration -run TestNATSBackendQueueGroup ./examples/plugins/nats-backend/...

# Run with debug logging
VERBOSE=1 go test -v -tags=integration -run TestNATS ./examples/plugins/nats-backend/...

# Run load tests (excluded from short tests)
go test -v -tags=integration -run TestNATSLoadTest ./examples/plugins/nats-backend/...
```

## Test Configuration

### Environment Variables
- `VERBOSE=1` - Enable verbose test output
- `OMNI_NATS_TEST_ADDR` - Override default NATS address (default: localhost:4222)

### NATS URI Parameters
The backend supports various URI parameters for testing:
- `queue=<name>` - Queue group name
- `async=<bool>` - Async mode (default: true)
- `batch=<int>` - Batch size (default: 100)
- `flush_interval=<ms>` - Flush interval in milliseconds
- `max_reconnect=<int>` - Maximum reconnection attempts
- `reconnect_wait=<sec>` - Reconnection wait time
- `tls=<bool>` - Enable TLS
- `format=<json|text>` - Message format

Example URIs:
```
nats://localhost:4222/logs.app
nats://localhost:4222/logs.app?queue=workers
nats://localhost:4222/logs.app?batch=500&flush_interval=100
nats://user:pass@localhost:4222/secure.logs?tls=true
```

## Troubleshooting

### Common Issues

1. **Connection Refused**
   ```bash
   # Check if NATS is running
   ./scripts/debug-nats status
   
   # Start NATS if needed
   ./scripts/debug-nats start
   ```

2. **Test Timeouts**
   - Increase timeout: `go test -timeout=30s ...`
   - Check NATS logs: `./scripts/debug-nats logs`

3. **Docker Issues**
   ```bash
   # Check Docker status
   docker info
   
   # Clean up old containers
   docker rm -f omni-test-nats
   ```

### Debug Mode

Run tests with debug tags for extra logging:
```bash
go test -v -tags="integration,debug" -run TestNATSConnectionDebug ./examples/plugins/nats-backend/...
```

This provides detailed information about:
- Connection attempts and failures
- Message publishing details
- Performance metrics
- Error recovery behavior

## Performance Benchmarks

Run performance tests:
```bash
# Basic benchmarks
go test -bench=. ./examples/plugins/nats-backend/...

# Load test with monitoring
./scripts/debug-nats test TestNATSLoadTest
```

Expected performance metrics:
- Messages/sec: > 1,000
- Throughput: > 1 MB/s
- Error rate: < 1%
- Latency: < 100ms average

## CI/CD Integration

The integration tests are designed to work in CI environments:

```yaml
# Example GitHub Actions workflow
- name: Run NATS Integration Tests
  run: |
    make integration-nats
```

The tests will:
1. Automatically start required containers
2. Run all integration tests
3. Clean up resources
4. Report results with proper exit codes