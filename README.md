# FlexLog - Flexible Logging for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/wayneeseguin/flexlog.svg)](https://pkg.go.dev/github.com/wayneeseguin/flexlog)
[![MIT License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

FlexLog is a high-performance, extensible logging library for Go applications with support for multiple destinations, structured logging, and distributed systems integration.

## Key Features

### Core Capabilities
- **🔒 Process-Safe Logging**: File locking ensures safe cross-process synchronization
- **⚡ Non-Blocking Architecture**: Asynchronous message processing prevents application blocking
- **📊 Structured Logging**: Rich context with fields, stack traces, and error metadata
- **🎯 Multiple Destinations**: Log to files, syslog, databases, message queues, and more
- **🔌 Plugin System**: Extend with custom backends, formatters, and filters

### Advanced Features
- **📦 Log Management**: Automatic rotation, compression, and cleanup based on size/age
- **🎚️ Flexible Filtering**: Content-based, regex, and custom filtering logic
- **📈 Smart Sampling**: Reduce log volume with interval, random, or consistent sampling
- **🚨 Enhanced Error Handling**: Stack traces, error wrapping, panic recovery, and severity levels
- **🔐 Security Features**: API request/response redaction, sensitive data masking
- **📡 Distributed Logging**: NATS integration for real-time log streaming across systems

### Performance & Reliability
- **💾 Buffered I/O**: Optimized write performance with configurable buffer sizes
- **🔄 Graceful Shutdown**: Context-aware shutdown with timeout support
- **📊 Built-in Metrics**: Track messages logged, bytes written, and errors
- **🛡️ Recovery Mechanisms**: Automatic recovery from transient failures

## Installation

```bash
go get github.com/wayneeseguin/flexlog
```

## Documentation

- 📖 **[Getting Started Guide](docs/getting-started.md)** - Quick introduction and basic usage
- 📚 **[API Reference](docs/API.md)** - Complete API documentation
- 🏗️ **[Architecture Overview](docs/architecture.md)** - Internal design and components
- 🔌 **[Plugin Development](docs/plugins.md)** - Create custom backends and formatters
- 💡 **[Best Practices](docs/best-practices.md)** - Production deployment guidelines
- 🔄 **[Migration Guide](docs/migration.md)** - Migrate from other logging libraries
- 🔧 **[Troubleshooting](docs/troubleshooting.md)** - Common issues and solutions

## Quick Start

### Basic Usage

```go
package main

import (
    "github.com/wayneeseguin/flexlog"
)

func main() {
    // Create a simple file logger
    logger, err := flexlog.New("app.log")
    if err != nil {
        panic(err)
    }
    defer logger.Close()

    // Log messages at different levels
    logger.Info("Application started")
    logger.Debug("Debug mode enabled")
    logger.Warn("Low memory warning")
    logger.Error("Failed to connect to database")
}
```

### Structured Logging

```go
// Log with structured fields for better querying
logger.InfoWithFields("User action", map[string]interface{}{
    "user_id":    123,
    "action":     "login",
    "ip_address": "192.168.1.100",
    "timestamp":  time.Now().Unix(),
})

// Use the Builder pattern for advanced configuration
logger, err := flexlog.NewBuilder().
    WithLevel(flexlog.LevelDebug).
    WithJSON().
    WithDestination("/var/log/app.log").
    WithRotation(10*1024*1024, 5). // 10MB files, keep 5
    Build()
```

### Multiple Destinations

```go
// Create logger with primary destination
logger, err := flexlog.New("/var/log/app.log")
if err != nil {
    panic(err)
}

// Add additional destinations
logger.AddDestination("syslog://localhost:514")
logger.AddDestination("/var/log/app-errors.log")
logger.AddDestination("stdout")

// Destination-specific configuration
logger.SetDestinationEnabled(1, false)  // Disable second destination
logger.SetDestinationFilter(2, flexlog.LevelError) // Only errors to third
```

### Distributed Logging with NATS

```go
// Register NATS plugin
import natsplugin "github.com/wayneeseguin/flexlog/examples/plugins/nats-backend"

plugin := &natsplugin.NATSBackendPlugin{}
plugin.Initialize(nil)
flexlog.RegisterBackendPlugin(plugin)

// Add NATS destinations
logger.AddDestination("nats://localhost:4222/logs.app.info?queue=processors")
logger.AddDestination("nats://cluster:4222/logs.app.error?batch=100&flush_interval=1000")
logger.AddDestination("nats://secure:4222/logs.audit?tls=true&token=secret")

// Log messages are now distributed across NATS subjects
logger.InfoWithFields("Order processed", map[string]interface{}{
    "order_id": "ORD-12345",
    "amount":   99.99,
    "customer": "user@example.com",
})
```

## Advanced Features

### Error Handling & Stack Traces

```go
// Enable stack traces for errors
logger.EnableStackTraces(true)

// Log errors with full context
if err := riskyOperation(); err != nil {
    logger.ErrorWithError("Operation failed", err)
    
    // With severity levels
    logger.ErrorWithErrorAndSeverity("Critical failure", err, flexlog.SeverityCritical)
}

// Wrap errors with additional context
wrappedErr := logger.WrapError(err, "failed to process payment")

// Safe goroutine execution
logger.SafeGo(func() {
    // This function runs with panic recovery
    processInBackground()
})
```

### Filtering & Sampling

```go
// Add custom filters
logger.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
    // Only log messages from specific users
    if userID, ok := fields["user_id"].(int); ok {
        return userID == 123 || userID == 456
    }
    return true
})

// Configure sampling to reduce volume
logger.SetSampling(flexlog.SamplingInterval, 10)    // Every 10th message
logger.SetSampling(flexlog.SamplingRandom, 0.1)     // 10% randomly
logger.SetSampling(flexlog.SamplingConsistent, 0.2) // 20% consistently by key
```

### Log Rotation & Compression

```go
// Configure rotation
logger.SetMaxSize(50 * 1024 * 1024)        // 50MB per file
logger.SetMaxFiles(10)                      // Keep 10 files
logger.SetMaxAge(7 * 24 * time.Hour)       // 7 days retention

// Enable compression
logger.SetCompression(flexlog.CompressionGzip)
logger.SetCompressMinAge(2)                 // Compress after 2 rotations
logger.SetCompressWorkers(3)                // 3 compression workers
```

### Plugin System

```go
// Load plugins from directory
flexlog.SetPluginSearchPaths([]string{
    "./plugins",
    "/usr/local/lib/flexlog/plugins",
})
flexlog.DiscoverAndLoadPlugins()

// Use custom formatter plugin
logger.SetCustomFormatter("xml", map[string]interface{}{
    "include_fields": true,
    "indent": "  ",
})

// Add custom backend plugin
logger.AddDestinationWithPlugin("redis://localhost:6379/0?key=app_logs")
logger.AddDestinationWithPlugin("elasticsearch://localhost:9200/logs")
```

## Production Best Practices

### 1. Configure Channel Size for High Load
```go
// Set before creating loggers
os.Setenv("FLEXLOG_CHANNEL_SIZE", "10000")
```

### 2. Monitor Logger Health
```go
metrics := logger.GetMetrics()
fmt.Printf("Messages logged: %v\n", metrics.MessagesLogged)
fmt.Printf("Messages dropped: %d\n", metrics.MessagesDropped)
fmt.Printf("Error count: %d\n", metrics.ErrorCount)
```

### 3. Graceful Shutdown
```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

if err := logger.Shutdown(ctx); err != nil {
    log.Printf("Shutdown error: %v", err)
}
```

### 4. Use Context for Request Tracing
```go
ctx := context.WithValue(context.Background(), "request_id", "req-123")
logger.WithContext(ctx).Info("Processing request")
```

## Performance Benchmarks

FlexLog is designed for high-performance logging with minimal overhead:

- **Throughput**: 1M+ messages/second (async mode)
- **Latency**: <1μs per log call (with buffering)
- **Memory**: Zero allocations in hot path
- **Concurrency**: Lock-free message passing

See [benchmarks](docs/benchmarks.md) for detailed performance analysis.

## Examples

Explore complete working examples:

- [Basic Usage](examples/basic/) - Simple file logging
- [Multiple Destinations](examples/multiple-destinations/) - Log routing
- [Structured Logging](examples/context-aware/) - Rich context logging
- [NATS Integration](examples/nats-logging/) - Distributed logging
- [API Service](examples/web-service/) - HTTP service with request logging
- [Microservice](examples/microservice/) - Complete microservice example
- [Plugin Development](examples/plugins/) - Custom backend and formatter plugins

## Contributing

Contributions are welcome! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

## License

FlexLog is released under the [MIT License](LICENSE).

## Support

- 🐛 [Report Issues](https://github.com/wayneeseguin/flexlog/issues)
- 💬 [Discussions](https://github.com/wayneeseguin/flexlog/discussions)
- 📧 Contact: wayne@wayneeseguin.com