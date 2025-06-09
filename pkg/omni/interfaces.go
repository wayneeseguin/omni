package omni

import (
	"context"
	"time"
)

// Logger is the core logging interface that provides basic logging functionality.
// All methods are thread-safe and can be called concurrently.
//
// Example usage:
//
//	var logger omni.Logger = omni.New("/var/log/app.log")
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
//	var manager omni.Manager = logger
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

// Closeable represents something that can be closed
type Closeable interface {
	Close() error
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

// Ensure Omni implements the interfaces
var (
	_ Logger           = (*Omni)(nil)
	_ Manager          = (*Omni)(nil)
	_ FilterableLogger = (*Omni)(nil)
	_ SamplableLogger  = (*Omni)(nil)
	_ MetricsProvider  = (*Omni)(nil)
	_ ErrorReporter    = (*Omni)(nil)
	_ Closeable        = (*Omni)(nil)
)
