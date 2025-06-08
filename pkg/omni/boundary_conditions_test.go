package omni

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestZeroMaxSize tests behavior with zero max size (should disable rotation)
func TestZeroMaxSize(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set zero max size (disables rotation)
	logger.SetMaxSize(0)

	// Log multiple messages
	for i := 0; i < 100; i++ {
		logger.Info("Message %d with some content to ensure we write more data", i)
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Should have no rotations with zero max size
	metrics := logger.GetMetrics()
	if metrics.RotationCount != 0 {
		t.Errorf("Expected no rotations with zero max size, got %d", metrics.RotationCount)
	}

	// Check that all data was written to a single file (excluding lock files)
	files, _ := os.ReadDir(dir)
	fileCount := 0
	var fileNames []string
	for _, f := range files {
		if !f.IsDir() && !strings.HasSuffix(f.Name(), ".lock") {
			fileCount++
			fileNames = append(fileNames, f.Name())
		}
	}
	if fileCount != 1 {
		t.Errorf("Expected exactly 1 log file with zero max size, got %d files: %v", fileCount, fileNames)
	}
}

// TestVeryLargeMessage tests handling of messages larger than buffer
func TestVeryLargeMessage(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Create a very large message
	largeData := strings.Repeat("A", 1024*1024) // 1MB message
	logger.Info("Large message: %s", largeData)

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	// Check that it was written
	metrics := logger.GetMetrics()
	if metrics.BytesWritten < 1024*1024 {
		t.Errorf("Expected at least 1MB written, got %d bytes", metrics.BytesWritten)
	}
}

// TestEmptyLogMessage tests handling of empty messages
func TestEmptyLogMessage(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set level to debug to capture all messages
	logger.SetLevel(LevelDebug)

	// Log empty messages
	logger.Info("")
	logger.Debug("")
	logger.Warn("")
	logger.Error("")

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Should still count as logged messages
	metrics := logger.GetMetrics()
	totalLogged := uint64(0)
	for _, count := range metrics.MessagesByLevel {
		totalLogged += count
	}
	if totalLogged != 4 {
		t.Errorf("Expected 4 messages logged, got %d", totalLogged)
	}
}

// TestMaxFilesZero tests behavior with zero max files (no cleanup)
func TestMaxFilesZero(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set zero max files and small rotation size
	logger.SetMaxFiles(0)
	logger.SetMaxSize(50) // Very small size to force many rotations

	// Log enough to cause multiple rotations
	for i := 0; i < 10; i++ {
		logger.Info("This is a longer message that will cause rotation number %d", i)
		// Force flush after each message to ensure rotation happens
		logger.FlushAll()
		time.Sleep(10 * time.Millisecond) // Small delay to ensure rotation completes
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Ensure all messages are flushed
	logger.FlushAll()

	// Count rotated files
	files, _ := os.ReadDir(dir)
	rotatedCount := 0
	for _, f := range files {
		if strings.Contains(f.Name(), "test.log.") {
			rotatedCount++
		}
	}

	// With zero max files, all rotated files should be kept
	// With 10 messages and maxSize of 50, we should get multiple rotations
	// Each message is approximately 60+ bytes, so we expect around 10 rotations
	if rotatedCount < 3 {
		t.Errorf("Expected at least 3 rotated files, got %d", rotatedCount)
		// Debug: print file sizes
		for _, f := range files {
			if !f.IsDir() {
				info, _ := f.Info()
				t.Logf("File: %s, Size: %d bytes", f.Name(), info.Size())
			}
		}
	}
}

// TestNegativeValues tests handling of negative configuration values
func TestNegativeValues(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set negative values (should be treated as zero/disabled)
	logger.SetMaxSize(-1000)
	logger.SetMaxFiles(-5)
	logger.SetMaxAge(-24 * time.Hour)

	// Log some messages
	logger.Info("Test message")

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Should still work without crashing
	metrics := logger.GetMetrics()
	if metrics.BytesWritten == 0 {
		t.Error("Expected some bytes written despite negative config")
	}
}

// TestShutdownWithFullChannel tests shutdown when channel is full
func TestShutdownWithFullChannel(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Fill the channel completely
	logger.channelSize = 10
	for i := 0; i < 1000; i++ {
		logger.Info("Filling channel %d", i)
	}

	// Shutdown should still work and process remaining messages
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = logger.Shutdown(ctx)
	cancel()

	if err != nil {
		t.Logf("Shutdown error (might be expected): %v", err)
	}

	// Check some messages were processed
	metrics := logger.GetMetrics()
	totalLogged := uint64(0)
	for _, count := range metrics.MessagesByLevel {
		totalLogged += count
	}
	if totalLogged == 0 {
		t.Error("No messages were processed during shutdown")
	}
}

// TestRapidOpenClose tests rapid open/close cycles
func TestRapidOpenClose(t *testing.T) {
	dir := t.TempDir()

	// Rapidly create and close loggers
	for i := 0; i < 50; i++ {
		logFile := filepath.Join(dir, "test.log")
		logger, err := New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger %d: %v", i, err)
		}

		// Log one message
		logger.Info("Rapid test %d", i)

		// Close immediately
		if err := logger.Close(); err != nil {
			t.Errorf("Failed to close logger %d: %v", i, err)
		}
	}

	// No goroutine leaks or file descriptor leaks should occur
	time.Sleep(500 * time.Millisecond)
}

// TestConcurrentDestinationOperations tests concurrent operations on destinations
func TestConcurrentDestinationOperations(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Add multiple destinations
	for i := 0; i < 5; i++ {
		destFile := filepath.Join(dir, fmt.Sprintf("dest%d.log", i))
		if err := logger.AddDestinationWithBackend(destFile, BackendFlock); err != nil {
			t.Fatalf("Failed to add destination %d: %v", i, err)
		}
	}

	// Concurrently log, add, remove destinations
	done := make(chan bool)

	// Logger goroutine
	go func() {
		for i := 0; i < 1000; i++ {
			logger.Info("Concurrent message %d", i)
			time.Sleep(time.Microsecond)
		}
		done <- true
	}()

	// Destination modifier goroutine
	go func() {
		for i := 0; i < 10; i++ {
			destFile := filepath.Join(dir, fmt.Sprintf("dynamic%d.log", i))
			logger.AddDestinationWithBackend(destFile, BackendFlock)
			time.Sleep(10 * time.Millisecond)
			logger.RemoveDestination(destFile)
		}
		done <- true
	}()

	// Wait for both to complete
	<-done
	<-done

	// Should complete without deadlock or panic
	metrics := logger.GetMetrics()
	if metrics.ActiveDestinations == 0 {
		t.Error("No destinations remaining")
	}
}

// TestUnicodeAndSpecialCharacters tests handling of unicode and special characters
func TestUnicodeAndSpecialCharacters(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test various unicode and special characters
	testCases := []string{
		"Hello 世界",
		"Emoji: 😀🎉🔥",
		"Special: \n\t\r",
		"Null byte: \x00",
		"Long unicode: " + strings.Repeat("🦄", 1000),
		"Mixed: Hello\x00世界\n😀",
	}

	for _, msg := range testCases {
		logger.Info(msg)
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Check all messages were logged
	metrics := logger.GetMetrics()
	if metrics.MessagesByLevel[LevelInfo] != uint64(len(testCases)) {
		t.Errorf("Expected %d messages logged, got %d", len(testCases), metrics.MessagesByLevel[LevelInfo])
	}
}

// TestExtremeConcurrency tests with extreme number of goroutines
func TestExtremeConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping extreme concurrency test in short mode")
	}

	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Launch extreme number of goroutines
	const numGoroutines = 1000
	done := make(chan bool, numGoroutines)

	start := time.Now()
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				logger.Info("Goroutine %d message %d", id, j)
			}
			done <- true
		}(i)
	}

	// Wait for all to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
	elapsed := time.Since(start)

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	// Check performance
	metrics := logger.GetMetrics()
	totalLogged := uint64(0)
	for _, count := range metrics.MessagesByLevel {
		totalLogged += count
	}

	expectedMessages := uint64(numGoroutines * 100)
	droppedRatio := float64(metrics.MessagesDropped) / float64(expectedMessages)

	t.Logf("Extreme concurrency test completed in %v", elapsed)
	t.Logf("Messages logged: %d, dropped: %d (%.2f%%)", totalLogged, metrics.MessagesDropped, droppedRatio*100)
	t.Logf("Average write time: %v, Max write time: %v", metrics.AverageWriteTime, metrics.MaxWriteTime)
}

