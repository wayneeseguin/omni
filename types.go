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

// ErrorLevel represents additional error severity levels
type ErrorLevel int

// CompressionType defines the compression algorithm used
type CompressionType int

// FilterFunc is a function that determines if a log entry should be logged
type FilterFunc func(level int, message string, fields map[string]interface{}) bool

// SamplingStrategy defines how log sampling is performed
type SamplingStrategy int

// LogEntry represents a structured log entry
type LogEntry struct {
	Fields     map[string]interface{} `json:"fields,omitempty"`
	File       string                 `json:"file,omitempty"`
	Level      string                 `json:"level"`
	Line       int                    `json:"line,omitempty"`
	Message    string                 `json:"message"`
	StackTrace string                 `json:"stack_trace,omitempty"`
	Timestamp  string                 `json:"timestamp"`
}

// LogFormat defines the format for log output
type LogFormat int

// FormatOption defines formatting options for log outputs
type FormatOption int

// LevelFormat defines level format options
type LevelFormat int

// LogDestination represents a destination for logs
type LogDestination struct {
	// Writer is the io.Writer to write logs to
	Writer io.Writer

	// Name is a unique identifier for this destination
	Name string

	// Enabled determines if logs should be written to this destination
	Enabled bool
}

// LogMessage represents a message to be logged by a background worker
type LogMessage struct {
	Level     int
	Format    string
	Args      []interface{}
	Entry     *LogEntry
	Timestamp time.Time
	Raw       []byte
}

// Destination represents a log destination with its own worker goroutine
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

// FlexLog is the main logger struct
type FlexLog struct {
	mu              sync.Mutex
	file            *os.File
	writer          *bufio.Writer
	path            string
	maxSize         int64
	maxFiles        int
	currentSize     int64
	level           int
	fileLock        *flock.Flock
	format          int
	includeTrace    bool
	stackSize       int
	captureAll      bool
	formatOptions   FormatOptions
	compression     int
	compressMinAge  int
	compressWorkers int
	compressCh      chan string
	maxAge          time.Duration
	cleanupInterval time.Duration
	cleanupTicker   *time.Ticker
	cleanupDone     chan struct{}
	filters         []Filter

	// Sampling fields
	samplingStrategy int
	samplingRate     float64
	sampleCounter    uint64
	sampleKeyFunc    func(int, string, map[string]interface{}) string

	// Non-blocking logging fields
	msgChan      chan LogMessage
	destinations []*Destination
	defaultDest  *Destination
	workerWg     sync.WaitGroup
	channelSize  int
	size         int64 // alias for currentSize for backward compatibility
}
