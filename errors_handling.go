package flexlog

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

// LogError represents an error that occurred during logging operations.
// It provides detailed context about when and where the error occurred.
type LogError struct {
	Time        time.Time  // When the error occurred
	Level       ErrorLevel // Severity level of the error
	Source      string     // Operation that failed ("write", "rotate", "compress", "lock", etc.)
	Message     string     // Human-readable error message
	Err         error      // The underlying error
	Destination string     // Name of the destination where error occurred
}

// ErrorHandler is a function that handles logging errors.
// Implement custom error handlers to control how logging errors are reported.
//
// Example:
//
//	emailHandler := func(err LogError) {
//	    if err.Level >= flexlog.ErrorLevelHigh {
//	        sendEmail("admin@example.com", "Logging Error", err.Error())
//	    }
//	}
type ErrorHandler func(LogError)

// Define error levels using the existing ErrorLevel constants
const (
	ErrorLevelLow      = SeverityLow
	ErrorLevelMedium   = SeverityMedium
	ErrorLevelHigh     = SeverityHigh
	ErrorLevelCritical = SeverityCritical
)

// Error returns the string representation of the LogError.
// Implements the error interface, allowing LogError to be used as a standard error.
//
// Returns:
//   - string: Formatted error message with timestamp, source, and details
func (le LogError) Error() string {
	if le.Destination != "" {
		return fmt.Sprintf("[%s] %s error in %s: %s - %v",
			le.Time.Format("2006-01-02 15:04:05"),
			le.Source, le.Destination, le.Message, le.Err)
	}
	return fmt.Sprintf("[%s] %s error: %s - %v",
		le.Time.Format("2006-01-02 15:04:05"),
		le.Source, le.Message, le.Err)
}

// SetErrorHandler sets the error handler for the logger.
// The error handler is called whenever an error occurs during logging operations.
//
// Parameters:
//   - handler: The error handler function to use
//
// Example:
//
//	logger.SetErrorHandler(flexlog.StderrErrorHandler)  // Log errors to stderr
//	logger.SetErrorHandler(flexlog.SilentErrorHandler)  // Suppress error output
//	logger.SetErrorHandler(customHandler)               // Use custom handler
func (f *FlexLog) SetErrorHandler(handler ErrorHandler) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errorHandler = handler
}

// GetErrorCount returns the total number of errors encountered.
// This is useful for monitoring and alerting on logging system health.
//
// Returns:
//   - uint64: Total number of errors since logger creation
func (f *FlexLog) GetErrorCount() uint64 {
	return atomic.LoadUint64(&f.errorCount)
}

// GetErrors returns a read-only channel for receiving errors.
// The channel will be closed when the logger is closed.
// This allows external monitoring of logging errors as they occur.
//
// Returns:
//   - <-chan LogError: Read-only channel for error notifications (buffer size: 100)
//
// Example:
//
//	go func() {
//	    for err := range logger.GetErrors() {
//	        fmt.Printf("Logging error: %v\n", err)
//	    }
//	}()
func (f *FlexLog) GetErrors() <-chan LogError {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Create error channel if it doesn't exist
	if f.errorChan == nil {
		f.errorChan = make(chan LogError, 100)
	}

	return f.errorChan
}

// logError is an internal method that handles errors using the configured error handler.
// It tracks error metrics and sends errors to the error channel if available.
//
// Parameters:
//   - source: The operation that failed (e.g., "write", "rotate", "compress")
//   - destination: The destination name where the error occurred
//   - message: Human-readable error message
//   - err: The underlying error (can be nil)
//   - level: The error severity level
func (f *FlexLog) logError(source string, destination string, message string, err error, level ErrorLevel) {
	// Always increment error count
	atomic.AddUint64(&f.errorCount, 1)

	// Track errors by source using thread-safe sync.Map with atomic counters
	val, _ := f.errorsBySource.LoadOrStore(source, &atomic.Uint64{})
	counter := val.(*atomic.Uint64)
	counter.Add(1)

	logErr := LogError{
		Time:        time.Now(),
		Level:       level,
		Source:      source,
		Message:     message,
		Err:         err,
		Destination: destination,
	}

	// Store last error
	f.mu.Lock()
	f.lastError = &logErr
	now := time.Now()
	f.lastErrorTime = &now
	f.mu.Unlock()

	// Get the error handler
	f.mu.Lock()
	handler := f.errorHandler
	errorChan := f.errorChan
	f.mu.Unlock()

	// Call the error handler if set
	if handler != nil {
		handler(logErr)
	}

	// Send to error channel if available (non-blocking)
	if errorChan != nil {
		select {
		case errorChan <- logErr:
		default:
			// Channel full, use fallback to stderr (but only in non-test mode)
			if handler == nil && !isTestMode() {
				StderrErrorHandler(logErr)
			}
		}
	}
}

// Predefined error handlers

// StderrErrorHandler writes errors to stderr (default behavior).
// This is the default error handler for production environments.
func StderrErrorHandler(err LogError) {
	fmt.Fprintf(os.Stderr, "%s\n", err.Error())
}

// SilentErrorHandler discards all errors without any output.
// Useful for testing or when error logging needs to be completely suppressed.
func SilentErrorHandler(err LogError) {
	// Do nothing
}

// ChannelErrorHandler returns an error handler that sends errors to a channel.
// If the channel is full, errors are written to stderr as a fallback.
//
// Parameters:
//   - ch: The channel to send errors to
//
// Returns:
//   - ErrorHandler: An error handler function
//
// Example:
//
//	errChan := make(chan flexlog.LogError, 100)
//	logger.SetErrorHandler(flexlog.ChannelErrorHandler(errChan))
func ChannelErrorHandler(ch chan<- LogError) ErrorHandler {
	return func(err LogError) {
		select {
		case ch <- err:
		default:
			// Channel full, fallback to stderr
			StderrErrorHandler(err)
		}
	}
}

// MultiErrorHandler combines multiple error handlers.
// All provided handlers will be called for each error.
//
// Parameters:
//   - handlers: Variable number of error handlers to combine
//
// Returns:
//   - ErrorHandler: A combined error handler
//
// Example:
//
//	handler := flexlog.MultiErrorHandler(
//	    flexlog.StderrErrorHandler,
//	    flexlog.ChannelErrorHandler(errChan),
//	    customAlertHandler,
//	)
//	logger.SetErrorHandler(handler)
func MultiErrorHandler(handlers ...ErrorHandler) ErrorHandler {
	return func(err LogError) {
		for _, handler := range handlers {
			if handler != nil {
				handler(err)
			}
		}
	}
}

// ThresholdErrorHandler only handles errors at or above a certain severity level.
// Errors below the threshold are ignored.
//
// Parameters:
//   - threshold: Minimum severity level to handle
//   - handler: The handler to call for errors meeting the threshold
//
// Returns:
//   - ErrorHandler: A filtered error handler
//
// Example:
//
//	// Only handle high and critical errors
//	handler := flexlog.ThresholdErrorHandler(
//	    flexlog.ErrorLevelHigh,
//	    flexlog.StderrErrorHandler,
//	)
//	logger.SetErrorHandler(handler)
func ThresholdErrorHandler(threshold ErrorLevel, handler ErrorHandler) ErrorHandler {
	return func(err LogError) {
		if err.Level >= threshold {
			handler(err)
		}
	}
}
