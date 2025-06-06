package utils

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"
	
	"github.com/wayneeseguin/omni/pkg/omni"
)

// ContextKey is a type for context value keys.
// Using a custom type prevents collisions with other packages.
type ContextKey string

// Common context keys for structured logging
const (
	ContextKeyRequestID   ContextKey = "request_id"    // HTTP request ID for tracing
	ContextKeyTraceID     ContextKey = "trace_id"      // Distributed trace ID
	ContextKeyUserID      ContextKey = "user_id"       // User ID for audit trails
	ContextKeySessionID   ContextKey = "session_id"    // Session ID for user sessions
	ContextKeyCorrelation ContextKey = "correlation_id" // Correlation ID for event tracking
	ContextKeyComponent   ContextKey = "component"      // Component/service name
	ContextKeyOperation   ContextKey = "operation"      // Operation being performed
	ContextKeySourceIP    ContextKey = "source_ip"      // Source IP address
	ContextKeyUserAgent   ContextKey = "user_agent"     // HTTP User-Agent
	ContextKeyMethod      ContextKey = "method"         // HTTP method or RPC method
	ContextKeyPath        ContextKey = "path"           // Request path or endpoint
	ContextKeyDuration    ContextKey = "duration"       // Operation duration
	ContextKeyStatus      ContextKey = "status"         // Response status
	ContextKeyError       ContextKey = "error"          // Error information
	ContextKeySpanID      ContextKey = "span_id"        // Distributed tracing span ID
	ContextKeyParentSpan  ContextKey = "parent_span"    // Parent span ID
	ContextKeyTraceFlags  ContextKey = "trace_flags"    // Trace flags (sampled, debug, etc.)
	ContextKeyEnvironment ContextKey = "environment"    // Environment (dev, staging, prod)
	ContextKeyVersion     ContextKey = "version"        // Application/API version
	ContextKeyBuildID     ContextKey = "build_id"       // Build/commit ID
	ContextKeyTimestamp   ContextKey = "timestamp"      // Event timestamp
	ContextKeyLogLevel    ContextKey = "log_level"      // Override log level for this context
)

// NOTE: Context-aware logging methods have been moved to pkg/omni

// ExtractContextFields extracts common fields from a context.
// This is useful for automatically including context values in log entries.
//
// Parameters:
//   - ctx: The context to extract fields from
//   - keys: The context keys to extract (if empty, extracts all common keys)
//
// Returns:
//   - map[string]interface{}: A map of extracted fields
//
// Example:
//
//	fields := ExtractContextFields(ctx, ContextKeyRequestID, ContextKeyUserID)
//	logger.WithFields(fields).Info("Processing request")
func ExtractContextFields(ctx context.Context, keys ...ContextKey) map[string]interface{} {
	fields := make(map[string]interface{})
	
	// If no specific keys provided, extract common ones
	if len(keys) == 0 {
		keys = []ContextKey{
			ContextKeyRequestID,
			ContextKeyTraceID,
			ContextKeyUserID,
			ContextKeySessionID,
			ContextKeyCorrelation,
			ContextKeyComponent,
			ContextKeyOperation,
			ContextKeySourceIP,
			ContextKeyMethod,
			ContextKeyPath,
			ContextKeyEnvironment,
			ContextKeyVersion,
		}
	}
	
	// Extract requested fields
	for _, key := range keys {
		if value := ctx.Value(key); value != nil {
			fields[string(key)] = value
		}
	}
	
	return fields
}

// MergeContextFields merges fields from a context with additional fields.
// Context fields take precedence over additional fields with the same key.
//
// Parameters:
//   - ctx: The context to extract fields from
//   - additionalFields: Additional fields to merge
//   - keys: The context keys to extract (if empty, extracts all common keys)
//
// Returns:
//   - map[string]interface{}: Merged fields map
func MergeContextFields(ctx context.Context, additionalFields map[string]interface{}, keys ...ContextKey) map[string]interface{} {
	// Start with additional fields
	merged := make(map[string]interface{}, len(additionalFields))
	for k, v := range additionalFields {
		merged[k] = v
	}
	
	// Override with context fields
	contextFields := ExtractContextFields(ctx, keys...)
	for k, v := range contextFields {
		merged[k] = v
	}
	
	return merged
}

// WithContextFields returns a new context with the provided fields.
// This is useful for propagating logging context through a call chain.
//
// Parameters:
//   - ctx: The parent context
//   - fields: Fields to add to the context
//
// Returns:
//   - context.Context: A new context with the fields
//
// Example:
//
//	ctx = WithContextFields(ctx, map[ContextKey]interface{}{
//		ContextKeyRequestID: "123",
//		ContextKeyUserID: "456",
//	})
func WithContextFields(ctx context.Context, fields map[ContextKey]interface{}) context.Context {
	for key, value := range fields {
		ctx = context.WithValue(ctx, key, value)
	}
	return ctx
}

// TraceContext adds tracing information to a context.
// This is useful for distributed tracing scenarios.
//
// Parameters:
//   - ctx: The parent context
//   - traceID: The trace ID
//   - spanID: The span ID
//   - parentSpanID: The parent span ID (optional)
//
// Returns:
//   - context.Context: A new context with tracing information
func TraceContext(ctx context.Context, traceID, spanID, parentSpanID string) context.Context {
	ctx = context.WithValue(ctx, ContextKeyTraceID, traceID)
	ctx = context.WithValue(ctx, ContextKeySpanID, spanID)
	if parentSpanID != "" {
		ctx = context.WithValue(ctx, ContextKeyParentSpan, parentSpanID)
	}
	return ctx
}

// RequestContext creates a context with common HTTP request information.
// This is a convenience function for web applications.
//
// Parameters:
//   - ctx: The parent context
//   - requestID: The request ID
//   - method: The HTTP method
//   - path: The request path
//   - sourceIP: The client IP address
//
// Returns:
//   - context.Context: A new context with request information
func RequestContext(ctx context.Context, requestID, method, path, sourceIP string) context.Context {
	return WithContextFields(ctx, map[ContextKey]interface{}{
		ContextKeyRequestID: requestID,
		ContextKeyMethod:    method,
		ContextKeyPath:      path,
		ContextKeySourceIP:  sourceIP,
		ContextKeyTimestamp: time.Now(),
	})
}

// UserContext adds user information to a context.
// This is useful for audit logging and user-specific operations.
//
// Parameters:
//   - ctx: The parent context
//   - userID: The user ID
//   - sessionID: The session ID (optional)
//
// Returns:
//   - context.Context: A new context with user information
func UserContext(ctx context.Context, userID, sessionID string) context.Context {
	ctx = context.WithValue(ctx, ContextKeyUserID, userID)
	if sessionID != "" {
		ctx = context.WithValue(ctx, ContextKeySessionID, sessionID)
	}
	return ctx
}

// OperationContext creates a context for a specific operation.
// This helps track the flow of operations through the system.
//
// Parameters:
//   - ctx: The parent context
//   - component: The component/service name
//   - operation: The operation being performed
//
// Returns:
//   - context.Context: A new context with operation information
func OperationContext(ctx context.Context, component, operation string) context.Context {
	return WithContextFields(ctx, map[ContextKey]interface{}{
		ContextKeyComponent: component,
		ContextKeyOperation: operation,
		ContextKeyTimestamp: time.Now(),
	})
}

// ErrorContext adds error information to a context.
// This is useful for error tracking and debugging.
//
// Parameters:
//   - ctx: The parent context
//   - err: The error to add
//
// Returns:
//   - context.Context: A new context with error information
func ErrorContext(ctx context.Context, err error) context.Context {
	if err == nil {
		return ctx
	}
	
	fields := map[ContextKey]interface{}{
		ContextKeyError: err.Error(),
	}
	
	// Add stack trace for debugging
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	if n > 0 {
		fields["stack_trace"] = string(buf[:n])
	}
	
	return WithContextFields(ctx, fields)
}

// DurationContext wraps an operation and adds its duration to the context.
// This is useful for performance monitoring.
//
// Parameters:
//   - ctx: The parent context
//   - operation: The operation name
//   - fn: The function to execute
//
// Returns:
//   - context.Context: Updated context with duration
//   - error: Any error from the function
//
// Example:
//
//	ctx, err := DurationContext(ctx, "database_query", func() error {
//		return db.Query(...)
//	})
func DurationContext(ctx context.Context, operation string, fn func() error) (context.Context, error) {
	start := time.Now()
	err := fn()
	duration := time.Since(start)
	
	ctx = WithContextFields(ctx, map[ContextKey]interface{}{
		ContextKeyOperation: operation,
		ContextKeyDuration:  duration,
		ContextKeyStatus:    getStatus(err),
	})
	
	if err != nil {
		ctx = ErrorContext(ctx, err)
	}
	
	return ctx, err
}

// getStatus returns a status string based on error
func getStatus(err error) string {
	if err != nil {
		return "error"
	}
	return "success"
}

// LogLevelFromContext extracts a log level override from context.
// This allows dynamic log level adjustment for specific operations.
//
// Parameters:
//   - ctx: The context to check
//   - defaultLevel: The default level to use if none in context
//
// Returns:
//   - int: The log level to use
func LogLevelFromContext(ctx context.Context, defaultLevel int) int {
	if level := ctx.Value(ContextKeyLogLevel); level != nil {
		if intLevel, ok := level.(int); ok {
			return intLevel
		}
	}
	return defaultLevel
}

// ContextWithLogLevel returns a context with a specific log level.
// This can be used to temporarily increase log verbosity for debugging.
//
// Parameters:
//   - ctx: The parent context
//   - level: The log level to set
//
// Returns:
//   - context.Context: A new context with the log level
func ContextWithLogLevel(ctx context.Context, level int) context.Context {
	return context.WithValue(ctx, ContextKeyLogLevel, level)
}

// EnvironmentContext adds environment information to a context.
// This helps distinguish logs from different environments.
//
// Parameters:
//   - ctx: The parent context
//   - env: The environment name (dev, staging, prod, etc.)
//   - version: The application version
//   - buildID: The build/commit ID (optional)
//
// Returns:
//   - context.Context: A new context with environment information
func EnvironmentContext(ctx context.Context, env, version, buildID string) context.Context {
	fields := map[ContextKey]interface{}{
		ContextKeyEnvironment: env,
		ContextKeyVersion:     version,
	}
	if buildID != "" {
		fields[ContextKeyBuildID] = buildID
	}
	return WithContextFields(ctx, fields)
}

// CorrelationContext creates a context with a correlation ID.
// This helps track related events across services.
//
// Parameters:
//   - ctx: The parent context
//   - correlationID: The correlation ID (generates one if empty)
//
// Returns:
//   - context.Context: A new context with correlation ID
//   - string: The correlation ID used
func CorrelationContext(ctx context.Context, correlationID string) (context.Context, string) {
	if correlationID == "" {
		correlationID = generateCorrelationID()
	}
	ctx = context.WithValue(ctx, ContextKeyCorrelation, correlationID)
	return ctx, correlationID
}

// generateCorrelationID generates a unique correlation ID
func generateCorrelationID() string {
	// Simple implementation - in production, use a proper UUID library
	return fmt.Sprintf("corr-%d-%d", time.Now().UnixNano(), runtime.NumGoroutine())
}

// FormatContextFields formats context fields for display.
// This is useful for debugging context contents.
//
// Parameters:
//   - ctx: The context to format
//   - keys: The keys to include (if empty, includes all common keys)
//
// Returns:
//   - string: Formatted context fields
func FormatContextFields(ctx context.Context, keys ...ContextKey) string {
	fields := ExtractContextFields(ctx, keys...)
	if len(fields) == 0 {
		return "no context fields"
	}
	
	var parts []string
	for k, v := range fields {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	
	return strings.Join(parts, " ")
}

// ContextLogger provides context-aware logging by automatically including
// context fields in all log entries. It wraps an existing logger and
// enriches log messages with contextual information.
type ContextLogger struct {
	logger omni.Logger
	ctx    context.Context
	fields map[string]interface{}
}

// NewContextLogger creates a new context-aware logger.
// This logger automatically includes context fields in all log entries.
//
// Parameters:
//   - logger: The underlying logger to wrap
//   - ctx: The context to extract fields from
//
// Returns:
//   - *ContextLogger: A new context-aware logger
//
// Example:
//
//	ctxLogger := NewContextLogger(logger, ctx)
//	ctxLogger.Info("Processing request") // Automatically includes context fields
func NewContextLogger(logger omni.Logger, ctx context.Context) *ContextLogger {
	return &ContextLogger{
		logger: logger,
		ctx:    ctx,
		fields: ExtractContextFields(ctx),
	}
}

// Trace logs a trace message with context fields
func (cl *ContextLogger) Trace(args ...interface{}) {
	cl.logger.WithFields(cl.fields).Trace(args...)
}

// Tracef logs a formatted trace message with context fields
func (cl *ContextLogger) Tracef(format string, args ...interface{}) {
	cl.logger.WithFields(cl.fields).Tracef(format, args...)
}

// Debug logs a debug message with context fields
func (cl *ContextLogger) Debug(args ...interface{}) {
	cl.logger.WithFields(cl.fields).Debug(args...)
}

// Debugf logs a formatted debug message with context fields
func (cl *ContextLogger) Debugf(format string, args ...interface{}) {
	cl.logger.WithFields(cl.fields).Debugf(format, args...)
}

// Info logs an info message with context fields
func (cl *ContextLogger) Info(args ...interface{}) {
	cl.logger.WithFields(cl.fields).Info(args...)
}

// Infof logs a formatted info message with context fields
func (cl *ContextLogger) Infof(format string, args ...interface{}) {
	cl.logger.WithFields(cl.fields).Infof(format, args...)
}

// Warn logs a warning message with context fields
func (cl *ContextLogger) Warn(args ...interface{}) {
	cl.logger.WithFields(cl.fields).Warn(args...)
}

// Warnf logs a formatted warning message with context fields
func (cl *ContextLogger) Warnf(format string, args ...interface{}) {
	cl.logger.WithFields(cl.fields).Warnf(format, args...)
}

// Error logs an error message with context fields
func (cl *ContextLogger) Error(args ...interface{}) {
	cl.logger.WithFields(cl.fields).Error(args...)
}

// Errorf logs a formatted error message with context fields
func (cl *ContextLogger) Errorf(format string, args ...interface{}) {
	cl.logger.WithFields(cl.fields).Errorf(format, args...)
}

// WithContext returns a new ContextLogger with an updated context.
// This allows changing the context while preserving the logger configuration.
//
// Parameters:
//   - ctx: The new context to use
//
// Returns:
//   - Logger: A new ContextLogger with updated context
func (cl *ContextLogger) WithContext(ctx context.Context) omni.Logger {
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
func (cl *ContextLogger) WithFields(fields map[string]interface{}) omni.Logger {
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
//
// Parameters:
//   - key: The field key
//   - value: The field value
//
// Returns:
//   - Logger: A new ContextLogger with the added field
func (cl *ContextLogger) WithField(key string, value interface{}) omni.Logger {
	return cl.WithFields(map[string]interface{}{key: value})
}

// WithError returns a new ContextLogger with an error field.
//
// Parameters:
//   - err: The error to add
//
// Returns:
//   - Logger: A new ContextLogger with error information
func (cl *ContextLogger) WithError(err error) omni.Logger {
	if err == nil {
		return cl
	}
	return cl.WithField("error", err.Error())
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

// Context returns the underlying context
func (cl *ContextLogger) Context() context.Context {
	return cl.ctx
}

// Logger returns the underlying logger
func (cl *ContextLogger) Logger() omni.Logger {
	return cl.logger
}

// Fields returns the current fields
func (cl *ContextLogger) Fields() map[string]interface{} {
	return cl.fields
}