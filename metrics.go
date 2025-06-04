package omni

import (
	"sync/atomic"
	"time"
)

// LoggerMetrics contains runtime metrics for the logger.
// It provides a comprehensive view of logger performance and health,
// including message counts, queue status, file operations, and errors.
type LoggerMetrics struct {
	// Message counts by level
	MessagesLogged  map[int]uint64 `json:"messages_logged"`
	MessagesDropped uint64         `json:"messages_dropped"`

	// Queue metrics
	QueueDepth       int     `json:"queue_depth"`
	QueueCapacity    int     `json:"queue_capacity"`
	QueueUtilization float64 `json:"queue_utilization"`

	// File operations
	RotationCount    uint64 `json:"rotation_count"`
	CompressionCount uint64 `json:"compression_count"`
	BytesWritten     uint64 `json:"bytes_written"`

	// Error metrics
	ErrorCount     uint64            `json:"error_count"`
	ErrorsBySource map[string]uint64 `json:"errors_by_source"`
	LastError      *LogError         `json:"last_error,omitempty"`
	LastErrorTime  *time.Time        `json:"last_error_time,omitempty"`

	// Performance metrics
	AverageWriteTime time.Duration `json:"average_write_time"`
	MaxWriteTime     time.Duration `json:"max_write_time"`

	// Destination metrics
	DestinationCount int                  `json:"destination_count"`
	Destinations     []DestinationMetrics `json:"destinations"`
}

// DestinationMetrics contains metrics for a single destination.
// It tracks performance and health metrics for each logging destination,
// including write statistics, errors, and average latency.
type DestinationMetrics struct {
	Name           string        `json:"name"`
	Type           string        `json:"type"`
	Enabled        bool          `json:"enabled"`
	BytesWritten   uint64        `json:"bytes_written"`
	CurrentSize    int64         `json:"current_size"`
	Rotations      uint64        `json:"rotations"`
	Errors         uint64        `json:"errors"`
	LastWrite      time.Time     `json:"last_write"`
	AverageLatency time.Duration `json:"average_latency"`
}

// GetMetrics returns current logger metrics.
// This method provides a snapshot of all logger statistics including message counts,
// queue depth, error counts, and performance metrics. It's thread-safe and designed
// to avoid lock contention by copying data before processing.
//
// Returns:
//   - LoggerMetrics: A complete snapshot of current logger metrics
//
// Example:
//
//	metrics := logger.GetMetrics()
//	fmt.Printf("Messages logged: %d\n", metrics.MessagesLogged[omni.LevelInfo])
//	fmt.Printf("Queue utilization: %.2f%%\n", metrics.QueueUtilization*100)
func (f *Omni) GetMetrics() LoggerMetrics {
	// Collect data that requires f.mu lock
	f.mu.RLock()
	queueDepth := len(f.msgChan)
	queueCapacity := cap(f.msgChan)
	destinations := make([]*Destination, len(f.Destinations))
	copy(destinations, f.Destinations)
	lastError := f.lastError
	lastErrorTime := f.lastErrorTime
	f.mu.RUnlock()
	// Note: f.mu is released before acquiring any dest.mu locks to prevent deadlock

	metrics := LoggerMetrics{
		MessagesLogged:   make(map[int]uint64),
		MessagesDropped:  atomic.LoadUint64(&f.messagesDropped),
		QueueDepth:       queueDepth,
		QueueCapacity:    queueCapacity,
		RotationCount:    atomic.LoadUint64(&f.rotationCount),
		CompressionCount: atomic.LoadUint64(&f.compressionCount),
		BytesWritten:     atomic.LoadUint64(&f.bytesWritten),
		ErrorCount:       atomic.LoadUint64(&f.errorCount),
		ErrorsBySource:   make(map[string]uint64),
		DestinationCount: len(destinations),
	}

	// Calculate queue utilization
	if metrics.QueueCapacity > 0 {
		metrics.QueueUtilization = float64(metrics.QueueDepth) / float64(metrics.QueueCapacity)
	}

	// Copy message counts by level
	f.messagesByLevel.Range(func(key, value interface{}) bool {
		level := key.(int)
		// Handle both old uint64 values and new *atomic.Uint64 values for compatibility
		switch v := value.(type) {
		case *atomic.Uint64:
			count := v.Load()
			if count > 0 { // Only include non-zero counts
				metrics.MessagesLogged[level] = count
			}
		case uint64:
			if v > 0 { // Only include non-zero counts
				metrics.MessagesLogged[level] = v
			}
		}
		return true
	})

	// Copy error counts by source
	f.errorsBySource.Range(func(key, value interface{}) bool {
		source := key.(string)
		// Handle both old uint64 values and new *atomic.Uint64 values for compatibility
		switch v := value.(type) {
		case *atomic.Uint64:
			count := v.Load()
			if count > 0 { // Only include non-zero counts
				metrics.ErrorsBySource[source] = count
			}
		case uint64:
			if v > 0 { // Only include non-zero counts
				metrics.ErrorsBySource[source] = v
			}
		}
		return true
	})

	// Get last error info
	if lastError != nil {
		metrics.LastError = lastError
		metrics.LastErrorTime = lastErrorTime
	}

	// Get performance metrics
	metrics.AverageWriteTime = time.Duration(atomic.LoadInt64(&f.totalWriteTime)) /
		time.Duration(atomic.LoadUint64(&f.writeCount)+1) // +1 to avoid division by zero
	metrics.MaxWriteTime = time.Duration(atomic.LoadInt64(&f.maxWriteTime))

	// Collect destination metrics (no f.mu lock held)
	metrics.Destinations = make([]DestinationMetrics, 0, len(destinations))
	for _, dest := range destinations {
		dest.mu.Lock()
		dm := DestinationMetrics{
			Name:         dest.Name,
			Enabled:      dest.Enabled,
			CurrentSize:  dest.Size,
			BytesWritten: atomic.LoadUint64(&dest.bytesWritten),
			Rotations:    atomic.LoadUint64(&dest.rotations),
			Errors:       atomic.LoadUint64(&dest.errors),
			LastWrite:    dest.lastWrite,
			AverageLatency: time.Duration(atomic.LoadInt64(&dest.totalLatency)) /
				time.Duration(atomic.LoadUint64(&dest.writeCount)+1),
		}

		// Determine type
		switch dest.Backend {
		case BackendFlock:
			dm.Type = "file"
		case BackendSyslog:
			dm.Type = "syslog"
		default:
			dm.Type = "custom"
		}

		dest.mu.Unlock()
		metrics.Destinations = append(metrics.Destinations, dm)
	}

	return metrics
}

// ResetMetrics resets all metrics counters.
// This clears all accumulated statistics including message counts, error counts,
// performance metrics, and destination-specific metrics. Use this to start fresh
// metric collection, for example after a configuration change or at regular intervals.
//
// Note: This operation is thread-safe but may briefly impact metric collection accuracy
// during the reset process.
func (f *Omni) ResetMetrics() {
	// Get a copy of destinations to avoid holding f.mu while locking dest.mu
	f.mu.Lock()
	destinations := make([]*Destination, len(f.Destinations))
	copy(destinations, f.Destinations)

	// Clear last error
	f.lastError = nil
	f.lastErrorTime = nil
	f.mu.Unlock()

	// Reset message counts
	f.messagesByLevel.Range(func(key, value interface{}) bool {
		counter := value.(*atomic.Uint64)
		counter.Store(0)
		return true
	})

	// Reset other counters
	atomic.StoreUint64(&f.messagesDropped, 0)
	atomic.StoreUint64(&f.rotationCount, 0)
	atomic.StoreUint64(&f.compressionCount, 0)
	atomic.StoreUint64(&f.bytesWritten, 0)
	atomic.StoreUint64(&f.errorCount, 0)
	atomic.StoreUint64(&f.writeCount, 0)
	atomic.StoreInt64(&f.totalWriteTime, 0)
	atomic.StoreInt64(&f.maxWriteTime, 0)

	// Reset error counts by source
	f.errorsBySource.Range(func(key, value interface{}) bool {
		counter := value.(*atomic.Uint64)
		counter.Store(0)
		return true
	})

	// Reset destination metrics (no f.mu lock held)
	for _, dest := range destinations {
		dest.mu.Lock()
		atomic.StoreUint64(&dest.bytesWritten, 0)
		atomic.StoreUint64(&dest.rotations, 0)
		atomic.StoreUint64(&dest.errors, 0)
		atomic.StoreUint64(&dest.writeCount, 0)
		atomic.StoreInt64(&dest.totalLatency, 0)
		dest.mu.Unlock()
	}
}

// trackMessageLogged increments the message counter for a level.
// This internal method atomically updates the message count for the specified log level.
//
// Parameters:
//   - level: The log level to track
func (f *Omni) trackMessageLogged(level int) {
	// Use LoadOrStore to ensure the counter exists
	val, _ := f.messagesByLevel.LoadOrStore(level, &atomic.Uint64{})
	counter := val.(*atomic.Uint64)
	counter.Add(1)
}

// trackMessageDropped increments the dropped message counter.
// Called when a message cannot be sent to the processing channel due to backpressure.
func (f *Omni) trackMessageDropped() {
	atomic.AddUint64(&f.messagesDropped, 1)
}

// trackRotation increments the rotation counter.
// Called when a log file is successfully rotated.
func (f *Omni) trackRotation() {
	atomic.AddUint64(&f.rotationCount, 1)
}

// trackCompression increments the compression counter.
// Called when a log file is successfully compressed.
func (f *Omni) trackCompression() {
	atomic.AddUint64(&f.compressionCount, 1)
}

// trackWrite records write metrics.
// This method updates write statistics including bytes written, write count,
// total write time, and maximum write time.
//
// Parameters:
//   - bytes: Number of bytes written
//   - duration: Time taken for the write operation
func (f *Omni) trackWrite(bytes int64, duration time.Duration) {
	atomic.AddUint64(&f.bytesWritten, uint64(bytes))
	atomic.AddUint64(&f.writeCount, 1)
	atomic.AddInt64(&f.totalWriteTime, int64(duration))

	// Update max write time
	for {
		oldMax := atomic.LoadInt64(&f.maxWriteTime)
		if int64(duration) <= oldMax {
			break
		}
		if atomic.CompareAndSwapInt64(&f.maxWriteTime, oldMax, int64(duration)) {
			break
		}
	}
}

// trackDestinationWrite records destination-specific write metrics.
// Updates write statistics for a specific destination including bytes written,
// write count, total latency, and last write timestamp.
//
// Parameters:
//   - bytes: Number of bytes written
//   - duration: Time taken for the write operation
func (dest *Destination) trackWrite(bytes int64, duration time.Duration) {
	atomic.AddUint64(&dest.bytesWritten, uint64(bytes))
	atomic.AddUint64(&dest.writeCount, 1)
	atomic.AddInt64(&dest.totalLatency, int64(duration))
	
	// Protect lastWrite with mutex to avoid race condition with GetMetrics
	dest.mu.Lock()
	dest.lastWrite = time.Now()
	dest.mu.Unlock()
}

// trackDestinationError increments the destination error counter.
// Called when an error occurs while writing to this destination.
func (dest *Destination) trackError() {
	atomic.AddUint64(&dest.errors, 1)
}

// trackDestinationRotation increments the destination rotation counter.
// Called when this destination's log file is rotated.
func (dest *Destination) trackRotation() {
	atomic.AddUint64(&dest.rotations, 1)
}


// GetMessageCount returns the number of messages logged at a specific level.
// This is a convenience method for accessing level-specific message counts.
//
// Parameters:
//   - level: The log level to query (e.g., LevelInfo, LevelError)
//
// Returns:
//   - uint64: The number of messages logged at the specified level
//
// Example:
//
//	count := logger.GetMessageCount(omni.LevelError)
//	if count > threshold {
//	    // Alert on high error rate
//	}
func (f *Omni) GetMessageCount(level int) uint64 {
	if val, ok := f.messagesByLevel.Load(level); ok {
		if counter, ok := val.(*atomic.Uint64); ok {
			return counter.Load()
		}
	}
	return 0
}

// GetLastError returns the last error that occurred.
// This is useful for debugging and monitoring the logger's health.
//
// Returns:
//   - *LogError: The last error that occurred, or nil if no errors
//
// Example:
//
//	if err := logger.GetLastError(); err != nil {
//	    fmt.Printf("Last error: %v at %v\n", err.Message, err.Time)
//	}
func (f *Omni) GetLastError() *LogError {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.lastError
}
