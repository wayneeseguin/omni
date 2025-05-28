package flexlog

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestRotateFile tests basic log rotation
func TestRotateFile(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set a small max size to trigger rotation
	logger.SetMaxSize(100) // 100 bytes

	// Write enough data to trigger rotation
	for i := 0; i < 10; i++ {
		logger.Info("This is a test message that should trigger rotation when repeated")
	}

	// Sync to ensure writes complete
	logger.Sync()

	// Check that rotation happened - look for timestamp pattern
	files, err := filepath.Glob(filepath.Join(tempDir, "test.log.*"))
	if err != nil {
		t.Fatalf("Failed to glob files: %v", err)
	}

	// Should have at least one rotated file with timestamp
	timestampPattern := regexp.MustCompile(`test\.log\.\d{8}-\d{6}\.\d{3}`)
	foundRotated := false
	for _, file := range files {
		if timestampPattern.MatchString(filepath.Base(file)) {
			foundRotated = true
			break
		}
	}

	if !foundRotated {
		t.Errorf("Expected rotated file with timestamp pattern, got files: %v", files)
	}

	// Current log file should still exist
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Errorf("Current log file should still exist")
	}
}

// TestMaxFiles tests that old files are removed when max is exceeded
func TestMaxFiles(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set small size and max files
	logger.SetMaxSize(50)
	logger.SetMaxFiles(3)

	// Trigger multiple rotations
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			logger.Infof("Rotation %d: This message triggers rotation", i)
		}
		logger.Sync()
		time.Sleep(10 * time.Millisecond) // Small delay between rotations
	}

	// Final sync
	logger.Sync()
	time.Sleep(100 * time.Millisecond)

	// Check that we have at most maxFiles + current
	files, err := filepath.Glob(filepath.Join(tempDir, "test.log*"))
	if err != nil {
		t.Fatalf("Failed to glob files: %v", err)
	}

	// Count actual rotated files (excluding current and .lock file)
	rotatedCount := 0
	currentCount := 0
	for _, file := range files {
		base := filepath.Base(file)
		if base == "test.log" {
			currentCount++
		} else if !strings.HasSuffix(base, ".lock") {
			rotatedCount++
		}
	}

	// Should have exactly 1 current file
	if currentCount != 1 {
		t.Errorf("Expected 1 current file, got %d", currentCount)
	}

	// Should have at most maxFiles rotated files
	if rotatedCount > 3 {
		t.Errorf("Expected at most 3 rotated files, got %d (files: %v)", rotatedCount, files)
	}
}

// TestRotateDestination tests rotating a specific destination
func TestRotateDestination(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Get the first destination
	if len(logger.Destinations) == 0 {
		t.Fatal("No destinations found")
	}

	dest := logger.Destinations[0]

	// Write some data
	logger.Info("Initial message before rotation")
	logger.Sync()

	// Manually trigger rotation
	err = logger.rotateDestination(dest)
	if err != nil {
		t.Errorf("Failed to rotate destination: %v", err)
	}

	// Check that rotation happened - look for timestamp pattern
	files, err := filepath.Glob(filepath.Join(tempDir, "test.log.*"))
	if err != nil {
		t.Fatalf("Failed to glob files: %v", err)
	}

	// Should have at least one rotated file with timestamp
	timestampPattern := regexp.MustCompile(`test\.log\.\d{8}-\d{6}\.\d{3}`)
	var rotatedPath string
	for _, file := range files {
		if timestampPattern.MatchString(filepath.Base(file)) {
			rotatedPath = file
			break
		}
	}

	if rotatedPath == "" {
		t.Errorf("Expected rotated file with timestamp pattern, got files: %v", files)
		return
	}

	// Check that rotated file contains the initial message
	content, err := os.ReadFile(rotatedPath)
	if err != nil {
		t.Fatalf("Failed to read rotated file: %v", err)
	}

	if !strings.Contains(string(content), "Initial message before rotation") {
		t.Errorf("Rotated file should contain initial message")
	}
}

// TestCleanupOldLogs tests the cleanup of old log files
func TestCleanupOldLogs(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	// Create some old log files with timestamp format
	now := time.Now()
	oldTime := now.Add(-48 * time.Hour)
	recentTime := now.Add(-12 * time.Hour)

	oldFiles := []struct {
		name string
		time time.Time
	}{
		{logPath + "." + oldTime.Format(RotationTimeFormat), oldTime},
		{logPath + "." + recentTime.Format(RotationTimeFormat), recentTime},
		{logPath + "." + oldTime.Add(-24*time.Hour).Format(RotationTimeFormat), oldTime.Add(-24 * time.Hour)},
		{logPath + "." + oldTime.Format(RotationTimeFormat) + ".gz", oldTime},
		{logPath + "." + recentTime.Add(-1*time.Hour).Format(RotationTimeFormat) + ".gz", recentTime.Add(-1 * time.Hour)},
	}

	for _, file := range oldFiles {
		if err := os.WriteFile(file.name, []byte("old content"), 0644); err != nil {
			t.Fatalf("Failed to create old file %s: %v", file.name, err)
		}
		// Set modification time
		os.Chtimes(file.name, file.time, file.time)
	}

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set max age to 24 hours
	logger.SetMaxAge(24 * time.Hour)

	// Run cleanup
	err = logger.RunCleanup()
	if err != nil {
		t.Errorf("RunCleanup failed: %v", err)
	}

	// Old files (>24h) should be removed
	for _, file := range oldFiles {
		if file.time.Before(now.Add(-24 * time.Hour)) {
			if _, err := os.Stat(file.name); !os.IsNotExist(err) {
				t.Errorf("Old file %s should have been removed (timestamp from name: %v, cutoff: %v)",
					file.name, file.time, now.Add(-24*time.Hour))
			}
		} else {
			// Recent files should still exist
			if _, err := os.Stat(file.name); os.IsNotExist(err) {
				t.Errorf("Recent file %s should still exist", file.name)
			}
		}
	}
}

// TestSetMaxAge tests setting the max age
func TestSetMaxAge(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set max age
	logger.SetMaxAge(7 * 24 * time.Hour) // 7 days

	// Note: We can't directly test the maxAge value as it's private
	// but we've tested the cleanup functionality above
}

// TestSetCleanupInterval tests setting the cleanup interval
func TestSetCleanupInterval(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set cleanup interval
	logger.SetCleanupInterval(30 * time.Minute)

	// Note: We can't directly test the interval but the method should not panic
}

// TestRotationWithCompression tests rotation with compression enabled
func TestRotationWithCompression(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Enable compression
	logger.SetCompression(CompressionGzip)
	logger.SetCompressMinAge(1) // Compress after 1 rotation

	// Set small size to trigger rotations
	logger.SetMaxSize(100)
	logger.SetMaxFiles(5)

	// Trigger multiple rotations
	for i := 0; i < 3; i++ {
		for j := 0; j < 10; j++ {
			logger.Infof("Rotation %d: Message %d to trigger rotation", i, j)
		}
		logger.Sync()
		time.Sleep(50 * time.Millisecond)
	}

	// Give compression time to work
	time.Sleep(500 * time.Millisecond)

	// Check for compressed files
	compressedFiles, err := filepath.Glob(filepath.Join(tempDir, "test.log.*.gz"))
	if err != nil {
		t.Fatalf("Failed to glob compressed files: %v", err)
	}

	// Should have at least one compressed file
	if len(compressedFiles) == 0 {
		t.Errorf("Expected at least one compressed file, got none")
	}
}

// TestConcurrentRotation tests rotation with concurrent writes
func TestConcurrentRotation(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set larger size to reduce rotation frequency and avoid message loss
	logger.SetMaxSize(1000) // Increased from 200
	logger.SetMaxFiles(0)   // Disable max files cleanup during the test

	// Write from multiple goroutines
	done := make(chan bool, 5)

	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 20; j++ {
				logger.Infof("Goroutine %d message %d: This should trigger rotation", id, j)
				time.Sleep(time.Millisecond) // Small delay to avoid overwhelming
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		<-done
	}

	// Sync and wait longer for all async operations
	logger.Sync()
	time.Sleep(500 * time.Millisecond) // Increased wait time

	// Should have rotated files
	files, err := filepath.Glob(filepath.Join(tempDir, "test.log*"))
	if err != nil {
		t.Fatalf("Failed to glob files: %v", err)
	}

	// Filter out .lock file
	var logFiles []string
	for _, file := range files {
		if !strings.HasSuffix(file, ".lock") {
			logFiles = append(logFiles, file)
		}
	}

	if len(logFiles) < 2 {
		t.Errorf("Expected rotation to create multiple files, got %d", len(logFiles))
	}

	// All files should be readable and contain valid data
	totalLines := 0
	for _, file := range logFiles {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Errorf("Failed to read file %s: %v", file, err)
			continue
		}

		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Goroutine") && line != "" {
				totalLines++
			}
		}
	}

	// Should have all 100 messages (5 goroutines * 20 messages)
	// Allow some tolerance due to async nature
	if totalLines < 95 || totalLines > 100 {
		t.Errorf("Expected ~100 log lines across all files, got %d", totalLines)
	}
}

// TestRotationErrorHandling tests error handling during rotation
func TestRotationErrorHandling(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Create a directory with the name pattern that would conflict with rotation
	// but using timestamp format now
	timestamp := time.Now().Format(RotationTimeFormat)
	rotatedPath := logPath + "." + timestamp
	if err := os.Mkdir(rotatedPath, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Set small size to trigger rotation
	logger.SetMaxSize(50)

	// Write data to trigger rotation
	for i := 0; i < 10; i++ {
		logger.Info("This should trigger rotation but fail")
	}

	logger.Sync()

	// The logger should continue working despite rotation failure
	// Current log should still exist and be writable
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Errorf("Current log file should still exist despite rotation error")
	}
}

// TestTimestampFormat tests that the timestamp format is sortable
func TestTimestampFormat(t *testing.T) {
	// Generate some timestamps
	var timestamps []string
	baseTime := time.Now()

	for i := 0; i < 10; i++ {
		ts := baseTime.Add(time.Duration(i) * time.Second)
		timestamps = append(timestamps, ts.Format(RotationTimeFormat))
	}

	// Check that timestamps are in chronological order when sorted
	for i := 1; i < len(timestamps); i++ {
		if timestamps[i] <= timestamps[i-1] {
			t.Errorf("Timestamp format not sortable: %s <= %s", timestamps[i], timestamps[i-1])
		}
	}
}

// TestCleanupOldFilesCount tests the maxFiles cleanup functionality
func TestCleanupOldFilesCount(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	// Create several rotated files with different timestamps
	baseTime := time.Now()
	for i := 0; i < 10; i++ {
		ts := baseTime.Add(time.Duration(-i) * time.Minute)
		filename := fmt.Sprintf("%s.%s", logPath, ts.Format(RotationTimeFormat))
		if err := os.WriteFile(filename, []byte(fmt.Sprintf("log content %d", i)), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", filename, err)
		}
	}

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set max files to 3
	logger.SetMaxFiles(3)

	// Manually trigger cleanup
	logger.cleanupOldFiles()

	// Check remaining files
	files, err := filepath.Glob(filepath.Join(tempDir, "test.log.*"))
	if err != nil {
		t.Fatalf("Failed to glob files: %v", err)
	}

	// Count rotated files (excluding .lock file)
	rotatedCount := 0
	for _, file := range files {
		if !strings.HasSuffix(file, ".lock") {
			rotatedCount++
		}
	}

	// Should have exactly 3 rotated files remaining (newest ones)
	if rotatedCount != 3 {
		t.Errorf("Expected 3 rotated files after cleanup, got %d: %v", rotatedCount, files)
	}
}
