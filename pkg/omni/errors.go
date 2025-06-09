package omni

import (
	"time"
)

// ErrorLevel represents additional error severity levels beyond the standard log levels
type ErrorLevel int

const (
	// ErrorLevelLow represents minor errors that don't significantly impact operation
	ErrorLevelLow ErrorLevel = iota
	// ErrorLevelWarn represents warning-level errors
	ErrorLevelWarn
	// ErrorLevelMedium represents important errors that may degrade functionality
	ErrorLevelMedium
	// ErrorLevelHigh represents critical errors that significantly impact operation
	ErrorLevelHigh
	// ErrorLevelCritical represents fatal errors that require immediate attention
	ErrorLevelCritical
)

// LogError represents an error that occurred during logging operations
type LogError struct {
	Operation   string                 // The operation that failed
	Destination string                 // The destination where the error occurred
	Message     string                 // Human readable error message
	Err         error                  // The underlying error
	Level       ErrorLevel             // The severity level of the error
	Timestamp   time.Time              // When the error occurred
	Context     map[string]interface{} // Additional context
	Code        int                    // Error code
}

// Error implements the error interface
func (e LogError) Error() string {
	return e.Message
}

// Unwrap returns the underlying error
func (e LogError) Unwrap() error {
	return e.Err
}

// ErrorHandler defines a function type for handling logger errors
type ErrorHandler func(err LogError)

// SilentErrorHandler discards all errors (used in tests)
var SilentErrorHandler ErrorHandler = func(err LogError) {
	// Do nothing
}

// StderrErrorHandler writes errors to stderr
var StderrErrorHandler ErrorHandler = func(err LogError) {
	// Implementation would write to stderr
	// This is a placeholder for the public API
}
