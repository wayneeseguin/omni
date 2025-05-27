# FlexLog API Documentation

FlexLog is a flexible, high-performance logging library for Go with support for multiple destinations, structured logging, and advanced features like sampling, filtering, and rotation.

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Core Concepts](#core-concepts)
- [API Reference](#api-reference)
- [Advanced Features](#advanced-features)
- [Configuration](#configuration)
- [Best Practices](#best-practices)

## Installation

```bash
go get github.com/wayneeseguin/flexlog
```

## Quick Start

```go
package main

import (
    "github.com/wayneeseguin/flexlog"
)

func main() {
    // Create a new logger with default settings
    logger, err := flexlog.NewFlexLog()
    if err != nil {
        panic(err)
    }
    defer logger.Close()

    // Log messages at different levels
    logger.Info("Application started")
    logger.Debug("Debug information", "user", "john", "action", "login")
    logger.Error("An error occurred", "error", err)
}
```

## Core Concepts

### Log Levels

FlexLog supports four log levels:
- `DEBUG` - Detailed information for debugging
- `INFO` - Informational messages
- `WARN` - Warning messages
- `ERROR` - Error messages

### Destinations

A destination represents where logs are written. FlexLog supports multiple concurrent destinations:
- **File**: Write to files with rotation and compression
- **Syslog**: System log integration
- **Custom**: Implement your own destination

### Structured Logging

FlexLog supports structured logging with key-value pairs:

```go
logger.InfoWithFields("User action", map[string]interface{}{
    "user_id": 123,
    "action": "purchase",
    "amount": 99.99,
    "currency": "USD",
})
```

## API Reference

### Creating a Logger

#### NewFlexLog()
```go
func NewFlexLog() (*FlexLog, error)
```
Creates a new logger with default configuration.

#### NewFlexLogWithConfig()
```go
func NewFlexLogWithConfig(config Config) (*FlexLog, error)
```
Creates a new logger with custom configuration.

### Logging Methods

#### Basic Logging
```go
func (f *FlexLog) Debug(args ...interface{})
func (f *FlexLog) Info(args ...interface{})
func (f *FlexLog) Warn(args ...interface{})
func (f *FlexLog) Error(args ...interface{})
```

#### Formatted Logging
```go
func (f *FlexLog) Debugf(format string, args ...interface{})
func (f *FlexLog) Infof(format string, args ...interface{})
func (f *FlexLog) Warnf(format string, args ...interface{})
func (f *FlexLog) Errorf(format string, args ...interface{})
```

#### Structured Logging
```go
func (f *FlexLog) DebugWithFields(message string, fields map[string]interface{})
func (f *FlexLog) InfoWithFields(message string, fields map[string]interface{})
func (f *FlexLog) WarnWithFields(message string, fields map[string]interface{})
func (f *FlexLog) ErrorWithFields(message string, fields map[string]interface{})
```

### Destination Management

#### AddDestination()
```go
func (f *FlexLog) AddDestination(name string, config DestinationConfig) error
```
Adds a new log destination.

#### RemoveDestination()
```go
func (f *FlexLog) RemoveDestination(name string) error
```
Removes a log destination.

#### SetLogLevel()
```go
func (f *FlexLog) SetLogLevel(level LogLevel)
```
Sets the minimum log level.

### Configuration

#### Config Structure
```go
type Config struct {
    // Channel size for async logging
    ChannelSize int
    
    // Default log level
    DefaultLevel LogLevel
    
    // Default format (JSON or Text)
    DefaultFormat Format
    
    // Enable metrics collection
    EnableMetrics bool
    
    // Redaction settings
    Redaction RedactionConfig
    
    // Sampling configuration
    Sampling SamplingConfig
}
```

#### DestinationConfig
```go
type DestinationConfig struct {
    // Backend type (File, Syslog, Custom)
    Backend BackendType
    
    // File-specific settings
    FilePath    string
    MaxSize     int64  // Max file size before rotation
    MaxBackups  int    // Number of backups to keep
    Compress    bool   // Compress rotated files
    
    // Format for this destination
    Format Format
    
    // Minimum log level
    MinLevel LogLevel
    
    // Filters for this destination
    Filters []Filter
}
```

## Advanced Features

### Log Rotation

Automatic log rotation based on file size:

```go
config := DestinationConfig{
    Backend:    BackendFile,
    FilePath:   "/var/log/app.log",
    MaxSize:    100 * 1024 * 1024, // 100MB
    MaxBackups: 5,
    Compress:   true,
}
```

### Sampling

Reduce log volume with sampling:

```go
config := Config{
    Sampling: SamplingConfig{
        Enabled:     true,
        Rate:        0.1,  // Log 10% of messages
        BurstSize:   100,  // Allow bursts of 100 messages
        BurstWindow: time.Minute,
    },
}
```

### Filtering

Filter messages based on criteria:

```go
filter := NewFieldFilter("user_id", "123")
config.Filters = append(config.Filters, filter)
```

### Redaction

Automatically redact sensitive data:

```go
config := Config{
    Redaction: RedactionConfig{
        Enabled: true,
        Patterns: []string{
            `\b\d{16}\b`,           // Credit card numbers
            `\b\d{3}-\d{2}-\d{4}\b`, // SSN
        },
        Fields: []string{"password", "api_key", "token"},
    },
}
```

### Metrics

Track logging performance:

```go
metrics := logger.GetMetrics()
fmt.Printf("Total messages: %d\n", metrics.TotalMessages)
fmt.Printf("Dropped messages: %d\n", metrics.DroppedMessages)
fmt.Printf("Average latency: %v\n", metrics.AverageLatency)
```

### Error Handling

Enhanced error logging with stack traces:

```go
err := someFunction()
if err != nil {
    logger.ErrorWithStack("Operation failed", err)
}
```

## Best Practices

### 1. Use Structured Logging

Instead of:
```go
logger.Info(fmt.Sprintf("User %s performed action %s", userID, action))
```

Use:
```go
logger.Info("User action", "user_id", userID, "action", action)
```

### 2. Set Appropriate Log Levels

- Use DEBUG for detailed debugging information
- Use INFO for general application flow
- Use WARN for recoverable issues
- Use ERROR for failures requiring attention

### 3. Context Propagation

Pass context through your application:
```go
func HandleRequest(ctx context.Context, req Request) {
    logger.InfoContext(ctx, "Processing request", "request_id", req.ID)
    // ... process request
}
```

### 4. Configure Sampling for High-Volume Logs

For high-traffic applications:
```go
config.Sampling = SamplingConfig{
    Enabled:   true,
    Rate:      0.01,  // Log 1% of debug messages
    Levels:    []LogLevel{DEBUG},
}
```

### 5. Use Lazy Evaluation for Expensive Operations

```go
logger.Debug("Complex calculation", "result", Lazy(func() interface{} {
    return expensiveCalculation()
}))
```

### 6. Proper Cleanup

Always close the logger:
```go
defer logger.Close()
```

Or use context for automatic cleanup:
```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
logger.Start(ctx)
```

## Performance Considerations

- FlexLog uses buffered channels to prevent blocking
- File operations use OS-level locking for process safety
- Object pooling reduces GC pressure
- Lazy evaluation defers expensive computations

## Thread Safety

FlexLog is thread-safe and can be used concurrently from multiple goroutines.

## Environment Variables

- `FLEXLOG_CHANNEL_SIZE`: Override default channel size (default: 1000)
- `FLEXLOG_MIN_LEVEL`: Set minimum log level (DEBUG, INFO, WARN, ERROR)

## Migration Guide

See [Migration Guide](./MIGRATION.md) for moving from other logging libraries.