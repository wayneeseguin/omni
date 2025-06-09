package types

import (
	"time"
)

// LogMessage represents a message to be logged by a background worker.
// This is the internal message type passed through channels for async logging.
type LogMessage struct {
	Level     int
	Format    string
	Args      []interface{}
	Entry     *LogEntry
	Timestamp time.Time
	Raw       []byte
	SyncDone  chan struct{} // Used for synchronization in Sync() calls
}

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
	Metadata   map[string]interface{} `json:"metadata,omitempty"` // Additional metadata
}

// Backend interface for pluggable log backends
type Backend interface {
	// Write writes a log entry to the backend
	Write(entry []byte) (int, error)

	// Flush ensures all buffered data is written
	Flush() error

	// Close closes the backend
	Close() error

	// SupportsAtomic returns whether the backend supports atomic writes
	SupportsAtomic() bool
}

// Formatter interface for pluggable log formatters
type Formatter interface {
	// Format formats a log message
	Format(msg LogMessage) ([]byte, error)
}

// BackendStats represents statistics for a backend
type BackendStats struct {
	WriteCount     uint64
	BytesWritten   uint64
	ErrorCount     uint64
	LastError      time.Time
	TotalWriteTime time.Duration
	MaxWriteTime   time.Duration
}

// FilterFunc is a function that determines if a log entry should be logged
type FilterFunc func(level int, message string, fields map[string]interface{}) bool

// Destination represents a log destination (for recovery.go)
type Destination struct {
	URI     string
	Backend Backend
}

// Compression type constants
const (
	CompressionNone = 0
	CompressionGzip = 1
)

// Sampling strategy constants
const (
	SamplingNone        = 0
	SamplingRandom      = 1
	SamplingAdaptive    = 2
	SamplingRateLimited = 3
	SamplingHead        = 4
	SamplingTail        = 5
	SamplingConsistent  = 6
)
