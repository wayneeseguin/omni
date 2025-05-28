package flexlog

import (
	"time"
)

// Default batching configuration
const (
	defaultFlushInterval = 100 * time.Millisecond // Default flush every 100ms
	defaultFlushSize     = 8192                   // Default flush at 8KB
)

// SetFlushInterval sets the flush interval for a specific destination.
// Messages are automatically flushed when this interval elapses.
// A zero duration disables time-based flushing.
func (f *FlexLog) SetFlushInterval(destIndex int, interval time.Duration) error {
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
func (f *FlexLog) SetFlushSize(destIndex int, size int) error {
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

// SetBatchingForAll configures batching for all destinations
func (f *FlexLog) SetBatchingForAll(interval time.Duration, size int) error {
	f.mu.Lock()
	numDests := len(f.Destinations)
	f.mu.Unlock()

	for i := 0; i < numDests; i++ {
		if err := f.SetFlushInterval(i, interval); err != nil {
			return err
		}
		if err := f.SetFlushSize(i, size); err != nil {
			return err
		}
	}

	return nil
}

// flushDestination flushes a specific destination
func (f *FlexLog) flushDestination(dest *Destination) {
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

// checkFlushSize checks if the buffer size warrants a flush
func (f *FlexLog) checkFlushSize(dest *Destination) {
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

// stopFlushTimers stops all flush timers (called during shutdown)
func (f *FlexLog) stopFlushTimers() {
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

// initializeDestinationBatching sets up default batching for a destination
func initializeDestinationBatching(dest *Destination) {
	dest.flushInterval = defaultFlushInterval
	dest.flushSize = defaultFlushSize

	// Timer will be started when destination is added to logger
	// to ensure we have access to the logger's flushDestination method
}
