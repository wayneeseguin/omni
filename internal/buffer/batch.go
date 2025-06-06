package buffer

import (
	"bufio"
	"errors"
	"sync"
	"time"
)

// ErrClosed is returned when operations are attempted on a closed BatchWriter
var ErrClosed = errors.New("BatchWriter is closed")

// BatchWriter implements efficient batched writing with configurable flush triggers.
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
func (bw *BatchWriter) Write(data []byte) (int, error) {
	bw.mu.Lock()
	defer bw.mu.Unlock()

	if bw.closed {
		return 0, ErrClosed
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
func (bw *BatchWriter) WriteString(data string) (int, error) {
	return bw.Write([]byte(data))
}

// Flush forces all buffered data to be written.
func (bw *BatchWriter) Flush() error {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	return bw.flushLocked()
}

// flushLocked performs the actual flush (must be called with lock held).
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
func (bw *BatchWriter) Stats() Stats {
	bw.mu.Lock()
	defer bw.mu.Unlock()

	return Stats{
		BufferedEntries: len(bw.buffer),
		BufferedBytes:   bw.totalSize,
		MaxEntries:      bw.maxCount,
		MaxBytes:        bw.maxSize,
		FlushInterval:   bw.flushInterval,
	}
}

// Stats contains statistics about the batch writer.
type Stats struct {
	BufferedEntries int           `json:"buffered_entries"`
	BufferedBytes   int           `json:"buffered_bytes"`
	MaxEntries      int           `json:"max_entries"`
	MaxBytes        int           `json:"max_bytes"`
	FlushInterval   time.Duration `json:"flush_interval"`
}

// SetFlushInterval updates the flush interval.
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
