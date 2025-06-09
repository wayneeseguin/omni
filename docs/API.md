# Omni API Documentation

Omni is a flexible, high-performance logging library for Go with support for multiple destinations, structured logging, and advanced features like sampling, filtering, and rotation.

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Core Concepts](#core-concepts)
- [API Reference](#api-reference)
  - [Logger Creation](#logger-creation)
  - [Logging Methods](#logging-methods)
  - [Destination Management](#destination-management)
  - [Backend Types](#backend-types)
  - [Features](#features)
- [Advanced Features](#advanced-features)
- [Configuration](#configuration)
- [Best Practices](#best-practices)

## Installation

```bash
go get github.com/wayneeseguin/omni
```

Import the packages:
```go
import (
    "github.com/wayneeseguin/omni/pkg/omni"
    "github.com/wayneeseguin/omni/pkg/backends"
    "github.com/wayneeseguin/omni/pkg/features"
)
```

## Quick Start

```go
package main

import (
    "github.com/wayneeseguin/omni/pkg/omni"
)

func main() {
    // Create a simple file logger
    logger, err := omni.New("app.log")
    if err != nil {
        panic(err)
    }
    defer logger.Close()

    // Log messages
    logger.Info("Application started")
    logger.ErrorWithFields("Database error", map[string]interface{}{
        "error": "connection timeout",
        "retry": 3,
    })
}
```

## Core Concepts

### Log Levels

- `LevelDebug` (0) - Detailed debugging information
- `LevelInfo` (1) - Informational messages
- `LevelWarn` (2) - Warning messages
- `LevelError` (3) - Error messages

### Package Structure

- `pkg/omni` - Core logger functionality
- `pkg/backends` - Backend implementations (file, syslog, plugin)
- `pkg/features` - Feature modules (compression, filtering, rotation, etc.)
- `pkg/formatters` - Output formatters (JSON, text, custom)
- `pkg/plugins` - Plugin system for extensibility
- `pkg/types` - Common types and interfaces

## API Reference

### Logger Creation

#### New(path string) (*Omni, error)
Creates a new logger with a file destination.

```go
logger, err := omni.New("/var/log/app.log")
```

#### NewSyslog(address, tag string) (*Omni, error)
Creates a new logger with syslog destination.

```go
logger, err := omni.NewSyslog("localhost:514", "myapp")
```

#### NewBuilder() *Builder
Creates a new builder for advanced configuration.

```go
logger, err := omni.NewBuilder().
    WithLevel(omni.LevelDebug).
    WithJSON().
    WithDestination("/var/log/app.log").
    WithRotation(10*1024*1024, 5).
    Build()
```

### Logging Methods

#### Basic Logging

```go
// Log at different levels
logger.Debug(message string, args ...interface{})
logger.Info(message string, args ...interface{})
logger.Warn(message string, args ...interface{})
logger.Error(message string, args ...interface{})
```

#### Structured Logging

```go
// Log with fields
logger.DebugWithFields(message string, fields map[string]interface{})
logger.InfoWithFields(message string, fields map[string]interface{})
logger.WarnWithFields(message string, fields map[string]interface{})
logger.ErrorWithFields(message string, fields map[string]interface{})

// Example
logger.InfoWithFields("User login", map[string]interface{}{
    "user_id": 123,
    "ip": "192.168.1.1",
    "timestamp": time.Now(),
})
```

#### Error Logging

```go
// Log with error object
logger.ErrorWithError(message string, err error)

// Log with error and severity
logger.ErrorWithErrorAndSeverity(message string, err error, severity int)

// Wrap error with context
wrappedErr := logger.WrapError(err, "additional context")
```

### Destination Management

#### AddDestination(path string) error
Adds a new destination to the logger.

```go
logger.AddDestination("/var/log/errors.log")
logger.AddDestination("syslog://localhost:514")
logger.AddDestination("stdout")
```

#### RemoveDestination(index int) error
Removes a destination by index.

```go
logger.RemoveDestination(1)
```

#### SetDestinationEnabled(index int, enabled bool) error
Enables or disables a specific destination.

```go
logger.SetDestinationEnabled(0, false) // Disable first destination
```

### Backend Types

#### File Backend

Standard file backend with basic functionality:

```go
backend, err := backends.NewFileBackend("/var/log/app.log")
```

#### File Backend with Rotation (Disk Full Handling)

Enhanced file backend with automatic disk full recovery:

```go
// Create rotation manager
rotMgr := features.NewRotationManager()
rotMgr.SetMaxFiles(5)     // Keep only 5 rotated files
rotMgr.SetMaxAge(7 * 24 * time.Hour) // 7 days retention

// Create backend with disk full handling
backend, err := backends.NewFileBackendWithRotation("/var/log/app.log", rotMgr)
backend.SetMaxRetries(3)  // Retry up to 3 times on disk full

// Set error handler for monitoring
backend.SetErrorHandler(func(source, dest, msg string, err error) {
    if strings.Contains(msg, "disk full") {
        // Alert operations team
        alertOps("Disk full on " + dest)
    }
})

// Disk full behavior:
// 1. Detects disk full errors (ENOSPC, "no space left", etc.)
// 2. Automatically rotates current log file
// 3. Removes oldest rotated logs to free space
// 4. Retries the failed write operation
// 5. Continues normal operation after recovery
```

#### Syslog Backend

```go
backend, err := backends.NewSyslogBackend("tcp", "localhost:514", "myapp")
```

### Features

#### Rotation

```go
// Set rotation parameters
logger.SetMaxSize(50 * 1024 * 1024)        // 50MB per file
logger.SetMaxFiles(10)                      // Keep 10 files
logger.SetMaxAge(7 * 24 * time.Hour)       // 7 days retention

// Manual rotation
logger.Rotate()
```

#### Compression

```go
// Enable compression
logger.SetCompression(omni.CompressionGzip)
logger.SetCompressMinAge(2)                 // Compress after 2 rotations
logger.SetCompressWorkers(3)                // 3 compression workers
```

#### Filtering

```go
// Add custom filter
logger.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
    // Return true to log, false to skip
    return level >= omni.LevelWarn
})

// Content-based filtering
logger.SetContentFilter([]string{"password", "secret", "token"})
```

#### Sampling

```go
// Configure sampling strategies
logger.SetSampling(omni.SamplingInterval, 10)    // Every 10th message
logger.SetSampling(omni.SamplingRandom, 0.1)     // 10% randomly
logger.SetSampling(omni.SamplingConsistent, 0.2) // 20% consistently by key
```

#### Redaction

```go
// Enable API redaction
logger.EnableRedaction(true)

// Add custom redaction patterns
logger.AddRedactionPattern(`\b\d{4}-\d{4}-\d{4}-\d{4}\b`, "[CARD]")
logger.AddRedactionPattern(`\b\d{3}-\d{2}-\d{4}\b`, "[SSN]")
```

## Advanced Features

### Context-Aware Logging

```go
// Create context-aware logger
ctx := context.Background()
ctxLogger := logger.WithContext(ctx)

// Use with timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
logger.InfoContext(ctx, "Operation started")
```

### Performance Optimization

```go
// Adjust channel buffer size
os.Setenv("OMNI_CHANNEL_SIZE", "10000")

// Enable batch processing
logger.SetBatchSize(100)
logger.SetFlushInterval(time.Second)
```

### Error Recovery

```go
// Enable panic recovery
logger.EnablePanicRecovery(true)

// Safe goroutine execution
logger.SafeGo(func() {
    // This function runs with panic recovery
    riskyOperation()
})
```

### Metrics

```go
// Get logger statistics
stats := logger.GetStats()
fmt.Printf("Messages logged: %d\n", stats.MessagesLogged)
fmt.Printf("Bytes written: %d\n", stats.BytesWritten)
fmt.Printf("Errors: %d\n", stats.Errors)
```

## Configuration

### Environment Variables

- `OMNI_CHANNEL_SIZE` - Message channel buffer size (default: 1000)
- `OMNI_FLUSH_INTERVAL` - Flush interval for buffered writes
- `OMNI_DEFAULT_LEVEL` - Default log level

### Builder Pattern

```go
logger, err := omni.NewBuilder().
    WithLevel(omni.LevelDebug).
    WithFormatter(formatters.NewJSONFormatter()).
    WithDestination("/var/log/app.log").
    WithRotation(10*1024*1024, 5).
    WithCompression(true).
    WithFilter(customFilter).
    WithSampling(omni.SamplingRandom, 0.1).
    Build()
```

## Best Practices

1. **Always defer Close()**: Ensure proper cleanup
   ```go
   logger, err := omni.New("app.log")
   if err != nil {
       return err
   }
   defer logger.Close()
   ```

2. **Use structured logging**: Include context as fields
   ```go
   logger.InfoWithFields("Order processed", map[string]interface{}{
       "order_id": orderID,
       "user_id": userID,
       "amount": amount,
   })
   ```

3. **Handle disk full scenarios**: Use rotation backend for critical logs
   ```go
   rotMgr := features.NewRotationManager()
   backend, _ := backends.NewFileBackendWithRotation(path, rotMgr)
   ```

4. **Monitor errors**: Set up error handlers
   ```go
   logger.SetErrorHandler(func(err error) {
       // Alert monitoring system
   })
   ```

5. **Use appropriate log levels**: Reserve ERROR for actual errors
   ```go
   logger.Debug("Detailed trace information")
   logger.Info("Normal operation events")
   logger.Warn("Warning conditions")
   logger.Error("Error conditions requiring attention")
   ```