# Omni Best Practices Guide

This guide covers best practices for using Omni effectively in production environments.

## Logger Initialization

### DO: Initialize Once and Share
```go
// Good: Create once and share across your application
var logger *omni.Omni

func init() {
    var err error
    logger, err = omni.NewBuilder().
        WithPath("/var/log/app.log").
        WithLevel(omni.LevelInfo).
        WithJSON().
        WithRotation(100*1024*1024, 10).
        Build()
    if err != nil {
        log.Fatal(err)
    }
}
```

### DON'T: Create Multiple Logger Instances
```go
// Bad: Creating new loggers for each operation
func handleRequest() {
    logger, _ := omni.New("/var/log/app.log")
    defer logger.Close()
    logger.Info("Processing request")
}
```

## Resource Management

### Always Defer Close
```go
// Good: Ensure proper cleanup
logger, err := omni.New("/var/log/app.log")
if err != nil {
    return err
}
defer logger.Close()
```

### Graceful Shutdown
```go
// Good: Use context for graceful shutdown
func shutdown(logger *omni.Omni) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    if err := logger.Shutdown(ctx); err != nil {
        log.Printf("Error during shutdown: %v", err)
    }
}
```

## Error Handling

### Set Up Error Handlers
```go
// Good: Handle logging errors gracefully
logger.SetErrorHandler(func(err omni.LogError) {
    // Send to monitoring system
    metrics.IncrementCounter("logging.errors", map[string]string{
        "level": err.Level,
        "source": err.Source,
    })
    
    // Fallback logging
    fmt.Fprintf(os.Stderr, "[%s] Logging error in %s: %v\n", 
        time.Now().Format(time.RFC3339), err.Source, err.Err)
})
```

### Check Error Types
```go
// Good: Handle specific error types
if err := logger.AddDestination("/var/log/backup.log"); err != nil {
    if omni.IsFileError(err) {
        // Handle file permission issues
        return fmt.Errorf("check file permissions: %w", err)
    } else if omni.IsConfigError(err) {
        // Handle configuration issues
        return fmt.Errorf("invalid configuration: %w", err)
    }
    return err
}
```

## Structured Logging

### Use Fields Instead of String Formatting
```go
// Good: Structured fields
logger.WithFields(map[string]interface{}{
    "user_id": userID,
    "action": "login",
    "ip": request.RemoteAddr,
}).Info("User authenticated")

// Bad: String concatenation
logger.Info(fmt.Sprintf("User %d authenticated from %s", userID, request.RemoteAddr))
```

### Consistent Field Names
```go
// Good: Define constants for field names
const (
    FieldUserID    = "user_id"
    FieldRequestID = "request_id"
    FieldMethod    = "method"
    FieldPath      = "path"
    FieldDuration  = "duration_ms"
)

logger.WithFields(map[string]interface{}{
    FieldUserID:    userID,
    FieldRequestID: requestID,
    FieldMethod:    "POST",
    FieldPath:      "/api/users",
    FieldDuration:  latency.Milliseconds(),
}).Info("API request completed")
```

### Add Context Early
```go
// Good: Create logger with context at request start
func handleRequest(w http.ResponseWriter, r *http.Request) {
    requestID := generateRequestID()
    
    // Create request-scoped logger
    reqLogger := logger.WithFields(map[string]interface{}{
        "request_id": requestID,
        "method":     r.Method,
        "path":       r.URL.Path,
        "remote_ip":  r.RemoteAddr,
    })
    
    // Use reqLogger throughout request handling
    reqLogger.Info("Request started")
    
    // Pass to other functions
    processRequest(reqLogger, r)
    
    reqLogger.WithField("status", 200).Info("Request completed")
}
```

## Performance Optimization

### Use Appropriate Log Levels
```go
// Good: Use debug/trace for development, info and above for production
if logger.IsLevelEnabled(omni.LevelDebug) {
    logger.WithField("payload", request.Body).Debug("Request payload")
}

// Production configuration
logger.SetLevel(omni.LevelInfo)
```

### Enable Sampling for High-Volume Logs
```go
// Good: Sample non-critical logs
logger.EnableLevelBasedSampling(map[int]float64{
    omni.LevelDebug: 0.01,  // 1% of debug logs
    omni.LevelInfo:  0.1,   // 10% of info logs
    omni.LevelWarn:  1.0,   // 100% of warnings
    omni.LevelError: 1.0,   // 100% of errors
})

// For specific patterns
logger.EnablePatternBasedSampling([]omni.PatternSamplingRule{
    {
        Pattern: "health check",
        Rate:    0.001, // Sample 0.1% of health checks
    },
})
```

### Buffer Size Tuning
```go
// Good: Adjust buffer size based on load
// For high-throughput applications
export OMNI_CHANNEL_SIZE=10000

// Or in code
logger, _ := omni.NewBuilder().
    WithPath("/var/log/app.log").
    WithChannelSize(10000).
    Build()
```

## Log Rotation and Retention

### Configure Rotation Properly
```go
// Good: Set appropriate rotation limits
logger, _ := omni.NewBuilder().
    WithPath("/var/log/app.log").
    WithRotation(100*1024*1024, 10). // 100MB files, keep 10
    WithGzipCompression().            // Compress rotated files
    Build()

// For time-based cleanup
logger.SetMaxAge(7 * 24 * time.Hour) // Keep logs for 7 days
```

### Monitor Disk Usage
```go
// Good: Monitor log metrics
go func() {
    ticker := time.NewTicker(time.Minute)
    defer ticker.Stop()
    
    for range ticker.C {
        metrics := logger.GetMetrics()
        
        // Monitor channel usage
        if metrics.ChannelUsage > 0.8 {
            alert("Log channel usage high", metrics.ChannelUsage)
        }
        
        // Monitor error rate
        errorRate := float64(metrics.ErrorCount) / float64(metrics.TotalMessages)
        if errorRate > 0.01 {
            alert("High logging error rate", errorRate)
        }
    }
}()
```

## Context Integration

### Use Context for Distributed Tracing
```go
// Good: Propagate trace information
func handleRequest(ctx context.Context) {
    // Extract trace ID from incoming request
    traceID := extractTraceID(ctx)
    
    // Add to context
    ctx = omni.WithTraceID(ctx, traceID)
    ctx = omni.WithRequestID(ctx, generateRequestID())
    
    // Log with context
    logger.StructuredLogWithContext(ctx, omni.LevelInfo, 
        "Processing request", nil)
    
    // Pass context to downstream services
    callService(ctx)
}
```

### Create Context-Aware Loggers
```go
// Good: Use context logger for automatic field inclusion
func processUser(ctx context.Context, userID string) {
    ctx = omni.WithUserID(ctx, userID)
    ctxLogger := omni.NewContextLogger(logger, ctx)
    
    ctxLogger.Info("Processing user") // Automatically includes user_id
    
    // Pass to other functions
    updateUserProfile(ctxLogger, userID)
}
```

## Security Considerations

### Redact Sensitive Information
```go
// Good: Use redaction for sensitive data
logger, _ := omni.NewBuilder().
    WithPath("/var/log/app.log").
    WithRedaction(omni.RedactionConfig{
        Enabled: true,
        Patterns: []string{
            `\b\d{16}\b`,              // Credit card numbers
            `password[:=]\s*"[^"]*"`,   // Password fields
            `\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`, // Emails
        },
        Replacement: "[REDACTED]",
        Fields: []string{"password", "credit_card", "ssn"},
    }).
    Build()
```

### Avoid Logging Sensitive Data
```go
// Good: Explicitly exclude sensitive fields
func logUserAction(user User, action string) {
    logger.WithFields(map[string]interface{}{
        "user_id": user.ID,
        "action":  action,
        // Don't log: user.Password, user.SSN, user.CreditCard
    }).Info("User action")
}
```

## Testing

### Use Interfaces for Testability
```go
// Good: Accept logger interface
func processOrder(logger omni.Logger, order Order) error {
    logger.WithField("order_id", order.ID).Info("Processing order")
    // ... process order ...
    return nil
}

// In tests
func TestProcessOrder(t *testing.T) {
    mockLogger := &MockLogger{}
    err := processOrder(mockLogger, testOrder)
    assert.NoError(t, err)
    assert.Equal(t, 1, mockLogger.InfoCallCount)
}
```

### Test Log Output
```go
// Good: Verify log behavior
func TestLoggingBehavior(t *testing.T) {
    // Create test logger
    tmpFile := filepath.Join(t.TempDir(), "test.log")
    logger, err := omni.New(tmpFile)
    require.NoError(t, err)
    defer logger.Close()
    
    // Log test message
    logger.WithField("test", true).Info("Test message")
    
    // Flush and verify
    require.NoError(t, logger.Flush())
    
    // Read and verify log content
    content, err := os.ReadFile(tmpFile)
    require.NoError(t, err)
    assert.Contains(t, string(content), "Test message")
    assert.Contains(t, string(content), `"test":true`)
}
```

## Monitoring and Alerting

### Export Metrics
```go
// Good: Expose metrics for monitoring
func exposeMetrics(logger *omni.Omni) {
    http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
        metrics := logger.GetMetrics()
        
        fmt.Fprintf(w, "# HELP omni_messages_total Total messages logged\n")
        fmt.Fprintf(w, "omni_messages_total %d\n", metrics.TotalMessages)
        
        fmt.Fprintf(w, "# HELP omni_errors_total Total logging errors\n")
        fmt.Fprintf(w, "omni_errors_total %d\n", metrics.ErrorCount)
        
        fmt.Fprintf(w, "# HELP omni_channel_usage Channel buffer usage\n")
        fmt.Fprintf(w, "omni_channel_usage %f\n", metrics.ChannelUsage)
    })
}
```

### Set Up Alerts
```go
// Good: Alert on critical conditions
func monitorLogger(logger *omni.Omni) {
    errorChan := logger.GetErrors()
    
    for err := range errorChan {
        // Check severity
        if err.Severity == omni.ErrorLevelHigh {
            alert := Alert{
                Title:    "Critical logging error",
                Message:  fmt.Sprintf("%s: %v", err.Source, err.Err),
                Severity: "critical",
            }
            sendAlert(alert)
        }
    }
}
```

## Common Pitfalls to Avoid

### 1. Not Checking IsLevelEnabled
```go
// Bad: Expensive operation always runs
logger.Debug(fmt.Sprintf("User data: %+v", expensiveUserDataDump()))

// Good: Check level first
if logger.IsLevelEnabled(omni.LevelDebug) {
    logger.Debug(fmt.Sprintf("User data: %+v", expensiveUserDataDump()))
}
```

### 2. Logging in Hot Paths
```go
// Bad: Logging in tight loops
for _, item := range items {
    logger.Debug("Processing item", "id", item.ID)
    process(item)
}

// Good: Aggregate or sample
processed := 0
for _, item := range items {
    process(item)
    processed++
}
logger.Info("Processed items", "count", processed)
```

### 3. Not Setting Production Defaults
```go
// Bad: Using development settings in production
logger, _ := omni.New("/var/log/app.log")

// Good: Use production defaults
logger, _ := omni.NewBuilder().
    WithPath("/var/log/app.log").
    WithProductionDefaults().
    Build()
```

### 4. Ignoring Errors
```go
// Bad: Ignoring errors
logger.AddDestination("/var/log/backup.log")

// Good: Handle errors appropriately
if err := logger.AddDestination("/var/log/backup.log"); err != nil {
    // Fall back to stderr or alert ops team
    fmt.Fprintf(os.Stderr, "Failed to add backup destination: %v\n", err)
}
```

## Summary

Following these best practices will help you:

- Maximize performance and minimize resource usage
- Ensure reliable logging in production
- Make logs useful for debugging and monitoring
- Maintain security and compliance
- Build testable and maintainable code

Remember: logs are often the first place you look when debugging issues. Invest in good logging practices early, and they'll pay dividends when you need them most.