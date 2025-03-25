package flexlog

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

// writeLogEntry writes the log entry to the file
func (f *FlexLog) writeLogEntry(entry LogEntry) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Format timestamp according to options
	entry.Timestamp = f.formatTimestamp(time.Now())

	var entryBytes []byte
	var err error

	// Format the log entry based on the selected format
	if f.format == FormatJSON {
		if indent, _ := f.formatOptions[FormatOptionIndentJSON].(bool); indent {
			entryBytes, err = json.MarshalIndent(entry, "", "  ")
		} else {
			entryBytes, err = json.Marshal(entry)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to marshal log entry to JSON: %v\n", err)
			return
		}
		entryBytes = append(entryBytes, '\n')
	} else {
		// Text format
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("[%s]", entry.Timestamp))

		if includeLevel, _ := f.formatOptions[FormatOptionIncludeLevel].(bool); includeLevel {
			sb.WriteString(fmt.Sprintf(" [%s]", entry.Level))
		}

		sb.WriteString(fmt.Sprintf(" %s", entry.Message))

		sep, _ := f.formatOptions[FormatOptionFieldSeparator].(string)
		if len(entry.Fields) > 0 {
			for k, v := range entry.Fields {
				sb.WriteString(fmt.Sprintf("%s%s=%v", sep, k, v))
			}
		}

		if includeLocation, _ := f.formatOptions[FormatOptionIncludeLocation].(bool); includeLocation && entry.File != "" {
			sb.WriteString(fmt.Sprintf("%sfile=%s:%d", sep, entry.File, entry.Line))
		}

		if entry.StackTrace != "" {
			sb.WriteString(fmt.Sprintf("\nStack Trace:\n%s", entry.StackTrace))
		}

		sb.WriteString("\n")
		entryBytes = []byte(sb.String())
	}

	// Acquire file lock for cross-process safety
	if err := f.acquireLock(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to acquire file lock: %v\n", err)
		return
	}
	defer f.releaseLock()

	// Check if rotation needed
	entrySize := int64(len(entryBytes))
	if f.currentSize+entrySize > f.maxSize {
		if err := f.rotate(); err != nil {
			// If rotation fails, try to write error to file
			now := time.Now().Format("2006-01-02 15:04:05.000")
			fmt.Fprintf(f.writer, "[%s] ERROR: Failed to rotate log file: %v\n", now, err)
			f.writer.Flush()
			return
		}
	}

	// Write entry
	if _, err := f.writer.Write(entryBytes); err != nil {
		// If write fails, try to write error to stderr
		fmt.Fprintf(os.Stderr, "Failed to write to log file: %v\n", err)
		return
	}

	// Update size and flush periodically
	f.currentSize += entrySize
	if f.currentSize%defaultBufferSize == 0 {
		f.writer.Flush()
	}
}

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
		Timestamp: time.Now().Format("2006-01-02 15:04:05.000"),
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
