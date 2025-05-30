# Multiple NATS Destinations Testing

This document describes the comprehensive testing scenarios implemented for multiple NATS destinations with the FlexLog NATS plugin.

## Test Coverage

### 1. **TestMultipleNATSDestinations**
Tests basic functionality with multiple NATS destinations configured simultaneously.

**Scenarios Covered:**
- Info level logs → `nats://localhost:4222/logs.app.info?queue=info-processors`
- Error level logs → `nats://localhost:4222/logs.app.error?queue=error-processors`
- Debug logs (sync) → `nats://localhost:4222/logs.app.debug?async=false`
- Metrics → `nats://localhost:4222/metrics.app?batch=50&flush_interval=1000`
- Audit trail → `nats://localhost:4222/audit.app.events?format=json`

**Key Features:**
- Different queue groups for load balancing
- Mixed synchronous and asynchronous publishing
- Various batching configurations
- Different message formats

### 2. **TestMultipleNATSDestinationsWithFailover**
Tests failover scenarios with multiple NATS clusters.

**Scenarios Covered:**
- Primary cluster: `nats://nats1:4222,nats2:4222,nats3:4222/logs.production?max_reconnect=5`
- Secondary cluster: `nats://backup-nats1:4222,backup-nats2:4222/logs.backup?max_reconnect=3`
- Edge cluster: `nats://edge-nats:4222/logs.edge?reconnect_wait=5`

**Key Features:**
- Multiple server URIs per destination
- Different reconnection strategies
- Graceful failover handling

### 3. **TestNATSDestinationRouting**
Tests complex routing patterns with multiple loggers and destinations.

**Logger Types:**
- **Application Logger**: Routes to multiple subjects based on purpose
  - `logs.app.all` - All application logs
  - `logs.app.errors?queue=error-handlers` - Error-specific processing
  - `logs.app.debug?async=false` - Synchronous debug logs

- **Audit Logger**: Dedicated audit trail logging
  - `audit.events?format=json` - Real-time audit events
  - `audit.archive?batch=100&flush_interval=5000` - Batched archival

- **Metrics Logger**: Performance metrics routing
  - `metrics.performance?batch=50` - Standard metrics
  - `metrics.realtime?async=true&batch=1` - Near real-time metrics

**Key Features:**
- Concurrent logging from multiple goroutines
- Different message types (debug, info, error, audit, metrics)
- Subject-specific routing logic
- Performance optimizations per use case

### 4. **TestNATSSubjectHierarchy**
Tests NATS subject hierarchy patterns for flexible subscription.

**Subject Structure:**
```
logs                          # Root
├── logs.app                  # Application logs
│   ├── logs.app.service1     # Service 1 logs
│   ├── logs.app.service2     # Service 2 logs
│   ├── logs.app.service1.error
│   └── logs.app.service2.error
├── logs.system               # System logs
│   └── logs.system.health    # Health checks
├── logs.audit                # Audit logs
└── logs.metrics              # Metrics logs
```

**Wildcard Subscription Examples:**
- `logs.*` → All top-level categories
- `logs.app.*` → All application logs
- `logs.*.error` → All error logs across services
- `logs.app.service1.*` → All logs from service1

### 5. **TestNATSPerformanceWithMultipleDestinations**
Benchmarks performance with multiple destinations and configurations.

**Test Configurations:**
- **Sync Small**: `async=false` - Immediate delivery
- **Async Small**: `batch=10&flush_interval=50` - Low latency
- **Async Medium**: `batch=100&flush_interval=100` - Balanced
- **Async Large**: `batch=500&flush_interval=500` - High throughput
- **Queue Group**: `queue=workers&batch=100` - Load balanced

**Metrics Measured:**
- Messages per second
- Total throughput across all destinations
- Average message size
- Write counts and error rates

## Integration Tests

### 1. **TestMultipleNATSSubjectsIntegration**
Real NATS server integration test (requires running NATS server).

**Features Tested:**
- Actual message delivery to different subjects
- Message format verification (JSON parsing)
- Routing correctness validation
- Synchronous vs asynchronous delivery

### 2. **TestNATSWildcardSubscriptions**
Tests wildcard subscription patterns with real NATS server.

**Patterns Tested:**
- `logs.*` - Single-level wildcard
- `logs.app.*` - Service-specific wildcards
- `logs.*.error` - Cross-service error collection
- `logs.app.>` - Multi-level wildcard (all app logs)
- `logs.app.api.*` - API-specific logs

### 3. **TestNATSQueueGroupLoadBalancing**
Tests load balancing across queue group members.

**Features Verified:**
- Message distribution across multiple workers
- Exactly-once delivery guarantee
- Balanced load distribution (within tolerance)
- Queue group isolation

### 4. **TestComplexRoutingScenario**
Real-world multi-service architecture simulation.

**Services Simulated:**
- **Frontend Service**: `logs.frontend.{level}`
- **API Service**: `logs.api.{version}.{level}`
- **Background Workers**: `logs.worker.{type}.{level}`
- **System Monitoring**: `logs.system.{component}`

**Routing Patterns:**
- Level-based routing (info, error, debug)
- Version-based routing (v1, v2)
- Component-based routing (payment, health, metrics)
- Cross-cutting concerns (audit, alerts)

## Error Handling and Edge Cases

### Connection Failures
- Tests gracefully handle NATS server unavailability
- Proper error reporting without test failures
- Fallback behavior documentation

### Plugin Registration
- Helper function handles duplicate plugin registration
- Shared plugin state across test functions
- Clean registration error handling

### Resource Management
- Proper cleanup of loggers and connections
- Goroutine synchronization with WaitGroups
- Timeout handling for long-running operations

## Running the Tests

### Unit Tests (No NATS Required)
```bash
go test -v ./examples/plugins/nats-backend/... -run "TestMultiple|TestRouting|TestHierarchy|TestPerformance"
```

### Integration Tests (Requires NATS Server)
```bash
# Start NATS server
docker run -d --name nats -p 4222:4222 nats:latest

# Run integration tests
go test -tags=integration -v ./examples/plugins/nats-backend/...
```

### Performance Tests
```bash
go test -v ./examples/plugins/nats-backend/... -run TestNATSPerformance
```

## Key Benefits Demonstrated

1. **Scalability**: Multiple destinations with different configurations
2. **Flexibility**: Various routing patterns and subject hierarchies
3. **Performance**: Optimized configurations for different use cases
4. **Reliability**: Failover and error handling mechanisms
5. **Observability**: Comprehensive logging and metrics collection
6. **Load Balancing**: Queue groups for distributed processing

## Future Enhancements

1. **Schema Validation**: Structured log schema enforcement
2. **Dynamic Routing**: Runtime routing rule changes
3. **Compression**: Message compression for bandwidth optimization
4. **Persistence**: JetStream integration for message persistence
5. **Metrics Export**: Prometheus/OpenTelemetry integration