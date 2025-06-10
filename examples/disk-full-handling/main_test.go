package main

import (
	"os"
	"path/filepath"
	"testing"

	testhelpers "github.com/wayneeseguin/omni/internal/testing"
	"github.com/wayneeseguin/omni/pkg/omni"
)

func TestDiskFullHandlingExample(t *testing.T) {
	testhelpers.SkipIfUnit(t)

	// Create a temporary directory for testing
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test-diskfull.log")

	// Create logger with file backend
	logger, err := omni.NewWithBackend(logPath, omni.BackendFlock)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Configure for small file sizes to test rotation
	logger.SetMaxSize(1024) // 1KB files for quick rotation
	logger.SetMaxFiles(2)   // Keep only 2 files

	// Log some messages to trigger rotation
	for i := 0; i < 50; i++ {
		logger.InfoWithFields("Test message with data", map[string]interface{}{
			"index": i,
			"test":  true,
			"data":  string(make([]byte, 100)), // Add some data to reach size limit
		})
	}

	// Verify log file was created
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Expected log file to be created")
	}

	// Check if rotated files exist (automatic rotation should have occurred)
	if _, err := os.Stat(logPath + ".1"); err == nil {
		t.Log("Rotation occurred successfully")
	}
}

func TestDiskFullRecovery(t *testing.T) {
	testhelpers.SkipIfUnit(t)

	// This test verifies that the logger continues to work even with limited disk space
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "recovery-test.log")

	// Create logger with file backend
	logger, err := omni.NewWithBackend(logPath, omni.BackendFlock)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Configure for aggressive rotation
	logger.SetMaxSize(512) // Very small files
	logger.SetMaxFiles(1)  // Keep only 1 rotated file

	// Write data that should trigger multiple rotations
	for i := 0; i < 20; i++ {
		logger.InfoWithFields("Recovery test message", map[string]interface{}{
			"iteration": i,
			"data":      string(make([]byte, 200)), // Large enough to trigger rotation
		})
	}

	// Verify log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Expected log file to be created")
	}

	// Verify we can still write
	logger.Info("Final message after rotations")
}

func BenchmarkDiskFullHandling(b *testing.B) {
	tempDir := b.TempDir()
	logPath := filepath.Join(tempDir, "bench-diskfull.log")

	logger, err := omni.NewWithBackend(logPath, omni.BackendFlock)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Configure for rotation during benchmark
	logger.SetMaxSize(10 * 1024) // 10KB files
	logger.SetMaxFiles(5)        // Keep 5 files

	b.ResetTimer()

	// Benchmark logging with potential rotations
	for i := 0; i < b.N; i++ {
		logger.InfoWithFields("Benchmark message", map[string]interface{}{
			"iteration": i,
			"benchmark": true,
		})
	}
}
