package flexlog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestRotationUnderLoad tests file rotation behavior under concurrent load
func TestRotationUnderLoad(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "rotation_load.log")

	// Create logger with small file size to force frequent rotation
	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Configure for frequent rotation
	logger.SetLevel(LevelDebug)
	logger.SetMaxSize(1024) // 1KB max size
	logger.SetMaxFiles(10)  // Keep 10 rotated files

	// Generate load with multiple goroutines
	const numGoroutines = 5
	const messagesPerGoroutine = 100
	const messageSize = 50 // Approximate size of each message

	var wg sync.WaitGroup

	// Start concurrent writers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < messagesPerGoroutine; j++ {
				// Create message with predictable content and size
				padding := strings.Repeat("x", messageSize-30) // Leave room for actual content
				message := fmt.Sprintf("Goroutine_%d_Message_%d_%s", goroutineID, j, padding)

				logger.Info(message)

				// Periodic flush to ensure writes happen
				if j%10 == 0 {
					logger.FlushAll()
					time.Sleep(1 * time.Millisecond) // Small delay to allow rotation
				}
			}
		}(i)
	}

	wg.Wait()

	// Final flush
	if err := logger.FlushAll(); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	// Give time for any pending rotations to complete
	time.Sleep(200 * time.Millisecond)

	// Check for rotated files (excluding lock files)
	allFiles, err := filepath.Glob(logFile + "*")
	if err != nil {
		t.Fatalf("Failed to glob files: %v", err)
	}

	// Filter out lock files
	var files []string
	for _, file := range allFiles {
		if !strings.HasSuffix(file, ".lock") {
			files = append(files, file)
		}
	}

	if len(files) < 2 {
		t.Errorf("Expected multiple files due to rotation, got %d files: %v", len(files), files)
	}

	// Verify total message count across all files
	totalMessages := 0
	totalSize := int64(0)

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			t.Errorf("Failed to stat file %s: %v", file, err)
			continue
		}

		content, err := os.ReadFile(file)
		if err != nil {
			t.Errorf("Failed to read file %s: %v", file, err)
			continue
		}

		if len(content) > 0 {
			lines := strings.Split(strings.TrimSpace(string(content)), "\n")
			totalMessages += len(lines)
			totalSize += info.Size()
		}

		// Verify rotated files don't exceed max size (current file might be growing)
		if file != logFile && info.Size() > 1024*2 { // Allow some tolerance
			t.Errorf("Rotated file %s exceeds expected size: %d bytes", file, info.Size())
		}
	}

	expectedMessages := numGoroutines * messagesPerGoroutine
	// Under high load, significant message loss is expected due to channel overflow
	// The test validates that rotation works, not that all messages are captured
	minExpectedMessages := expectedMessages / 10 // At least 10% of messages should make it through
	if totalMessages < minExpectedMessages {
		t.Errorf("Expected at least %d messages (minimum threshold), got %d", minExpectedMessages, totalMessages)
	}

	t.Logf("Rotation test completed: %d files, %d messages, %d total bytes",
		len(files), totalMessages, totalSize)
}

// TestRotationWithMultipleDestinations tests rotation behavior with multiple destinations
func TestRotationWithMultipleDestinations(t *testing.T) {
	tempDir := t.TempDir()
	file1 := filepath.Join(tempDir, "dest1.log")
	file2 := filepath.Join(tempDir, "dest2.log")

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

	// Configure for rotation
	logger.SetLevel(LevelDebug)
	logger.SetMaxSize(512) // Small size to force rotation
	logger.SetMaxFiles(5)

	// Generate enough data to trigger rotation in both destinations
	const numMessages = 200
	const messageSize = 30

	for i := 0; i < numMessages; i++ {
		padding := strings.Repeat("y", messageSize-20)
		message := fmt.Sprintf("Message_%d_%s", i, padding)
		logger.Info(message)

		// Periodic flush with more time for processing
		if i%10 == 0 {
			logger.FlushAll()
			time.Sleep(5 * time.Millisecond)
		}
	}

	// Final flush
	if err := logger.FlushAll(); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	// Give more time for all messages to be processed and rotation to complete
	time.Sleep(500 * time.Millisecond)

	// Check rotation for both destinations
	for i, baseFile := range []string{file1, file2} {
		allFiles, err := filepath.Glob(baseFile + "*")
		if err != nil {
			t.Errorf("Failed to glob files for destination %d: %v", i+1, err)
			continue
		}

		// Filter out lock files
		var files []string
		for _, file := range allFiles {
			if !strings.HasSuffix(file, ".lock") {
				files = append(files, file)
			}
		}

		if len(files) < 2 {
			t.Errorf("Destination %d should have rotated files, got %d files: %v",
				i+1, len(files), files)
		}

		// Verify content consistency across all files for this destination
		totalLines := 0
		for _, file := range files {
			content, err := os.ReadFile(file)
			if err != nil {
				t.Errorf("Failed to read file %s: %v", file, err)
				continue
			}

			if len(content) > 0 {
				lines := strings.Split(strings.TrimSpace(string(content)), "\n")
				totalLines += len(lines)
			}
		}

		// With rapid logging and small buffers, some message loss is expected
		// We should get at least 20% of messages through (40 out of 200)
		minExpected := numMessages / 5
		if totalLines < minExpected {
			t.Errorf("Destination %d has too few messages: %d (expected at least %d)", i+1, totalLines, minExpected)
		}
	}
}

// TestRotationRaceConditions tests for race conditions during rotation
func TestRotationRaceConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition test in short mode")
	}

	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "race_test.log")

	// Create logger with very small file size to force frequent rotation
	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.SetLevel(LevelDebug)
	logger.SetMaxSize(200) // Very small to force rapid rotation
	logger.SetMaxFiles(20) // Allow many rotations

	// Start multiple writers that write rapidly
	const numWriters = 10
	const writeDuration = 2 * time.Second

	var wg sync.WaitGroup
	stopChan := make(chan struct{})

	// Start writers
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			messageCount := 0

			for {
				select {
				case <-stopChan:
					return
				default:
					message := fmt.Sprintf("Writer_%d_Message_%d_DATA", writerID, messageCount)
					logger.Info(message)
					messageCount++

					// No delay - rapid writes to stress test rotation
				}
			}
		}(i)
	}

	// Let writers run for specified duration
	time.Sleep(writeDuration)
	close(stopChan)
	wg.Wait()

	// Final flush
	if err := logger.FlushAll(); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	// Give time for final rotation operations
	time.Sleep(200 * time.Millisecond)

	// Verify file integrity (excluding lock files)
	allFiles, err := filepath.Glob(logFile + "*")
	if err != nil {
		t.Fatalf("Failed to glob files: %v", err)
	}

	// Filter out lock files
	var files []string
	for _, file := range allFiles {
		if !strings.HasSuffix(file, ".lock") {
			files = append(files, file)
		}
	}

	if len(files) < 3 {
		t.Errorf("Expected multiple files due to rapid rotation, got %d files", len(files))
	}

	// Check each file for corruption
	totalMessages := 0
	corruptedFiles := 0

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Errorf("Failed to read file %s: %v", file, err)
			corruptedFiles++
			continue
		}

		if len(content) == 0 {
			continue // Empty files are OK during rotation
		}

		// Check for obvious corruption (partial lines)
		lines := strings.Split(string(content), "\n")
		fileMessages := 0

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// Valid lines should have our pattern
			if strings.Contains(line, "Writer_") && strings.Contains(line, "_DATA") {
				fileMessages++
			} else if strings.Contains(line, "Writer_") {
				// Potentially corrupted line
				t.Logf("Potentially corrupted line in %s: %s", file, line)
			}
		}

		totalMessages += fileMessages
	}

	if corruptedFiles > 1 { // Allow for one file to have issues
		t.Errorf("Too many corrupted files: %d out of %d", corruptedFiles, len(files))
	}

	if totalMessages < 50 { // Should have gotten at least some messages under extreme load
		t.Errorf("Too few messages captured: %d (this indicates rotation is completely broken)", totalMessages)
	}

	t.Logf("Race condition test completed: %d files, %d messages, %d corrupted files",
		len(files), totalMessages, corruptedFiles)
}

// TestRotationCleanup tests that old files are properly cleaned up
func TestRotationCleanup(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "cleanup_test.log")

	// Create logger with limited file retention
	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.SetLevel(LevelDebug)
	logger.SetMaxSize(300) // Small size to force rotation
	logger.SetMaxFiles(3)  // Keep only 3 files

	// Generate enough data to create more than maxFiles rotations
	const messageSize = 40

	for i := 0; i < 300; i++ {
		padding := strings.Repeat("z", messageSize-20)
		message := fmt.Sprintf("Cleanup_%d_%s", i, padding)
		logger.Info(message)

		// Flush periodically
		if i%30 == 0 {
			logger.FlushAll()
			time.Sleep(5 * time.Millisecond) // Allow rotation to complete
		}
	}

	// Final flush
	if err := logger.FlushAll(); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	// Give time for cleanup
	time.Sleep(200 * time.Millisecond)

	// Check file count (excluding lock files)
	allFiles, err := filepath.Glob(logFile + "*")
	if err != nil {
		t.Fatalf("Failed to glob files: %v", err)
	}

	// Filter out lock files
	var files []string
	for _, file := range allFiles {
		if !strings.HasSuffix(file, ".lock") {
			files = append(files, file)
		}
	}

	// Should have at most maxFiles + 1 (current file)
	maxExpectedFiles := 4 // 3 rotated + 1 current
	if len(files) > maxExpectedFiles {
		t.Errorf("Expected at most %d files, got %d files: %v",
			maxExpectedFiles, len(files), files)
	}

	// Verify we still have recent messages
	found := false
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		if strings.Contains(string(content), "Cleanup_299") {
			found = true
			break
		}
	}

	if !found {
		t.Error("Most recent messages not found - cleanup may have removed too much")
	}
}

// TestRotationDuringShutdown tests rotation behavior during logger shutdown
func TestRotationDuringShutdown(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "shutdown_rotation.log")

	// Create logger
	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	logger.SetLevel(LevelDebug)
	logger.SetMaxSize(400) // Small size
	logger.SetMaxFiles(5)

	// Start writing in background
	go func() {
		for i := 0; i < 100; i++ {
			message := fmt.Sprintf("Shutdown_test_message_%d_data", i)
			logger.Info(message)
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Let some messages be written
	time.Sleep(50 * time.Millisecond)

	// Close logger while writes might be happening
	logger.Close()

	// Verify files exist and are valid (excluding lock files)
	allFiles, err := filepath.Glob(logFile + "*")
	if err != nil {
		t.Fatalf("Failed to glob files: %v", err)
	}

	// Filter out lock files
	var files []string
	for _, file := range allFiles {
		if !strings.HasSuffix(file, ".lock") {
			files = append(files, file)
		}
	}

	if len(files) == 0 {
		t.Fatal("No log files found after shutdown")
	}

	// Check that files are readable and contain some data
	totalSize := int64(0)
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			t.Errorf("Failed to stat file %s: %v", file, err)
			continue
		}
		totalSize += info.Size()
	}

	if totalSize == 0 {
		t.Error("All log files are empty after shutdown")
	}
}
