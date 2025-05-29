package flexlog

import (
	"time"
)

// Option is a functional option for configuring FlexLog
type Option func(*Config) error

// NewWithOptions creates a new FlexLog instance with the provided options
func NewWithOptions(options ...Option) (*FlexLog, error) {
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

// WithPath sets the primary log file path
func WithPath(path string) Option {
	return func(c *Config) error {
		if path == "" {
			return NewFlexLogError(ErrCodeInvalidConfig, "config", "", nil).
				WithContext("error", "path cannot be empty")
		}
		c.Path = path
		return nil
	}
}

// WithLevel sets the minimum log level
func WithLevel(level int) Option {
	return func(c *Config) error {
		if level < LevelTrace || level > LevelError {
			return NewFlexLogError(ErrCodeInvalidLevel, "config", "", nil).
				WithContext("level", level)
		}
		c.Level = level
		return nil
	}
}

// WithFormat sets the output format
func WithFormat(format int) Option {
	return func(c *Config) error {
		if format != FormatText && format != FormatJSON {
			return NewFlexLogError(ErrCodeInvalidFormat, "config", "", nil).
				WithContext("format", format)
		}
		c.Format = format
		return nil
	}
}

// WithJSON sets JSON output format
func WithJSON() Option {
	return WithFormat(FormatJSON)
}

// WithText sets text output format
func WithText() Option {
	return WithFormat(FormatText)
}

// WithChannelSize sets the message channel buffer size
func WithChannelSize(size int) Option {
	return func(c *Config) error {
		if size <= 0 {
			return NewFlexLogError(ErrCodeInvalidConfig, "config", "", nil).
				WithContext("channelSize", size)
		}
		c.ChannelSize = size
		return nil
	}
}

// WithRotation configures log rotation
func WithRotation(maxSize int64, maxFiles int) Option {
	return func(c *Config) error {
		c.MaxSize = maxSize
		c.MaxFiles = maxFiles
		return nil
	}
}

// WithCompression enables compression with specified type and workers
func WithCompression(compressionType int, workers int) Option {
	return func(c *Config) error {
		c.Compression = compressionType
		if workers > 0 {
			c.CompressWorkers = workers
		}
		return nil
	}
}

// WithGzipCompression enables gzip compression
func WithGzipCompression() Option {
	return WithCompression(CompressionGzip, 1)
}

// WithStackTrace enables stack trace capture
func WithStackTrace(size int) Option {
	return func(c *Config) error {
		c.IncludeTrace = true
		if size > 0 {
			c.StackSize = size
		}
		return nil
	}
}

// WithSampling configures log sampling
func WithSampling(strategy int, rate float64) Option {
	return func(c *Config) error {
		if rate < 0 || rate > 1 {
			return NewFlexLogError(ErrCodeInvalidConfig, "config", "", nil).
				WithContext("samplingRate", rate).
				WithContext("error", "rate must be between 0 and 1")
		}
		c.SamplingStrategy = strategy
		c.SamplingRate = rate
		return nil
	}
}

// WithRateSampling enables rate-based sampling
func WithRateSampling(rate float64) Option {
	return WithSampling(SamplingRandom, rate)
}

// WithFilter adds a filter function
func WithFilter(filter FilterFunc) Option {
	return func(c *Config) error {
		if filter == nil {
			return NewFlexLogError(ErrCodeInvalidConfig, "config", "", nil).
				WithContext("error", "filter cannot be nil")
		}
		if c.Filters == nil {
			c.Filters = make([]FilterFunc, 0)
		}
		c.Filters = append(c.Filters, filter)
		return nil
	}
}

// WithLevelFilter creates a filter that only allows messages at or above the specified level
func WithLevelFilter(minLevel int) Option {
	return WithFilter(func(level int, message string, fields map[string]interface{}) bool {
		return level >= minLevel
	})
}

// WithErrorHandler sets the error handler
func WithErrorHandler(handler ErrorHandler) Option {
	return func(c *Config) error {
		if handler == nil {
			return NewFlexLogError(ErrCodeInvalidConfig, "config", "", nil).
				WithContext("error", "error handler cannot be nil")
		}
		c.ErrorHandler = handler
		return nil
	}
}

// WithMaxAge sets the maximum age for log files
func WithMaxAge(duration time.Duration) Option {
	return func(c *Config) error {
		if duration < 0 {
			return NewFlexLogError(ErrCodeInvalidConfig, "config", "", nil).
				WithContext("maxAge", duration).
				WithContext("error", "duration cannot be negative")
		}
		c.MaxAge = duration
		return nil
	}
}

// WithCleanupInterval sets the cleanup interval
func WithCleanupInterval(interval time.Duration) Option {
	return func(c *Config) error {
		if interval <= 0 {
			return NewFlexLogError(ErrCodeInvalidConfig, "config", "", nil).
				WithContext("cleanupInterval", interval).
				WithContext("error", "interval must be positive")
		}
		c.CleanupInterval = interval
		return nil
	}
}

// WithTimezone sets the timezone for timestamps
func WithTimezone(tz *time.Location) Option {
	return func(c *Config) error {
		if tz == nil {
			return NewFlexLogError(ErrCodeInvalidConfig, "config", "", nil).
				WithContext("error", "timezone cannot be nil")
		}
		c.FormatOptions.TimeZone = tz
		return nil
	}
}

// WithUTC sets UTC timezone
func WithUTC() Option {
	return WithTimezone(time.UTC)
}

// WithTimestampFormat sets the timestamp format
func WithTimestampFormat(format string) Option {
	return func(c *Config) error {
		if format == "" {
			return NewFlexLogError(ErrCodeInvalidConfig, "config", "", nil).
				WithContext("error", "timestamp format cannot be empty")
		}
		c.FormatOptions.TimestampFormat = format
		return nil
	}
}

// WithRecovery enables recovery with fallback
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

// WithRedaction enables sensitive data redaction
func WithRedaction(patterns []string, replacement string) Option {
	return func(c *Config) error {
		c.RedactionPatterns = patterns
		c.RedactionReplace = replacement
		return nil
	}
}

// WithBatchProcessing enables batch processing for writes
func WithBatchProcessing(maxSize, maxCount int, flushInterval time.Duration) Option {
	return func(c *Config) error {
		if maxSize <= 0 || maxCount <= 0 {
			return NewFlexLogError(ErrCodeInvalidConfig, "config", "", nil).
				WithContext("error", "batch size and count must be positive")
		}
		if flushInterval < 0 {
			return NewFlexLogError(ErrCodeInvalidConfig, "config", "", nil).
				WithContext("error", "flush interval cannot be negative")
		}
		c.EnableBatching = true
		c.BatchMaxSize = maxSize
		c.BatchMaxCount = maxCount
		c.BatchFlushInterval = flushInterval
		return nil
	}
}

// WithDefaultBatching enables batching with default settings
func WithDefaultBatching() Option {
	return WithBatchProcessing(64*1024, 100, 100*time.Millisecond) // 64KB, 100 entries, 100ms
}

// Preset configurations

// WithProductionDefaults sets recommended production settings
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

// WithDevelopmentDefaults sets recommended development settings
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

// Example usage:
//
// logger, err := flexlog.NewWithOptions(
//     flexlog.WithPath("/var/log/app.log"),
//     flexlog.WithLevel(flexlog.LevelInfo),
//     flexlog.WithJSON(),
//     flexlog.WithRotation(100*1024*1024, 10),
//     flexlog.WithGzipCompression(),
//     flexlog.WithUTC(),
//     flexlog.WithErrorHandler(flexlog.StderrErrorHandler),
// )