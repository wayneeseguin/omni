package omni

import (
	"fmt"
	"time"
)

// ErrorCode represents specific error types in omni.
// These codes help categorize and handle errors systematically.
type ErrorCode int

const (
	// ErrCodeUnknown represents an unknown or unclassified error
	ErrCodeUnknown ErrorCode = iota

	// File operation errors
	ErrCodeFileOpen      // Failed to open log file
	ErrCodeFileClose     // Failed to close log file
	ErrCodeFileWrite     // Failed to write to log file
	ErrCodeFileFlush     // Failed to flush file buffer
	ErrCodeFileRotate    // Failed to rotate log file
	ErrCodeFileLock      // Failed to acquire file lock
	ErrCodeFileUnlock    // Failed to release file lock
	ErrCodeFileStat      // Failed to stat file

	// Destination errors
	ErrCodeDestinationNotFound // Destination not found
	ErrCodeDestinationDisabled // Destination is disabled
	ErrCodeDestinationNil      // Destination is nil

	// Channel errors
	ErrCodeChannelFull   // Message channel is full
	ErrCodeChannelClosed // Message channel is closed

	// Configuration errors
	ErrCodeInvalidConfig // Invalid configuration
	ErrCodeInvalidLevel  // Invalid log level
	ErrCodeInvalidFormat // Invalid format specified

	// Compression errors
	ErrCodeCompressionFailed    // Compression operation failed
	ErrCodeCompressionQueueFull // Compression queue is full

	// Syslog errors
	ErrCodeSyslogConnection // Failed to connect to syslog
	ErrCodeSyslogWrite      // Failed to write to syslog

	// Shutdown errors
	ErrCodeShutdownTimeout // Shutdown operation timed out
	ErrCodeAlreadyClosed   // Logger already closed
)

// OmniError represents a structured error with context.
// It provides detailed information about what went wrong and where.
type OmniError struct {
	Code        ErrorCode              // The error code indicating the type of error
	Op          string                 // Operation that failed (e.g., "rotate", "write", "compress")
	Path        string                 // File path or destination name
	Err         error                  // Underlying error
	Time        time.Time              // When the error occurred
	Destination string                 // Destination name if applicable
	Context     map[string]interface{} // Additional context information
}

// Error implements the error interface.
// Returns a formatted error message with all available context.
func (e *OmniError) Error() string {
	if e.Destination != "" {
		return fmt.Sprintf("[%s] %s operation failed on %s (destination: %s): %v",
			e.Time.Format("2006-01-02 15:04:05"),
			e.Op,
			e.Path,
			e.Destination,
			e.Err)
	}
	return fmt.Sprintf("[%s] %s operation failed on %s: %v",
		e.Time.Format("2006-01-02 15:04:05"),
		e.Op,
		e.Path,
		e.Err)
}

// Unwrap returns the underlying error.
// Implements the error unwrapping interface for error chain support.
func (e *OmniError) Unwrap() error {
	return e.Err
}

// Is checks if the error matches a target error.
// Implements the errors.Is interface for error comparison.
// Two OmniErrors are considered equal if they have the same ErrorCode.
func (e *OmniError) Is(target error) bool {
	if target == nil {
		return false
	}

	// Check if target is also a OmniError
	if targetErr, ok := target.(*OmniError); ok {
		return e.Code == targetErr.Code
	}

	// Check underlying error
	return e.Err != nil && e.Err == target
}

// NewOmniError creates a new OmniError with the specified details.
//
// Parameters:
//   - code: The error code
//   - op: The operation that failed
//   - path: The file path or resource identifier
//   - err: The underlying error
//
// Returns:
//   - *OmniError: A new error instance
func NewOmniError(code ErrorCode, op, path string, err error) *OmniError {
	return &OmniError{
		Code:    code,
		Op:      op,
		Path:    path,
		Err:     err,
		Time:    time.Now(),
		Context: make(map[string]interface{}),
	}
}

// WithDestination adds destination information to the error.
// This method supports method chaining for building detailed errors.
//
// Parameters:
//   - dest: The destination name
//
// Returns:
//   - *OmniError: The error instance for chaining
func (e *OmniError) WithDestination(dest string) *OmniError {
	e.Destination = dest
	return e
}

// WithContext adds context to the error.
// This method supports method chaining for building detailed errors.
//
// Parameters:
//   - key: The context key
//   - value: The context value
//
// Returns:
//   - *OmniError: The error instance for chaining
//
// Example:
//
//	err := NewOmniError(ErrCodeFileWrite, "write", "/var/log/app.log", ioErr).
//	    WithDestination("primary").
//	    WithContext("bytes_written", 1024).
//	    WithContext("retry_count", 3)
func (e *OmniError) WithContext(key string, value interface{}) *OmniError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// Helper functions for creating common errors

// ErrFileOpen creates a file open error.
// Use this when a log file cannot be opened.
func ErrFileOpen(path string, err error) *OmniError {
	return NewOmniError(ErrCodeFileOpen, "open", path, err)
}

// ErrFileWrite creates a file write error.
// Use this when writing to a log file fails.
func ErrFileWrite(path string, err error) *OmniError {
	return NewOmniError(ErrCodeFileWrite, "write", path, err)
}

// ErrFileFlush creates a file flush error.
// Use this when flushing file buffers fails.
func ErrFileFlush(path string, err error) *OmniError {
	return NewOmniError(ErrCodeFileFlush, "flush", path, err)
}

// ErrFileRotate creates a file rotation error.
// Use this when log rotation fails.
func ErrFileRotate(path string, err error) *OmniError {
	return NewOmniError(ErrCodeFileRotate, "rotate", path, err)
}

// NewChannelFullError creates a channel full error.
// Use this when the message channel buffer is full.
//
// Parameters:
//   - op: The operation that failed due to full channel
func NewChannelFullError(op string) *OmniError {
	return NewOmniError(ErrCodeChannelFull, op, "", fmt.Errorf("message channel full"))
}

// NewDestinationNotFoundError creates a destination not found error.
// Use this when a requested destination doesn't exist.
//
// Parameters:
//   - name: The name of the missing destination
func NewDestinationNotFoundError(name string) *OmniError {
	return NewOmniError(ErrCodeDestinationNotFound, "find", name, fmt.Errorf("destination not found"))
}

// NewShutdownTimeoutError creates a shutdown timeout error.
// Use this when graceful shutdown doesn't complete within the timeout.
//
// Parameters:
//   - duration: The timeout duration that was exceeded
func NewShutdownTimeoutError(duration time.Duration) *OmniError {
	err := NewOmniError(ErrCodeShutdownTimeout, "shutdown", "", fmt.Errorf("shutdown timed out after %v", duration))
	err.WithContext("timeout", duration)
	return err
}

// IsRetryable returns true if the error is retryable.
// Certain errors like temporary network issues or file locks can be retried.
//
// Parameters:
//   - err: The error to check
//
// Returns:
//   - bool: true if the error can be retried
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's a OmniError
	if omniErr, ok := err.(*OmniError); ok {
		switch omniErr.Code {
		case ErrCodeChannelFull,
			ErrCodeCompressionQueueFull,
			ErrCodeFileLock:
			return true
		}
	}

	// Check for specific error strings that indicate retryable conditions
	errStr := err.Error()
	return contains(errStr, "resource temporarily unavailable") ||
		contains(errStr, "too many open files") ||
		contains(errStr, "no space left on device")
}

// FileError represents file-specific errors for backwards compatibility
type FileError struct {
	Op   string // Operation that failed
	Path string // File path
	Err  error  // Underlying error
}

func (e *FileError) Error() string {
	return fmt.Sprintf("file %s error on %s: %v", e.Op, e.Path, e.Err)
}

func (e *FileError) Unwrap() error {
	return e.Err
}

// DestinationError represents destination-specific errors
type DestinationError struct {
	Op   string // Operation that failed
	Name string // Destination name
	Err  error  // Underlying error
}

func (e *DestinationError) Error() string {
	return fmt.Sprintf("destination %s: %s failed: %v", e.Name, e.Op, e.Err)
}

func (e *DestinationError) Unwrap() error {
	return e.Err
}

// ConfigError represents configuration-specific errors
type ConfigError struct {
	Field string      // Configuration field that failed
	Value interface{} // Invalid value
	Err   error       // Underlying error
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("config error: field %s with value %v: %v", e.Field, e.Value, e.Err)
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	// Empty substring is contained in any string
	if len(substr) == 0 {
		return true
	}
	// If substring is longer than s, it can't be contained
	if len(s) < len(substr) {
		return false
	}
	// Check if s equals substr or if substr is contained in s
	return s == substr || containsHelper(s, substr)
}

// containsHelper is a helper function for case-insensitive contains
func containsHelper(s, substr string) bool {
	// Simple implementation - in production, use strings.Contains with strings.ToLower
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] && s[i+j] != substr[j]+'A'-'a' && s[i+j] != substr[j]-'A'+'a' {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
