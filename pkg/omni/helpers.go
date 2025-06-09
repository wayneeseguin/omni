package omni

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Constants are defined in const.go

// DefaultFormatOptions returns default formatting options
func DefaultFormatOptions() FormatOptions {
	return FormatOptions{
		TimestampFormat: time.RFC3339,
		IncludeLevel:    true,
		IncludeTime:     true,
		LevelFormat:     LevelFormatName,
		IndentJSON:      false,
		FieldSeparator:  " ",
		TimeZone:        time.UTC,
	}
}

// DefaultSampleKeyFunc is the default function for generating sampling keys
func DefaultSampleKeyFunc(level int, message string, fields map[string]interface{}) string {
	return fmt.Sprintf("%d:%s", level, message)
}

// isTestMode detects if we're running under go test
func isTestMode() bool {
	// Check command line arguments for test-related flags first
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.") {
			return true
		}
	}

	// Check if we're running under go test via executable name
	if exe, err := os.Executable(); err == nil {
		if strings.HasSuffix(exe, ".test") {
			return true
		}
		basename := filepath.Base(exe)
		if basename == "go" || strings.Contains(basename, ".test") {
			return true
		}
	}

	return false
}

// getDefaultErrorHandler returns the appropriate error handler based on environment
func getDefaultErrorHandler() ErrorHandler {
	if isTestMode() {
		return SilentErrorHandler
	}
	return StderrErrorHandler
}

// getDefaultChannelSize retrieves the default channel size from an environment variable or uses the default value
func getDefaultChannelSize() int {
	if value, exists := os.LookupEnv("OMNI_CHANNEL_SIZE"); exists {
		if size, err := strconv.Atoi(value); err == nil && size > 0 && size <= 2147483647 {
			return size
		}
	}
	return 100 // Default to 100 if not specified in environment
}

// GetHostname gets the hostname for metadata
func GetHostname() (string, error) {
	return os.Hostname()
}

// NewBatchWriter creates a new batch writer (placeholder for now)
func NewBatchWriter(writer interface{}, maxSize, maxCount int, flushInterval time.Duration) interface{} {
	// For now, return the writer directly until we can properly integrate
	// This maintains backward compatibility
	return writer
}

// NewSyslog creates a new logger with syslog backend.
//
// Parameters:
//   - address: The syslog server address (e.g., "localhost:514", "/dev/log")
//   - tag: The syslog tag to use for messages
//
// Returns:
//   - *Omni: The logger instance configured for syslog
//   - error: Any error encountered during creation
func NewSyslog(address, tag string) (*Omni, error) {
	// Create syslog URI
	uri := address
	if !strings.HasPrefix(uri, "syslog://") {
		if strings.HasPrefix(address, "/") {
			// Unix socket path
			uri = "syslog://" + address
		} else {
			// Network address
			uri = "syslog://" + address
		}
	}

	return NewWithBackend(uri, BackendSyslog)
}

// Error codes
const (
	ErrCodeInvalidConfig = 1001
	ErrCodeInvalidLevel  = 1002
	ErrCodeInvalidFormat = 1003
)
