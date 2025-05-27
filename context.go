package flexlog

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"
)

// ContextKey is a type for context value keys
type ContextKey string

// Common context keys
const (
	ContextKeyRequestID ContextKey = "request_id"
	ContextKeyTraceID   ContextKey = "trace_id"
)

// LogWithContext logs a message with the given level, respecting context cancellation
func (f *FlexLog) LogWithContext(ctx context.Context, level int, format string, args ...interface{}) error {
	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Check if logger is closed
	if f.IsClosed() {
		return fmt.Errorf("logger is closed")
	}

	// Check if we should log this based on level
	if level < f.GetLevel() {
		return nil
	}

	// Format the message
	message := fmt.Sprintf(format, args...)

	// Check if we should log this based on sampling
	if !f.shouldLog(level, message, nil) {
		return nil
	}

	// Create log message
	msg := LogMessage{
		Level:     level,
		Format:    format,
		Args:      args,
		Timestamp: time.Now(),
	}

	// Try to send to channel with context
	select {
	case f.msgChan <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Channel is full, try with a short timeout
		timer := time.NewTimer(10 * time.Millisecond)
		defer timer.Stop()
		
		select {
		case f.msgChan <- msg:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return fmt.Errorf("message channel full")
		}
	}
}

// DebugWithContext logs a debug message with context
func (f *FlexLog) DebugWithContext(ctx context.Context, format string, args ...interface{}) error {
	return f.LogWithContext(ctx, LevelDebug, format, args...)
}

// InfoWithContext logs an info message with context
func (f *FlexLog) InfoWithContext(ctx context.Context, format string, args ...interface{}) error {
	return f.LogWithContext(ctx, LevelInfo, format, args...)
}

// WarnWithContext logs a warning message with context
func (f *FlexLog) WarnWithContext(ctx context.Context, format string, args ...interface{}) error {
	return f.LogWithContext(ctx, LevelWarn, format, args...)
}

// ErrorWithContext logs an error message with context
func (f *FlexLog) ErrorWithContext(ctx context.Context, format string, args ...interface{}) error {
	return f.LogWithContext(ctx, LevelError, format, args...)
}

// StructuredLogWithContext logs a structured message with context
func (f *FlexLog) StructuredLogWithContext(ctx context.Context, level int, message string, fields map[string]interface{}) error {
	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Check if logger is closed
	if f.IsClosed() {
		return fmt.Errorf("logger is closed")
	}

	// Check if we should log this based on filters and sampling
	if !f.shouldLog(level, message, fields) {
		return nil
	}

	// Extract context values if available
	if fields == nil {
		fields = make(map[string]interface{})
	}

	// Add request ID from context if available
	if requestID := ctx.Value(ContextKeyRequestID); requestID != nil {
		fields["request_id"] = requestID
	}

	// Add trace ID from context if available
	if traceID := ctx.Value(ContextKeyTraceID); traceID != nil {
		fields["trace_id"] = traceID
	}

	// Create structured log entry
	entry := &LogEntry{
		Timestamp: time.Now().Format(f.formatOptions.TimestampFormat),
		Level:     levelToString(level),
		Message:   message,
		Fields:    fields, // This now includes the context values
	}

	// Include stack trace if enabled for this level
	if f.includeTrace && (level >= LevelError || f.captureAll) {
		entry.StackTrace = captureStackTrace(f.stackSize)
		// Parse file and line from stack trace
		file, line := parseFileAndLine(entry.StackTrace)
		entry.File = file
		entry.Line = line
	}

	// Create log message
	msg := LogMessage{
		Level:     level,
		Entry:     entry,
		Timestamp: time.Now(),
	}

	// Try to send to channel with context
	select {
	case f.msgChan <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Channel is full, try with a short timeout
		timer := time.NewTimer(10 * time.Millisecond)
		defer timer.Stop()
		
		select {
		case f.msgChan <- msg:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return fmt.Errorf("message channel full")
		}
	}
}

// CloseWithContext gracefully closes the logger with a timeout
func (f *FlexLog) CloseWithContext(ctx context.Context) error {
	// Create a channel to signal completion
	done := make(chan error, 1)

	go func() {
		done <- f.CloseAll()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		// Context cancelled, but still try to close
		go f.CloseAll()
		return ctx.Err()
	}
}

// FlushWithContext flushes all destinations with a timeout
func (f *FlexLog) FlushWithContext(ctx context.Context) error {
	// Create a channel to signal completion
	done := make(chan error, 1)

	go func() {
		done <- f.FlushAll()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// levelToString converts a log level to its string representation
func levelToString(level int) string {
	switch level {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "LOG"
	}
}

// captureStackTrace captures the current stack trace
func captureStackTrace(maxFrames int) string {
	if maxFrames <= 0 {
		maxFrames = 32
	}

	// Capture stack trace
	buf := make([]byte, 1024*16)
	n := runtime.Stack(buf, false)
	stack := string(buf[:n])

	// Skip frames from this function and its callers
	lines := strings.Split(stack, "\n")
	if len(lines) > 7 {
		lines = lines[7:] // Skip runtime and flexlog internals
	}

	// Limit to maxFrames
	if len(lines) > maxFrames*2 { // Each frame is 2 lines
		lines = lines[:maxFrames*2]
	}

	return strings.Join(lines, "\n")
}

// parseFileAndLine extracts file and line number from a stack trace
func parseFileAndLine(stackTrace string) (string, int) {
	lines := strings.Split(stackTrace, "\n")
	if len(lines) >= 2 {
		// The file and line are typically in the second line of each frame
		fileLine := strings.TrimSpace(lines[1])
		parts := strings.Split(fileLine, ":")
		if len(parts) >= 2 {
			file := parts[0]
			var line int
			fmt.Sscanf(parts[1], "%d", &line)
			return file, line
		}
	}
	return "", 0
}