package flexlog

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestMetricsConcurrentAccess tests that metrics can be safely accessed concurrently
func TestMetricsConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Number of concurrent goroutines
	const numGoroutines = 100
	const numMessages = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3) // 3 operations per goroutine

	// Goroutines that log messages
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numMessages; j++ {
				switch j % 4 {
				case 0:
					logger.Debug("Debug message %d-%d", id, j)
				case 1:
					logger.Info("Info message %d-%d", id, j)
				case 2:
					logger.Warn("Warn message %d-%d", id, j)
				case 3:
					logger.Errorf("Error message %d-%d", id, j)
				}
			}
		}(i)
	}

	// Goroutines that read metrics
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numMessages; j++ {
				metrics := logger.GetMetrics()
				// Just access the metrics to ensure no race
				_ = metrics.MessagesLogged
				_ = metrics.ErrorsBySource
				time.Sleep(time.Microsecond) // Small delay to increase contention
			}
		}()
	}

	// Goroutines that reset metrics
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond) // Let some messages accumulate
			logger.ResetMetrics()
			time.Sleep(10 * time.Millisecond)
			logger.ResetMetrics()
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Give time for messages to be processed
	time.Sleep(100 * time.Millisecond)

	// Final metrics check
	metrics := logger.GetMetrics()
	t.Logf("Final metrics: %+v", metrics)
}

// TestMetricsTypeCompatibility tests that metrics handle both old and new counter types
func TestMetricsTypeCompatibility(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Manually store an old-style uint64 value (simulating old code)
	logger.messagesByLevel.Store(99, uint64(42))

	// Log some messages with standard levels
	logger.Info("Test message")
	logger.Error("Test error")

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Get metrics should handle both types
	metrics := logger.GetMetrics()

	// Check that we got the old-style value
	if count, ok := metrics.MessagesLogged[99]; !ok || count != 42 {
		t.Errorf("Expected custom level 99 to have count 42, got %d", count)
	}

	// Check that new-style atomic counters work
	if metrics.MessagesLogged[LevelInfo] != 1 {
		t.Errorf("Expected 1 info message, got %d", metrics.MessagesLogged[LevelInfo])
	}
	if metrics.MessagesLogged[LevelError] != 1 {
		t.Errorf("Expected 1 error message, got %d", metrics.MessagesLogged[LevelError])
	}
}

// TestErrorMetricsConcurrency tests concurrent error tracking
func TestErrorMetricsConcurrency(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Simulate concurrent errors from different sources
	sources := []string{"write", "rotate", "compress", "lock", "flush"}
	const numGoroutines = 50
	const errorsPerSource = 100

	var wg sync.WaitGroup
	wg.Add(len(sources) * numGoroutines)

	for _, source := range sources {
		for i := 0; i < numGoroutines; i++ {
			go func(src string) {
				defer wg.Done()
				for j := 0; j < errorsPerSource; j++ {
					logger.logError(src, "test-dest", "Test error", nil, ErrorLevelLow)
				}
			}(source)
		}
	}

	// Wait for all errors to be logged
	wg.Wait()
	logger.Close()

	// Check metrics
	metrics := logger.GetMetrics()

	// Each source should have exactly numGoroutines * errorsPerSource errors
	expectedPerSource := uint64(numGoroutines * errorsPerSource)
	for _, source := range sources {
		if count, ok := metrics.ErrorsBySource[source]; !ok || count != expectedPerSource {
			t.Errorf("Expected %d errors for source %s, got %d", expectedPerSource, source, count)
		}
	}

	// Total error count should match
	expectedTotal := uint64(len(sources) * numGoroutines * errorsPerSource)
	if metrics.ErrorCount != expectedTotal {
		t.Errorf("Expected total error count %d, got %d", expectedTotal, metrics.ErrorCount)
	}
}

// TestMetricsRaceWithRotation tests metrics access during log rotation
func TestMetricsRaceWithRotation(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set small rotation size
	logger.SetMaxSize(1024) // 1KB

	var wg sync.WaitGroup
	wg.Add(3)

	// Goroutine that logs messages to trigger rotation
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			logger.Info("This is a message that will help trigger rotation: %d", i)
		}
	}()

	// Goroutine that constantly reads metrics
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			metrics := logger.GetMetrics()
			_ = metrics.RotationCount
			_ = metrics.MessagesLogged
			time.Sleep(time.Microsecond)
		}
	}()

	// Goroutine that resets metrics during rotation
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			time.Sleep(10 * time.Millisecond)
			logger.ResetMetrics()
		}
	}()

	wg.Wait()

	// Final check
	metrics := logger.GetMetrics()
	if metrics.RotationCount == 0 {
		t.Log("Warning: No rotations occurred during test")
	}
	t.Logf("Final rotation count: %d", metrics.RotationCount)
}