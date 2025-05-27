# FlexLog Examples

This directory contains example programs demonstrating various features of FlexLog.

## Examples Overview

### 1. [Basic Usage](basic/main.go)
- Simple logger creation
- Different log levels (DEBUG, INFO, WARN, ERROR)
- Formatted logging
- Structured logging with fields

```bash
go run examples/basic/main.go
```

### 2. [Multiple Destinations](multiple-destinations/main.go)
- Logging to multiple files simultaneously
- Different formats (JSON/Text) for different destinations
- Level-based filtering per destination
- Metrics collection and display

```bash
go run examples/multiple-destinations/main.go
```

### 3. [Context-Aware Logging](context-aware/main.go)
- Using context for request tracing
- Context cancellation handling
- Request ID propagation
- Simulating request processing

```bash
go run examples/context-aware/main.go
```

### 4. [Advanced Features](advanced-features/main.go)
- Sensitive data redaction
- Log sampling to reduce volume
- Error logging with stack traces
- Lazy evaluation for expensive operations
- Log filtering
- Automatic log rotation and compression

```bash
go run examples/advanced-features/main.go
```

### 5. [Performance Optimized](performance-optimized/main.go)
- High-throughput logging configuration
- Concurrent logging from multiple goroutines
- Zero-allocation logging patterns
- Performance benchmarking
- Object pooling for reduced GC pressure

```bash
go run examples/performance-optimized/main.go
```

## Running All Examples

You can run all examples sequentially:

```bash
for example in basic multiple-destinations context-aware advanced-features performance-optimized; do
    echo "Running $example example..."
    go run examples/$example/main.go
    echo "---"
done
```

## Common Patterns

### Creating a Logger

```go
// Simple logger
logger, err := flexlog.NewFlexLog()

// With configuration
config := flexlog.Config{
    ChannelSize: 2000,
    DefaultLevel: flexlog.INFO,
}
logger, err := flexlog.NewFlexLogWithConfig(config)
```

### Adding Destinations

```go
// File destination
logger.AddDestination("file", flexlog.DestinationConfig{
    Backend:  flexlog.BackendFile,
    FilePath: "app.log",
    Format:   flexlog.FormatJSON,
})

// Syslog destination
logger.AddDestination("syslog", flexlog.DestinationConfig{
    Backend: flexlog.BackendSyslog,
})
```

### Structured Logging

```go
logger.Info("User action",
    "user_id", 123,
    "action", "login",
    "ip", "192.168.1.1",
)
```

### Error Handling

```go
if err != nil {
    logger.ErrorWithStack("Operation failed", err,
        "operation", "database_query",
        "retry_count", 3,
    )
}
```

## Tips for Production Use

1. **Always defer Close()**: Ensure logs are flushed on shutdown
   ```go
   defer logger.Close()
   ```

2. **Use appropriate channel size**: For high-throughput applications
   ```go
   config.ChannelSize = 10000
   ```

3. **Enable metrics**: Monitor logging performance
   ```go
   config.EnableMetrics = true
   ```

4. **Configure sampling**: For high-volume debug logs
   ```go
   config.Sampling.Enabled = true
   config.Sampling.Rate = 0.01 // 1%
   ```

5. **Set up rotation**: Prevent disk space issues
   ```go
   destConfig.MaxSize = 100 * 1024 * 1024 // 100MB
   destConfig.MaxBackups = 5
   destConfig.Compress = true
   ```