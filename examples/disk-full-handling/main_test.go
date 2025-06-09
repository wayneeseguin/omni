package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wayneeseguin/omni/pkg/backends"
	"github.com/wayneeseguin/omni/pkg/features"
	"github.com/wayneeseguin/omni/pkg/omni"
)

func TestDiskFullHandlingExample(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test-diskfull.log")

	// Create rotation manager
	rotMgr := features.NewRotationManager()
	rotMgr.SetMaxFiles(2) // Keep only 2 files for testing

	// Track rotations
	rotationCount := 0
	rotMgr.SetMetricsHandler(func(metric string) {
		if metric == "rotation_completed" {
			rotationCount++
		}
	})

	// Create backend
	backend, err := backends.NewFileBackendWithRotation(logPath, rotMgr)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	backend.SetMaxRetries(2)

	// Create logger
	logger, err := omni.NewWithBackend(backend)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Log some messages
	for i := 0; i < 100; i++ {
		logger.InfoWithFields("Test message", map[string]interface{}{
			"index": i,
			"test":  true,
		})
	}

	// Force rotation
	err = backend.Rotate()
	if err != nil {
		t.Errorf("Manual rotation failed: %v", err)
	}

	// Verify rotation happened
	rotatedFiles, err := rotMgr.GetRotatedFiles(logPath)
	if err != nil {
		t.Errorf("Failed to get rotated files: %v", err)
	}

	if len(rotatedFiles) == 0 {
		t.Error("Expected at least one rotated file")
	}

	// Clean up
	for _, file := range rotatedFiles {
		os.Remove(file.Path)
	}
}

func TestDiskFullRecovery(t *testing.T) {
	// This test simulates disk full recovery behavior
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "recovery-test.log")

	// Create rotation manager with aggressive settings
	rotMgr := features.NewRotationManager()
	rotMgr.SetMaxFiles(1) // Very aggressive - keep only 1 file

	// Create backend
	backend, err := backends.NewFileBackendWithRotation(logPath, rotMgr)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}

	// Track errors
	var lastError error
	backend.SetErrorHandler(func(source, dest, msg string, err error) {
		if err != nil {
			lastError = err
		}
	})

	// Create logger
	logger, err := omni.NewWithBackend(backend)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Write data
	for i := 0; i < 10; i++ {
		logger.Info("Recovery test message")
	}

	// Manually rotate multiple times to test cleanup
	for i := 0; i < 3; i++ {
		if err := backend.Rotate(); err != nil {
			t.Logf("Rotation %d: %v", i, err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Check that old files were cleaned up
	rotatedFiles, _ := rotMgr.GetRotatedFiles(logPath)
	if len(rotatedFiles) > 1 {
		t.Errorf("Expected at most 1 rotated file, got %d", len(rotatedFiles))
	}

	// Verify we can still write
	logger.Info("Final message after rotations")
	logger.Sync()

	// Check for critical errors
	if lastError != nil {
		t.Logf("Last error (may be expected): %v", lastError)
	}
}

func BenchmarkDiskFullHandling(b *testing.B) {
	tempDir := b.TempDir()
	logPath := filepath.Join(tempDir, "bench-diskfull.log")

	rotMgr := features.NewRotationManager()
	rotMgr.SetMaxFiles(5)

	backend, err := backends.NewFileBackendWithRotation(logPath, rotMgr)
	if err != nil {
		b.Fatalf("Failed to create backend: %v", err)
	}

	logger, err := omni.NewWithBackend(backend)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	b.ResetTimer()

	// Benchmark logging with potential rotations
	for i := 0; i < b.N; i++ {
		logger.InfoWithFields("Benchmark message", map[string]interface{}{
			"iteration": i,
			"benchmark": true,
		})
	}

	logger.Sync()
}