package flexlog

import (
	"bufio"
	"sync"
	"time"
)

// BatchWriter implements efficient batched writing with configurable flush triggers.
// It accumulates multiple write operations into a single batch to improve I/O performance.
// Batches are flushed when size/count limits are reached or after a timeout.
type BatchWriter struct {
	writer        *bufio.Writer
	mu            sync.Mutex
	buffer        [][]byte    // Batch buffer for pending writes
	totalSize     int         // Total size of pending data
	maxSize       int         // Maximum batch size before auto-flush
	maxCount      int         // Maximum number of entries before auto-flush
	flushTimer    *time.Timer // Timer for periodic flushes
	flushInterval time.Duration
	closed        bool
}

// NewBatchWriter creates a new batch writer with the specified configuration.
// The batch writer improves write performance by reducing the number of system calls.
//
// Parameters:
//   - writer: The underlying buffered writer
//   - maxSize: Maximum batch size in bytes before auto-flush
//   - maxCount: Maximum number of entries before auto-flush
//   - flushInterval: Time interval for periodic flushes (0 disables timer)
//
// Returns:
//   - *BatchWriter: A new batch writer instance
//
// Example:
//
//	bw := NewBatchWriter(bufWriter, 64*1024, 100, 100*time.Millisecond)
func NewBatchWriter(writer *bufio.Writer, maxSize, maxCount int, flushInterval time.Duration) *BatchWriter {
	bw := &BatchWriter{
		writer:        writer,
		buffer:        make([][]byte, 0, maxCount),
		maxSize:       maxSize,
		maxCount:      maxCount,
		flushInterval: flushInterval,
	}

	// Start flush timer if interval is set
	if flushInterval > 0 {
		bw.flushTimer = time.AfterFunc(flushInterval, func() {
			bw.timedFlush()
		})
	}

	return bw
}

// Write adds data to the batch buffer and flushes if necessary.
// Data is copied to avoid race conditions. The batch is automatically
// flushed when size or count limits are exceeded.
//
// Parameters:
//   - data: The data to write
//
// Returns:
//   - int: Number of bytes accepted (always len(data) unless closed)
//   - error: Write or flush error
func (bw *BatchWriter) Write(data []byte) (int, error) {
	bw.mu.Lock()
	defer bw.mu.Unlock()

	if bw.closed {
		return 0, ErrLoggerClosed
	}

	// Create a copy of the data to avoid races
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	// Add to batch
	bw.buffer = append(bw.buffer, dataCopy)
	bw.totalSize += len(dataCopy)

	// Check if we need to flush
	shouldFlush := bw.totalSize >= bw.maxSize || len(bw.buffer) >= bw.maxCount

	if shouldFlush {
		return len(data), bw.flushLocked()
	}

	// Reset timer for next interval
	if bw.flushTimer != nil {
		bw.flushTimer.Reset(bw.flushInterval)
	}

	return len(data), nil
}

// WriteString is a convenience method for string data.
// Equivalent to Write([]byte(data)).
//
// Parameters:
//   - data: The string to write
//
// Returns:
//   - int: Number of bytes written
//   - error: Write or flush error
func (bw *BatchWriter) WriteString(data string) (int, error) {
	return bw.Write([]byte(data))
}

// Flush forces all buffered data to be written.
// This method blocks until all pending data is written to the underlying writer.
//
// Returns:
//   - error: Flush error if write fails
func (bw *BatchWriter) Flush() error {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	return bw.flushLocked()
}

// flushLocked performs the actual flush (must be called with lock held).
// This internal method writes all buffered data and resets the batch state.
func (bw *BatchWriter) flushLocked() error {
	if len(bw.buffer) == 0 {
		return nil
	}

	// Write all buffered data in one go
	for _, data := range bw.buffer {
		if _, err := bw.writer.Write(data); err != nil {
			return err
		}
	}

	// Flush the underlying writer
	if err := bw.writer.Flush(); err != nil {
		return err
	}

	// Reset batch state
	bw.buffer = bw.buffer[:0] // Reset slice but keep capacity
	bw.totalSize = 0

	return nil
}

// timedFlush is called by the timer to flush periodically.
// This ensures data doesn't remain buffered indefinitely even with low write volume.
func (bw *BatchWriter) timedFlush() {
	bw.mu.Lock()
	defer bw.mu.Unlock()

	if bw.closed {
		return
	}

	// Flush if there's data
	if len(bw.buffer) > 0 {
		bw.flushLocked()
	}

	// Reset timer for next interval
	if bw.flushTimer != nil {
		bw.flushTimer.Reset(bw.flushInterval)
	}
}

// Close flushes any remaining data and stops the timer.
// After closing, all write operations will fail with ErrLoggerClosed.
//
// Returns:
//   - error: Final flush error if write fails
func (bw *BatchWriter) Close() error {
	bw.mu.Lock()
	defer bw.mu.Unlock()

	if bw.closed {
		return nil
	}

	bw.closed = true

	// Stop timer
	if bw.flushTimer != nil {
		bw.flushTimer.Stop()
	}

	// Flush remaining data
	return bw.flushLocked()
}

// Stats returns current batch writer statistics.
// Useful for monitoring buffer utilization and tuning batch parameters.
//
// Returns:
//   - BatchWriterStats: Current statistics snapshot
func (bw *BatchWriter) Stats() BatchWriterStats {
	bw.mu.Lock()
	defer bw.mu.Unlock()

	return BatchWriterStats{
		BufferedEntries: len(bw.buffer),
		BufferedBytes:   bw.totalSize,
		MaxEntries:      bw.maxCount,
		MaxBytes:        bw.maxSize,
		FlushInterval:   bw.flushInterval,
	}
}

// BatchWriterStats contains statistics about the batch writer.
// These metrics help monitor batching effectiveness and buffer utilization.
type BatchWriterStats struct {
	BufferedEntries int           `json:"buffered_entries"`
	BufferedBytes   int           `json:"buffered_bytes"`
	MaxEntries      int           `json:"max_entries"`
	MaxBytes        int           `json:"max_bytes"`
	FlushInterval   time.Duration `json:"flush_interval"`
}

// SetFlushInterval updates the flush interval.
// This allows dynamic adjustment of the flush timer based on workload.
// Setting interval to 0 disables periodic flushing.
//
// Parameters:
//   - interval: New flush interval duration
func (bw *BatchWriter) SetFlushInterval(interval time.Duration) {
	bw.mu.Lock()
	defer bw.mu.Unlock()

	bw.flushInterval = interval

	// Update timer
	if bw.flushTimer != nil {
		bw.flushTimer.Stop()
	}

	if interval > 0 && !bw.closed {
		bw.flushTimer = time.AfterFunc(interval, func() {
			bw.timedFlush()
		})
	}
}

// SetBatchSize updates the maximum batch size.
// If the current buffer exceeds the new limits, it will be flushed immediately.
// This allows dynamic tuning of batch parameters based on system conditions.
//
// Parameters:
//   - maxSize: New maximum batch size in bytes
//   - maxCount: New maximum number of entries
func (bw *BatchWriter) SetBatchSize(maxSize, maxCount int) {
	bw.mu.Lock()
	defer bw.mu.Unlock()

	bw.maxSize = maxSize
	bw.maxCount = maxCount

	// Check if current buffer exceeds new limits
	if bw.totalSize >= bw.maxSize || len(bw.buffer) >= bw.maxCount {
		bw.flushLocked()
	}
}
