package flexlog

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

// LogError represents an error that occurred during logging operations
type LogError struct {
	Time        time.Time
	Level       ErrorLevel
	Source      string // "write", "rotate", "compress", "lock", etc.
	Message     string
	Err         error
	Destination string // Name of the destination where error occurred
}

// ErrorHandler is a function that handles logging errors
type ErrorHandler func(LogError)

// Define error levels using the existing ErrorLevel constants
const (
	ErrorLevelLow      = SeverityLow
	ErrorLevelMedium   = SeverityMedium
	ErrorLevelHigh     = SeverityHigh
	ErrorLevelCritical = SeverityCritical
)

// Error returns the string representation of the LogError
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

// SetErrorHandler sets the error handler for the logger
func (f *FlexLog) SetErrorHandler(handler ErrorHandler) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errorHandler = handler
}

// GetErrorCount returns the total number of errors encountered
func (f *FlexLog) GetErrorCount() uint64 {
	return atomic.LoadUint64(&f.errorCount)
}

// GetErrors returns a read-only channel for receiving errors
// The channel will be closed when the logger is closed
func (f *FlexLog) GetErrors() <-chan LogError {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Create error channel if it doesn't exist
	if f.errorChan == nil {
		f.errorChan = make(chan LogError, 100)
	}

	return f.errorChan
}

// logError handles an error using the configured error handler
func (f *FlexLog) logError(source string, destination string, message string, err error, level ErrorLevel) {
	// Increment error count
	atomic.AddUint64(&f.errorCount, 1)

	// Track errors by source using thread-safe sync.Map
	val, _ := f.errorsBySource.LoadOrStore(source, uint64(0))
	current := val.(uint64)
	for {
		if f.errorsBySource.CompareAndSwap(source, current, current+1) {
			break
		}
		// Reload and retry if another goroutine changed the value
		val, _ := f.errorsBySource.Load(source)
		current = val.(uint64)
	}

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
			// Channel full, use fallback to stderr
			if handler == nil {
				StderrErrorHandler(logErr)
			}
		}
	}
}

// Predefined error handlers

// StderrErrorHandler writes errors to stderr (default behavior)
func StderrErrorHandler(err LogError) {
	fmt.Fprintf(os.Stderr, "%s\n", err.Error())
}

// SilentErrorHandler discards all errors
func SilentErrorHandler(err LogError) {
	// Do nothing
}

// ChannelErrorHandler returns an error handler that sends errors to a channel
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

// MultiErrorHandler combines multiple error handlers
func MultiErrorHandler(handlers ...ErrorHandler) ErrorHandler {
	return func(err LogError) {
		for _, handler := range handlers {
			if handler != nil {
				handler(err)
			}
		}
	}
}

// ThresholdErrorHandler only handles errors above a certain severity
func ThresholdErrorHandler(threshold ErrorLevel, handler ErrorHandler) ErrorHandler {
	return func(err LogError) {
		if err.Level >= threshold {
			handler(err)
		}
	}
}
