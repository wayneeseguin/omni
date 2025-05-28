package flexlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRotationTimeFormat(t *testing.T) {
	// Test that the format is sortable
	times := []time.Time{
		time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC),
		time.Date(2023, 1, 1, 10, 0, 0, 1000000, time.UTC), // 1ms later
		time.Date(2023, 1, 1, 10, 0, 1, 0, time.UTC),       // 1s later
		time.Date(2023, 1, 2, 10, 0, 0, 0, time.UTC),       // 1 day later
	}

	formatted := make([]string, len(times))
	for i, t := range times {
		formatted[i] = t.Format(RotationTimeFormat)
	}

	// Check that they are in order
	for i := 1; i < len(formatted); i++ {
		if formatted[i] <= formatted[i-1] {
			t.Errorf("Time format not sortable: %s <= %s", formatted[i], formatted[i-1])
		}
	}

	// Check format includes milliseconds
	if !strings.Contains(formatted[1], ".001") {
		t.Errorf("Format doesn't include milliseconds: %s", formatted[1])
	}
}

func TestRotateOperation(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test_rotate.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Write initial content
	logger.Info("Before rotation")
	logger.FlushAll()

	// Verify file exists
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("Log file doesn't exist: %v", err)
	}

	// Perform rotation
	err = logger.rotate()
	if err != nil {
		t.Fatalf("Rotation failed: %v", err)
	}

	// Write after rotation
	logger.Info("After rotation")
	logger.FlushAll()

	// Check that we have a rotated file
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	rotatedCount := 0
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "test_rotate.log.") {
			rotatedCount++
		}
	}

	if rotatedCount != 1 {
		t.Errorf("Expected 1 rotated file, found %d", rotatedCount)
	}

	// Verify new file exists and contains new content
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read new log file: %v", err)
	}

	if !strings.Contains(string(content), "After rotation") {
		t.Error("New log file doesn't contain post-rotation content")
	}

	if strings.Contains(string(content), "Before rotation") {
		t.Error("New log file contains pre-rotation content")
	}
}

func TestCleanupOldFilesOperation(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test_cleanup.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set max files
	logger.SetMaxFiles(3)

	// Create mock rotated files with different timestamps
	baseTime := time.Now()
	for i := 0; i < 5; i++ {
		timestamp := baseTime.Add(time.Duration(i) * time.Minute).Format(RotationTimeFormat)
		rotatedPath := filepath.Join(tmpDir, "test_cleanup.log."+timestamp)
		if err := os.WriteFile(rotatedPath, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Run cleanup
	err = logger.cleanupOldFiles()
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Check remaining files
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	rotatedCount := 0
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "test_cleanup.log.") {
			rotatedCount++
		}
	}

	// Should have at most maxFiles rotated files
	if rotatedCount > 3 {
		t.Errorf("Expected at most 3 rotated files, found %d", rotatedCount)
	}
}

func TestCleanupOldLogsOperation(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test_age_cleanup.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set max age to 1 hour
	logger.SetMaxAge(1 * time.Hour)

	// Create old and new files
	now := time.Now()
	oldTime := now.Add(-2 * time.Hour)    // 2 hours old
	newTime := now.Add(-30 * time.Minute) // 30 minutes old

	// Create old file
	oldTimestamp := oldTime.Format(RotationTimeFormat)
	oldPath := filepath.Join(tmpDir, "test_age_cleanup.log."+oldTimestamp)
	if err := os.WriteFile(oldPath, []byte("old"), 0644); err != nil {
		t.Fatalf("Failed to create old file: %v", err)
	}

	// Create new file
	newTimestamp := newTime.Format(RotationTimeFormat)
	newPath := filepath.Join(tmpDir, "test_age_cleanup.log."+newTimestamp)
	if err := os.WriteFile(newPath, []byte("new"), 0644); err != nil {
		t.Fatalf("Failed to create new file: %v", err)
	}

	// Run cleanup
	err = logger.cleanupOldLogs()
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Old file should be deleted
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("Old file was not deleted")
	}

	// New file should remain
	if _, err := os.Stat(newPath); err != nil {
		t.Error("New file was deleted")
	}
}

func TestRotateDestinationOperation(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test_dest_rotate.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Add a second destination
	secondPath := filepath.Join(tmpDir, "second.log")
	err = logger.AddDestination(secondPath)
	if err != nil {
		t.Fatalf("Failed to add destination: %v", err)
	}

	// Find the second destination
	var secondDest *Destination
	for _, d := range logger.Destinations {
		if d.URI == secondPath {
			secondDest = d
			break
		}
	}

	if secondDest == nil {
		t.Fatal("Second destination not found")
	}

	// Write content to second destination
	logger.Info("Before destination rotation")
	logger.FlushAll()

	// Rotate the second destination
	err = logger.rotateDestination(secondDest)
	if err != nil {
		t.Fatalf("Destination rotation failed: %v", err)
	}

	// Write after rotation
	logger.Info("After destination rotation")
	logger.FlushAll()

	// Check for rotated file
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	rotatedSecond := false
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "second.log.") {
			rotatedSecond = true
			break
		}
	}

	if !rotatedSecond {
		t.Error("Second destination was not rotated")
	}

	// Verify new file has new content only
	content, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatalf("Failed to read second log: %v", err)
	}

	if !strings.Contains(string(content), "After destination rotation") {
		t.Error("New log doesn't contain post-rotation content")
	}
}

func TestSetMaxAgeOperation(t *testing.T) {
	logger, err := New("/tmp/test.log")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Initially, cleanup should not be running
	logger.mu.Lock()
	if logger.cleanupTicker != nil {
		t.Error("Cleanup ticker should be nil initially")
	}
	logger.mu.Unlock()

	// Set max age
	logger.SetMaxAge(24 * time.Hour)

	// Cleanup should now be running
	logger.mu.Lock()
	if logger.cleanupTicker == nil {
		t.Error("Cleanup ticker should be started after setting max age")
	}
	if logger.maxAge != 24*time.Hour {
		t.Errorf("Max age not set correctly: %v", logger.maxAge)
	}
	logger.mu.Unlock()

	// Set to 0 to disable
	logger.SetMaxAge(0)

	// Cleanup should be stopped
	time.Sleep(100 * time.Millisecond) // Give time for cleanup to stop
	logger.mu.Lock()
	if logger.cleanupTicker != nil {
		t.Error("Cleanup ticker should be nil after disabling")
	}
	logger.mu.Unlock()
}

func TestSetCleanupIntervalOperation(t *testing.T) {
	logger, err := New("/tmp/test.log")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set cleanup interval
	logger.SetCleanupInterval(5 * time.Minute)

	logger.mu.Lock()
	if logger.cleanupInterval != 5*time.Minute {
		t.Errorf("Cleanup interval not set correctly: %v", logger.cleanupInterval)
	}
	logger.mu.Unlock()

	// Test minimum enforcement
	logger.SetCleanupInterval(30 * time.Second)

	logger.mu.Lock()
	if logger.cleanupInterval != time.Minute {
		t.Errorf("Cleanup interval should be enforced to minimum 1 minute, got: %v", logger.cleanupInterval)
	}
	logger.mu.Unlock()
}

func TestRunCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test_run_cleanup.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set max age
	logger.SetMaxAge(1 * time.Hour)

	// Create an old file
	oldTime := time.Now().Add(-2 * time.Hour)
	oldTimestamp := oldTime.Format(RotationTimeFormat)
	oldPath := filepath.Join(tmpDir, "test_run_cleanup.log."+oldTimestamp)
	if err := os.WriteFile(oldPath, []byte("old"), 0644); err != nil {
		t.Fatalf("Failed to create old file: %v", err)
	}

	// Run cleanup manually
	err = logger.RunCleanup()
	if err != nil {
		t.Fatalf("RunCleanup failed: %v", err)
	}

	// Old file should be deleted
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("Old file was not deleted by RunCleanup")
	}
}
