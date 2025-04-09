package flexlog

import (
	"runtime"
	"time"
)

// StructuredLog logs a message with structured fields
func (f *FlexLog) StructuredLog(level int, message string, fields map[string]interface{}) {
	// Check if we should log this based on filters and sampling
	if !f.shouldLog(level, message, fields) {
		return
	}

	var levelStr string
	switch level {
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

// DebugWithFields logs a debug message with structured fields
func (f *FlexLog) DebugWithFields(message string, fields map[string]interface{}) {
	f.StructuredLog(LevelDebug, message, fields)
}

// InfoWithFields logs an info message with structured fields
func (f *FlexLog) InfoWithFields(message string, fields map[string]interface{}) {
	f.StructuredLog(LevelInfo, message, fields)
}

// WarnWithFields logs a warning message with structured fields
func (f *FlexLog) WarnWithFields(message string, fields map[string]interface{}) {
	f.StructuredLog(LevelWarn, message, fields)
}

// ErrorWithFields logs an error message with structured fields
func (f *FlexLog) ErrorWithFields(message string, fields map[string]interface{}) {
	f.StructuredLog(LevelError, message, fields)
}
