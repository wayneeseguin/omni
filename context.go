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
	ContextKeyRequestID   ContextKey = "request_id"
	ContextKeyTraceID     ContextKey = "trace_id"
	ContextKeySpanID      ContextKey = "span_id"
	ContextKeyUserID      ContextKey = "user_id"
	ContextKeySessionID   ContextKey = "session_id"
	ContextKeyCorrelation ContextKey = "correlation_id"
	ContextKeyTenantID    ContextKey = "tenant_id"
	ContextKeyOperation   ContextKey = "operation"
	ContextKeyService     ContextKey = "service"
	ContextKeyVersion     ContextKey = "version"
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

	// Extract all known context values
	extractContextValues(ctx, fields)

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
		// Context cancelled, but still try to close in background
		// We don't wait for it to complete since the context is already cancelled
		// The CloseAll() method will clean up resources properly
		go func() {
			f.CloseAll()
		}()
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

// extractContextValues extracts known context values into fields map
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

// WithContextFields returns a new context with the given fields
func WithContextFields(ctx context.Context, fields map[string]interface{}) context.Context {
	for key, value := range fields {
		ctx = context.WithValue(ctx, ContextKey(key), value)
	}
	return ctx
}

// WithRequestID returns a new context with the given request ID
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, ContextKeyRequestID, requestID)
}

// WithTraceID returns a new context with the given trace ID
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, ContextKeyTraceID, traceID)
}

// WithUserID returns a new context with the given user ID
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, ContextKeyUserID, userID)
}

// GetRequestID extracts the request ID from context
func GetRequestID(ctx context.Context) (string, bool) {
	if val := ctx.Value(ContextKeyRequestID); val != nil {
		if str, ok := val.(string); ok {
			return str, true
		}
	}
	return "", false
}

// GetTraceID extracts the trace ID from context
func GetTraceID(ctx context.Context) (string, bool) {
	if val := ctx.Value(ContextKeyTraceID); val != nil {
		if str, ok := val.(string); ok {
			return str, true
		}
	}
	return "", false
}

// ContextLogger wraps a FlexLog instance with automatic context extraction
type ContextLogger struct {
	logger *FlexLog
	ctx    context.Context
	fields map[string]interface{}
}

// NewContextLogger creates a new context-aware logger wrapper
func NewContextLogger(logger *FlexLog, ctx context.Context) *ContextLogger {
	return &ContextLogger{
		logger: logger,
		ctx:    ctx,
		fields: make(map[string]interface{}),
	}
}

// WithContext returns a new ContextLogger with the given context
func (cl *ContextLogger) WithContext(ctx context.Context) Logger {
	return &ContextLogger{
		logger: cl.logger,
		ctx:    ctx,
		fields: cl.fields,
	}
}

// WithFields returns a new ContextLogger with additional fields
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

// WithField returns a new ContextLogger with an additional field
func (cl *ContextLogger) WithField(key string, value interface{}) Logger {
	return cl.WithFields(map[string]interface{}{key: value})
}

// WithError returns a new ContextLogger with an error field
func (cl *ContextLogger) WithError(err error) Logger {
	if err == nil {
		return cl
	}
	return cl.WithField("error", err.Error())
}

// Trace logs a trace message with context
func (cl *ContextLogger) Trace(args ...interface{}) {
	msg := fmt.Sprint(args...)
	fields := make(map[string]interface{}, len(cl.fields))
	for k, v := range cl.fields {
		fields[k] = v
	}
	cl.logger.StructuredLogWithContext(cl.ctx, LevelTrace, msg, fields)
}

// Tracef logs a formatted trace message with context
func (cl *ContextLogger) Tracef(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fields := make(map[string]interface{}, len(cl.fields))
	for k, v := range cl.fields {
		fields[k] = v
	}
	cl.logger.StructuredLogWithContext(cl.ctx, LevelTrace, msg, fields)
}

// Debug logs a debug message with context
func (cl *ContextLogger) Debug(args ...interface{}) {
	msg := fmt.Sprint(args...)
	fields := make(map[string]interface{}, len(cl.fields))
	for k, v := range cl.fields {
		fields[k] = v
	}
	cl.logger.StructuredLogWithContext(cl.ctx, LevelDebug, msg, fields)
}

// Debugf logs a formatted debug message with context
func (cl *ContextLogger) Debugf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fields := make(map[string]interface{}, len(cl.fields))
	for k, v := range cl.fields {
		fields[k] = v
	}
	cl.logger.StructuredLogWithContext(cl.ctx, LevelDebug, msg, fields)
}

// Info logs an info message with context
func (cl *ContextLogger) Info(args ...interface{}) {
	msg := fmt.Sprint(args...)
	fields := make(map[string]interface{}, len(cl.fields))
	for k, v := range cl.fields {
		fields[k] = v
	}
	cl.logger.StructuredLogWithContext(cl.ctx, LevelInfo, msg, fields)
}

// Infof logs a formatted info message with context
func (cl *ContextLogger) Infof(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fields := make(map[string]interface{}, len(cl.fields))
	for k, v := range cl.fields {
		fields[k] = v
	}
	cl.logger.StructuredLogWithContext(cl.ctx, LevelInfo, msg, fields)
}

// Warn logs a warning message with context
func (cl *ContextLogger) Warn(args ...interface{}) {
	msg := fmt.Sprint(args...)
	fields := make(map[string]interface{}, len(cl.fields))
	for k, v := range cl.fields {
		fields[k] = v
	}
	cl.logger.StructuredLogWithContext(cl.ctx, LevelWarn, msg, fields)
}

// Warnf logs a formatted warning message with context
func (cl *ContextLogger) Warnf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fields := make(map[string]interface{}, len(cl.fields))
	for k, v := range cl.fields {
		fields[k] = v
	}
	cl.logger.StructuredLogWithContext(cl.ctx, LevelWarn, msg, fields)
}

// Error logs an error message with context
func (cl *ContextLogger) Error(args ...interface{}) {
	msg := fmt.Sprint(args...)
	fields := make(map[string]interface{}, len(cl.fields))
	for k, v := range cl.fields {
		fields[k] = v
	}
	cl.logger.StructuredLogWithContext(cl.ctx, LevelError, msg, fields)
}

// Errorf logs a formatted error message with context
func (cl *ContextLogger) Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fields := make(map[string]interface{}, len(cl.fields))
	for k, v := range cl.fields {
		fields[k] = v
	}
	cl.logger.StructuredLogWithContext(cl.ctx, LevelError, msg, fields)
}

// SetLevel sets the log level
func (cl *ContextLogger) SetLevel(level int) {
	cl.logger.SetLevel(level)
}

// GetLevel returns the current log level
func (cl *ContextLogger) GetLevel() int {
	return cl.logger.GetLevel()
}

// IsLevelEnabled checks if a log level is enabled
func (cl *ContextLogger) IsLevelEnabled(level int) bool {
	return cl.logger.IsLevelEnabled(level)
}

// Structured logs a structured message with context at the given level
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
