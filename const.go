package flexlog

const (
	defaultMaxSize    = 100 * 1024 * 1024 // 100 MB
	defaultMaxFiles   = 10
	defaultBufferSize = 32 * 1024 // 32 KB

	BlockingMode = true // TODO: Enable toggling this configuration flag to specify blocking or non-blocking behavior

	// Log levels
	LevelTrace = 0
	LevelDebug = 1
	LevelInfo  = 2
	LevelWarn  = 3
	LevelError = 4

	// Format types
	FormatText   = 0
	FormatJSON   = 1
	FormatCustom = 2 // Custom formatter via plugin

	// Compression types
	CompressionNone = 0
	CompressionGzip = 1

	// Sampling strategies
	SamplingNone       = 0
	SamplingRandom     = 1
	SamplingHash       = 2
	SamplingConsistent = 3 // SamplingConsistent uses consistent sampling based on message content

	// SamplingInterval samples every Nth message
	SamplingInterval = 5

	// SamplingAdaptive adjusts sampling rate dynamically based on volume
	SamplingAdaptive = 6

	// Backend types
	BackendFlock  = 0 // Default file backend with flock
	BackendSyslog = 1 // Syslog backend (local or remote)

	// SeverityLow for minor errors
	SeverityLow ErrorLevel = iota
	// SeverityMedium for important errors
	SeverityMedium
	// SeverityHigh for critical errors
	SeverityHigh
	// SeverityCritical for fatal errors
	SeverityCritical
	//
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
	// FormatOptionIncludeTime controls whether to include timestamp
	FormatOptionIncludeTime
	//
	// LevelFormatName shows level as name (e.g., "INFO")
	LevelFormatName LevelFormat = iota
	// LevelFormatNameUpper shows level as uppercase name (e.g., "INFO")
	LevelFormatNameUpper
	// LevelFormatNameLower shows level as lowercase name (e.g., "info")
	LevelFormatNameLower
	// LevelFormatSymbol shows level as symbol (e.g., "I" for INFO)
	LevelFormatSymbol
)
