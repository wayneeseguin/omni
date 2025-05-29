# Migration Guide: FlexLog v1 to v2

This guide helps you migrate from FlexLog v1 API to the new v2 API patterns introduced with builder pattern, functional options, and enhanced interfaces.

## Overview of Changes

FlexLog v2 introduces several improvements while maintaining backward compatibility:

- **Builder Pattern**: Fluent configuration API
- **Functional Options**: Alternative configuration approach
- **Focused Interfaces**: Better separation of concerns
- **Enhanced Error Types**: More informative error handling
- **Dynamic Configuration**: Runtime config changes
- **Context Integration**: Better distributed tracing support

## Basic Logger Migration

### Old API (v1)
```go
// Simple logger creation
logger, err := flexlog.New("/var/log/app.log")

// With configuration
logger, err := flexlog.New("/var/log/app.log")
logger.SetLevel(flexlog.LevelInfo)
logger.SetFormat(flexlog.FormatJSON)
logger.SetMaxSize(100 * 1024 * 1024)
logger.SetMaxFiles(10)
logger.SetCompression(flexlog.CompressionGzip)
```

### New API (v2) - Builder Pattern
```go
// Using builder pattern
logger, err := flexlog.NewBuilder().
    WithPath("/var/log/app.log").
    WithLevel(flexlog.LevelInfo).
    WithJSON().
    WithRotation(100*1024*1024, 10).
    WithGzipCompression().
    Build()
```

### New API (v2) - Functional Options
```go
// Using functional options
logger, err := flexlog.NewWithOptions(
    flexlog.WithPath("/var/log/app.log"),
    flexlog.WithLevel(flexlog.LevelInfo),
    flexlog.WithJSON(),
    flexlog.WithRotation(100*1024*1024, 10),
    flexlog.WithCompression(flexlog.CompressionGzip),
)
```

## Configuration Migration

### Log Levels
```go
// Old API
logger.SetLevel(flexlog.DEBUG)

// New API (runtime change still supported)
logger.SetLevel(flexlog.LevelDebug) // Note: constants renamed

// New API (at creation)
logger, _ := flexlog.NewBuilder().
    WithLevel(flexlog.LevelDebug).
    Build()
```

### Output Format
```go
// Old API
logger.SetFormat(flexlog.FORMAT_JSON)

// New API (runtime change)
logger.SetFormat(flexlog.FormatJSON) // Note: constants renamed

// New API (at creation)
logger, _ := flexlog.NewBuilder().
    WithJSON(). // or WithText()
    Build()
```

### Log Rotation
```go
// Old API
logger.SetMaxSize(50 * 1024 * 1024)
logger.SetMaxFiles(5)

// New API (runtime change still supported)
logger.SetMaxSize(50 * 1024 * 1024)
logger.SetMaxFiles(5)

// New API (at creation)
logger, _ := flexlog.NewBuilder().
    WithRotation(50*1024*1024, 5).
    Build()
```

## Multiple Destinations

### Old API
```go
logger, _ := flexlog.New("/var/log/app.log")
logger.AddDestination("/var/log/errors.log", flexlog.BackendFlock)
```

### New API
```go
// Option 1: Add after creation (still supported)
logger.AddDestination("/var/log/errors.log")

// Option 2: Configure at creation
logger, _ := flexlog.NewBuilder().
    WithPath("/var/log/app.log").
    WithDestination("/var/log/errors.log").
    WithDestination("syslog://localhost:514").
    Build()

// Option 3: Using functional options
logger, _ := flexlog.NewWithOptions(
    flexlog.WithPath("/var/log/app.log"),
    flexlog.WithDestinations(
        "/var/log/errors.log",
        "syslog://localhost:514",
    ),
)
```

## Error Handling

### Old API
```go
err := logger.SomeMethod()
if err != nil {
    // Generic error handling
    log.Printf("Error: %v", err)
}
```

### New API - Enhanced Error Types
```go
err := logger.SomeMethod()
if err != nil {
    // Check specific error types
    if flexlog.IsFileError(err) {
        // Handle file-specific errors
        fmt.Printf("File error: %v\n", err)
    } else if flexlog.IsConfigError(err) {
        // Handle configuration errors
        fmt.Printf("Config error: %v\n", err)
    }
    
    // Get detailed error information
    if logErr, ok := err.(*flexlog.FlexLogError); ok {
        fmt.Printf("Error code: %d, Operation: %s\n", 
            logErr.Code, logErr.Operation)
    }
}

// Set error handler for async errors
logger.SetErrorHandler(func(err flexlog.LogError) {
    fmt.Printf("[%s] %s: %v\n", err.Level, err.Source, err.Err)
})
```

## Structured Logging

### Old API
```go
// Limited structured logging
logger.Log(flexlog.INFO, "User login", map[string]interface{}{
    "user_id": 123,
    "ip": "192.168.1.1",
})
```

### New API - Enhanced Structured Logging
```go
// Fluent interface
logger.WithFields(map[string]interface{}{
    "user_id": 123,
    "ip": "192.168.1.1",
}).Info("User login")

// Chained fields
logger.
    WithField("request_id", "req-123").
    WithField("method", "POST").
    WithField("path", "/api/login").
    Info("API request")

// With validation and normalization
logger, _ := flexlog.NewBuilder().
    WithStructuredOptions(
        flexlog.WithFieldValidation(true),
        flexlog.WithFieldNormalization(true),
        flexlog.WithRequiredFields("request_id", "user_id"),
    ).
    Build()
```

## Context Integration

### Old API
```go
// Manual context value extraction
requestID := ctx.Value("request_id")
logger.Info("Processing request", "request_id", requestID)
```

### New API - Native Context Support
```go
// Add values to context
ctx := flexlog.WithRequestID(context.Background(), "req-123")
ctx = flexlog.WithUserID(ctx, "user-456")
ctx = flexlog.WithTraceID(ctx, "trace-789")

// Log with context
logger.StructuredLogWithContext(ctx, flexlog.LevelInfo, 
    "Processing request", map[string]interface{}{
        "endpoint": "/api/users",
    })

// Create context-aware logger
ctxLogger := flexlog.NewContextLogger(logger, ctx)
ctxLogger.Info("This includes context values automatically")
```

## Dynamic Configuration

### Old API
```go
// Manual configuration updates
logger.SetLevel(flexlog.DEBUG)
logger.SetFormat(flexlog.FORMAT_JSON)
// No hot reload support
```

### New API - Dynamic Configuration
```go
// Enable configuration watching
err := logger.EnableDynamicConfig("/etc/myapp/logging.json", 
    10 * time.Second)

// Configuration file (logging.json)
{
    "level": 2,              // INFO
    "format": 1,             // JSON
    "sampling_strategy": 1,  // Random sampling
    "sampling_rate": 0.1,    // 10% sampling
    "destinations": [
        {
            "uri": "/var/log/app.log",
            "enabled": true
        },
        {
            "uri": "/var/log/errors.log",
            "enabled": true,
            "filter": {
                "min_level": 3  // WARN and above
            }
        }
    ]
}
```

## Sampling Enhancements

### Old API
```go
// Basic sampling
logger.SetSampling(flexlog.SAMPLING_RANDOM, 0.1)
```

### New API - Advanced Sampling
```go
// Random sampling
logger.SetSampling(flexlog.SamplingRandom, 0.1)

// Level-based sampling
logger.EnableLevelBasedSampling(map[int]float64{
    flexlog.LevelDebug: 0.01,  // 1% of debug logs
    flexlog.LevelInfo:  0.1,   // 10% of info logs
    flexlog.LevelWarn:  1.0,   // 100% of warnings
    flexlog.LevelError: 1.0,   // 100% of errors
})

// Pattern-based sampling
logger.EnablePatternBasedSampling([]flexlog.PatternSamplingRule{
    {
        Pattern: "health check",
        Rate:    0.001, // 0.1% of health check logs
    },
    {
        FieldPattern: map[string]string{
            "endpoint": "/metrics",
        },
        Rate: 0.01, // 1% of metrics endpoint logs
    },
})

// Adaptive sampling
logger.EnableAdaptiveSampling(flexlog.AdaptiveSamplingConfig{
    TargetRate:    1000,        // Target 1000 logs/second
    WindowSize:    time.Minute,
    MinSampleRate: 0.001,       // Never sample less than 0.1%
    MaxSampleRate: 1.0,         // Never sample more than 100%
})
```

## Using Interfaces

### Old API
```go
// Tight coupling to FlexLog struct
func processRequest(logger *flexlog.FlexLog) {
    logger.Info("Processing request")
}
```

### New API - Interface-based
```go
// Use interfaces for better testability
func processRequest(logger flexlog.Logger) {
    logger.Info("Processing request")
}

// Different interfaces for different needs
func configureLogging(manager flexlog.Manager) {
    manager.SetMaxSize(100 * 1024 * 1024)
    manager.AddDestination("/var/log/backup.log")
}

func handleErrors(reporter flexlog.ErrorReporter) {
    reporter.SetErrorHandler(func(err flexlog.LogError) {
        // Handle errors
    })
}
```

## Production Defaults

### Old API
```go
// Manual production configuration
logger, _ := flexlog.New("/var/log/app.log")
logger.SetLevel(flexlog.INFO)
logger.SetFormat(flexlog.FORMAT_JSON)
logger.SetMaxSize(100 * 1024 * 1024)
logger.SetMaxFiles(10)
logger.SetCompression(flexlog.COMPRESSION_GZIP)
logger.SetSampling(flexlog.SAMPLING_RANDOM, 0.1)
```

### New API - Production Preset
```go
// Single option for production defaults
logger, _ := flexlog.NewWithOptions(
    flexlog.WithPath("/var/log/app.log"),
    flexlog.WithProductionDefaults(),
)

// Or with builder
logger, _ := flexlog.NewBuilder().
    WithPath("/var/log/app.log").
    WithProductionDefaults().
    Build()
```

## Migration Checklist

- [ ] Update import statements if package moved
- [ ] Replace constant names (e.g., `DEBUG` → `LevelDebug`)
- [ ] Update logger creation to use builder or options
- [ ] Replace error handling with new error types
- [ ] Update structured logging calls
- [ ] Add context integration where appropriate
- [ ] Consider using interfaces instead of concrete types
- [ ] Enable dynamic configuration if needed
- [ ] Update sampling configuration
- [ ] Test thoroughly with race detector enabled

## Backward Compatibility

The v2 API maintains backward compatibility with v1. Your existing code will continue to work, but we recommend migrating to the new patterns for:

- Better performance
- Cleaner code
- More features
- Better error handling
- Improved testability

## Getting Help

If you encounter any issues during migration:

1. Check the [examples](../examples/) directory for working code
2. Read the [API documentation](https://pkg.go.dev/github.com/wayneeseguin/flexlog)
3. File an issue on [GitHub](https://github.com/wayneeseguin/flexlog/issues)
4. See the [troubleshooting guide](./troubleshooting.md)