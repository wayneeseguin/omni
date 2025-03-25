package flexlog

import (
	"bufio"
	"io"
	"os"
	"sync"
	"time"
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
	Timestamp  string                 `json:"timestamp"`
	Level      string                 `json:"level"`
	Message    string                 `json:"message"`
	Fields     map[string]interface{} `json:"fields,omitempty"`
	StackTrace string                 `json:"stack_trace,omitempty"`
	File       string                 `json:"file,omitempty"`
	Line       int                    `json:"line,omitempty"`
}

// FlexLog implements file-based logging with rotation
type FlexLog struct {
	mu               sync.Mutex
	file             *os.File
	writer           *bufio.Writer
	path             string
	maxSize          int64
	maxFiles         int
	currentSize      int64
	level            int // minimum log level
	lockFd           int // file descriptor for file locking
	format           LogFormat
	includeTrace     bool
	stackSize        int  // size of stack trace buffer
	captureAll       bool // capture stack traces for all levels, not just errors
	formatOptions    map[FormatOption]interface{}
	compression      CompressionType
	compressMinAge   int // minimum age (in rotations) before compressing
	compressWorkers  int // number of worker goroutines for compression
	compressCh       chan string
	maxAge           time.Duration                                                         // maximum age for log files
	cleanupInterval  time.Duration                                                         // how often to check for old logs
	cleanupTicker    *time.Ticker                                                          // ticker for periodic cleanup
	cleanupDone      chan struct{}                                                         // signal for cleanup goroutine shutdown
	filters          []FilterFunc                                                          // Log filters
	samplingStrategy SamplingStrategy                                                      // Sampling strategy
	samplingRate     float64                                                               // Sampling rate (0.0-1.0 for random, >1 for interval)
	sampleCounter    uint64                                                                // Atomic counter for interval sampling
	sampleKeyFunc    func(level int, message string, fields map[string]interface{}) string // Key function for consistent sampling
	destinations     []LogDestination
	size             int64
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
