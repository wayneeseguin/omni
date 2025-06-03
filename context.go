package omni

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"
)

// ContextKey is a type for context value keys.
// Using a custom type prevents collisions with other packages.
type ContextKey string

// Common context keys for structured logging
const (
	ContextKeyRequestID   ContextKey = "request_id"    // HTTP request ID for tracing
	ContextKeyTraceID     ContextKey = "trace_id"      // Distributed trace ID
	ContextKeySpanID      ContextKey = "span_id"       // Span ID within a trace
	ContextKeyUserID      ContextKey = "user_id"       // User identifier
	ContextKeySessionID   ContextKey = "session_id"    // Session identifier
	ContextKeyCorrelation ContextKey = "correlation_id" // Correlation ID for related events
	ContextKeyTenantID    ContextKey = "tenant_id"     // Multi-tenant identifier
	ContextKeyOperation   ContextKey = "operation"     // Current operation name
	ContextKeyService     ContextKey = "service"       // Service name
	ContextKeyVersion     ContextKey = "version"       // Service version
)

// LogWithContext logs a message with the given level, respecting context cancellation.
// It extracts context values and includes them as structured fields in the log entry.
// The operation will be cancelled if the context is cancelled.
//
// Parameters:
//   - ctx: Context with potential values and cancellation
//   - level: Log level
//   - format: Printf-style format string
//   - args: Arguments for the format string
//
// Returns:
//   - error: Context cancellation error or logger closed error
//
// Example:
//
//	ctx := WithRequestID(context.Background(), "req-123")
//	err := logger.LogWithContext(ctx, omni.LevelInfo, "Processing request")
func (f *Omni) LogWithContext(ctx context.Context, level int, format string, args ...interface{}) error {
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

// DebugWithContext logs a debug message with context.
// Convenience method that calls LogWithContext with LevelDebug.
//
// Parameters:
//   - ctx: Context with potential values and cancellation
//   - format: Printf-style format string
//   - args: Arguments for the format string
//
// Returns:
//   - error: Context cancellation error or logger closed error
func (f *Omni) DebugWithContext(ctx context.Context, format string, args ...interface{}) error {
	return f.LogWithContext(ctx, LevelDebug, format, args...)
}

// InfoWithContext logs an info message with context.
// Convenience method that calls LogWithContext with LevelInfo.
//
// Parameters:
//   - ctx: Context with potential values and cancellation
//   - format: Printf-style format string
//   - args: Arguments for the format string
//
// Returns:
//   - error: Context cancellation error or logger closed error
func (f *Omni) InfoWithContext(ctx context.Context, format string, args ...interface{}) error {
	return f.LogWithContext(ctx, LevelInfo, format, args...)
}

// WarnWithContext logs a warning message with context.
// Convenience method that calls LogWithContext with LevelWarn.
//
// Parameters:
//   - ctx: Context with potential values and cancellation
//   - format: Printf-style format string
//   - args: Arguments for the format string
//
// Returns:
//   - error: Context cancellation error or logger closed error
func (f *Omni) WarnWithContext(ctx context.Context, format string, args ...interface{}) error {
	return f.LogWithContext(ctx, LevelWarn, format, args...)
}

// ErrorWithContext logs an error message with context.
// Convenience method that calls LogWithContext with LevelError.
//
// Parameters:
//   - ctx: Context with potential values and cancellation
//   - format: Printf-style format string
//   - args: Arguments for the format string
//
// Returns:
//   - error: Context cancellation error or logger closed error
func (f *Omni) ErrorWithContext(ctx context.Context, format string, args ...interface{}) error {
	return f.LogWithContext(ctx, LevelError, format, args...)
}

// StructuredLogWithContext logs a structured message with context.
// This method extracts context values and includes them as structured fields,
// handles context cancellation, and supports stack trace capture for error levels.
//
// Parameters:
//   - ctx: Context with potential values and cancellation
//   - level: Log level
//   - message: The log message
//   - fields: Additional structured fields (context values will be merged)
//
// Returns:
//   - error: Context cancellation error, logger closed error, or channel full error
func (f *Omni) StructuredLogWithContext(ctx context.Context, level int, message string, fields map[string]interface{}) error {
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

	// Extract all known context values
	extractContextValues(ctx, fields)

	// Create structured log entry
	entry := &LogEntry{
		Timestamp: time.Now().Format(f.formatOptions.TimestampFormat),
		Level:     levelToString(level),
		Message:   message,
		Fields:    safeFields(fields), // This now includes the context values
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

// CloseWithContext gracefully closes the logger with a timeout.
// It attempts to close all destinations within the context deadline.
// If the context is cancelled before completion, it starts cleanup in the background.
//
// Parameters:
//   - ctx: Context with cancellation/timeout for the close operation
//
// Returns:
//   - error: Close error or context cancellation error
func (f *Omni) CloseWithContext(ctx context.Context) error {
	// Create a channel to signal completion
	done := make(chan error, 1)

	go func() {
		done <- f.CloseAll()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		// Context cancelled, but still try to close in background
		// We don't wait for it to complete since the context is already cancelled
		// The CloseAll() method will clean up resources properly
		go func() {
			f.CloseAll()
		}()
		return ctx.Err()
	}
}

// FlushWithContext flushes all destinations with a timeout.
// It attempts to flush all buffered data within the context deadline.
//
// Parameters:
//   - ctx: Context with cancellation/timeout for the flush operation
//
// Returns:
//   - error: Flush error or context cancellation error
func (f *Omni) FlushWithContext(ctx context.Context) error {
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

// captureStackTrace captures the current stack trace.
// It skips runtime and omni internal frames to show only relevant application code.
//
// Parameters:
//   - maxFrames: Maximum number of stack frames to capture (0 defaults to 32)
//
// Returns:
//   - string: Formatted stack trace with newline-separated frames
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
		lines = lines[7:] // Skip runtime and omni internals
	}

	// Limit to maxFrames
	if len(lines) > maxFrames*2 { // Each frame is 2 lines
		lines = lines[:maxFrames*2]
	}

	return strings.Join(lines, "\n")
}

// parseFileAndLine extracts file and line number from a stack trace.
// It parses the second line of the first frame which contains file:line information.
//
// Parameters:
//   - stackTrace: The stack trace to parse
//
// Returns:
//   - string: The file path
//   - int: The line number (0 if parsing fails)
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

// extractContextValues extracts known context values into fields map.
// It looks for all predefined context keys and adds their values if present.
//
// Parameters:
//   - ctx: Context to extract values from
//   - fields: Map to add extracted values to
func extractContextValues(ctx context.Context, fields map[string]interface{}) {
	// List of known context keys to extract
	contextKeys := []ContextKey{
		ContextKeyRequestID,
		ContextKeyTraceID,
		ContextKeySpanID,
		ContextKeyUserID,
		ContextKeySessionID,
		ContextKeyCorrelation,
		ContextKeyTenantID,
		ContextKeyOperation,
		ContextKeyService,
		ContextKeyVersion,
	}

	// Extract each known key if present
	for _, key := range contextKeys {
		if value := ctx.Value(key); value != nil {
			fields[string(key)] = value
		}
	}
}

// WithContextFields returns a new context with the given fields.
// Each field in the map is added as a context value with a ContextKey type.
//
// Parameters:
//   - ctx: Base context
//   - fields: Map of key-value pairs to add to context
//
// Returns:
//   - context.Context: New context with all fields added
func WithContextFields(ctx context.Context, fields map[string]interface{}) context.Context {
	for key, value := range fields {
		ctx = context.WithValue(ctx, ContextKey(key), value)
	}
	return ctx
}

// WithRequestID returns a new context with the given request ID.
// The request ID can be retrieved using GetRequestID.
//
// Parameters:
//   - ctx: Base context
//   - requestID: The request ID to associate with the context
//
// Returns:
//   - context.Context: New context with request ID
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, ContextKeyRequestID, requestID)
}

// WithTraceID returns a new context with the given trace ID.
// The trace ID can be retrieved using GetTraceID.
//
// Parameters:
//   - ctx: Base context
//   - traceID: The distributed trace ID
//
// Returns:
//   - context.Context: New context with trace ID
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, ContextKeyTraceID, traceID)
}

// WithUserID returns a new context with the given user ID.
// The user ID can be retrieved using context.Value(ContextKeyUserID).
//
// Parameters:
//   - ctx: Base context
//   - userID: The user identifier
//
// Returns:
//   - context.Context: New context with user ID
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, ContextKeyUserID, userID)
}

// GetRequestID extracts the request ID from context.
// Returns false if no request ID is found or if it's not a string.
//
// Parameters:
//   - ctx: Context to extract from
//
// Returns:
//   - string: The request ID if found
//   - bool: true if request ID was found and is a string
func GetRequestID(ctx context.Context) (string, bool) {
	if val := ctx.Value(ContextKeyRequestID); val != nil {
		if str, ok := val.(string); ok {
			return str, true
		}
	}
	return "", false
}

// GetTraceID extracts the trace ID from context.
// Returns false if no trace ID is found or if it's not a string.
//
// Parameters:
//   - ctx: Context to extract from
//
// Returns:
//   - string: The trace ID if found
//   - bool: true if trace ID was found and is a string
func GetTraceID(ctx context.Context) (string, bool) {
	if val := ctx.Value(ContextKeyTraceID); val != nil {
		if str, ok := val.(string); ok {
			return str, true
		}
	}
	return "", false
}

// ContextLogger wraps a Omni instance with automatic context extraction.
// It maintains a context and a set of fields that are automatically included
// in all log messages. This implements the Logger interface for compatibility.
type ContextLogger struct {
	logger *Omni
	ctx    context.Context
	fields map[string]interface{}
}

// NewContextLogger creates a new context-aware logger wrapper.
// The wrapper automatically extracts context values and includes them in logs.
//
// Parameters:
//   - logger: The underlying Omni instance
//   - ctx: The context to associate with this logger
//
// Returns:
//   - *ContextLogger: A new context-aware logger
func NewContextLogger(logger *Omni, ctx context.Context) *ContextLogger {
	return &ContextLogger{
		logger: logger,
		ctx:    ctx,
		fields: make(map[string]interface{}),
	}
}

// WithContext returns a new ContextLogger with the given context.
// The fields from the current logger are preserved.
//
// Parameters:
//   - ctx: The new context to use
//
// Returns:
//   - Logger: A new ContextLogger with updated context
func (cl *ContextLogger) WithContext(ctx context.Context) Logger {
	return &ContextLogger{
		logger: cl.logger,
		ctx:    ctx,
		fields: cl.fields,
	}
}

// WithFields returns a new ContextLogger with additional fields.
// The new fields are merged with existing fields, with new fields taking precedence.
//
// Parameters:
//   - fields: Map of fields to add
//
// Returns:
//   - Logger: A new ContextLogger with merged fields
func (cl *ContextLogger) WithFields(fields map[string]interface{}) Logger {
	newFields := make(map[string]interface{}, len(cl.fields)+len(fields))
	for k, v := range cl.fields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}
	return &ContextLogger{
		logger: cl.logger,
		ctx:    cl.ctx,
		fields: newFields,
	}
}

// WithField returns a new ContextLogger with an additional field.
// This is a convenience method for adding a single field.
//
// Parameters:
//   - key: The field name
//   - value: The field value
//
// Returns:
//   - Logger: A new ContextLogger with the added field
func (cl *ContextLogger) WithField(key string, value interface{}) Logger {
	return cl.WithFields(map[string]interface{}{key: value})
}

// WithError returns a new ContextLogger with an error field.
// If the error is nil, returns the current logger unchanged.
//
// Parameters:
//   - err: The error to add (can be nil)
//
// Returns:
//   - Logger: A new ContextLogger with error field, or self if err is nil
func (cl *ContextLogger) WithError(err error) Logger {
	if err == nil {
		return cl
	}
	return cl.WithField("error", err.Error())
}

// Trace logs a trace message with context.
// The message is created by concatenating args with fmt.Sprint.
func (cl *ContextLogger) Trace(args ...interface{}) {
	msg := fmt.Sprint(args...)
	fields := make(map[string]interface{}, len(cl.fields))
	for k, v := range cl.fields {
		fields[k] = v
	}
	cl.logger.StructuredLogWithContext(cl.ctx, LevelTrace, msg, fields)
}

// Tracef logs a formatted trace message with context.
// The message is formatted using fmt.Sprintf.
func (cl *ContextLogger) Tracef(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fields := make(map[string]interface{}, len(cl.fields))
	for k, v := range cl.fields {
		fields[k] = v
	}
	cl.logger.StructuredLogWithContext(cl.ctx, LevelTrace, msg, fields)
}

// Debug logs a debug message with context.
// The message is created by concatenating args with fmt.Sprint.
func (cl *ContextLogger) Debug(args ...interface{}) {
	msg := fmt.Sprint(args...)
	fields := make(map[string]interface{}, len(cl.fields))
	for k, v := range cl.fields {
		fields[k] = v
	}
	cl.logger.StructuredLogWithContext(cl.ctx, LevelDebug, msg, fields)
}

// Debugf logs a formatted debug message with context.
// The message is formatted using fmt.Sprintf.
func (cl *ContextLogger) Debugf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fields := make(map[string]interface{}, len(cl.fields))
	for k, v := range cl.fields {
		fields[k] = v
	}
	cl.logger.StructuredLogWithContext(cl.ctx, LevelDebug, msg, fields)
}

// Info logs an info message with context.
// The message is created by concatenating args with fmt.Sprint.
func (cl *ContextLogger) Info(args ...interface{}) {
	msg := fmt.Sprint(args...)
	fields := make(map[string]interface{}, len(cl.fields))
	for k, v := range cl.fields {
		fields[k] = v
	}
	cl.logger.StructuredLogWithContext(cl.ctx, LevelInfo, msg, fields)
}

// Infof logs a formatted info message with context.
// The message is formatted using fmt.Sprintf.
func (cl *ContextLogger) Infof(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fields := make(map[string]interface{}, len(cl.fields))
	for k, v := range cl.fields {
		fields[k] = v
	}
	cl.logger.StructuredLogWithContext(cl.ctx, LevelInfo, msg, fields)
}

// Warn logs a warning message with context.
// The message is created by concatenating args with fmt.Sprint.
func (cl *ContextLogger) Warn(args ...interface{}) {
	msg := fmt.Sprint(args...)
	fields := make(map[string]interface{}, len(cl.fields))
	for k, v := range cl.fields {
		fields[k] = v
	}
	cl.logger.StructuredLogWithContext(cl.ctx, LevelWarn, msg, fields)
}

// Warnf logs a formatted warning message with context.
// The message is formatted using fmt.Sprintf.
func (cl *ContextLogger) Warnf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fields := make(map[string]interface{}, len(cl.fields))
	for k, v := range cl.fields {
		fields[k] = v
	}
	cl.logger.StructuredLogWithContext(cl.ctx, LevelWarn, msg, fields)
}

// Error logs an error message with context.
// The message is created by concatenating args with fmt.Sprint.
func (cl *ContextLogger) Error(args ...interface{}) {
	msg := fmt.Sprint(args...)
	fields := make(map[string]interface{}, len(cl.fields))
	for k, v := range cl.fields {
		fields[k] = v
	}
	cl.logger.StructuredLogWithContext(cl.ctx, LevelError, msg, fields)
}

// Errorf logs a formatted error message with context.
// The message is formatted using fmt.Sprintf.
func (cl *ContextLogger) Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fields := make(map[string]interface{}, len(cl.fields))
	for k, v := range cl.fields {
		fields[k] = v
	}
	cl.logger.StructuredLogWithContext(cl.ctx, LevelError, msg, fields)
}

// SetLevel sets the log level.
// This affects the underlying Omni instance.
func (cl *ContextLogger) SetLevel(level int) {
	cl.logger.SetLevel(level)
}

// GetLevel returns the current log level.
// Returns the level from the underlying Omni instance.
func (cl *ContextLogger) GetLevel() int {
	return cl.logger.GetLevel()
}

// IsLevelEnabled checks if a log level is enabled.
// Returns true if messages at the given level will be logged.
func (cl *ContextLogger) IsLevelEnabled(level int) bool {
	return cl.logger.IsLevelEnabled(level)
}

// Structured logs a structured message with context at the given level.
// Fields from the ContextLogger and additional fields are merged before logging.
//
// Parameters:
//   - level: The log level
//   - message: The log message
//   - additionalFields: Extra fields to include (merged with existing fields)
//
// Returns:
//   - error: Any error from the logging operation
func (cl *ContextLogger) Structured(level int, message string, additionalFields map[string]interface{}) error {
	// Merge all fields
	fields := make(map[string]interface{}, len(cl.fields)+len(additionalFields))
	for k, v := range cl.fields {
		fields[k] = v
	}
	for k, v := range additionalFields {
		fields[k] = v
	}
	return cl.logger.StructuredLogWithContext(cl.ctx, level, message, fields)
}
