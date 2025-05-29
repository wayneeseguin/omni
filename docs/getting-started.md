# Getting Started with FlexLog

FlexLog is a flexible, high-performance logging library for Go applications. This guide will help you get started quickly.

## Installation

```bash
go get github.com/wayneeseguin/flexlog
```

## Quick Start

### Basic Logger

The simplest way to start logging:

```go
package main

import (
    "log"
    "github.com/wayneeseguin/flexlog"
)

func main() {
    // Create a logger that writes to a file
    logger, err := flexlog.New("/var/log/myapp.log")
    if err != nil {
        log.Fatal(err)
    }
    defer logger.Close()

    // Log some messages
    logger.Info("Application started")
    logger.Debug("Debug information")
    logger.Warn("Warning message")
    logger.Error("Error occurred")
}
```

### Using the Builder Pattern

For more control over configuration:

```go
logger, err := flexlog.NewBuilder().
    WithPath("/var/log/myapp.log").
    WithLevel(flexlog.LevelInfo).
    WithJSON().                           // Use JSON format
    WithRotation(100*1024*1024, 10).     // Rotate at 100MB, keep 10 files
    WithGzipCompression().               // Compress rotated files
    Build()
```

### Using Functional Options

Alternative configuration approach:

```go
logger, err := flexlog.NewWithOptions(
    flexlog.WithPath("/var/log/myapp.log"),
    flexlog.WithLevel(flexlog.LevelInfo),
    flexlog.WithJSON(),
    flexlog.WithRotation(100*1024*1024, 10),
    flexlog.WithProductionDefaults(), // Use production-ready defaults
)
```

## Log Levels

FlexLog supports five log levels:

- `LevelTrace` (0) - Very detailed diagnostic information
- `LevelDebug` (1) - Diagnostic information  
- `LevelInfo` (2) - Informational messages
- `LevelWarn` (3) - Warning messages
- `LevelError` (4) - Error messages

Set the minimum log level:

```go
logger.SetLevel(flexlog.LevelInfo) // Only log Info and above
```

## Structured Logging

Add context to your logs with structured fields:

```go
// Log with fields
logger.WithFields(map[string]interface{}{
    "user_id": 123,
    "action": "login",
    "ip": "192.168.1.100",
}).Info("User authenticated")

// Chain fields
logger.
    WithField("request_id", "abc-123").
    WithField("method", "POST").
    WithField("path", "/api/users").
    Info("API request")

// Log errors with context
err := doSomething()
if err != nil {
    logger.WithError(err).
        WithField("operation", "user_create").
        Error("Operation failed")
}
```

## Output Formats

### Text Format (Default)

Human-readable format:

```
2024-01-15 10:30:45 INFO Application started
2024-01-15 10:30:46 ERROR Database connection failed host=localhost port=5432
```

### JSON Format

Machine-readable format for log aggregation:

```go
logger.SetFormat(flexlog.FormatJSON)
```

Output:
```json
{"timestamp":"2024-01-15T10:30:45Z","level":"INFO","message":"Application started"}
{"timestamp":"2024-01-15T10:30:46Z","level":"ERROR","message":"Database connection failed","host":"localhost","port":5432}
```

## Multiple Destinations

Log to multiple outputs simultaneously:

```go
// Create primary logger
logger, _ := flexlog.New("/var/log/app.log")

// Add additional destinations
logger.AddDestination("/var/log/errors.log")     // Another file
logger.AddDestination("syslog://localhost:514")  // Syslog

// Configure specific destination
logger.EnableDestination("/var/log/errors.log")
logger.DisableDestination("/var/log/errors.log")
```

## Log Rotation

Automatic log rotation based on file size:

```go
logger.SetMaxSize(50 * 1024 * 1024)  // Rotate at 50MB
logger.SetMaxFiles(10)                // Keep 10 rotated files

// Enable compression for rotated files
logger.SetCompression(flexlog.CompressionGzip)
```

## Performance Tuning

### Channel Buffer Size

Control the message queue size:

```go
// Via environment variable
export FLEXLOG_CHANNEL_SIZE=10000

// Or in code
logger, _ := flexlog.NewWithOptions(
    flexlog.WithPath("/var/log/app.log"),
    flexlog.WithChannelSize(10000),
)
```

### Sampling

Reduce log volume in high-throughput scenarios:

```go
// Random sampling - log 10% of messages
logger.SetSampling(flexlog.SamplingRandom, 0.1)

// Interval sampling - log every 100th message
logger.SetSampling(flexlog.SamplingInterval, 100)
```

## Error Handling

Handle logging errors gracefully:

```go
logger.SetErrorHandler(func(err flexlog.LogError) {
    // Handle error (e.g., send alert, fallback logging)
    fmt.Fprintf(os.Stderr, "Logging error: %v\n", err)
})
```

## Context Integration

Use with Go's context for request tracing:

```go
// Add values to context
ctx := flexlog.WithRequestID(context.Background(), "req-123")
ctx = flexlog.WithUserID(ctx, "user-456")

// Log with context
logger.StructuredLogWithContext(ctx, flexlog.LevelInfo, 
    "Processing request", map[string]interface{}{
        "endpoint": "/api/users",
    })
```

## Best Practices

1. **Always defer Close()**: Ensure logs are flushed on shutdown
   ```go
   logger, _ := flexlog.New("/var/log/app.log")
   defer logger.Close()
   ```

2. **Use structured logging**: Add context with fields rather than formatting strings
   ```go
   // Good
   logger.WithField("user_id", userID).Info("User logged in")
   
   // Avoid
   logger.Info(fmt.Sprintf("User %d logged in", userID))
   ```

3. **Set appropriate log levels**: Use Debug/Trace for development, Info and above for production

4. **Handle errors**: Set up error handlers to catch logging issues

5. **Use log rotation**: Prevent disk space issues with automatic rotation

6. **Consider sampling**: For high-volume applications, use sampling to reduce log volume

## Next Steps

- See [examples](../examples/) for real-world usage patterns
- Read the [API documentation](https://pkg.go.dev/github.com/wayneeseguin/flexlog)
- Learn about [advanced features](./advanced-features.md)
- Check out [troubleshooting guide](./troubleshooting.md)