package omni

const (
	// defaultMaxSize defines the default maximum size for a log file before rotation (100 MB).
	defaultMaxSize    = 100 * 1024 * 1024 // 100 MB
	// defaultMaxFiles defines the default maximum number of rotated log files to keep.
	defaultMaxFiles   = 10
	// defaultBufferSize defines the default size for internal buffers (32 KB).
	defaultBufferSize = 32 * 1024 // 32 KB

	// BlockingMode determines whether the logger blocks when channels are full.
	// When true, log operations will block until space is available.
	// TODO: Enable toggling this configuration flag to specify blocking or non-blocking behavior
	BlockingMode = true

	// LevelTrace represents the most verbose logging level for detailed debugging.
	// Use for tracing program execution and detailed diagnostics.
	LevelTrace = 0
	// LevelDebug represents debug-level logging for development and troubleshooting.
	// Use for information helpful during development but not needed in production.
	LevelDebug = 1
	// LevelInfo represents informational messages about normal program operation.
	// Use for general informational messages about application state and progress.
	LevelInfo  = 2
	// LevelWarn represents warning messages for potentially problematic situations.
	// Use for recoverable issues that might need attention.
	LevelWarn  = 3
	// LevelError represents error messages for serious problems.
	// Use for errors that prevent normal operation but don't crash the application.
	LevelError = 4

	// FormatText specifies plain text output format.
	// Messages are formatted as human-readable text with configurable field separators.
	FormatText   = 0
	// FormatJSON specifies JSON output format.
	// Messages are formatted as JSON objects with structured fields.
	FormatJSON   = 1
	// FormatCustom specifies a custom output format provided by a plugin.
	// Requires a custom formatter implementation to be registered.
	FormatCustom = 2

	// CompressionNone disables compression for rotated log files.
	CompressionNone = 0
	// CompressionGzip enables gzip compression for rotated log files.
	// Rotated files will be compressed with gzip to save disk space.
	CompressionGzip = 1

	// SamplingNone disables log sampling - all messages are logged.
	SamplingNone       = 0
	// SamplingRandom randomly samples log messages based on a probability.
	// Use to reduce log volume while maintaining statistical representation.
	SamplingRandom     = 1
	// SamplingHash samples based on a hash of the message content.
	// Ensures consistent sampling of identical messages.
	SamplingHash       = 2
	// SamplingConsistent uses consistent sampling based on message content.
	// Similar messages are consistently included or excluded from sampling.
	SamplingConsistent = 3

	// SamplingInterval samples every Nth message.
	// For example, with N=5, logs messages 1, 6, 11, 16, etc.
	SamplingInterval = 5

	// SamplingAdaptive adjusts sampling rate dynamically based on volume.
	// Increases sampling when volume is low, decreases when volume is high.
	SamplingAdaptive = 6

	// BackendFlock specifies the default file backend with Unix file locking.
	// Provides process-safe file logging using flock for synchronization.
	BackendFlock  = 0
	// BackendSyslog specifies the syslog backend for local or remote syslog.
	// Integrates with system logging infrastructure.
	BackendSyslog = 1

	// SeverityLow represents minor errors that don't significantly impact operation.
	// Use for errors that are automatically recoverable or have minimal impact.
	SeverityLow ErrorLevel = iota
	// SeverityMedium represents important errors that may degrade functionality.
	// Use for errors that require attention but don't prevent core operation.
	SeverityMedium
	// SeverityHigh represents critical errors that significantly impact operation.
	// Use for errors that prevent important features from working correctly.
	SeverityHigh
	// SeverityCritical represents fatal errors that require immediate attention.
	// Use for errors that may cause data loss or system failure.
	SeverityCritical
	
	// FormatOptionTimestampFormat controls the timestamp format in log messages.
	// Accepts time format strings like "2006-01-02 15:04:05" or "RFC3339".
	FormatOptionTimestampFormat FormatOption = iota
	// FormatOptionIncludeLevel controls whether to include the log level in output.
	// When enabled, prepends messages with level indicators like [INFO] or [ERROR].
	FormatOptionIncludeLevel
	// FormatOptionLevelFormat controls how the log level is formatted.
	// See LevelFormat constants for available formatting options.
	FormatOptionLevelFormat
	// FormatOptionIncludeLocation controls whether to include source file location.
	// When enabled, adds file:line information to log messages.
	FormatOptionIncludeLocation
	// FormatOptionIndentJSON controls JSON output indentation.
	// When enabled, formats JSON with proper indentation for readability.
	FormatOptionIndentJSON
	// FormatOptionFieldSeparator controls the field separator in text format.
	// Default is space, but can be customized to tab, comma, etc.
	FormatOptionFieldSeparator
	// FormatOptionTimeZone controls the timezone for timestamps.
	// Can be set to specific timezones like "UTC" or "America/New_York".
	FormatOptionTimeZone
	// FormatOptionIncludeTime controls whether to include timestamps in output.
	// When disabled, messages are logged without time information.
	FormatOptionIncludeTime
	
	// LevelFormatName formats the level as its name (e.g., "INFO", "ERROR").
	// This is the default format for log levels.
	LevelFormatName LevelFormat = iota
	// LevelFormatNameUpper formats the level as uppercase (e.g., "INFO", "ERROR").
	// Ensures consistent uppercase formatting regardless of configuration.
	LevelFormatNameUpper
	// LevelFormatNameLower formats the level as lowercase (e.g., "info", "error").
	// Useful for systems that prefer lowercase identifiers.
	LevelFormatNameLower
	// LevelFormatSymbol formats the level as a single character (e.g., "I", "E").
	// Provides compact output for space-constrained environments.
	LevelFormatSymbol
)
