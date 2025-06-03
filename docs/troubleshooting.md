# Omni Troubleshooting Guide

This guide helps you diagnose and resolve common issues with Omni.

## Common Issues

### 1. Logs Not Appearing

#### Symptom
Your application seems to be running, but no logs are being written to the log file.

#### Possible Causes and Solutions

**A. Logger Not Properly Initialized**
```go
// Check if logger was created successfully
logger, err := omni.New("/var/log/app.log")
if err != nil {
    // This error often gets ignored
    log.Fatalf("Failed to create logger: %v", err)
}
```

**B. Log Level Too High**
```go
// Check current log level
fmt.Printf("Current log level: %d\n", logger.GetLevel())

// If you're trying to log Debug messages but level is Info or higher
logger.SetLevel(omni.LevelDebug)
```

**C. Channel Buffer Full**
```go
// Check channel usage
metrics := logger.GetMetrics()
fmt.Printf("Channel usage: %.2f%%\n", metrics.ChannelUsage * 100)

// If near 100%, increase buffer size
export OMNI_CHANNEL_SIZE=10000
```

**D. Logger Closed Prematurely**
```go
// Bad: Logger closes immediately
func init() {
    logger, _ := omni.New("/var/log/app.log")
    defer logger.Close() // Closes as soon as init() returns!
}

// Good: Close at program exit
var logger *omni.Omni

func main() {
    var err error
    logger, err = omni.New("/var/log/app.log")
    if err != nil {
        log.Fatal(err)
    }
    defer logger.Close()
    
    // Run application
}
```

### 2. Permission Denied Errors

#### Symptom
```
Error: open /var/log/app.log: permission denied
```

#### Solutions

**A. Check File Permissions**
```bash
# Check permissions
ls -la /var/log/app.log

# Fix permissions (adjust as needed)
sudo touch /var/log/app.log
sudo chown $USER:$USER /var/log/app.log
chmod 644 /var/log/app.log
```

**B. Check Directory Permissions**
```bash
# Ensure directory exists and is writable
sudo mkdir -p /var/log
sudo chmod 755 /var/log
```

**C. Use User-Writable Location**
```go
// For development/testing
homeDir, _ := os.UserHomeDir()
logPath := filepath.Join(homeDir, "logs", "app.log")

// Ensure directory exists
os.MkdirAll(filepath.Dir(logPath), 0755)

logger, err := omni.New(logPath)
```

### 3. File Lock Errors

#### Symptom
```
Error: resource temporarily unavailable
```

#### Cause
Another process has locked the log file.

#### Solutions

**A. Check for Other Processes**
```bash
# Find processes using the file
lsof /var/log/app.log

# Or with fuser
fuser /var/log/app.log
```

**B. Use Different Destinations**
```go
// Each instance uses a different file
instanceID := os.Getenv("INSTANCE_ID")
logPath := fmt.Sprintf("/var/log/app-%s.log", instanceID)
logger, _ := omni.New(logPath)
```

**C. Use Syslog Backend**
```go
// Syslog doesn't have file locking issues
logger.AddDestination("syslog://localhost:514")
```

### 4. High Memory Usage

#### Symptom
Application memory usage grows continuously.

#### Possible Causes and Solutions

**A. Channel Buffer Too Large**
```go
// Check current buffer size
metrics := logger.GetMetrics()
fmt.Printf("Channel capacity: %d\n", metrics.ChannelCapacity)

// Use reasonable buffer size
logger, _ := omni.NewBuilder().
    WithChannelSize(1000). // Instead of 100000
    Build()
```

**B. Compression Queue Backlog**
```go
// Monitor compression queue
// If files aren't being compressed fast enough
logger.SetCompression(omni.CompressionNone) // Disable temporarily
```

**C. Too Many Destinations**
```go
// List all destinations
destinations := logger.ListDestinations()
fmt.Printf("Active destinations: %d\n", len(destinations))

// Remove unnecessary ones
logger.RemoveDestination("backup-destination")
```

### 5. Logs Being Lost

#### Symptom
Some log messages are missing from output files.

#### Solutions

**A. Check Sampling Configuration**
```go
// Verify sampling settings
rate := logger.GetSamplingRate()
fmt.Printf("Sampling rate: %.2f\n", rate)

// Disable sampling to see all logs
logger.SetSampling(omni.SamplingNone, 0)
```

**B. Ensure Proper Shutdown**
```go
// Always flush before exit
func cleanup() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    if err := logger.Shutdown(ctx); err != nil {
        // Force flush if shutdown fails
        logger.FlushAll()
    }
}
```

**C. Check Error Handler**
```go
// Set up error monitoring
logger.SetErrorHandler(func(err omni.LogError) {
    fmt.Fprintf(os.Stderr, "Logging error: %v\n", err)
})
```

### 6. Rotation Not Working

#### Symptom
Log files grow beyond the configured size limit.

#### Solutions

**A. Check Rotation Configuration**
```go
// Verify settings
fmt.Printf("Max size: %d\n", logger.GetMaxSize())
fmt.Printf("Max files: %d\n", logger.GetMaxFiles())

// Ensure they're set
logger.SetMaxSize(100 * 1024 * 1024) // 100MB
logger.SetMaxFiles(10)
```

**B. Check Write Permissions for Rotated Files**
```bash
# Ensure directory is writable for rotation
ls -la /var/log/
# Should see rotated files like:
# app.log.20240115-103045.000
# app.log.20240115-093022.000.gz
```

**C. Force Rotation (Testing)**
```go
// For testing rotation
logger.ForceRotate()
```

### 7. JSON Parsing Errors

#### Symptom
Log aggregation tools can't parse JSON logs.

#### Solutions

**A. Verify Format Setting**
```go
// Check current format
if logger.GetFormat() != omni.FormatJSON {
    logger.SetFormat(omni.FormatJSON)
}
```

**B. Check for Invalid Characters**
```go
// Ensure fields don't contain invalid JSON
logger.WithFields(map[string]interface{}{
    "message": strings.ReplaceAll(userInput, "\n", "\\n"),
    "data": base64.StdEncoding.EncodeToString(binaryData),
}).Info("Processed data")
```

**C. Validate JSON Output**
```bash
# Test JSON validity
tail -n 1 /var/log/app.log | jq .
```

### 8. Performance Issues

#### Symptom
Logging is causing application slowdown.

#### Diagnostic Steps

**1. Check Metrics**
```go
func diagnosePerformance(logger *omni.Omni) {
    metrics := logger.GetMetrics()
    
    fmt.Printf("Total messages: %d\n", metrics.TotalMessages)
    fmt.Printf("Dropped messages: %d\n", metrics.DroppedMessages)
    fmt.Printf("Channel usage: %.2f%%\n", metrics.ChannelUsage * 100)
    fmt.Printf("Write latency: %v\n", metrics.AvgWriteLatency)
}
```

**2. Profile the Application**
```go
import _ "net/http/pprof"

func init() {
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()
}

// Check with: go tool pprof http://localhost:6060/debug/pprof/profile
```

**3. Optimize Configuration**
```go
// For high-performance scenarios
logger, _ := omni.NewBuilder().
    WithPath("/var/log/app.log").
    WithChannelSize(10000).
    WithBatching(100, 10*time.Millisecond).
    WithCompression(omni.CompressionNone). // Compress separately
    WithSampling(omni.SamplingRandom, 0.1).
    Build()
```

## Debugging Techniques

### Enable Debug Output

```go
// Temporary debug helper
func debugLogger(logger *omni.Omni) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        metrics := logger.GetMetrics()
        log.Printf("Omni Debug - Messages: %d, Errors: %d, Channel: %.1f%%, Destinations: %d",
            metrics.TotalMessages,
            metrics.ErrorCount,
            metrics.ChannelUsage * 100,
            len(logger.ListDestinations()))
    }
}
```

### Trace Individual Messages

```go
// Add trace ID to track specific messages
traceID := generateTraceID()
logger.WithField("trace_id", traceID).Info("Start of operation")

// Later, search for this trace ID in logs
grep "trace_id.*${traceID}" /var/log/app.log
```

### Test in Isolation

```go
// Minimal test case
func testLogging() error {
    // Create temporary logger
    tmpDir, _ := os.MkdirTemp("", "omni-test")
    defer os.RemoveAll(tmpDir)
    
    logPath := filepath.Join(tmpDir, "test.log")
    logger, err := omni.New(logPath)
    if err != nil {
        return fmt.Errorf("create logger: %w", err)
    }
    defer logger.Close()
    
    // Test logging
    logger.Info("Test message")
    
    // Force flush and check
    if err := logger.Flush(); err != nil {
        return fmt.Errorf("flush: %w", err)
    }
    
    // Verify file exists and has content
    content, err := os.ReadFile(logPath)
    if err != nil {
        return fmt.Errorf("read log: %w", err)
    }
    
    if !bytes.Contains(content, []byte("Test message")) {
        return fmt.Errorf("message not found in log")
    }
    
    return nil
}
```

## Getting More Help

### 1. Enable Verbose Error Reporting
```go
logger.SetErrorHandler(func(err omni.LogError) {
    // Print full error details
    fmt.Fprintf(os.Stderr, 
        "[%s] Omni Error:\n"+
        "  Level: %s\n"+
        "  Source: %s\n"+
        "  Error: %v\n"+
        "  Time: %s\n"+
        "  Stack: %s\n",
        time.Now().Format(time.RFC3339),
        err.Level,
        err.Source,
        err.Err,
        err.Time,
        err.Stack)
})
```

### 2. Collect Diagnostic Information
When reporting issues, include:

```bash
# System information
uname -a
go version

# File permissions
ls -la /path/to/logfile
ls -la /path/to/logdir/

# Disk space
df -h /path/to/logdir/

# Process limits
ulimit -a

# Open files
lsof -p $(pgrep your-app) | grep -E '(log|omni)'
```

### 3. Create Minimal Reproduction
```go
// Minimal example that reproduces the issue
package main

import (
    "github.com/wayneeseguin/omni"
    "log"
)

func main() {
    logger, err := omni.New("/tmp/test.log")
    if err != nil {
        log.Fatal(err)
    }
    defer logger.Close()
    
    // Add minimal code that reproduces the issue
    logger.Info("Test message")
}
```

### 4. Check GitHub Issues
Search existing issues: https://github.com/wayneeseguin/omni/issues

### 5. File a Bug Report
Include:
- Omni version
- Go version
- Operating system
- Minimal reproduction code
- Expected vs actual behavior
- Any error messages
- Relevant configuration