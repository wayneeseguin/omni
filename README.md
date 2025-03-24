# Flock Logger ("flocklogger")

A Go library for logging with flock-based synchronization on Unix-like systems.

**Note:** This library uses `unix.Flock` and is intended for use on Unix-like systems (e.g., Linux, macOS).

The primary use case of this is to allow for background logging to a single log from multiple processes without blocking without clobbering each other.

## Installation

To install `flocklogger`, use the following command:

```sh
go get github.com/wayneeseguin/flocklogger
```

## Usage

Here's a simple example of how to use `flocklogger`:

```go
package main

import (
    "github.com/wayneeseguin/flocklogger"
)

func main() {
    logger, err := flocklogger.NewFlockLogger("app.log")
    if err != nil {
        panic(err)
    }
    defer logger.Close()

    logger.Flog("Hello, %s!", "world")
}
```

### Standard Logging Levels

FlockLogger supports standard logging levels (DEBUG, INFO, WARN, ERROR):

```go
logger, err := flocklogger.NewFlockLogger("app.log")
if err != nil {
    panic(err)
}
defer logger.Close()

// Set minimum log level (default is INFO)
logger.SetLevel(flocklogger.LevelDebug) // Options: LevelDebug, LevelInfo, LevelWarn, LevelError

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

### Log Rotation

FlockLogger supports automatic log rotation:

```go
logger, err := flocklogger.NewFlockLogger("app.log")
if err != nil {
    panic(err)
}
defer logger.Close()

// Set maximum log file size (in bytes)
logger.SetMaxSize(10 * 1024 * 1024) // 10MB

// Set maximum number of log files to keep
logger.SetMaxFiles(5)
```

### API Request/Response Logging

FlockLogger provides methods for safely logging API requests and responses with automatic redaction of sensitive data:

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

For more examples, see the [examples](examples) directory.

## Testing

To run the tests, use the following command:

```sh
go test
```

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

