# Migration Guide

This guide helps you migrate from popular logging libraries to FlexLog.

## Table of Contents

- [From log/slog](#from-logslog)
- [From logrus](#from-logrus)
- [From zap](#from-zap)
- [From zerolog](#from-zerolog)
- [Common Patterns](#common-patterns)

## From log/slog

### Basic Setup

**slog:**
```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
```

**FlexLog:**
```go
logger, _ := flexlog.NewFlexLog()
logger.AddDestination("stdout", flexlog.DestinationConfig{
    Backend: flexlog.BackendFile,
    FilePath: "/dev/stdout",
    Format: flexlog.FormatJSON,
})
```

### Structured Logging

**slog:**
```go
logger.Info("user login", 
    slog.String("user", "john"),
    slog.Int("user_id", 123))
```

**FlexLog:**
```go
logger.Info("user login",
    "user", "john",
    "user_id", 123)
```

### With Context

**slog:**
```go
logger.InfoContext(ctx, "processing request")
```

**FlexLog:**
```go
logger.InfoContext(ctx, "processing request")
```

## From logrus

### Basic Setup

**logrus:**
```go
log := logrus.New()
log.SetFormatter(&logrus.JSONFormatter{})
log.SetOutput(file)
log.SetLevel(logrus.InfoLevel)
```

**FlexLog:**
```go
logger, _ := flexlog.NewFlexLog()
logger.SetLogLevel(flexlog.INFO)
logger.AddDestination("file", flexlog.DestinationConfig{
    Backend: flexlog.BackendFile,
    FilePath: "app.log",
    Format: flexlog.FormatJSON,
    MinLevel: flexlog.INFO,
})
```

### Fields

**logrus:**
```go
log.WithFields(logrus.Fields{
    "user": "john",
    "action": "login",
}).Info("User logged in")
```

**FlexLog:**
```go
logger.Info("User logged in",
    "user", "john",
    "action", "login")
```

### Hooks

**logrus:**
```go
log.AddHook(customHook)
```

**FlexLog:**
```go
// Implement as a custom destination
logger.AddDestination("custom", flexlog.DestinationConfig{
    Backend: flexlog.BackendCustom,
    CustomWriter: customWriter,
})
```

## From zap

### Basic Setup

**zap:**
```go
logger, _ := zap.NewProduction()
defer logger.Sync()
sugar := logger.Sugar()
```

**FlexLog:**
```go
config := flexlog.Config{
    DefaultLevel: flexlog.INFO,
    DefaultFormat: flexlog.FormatJSON,
}
logger, _ := flexlog.NewFlexLogWithConfig(config)
defer logger.Close()
```

### Structured Logging

**zap:**
```go
logger.Info("failed to fetch URL",
    zap.String("url", url),
    zap.Int("attempt", 3),
    zap.Duration("backoff", time.Second))
```

**FlexLog:**
```go
logger.Info("failed to fetch URL",
    "url", url,
    "attempt", 3,
    "backoff", time.Second)
```

### Performance Mode

**zap:**
```go
logger := zap.NewExample()
logger.With(
    zap.String("user", username),
).Info("logged in")
```

**FlexLog:**
```go
// Use lazy evaluation for expensive operations
logger.Info("logged in",
    "user", username,
    "expensive_data", flexlog.Lazy(func() interface{} {
        return computeExpensiveData()
    }))
```

## From zerolog

### Basic Setup

**zerolog:**
```go
zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
```

**FlexLog:**
```go
logger, _ := flexlog.NewFlexLog()
logger.AddDestination("stdout", flexlog.DestinationConfig{
    Backend: flexlog.BackendFile,
    FilePath: "/dev/stdout",
    Format: flexlog.FormatJSON,
})
```

### Structured Logging

**zerolog:**
```go
logger.Info().
    Str("user", "john").
    Int("user_id", 123).
    Msg("User logged in")
```

**FlexLog:**
```go
logger.Info("User logged in",
    "user", "john", 
    "user_id", 123)
```

### Sampling

**zerolog:**
```go
sampled := logger.Sample(&zerolog.BasicSampler{N: 10})
```

**FlexLog:**
```go
config := flexlog.Config{
    Sampling: flexlog.SamplingConfig{
        Enabled: true,
        Rate: 0.1, // 10%
    },
}
```

## Common Patterns

### Global Logger

**Old pattern:**
```go
var log = logrus.New()
// or
var logger, _ = zap.NewProduction()
```

**FlexLog pattern:**
```go
var logger *flexlog.FlexLog

func init() {
    var err error
    logger, err = flexlog.NewFlexLog()
    if err != nil {
        panic(err)
    }
}
```

### Log Rotation

**Using lumberjack:**
```go
logger.SetOutput(&lumberjack.Logger{
    Filename:   "/var/log/myapp.log",
    MaxSize:    100, // megabytes
    MaxBackups: 3,
    MaxAge:     28, // days
    Compress:   true,
})
```

**FlexLog built-in:**
```go
logger.AddDestination("rotating", flexlog.DestinationConfig{
    Backend:    flexlog.BackendFile,
    FilePath:   "/var/log/myapp.log",
    MaxSize:    100 * 1024 * 1024, // bytes
    MaxBackups: 3,
    Compress:   true,
})
```

### Multiple Outputs

**Old pattern with io.MultiWriter:**
```go
file, _ := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
multi := io.MultiWriter(file, os.Stdout)
logger.SetOutput(multi)
```

**FlexLog native:**
```go
logger.AddDestination("file", flexlog.DestinationConfig{
    Backend: flexlog.BackendFile,
    FilePath: "app.log",
})
logger.AddDestination("stdout", flexlog.DestinationConfig{
    Backend: flexlog.BackendFile,
    FilePath: "/dev/stdout",
})
```

### Error Handling

**Old pattern:**
```go
if err != nil {
    log.WithError(err).Error("Operation failed")
}
```

**FlexLog:**
```go
if err != nil {
    logger.Error("Operation failed", "error", err)
    // Or with stack trace
    logger.ErrorWithStack("Operation failed", err)
}
```

### Testing

**Mock logger pattern:**
```go
// Old
type MockLogger struct {
    *logrus.Logger
    LastEntry *logrus.Entry
}

// FlexLog
testLogger, _ := flexlog.NewFlexLog()
testLogger.AddDestination("memory", flexlog.DestinationConfig{
    Backend: flexlog.BackendMemory, // For testing
})
```

## Feature Comparison

| Feature | logrus | zap | zerolog | slog | FlexLog |
|---------|--------|-----|---------|------|---------|
| Structured Logging | ✓ | ✓ | ✓ | ✓ | ✓ |
| Multiple Outputs | ✓ | ✓ | ✓ | ✓ | ✓ |
| JSON Format | ✓ | ✓ | ✓ | ✓ | ✓ |
| Log Rotation | ✗ | ✗ | ✗ | ✗ | ✓ |
| Compression | ✗ | ✗ | ✗ | ✗ | ✓ |
| Sampling | ✗ | ✓ | ✓ | ✗ | ✓ |
| Redaction | ✗ | ✗ | ✗ | ✗ | ✓ |
| Process-Safe | ✗ | ✗ | ✗ | ✗ | ✓ |
| Context Support | ✓ | ✓ | ✓ | ✓ | ✓ |
| Zero Allocation | ✗ | ✓ | ✓ | ✗ | ✓ |

## Migration Checklist

1. **Dependencies**: Replace old logger imports with `github.com/wayneeseguin/flexlog`
2. **Initialization**: Update logger creation to use FlexLog constructors
3. **Configuration**: Convert logger configuration to FlexLog Config struct
4. **Log Calls**: Update log method calls (usually minor syntax changes)
5. **Outputs**: Convert output configuration to FlexLog destinations
6. **Testing**: Update test mocks and assertions
7. **Cleanup**: Ensure `defer logger.Close()` is called

## Getting Help

If you encounter any issues during migration:

1. Check the [API Documentation](./API.md)
2. Review the [examples](../examples/) directory
3. Open an issue on GitHub with your specific use case