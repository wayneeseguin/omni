package flexlog

const (
	defaultMaxSize    = 10 * 1024 * 1024 // 10MB
	defaultMaxFiles   = 5
	defaultBufferSize = 4096

	// Log levels
	LevelDebug = iota
	LevelInfo
	LevelWarn
	LevelError
)

const (
	// SeverityLow for minor errors
	SeverityLow ErrorLevel = iota
	// SeverityMedium for important errors
	SeverityMedium
	// SeverityHigh for critical errors
	SeverityHigh
	// SeverityCritical for fatal errors
	SeverityCritical
)

const (
	// CompressionNone means no compression
	CompressionNone CompressionType = iota
	// CompressionGzip uses gzip compression
	CompressionGzip
	// Future compression types can be added here
)

const (
	// SamplingNone disables sampling
	SamplingNone SamplingStrategy = iota
	// SamplingRandom randomly samples logs
	SamplingRandom
	// SamplingConsistent uses consistent sampling based on message content
	SamplingConsistent
	// SamplingInterval samples every Nth message
	SamplingInterval
)

const (
	// FormatText outputs logs as plain text (default)
	FormatText LogFormat = iota
	// FormatJSON outputs logs as JSON objects
	FormatJSON
)

const (
	// FormatOptionTimestampFormat controls timestamp format
	FormatOptionTimestampFormat FormatOption = iota
	// FormatOptionIncludeLevel controls whether to include level prefix
	FormatOptionIncludeLevel
	// FormatOptionLevelFormat controls how level is formatted
	FormatOptionLevelFormat
	// FormatOptionIncludeLocation controls whether to include file/line
	FormatOptionIncludeLocation
	// FormatOptionIndentJSON controls JSON indentation
	FormatOptionIndentJSON
	// FormatOptionFieldSeparator controls field separator in text mode
	FormatOptionFieldSeparator
	// FormatOptionTimeZone controls timezone for timestamps
	FormatOptionTimeZone
)

const (
	// LevelFormatName shows level as name (e.g., "INFO")
	LevelFormatName LevelFormat = iota
	// LevelFormatNameUpper shows level as uppercase name (e.g., "INFO")
	LevelFormatNameUpper
	// LevelFormatNameLower shows level as lowercase name (e.g., "info")
	LevelFormatNameLower
	// LevelFormatSymbol shows level as symbol (e.g., "I" for INFO)
	LevelFormatSymbol
)
