package features

import (
	"regexp"
	"time"

	"github.com/wayneeseguin/omni/pkg/types"
)

// Compressor interface for log file compression
type Compressor interface {
	// Compress compresses a file
	Compress(src, dest string) error
	
	// Type returns the compression type
	Type() int
	
	// Extension returns the file extension for compressed files
	Extension() string
}

// Rotator interface for log file rotation
type Rotator interface {
	// Rotate rotates the log file
	Rotate(currentPath string) error
	
	// ShouldRotate determines if rotation is needed
	ShouldRotate(size int64, age time.Duration) bool
	
	// GetRotatedFilename generates a rotated filename
	GetRotatedFilename(basePath string, index int) string
}

// Filter interface for log message filtering
type Filter interface {
	// ShouldLog determines if a message should be logged
	ShouldLog(level int, message string, fields map[string]interface{}) bool
	
	// Name returns the filter name for identification
	Name() string
}

// RegexFilter filters based on regular expression patterns
type RegexFilter struct {
	Pattern *regexp.Regexp
	Include bool // true to include matches, false to exclude
}

// ShouldLog implements Filter interface
func (f *RegexFilter) ShouldLog(level int, message string, fields map[string]interface{}) bool {
	matches := f.Pattern.MatchString(message)
	if f.Include {
		return matches
	}
	return !matches
}

// Name returns the filter name
func (f *RegexFilter) Name() string {
	return "regex"
}

// LevelFilter filters based on log levels
type LevelFilter struct {
	MinLevel int
	MaxLevel int
}

// ShouldLog implements Filter interface
func (f *LevelFilter) ShouldLog(level int, message string, fields map[string]interface{}) bool {
	return level >= f.MinLevel && level <= f.MaxLevel
}

// Name returns the filter name
func (f *LevelFilter) Name() string {
	return "level"
}

// Sampler interface for log sampling
type Sampler interface {
	// ShouldSample determines if a message should be sampled
	ShouldSample(level int, message string, fields map[string]interface{}) bool
	
	// Rate returns the current sampling rate
	Rate() float64
	
	// SetRate sets the sampling rate
	SetRate(rate float64)
}

// RedactorInterface interface for sensitive data redaction
type RedactorInterface interface {
	// Redact redacts sensitive data from a message
	Redact(message string) string
	
	// RedactFields redacts sensitive data from structured fields
	RedactFields(fields map[string]interface{}) map[string]interface{}
	
	// AddPattern adds a redaction pattern
	AddPattern(pattern string)
	
	// RemovePattern removes a redaction pattern
	RemovePattern(pattern string)
}

// Recovery interface for error recovery
type Recovery interface {
	// Recover attempts to recover from an error
	Recover(err error) error
	
	// IsRecoverable determines if an error is recoverable
	IsRecoverable(err error) bool
	
	// GetFallbackOptions returns fallback options
	GetFallbackOptions() []string
}

// CompressionConfig holds compression configuration
type CompressionConfig struct {
	Type       int  // Compression type (types.CompressionNone, types.CompressionGzip)
	MinAge     int  // Minimum number of rotations before compression
	Workers    int  // Number of compression workers
	Enabled    bool // Whether compression is enabled
}

// RotationConfig holds rotation configuration
type RotationConfig struct {
	MaxSize       int64         // Maximum file size before rotation
	MaxFiles      int           // Maximum number of files to keep
	MaxAge        time.Duration // Maximum age of files
	KeepOriginal  bool          // Whether to keep the original file
}

// SamplingConfig holds sampling configuration
type SamplingConfig struct {
	Strategy int     // Sampling strategy (types.SamplingNone, types.SamplingRandom, etc.)
	Rate     float64 // Sampling rate (0.0-1.0)
	KeyFunc  func(int, string, map[string]interface{}) string // Key function for consistent sampling
}

// FieldRedactionRule defines redaction rules for specific fields
type FieldRedactionRule struct {
	FieldPath string // JSONPath-like field path
	Pattern   string // Regex pattern to match
	Replace   string // Replacement string
}

// Logger interface that defines the basic logging operations
type Logger interface {
	// Basic logging methods
	Debug(args ...interface{})
	Info(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})
	
	// Formatted logging methods
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	
	// Close closes the logger
	Close() error
}

// StructuredLogger interface for structured logging with fields
type StructuredLogger interface {
	Logger
	
	// WithField adds a single field
	WithField(key string, value interface{}) StructuredLogger
	
	// WithFields adds multiple fields
	WithFields(fields map[string]interface{}) StructuredLogger
}

// FilterableLogger interface for loggers that support filtering
type FilterableLogger interface {
	Logger
	
	// AddFilter adds a filter function
	AddFilter(filter types.FilterFunc) error
	
	// RemoveFilter removes a filter function
	RemoveFilter(filter types.FilterFunc) error
	
	// ClearFilters removes all filters
	ClearFilters()
}

// SamplableLogger interface for loggers that support sampling
type SamplableLogger interface {
	Logger
	
	// SetSampling configures sampling
	SetSampling(strategy int, rate float64) error
	
	// GetSamplingRate returns current sampling rate
	GetSamplingRate() float64
}

// RotatableLogger interface for loggers that support rotation
type RotatableLogger interface {
	Logger
	
	// SetMaxSize sets maximum file size before rotation
	SetMaxSize(size int64)
	
	// SetMaxFiles sets maximum number of files to keep
	SetMaxFiles(files int)
	
	// SetMaxAge sets maximum age for log files
	SetMaxAge(age time.Duration) error
	
	// Rotate manually triggers rotation
	Rotate() error
}

// CompressibleLogger interface for loggers that support compression
type CompressibleLogger interface {
	Logger
	
	// SetCompression enables/disables compression
	SetCompression(compressionType int) error
	
	// SetCompressMinAge sets minimum age before compression
	SetCompressMinAge(age int)
	
	// SetCompressWorkers sets number of compression workers
	SetCompressWorkers(workers int)
}

// RecoverableLogger interface for loggers that support error recovery
type RecoverableLogger interface {
	Logger
	
	// SetRecoveryConfig sets recovery configuration (defined in recovery.go)
	SetRecoveryConfig(config interface{})
	
	// RecoverFromError attempts to recover from an error
	RecoverFromError(err error, msg types.LogMessage, dest *types.Destination)
}

// RedactableLogger interface for loggers that support redaction
type RedactableLogger interface {
	Logger
	
	// SetRedaction sets redaction patterns
	SetRedaction(patterns []string, replace string) error
	
	// SetRedactionConfig sets advanced redaction configuration (defined in redaction.go)
	SetRedactionConfig(config interface{})
	
	// EnableRedactionForLevel enables/disables redaction for a specific level
	EnableRedactionForLevel(level int, enable bool)
}

// MetricsLogger interface for loggers that support metrics
type MetricsLogger interface {
	Logger
	
	// GetMetrics returns logger metrics
	GetMetrics() map[string]interface{}
	
	// ResetMetrics resets all metrics
	ResetMetrics()
}

// FullFeaturedLogger interface that combines all logger capabilities
type FullFeaturedLogger interface {
	StructuredLogger
	FilterableLogger
	SamplableLogger
	RotatableLogger
	CompressibleLogger
	RecoverableLogger
	RedactableLogger
	MetricsLogger
}