package flexlog

import (
	"time"
)

// Builder provides a fluent interface for constructing FlexLog instances
// with a clean, intuitive API that guides users through configuration.
type Builder struct {
	config Config
	destinations []destinationConfig
	err error
}

// destinationConfig holds destination configuration during building
type destinationConfig struct {
	uri         string
	backendType int
	options     []DestinationOption
}

// DestinationOption configures a destination
type DestinationOption func(*Destination) error

// NewBuilder creates a new FlexLog builder
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

// WithLevel sets the minimum log level
func (b *Builder) WithLevel(level int) *Builder {
	if b.err != nil {
		return b
	}
	if level < LevelTrace || level > LevelError {
		b.err = NewFlexLogError(ErrCodeInvalidLevel, "config", "", nil).
			WithContext("level", level)
		return b
	}
	b.config.Level = level
	return b
}

// WithFormat sets the output format
func (b *Builder) WithFormat(format int) *Builder {
	if b.err != nil {
		return b
	}
	if format != FormatText && format != FormatJSON {
		b.err = NewFlexLogError(ErrCodeInvalidFormat, "config", "", nil).
			WithContext("format", format)
		return b
	}
	b.config.Format = format
	return b
}

// WithDestination adds a file destination
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

// WithSyslogDestination adds a syslog destination
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

// WithBufferSize sets the channel buffer size
func (b *Builder) WithBufferSize(size int) *Builder {
	if b.err != nil {
		return b
	}
	if size <= 0 {
		b.err = NewFlexLogError(ErrCodeInvalidConfig, "config", "", nil).
			WithContext("bufferSize", size)
		return b
	}
	b.config.ChannelSize = size
	return b
}

// WithRotation configures log rotation
func (b *Builder) WithRotation(maxSize int64, maxFiles int) *Builder {
	if b.err != nil {
		return b
	}
	b.config.MaxSize = maxSize
	b.config.MaxFiles = maxFiles
	return b
}

// WithCompression enables compression
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

// WithStackTrace enables stack trace capture
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

// WithSampling configures log sampling
func (b *Builder) WithSampling(strategy int, rate float64) *Builder {
	if b.err != nil {
		return b
	}
	b.config.SamplingStrategy = strategy
	b.config.SamplingRate = rate
	return b
}

// WithFilter adds a filter function
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

// WithErrorHandler sets the error handler
func (b *Builder) WithErrorHandler(handler ErrorHandler) *Builder {
	if b.err != nil {
		return b
	}
	b.config.ErrorHandler = handler
	return b
}

// WithMaxAge sets the maximum age for log files
func (b *Builder) WithMaxAge(duration time.Duration) *Builder {
	if b.err != nil {
		return b
	}
	b.config.MaxAge = duration
	return b
}

// WithCleanupInterval sets the cleanup interval
func (b *Builder) WithCleanupInterval(interval time.Duration) *Builder {
	if b.err != nil {
		return b
	}
	b.config.CleanupInterval = interval
	return b
}

// WithTimezone sets the timezone for timestamps
func (b *Builder) WithTimezone(tz *time.Location) *Builder {
	if b.err != nil {
		return b
	}
	b.config.FormatOptions.TimeZone = tz
	return b
}

// WithTimestampFormat sets the timestamp format
func (b *Builder) WithTimestampFormat(format string) *Builder {
	if b.err != nil {
		return b
	}
	b.config.FormatOptions.TimestampFormat = format
	return b
}

// WithJSON is a convenience method to set JSON format
func (b *Builder) WithJSON() *Builder {
	return b.WithFormat(FormatJSON)
}

// WithDebugLevel is a convenience method to set debug level
func (b *Builder) WithDebugLevel() *Builder {
	return b.WithLevel(LevelDebug)
}

// Build creates the FlexLog instance
func (b *Builder) Build() (*FlexLog, error) {
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

// WithBatching configures batching for a destination
func WithBatching(maxSize int, interval time.Duration) DestinationOption {
	return func(d *Destination) error {
		d.batchEnabled = true
		d.batchMaxSize = maxSize
		d.flushInterval = interval
		return nil
	}
}

// WithDestinationName sets a custom name for the destination
func WithDestinationName(name string) DestinationOption {
	return func(d *Destination) error {
		d.Name = name
		return nil
	}
}

// Example usage:
//
// logger, err := flexlog.NewBuilder().
//     WithLevel(flexlog.LevelInfo).
//     WithJSON().
//     WithDestination("/var/log/app.log",
//         flexlog.WithBatching(8192, 100*time.Millisecond),
//         flexlog.WithDestinationName("primary")).
//     WithSyslogDestination("syslog://localhost:514",
//         flexlog.WithDestinationName("syslog")).
//     WithRotation(100*1024*1024, 10).
//     WithCompression(flexlog.CompressionGzip, 2).
//     WithErrorHandler(flexlog.StderrErrorHandler).
//     Build()