package omni

import (
	"fmt"
	"time"
)

// Default batching configuration constants.
// These provide reasonable defaults for most use cases.
const (
	defaultFlushInterval = 100 * time.Millisecond // Default flush every 100ms
	defaultFlushSize     = 8192                   // Default flush at 8KB
)

// SetFlushInterval sets the flush interval for a specific destination.
// Messages are automatically flushed when this interval elapses.
// A zero duration disables time-based flushing.
//
// Parameters:
//   - destIndex: Index of the destination to configure
//   - interval: Flush interval duration
//
// Returns:
//   - error: If destination index is invalid
//
// Example:
//
//	// Flush every 50ms
//	err := logger.SetFlushInterval(0, 50*time.Millisecond)
func (f *Omni) SetFlushInterval(destIndex int, interval time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if destIndex < 0 || destIndex >= len(f.Destinations) {
		return ErrInvalidIndex
	}

	dest := f.Destinations[destIndex]
	dest.mu.Lock()
	defer dest.mu.Unlock()

	// Stop existing timer if any
	if dest.flushTimer != nil {
		dest.flushTimer.Stop()
	}

	dest.flushInterval = interval

	// Start new timer if interval is positive
	if interval > 0 {
		dest.flushTimer = time.AfterFunc(interval, func() {
			f.flushDestination(dest)
		})
	}

	return nil
}

// SetFlushSize sets the buffer size threshold for automatic flushing.
// When the buffer reaches this size, it's automatically flushed.
// A zero or negative value disables size-based flushing.
//
// Parameters:
//   - destIndex: Index of the destination to configure
//   - size: Buffer size threshold in bytes
//
// Returns:
//   - error: If destination index is invalid
//
// Example:
//
//	// Flush when buffer reaches 16KB
//	err := logger.SetFlushSize(0, 16*1024)
func (f *Omni) SetFlushSize(destIndex int, size int) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if destIndex < 0 || destIndex >= len(f.Destinations) {
		return ErrInvalidIndex
	}

	dest := f.Destinations[destIndex]
	dest.mu.Lock()
	defer dest.mu.Unlock()

	dest.flushSize = size
	return nil
}

// SetBatchingForAll configures batching for all destinations.
// This is a convenience method to enable and configure batching
// uniformly across all log destinations.
//
// Parameters:
//   - interval: Flush interval for all destinations
//   - size: Flush size threshold for all destinations
//
// Returns:
//   - error: If any configuration fails
func (f *Omni) SetBatchingForAll(interval time.Duration, size int) error {
	f.mu.Lock()
	numDests := len(f.Destinations)
	f.mu.Unlock()

	for i := 0; i < numDests; i++ {
		// Enable batching for the destination
		if err := f.EnableBatching(i, true); err != nil {
			return err
		}
		
		// Set batching configuration
		if err := f.SetBatchingConfig(i, size, 100, interval); err != nil {
			return err
		}
		
		// Also set flush parameters for backward compatibility
		if err := f.SetFlushInterval(i, interval); err != nil {
			return err
		}
		if err := f.SetFlushSize(i, size); err != nil {
			return err
		}
	}

	return nil
}

// flushDestination flushes a specific destination.
// This internal method handles the actual flush operation and
// reschedules the flush timer if periodic flushing is enabled.
func (f *Omni) flushDestination(dest *Destination) {
	dest.mu.Lock()
	defer dest.mu.Unlock()

	if dest.Writer != nil {
		dest.Writer.Flush()
	}

	// Reschedule timer if interval is set
	if dest.flushInterval > 0 {
		dest.flushTimer = time.AfterFunc(dest.flushInterval, func() {
			f.flushDestination(dest)
		})
	}
}

// checkFlushSize checks if the buffer size warrants a flush.
// This internal method monitors buffer utilization and triggers
// a flush when the buffer is sufficiently full.
func (f *Omni) checkFlushSize(dest *Destination) {
	// This is called with dest.mu already locked
	if dest.flushSize > 0 && dest.Writer != nil {
		// bufio.Writer doesn't expose buffer size directly,
		// but we can check Available() space
		available := dest.Writer.Available()
		bufSize := 4096            // Default bufio size
		if available < bufSize/4 { // If buffer is 75% full
			dest.Writer.Flush()
		}
	}
}

// stopFlushTimers stops all flush timers (called during shutdown).
// This ensures clean shutdown by stopping all background flush operations.
func (f *Omni) stopFlushTimers() {
	f.mu.Lock()
	destinations := make([]*Destination, len(f.Destinations))
	copy(destinations, f.Destinations)
	f.mu.Unlock()

	for _, dest := range destinations {
		dest.mu.Lock()
		if dest.flushTimer != nil {
			dest.flushTimer.Stop()
			dest.flushTimer = nil
		}
		dest.mu.Unlock()
	}
}

// initializeDestinationBatching sets up default batching for a destination.
// This internal function initializes batching parameters with sensible defaults.
func initializeDestinationBatching(dest *Destination) {
	dest.flushInterval = defaultFlushInterval
	dest.flushSize = defaultFlushSize

	// Initialize batch settings from defaults
	dest.batchEnabled = false     // Will be set based on configuration
	dest.batchMaxSize = 64 * 1024 // 64KB default
	dest.batchMaxCount = 100      // 100 entries default

	// Timer will be started when destination is added to logger
	// to ensure we have access to the logger's flushDestination method
}

// EnableBatching enables or disables batch processing for a destination.
// When enabled, writes are accumulated in batches for improved performance.
// When disabled, writes go directly to the underlying writer.
//
// Parameters:
//   - destIndex: Index of the destination to configure
//   - enabled: true to enable batching, false to disable
//
// Returns:
//   - error: If destination index is invalid or writer is nil
//
// Example:
//
//	// Enable batching for the primary destination
//	err := logger.EnableBatching(0, true)
func (f *Omni) EnableBatching(destIndex int, enabled bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if destIndex < 0 || destIndex >= len(f.Destinations) {
		return ErrInvalidIndex
	}

	dest := f.Destinations[destIndex]
	dest.mu.Lock()
	defer dest.mu.Unlock()

	if enabled && !dest.batchEnabled {
		// Enable batching - create BatchWriter
		if dest.Writer == nil {
			return fmt.Errorf("destination writer is nil")
		}

		dest.batchWriter = NewBatchWriter(
			dest.Writer,
			dest.batchMaxSize,
			dest.batchMaxCount,
			100*time.Millisecond, // Default flush interval for batch writer
		)
		dest.batchEnabled = true
	} else if !enabled && dest.batchEnabled {
		// Disable batching - cleanup BatchWriter
		if dest.batchWriter != nil {
			dest.batchWriter.Close()
			dest.batchWriter = nil
		}
		dest.batchEnabled = false
	}

	return nil
}

// SetBatchingConfig configures batch settings for a destination.
// This allows fine-tuning of batch parameters for optimal performance.
// Changes take effect immediately if batching is currently enabled.
//
// Parameters:
//   - destIndex: Index of the destination to configure
//   - maxSize: Maximum batch size in bytes
//   - maxCount: Maximum number of entries per batch
//   - flushInterval: Time interval for periodic flushes
//
// Returns:
//   - error: If destination index is invalid
//
// Example:
//
//	// Configure aggressive batching for high-throughput logging
//	err := logger.SetBatchingConfig(0, 128*1024, 1000, 50*time.Millisecond)
func (f *Omni) SetBatchingConfig(destIndex int, maxSize, maxCount int, flushInterval time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if destIndex < 0 || destIndex >= len(f.Destinations) {
		return ErrInvalidIndex
	}

	dest := f.Destinations[destIndex]
	dest.mu.Lock()
	defer dest.mu.Unlock()

	dest.batchMaxSize = maxSize
	dest.batchMaxCount = maxCount

	// If batching is currently enabled, update the existing BatchWriter
	if dest.batchEnabled && dest.batchWriter != nil {
		dest.batchWriter.SetBatchSize(maxSize, maxCount)
		dest.batchWriter.SetFlushInterval(flushInterval)
	}

	return nil
}

// IsBatchingEnabled returns whether batching is enabled for the specified destination.
// Useful for checking the current batching state of a destination.
//
// Parameters:
//   - destIndex: Index of the destination to check
//
// Returns:
//   - bool: true if batching is enabled
//   - error: If destination index is invalid
func (f *Omni) IsBatchingEnabled(destIndex int) (bool, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if destIndex < 0 || destIndex >= len(f.Destinations) {
		return false, ErrInvalidIndex
	}

	dest := f.Destinations[destIndex]
	dest.mu.Lock()
	defer dest.mu.Unlock()

	return dest.batchEnabled, nil
}
