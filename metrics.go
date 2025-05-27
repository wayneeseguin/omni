package flexlog

import (
	"sync/atomic"
	"time"
)

// LoggerMetrics contains runtime metrics for the logger
type LoggerMetrics struct {
	// Message counts by level
	MessagesLogged map[int]uint64 `json:"messages_logged"`
	MessagesDropped uint64         `json:"messages_dropped"`
	
	// Queue metrics
	QueueDepth      int     `json:"queue_depth"`
	QueueCapacity   int     `json:"queue_capacity"`
	QueueUtilization float64 `json:"queue_utilization"`
	
	// File operations
	RotationCount    uint64 `json:"rotation_count"`
	CompressionCount uint64 `json:"compression_count"`
	BytesWritten     uint64 `json:"bytes_written"`
	
	// Error metrics
	ErrorCount       uint64            `json:"error_count"`
	ErrorsBySource   map[string]uint64 `json:"errors_by_source"`
	LastError        *LogError         `json:"last_error,omitempty"`
	LastErrorTime    *time.Time        `json:"last_error_time,omitempty"`
	
	// Performance metrics
	AverageWriteTime time.Duration `json:"average_write_time"`
	MaxWriteTime     time.Duration `json:"max_write_time"`
	
	// Destination metrics
	DestinationCount int                       `json:"destination_count"`
	Destinations     []DestinationMetrics      `json:"destinations"`
}

// DestinationMetrics contains metrics for a single destination
type DestinationMetrics struct {
	Name         string        `json:"name"`
	Type         string        `json:"type"`
	Enabled      bool          `json:"enabled"`
	BytesWritten uint64        `json:"bytes_written"`
	CurrentSize  int64         `json:"current_size"`
	Rotations    uint64        `json:"rotations"`
	Errors       uint64        `json:"errors"`
	LastWrite    time.Time     `json:"last_write"`
	AverageLatency time.Duration `json:"average_latency"`
}

// GetMetrics returns current logger metrics
func (f *FlexLog) GetMetrics() LoggerMetrics {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	metrics := LoggerMetrics{
		MessagesLogged:   make(map[int]uint64),
		MessagesDropped:  atomic.LoadUint64(&f.messagesDropped),
		QueueDepth:       len(f.msgChan),
		QueueCapacity:    cap(f.msgChan),
		RotationCount:    atomic.LoadUint64(&f.rotationCount),
		CompressionCount: atomic.LoadUint64(&f.compressionCount),
		BytesWritten:     atomic.LoadUint64(&f.bytesWritten),
		ErrorCount:       atomic.LoadUint64(&f.errorCount),
		ErrorsBySource:   make(map[string]uint64),
		DestinationCount: len(f.Destinations),
	}
	
	// Calculate queue utilization
	if metrics.QueueCapacity > 0 {
		metrics.QueueUtilization = float64(metrics.QueueDepth) / float64(metrics.QueueCapacity)
	}
	
	// Copy message counts by level
	for level, count := range f.messagesByLevel {
		metrics.MessagesLogged[level] = count
	}
	
	// Copy error counts by source
	for source, count := range f.errorsBySource {
		metrics.ErrorsBySource[source] = count
	}
	
	// Get last error info
	if f.lastError != nil {
		metrics.LastError = f.lastError
		metrics.LastErrorTime = f.lastErrorTime
	}
	
	// Get performance metrics
	metrics.AverageWriteTime = time.Duration(atomic.LoadInt64(&f.totalWriteTime)) / 
		time.Duration(atomic.LoadUint64(&f.writeCount)+1) // +1 to avoid division by zero
	metrics.MaxWriteTime = time.Duration(atomic.LoadInt64(&f.maxWriteTime))
	
	// Collect destination metrics
	metrics.Destinations = make([]DestinationMetrics, 0, len(f.Destinations))
	for _, dest := range f.Destinations {
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

// ResetMetrics resets all metrics counters
func (f *FlexLog) ResetMetrics() {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	// Reset message counts
	for level := range f.messagesByLevel {
		f.messagesByLevel[level] = 0
	}
	
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
	for source := range f.errorsBySource {
		f.errorsBySource[source] = 0
	}
	
	// Reset destination metrics
	for _, dest := range f.Destinations {
		dest.mu.Lock()
		atomic.StoreUint64(&dest.bytesWritten, 0)
		atomic.StoreUint64(&dest.rotations, 0)
		atomic.StoreUint64(&dest.errors, 0)
		atomic.StoreUint64(&dest.writeCount, 0)
		atomic.StoreInt64(&dest.totalLatency, 0)
		dest.mu.Unlock()
	}
	
	// Clear last error
	f.lastError = nil
	f.lastErrorTime = nil
}

// trackMessageLogged increments the message counter for a level
func (f *FlexLog) trackMessageLogged(level int) {
	f.mu.Lock()
	if _, exists := f.messagesByLevel[level]; !exists {
		f.messagesByLevel[level] = 0
	}
	f.messagesByLevel[level]++
	f.mu.Unlock()
}

// trackMessageDropped increments the dropped message counter
func (f *FlexLog) trackMessageDropped() {
	atomic.AddUint64(&f.messagesDropped, 1)
}

// trackRotation increments the rotation counter
func (f *FlexLog) trackRotation() {
	atomic.AddUint64(&f.rotationCount, 1)
}

// trackCompression increments the compression counter
func (f *FlexLog) trackCompression() {
	atomic.AddUint64(&f.compressionCount, 1)
}

// trackWrite records write metrics
func (f *FlexLog) trackWrite(bytes int64, duration time.Duration) {
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

// trackDestinationWrite records destination-specific write metrics
func (dest *Destination) trackWrite(bytes int64, duration time.Duration) {
	atomic.AddUint64(&dest.bytesWritten, uint64(bytes))
	atomic.AddUint64(&dest.writeCount, 1)
	atomic.AddInt64(&dest.totalLatency, int64(duration))
	dest.lastWrite = time.Now()
}

// trackDestinationError increments the destination error counter
func (dest *Destination) trackError() {
	atomic.AddUint64(&dest.errors, 1)
}

// trackDestinationRotation increments the destination rotation counter
func (dest *Destination) trackRotation() {
	atomic.AddUint64(&dest.rotations, 1)
}