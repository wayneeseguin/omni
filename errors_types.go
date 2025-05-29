package flexlog

import (
	"fmt"
	"time"
)

// ErrorCode represents specific error types in flexlog
type ErrorCode int

const (
	// ErrCodeUnknown represents an unknown error
	ErrCodeUnknown ErrorCode = iota

	// File operation errors
	ErrCodeFileOpen
	ErrCodeFileClose
	ErrCodeFileWrite
	ErrCodeFileFlush
	ErrCodeFileRotate
	ErrCodeFileLock
	ErrCodeFileUnlock
	ErrCodeFileStat

	// Destination errors
	ErrCodeDestinationNotFound
	ErrCodeDestinationDisabled
	ErrCodeDestinationNil

	// Channel errors
	ErrCodeChannelFull
	ErrCodeChannelClosed

	// Configuration errors
	ErrCodeInvalidConfig
	ErrCodeInvalidLevel
	ErrCodeInvalidFormat

	// Compression errors
	ErrCodeCompressionFailed
	ErrCodeCompressionQueueFull

	// Syslog errors
	ErrCodeSyslogConnection
	ErrCodeSyslogWrite

	// Shutdown errors
	ErrCodeShutdownTimeout
	ErrCodeAlreadyClosed
)

// FlexLogError represents a structured error with context
type FlexLogError struct {
	Code        ErrorCode
	Op          string                 // Operation that failed (e.g., "rotate", "write", "compress")
	Path        string                 // File path or destination name
	Err         error                  // Underlying error
	Time        time.Time              // When the error occurred
	Destination string                 // Destination name if applicable
	Context     map[string]interface{} // Additional context
}

// Error implements the error interface
func (e *FlexLogError) Error() string {
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

// Unwrap returns the underlying error
func (e *FlexLogError) Unwrap() error {
	return e.Err
}

// Is checks if the error matches a target error
func (e *FlexLogError) Is(target error) bool {
	if target == nil {
		return false
	}

	// Check if target is also a FlexLogError
	if targetErr, ok := target.(*FlexLogError); ok {
		return e.Code == targetErr.Code
	}

	// Check underlying error
	return e.Err != nil && e.Err == target
}

// NewFlexLogError creates a new FlexLogError
func NewFlexLogError(code ErrorCode, op, path string, err error) *FlexLogError {
	return &FlexLogError{
		Code:    code,
		Op:      op,
		Path:    path,
		Err:     err,
		Time:    time.Now(),
		Context: make(map[string]interface{}),
	}
}

// WithDestination adds destination information to the error
func (e *FlexLogError) WithDestination(dest string) *FlexLogError {
	e.Destination = dest
	return e
}

// WithContext adds context to the error
func (e *FlexLogError) WithContext(key string, value interface{}) *FlexLogError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// Helper functions for creating common errors

// ErrFileOpen creates a file open error
func ErrFileOpen(path string, err error) *FlexLogError {
	return NewFlexLogError(ErrCodeFileOpen, "open", path, err)
}

// ErrFileWrite creates a file write error
func ErrFileWrite(path string, err error) *FlexLogError {
	return NewFlexLogError(ErrCodeFileWrite, "write", path, err)
}

// ErrFileFlush creates a file flush error
func ErrFileFlush(path string, err error) *FlexLogError {
	return NewFlexLogError(ErrCodeFileFlush, "flush", path, err)
}

// ErrFileRotate creates a file rotation error
func ErrFileRotate(path string, err error) *FlexLogError {
	return NewFlexLogError(ErrCodeFileRotate, "rotate", path, err)
}

// NewChannelFullError creates a channel full error
func NewChannelFullError(op string) *FlexLogError {
	return NewFlexLogError(ErrCodeChannelFull, op, "", fmt.Errorf("message channel full"))
}

// NewDestinationNotFoundError creates a destination not found error
func NewDestinationNotFoundError(name string) *FlexLogError {
	return NewFlexLogError(ErrCodeDestinationNotFound, "find", name, fmt.Errorf("destination not found"))
}

// NewShutdownTimeoutError creates a shutdown timeout error
func NewShutdownTimeoutError(duration time.Duration) *FlexLogError {
	err := NewFlexLogError(ErrCodeShutdownTimeout, "shutdown", "", fmt.Errorf("shutdown timed out after %v", duration))
	err.WithContext("timeout", duration)
	return err
}

// IsRetryable returns true if the error is retryable
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's a FlexLogError
	if flexErr, ok := err.(*FlexLogError); ok {
		switch flexErr.Code {
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
	return len(s) >= len(substr) &&
		(s == substr || len(s) > 0 && len(substr) > 0 &&
			containsHelper(s, substr))
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
