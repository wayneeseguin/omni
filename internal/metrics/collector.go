package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// Collector handles metrics collection for the omni logger.
type Collector struct {
	// Message counts by level
	messagesByLevel sync.Map // map[int]*atomic.Uint64
	messagesDropped uint64

	// File operations
	rotationCount    uint64
	compressionCount uint64
	bytesWritten     uint64

	// Error metrics
	errorCount     uint64
	errorsBySource sync.Map // map[string]*atomic.Uint64

	// Performance metrics
	writeCount      uint64
	totalWriteTime  int64 // nanoseconds
	maxWriteTime    int64 // nanoseconds
}

// NewCollector creates a new metrics collector.
func NewCollector() *Collector {
	return &Collector{}
}

// Metrics contains runtime metrics for the logger.
type Metrics struct {
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

	// Performance metrics
	AverageWriteTime time.Duration `json:"average_write_time"`
	MaxWriteTime     time.Duration `json:"max_write_time"`

	// Destination metrics
	DestinationCount int                   `json:"destination_count"`
	Destinations     []DestinationMetrics `json:"destinations"`
}

// DestinationMetrics contains metrics for a single destination.
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

// GetMetrics returns current metrics snapshot.
func (c *Collector) GetMetrics(queueDepth, queueCapacity int, destinations []DestinationMetrics) Metrics {
	metrics := Metrics{
		MessagesLogged:   make(map[int]uint64),
		MessagesDropped:  atomic.LoadUint64(&c.messagesDropped),
		QueueDepth:       queueDepth,
		QueueCapacity:    queueCapacity,
		RotationCount:    atomic.LoadUint64(&c.rotationCount),
		CompressionCount: atomic.LoadUint64(&c.compressionCount),
		BytesWritten:     atomic.LoadUint64(&c.bytesWritten),
		ErrorCount:       atomic.LoadUint64(&c.errorCount),
		ErrorsBySource:   make(map[string]uint64),
		DestinationCount: len(destinations),
		Destinations:     destinations,
	}

	// Calculate queue utilization
	if metrics.QueueCapacity > 0 {
		metrics.QueueUtilization = float64(metrics.QueueDepth) / float64(metrics.QueueCapacity)
	}

	// Copy message counts by level
	c.messagesByLevel.Range(func(key, value interface{}) bool {
		level := key.(int)
		counter := value.(*atomic.Uint64)
		count := counter.Load()
		if count > 0 {
			metrics.MessagesLogged[level] = count
		}
		return true
	})

	// Copy error counts by source
	c.errorsBySource.Range(func(key, value interface{}) bool {
		source := key.(string)
		counter := value.(*atomic.Uint64)
		count := counter.Load()
		if count > 0 {
			metrics.ErrorsBySource[source] = count
		}
		return true
	})

	// Get performance metrics
	writeCount := atomic.LoadUint64(&c.writeCount)
	if writeCount > 0 {
		metrics.AverageWriteTime = time.Duration(atomic.LoadInt64(&c.totalWriteTime)) / time.Duration(writeCount)
	}
	metrics.MaxWriteTime = time.Duration(atomic.LoadInt64(&c.maxWriteTime))

	return metrics
}

// ResetMetrics resets all metrics counters.
func (c *Collector) ResetMetrics() {
	// Reset message counts
	c.messagesByLevel.Range(func(key, value interface{}) bool {
		counter := value.(*atomic.Uint64)
		counter.Store(0)
		return true
	})

	// Reset other counters
	atomic.StoreUint64(&c.messagesDropped, 0)
	atomic.StoreUint64(&c.rotationCount, 0)
	atomic.StoreUint64(&c.compressionCount, 0)
	atomic.StoreUint64(&c.bytesWritten, 0)
	atomic.StoreUint64(&c.errorCount, 0)
	atomic.StoreUint64(&c.writeCount, 0)
	atomic.StoreInt64(&c.totalWriteTime, 0)
	atomic.StoreInt64(&c.maxWriteTime, 0)

	// Reset error counts by source
	c.errorsBySource.Range(func(key, value interface{}) bool {
		counter := value.(*atomic.Uint64)
		counter.Store(0)
		return true
	})
}

// TrackMessageLogged increments the message counter for a level.
func (c *Collector) TrackMessageLogged(level int) {
	val, _ := c.messagesByLevel.LoadOrStore(level, &atomic.Uint64{})
	counter := val.(*atomic.Uint64)
	counter.Add(1)
}

// TrackMessageDropped increments the dropped message counter.
func (c *Collector) TrackMessageDropped() {
	atomic.AddUint64(&c.messagesDropped, 1)
}

// TrackRotation increments the rotation counter.
func (c *Collector) TrackRotation() {
	atomic.AddUint64(&c.rotationCount, 1)
}

// TrackCompression increments the compression counter.
func (c *Collector) TrackCompression() {
	atomic.AddUint64(&c.compressionCount, 1)
}

// TrackWrite records write metrics.
func (c *Collector) TrackWrite(bytes int64, duration time.Duration) {
	atomic.AddUint64(&c.bytesWritten, uint64(bytes))
	atomic.AddUint64(&c.writeCount, 1)
	atomic.AddInt64(&c.totalWriteTime, int64(duration))

	// Update max write time
	for {
		oldMax := atomic.LoadInt64(&c.maxWriteTime)
		if int64(duration) <= oldMax {
			break
		}
		if atomic.CompareAndSwapInt64(&c.maxWriteTime, oldMax, int64(duration)) {
			break
		}
	}
}

// TrackError increments the error counter and tracks by source.
func (c *Collector) TrackError(source string) {
	atomic.AddUint64(&c.errorCount, 1)
	
	val, _ := c.errorsBySource.LoadOrStore(source, &atomic.Uint64{})
	counter := val.(*atomic.Uint64)
	counter.Add(1)
}

// GetMessageCount returns the number of messages logged at a specific level.
func (c *Collector) GetMessageCount(level int) uint64 {
	if val, ok := c.messagesByLevel.Load(level); ok {
		if counter, ok := val.(*atomic.Uint64); ok {
			return counter.Load()
		}
	}
	return 0
}

// GetErrorCount returns the total error count.
func (c *Collector) GetErrorCount() uint64 {
	return atomic.LoadUint64(&c.errorCount)
}

// GetErrorCountBySource returns the error count for a specific source.
func (c *Collector) GetErrorCountBySource(source string) uint64 {
	if val, ok := c.errorsBySource.Load(source); ok {
		if counter, ok := val.(*atomic.Uint64); ok {
			return counter.Load()
		}
	}
	return 0
}

// Stats represents basic statistics
type Stats struct {
	WriteCount   uint64
	ErrorCount   uint64
	DroppedCount uint64
	BytesWritten uint64
}

// GetStats returns basic statistics
func (c *Collector) GetStats() Stats {
	return Stats{
		WriteCount:   atomic.LoadUint64(&c.writeCount),
		ErrorCount:   atomic.LoadUint64(&c.errorCount),
		DroppedCount: atomic.LoadUint64(&c.messagesDropped),
		BytesWritten: atomic.LoadUint64(&c.bytesWritten),
	}
}

// TrackMessage is an alias for TrackMessageLogged
func (c *Collector) TrackMessage(level int) {
	c.TrackMessageLogged(level)
}

// TrackDropped is an alias for TrackMessageDropped
func (c *Collector) TrackDropped() {
	c.TrackMessageDropped()
}
