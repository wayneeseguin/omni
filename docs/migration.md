# Migration Guide

This guide helps you migrate from popular logging libraries to Omni.

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

**Omni:**
```go
import "github.com/wayneeseguin/omni/pkg/omni"

logger, _ := omni.New("stdout")
logger.SetFormat(omni.FormatJSON)
```

### Structured Logging

**slog:**
```go
logger.Info("user login", 
    slog.String("user", "john"),
    slog.Int("user_id", 123))
```

**Omni:**
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

**Omni:**
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

**Omni:**
```go
import "github.com/wayneeseguin/omni/pkg/omni"

logger, _ := omni.New("app.log")
logger.SetLevel(omni.LevelInfo)
logger.SetFormat(omni.FormatJSON)
```

### Fields

**logrus:**
```go
log.WithFields(logrus.Fields{
    "user": "john",
    "action": "login",
}).Info("User logged in")
```

**Omni:**
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

**Omni:**
```go
// Implement as a plugin backend
import "github.com/wayneeseguin/omni/pkg/plugins"

plugin := &CustomPlugin{Writer: customWriter}
plugins.RegisterPlugin("custom", plugin)
logger.AddDestination("plugin://custom")
```

## From zap

### Basic Setup

**zap:**
```go
logger, _ := zap.NewProduction()
defer logger.Sync()
sugar := logger.Sugar()
```

**Omni:**
```go
import "github.com/wayneeseguin/omni/pkg/omni"

logger, _ := omni.NewBuilder().
    WithLevel(omni.LevelInfo).
    WithJSON().
    Build()
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

**Omni:**
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

**Omni:**
```go
// Omni uses efficient structured logging
logger.InfoWithFields("logged in", map[string]interface{}{
    "user": username,
})
```

## From zerolog

### Basic Setup

**zerolog:**
```go
zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
```

**Omni:**
```go
import "github.com/wayneeseguin/omni/pkg/omni"

logger, _ := omni.New("stdout")
logger.SetFormat(omni.FormatJSON)
```

### Structured Logging

**zerolog:**
```go
logger.Info().
    Str("user", "john").
    Int("user_id", 123).
    Msg("User logged in")
```

**Omni:**
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

**Omni:**
```go
logger.SetSampling(omni.SamplingInterval, 10) // Every 10th message
// or
logger.SetSampling(omni.SamplingRandom, 0.1) // 10% randomly
```

## Common Patterns

### Global Logger

**Old pattern:**
```go
var log = logrus.New()
// or
var logger, _ = zap.NewProduction()
```

**Omni pattern:**
```go
import "github.com/wayneeseguin/omni/pkg/omni"

var logger *omni.Omni

func init() {
    var err error
    logger, err = omni.New("/var/log/app.log")
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

**Omni built-in:**
```go
logger.SetMaxSize(100 * 1024 * 1024) // 100MB
logger.SetMaxFiles(3)
logger.SetCompression(omni.CompressionGzip)
```

### Multiple Outputs

**Old pattern with io.MultiWriter:**
```go
file, _ := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
multi := io.MultiWriter(file, os.Stdout)
logger.SetOutput(multi)
```

**Omni native:**
```go
logger, _ := omni.New("app.log")
logger.AddDestination("stdout")
```

### Error Handling

**Old pattern:**
```go
if err != nil {
    log.WithError(err).Error("Operation failed")
}
```

**Omni:**
```go
if err != nil {
    logger.ErrorWithFields("Operation failed", map[string]interface{}{
        "error": err,
    })
    // Or with enhanced error
    logger.ErrorWithError("Operation failed", err)
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

// Omni
import "github.com/wayneeseguin/omni/pkg/omni"

// Use a temporary file for testing
testLogger, _ := omni.New("/tmp/test.log")
// Or use stdout
testLogger, _ := omni.New("stdout")
```

## Feature Comparison

| Feature | logrus | zap | zerolog | slog | Omni |
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

1. **Dependencies**: Replace old logger imports with `github.com/wayneeseguin/omni/pkg/omni`
2. **Initialization**: Update logger creation to use Omni constructors
3. **Configuration**: Convert logger configuration to Omni Config struct
4. **Log Calls**: Update log method calls (usually minor syntax changes)
5. **Outputs**: Convert output configuration to Omni destinations
6. **Testing**: Update test mocks and assertions
7. **Cleanup**: Ensure `defer logger.Close()` is called

## Getting Help

If you encounter any issues during migration:

1. Check the [API Documentation](./API.md)
2. Review the [examples](../examples/) directory
3. Open an issue on GitHub with your specific use case