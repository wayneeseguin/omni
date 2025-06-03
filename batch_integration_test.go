package omni

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBatchProcessingIntegration(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "batch_test.log")

	// Create a config with batching enabled
	config := DefaultConfig()
	config.Path = logFile
	config.EnableBatching = true
	config.BatchMaxSize = 1024 // 1KB
	config.BatchMaxCount = 5   // 5 entries
	config.BatchFlushInterval = 50 * time.Millisecond

	// Create logger with batching
	logger, err := NewWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Write some messages that should be batched
	logger.Info("Message 1")
	logger.Info("Message 2")
	logger.Info("Message 3")

	// Sleep for a short time to allow batching
	time.Sleep(10 * time.Millisecond)

	// The messages should not be immediately visible (still in batch)
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		// File might not exist yet if batch hasn't flushed
		t.Log("File doesn't exist yet, which is expected with batching")
	}

	// Wait for flush interval to pass
	time.Sleep(100 * time.Millisecond)

	// Now messages should be flushed to file
	if _, err := os.Stat(logFile); err != nil {
		t.Fatalf("Log file should exist after flush interval: %v", err)
	}

	// Check that the batch writer is properly configured
	if len(logger.Destinations) == 0 {
		t.Fatal("Expected at least one destination")
	}

	dest := logger.Destinations[0]
	dest.mu.Lock()
	batchEnabled := dest.batchEnabled
	hasBatchWriter := dest.batchWriter != nil
	dest.mu.Unlock()

	if !batchEnabled {
		t.Error("Expected batching to be enabled")
	}

	if !hasBatchWriter {
		t.Error("Expected batch writer to be configured")
	}

	t.Log("Batch processing integration test completed successfully")
}

func TestBatchProcessingDisabled(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "no_batch_test.log")

	// Create a config with batching disabled (default)
	config := DefaultConfig()
	config.Path = logFile
	config.EnableBatching = false

	// Create logger without batching
	logger, err := NewWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Write a message
	logger.Info("Immediate message")

	// The message should be immediately visible
	if _, err := os.Stat(logFile); err != nil {
		t.Fatalf("Log file should exist immediately without batching: %v", err)
	}

	// Check that the batch writer is not configured
	if len(logger.Destinations) == 0 {
		t.Fatal("Expected at least one destination")
	}

	dest := logger.Destinations[0]
	dest.mu.Lock()
	batchEnabled := dest.batchEnabled
	hasBatchWriter := dest.batchWriter != nil
	dest.mu.Unlock()

	if batchEnabled {
		t.Error("Expected batching to be disabled")
	}

	if hasBatchWriter {
		t.Error("Expected no batch writer when batching is disabled")
	}

	t.Log("Non-batch processing test completed successfully")
}
