# Flex Logger ("flexlog")

A Go library for flexibly logging with multiple destination backends as an option:
- Unix flock-based synchronization on Unix-like systems
- syslog

**Note:** This library is intended for use on Unix-like systems (e.g., Linux, macOS) and uses file locking for cross-process synchronization.

The primary use case is to allow for background logging to a single log from multiple processes without blocking or corrupting log entries.

## Installation

To install `flexlog`, use the following command:

```sh
go get github.com/wayneeseguin/flexlog
```

## Features

- **Process-Safe Logging**: Uses file locking for cross-process synchronization
- **Standard Log Levels**: Supports DEBUG, INFO, WARN, ERROR levels
- **Structured Logging**: Supports structured logging with fields
- **Log Rotation**: Automatic log rotation based on file size
- **Multiple Output Destinations**: Log to multiple destinations simultaneously
- **Format Options**: JSON or text format with customizable options
- **Stack Traces**: Optional stack trace capture for errors or all log levels
- **Log Filtering**: Filter logs based on various criteria
- **Log Sampling**: Sample logs using different strategies
- **Error Handling**: Enhanced error logging with severity levels
- **Compression**: Automatic compression of rotated logs
- **API Logging**: Methods for safely logging API requests/responses with redaction
- **Safe Goroutines**: Helper for executing goroutines with panic recovery

## Basic Usage

Here's a simple example of how to use `flexlog`:

```go
package main

import (
    "github.com/wayneeseguin/flexlog"
)

func main() {
    logger, err := flexlog.Newflexlog("app.log")
    if err != nil {
        panic(err)
    }
    defer logger.Close()

    logger.log("Hello, %s!", "world")
}
```

## Log Levels

flexlog supports standard logging levels:

```go
logger, err := flexlog.Newflexlog("app.log")
if err != nil {
    panic(err)
}
defer logger.Close()

// Set minimum log level (default is INFO)
logger.SetLevel(flexlog.LevelDebug) // Options: LevelDebug, LevelInfo, LevelWarn, LevelError

// Log at different levels
logger.Debug("Debug message")
logger.Debugf("Formatted %s message", "debug")

logger.Info("Info message")
logger.Infof("Formatted %s message", "info")

logger.Warn("Warning message")
logger.Warnf("Formatted %s message", "warning")

logger.Error("Error message")
logger.Errorf("Formatted %s message", "error")
```

## Structured Logging

For more detailed logs, use structured logging with fields:

```go
// Structured logging with fields
logger.DebugWithFields("User logged in", map[string]interface{}{
    "user_id": 123,
    "ip":      "192.168.1.1",
})

logger.InfoWithFields("Payment processed", map[string]interface{}{
    "amount":    199.99,
    "currency":  "USD",
    "payment_id": "pay_123456",
})

logger.WarnWithFields("Rate limit approaching", map[string]interface{}{
    "current_rate": 95,
    "limit":        100,
    "user_id":      123,
})

logger.ErrorWithFields("Database connection failed", map[string]interface{}{
    "db_host":   "db.example.com",
    "error_code": "CONNECTION_REFUSED",
})
```

## Log Rotation and Cleanup

Configure log rotation and cleanup:

```go
// Set maximum log file size (in bytes)
logger.SetMaxSize(10 * 1024 * 1024) // 10MB

// Set maximum number of log files to keep
logger.SetMaxFiles(5)

// Set maximum age for log files (0 disables age-based cleanup)
logger.SetMaxAge(7 * 24 * time.Hour) // 7 days

// Set cleanup interval
logger.SetCleanupInterval(1 * time.Hour)

// Run cleanup manually
logger.RunCleanup()
```

## Multiple Output Destinations

Send logs to multiple destinations:

```go
// Add stdout as a destination
logger.AddDestination("stdout", os.Stdout)

// Add a network writer
conn, _ := net.Dial("tcp", "logserver:1234")
logger.AddDestination("network", conn)

// Disable a destination temporarily
logger.DisableDestination("network")

// Re-enable a destination
logger.EnableDestination("network")

// Remove a destination
logger.RemoveDestination("network")

// List all destinations
destinations := logger.ListDestinations()
```

## Log Formats and Formatting Options

Configure log formats:

```go
// Use JSON format
logger.SetFormat(flexlog.FormatJSON)

// Or use text format (default)
logger.SetFormat(flexlog.FormatText)

// Customize formatting options
logger.SetFormatOption(flexlog.FormatOptionTimestampFormat, "2006-01-02T15:04:05.000Z07:00")
logger.SetFormatOption(flexlog.FormatOptionIncludeLevel, true)
logger.SetFormatOption(flexlog.FormatOptionLevelFormat, flexlog.LevelFormatSymbol)
logger.SetFormatOption(flexlog.FormatOptionIndentJSON, true)
```

## Stack Traces

Configure stack trace capture:

```go
// Enable stack traces for error logs
logger.EnableStackTraces(true)

// Capture stack traces for all log levels, not just errors
logger.SetCaptureAllStacks(true)

// Set maximum stack trace buffer size
logger.SetStackSize(8192)
```

## Enhanced Error Handling

flexlog provides enhanced error handling:

```go
// Log an error with stack trace
err := someFunction()
logger.ErrorWithError("Operation failed", err)

// Log an error with a severity level
logger.ErrorWithErrorAndSeverity("Critical system failure", err, flexlog.SeverityCritical)

// Wrap an error with stack trace
wrappedErr := logger.WrapError(err, "failed to process request")

// Get the root cause of a wrapped error
rootErr := logger.CauseOf(wrappedErr)

// Format an error with stack trace for output
verboseErr := logger.FormatErrorVerbose(wrappedErr)

// Log a recovered panic
defer func() {
    if r := recover(); r != nil {
        logger.LogPanic(r)
    }
}()

// Run a function in a goroutine with panic recovery
logger.SafeGo(func() {
    // This code will run in a goroutine with panic recovery
    riskyOperation()
})
```

## Log Filtering

Filter logs based on various criteria:

```go
// Add a custom filter
logger.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
    // Only log messages containing "important"
    return strings.Contains(message, "important")
})

// Filter logs with specific field value
logger.SetFieldFilter("user_id", 123, 456) // Only log entries with user_id 123 or 456

// Filter by level and field
logger.SetLevelFieldFilter(flexlog.LevelError, "service", "payment")

// Filter using regex
logger.SetRegexFilter(regexp.MustCompile(`user_id=(1|2|3)\d{2}`))

// Exclude logs matching a pattern
logger.SetExcludeRegexFilter(regexp.MustCompile(`health_check`))

// Clear all filters
logger.ClearFilters()
```

## Log Sampling

Sample logs to reduce volume:

```go
// Log every 10th message
logger.SetSampling(flexlog.SamplingInterval, 10)

// Log 10% of messages randomly
logger.SetSampling(flexlog.SamplingRandom, 0.1)

// Use consistent sampling based on message content
logger.SetSampling(flexlog.SamplingConsistent, 0.2)

// Customize the sampling key function
logger.SetSampleKeyFunc(func(level int, message string, fields map[string]interface{}) string {
    // Use user_id as the sampling key if available
    if fields != nil {
        if userID, ok := fields["user_id"].(int); ok {
            return fmt.Sprintf("%d", userID)
        }
    }
    return message
})
```

## Log Compression

Configure compression for rotated logs:

```go
// Enable gzip compression for rotated logs
logger.SetCompression(flexlog.CompressionGzip)

// Set minimum rotation age before compressing
logger.SetCompressMinAge(2) // Compress logs that are at least 2 rotations old

// Set number of compression worker goroutines
logger.SetCompressWorkers(3)
```

## API Request/Response Logging

flexlog provides methods for safely logging API requests and responses with automatic redaction of sensitive data:

```go
// Log an API request
headers := map[string][]string{
    "Authorization": {"Bearer token123"},
    "Content-Type": {"application/json"},
}
body := `{"username": "user", "password": "secret"}`
logger.FlogRequest("POST", "/api/login", headers, body)

// Log an API response
respHeaders := map[string][]string{
    "X-Auth-Token": {"sensitive-token"},
    "Content-Type": {"application/json"},
}
respBody := `{"status": "success", "token": "jwt-token-here"}`
logger.FlogResponse(200, respHeaders, respBody)
```

## Testing

To run the tests, use the following command:

```sh
go test
```

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

# Examples

## MultiLogger Example

```go
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/wayneeseguin/flexlog"
)

func main() {
	// Example 1: Basic file logger with flock backend (default)
	fileLogger, err := flexlog.New("./logs/application.log")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating file logger: %v\n", err)
		os.Exit(1)
	}
	defer fileLogger.CloseAll()

	// Example 2: Logger with syslog backend
	syslogLogger, err := flexlog.NewSyslog("localhost", "myapp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating syslog logger: %v\n", err)
		// Continue with just the file logger
	} else {
		defer syslogLogger.CloseAll()
	}

	// Example 3: Multi-destination logger
	multiLogger, err := flexlog.New("./logs/multi.log")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating multi logger: %v\n", err)
		os.Exit(1)
	}

	// Add a syslog destination
	err = multiLogger.AddDestinationWithBackend("syslog:///dev/log", flexlog.BackendSyslog)
	if err != nil {
		fmt.Printf("Warning: Could not add syslog destination: %v\n", err)
	}

	// Add another file destination
	err = multiLogger.AddDestination("./logs/secondary.log")
	if err != nil {
		fmt.Printf("Warning: Could not add secondary file destination: %v\n", err)
	}

	defer multiLogger.CloseAll()

	// Set configuration
	fileLogger.SetLevel(flexlog.LevelDebug)
	multiLogger.SetLevel(flexlog.LevelInfo)

	// Log some messages to the file logger
	fileLogger.Debug("This is a debug message")
	fileLogger.Info("File logger info message with value: %d", 42)
	fileLogger.Warn("Warning: Something might be wrong")
	fileLogger.Error("Error occurred: %v", fmt.Errorf("sample error"))

	// If syslog logger was created successfully, log to it
	if syslogLogger != nil {
		syslogLogger.Info("Syslog message from application")
		syslogLogger.Error("Error reported to syslog: %v", fmt.Errorf("connection timeout"))
	}

	// Log to multiple destinations
	for i := 0; i < 5; i++ {
		multiLogger.Info("Message %d going to multiple destinations", i)
		time.Sleep(100 * time.Millisecond)
	}

	// Demonstrate non-blocking behavior
	start := time.Now()
	for i := 0; i < 10000; i++ {
		multiLogger.Info("Non-blocking log message %d", i)
	}
	elapsed := time.Since(start)
	fmt.Printf("Logged 10,000 messages in %v (would be much longer if blocking)\n", elapsed)

	// Make sure messages are flushed before exiting
	multiLogger.FlushAll()
}

```
