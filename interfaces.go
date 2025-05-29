package flexlog

import (
	"context"
	"fmt"
	"time"
)

// Logger is the core logging interface that provides basic logging functionality.
// All methods are thread-safe and can be called concurrently.
//
// Example usage:
//
//	var logger flexlog.Logger = flexlog.New("/var/log/app.log")
//	logger.Info("Server started", "port", 8080)
//	logger.WithField("user_id", 123).Debug("Processing request")
type Logger interface {
	// Basic logging methods
	Trace(args ...interface{})
	Tracef(format string, args ...interface{})
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})

	// Structured logging
	WithFields(fields map[string]interface{}) Logger
	WithField(key string, value interface{}) Logger
	WithError(err error) Logger
	WithContext(ctx context.Context) Logger

	// Control methods
	SetLevel(level int)
	GetLevel() int
	IsLevelEnabled(level int) bool
}

// Manager handles logger configuration and lifecycle management.
// It provides methods for runtime configuration changes, destination management,
// and graceful shutdown.
//
// Example usage:
//
//	var manager flexlog.Manager = logger
//	manager.SetMaxSize(50 * 1024 * 1024) // 50MB
//	manager.AddDestination("/var/log/errors.log")
//	manager.FlushAll() // Ensure all logs are written
//	manager.Shutdown(context.Background()) // Graceful shutdown
type Manager interface {
	// Configuration
	SetFormat(format int) error
	GetFormat() int
	SetMaxSize(size int64)
	GetMaxSize() int64
	SetMaxFiles(count int)
	GetMaxFiles() int
	SetCompression(compressionType int) error
	SetMaxAge(duration time.Duration) error

	// Destination management
	AddDestination(uri string) error
	AddDestinationWithBackend(uri string, backendType int) error
	RemoveDestination(name string) error
	ListDestinations() []string
	EnableDestination(name string) error
	DisableDestination(name string) error

	// Lifecycle management
	Flush() error
	FlushAll() error
	Sync() error
	Close() error
	Shutdown(ctx context.Context) error

	// Metrics and monitoring
	GetMetrics() LoggerMetrics
	ResetMetrics()
	GetErrors() <-chan LogError
}

// Destination represents a log output destination
type DestinationInterface interface {
	// Write writes a log entry to the destination
	Write(entry []byte) (int, error)
	
	// Flush ensures all buffered data is written
	Flush() error
	
	// Close closes the destination
	Close() error
	
	// Info returns information about the destination
	Info() DestinationInfo
}

// DestinationInfo provides information about a destination
type DestinationInfo struct {
	Name         string
	Type         string
	URI          string
	Enabled      bool
	BytesWritten uint64
	Errors       uint64
	LastWrite    time.Time
}

// FilterableLogger supports message filtering
type FilterableLogger interface {
	Logger
	AddFilter(filter FilterFunc) error
	RemoveFilter(filter FilterFunc) error
	ClearFilters()
}

// SamplableLogger supports log sampling
type SamplableLogger interface {
	Logger
	SetSampling(strategy int, rate float64) error
	GetSamplingRate() float64
}

// StructuredLogger provides enhanced structured logging capabilities
type StructuredLogger interface {
	Logger
	
	// Additional structured logging methods
	WithValues(keysAndValues ...interface{}) Logger
	WithName(name string) Logger
	WithCallDepth(depth int) Logger
}

// MetricsProvider provides access to logger metrics
type MetricsProvider interface {
	GetMetrics() LoggerMetrics
	ResetMetrics()
	GetErrorCount() uint64
	GetMessageCount(level int) uint64
}

// ErrorReporter handles error reporting
type ErrorReporter interface {
	SetErrorHandler(handler ErrorHandler)
	GetErrors() <-chan LogError
	GetLastError() *LogError
}

// LoggerFactory creates logger instances
type LoggerFactory interface {
	// Create a logger with default configuration
	Create(path string) (Logger, error)
	
	// Create a logger with options
	CreateWithOptions(options ...Option) (Logger, error)
	
	// Create a logger with config
	CreateWithConfig(config *Config) (Logger, error)
}

// Default factory implementation
type defaultFactory struct{}

// DefaultFactory is the default logger factory
var DefaultFactory LoggerFactory = &defaultFactory{}

func (f *defaultFactory) Create(path string) (Logger, error) {
	return New(path)
}

func (f *defaultFactory) CreateWithOptions(options ...Option) (Logger, error) {
	return NewWithOptions(options...)
}

func (f *defaultFactory) CreateWithConfig(config *Config) (Logger, error) {
	return NewWithConfig(config)
}

// Ensure FlexLog implements the interfaces
var (
	_ Logger           = (*FlexLog)(nil)
	_ Manager          = (*FlexLog)(nil)
	_ FilterableLogger = (*FlexLog)(nil)
	_ SamplableLogger  = (*FlexLog)(nil)
	_ MetricsProvider  = (*FlexLog)(nil)
	_ ErrorReporter    = (*FlexLog)(nil)
)

// LoggerAdapter allows using FlexLog where only the Logger interface is needed
type LoggerAdapter struct {
	*FlexLog
}

// NewLoggerAdapter creates a new logger adapter
func NewLoggerAdapter(logger *FlexLog) Logger {
	return &LoggerAdapter{logger}
}

// WithFields returns a new logger with the specified fields
func (a *LoggerAdapter) WithFields(fields map[string]interface{}) Logger {
	// Return a new adapter with the same underlying logger
	// The fields will be included in the log message
	return &fieldsLogger{
		logger: a.FlexLog,
		fields: fields,
	}
}

// WithField returns a new logger with a single field
func (a *LoggerAdapter) WithField(key string, value interface{}) Logger {
	return a.WithFields(map[string]interface{}{key: value})
}

// WithError returns a new logger with an error field
func (a *LoggerAdapter) WithError(err error) Logger {
	if err == nil {
		return a
	}
	return a.WithField("error", err.Error())
}

// WithContext returns a new logger with context values
func (a *LoggerAdapter) WithContext(ctx context.Context) Logger {
	// Extract trace ID or other context values if present
	return a
}

// fieldsLogger wraps FlexLog to include fields in every log message
type fieldsLogger struct {
	logger *FlexLog
	fields map[string]interface{}
}

// Implement all Logger methods for fieldsLogger
func (f *fieldsLogger) Trace(args ...interface{}) {
	// Convert args to message
	message := fmt.Sprint(args...)
	f.logger.TraceWithFields(message, f.fields)
}

func (f *fieldsLogger) Tracef(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	f.logger.TraceWithFields(message, f.fields)
}

func (f *fieldsLogger) Debug(args ...interface{}) {
	message := fmt.Sprint(args...)
	f.logger.DebugWithFields(message, f.fields)
}

func (f *fieldsLogger) Debugf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	f.logger.DebugWithFields(message, f.fields)
}

func (f *fieldsLogger) Info(args ...interface{}) {
	message := fmt.Sprint(args...)
	f.logger.InfoWithFields(message, f.fields)
}

func (f *fieldsLogger) Infof(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	f.logger.InfoWithFields(message, f.fields)
}

func (f *fieldsLogger) Warn(args ...interface{}) {
	message := fmt.Sprint(args...)
	f.logger.WarnWithFields(message, f.fields)
}

func (f *fieldsLogger) Warnf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	f.logger.WarnWithFields(message, f.fields)
}

func (f *fieldsLogger) Error(args ...interface{}) {
	message := fmt.Sprint(args...)
	f.logger.ErrorWithFields(message, f.fields)
}

func (f *fieldsLogger) Errorf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	f.logger.ErrorWithFields(message, f.fields)
}

func (f *fieldsLogger) WithFields(newFields map[string]interface{}) Logger {
	// Merge fields
	merged := make(map[string]interface{}, len(f.fields)+len(newFields))
	for k, v := range f.fields {
		merged[k] = v
	}
	for k, v := range newFields {
		merged[k] = v
	}
	return &fieldsLogger{
		logger: f.logger,
		fields: merged,
	}
}

func (f *fieldsLogger) WithField(key string, value interface{}) Logger {
	return f.WithFields(map[string]interface{}{key: value})
}

func (f *fieldsLogger) WithError(err error) Logger {
	if err == nil {
		return f
	}
	return f.WithField("error", err.Error())
}

func (f *fieldsLogger) WithContext(ctx context.Context) Logger {
	return f
}

func (f *fieldsLogger) SetLevel(level int) {
	f.logger.SetLevel(level)
}

func (f *fieldsLogger) GetLevel() int {
	return f.logger.GetLevel()
}

func (f *fieldsLogger) IsLevelEnabled(level int) bool {
	return f.logger.IsLevelEnabled(level)
}