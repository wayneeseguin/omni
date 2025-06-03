package omni

import (
	"time"
)

// Option is a functional option for configuring Omni.
// Options provide a clean, extensible way to configure logger instances
// using the functional options pattern.
type Option func(*Config) error

// NewWithOptions creates a new Omni instance with the provided options.
// This is an alternative to NewWithConfig that uses functional options
// for a more ergonomic API.
//
// Parameters:
//   - options: Variable number of Option functions
//
// Returns:
//   - *Omni: The configured logger instance
//   - error: Configuration or initialization error
//
// Example:
//
//	logger, err := omni.NewWithOptions(
//	    omni.WithPath("/var/log/app.log"),
//	    omni.WithLevel(omni.LevelInfo),
//	    omni.WithJSON(),
//	    omni.WithRotation(100*1024*1024, 10),
//	    omni.WithGzipCompression(),
//	)
func NewWithOptions(options ...Option) (*Omni, error) {
	// Start with default config
	config := &Config{
		Level:           LevelInfo,
		Format:          FormatText,
		ChannelSize:     getDefaultChannelSize(),
		MaxSize:         defaultMaxSize,
		MaxFiles:        defaultMaxFiles,
		IncludeTrace:    false,
		StackSize:       4096,
		Compression:     CompressionNone,
		CompressMinAge:  1,
		CompressWorkers: 1,
		FormatOptions:   defaultFormatOptions(),
	}

	// Apply all options
	for _, opt := range options {
		if err := opt(config); err != nil {
			return nil, err
		}
	}

	// Create logger with config
	return NewWithConfig(config)
}

// WithPath sets the primary log file path.
// This option is required for file-based logging.
//
// Parameters:
//   - path: The log file path (cannot be empty)
//
// Returns:
//   - Option: The configuration option
func WithPath(path string) Option {
	return func(c *Config) error {
		if path == "" {
			return NewOmniError(ErrCodeInvalidConfig, "config", "", nil).
				WithContext("error", "path cannot be empty")
		}
		c.Path = path
		return nil
	}
}

// WithLevel sets the minimum log level.
// Only messages at or above this level will be logged.
//
// Parameters:
//   - level: The minimum log level (LevelTrace to LevelError)
//
// Returns:
//   - Option: The configuration option
func WithLevel(level int) Option {
	return func(c *Config) error {
		if level < LevelTrace || level > LevelError {
			return NewOmniError(ErrCodeInvalidLevel, "config", "", nil).
				WithContext("level", level)
		}
		c.Level = level
		return nil
	}
}

// WithFormat sets the output format.
// Supported formats are FormatText and FormatJSON.
//
// Parameters:
//   - format: The output format constant
//
// Returns:
//   - Option: The configuration option
func WithFormat(format int) Option {
	return func(c *Config) error {
		if format != FormatText && format != FormatJSON {
			return NewOmniError(ErrCodeInvalidFormat, "config", "", nil).
				WithContext("format", format)
		}
		c.Format = format
		return nil
	}
}

// WithJSON sets JSON output format.
// This is a convenience method for WithFormat(FormatJSON).
//
// Returns:
//   - Option: The configuration option
func WithJSON() Option {
	return WithFormat(FormatJSON)
}

// WithText sets text output format.
// This is a convenience method for WithFormat(FormatText).
//
// Returns:
//   - Option: The configuration option
func WithText() Option {
	return WithFormat(FormatText)
}

// WithChannelSize sets the message channel buffer size.
// A larger buffer can handle burst traffic better but uses more memory.
//
// Parameters:
//   - size: The buffer size (must be positive)
//
// Returns:
//   - Option: The configuration option
func WithChannelSize(size int) Option {
	return func(c *Config) error {
		if size <= 0 {
			return NewOmniError(ErrCodeInvalidConfig, "config", "", nil).
				WithContext("channelSize", size)
		}
		c.ChannelSize = size
		return nil
	}
}

// WithRotation configures log rotation.
// Files are rotated when they reach maxSize bytes.
//
// Parameters:
//   - maxSize: Maximum file size in bytes before rotation
//   - maxFiles: Maximum number of rotated files to keep (0 = unlimited)
//
// Returns:
//   - Option: The configuration option
func WithRotation(maxSize int64, maxFiles int) Option {
	return func(c *Config) error {
		c.MaxSize = maxSize
		c.MaxFiles = maxFiles
		return nil
	}
}

// WithCompression enables compression with specified type and workers.
// Rotated log files will be compressed to save disk space.
//
// Parameters:
//   - compressionType: Type of compression (e.g., CompressionGzip)
//   - workers: Number of compression workers (0 uses default)
//
// Returns:
//   - Option: The configuration option
func WithCompression(compressionType int, workers int) Option {
	return func(c *Config) error {
		c.Compression = compressionType
		if workers > 0 {
			c.CompressWorkers = workers
		}
		return nil
	}
}

// WithGzipCompression enables gzip compression.
// This is a convenience method for WithCompression(CompressionGzip, 1).
//
// Returns:
//   - Option: The configuration option
func WithGzipCompression() Option {
	return WithCompression(CompressionGzip, 1)
}

// WithStackTrace enables stack trace capture.
// Stack traces will be included in error-level logs.
//
// Parameters:
//   - size: Stack buffer size in bytes (0 uses default of 4096)
//
// Returns:
//   - Option: The configuration option
func WithStackTrace(size int) Option {
	return func(c *Config) error {
		c.IncludeTrace = true
		if size > 0 {
			c.StackSize = size
		}
		return nil
	}
}

// WithSampling configures log sampling.
// Sampling reduces log volume by only logging a percentage of messages.
//
// Parameters:
//   - strategy: Sampling strategy (e.g., SamplingRandom)
//   - rate: Sampling rate from 0.0 to 1.0 (1.0 = log all messages)
//
// Returns:
//   - Option: The configuration option
func WithSampling(strategy int, rate float64) Option {
	return func(c *Config) error {
		if rate < 0 || rate > 1 {
			return NewOmniError(ErrCodeInvalidConfig, "config", "", nil).
				WithContext("samplingRate", rate).
				WithContext("error", "rate must be between 0 and 1")
		}
		c.SamplingStrategy = strategy
		c.SamplingRate = rate
		return nil
	}
}

// WithRateSampling enables rate-based sampling.
// This is a convenience method for WithSampling(SamplingRandom, rate).
//
// Parameters:
//   - rate: Percentage of messages to log (0.0 to 1.0)
//
// Returns:
//   - Option: The configuration option
func WithRateSampling(rate float64) Option {
	return WithSampling(SamplingRandom, rate)
}

// WithFilter adds a filter function.
// Filters determine whether a message should be logged.
// Multiple filters can be added and all must pass for a message to be logged.
//
// Parameters:
//   - filter: The filter function (cannot be nil)
//
// Returns:
//   - Option: The configuration option
func WithFilter(filter FilterFunc) Option {
	return func(c *Config) error {
		if filter == nil {
			return NewOmniError(ErrCodeInvalidConfig, "config", "", nil).
				WithContext("error", "filter cannot be nil")
		}
		if c.Filters == nil {
			c.Filters = make([]FilterFunc, 0)
		}
		c.Filters = append(c.Filters, filter)
		return nil
	}
}

// WithLevelFilter creates a filter that only allows messages at or above the specified level.
// This provides more granular control than the global log level.
//
// Parameters:
//   - minLevel: Minimum level to allow through the filter
//
// Returns:
//   - Option: The configuration option
func WithLevelFilter(minLevel int) Option {
	return WithFilter(func(level int, message string, fields map[string]interface{}) bool {
		return level >= minLevel
	})
}

// WithErrorHandler sets the error handler.
// The error handler is called when logging operations fail.
//
// Parameters:
//   - handler: The error handler function (cannot be nil)
//
// Returns:
//   - Option: The configuration option
func WithErrorHandler(handler ErrorHandler) Option {
	return func(c *Config) error {
		if handler == nil {
			return NewOmniError(ErrCodeInvalidConfig, "config", "", nil).
				WithContext("error", "error handler cannot be nil")
		}
		c.ErrorHandler = handler
		return nil
	}
}

// WithMaxAge sets the maximum age for log files.
// Files older than this duration will be automatically deleted.
//
// Parameters:
//   - duration: Maximum age (must be non-negative)
//
// Returns:
//   - Option: The configuration option
func WithMaxAge(duration time.Duration) Option {
	return func(c *Config) error {
		if duration < 0 {
			return NewOmniError(ErrCodeInvalidConfig, "config", "", nil).
				WithContext("maxAge", duration).
				WithContext("error", "duration cannot be negative")
		}
		c.MaxAge = duration
		return nil
	}
}

// WithCleanupInterval sets the cleanup interval.
// This controls how often old log files are checked and removed.
//
// Parameters:
//   - interval: Cleanup check interval (must be positive)
//
// Returns:
//   - Option: The configuration option
func WithCleanupInterval(interval time.Duration) Option {
	return func(c *Config) error {
		if interval <= 0 {
			return NewOmniError(ErrCodeInvalidConfig, "config", "", nil).
				WithContext("cleanupInterval", interval).
				WithContext("error", "interval must be positive")
		}
		c.CleanupInterval = interval
		return nil
	}
}

// WithTimezone sets the timezone for timestamps.
// By default, local time is used.
//
// Parameters:
//   - tz: The timezone location (cannot be nil)
//
// Returns:
//   - Option: The configuration option
func WithTimezone(tz *time.Location) Option {
	return func(c *Config) error {
		if tz == nil {
			return NewOmniError(ErrCodeInvalidConfig, "config", "", nil).
				WithContext("error", "timezone cannot be nil")
		}
		c.FormatOptions.TimeZone = tz
		return nil
	}
}

// WithUTC sets UTC timezone.
// This is a convenience method for WithTimezone(time.UTC).
//
// Returns:
//   - Option: The configuration option
func WithUTC() Option {
	return WithTimezone(time.UTC)
}

// WithTimestampFormat sets the timestamp format.
// Uses Go's time format syntax.
//
// Parameters:
//   - format: Time format string (cannot be empty)
//
// Returns:
//   - Option: The configuration option
//
// Example:
//
//	WithTimestampFormat("2006-01-02 15:04:05.000")
func WithTimestampFormat(format string) Option {
	return func(c *Config) error {
		if format == "" {
			return NewOmniError(ErrCodeInvalidConfig, "config", "", nil).
				WithContext("error", "timestamp format cannot be empty")
		}
		c.FormatOptions.TimestampFormat = format
		return nil
	}
}

// WithRecovery enables recovery with fallback.
// If the primary log destination fails, logging will fall back to the specified path.
//
// Parameters:
//   - fallbackPath: Path to use when primary fails
//   - maxRetries: Maximum retry attempts
//
// Returns:
//   - Option: The configuration option
func WithRecovery(fallbackPath string, maxRetries int) Option {
	return func(c *Config) error {
		if c.Recovery == nil {
			c.Recovery = &RecoveryConfig{}
		}
		c.Recovery.FallbackPath = fallbackPath
		c.Recovery.MaxRetries = maxRetries
		c.Recovery.RetryDelay = 100 * time.Millisecond
		c.Recovery.BackoffMultiplier = 2.0
		c.Recovery.MaxRetryDelay = 5 * time.Second
		return nil
	}
}

// WithRedaction enables sensitive data redaction.
// Patterns matching sensitive data will be replaced in log output.
//
// Parameters:
//   - patterns: Regex patterns to match sensitive data
//   - replacement: String to replace matched patterns
//
// Returns:
//   - Option: The configuration option
func WithRedaction(patterns []string, replacement string) Option {
	return func(c *Config) error {
		c.RedactionPatterns = patterns
		c.RedactionReplace = replacement
		return nil
	}
}

// WithBatchProcessing enables batch processing for writes.
// Messages are batched to improve write performance.
//
// Parameters:
//   - maxSize: Maximum batch size in bytes
//   - maxCount: Maximum number of messages per batch
//   - flushInterval: Maximum time before flushing a batch
//
// Returns:
//   - Option: The configuration option
func WithBatchProcessing(maxSize, maxCount int, flushInterval time.Duration) Option {
	return func(c *Config) error {
		if maxSize <= 0 || maxCount <= 0 {
			return NewOmniError(ErrCodeInvalidConfig, "config", "", nil).
				WithContext("error", "batch size and count must be positive")
		}
		if flushInterval < 0 {
			return NewOmniError(ErrCodeInvalidConfig, "config", "", nil).
				WithContext("error", "flush interval cannot be negative")
		}
		c.EnableBatching = true
		c.BatchMaxSize = maxSize
		c.BatchMaxCount = maxCount
		c.BatchFlushInterval = flushInterval
		return nil
	}
}

// WithDefaultBatching enables batching with default settings.
// Uses 64KB max size, 100 message max count, and 100ms flush interval.
//
// Returns:
//   - Option: The configuration option
func WithDefaultBatching() Option {
	return WithBatchProcessing(64*1024, 100, 100*time.Millisecond) // 64KB, 100 entries, 100ms
}

// Preset configurations

// WithProductionDefaults sets recommended production settings.
// Configures the logger with:
// - Info level logging
// - JSON format
// - Large buffer (10000)
// - 100MB rotation with 10 files
// - Gzip compression
// - UTC timestamps
// - Stderr error handling
//
// Returns:
//   - Option: The configuration option
func WithProductionDefaults() Option {
	return func(c *Config) error {
		c.Level = LevelInfo
		c.Format = FormatJSON
		c.ChannelSize = 10000
		c.MaxSize = 100 * 1024 * 1024 // 100MB
		c.MaxFiles = 10
		c.Compression = CompressionGzip
		c.CompressWorkers = 2
		c.ErrorHandler = StderrErrorHandler
		c.FormatOptions.TimeZone = time.UTC
		return nil
	}
}

// WithDevelopmentDefaults sets recommended development settings.
// Configures the logger with:
// - Debug level logging
// - Text format
// - Moderate buffer (1000)
// - Stack traces enabled
// - Local timestamps
// - Stderr error handling
//
// Returns:
//   - Option: The configuration option
func WithDevelopmentDefaults() Option {
	return func(c *Config) error {
		c.Level = LevelDebug
		c.Format = FormatText
		c.ChannelSize = 1000
		c.IncludeTrace = true
		c.ErrorHandler = StderrErrorHandler
		c.FormatOptions.TimeZone = time.Local
		return nil
	}
}

