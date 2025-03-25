package flexlog

import (
	"fmt"
	"runtime"

	"github.com/pkg/errors"
)

// severityToString converts severity level to string
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

// ErrorWithError logs an error with stack trace
func (f *FlexLog) ErrorWithError(message string, err error) {
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

// ErrorWithErrorAndSeverity logs an error with stack trace and severity level
func (f *FlexLog) ErrorWithErrorAndSeverity(message string, err error, severity ErrorLevel) {
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

// WrapError wraps an error with stack trace information
func (f *FlexLog) WrapError(err error, message string) error {
	return errors.Wrap(err, message)
}

// WrapErrorWithSeverity wraps an error with stack trace information and severity
func (f *FlexLog) WrapErrorWithSeverity(err error, message string, severity ErrorLevel) error {
	wrapped := errors.Wrap(err, message)
	// Store severity in context or return a custom error type if needed
	// For simplicity, we'll just log it immediately
	f.ErrorWithErrorAndSeverity(message, wrapped, severity)
	return wrapped
}

// CauseOf returns the root cause of an error
func (f *FlexLog) CauseOf(err error) error {
	return errors.Cause(err)
}

// WithStack attaches a stack trace to an error
func (f *FlexLog) WithStack(err error) error {
	return errors.WithStack(err)
}

// IsErrorType checks if an error is of a specific type (when using errors.Is)
func (f *FlexLog) IsErrorType(err, target error) bool {
	return errors.Is(err, target)
}

// FormatErrorVerbose returns a detailed error representation with stack trace
func (f *FlexLog) FormatErrorVerbose(err error) string {
	if stackTracer, ok := err.(interface{ StackTrace() errors.StackTrace }); ok {
		return fmt.Sprintf("%+v\n%+v", err, stackTracer.StackTrace())
	}
	return fmt.Sprintf("%+v", err)
}

// LogPanic logs the error and stack trace for a recovered panic
func (f *FlexLog) LogPanic(recovered interface{}) {
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

// SafeGo runs a function in a goroutine with panic recovery
func (f *FlexLog) SafeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				f.LogPanic(r)
			}
		}()
		fn()
	}()
}
