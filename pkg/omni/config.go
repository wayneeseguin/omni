package omni

import (
	"time"
)

// Config contains all configuration options for Omni.
// This struct provides a comprehensive way to configure all aspects of the logger,
// from basic settings like log level and format to advanced features like compression,
// sampling, and batch processing.
type Config struct {
	// Core settings
	Path          string        // Primary log file path
	Level         int           // Minimum log level
	Format        int           // Output format (text/json)
	FormatOptions FormatOptions // Format-specific options
	ChannelSize   int           // Message channel buffer size

	// Rotation settings
	MaxSize         int64         // Maximum file size before rotation
	MaxFiles        int           // Maximum number of rotated files to keep
	MaxAge          time.Duration // Maximum age of log files
	CleanupInterval time.Duration // Interval for age-based cleanup

	// Compression settings
	Compression     int // Compression type (none/gzip)
	CompressMinAge  int // Minimum rotations before compression
	CompressWorkers int // Number of compression workers

	// Error handling
	ErrorHandler ErrorHandler // Custom error handler

	// Advanced settings
	IncludeTrace bool // Include stack traces on errors
	StackSize    int  // Stack trace buffer size
	CaptureAll   bool // Capture full stack on errors

	// Filtering settings
	Filters []FilterFunc // Filter functions
	
	// Sampling settings
	SamplingStrategy int                                              // Sampling strategy
	SamplingRate     float64                                          // Sampling rate (0.0-1.0)
	SampleKeyFunc    func(int, string, map[string]interface{}) string // Function to generate sampling keys

	// Redaction settings
	RedactionPatterns []string // Patterns to redact
	RedactionReplace  string   // Replacement string for redacted content

	// Plugin settings
	CustomFormatter       string                 // Custom formatter plugin name
	CustomFormatterConfig map[string]interface{} // Custom formatter configuration

	// Performance settings
	EnableBufferPool bool // Enable buffer pooling for formatting
	EnableLazyFormat bool // Enable lazy formatting

	// Batch processing settings
	EnableBatching     bool          // Enable batch processing for writes
	BatchMaxSize       int           // Maximum batch size in bytes (default: 64KB)
	BatchMaxCount      int           // Maximum number of entries in a batch (default: 100)
	BatchFlushInterval time.Duration // How often to flush batches (default: 100ms)
	
	// Recovery settings
	Recovery *RecoveryConfig // Recovery configuration
	
	// Backend factory for creating custom destinations
	BackendFactory BackendFactory // Custom backend factory
	
	// Formatter for custom log formatting
	Formatter Formatter // Custom formatter
}


// DefaultConfig returns a Config with sensible defaults.
// These defaults are designed to work well for most applications:
// - Info level logging
// - Text format output
// - No compression or rotation by default
// - Reasonable buffer sizes
//
// Returns:
//   - *Config: A configuration with default values
//
// Example:
//
//	config := omni.DefaultConfig()
//	config.Path = "/var/log/app.log"
//	config.Level = omni.LevelDebug
//	logger, err := omni.NewWithConfig(config)
func DefaultConfig() *Config {
	return &Config{
		Path:               "",
		Level:              LevelInfo,
		Format:             FormatText,
		FormatOptions:      DefaultFormatOptions(),
		ChannelSize:        getDefaultChannelSize(),
		MaxSize:            defaultMaxSize,
		MaxFiles:           defaultMaxFiles,
		MaxAge:             0,
		CleanupInterval:    1 * time.Hour,
		Compression:        CompressionNone,
		CompressMinAge:     1,
		CompressWorkers:    1,
		ErrorHandler:       getDefaultErrorHandler(),
		IncludeTrace:       false,
		StackSize:          4096,
		CaptureAll:         false,
		SamplingStrategy:   SamplingNone,
		SamplingRate:       1.0,
		SampleKeyFunc:      DefaultSampleKeyFunc,
		RedactionPatterns:  nil,
		RedactionReplace:   "[REDACTED]",
		EnableBufferPool:   false,
		EnableLazyFormat:   false,
		EnableBatching:     false,
		BatchMaxSize:       64 * 1024, // 64KB
		BatchMaxCount:      100,
		BatchFlushInterval: 100 * time.Millisecond,
	}
}

// Validate checks if the configuration is valid.
// This method ensures all configuration values are within acceptable ranges
// and applies defaults where necessary. It's called automatically by NewWithConfig.
//
// Returns:
//   - error: Validation error if configuration is invalid
//
// The following validations are performed:
// - ChannelSize > 0
// - MaxSize >= 0
// - MaxFiles >= 0
// - CleanupInterval >= 1 minute
// - CompressWorkers > 0
// - StackSize > 0
// - SamplingRate between 0.0 and 1.0
func (c *Config) Validate() error {
	if c.ChannelSize <= 0 {
		c.ChannelSize = getDefaultChannelSize()
	}

	if c.MaxSize < 0 {
		c.MaxSize = defaultMaxSize
	}

	if c.MaxFiles < 0 {
		c.MaxFiles = 0
	}

	if c.CleanupInterval < time.Minute {
		c.CleanupInterval = time.Minute
	}

	if c.CompressWorkers <= 0 {
		c.CompressWorkers = 1
	}

	if c.StackSize <= 0 {
		c.StackSize = 4096
	}

	if c.SamplingRate < 0 || c.SamplingRate > 1 {
		c.SamplingRate = 1.0
	}

	if c.ErrorHandler == nil {
		c.ErrorHandler = getDefaultErrorHandler()
	}

	if c.FormatOptions.TimestampFormat == "" {
		c.FormatOptions = DefaultFormatOptions()
	}

	if c.SampleKeyFunc == nil {
		c.SampleKeyFunc = DefaultSampleKeyFunc
	}

	return nil
}

// NewWithConfig creates a new logger with the given configuration.
// This is the recommended way to create a logger with custom settings.
// The configuration is validated and defaults are applied where necessary.
//
// Parameters:
//   - config: The configuration to use
//
// Returns:
//   - *Omni: The configured logger instance
//   - error: Configuration or initialization error
//
// Example:
//
//	config := &omni.Config{
//	    Path:     "/var/log/app.log",
//	    Level:    omni.LevelDebug,
//	    MaxSize:  100 * 1024 * 1024, // 100MB
//	    MaxFiles: 5,
//	    Compression: omni.CompressionGzip,
//	    EnableBatching: true,
//	}
//	logger, err := omni.NewWithConfig(config)
func NewWithConfig(config *Config) (*Omni, error) {
	// Validate and apply defaults
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Create logger instance
	f := &Omni{
		maxSize:          config.MaxSize,
		maxFiles:         config.MaxFiles,
		level:            config.Level,
		format:           config.Format,
		includeTrace:     config.IncludeTrace,
		stackSize:        config.StackSize,
		captureAll:       config.CaptureAll,
		formatOptions:    config.FormatOptions,
		compression:      config.Compression,
		compressMinAge:   config.CompressMinAge,
		compressWorkers:  config.CompressWorkers,
		compressCh:       nil,
		maxAge:           config.MaxAge,
		cleanupInterval:  config.CleanupInterval,
		cleanupTicker:    nil,
		cleanupDone:      nil,
		filters:          nil,
		samplingStrategy: config.SamplingStrategy,
		samplingRate:     config.SamplingRate,
		sampleCounter:    0,
		sampleKeyFunc:    config.SampleKeyFunc,
		msgChan:          make(chan LogMessage, config.ChannelSize),
		channelSize:      config.ChannelSize,
		Destinations:     make([]*Destination, 0),
		messageQueue:     make(chan *LogMessage, config.ChannelSize),
		// errorHandler will be set below
		// messagesByLevel and errorsBySource are sync.Map, no initialization needed
	}

	// Message level counters will be lazily initialized on first use

	// Set error handler if provided
	if config.ErrorHandler != nil {
		f.SetErrorHandler(config.ErrorHandler)
	}

	// Set backend factory if provided
	if config.BackendFactory != nil {
		f.backendFactory = config.BackendFactory
	}
	
	// Set formatter if provided
	if config.Formatter != nil {
		f.formatter = config.Formatter
	}

	// Add primary destination if path provided
	if config.Path != "" {
		dest, err := f.createDestination(config.Path, BackendFlock)
		if err != nil {
			return nil, err
		}

		f.defaultDest = dest
		f.Destinations = append(f.Destinations, dest)

		// Set backward compatibility fields
		f.file = dest.File
		f.writer = dest.Writer
		f.path = config.Path
		f.fileLock = dest.Lock
		f.currentSize = dest.Size
		f.size = dest.Size
	}

	// Apply redaction patterns if provided or enable default built-in redaction
	if config.RedactionPatterns != nil {
		_ = f.SetRedaction(config.RedactionPatterns, config.RedactionReplace)
	} else {
		// Enable built-in redaction by default with empty patterns
		_ = f.SetRedaction([]string{}, config.RedactionReplace)
	}

	// Apply performance settings
	if config.EnableLazyFormat {
		f.EnableLazyFormatting()
	}

	// Apply batching settings
	if config.EnableBatching {
		// Enable batching for all destinations
		for i := range f.Destinations {
			dest := f.Destinations[i]
			dest.mu.Lock()
			dest.batchEnabled = true
			dest.batchMaxSize = config.BatchMaxSize
			dest.batchMaxCount = config.BatchMaxCount
			if dest.Writer != nil {
				dest.batchWriter = NewBatchWriter(
					dest.Writer,
					config.BatchMaxSize,
					config.BatchMaxCount,
					config.BatchFlushInterval,
				)
			}
			dest.mu.Unlock()
		}
	}

	// Start message dispatcher
	f.workerWg.Add(1)
	f.workerStarted = true
	go f.messageDispatcher()

	// Start compression workers if enabled
	if config.Compression != CompressionNone {
		f.startCompressionWorkers()
	}

	// Initialize rotation manager with maxFiles if set
	if config.MaxFiles > 0 {
		f.SetMaxFiles(config.MaxFiles)
	}
	
	// Start cleanup routine if max age is set
	if config.MaxAge > 0 {
		f.startCleanupRoutine()
	}

	return f, nil
}

// GetConfig returns the current configuration of the logger.
// This creates a snapshot of the current configuration settings.
// Note that some settings may have been modified at runtime.
//
// Returns:
//   - *Config: A copy of the current configuration
//
// Example:
//
//	config := logger.GetConfig()
//	fmt.Printf("Current log level: %d\n", config.Level)
//	fmt.Printf("Compression enabled: %v\n", config.Compression != omni.CompressionNone)
func (f *Omni) GetConfig() *Config {
	f.mu.RLock()
	defer f.mu.RUnlock()

	config := &Config{
		Path:             f.path,
		Level:            f.level,
		Format:           f.format,
		FormatOptions:    f.formatOptions,
		ChannelSize:      f.channelSize,
		MaxSize:          f.maxSize,
		MaxFiles:         f.maxFiles,
		MaxAge:           f.maxAge,
		CleanupInterval:  f.cleanupInterval,
		Compression:      f.compression,
		CompressMinAge:   f.compressMinAge,
		CompressWorkers:  f.compressWorkers,
		// ErrorHandler cannot be easily converted back
		IncludeTrace:     f.includeTrace,
		StackSize:        f.stackSize,
		CaptureAll:       f.captureAll,
		SamplingStrategy: f.samplingStrategy,
		SamplingRate:     f.samplingRate,
		SampleKeyFunc:    f.sampleKeyFunc,
	}

	// Get redaction patterns if set
	if f.redactor != nil {
		config.RedactionPatterns = f.redactionPatterns
		config.RedactionReplace = f.redactionReplace
	}

	return config
}

// UpdateConfig updates the logger configuration.
// This allows runtime changes to most configuration settings.
// Note: Some settings like ChannelSize cannot be changed after creation.
//
// Parameters:
//   - config: The new configuration to apply
//
// Returns:
//   - error: Validation error if configuration is invalid
//
// Changeable settings:
// - Level, Format, FormatOptions
// - MaxSize, MaxFiles, MaxAge
// - Compression settings
// - Sampling settings
// - Error handler
// - Stack trace settings
// - Redaction patterns
//
// Example:
//
//	config := logger.GetConfig()
//	config.Level = omni.LevelDebug  // Enable debug logging
//	config.Compression = omni.CompressionGzip  // Enable compression
//	err := logger.UpdateConfig(config)
func (f *Omni) UpdateConfig(config *Config) error {
	if err := config.Validate(); err != nil {
		return err
	}

	f.mu.Lock()
	// Update settings that can be changed at runtime
	f.level = config.Level
	f.format = config.Format
	f.formatOptions = config.FormatOptions
	f.maxSize = config.MaxSize
	f.maxFiles = config.MaxFiles
	f.includeTrace = config.IncludeTrace
	f.stackSize = config.StackSize
	f.captureAll = config.CaptureAll
	f.samplingStrategy = config.SamplingStrategy
	f.samplingRate = config.SamplingRate
	f.sampleKeyFunc = config.SampleKeyFunc
	// Store error handler for later setting
	errorHandler := config.ErrorHandler
	f.mu.Unlock()
	
	// Set error handler after releasing the lock
	if errorHandler != nil {
		f.SetErrorHandler(errorHandler)
	}
	
	f.mu.Lock()
	defer f.mu.Unlock()

	// Update compression settings
	if f.compression != config.Compression {
		if f.compression == CompressionNone && config.Compression != CompressionNone {
			// Starting compression
			f.compression = config.Compression
			f.compressMinAge = config.CompressMinAge
			f.compressWorkers = config.CompressWorkers
			f.startCompressionWorkers()
		} else if f.compression != CompressionNone && config.Compression == CompressionNone {
			// Stopping compression
			f.stopCompressionWorkers()
			f.compression = CompressionNone
		} else {
			// Changing compression type
			f.compression = config.Compression
			f.compressMinAge = config.CompressMinAge
		}
	}

	// Update max age and cleanup settings
	if f.maxAge != config.MaxAge || f.cleanupInterval != config.CleanupInterval {
		oldMaxAge := f.maxAge
		f.maxAge = config.MaxAge
		f.cleanupInterval = config.CleanupInterval

		if oldMaxAge == 0 && config.MaxAge > 0 {
			// Start cleanup routine
			f.startCleanupRoutine()
		} else if oldMaxAge > 0 && config.MaxAge == 0 {
			// Stop cleanup routine
			f.stopCleanupRoutine()
		} else if config.MaxAge > 0 && f.cleanupInterval != config.CleanupInterval {
			// Restart with new interval
			f.stopCleanupRoutine()
			f.startCleanupRoutine()
		}
	}

	// Update redaction patterns
	if len(config.RedactionPatterns) > 0 {
		_ = f.SetRedaction(config.RedactionPatterns, config.RedactionReplace)
	} else if f.redactor != nil {
		// Clear redaction
		f.redactor = nil
		f.redactionPatterns = nil
		f.redactionReplace = ""
	}

	return nil
}
