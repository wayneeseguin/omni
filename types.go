package flexlog

import (
	"bufio"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/gofrs/flock"
)

// Lock Ordering Hierarchy:
// To prevent deadlocks, always acquire locks in this order:
// 1. f.mu (FlexLog main mutex) - acquire first
// 2. dest.mu (Destination mutex) - acquire second
// 3. dest.Lock (File lock) - acquire last
// Never acquire a higher-level lock while holding a lower-level lock.

// ErrorLevel represents additional error severity levels beyond the standard log levels.
// These can be used to categorize errors by their impact on the system.
type ErrorLevel int

// CompressionType defines the compression algorithm used for rotated log files.
// Currently supports gzip compression or no compression.
type CompressionType int

// FilterFunc is a function that determines if a log entry should be logged.
// It receives the log level, message, and structured fields.
// Returns true if the message should be logged, false to filter it out.
type FilterFunc func(level int, message string, fields map[string]interface{}) bool

// SamplingStrategy defines how log sampling is performed to reduce log volume.
// Supports interval-based, random, and consistent hash-based sampling.
type SamplingStrategy int

// LogEntry represents a structured log entry with all associated metadata.
// This is the internal representation of a log message that can be formatted
// as JSON or text output.
type LogEntry struct {
	Fields     map[string]interface{} `json:"fields,omitempty"`
	File       string                 `json:"file,omitempty"`
	Level      string                 `json:"level"`
	Line       int                    `json:"line,omitempty"`
	Message    string                 `json:"message"`
	StackTrace string                 `json:"stack_trace,omitempty"`
	Timestamp  string                 `json:"timestamp"`
}

// LogFormat defines the format for log output.
// Supports JSON for structured logging and Text for human-readable output.
type LogFormat int

// FormatOption defines formatting options for log outputs
type FormatOption int

// LevelFormat defines level format options
type LevelFormat int

// LogDestination represents a destination for logs.
// Each destination can be independently enabled/disabled and has its own writer.
type LogDestination struct {
	// Writer is the io.Writer to write logs to
	Writer io.Writer

	// Name is a unique identifier for this destination
	Name string

	// Enabled determines if logs should be written to this destination
	Enabled bool
}

// LogMessage represents a message to be logged by a background worker.
// This is the internal message type passed through channels for async logging.
type LogMessage struct {
	Level     int
	Format    string
	Args      []interface{}
	Entry     *LogEntry
	Timestamp time.Time
	Raw       []byte
}

// Destination represents a log destination with its own worker goroutine.
// Each destination runs independently with its own configuration, formatting,
// and output handling. Supports file-based and syslog backends.
type Destination struct {
	URI        string   // URI for the destination (file path or syslog address)
	Name       string   // Unique identifier for this destination
	Backend    int      // Backend type (BackendFlock or BackendSyslog)
	File       *os.File // File handle (for file backend)
	Writer     *bufio.Writer
	Lock       *flock.Flock // Lock (only for flock backend)
	Size       int64
	Done       chan struct{}
	SyslogConn *syslogConn // Connection for syslog backend
	Enabled    bool        // Whether this destination is enabled
	mu         sync.Mutex  // Protects concurrent access to Writer

	// Batching configuration
	batchWriter   *BatchWriter  // Batch writer for efficient writes
	flushInterval time.Duration // How often to flush the buffer
	flushTimer    *time.Timer   // Timer for periodic flushing
	flushSize     int           // Flush when buffer reaches this size
	batchEnabled  bool          // Whether batching is enabled for this destination
	batchMaxSize  int           // Maximum batch size in bytes
	batchMaxCount int           // Maximum number of entries in a batch

	// Metrics
	bytesWritten uint64
	rotations    uint64
	errors       uint64
	writeCount   uint64
	totalLatency int64
	lastWrite    time.Time
}

// syslogConn represents a connection to a syslog server
type syslogConn struct {
	network  string // "tcp", "udp", or "unix"
	address  string // Address or socket path
	conn     net.Conn
	priority int    // Syslog priority
	tag      string // Syslog tag
}

// FormatOptions controls the output format
type FormatOptions struct {
	TimestampFormat string
	IncludeLevel    bool
	IncludeTime     bool
	LevelFormat     LevelFormat
	IndentJSON      bool
	FieldSeparator  string
	TimeZone        *time.Location
}

// FlexLog is the main logger struct that manages logging to multiple destinations.
// It provides a non-blocking, thread-safe logging interface with support for:
//   - Multiple concurrent output destinations
//   - Structured logging with key-value pairs
//   - Log rotation and compression
//   - Filtering and sampling
//   - Process-safe file locking
//   - Configurable formatting
//
// FlexLog uses a background worker pattern with channels to ensure logging
// doesn't block the main application flow.
type FlexLog struct {
	mu            sync.RWMutex
	file          *os.File
	writer        *bufio.Writer
	path          string
	maxSize       int64
	maxFiles      int
	currentSize   int64
	level         int
	fileLock      *flock.Flock
	includeTrace  bool
	stackSize     int
	captureAll    bool
	formatOptions FormatOptions

	// Compression
	compression     int
	compressMinAge  int
	compressWorkers int
	compressCh      chan string
	compressWg      sync.WaitGroup
	maxAge          time.Duration
	cleanupInterval time.Duration
	cleanupTicker   *time.Ticker
	cleanupDone     chan struct{}
	cleanupWg       sync.WaitGroup

	// Filtering
	filters []Filter

	// Sampling fields
	samplingStrategy int
	samplingRate     float64
	sampleCounter    uint64
	sampleKeyFunc    func(int, string, map[string]interface{}) string

	// Non-blocking logging fields
	msgChan     chan LogMessage
	defaultDest *Destination
	workerWg    sync.WaitGroup
	channelSize int
	size        int64 // alias for currentSize for backward compatibility

	// Multilog
	Destinations []*Destination
	messageQueue chan *LogMessage

	// Formatting
	format     int
	formatOpts FormatOptions

	// Error handling
	errorHandler   ErrorHandler
	errorChan      chan LogError
	errorCount     uint64
	errorsBySource sync.Map // map[string]uint64 - thread-safe map for error counts by source
	lastError      *LogError
	lastErrorTime  *time.Time

	// Metrics
	messagesByLevel  sync.Map // map[int]uint64 - thread-safe map for message counts by level
	messagesDropped  uint64
	rotationCount    uint64
	compressionCount uint64
	bytesWritten     uint64
	writeCount       uint64
	totalWriteTime   int64
	maxWriteTime     int64

	// Redaction
	redactor          *Redactor
	redactionPatterns []string
	redactionReplace  string

	// Performance
	lazyFormatting bool

	// Recovery
	recoveryManager *RecoveryManager

	closed        bool
	workerStarted bool // Track if message dispatcher was started
}
