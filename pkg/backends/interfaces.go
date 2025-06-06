package backends

import (
	"bufio"
	"time"
)

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
	
	// Sync syncs the backend to persistent storage
	Sync() error
	
	// GetStats returns backend statistics
	GetStats() BackendStats
}

// DestinationInfo provides information about a destination
type DestinationInfo struct {
	Name         string
	Type         string
	URI          string
	Enabled      bool
	BytesWritten uint64
	Errors       uint64
	LastWrite    time.Time
}

// BackendStats represents statistics for a backend
type BackendStats struct {
	Path           string
	Size           int64
	WriteCount     uint64
	BytesWritten   uint64
	ErrorCount     uint64
	LastError      time.Time
	TotalWriteTime time.Duration
	MaxWriteTime   time.Duration
}

// DestinationInterface represents a log output destination
type DestinationInterface interface {
	// Write writes a log entry to the destination
	Write(entry []byte) (int, error)
	
	// Flush ensures all buffered data is written
	Flush() error
	
	// Close closes the destination
	Close() error
	
	// Info returns information about the destination
	Info() DestinationInfo
}



// FileBackend represents a file-based logging backend
type FileBackend interface {
	Backend
	
	// Rotate rotates the log file
	Rotate() error
	
	// Size returns the current file size
	Size() int64
	
	// Path returns the file path
	Path() string
	
	// GetWriter returns the buffered writer
	GetWriter() *bufio.Writer
}

// SyslogBackend represents a syslog logging backend  
type SyslogBackend interface {
	Backend
	
	// SetPriority sets the syslog priority
	SetPriority(priority int)
	
	// SetTag sets the syslog tag
	SetTag(tag string)
}

// PluginBackend represents a plugin-based logging backend
type PluginBackend interface {
	Backend
	
	// Name returns the plugin name
	Name() string
	
	// Version returns the plugin version
	Version() string
	
	// Configure configures the plugin with options
	Configure(options map[string]interface{}) error
}