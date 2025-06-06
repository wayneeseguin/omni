# Omni API Documentation

Omni is a flexible, high-performance logging library for Go with support for multiple destinations, structured logging, and advanced features like sampling, filtering, and rotation.

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
go get github.com/wayneeseguin/omni
```

Import the package in your code:
```go
import "github.com/wayneeseguin/omni/pkg/omni"
```

## Quick Start

```go
package main

import (
    "github.com/wayneeseguin/omni/pkg/omni"
)

func main() {
    // Create a new logger with default settings
    logger, err := omni.NewOmni()
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

Omni supports four log levels:
- `DEBUG` - Detailed information for debugging
- `INFO` - Informational messages
- `WARN` - Warning messages
- `ERROR` - Error messages

### Package Structure

Omni is organized into the following packages:
- `pkg/omni` - Core logger functionality
- `pkg/backends` - Backend implementations (file, syslog, plugin)
- `pkg/features` - Feature modules (compression, filtering, rotation, etc.)
- `pkg/formatters` - Output formatters (JSON, text, custom)
- `pkg/plugins` - Plugin system for extensibility
- `pkg/types` - Common types and interfaces

### Destinations

A destination represents where logs are written. Omni supports multiple concurrent destinations:
- **File**: Write to files with rotation and compression
- **Syslog**: System log integration
- **Custom**: Implement your own destination

### Structured Logging

Omni supports structured logging with key-value pairs:

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

#### NewOmni()
```go
func NewOmni() (*Omni, error)
```
Creates a new logger with default configuration.

#### NewOmniWithConfig()
```go
func NewOmniWithConfig(config Config) (*Omni, error)
```
Creates a new logger with custom configuration.

### Logging Methods

#### Basic Logging
```go
func (f *Omni) Debug(args ...interface{})
func (f *Omni) Info(args ...interface{})
func (f *Omni) Warn(args ...interface{})
func (f *Omni) Error(args ...interface{})
```

#### Formatted Logging
```go
func (f *Omni) Debugf(format string, args ...interface{})
func (f *Omni) Infof(format string, args ...interface{})
func (f *Omni) Warnf(format string, args ...interface{})
func (f *Omni) Errorf(format string, args ...interface{})
```

#### Structured Logging
```go
func (f *Omni) DebugWithFields(message string, fields map[string]interface{})
func (f *Omni) InfoWithFields(message string, fields map[string]interface{})
func (f *Omni) WarnWithFields(message string, fields map[string]interface{})
func (f *Omni) ErrorWithFields(message string, fields map[string]interface{})
```

### Destination Management

#### AddDestination()
```go
func (f *Omni) AddDestination(name string, config DestinationConfig) error
```
Adds a new log destination.

#### RemoveDestination()
```go
func (f *Omni) RemoveDestination(name string) error
```
Removes a log destination.

#### SetLogLevel()
```go
func (f *Omni) SetLogLevel(level LogLevel)
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
    // Backend type (File, Syslog, Plugin)
    Backend string
    
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
    Backend:    "file",
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

- Omni uses buffered channels to prevent blocking
- File operations use OS-level locking for process safety
- Object pooling reduces GC pressure
- Lazy evaluation defers expensive computations

## Thread Safety

Omni is thread-safe and can be used concurrently from multiple goroutines.

## Environment Variables

- `OMNI_CHANNEL_SIZE`: Override default channel size (default: 1000)
- `OMNI_MIN_LEVEL`: Set minimum log level (DEBUG, INFO, WARN, ERROR)

## Migration Guide

See [Migration Guide](./MIGRATION.md) for moving from other logging libraries.