package omni

import (
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wayneeseguin/omni/internal/buffer"
)

// TestBufferPoolDefensive tests defensive programming in buffer pool
func TestBufferPoolDefensive(t *testing.T) {
	pool := buffer.NewBufferPool()

	// Test normal operation
	buf1 := pool.Get()
	if buf1 == nil {
		t.Fatal("Got nil buffer from pool")
	}

	// Write some data
	buf1.WriteString("test data")

	// Put it back
	pool.Put(buf1)

	// Get another buffer - should be reset
	buf2 := pool.Get()
	if buf2.Len() != 0 {
		t.Errorf("Buffer not reset, has %d bytes", buf2.Len())
	}

	// Test putting nil buffer (should not panic)
	pool.Put(nil)

	// Note: Cannot test internal pool behavior as BufferPool fields are private
}

/*
// TestFlushDestinationNilCheck tests nil destination handling
func TestFlushDestinationNilCheck(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test flushing nil destination
	// FlushDestination is not available
	// err = logger.FlushDestination(nil)
	// if err == nil {
	// 	t.Error("Expected error when flushing nil destination")
	// }
	// if err.Error() != "destination is nil" {
	// 	t.Errorf("Unexpected error message: %v", err)
	// }

	// Test flushing valid destination
	// if len(logger.Destinations) > 0 {
	// 	err = logger.FlushDestination(logger.Destinations[0])
	// 	if err != nil {
	// 		t.Errorf("Failed to flush valid destination: %v", err)
	// 	}
	// }
}
*/

// TestProcessMessageNilDestination tests nil destination handling in processMessage
func TestProcessMessageNilDestination(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Create a message
	msg := LogMessage{
		Level:     LevelInfo,
		Format:    "Test message",
		Args:      []interface{}{},
		Timestamp: time.Now(),
	}

	// This should not panic, even with nil destination
	logger.processMessage(msg, nil)

	// Check that an error was logged
	time.Sleep(50 * time.Millisecond)
	metrics := logger.GetMetrics()
	if metrics.ErrorCount == 0 {
		t.Error("Expected error to be logged for nil destination")
	}
}

/*
// TestMetricsWithNilValues tests metrics handling with various nil scenarios
func TestMetricsWithNilValues(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Manually store nil in maps (simulating corruption)
	logger.messagesByLevel.Store(LevelInfo, nil)
	logger.errorsBySource.Store("test", nil)

	// GetMetrics should handle nil values gracefully
	metrics := logger.GetMetrics()

	// Should not have panic'd and should have skipped nil values
	if _, exists := metrics.MessagesLogged[LevelInfo]; exists {
		t.Error("Nil value should have been skipped in messages")
	}
	if _, exists := metrics.ErrorsBySource["test"]; exists {
		t.Error("Nil value should have been skipped in errors")
	}
}
*/

// TestDestinationWithNilWriter tests handling of destination with nil writer
func TestDestinationWithNilWriter(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Create a destination with nil writer
	dest := &Destination{
		URI:     "test-nil-writer",
		Backend: -1, // Custom backend
		Writer:  nil,
		Enabled: true,
	}

	// Add it to logger
	logger.mu.Lock()
	logger.Destinations = append(logger.Destinations, dest)
	logger.mu.Unlock()

	// Try to flush it - should not panic
	// FlushDestination is not available
	/*
		err = logger.FlushDestination(dest)
		if err != nil {
			t.Errorf("Unexpected error flushing destination with nil writer: %v", err)
		}
	*/

	// Try to log - should not panic
	logger.Info("Test message to nil writer")

	// Wait for processing
	time.Sleep(100 * time.Millisecond)
}

// TestConcurrentMapOperations tests concurrent operations on sync.Map
func TestConcurrentMapOperations(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	var wg sync.WaitGroup
	const numGoroutines = 100

	// Concurrent stores of different types (testing compatibility)
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			// Half store old-style uint64, half store new-style atomic
			if id%2 == 0 {
				logger.messagesByLevel.Store(100+id, uint64(id))
			} else {
				logger.messagesByLevel.Store(100+id, &atomic.Uint64{})
			}
		}(i)
	}

	// Concurrent reads while storing
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				metrics := logger.GetMetrics()
				// Just access to ensure no panic
				_ = metrics.MessagesLogged
				time.Sleep(time.Microsecond)
			}
		}()
	}

	wg.Wait()

	// Final check - should not have panicked
	finalMetrics := logger.GetMetrics()
	t.Logf("Final messages logged: %d", finalMetrics.MessagesLogged)
}

// TestErrorHandlingWithNilContext tests error handling with nil values
func TestErrorHandlingWithNilContext(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Log error with nil error object
	logger.logError("test", "dest", "Test with nil error", nil, ErrorLevelLow)

	// Log error with empty destination
	logger.logError("test", "", "Test with empty dest", nil, ErrorLevelLow)

	// Set nil error handler
	logger.SetErrorHandler(nil)

	// Log error with nil handler - should use default stderr handler
	logger.logError("test", "dest", "Test with nil handler", nil, ErrorLevelLow)

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Check metrics
	metrics := logger.GetMetrics()
	if metrics.ErrorCount != 3 {
		t.Errorf("Expected 3 errors logged, got %d", metrics.ErrorCount)
	}
}
