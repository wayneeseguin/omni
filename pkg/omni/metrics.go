package omni

import (
	"time"
)

// LoggerMetrics contains runtime metrics for the logger
type LoggerMetrics struct {
	// Message counters
	MessagesLogged  uint64         // Total messages logged
	MessagesByLevel map[int]uint64 // Messages by log level
	MessagesDropped uint64         // Messages dropped due to full channels

	// Performance metrics
	AverageWriteTime time.Duration // Average time to write a message
	MaxWriteTime     time.Duration // Maximum time to write a message
	TotalWriteTime   time.Duration // Total time spent writing
	BytesWritten     uint64        // Total bytes written
	WriteCount       uint64        // Number of write operations

	// File operations
	RotationCount    uint64 // Number of file rotations
	CompressionCount uint64 // Number of files compressed

	// Error tracking
	ErrorCount     uint64            // Total errors encountered
	ErrorsBySource map[string]uint64 // Errors by source/operation
	LastError      *LogError         // Most recent error
	LastErrorTime  *time.Time        // Time of last error

	// Queue status
	ChannelSize        int     // Size of message channel
	ChannelUsage       int     // Current channel usage
	ChannelUtilization float64 // Channel utilization percentage

	// Destination metrics
	ActiveDestinations   int // Number of active destinations
	DisabledDestinations int // Number of disabled destinations

	// Uptime and lifecycle
	StartTime time.Time     // When the logger was created
	Uptime    time.Duration // How long the logger has been running

	// Memory and resource usage
	BufferPoolHits   uint64 // Buffer pool cache hits
	BufferPoolMisses uint64 // Buffer pool cache misses
}
