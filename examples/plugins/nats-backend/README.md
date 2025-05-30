# NATS Backend Plugin for FlexLog

The NATS backend plugin enables FlexLog to send logs to NATS messaging servers, providing distributed logging capabilities for microservices and cloud-native applications.

## Features

- **Distributed Logging**: Send logs to NATS subjects for centralized collection
- **Queue Groups**: Load balance log processing across multiple consumers
- **Async Publishing**: Non-blocking message publishing with configurable batching
- **Connection Management**: Automatic reconnection with exponential backoff
- **TLS Support**: Secure connections with certificate validation
- **Authentication**: Username/password and token-based authentication
- **Flexible Formatting**: JSON or text message formats

## Installation

```bash
go get github.com/wayneeseguin/flexlog
go get github.com/nats-io/nats.go
```

## Usage

### Basic Usage

```go
import (
    "github.com/wayneeseguin/flexlog"
    _ "github.com/wayneeseguin/flexlog/examples/plugins/nats-backend"
)

func main() {
    logger := flexlog.New()
    defer logger.CloseAll()
    
    // Add NATS destination
    err := logger.AddDestination("nats://localhost:4222/logs.myapp")
    if err != nil {
        log.Fatal(err)
    }
    
    logger.Info("Application started")
}
```

### URI Format

```
nats://[user:pass@]host1[:port1][,host2[:port2]]/subject[?options]
```

### Query Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `queue` | string | - | Queue group name for load balancing |
| `async` | bool | true | Enable async publishing |
| `batch` | int | 100 | Batch size for buffering |
| `flush_interval` | int | 100 | Flush interval in milliseconds |
| `max_reconnect` | int | 10 | Maximum reconnection attempts |
| `reconnect_wait` | int | 2 | Reconnect wait time in seconds |
| `tls` | bool | false | Enable TLS connection |
| `format` | string | json | Message format (json or text) |

### Examples

#### Load Balanced Logging

```go
// Multiple instances use the same queue group
// Each message is delivered to only one consumer
logger.AddDestination("nats://nats:4222/logs.app?queue=log-processors")
```

#### High-Throughput Configuration

```go
// Larger batches and longer flush intervals for high volume
uri := "nats://nats:4222/logs.production?batch=500&flush_interval=200"
logger.AddDestination(uri)
```

#### Secure Connection

```go
// TLS with authentication
uri := "nats://user:pass@secure-nats:4222/logs.secure?tls=true&max_reconnect=20"
logger.AddDestination(uri)
```

#### Clustered NATS

```go
// Connect to NATS cluster
uri := "nats://nats1:4222,nats2:4222,nats3:4222/logs.cluster"
logger.AddDestination(uri)
```

### Message Formats

#### JSON Format (Default)

```json
{
  "timestamp": "2025-01-29T10:30:45.123Z",
  "level": "INFO",
  "message": "User logged in",
  "hostname": "app-server-01",
  "process": "api-service",
  "fields": {
    "user_id": 12345,
    "ip_address": "192.168.1.100",
    "session_id": "abc-123"
  }
}
```

#### Text Format

```
[2025-01-29T10:30:45.123Z] [INFO] User logged in hostname=app-server-01 process=api-service user_id=12345 ip_address=192.168.1.100 session_id=abc-123
```

## Integration with NATS

### Setting Up Consumers

```go
// Subscribe to logs
nc, _ := nats.Connect(nats.DefaultURL)
defer nc.Close()

// Simple subscriber
nc.Subscribe("logs.myapp", func(msg *nats.Msg) {
    log.Printf("Received: %s", msg.Data)
})

// Queue subscriber for load balancing
nc.QueueSubscribe("logs.myapp", "log-processors", func(msg *nats.Msg) {
    // Process log message
    var logEntry map[string]interface{}
    json.Unmarshal(msg.Data, &logEntry)
    // Handle log entry...
})
```

### Subject Hierarchies

Use NATS subject hierarchies for flexible routing:

```go
// Log to different subjects based on level
logger.AddDestination("nats://localhost:4222/logs.app.info")
logger.AddDestination("nats://localhost:4222/logs.app.error")

// Subscribe with wildcards
nc.Subscribe("logs.app.*", handler)  // All app logs
nc.Subscribe("logs.*.error", handler) // All error logs
```

## Performance Tuning

### Batching Strategy

- **Small batches (10-50)**: Low latency, good for real-time monitoring
- **Medium batches (100-200)**: Balanced performance, default setting
- **Large batches (500-1000)**: High throughput, some latency

### Flush Intervals

- **Short (10-50ms)**: Near real-time delivery
- **Medium (100-200ms)**: Good balance
- **Long (500-1000ms)**: Maximum throughput

### Example Configuration

```go
// Real-time monitoring
"nats://localhost:4222/logs.monitoring?batch=10&flush_interval=20"

// High-volume application logs  
"nats://localhost:4222/logs.app?batch=500&flush_interval=500"

// Balanced configuration
"nats://localhost:4222/logs.service?batch=100&flush_interval=100"
```

## Testing

### Unit Tests

```bash
cd examples/plugins/nats-backend
go test -v
```

### Integration Tests

Requires a running NATS server:

```bash
# Start NATS server
docker run -d --name nats -p 4222:4222 nats:latest

# Run integration tests
go test -tags=integration -v
```

## Monitoring

Monitor NATS plugin performance:

```go
// Get destination metrics
for _, dest := range logger.GetDestinations() {
    if dest.Backend == flexlog.BackendPlugin {
        metrics := dest.GetMetrics()
        log.Printf("Messages sent: %d", metrics.WriteCount)
        log.Printf("Bytes written: %d", metrics.BytesWritten)
        log.Printf("Errors: %d", metrics.Errors)
    }
}
```

## Troubleshooting

### Connection Issues

- Verify NATS server is running: `nc -zv localhost 4222`
- Check authentication credentials
- Ensure firewall allows connection
- Review NATS server logs

### Message Delivery

- Use NATS monitoring tools to verify message flow
- Check queue group configuration
- Verify subject names match between publisher and subscriber
- Monitor for connection drops and reconnections

### Performance

- Adjust batch size based on message volume
- Tune flush intervals for latency requirements
- Monitor NATS server resources
- Consider using NATS JetStream for persistence

## Future Enhancements

- JetStream support for message persistence
- Message compression
- Dynamic routing based on log content
- Metrics export to Prometheus
- Schema validation for structured logs