package omni

import (
	"time"
)

// Builder provides a fluent interface for constructing Omni instances
// with a clean, intuitive API that guides users through configuration.
// The builder pattern allows for readable, chainable configuration and
// performs validation as options are set.
type Builder struct {
	config Config
	destinations []destinationConfig
	err error
}

// destinationConfig holds destination configuration during building.
// This internal type stores destination settings until the logger is built.
type destinationConfig struct {
	uri         string
	backendType int
	options     []DestinationOption
}

// DestinationOption configures a destination.
// These options are applied to individual destinations after they are created.
type DestinationOption func(*Destination) error

// NewBuilder creates a new Omni builder.
// The builder starts with sensible defaults that can be customized
// through the fluent interface.
//
// Returns:
//   - *Builder: A new builder instance with default configuration
//
// Example:
//
//	logger, err := omni.NewBuilder().
//	    WithLevel(omni.LevelInfo).
//	    WithJSON().
//	    WithDestination("/var/log/app.log").
//	    Build()
func NewBuilder() *Builder {
	return &Builder{
		config: Config{
			// Set sensible defaults
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
		},
		destinations: make([]destinationConfig, 0),
	}
}

// WithLevel sets the minimum log level.
// Messages below this level will not be logged.
//
// Parameters:
//   - level: The minimum log level (LevelTrace to LevelError)
//
// Returns:
//   - *Builder: The builder for method chaining
func (b *Builder) WithLevel(level int) *Builder {
	if b.err != nil {
		return b
	}
	if level < LevelTrace || level > LevelError {
		b.err = NewOmniError(ErrCodeInvalidLevel, "config", "", nil).
			WithContext("level", level)
		return b
	}
	b.config.Level = level
	return b
}

// WithFormat sets the output format.
// Supported formats are FormatText and FormatJSON.
//
// Parameters:
//   - format: The output format constant
//
// Returns:
//   - *Builder: The builder for method chaining
func (b *Builder) WithFormat(format int) *Builder {
	if b.err != nil {
		return b
	}
	if format != FormatText && format != FormatJSON {
		b.err = NewOmniError(ErrCodeInvalidFormat, "config", "", nil).
			WithContext("format", format)
		return b
	}
	b.config.Format = format
	return b
}

// WithDestination adds a file destination.
// The first destination becomes the primary log path.
// Additional destinations can be added for multi-destination logging.
//
// Parameters:
//   - path: The file path for the destination
//   - options: Optional destination-specific configuration
//
// Returns:
//   - *Builder: The builder for method chaining
func (b *Builder) WithDestination(path string, options ...DestinationOption) *Builder {
	if b.err != nil {
		return b
	}
	b.destinations = append(b.destinations, destinationConfig{
		uri:         path,
		backendType: BackendFlock,
		options:     options,
	})
	// Also set as primary if it's the first destination
	if len(b.destinations) == 1 {
		b.config.Path = path
	}
	return b
}

// WithSyslogDestination adds a syslog destination.
// Supports standard syslog URIs like "syslog://localhost:514".
//
// Parameters:
//   - uri: The syslog URI
//   - options: Optional destination-specific configuration
//
// Returns:
//   - *Builder: The builder for method chaining
func (b *Builder) WithSyslogDestination(uri string, options ...DestinationOption) *Builder {
	if b.err != nil {
		return b
	}
	b.destinations = append(b.destinations, destinationConfig{
		uri:         uri,
		backendType: BackendSyslog,
		options:     options,
	})
	return b
}

// WithBufferSize sets the channel buffer size.
// A larger buffer can handle burst traffic better but uses more memory.
//
// Parameters:
//   - size: The buffer size (must be positive)
//
// Returns:
//   - *Builder: The builder for method chaining
func (b *Builder) WithBufferSize(size int) *Builder {
	if b.err != nil {
		return b
	}
	if size <= 0 {
		b.err = NewOmniError(ErrCodeInvalidConfig, "config", "", nil).
			WithContext("bufferSize", size)
		return b
	}
	b.config.ChannelSize = size
	return b
}

// WithRotation configures log rotation.
// Files are rotated when they reach maxSize bytes.
//
// Parameters:
//   - maxSize: Maximum file size in bytes before rotation
//   - maxFiles: Maximum number of rotated files to keep
//
// Returns:
//   - *Builder: The builder for method chaining
func (b *Builder) WithRotation(maxSize int64, maxFiles int) *Builder {
	if b.err != nil {
		return b
	}
	b.config.MaxSize = maxSize
	b.config.MaxFiles = maxFiles
	return b
}

// WithCompression enables compression.
// Rotated log files will be compressed to save disk space.
//
// Parameters:
//   - compressionType: Type of compression (e.g., CompressionGzip)
//   - workers: Number of compression workers
//
// Returns:
//   - *Builder: The builder for method chaining
func (b *Builder) WithCompression(compressionType int, workers int) *Builder {
	if b.err != nil {
		return b
	}
	b.config.Compression = compressionType
	if workers > 0 {
		b.config.CompressWorkers = workers
	}
	return b
}

// WithStackTrace enables stack trace capture.
// When enabled, error logs will include stack trace information.
//
// Parameters:
//   - capture: Whether to enable stack traces
//   - size: Stack buffer size in bytes
//
// Returns:
//   - *Builder: The builder for method chaining
func (b *Builder) WithStackTrace(capture bool, size int) *Builder {
	if b.err != nil {
		return b
	}
	b.config.IncludeTrace = capture
	if size > 0 {
		b.config.StackSize = size
	}
	return b
}

// WithSampling configures log sampling.
// Sampling reduces log volume by only logging a percentage of messages.
//
// Parameters:
//   - strategy: Sampling strategy (e.g., SamplingRandom)
//   - rate: Sampling rate from 0.0 to 1.0
//
// Returns:
//   - *Builder: The builder for method chaining
func (b *Builder) WithSampling(strategy int, rate float64) *Builder {
	if b.err != nil {
		return b
	}
	b.config.SamplingStrategy = strategy
	b.config.SamplingRate = rate
	return b
}

// WithFilter adds a filter function.
// Filters determine whether a message should be logged.
// Multiple filters can be added and all must pass.
//
// Parameters:
//   - filter: The filter function
//
// Returns:
//   - *Builder: The builder for method chaining
func (b *Builder) WithFilter(filter FilterFunc) *Builder {
	if b.err != nil {
		return b
	}
	if b.config.Filters == nil {
		b.config.Filters = make([]FilterFunc, 0)
	}
	b.config.Filters = append(b.config.Filters, filter)
	return b
}

// WithErrorHandler sets the error handler.
// The error handler is called when logging operations fail.
//
// Parameters:
//   - handler: The error handler function
//
// Returns:
//   - *Builder: The builder for method chaining
func (b *Builder) WithErrorHandler(handler ErrorHandler) *Builder {
	if b.err != nil {
		return b
	}
	b.config.ErrorHandler = handler
	return b
}

// WithMaxAge sets the maximum age for log files.
// Files older than this duration will be automatically deleted.
//
// Parameters:
//   - duration: Maximum age for log files
//
// Returns:
//   - *Builder: The builder for method chaining
func (b *Builder) WithMaxAge(duration time.Duration) *Builder {
	if b.err != nil {
		return b
	}
	b.config.MaxAge = duration
	return b
}

// WithCleanupInterval sets the cleanup interval.
// This controls how often old log files are checked and removed.
//
// Parameters:
//   - interval: Cleanup check interval
//
// Returns:
//   - *Builder: The builder for method chaining
func (b *Builder) WithCleanupInterval(interval time.Duration) *Builder {
	if b.err != nil {
		return b
	}
	b.config.CleanupInterval = interval
	return b
}

// WithTimezone sets the timezone for timestamps.
// By default, local time is used.
//
// Parameters:
//   - tz: The timezone location
//
// Returns:
//   - *Builder: The builder for method chaining
func (b *Builder) WithTimezone(tz *time.Location) *Builder {
	if b.err != nil {
		return b
	}
	b.config.FormatOptions.TimeZone = tz
	return b
}

// WithTimestampFormat sets the timestamp format.
// Uses Go's time format syntax.
//
// Parameters:
//   - format: Time format string
//
// Returns:
//   - *Builder: The builder for method chaining
func (b *Builder) WithTimestampFormat(format string) *Builder {
	if b.err != nil {
		return b
	}
	b.config.FormatOptions.TimestampFormat = format
	return b
}

// WithJSON is a convenience method to set JSON format.
// Equivalent to WithFormat(FormatJSON).
//
// Returns:
//   - *Builder: The builder for method chaining
func (b *Builder) WithJSON() *Builder {
	return b.WithFormat(FormatJSON)
}

// WithDebugLevel is a convenience method to set debug level.
// Equivalent to WithLevel(LevelDebug).
//
// Returns:
//   - *Builder: The builder for method chaining
func (b *Builder) WithDebugLevel() *Builder {
	return b.WithLevel(LevelDebug)
}

// Build creates the Omni instance.
// This finalizes the configuration and creates the logger.
// Any errors encountered during building will be returned here.
//
// Returns:
//   - *Omni: The configured logger instance
//   - error: Any configuration or initialization error
//
// Example:
//
//	logger, err := builder.Build()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer logger.Close()
func (b *Builder) Build() (*Omni, error) {
	// Check for errors during building
	if b.err != nil {
		return nil, b.err
	}

	// Create the logger
	logger, err := NewWithConfig(&b.config)
	if err != nil {
		return nil, err
	}

	// Add additional destinations
	for i, destConfig := range b.destinations {
		// Skip the first one if it's already set as primary
		if i == 0 && destConfig.uri == b.config.Path {
			// Apply destination options to the default destination
			if logger.defaultDest != nil && len(destConfig.options) > 0 {
				for _, opt := range destConfig.options {
					if err := opt(logger.defaultDest); err != nil {
						logger.Close()
						return nil, err
					}
				}
			}
			continue
		}

		// Add the destination
		err := logger.AddDestinationWithBackend(destConfig.uri, destConfig.backendType)
		if err != nil {
			logger.Close()
			return nil, err
		}

		// Apply destination options
		if len(destConfig.options) > 0 && len(logger.Destinations) > 0 {
			dest := logger.Destinations[len(logger.Destinations)-1]
			for _, opt := range destConfig.options {
				if err := opt(dest); err != nil {
					logger.Close()
					return nil, err
				}
			}
		}
	}

	return logger, nil
}

// Destination options

// WithBatching configures batching for a destination.
// Messages are batched to improve write performance.
//
// Parameters:
//   - maxSize: Maximum batch size in bytes
//   - interval: Maximum time before flushing a batch
//
// Returns:
//   - DestinationOption: The option function
func WithBatching(maxSize int, interval time.Duration) DestinationOption {
	return func(d *Destination) error {
		d.batchEnabled = true
		d.batchMaxSize = maxSize
		d.flushInterval = interval
		return nil
	}
}

// WithDestinationName sets a custom name for the destination.
// This name is used in metrics and error messages.
//
// Parameters:
//   - name: The destination name
//
// Returns:
//   - DestinationOption: The option function
func WithDestinationName(name string) DestinationOption {
	return func(d *Destination) error {
		d.Name = name
		return nil
	}
}

