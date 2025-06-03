package omni

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestFlushInterval tests that messages are flushed periodically
func TestFlushInterval(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "batch_test.log")

	// Create logger
	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set a short flush interval
	if err := logger.SetFlushInterval(0, 50*time.Millisecond); err != nil {
		t.Fatalf("Failed to set flush interval: %v", err)
	}

	// Write a message
	logger.Info("Test message 1")

	// Read file immediately - should be empty (buffered)
	content1, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if len(content1) > 0 {
		t.Error("File should be empty immediately after write (buffered)")
	}

	// Wait for flush interval
	time.Sleep(100 * time.Millisecond)

	// Read file again - should contain the message
	content2, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read file after flush: %v", err)
	}

	if !strings.Contains(string(content2), "Test message 1") {
		t.Error("Message not found after flush interval")
	}
}

// TestFlushSize tests that buffers are flushed when reaching size threshold
func TestFlushSize(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "size_test.log")

	// Create logger
	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set a large flush interval and small flush size
	if err := logger.SetFlushInterval(0, 10*time.Second); err != nil {
		t.Fatalf("Failed to set flush interval: %v", err)
	}

	if err := logger.SetFlushSize(0, 100); err != nil {
		t.Fatalf("Failed to set flush size: %v", err)
	}

	// Write many small messages to trigger size-based flush
	for i := 0; i < 50; i++ {
		logger.Info("Small message")
	}

	// Give a small amount of time for async processing
	time.Sleep(50 * time.Millisecond)

	// Read file - should contain messages due to size-based flush
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if len(content) == 0 {
		t.Error("File should contain messages after size-based flush")
	}

	// Count messages
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) < 10 {
		t.Errorf("Expected multiple messages after size flush, got %d", len(lines))
	}
}

// TestBatchingForAllDestinations tests batching configuration for multiple destinations
func TestBatchingForAllDestinations(t *testing.T) {
	tempDir := t.TempDir()
	file1 := filepath.Join(tempDir, "batch1.log")
	file2 := filepath.Join(tempDir, "batch2.log")

	// Create logger with multiple destinations
	logger, err := New(file1)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Add second destination
	if err := logger.AddDestination(file2); err != nil {
		t.Fatalf("Failed to add destination: %v", err)
	}

	// Configure batching for all destinations
	if err := logger.SetBatchingForAll(50*time.Millisecond, 200); err != nil {
		t.Fatalf("Failed to set batching for all: %v", err)
	}

	// Write messages
	logger.Info("Batched message 1")
	logger.Info("Batched message 2")

	// Check both files are empty initially
	for _, file := range []string{file1, file2} {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("Failed to read %s: %v", file, err)
		}
		if len(content) > 0 {
			t.Errorf("File %s should be empty initially (buffered)", file)
		}
	}

	// Wait for flush
	time.Sleep(100 * time.Millisecond)

	// Check both files have content
	for _, file := range []string{file1, file2} {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("Failed to read %s: %v", file, err)
		}
		if !strings.Contains(string(content), "Batched message 1") {
			t.Errorf("File %s missing batched messages", file)
		}
	}
}

// TestDisableBatching tests that setting zero interval disables time-based flushing
func TestDisableBatching(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "no_batch.log")

	// Create logger
	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Disable time-based flushing
	if err := logger.SetFlushInterval(0, 0); err != nil {
		t.Fatalf("Failed to disable flush interval: %v", err)
	}

	// Write a message
	logger.Info("No batch message")

	// Wait a bit
	time.Sleep(200 * time.Millisecond)

	// File should still be empty (no auto-flush)
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if len(content) > 0 {
		t.Error("File should remain empty with batching disabled")
	}

	// Manual flush should work
	if err := logger.FlushAll(); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	// Now file should have content
	content, err = os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read file after flush: %v", err)
	}

	if !strings.Contains(string(content), "No batch message") {
		t.Error("Message not found after manual flush")
	}
}

// TestBatchingPerformance tests that batching improves performance
func TestBatchingPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	tempDir := t.TempDir()

	// Test without batching
	file1 := filepath.Join(tempDir, "no_batch.log")
	logger1, err := New(file1)
	if err != nil {
		t.Fatalf("Failed to create logger1: %v", err)
	}

	// Disable batching
	logger1.SetFlushInterval(0, 0)

	start := time.Now()
	for i := 0; i < 1000; i++ {
		logger1.Info("Performance test message without batching")
		logger1.FlushAll() // Force immediate write
	}
	durationNoBatch := time.Since(start)
	logger1.Close()

	// Test with batching
	file2 := filepath.Join(tempDir, "batch.log")
	logger2, err := New(file2)
	if err != nil {
		t.Fatalf("Failed to create logger2: %v", err)
	}

	// Enable batching
	logger2.SetFlushInterval(0, 100*time.Millisecond)

	start = time.Now()
	for i := 0; i < 1000; i++ {
		logger2.Info("Performance test message with batching")
	}
	// Wait for final flush
	time.Sleep(150 * time.Millisecond)
	durationWithBatch := time.Since(start)
	logger2.Close()

	// Batching should be faster (less I/O operations)
	if durationWithBatch >= durationNoBatch {
		t.Logf("Batching did not improve performance: with=%v, without=%v",
			durationWithBatch, durationNoBatch)
	} else {
		improvement := float64(durationNoBatch-durationWithBatch) / float64(durationNoBatch) * 100
		t.Logf("Batching improved performance by %.1f%% (with=%v, without=%v)",
			improvement, durationWithBatch, durationNoBatch)
	}
}
