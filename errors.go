package omni

import (
	"fmt"
	"runtime"

	"github.com/pkg/errors"
)

// severityToString converts an ErrorLevel severity to its string representation.
// This is an internal helper function used for formatting error severity in logs.
//
// Parameters:
//   - severity: The error severity level
//
// Returns:
//   - string: The string representation ("low", "medium", "high", "critical", or "unknown")
func severityToString(severity ErrorLevel) string {
	switch severity {
	case SeverityLow:
		return "low"
	case SeverityMedium:
		return "medium"
	case SeverityHigh:
		return "high"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// ErrorWithError logs an error message with the error details and stack trace.
// If the error implements the stack trace interface from pkg/errors, the stack trace
// is automatically included in the log entry.
//
// Parameters:
//   - message: The error message to log
//   - err: The error to include
//
// Example:
//
//	err := db.Connect()
//	if err != nil {
//	    logger.ErrorWithError("Failed to connect to database", err)
//	}
func (f *Omni) ErrorWithError(message string, err error) {
	if f.level > LevelError {
		return
	}

	fields := map[string]interface{}{
		"error": err.Error(),
	}

	// Check if it's a wrapped error from pkg/errors
	if stackTracer, ok := err.(interface{ StackTrace() errors.StackTrace }); ok && f.includeTrace {
		stack := fmt.Sprintf("%+v", stackTracer.StackTrace())
		fields["stack_trace"] = stack
	}

	f.StructuredLog(LevelError, message, fields)
}

// ErrorWithErrorAndSeverity logs an error with stack trace and custom severity level.
// This allows categorizing errors by their impact on the system.
//
// Parameters:
//   - message: The error message to log
//   - err: The error to include
//   - severity: The error severity (SeverityLow, SeverityMedium, SeverityHigh, or SeverityCritical)
//
// Example:
//
//	err := validateInput(data)
//	if err != nil {
//	    logger.ErrorWithErrorAndSeverity("Invalid input data", err, omni.SeverityMedium)
//	}
func (f *Omni) ErrorWithErrorAndSeverity(message string, err error, severity ErrorLevel) {
	if f.level > LevelError {
		return
	}

	fields := map[string]interface{}{
		"error":    err.Error(),
		"severity": severityToString(severity),
	}

	// Check if it's a wrapped error from pkg/errors
	if stackTracer, ok := err.(interface{ StackTrace() errors.StackTrace }); ok && f.includeTrace {
		stack := fmt.Sprintf("%+v", stackTracer.StackTrace())
		fields["stack_trace"] = stack
	}

	f.StructuredLog(LevelError, message, fields)
}

// WrapError wraps an error with additional context and stack trace information.
// This uses pkg/errors to capture the stack trace at the point of wrapping.
//
// Parameters:
//   - err: The original error to wrap
//   - message: Additional context message
//
// Returns:
//   - error: The wrapped error with stack trace
//
// Example:
//
//	err := file.Read()
//	if err != nil {
//	    return logger.WrapError(err, "failed to read configuration file")
//	}
func (f *Omni) WrapError(err error, message string) error {
	return errors.Wrap(err, message)
}

// WrapErrorWithSeverity wraps an error with stack trace and logs it with severity.
// The wrapped error is both returned and immediately logged with the specified severity.
//
// Parameters:
//   - err: The original error to wrap
//   - message: Additional context message
//   - severity: The error severity level
//
// Returns:
//   - error: The wrapped error with stack trace
//
// Example:
//
//	err := criticalOperation()
//	if err != nil {
//	    return logger.WrapErrorWithSeverity(err, "critical operation failed", omni.SeverityCritical)
//	}
func (f *Omni) WrapErrorWithSeverity(err error, message string, severity ErrorLevel) error {
	wrapped := errors.Wrap(err, message)
	// Store severity in context or return a custom error type if needed
	// For simplicity, we'll just log it immediately
	f.ErrorWithErrorAndSeverity(message, wrapped, severity)
	return wrapped
}

// CauseOf returns the root cause of an error by unwrapping all layers.
// This is useful for error comparison and handling specific error types.
//
// Parameters:
//   - err: The error to unwrap
//
// Returns:
//   - error: The root cause error
//
// Example:
//
//	cause := logger.CauseOf(err)
//	if cause == sql.ErrNoRows {
//	    // Handle not found case
//	}
func (f *Omni) CauseOf(err error) error {
	return errors.Cause(err)
}

// WithStack attaches a stack trace to an error at the current call site.
// Use this when you want to add stack trace information to errors from external packages.
//
// Parameters:
//   - err: The error to attach stack trace to
//
// Returns:
//   - error: The error with stack trace attached
//
// Example:
//
//	err := externalLib.DoSomething()
//	if err != nil {
//	    return logger.WithStack(err)
//	}
func (f *Omni) WithStack(err error) error {
	return errors.WithStack(err)
}

// IsErrorType checks if an error is of a specific type using errors.Is.
// This works with wrapped errors and checks the entire error chain.
//
// Parameters:
//   - err: The error to check
//   - target: The target error type to compare against
//
// Returns:
//   - bool: true if err matches target or contains it in its chain
//
// Example:
//
//	if logger.IsErrorType(err, context.Canceled) {
//	    // Handle cancellation
//	}
func (f *Omni) IsErrorType(err, target error) bool {
	return errors.Is(err, target)
}

// FormatErrorVerbose returns a detailed error representation with stack trace.
// This is useful for debugging and detailed error reports.
//
// Parameters:
//   - err: The error to format
//
// Returns:
//   - string: Detailed error message including stack trace if available
//
// Example:
//
//	details := logger.FormatErrorVerbose(err)
//	fmt.Println("Full error details:", details)
func (f *Omni) FormatErrorVerbose(err error) string {
	if stackTracer, ok := err.(interface{ StackTrace() errors.StackTrace }); ok {
		return fmt.Sprintf("%+v\n%+v", err, stackTracer.StackTrace())
	}
	return fmt.Sprintf("%+v", err)
}

// LogPanic logs the error and stack trace for a recovered panic.
// Use this in defer statements to capture and log panic information.
//
// Parameters:
//   - recovered: The value returned by recover()
//
// Example:
//
//	defer func() {
//	    if r := recover(); r != nil {
//	        logger.LogPanic(r)
//	        // Optionally re-panic or handle gracefully
//	    }
//	}()
func (f *Omni) LogPanic(recovered interface{}) {
	// Capture the stack trace
	buf := make([]byte, f.stackSize)
	n := runtime.Stack(buf, false)
	stackTrace := string(buf[:n])

	var errMsg string
	switch v := recovered.(type) {
	case error:
		errMsg = v.Error()
	default:
		errMsg = fmt.Sprintf("%v", v)
	}

	fields := map[string]interface{}{
		"panic":       true,
		"stack_trace": stackTrace,
	}

	f.StructuredLog(LevelError, fmt.Sprintf("Recovered from panic: %s", errMsg), fields)
}

// SafeGo runs a function in a goroutine with automatic panic recovery and logging.
// Any panic that occurs in the function will be caught and logged, preventing
// the entire program from crashing.
//
// Parameters:
//   - fn: The function to run in a safe goroutine
//
// Example:
//
//	logger.SafeGo(func() {
//	    // This code runs in a goroutine with panic protection
//	    riskyOperation()
//	})
func (f *Omni) SafeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				f.LogPanic(r)
			}
		}()
		fn()
	}()
}
