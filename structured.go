package flexlog

import (
	"runtime"
	"time"
)

// StructuredLog logs a message with structured fields at the specified level.
// This is the core method for structured logging that other level-specific methods delegate to.
//
// Parameters:
//   - level: The log level (LevelTrace, LevelDebug, LevelInfo, LevelWarn, or LevelError)
//   - message: The log message
//   - fields: Key-value pairs to include with the log entry
//
// The method will:
//   - Apply filtering and sampling rules
//   - Add timestamp and level information
//   - Include file/line information if tracing is enabled
//   - Capture stack traces for errors when configured
//
// Example:
//
//	logger.StructuredLog(flexlog.LevelInfo, "User action", map[string]interface{}{
//		"user_id": 123,
//		"action": "login",
//		"ip": "192.168.1.1",
//	})
func (f *FlexLog) StructuredLog(level int, message string, fields map[string]interface{}) {
	// Check if we should log this based on filters and sampling
	if !f.shouldLog(level, message, fields) {
		return
	}

	var levelStr string
	switch level {
	case LevelTrace:
		levelStr = "TRACE"
	case LevelDebug:
		levelStr = "DEBUG"
	case LevelInfo:
		levelStr = "INFO"
	case LevelWarn:
		levelStr = "WARN"
	case LevelError:
		levelStr = "ERROR"
	default:
		levelStr = "LOG"
	}

	entry := LogEntry{
		Timestamp: f.formatTimestamp(time.Now()),
		Level:     levelStr,
		Message:   message,
		Fields:    fields,
	}

	// Add file and line information
	if f.includeTrace || level == LevelError {
		_, file, line, ok := runtime.Caller(2)
		if ok {
			entry.File = file
			entry.Line = line
		}

		if (level == LevelError && f.includeTrace) || f.captureAll {
			// Capture stack trace for errors or when capturing all is enabled
			buf := make([]byte, f.stackSize)
			n := runtime.Stack(buf, false)
			entry.StackTrace = string(buf[:n])
		}
	}

	f.writeLogEntry(entry)
}

// TraceWithFields logs a trace message with structured fields.
// This method is useful for very detailed diagnostic information with context.
//
// Parameters:
//   - message: The trace message
//   - fields: Key-value pairs providing additional context
//
// Example:
//
//	logger.TraceWithFields("Function entry", map[string]interface{}{
//		"function": "processOrder",
//		"params": params,
//		"caller": "api/handler.go:42",
//	})
func (f *FlexLog) TraceWithFields(message string, fields map[string]interface{}) {
	f.StructuredLog(LevelTrace, message, fields)
}

// DebugWithFields logs a debug message with structured fields.
// This method is useful for adding contextual information to debug logs.
//
// Parameters:
//   - message: The debug message
//   - fields: Key-value pairs providing additional context
//
// Example:
//
//	logger.DebugWithFields("Cache lookup", map[string]interface{}{
//		"key": cacheKey,
//		"hit": true,
//		"latency_ms": 0.5,
//	})
func (f *FlexLog) DebugWithFields(message string, fields map[string]interface{}) {
	f.StructuredLog(LevelDebug, message, fields)
}

// InfoWithFields logs an info message with structured fields.
// This method is ideal for logging application events with associated metadata.
//
// Parameters:
//   - message: The informational message
//   - fields: Key-value pairs providing additional context
//
// Example:
//
//	logger.InfoWithFields("Payment processed", map[string]interface{}{
//		"payment_id": "pay_123456",
//		"amount": 99.99,
//		"currency": "USD",
//		"method": "credit_card",
//	})
func (f *FlexLog) InfoWithFields(message string, fields map[string]interface{}) {
	f.StructuredLog(LevelInfo, message, fields)
}

// WarnWithFields logs a warning message with structured fields.
// Use this for potentially problematic situations that deserve attention.
//
// Parameters:
//   - message: The warning message
//   - fields: Key-value pairs providing additional context
//
// Example:
//
//	logger.WarnWithFields("High memory usage detected", map[string]interface{}{
//		"memory_percent": 85.5,
//		"threshold": 80.0,
//		"process": "worker-1",
//	})
func (f *FlexLog) WarnWithFields(message string, fields map[string]interface{}) {
	f.StructuredLog(LevelWarn, message, fields)
}

// ErrorWithFields logs an error message with structured fields.
// Use this for errors that need immediate attention, with contextual information.
// If stack trace capture is enabled, a stack trace will be included.
//
// Parameters:
//   - message: The error message
//   - fields: Key-value pairs providing error context
//
// Example:
//
//	logger.ErrorWithFields("Database connection failed", map[string]interface{}{
//		"host": "db.example.com",
//		"port": 5432,
//		"error": err.Error(),
//		"retry_count": 3,
//	})
func (f *FlexLog) ErrorWithFields(message string, fields map[string]interface{}) {
	f.StructuredLog(LevelError, message, fields)
}
