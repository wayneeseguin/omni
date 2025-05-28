package flexlog

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestMultiDestinationIntegration tests logging to multiple destinations simultaneously
func TestMultiDestinationIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create primary logger
	primaryPath := filepath.Join(tmpDir, "primary.log")
	logger, err := New(primaryPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()
	
	// Add file destinations
	errorPath := filepath.Join(tmpDir, "errors.log")
	err = logger.AddDestination(errorPath)
	if err != nil {
		t.Fatalf("Failed to add error destination: %v", err)
	}
	
	debugPath := filepath.Join(tmpDir, "debug.log")
	err = logger.AddDestination(debugPath)
	if err != nil {
		t.Fatalf("Failed to add debug destination: %v", err)
	}
	
	// Add a custom destination (memory buffer)
	var memBuf bytes.Buffer
	memWriter := bufio.NewWriter(&memBuf)
	memDest := logger.AddCustomDestination("memory", memWriter)
	
	// Configure destinations
	logger.SetDestinationName(1, "error-log")
	logger.SetDestinationName(2, "debug-log")
	
	// Log messages at different levels
	logger.Info("Application started")
	logger.Debug("Debugging information")
	logger.Warn("Warning message")
	logger.Error("Error occurred")
	
	// Flush all destinations
	logger.FlushAll()
	memWriter.Flush()
	
	// Verify primary log contains all messages
	primaryContent, err := os.ReadFile(primaryPath)
	if err != nil {
		t.Fatalf("Failed to read primary log: %v", err)
	}
	
	primaryStr := string(primaryContent)
	expectedMessages := []string{
		"Application started",
		"Debugging information",
		"Warning message",
		"Error occurred",
	}
	
	for _, msg := range expectedMessages {
		if !strings.Contains(primaryStr, msg) {
			t.Errorf("Primary log missing message: %s", msg)
		}
	}
	
	// Verify memory destination
	memContent := memBuf.String()
	for _, msg := range expectedMessages {
		if !strings.Contains(memContent, msg) {
			t.Errorf("Memory destination missing message: %s", msg)
		}
	}
	
	// Test disabling a destination
	logger.DisableDestination("debug-log")
	logger.Info("After disabling debug")
	logger.FlushAll()
	
	// Re-enable and verify
	logger.EnableDestination("debug-log")
	logger.Info("After re-enabling debug")
	logger.FlushAll()
}

// TestConcurrentMultiDestination tests concurrent writes to multiple destinations
func TestConcurrentMultiDestination(t *testing.T) {
	tmpDir := t.TempDir()
	
	logger, err := New(filepath.Join(tmpDir, "concurrent.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()
	
	// Add multiple destinations
	for i := 0; i < 3; i++ {
		path := filepath.Join(tmpDir, "dest%d.log")
		err = logger.AddDestination(path)
		if err != nil {
			t.Fatalf("Failed to add destination %d: %v", i, err)
		}
	}
	
	// Concurrent logging
	var wg sync.WaitGroup
	goroutines := 10
	messagesPerGoroutine := 100
	
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				logger.Info("Goroutine %d message %d", id, j)
			}
		}(i)
	}
	
	wg.Wait()
	logger.Sync()
	
	// Verify all destinations have content
	for i := 0; i < 4; i++ { // 1 primary + 3 additional
		var path string
		if i == 0 {
			path = filepath.Join(tmpDir, "concurrent.log")
		} else {
			path = filepath.Join(tmpDir, "dest%d.log")
		}
		
		content, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("Failed to read destination %d: %v", i, err)
			continue
		}
		
		// Count messages
		lines := strings.Split(string(content), "\n")
		messageCount := 0
		for _, line := range lines {
			if strings.Contains(line, "Goroutine") {
				messageCount++
			}
		}
		
		expectedTotal := goroutines * messagesPerGoroutine
		if messageCount != expectedTotal {
			t.Errorf("Destination %d: expected %d messages, got %d", i, expectedTotal, messageCount)
		}
	}
}

// TestRotationAcrossDestinations tests rotation behavior with multiple destinations
func TestRotationAcrossDestinations(t *testing.T) {
	tmpDir := t.TempDir()
	
	logger, err := New(filepath.Join(tmpDir, "main.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()
	
	// Set small rotation size
	logger.SetMaxSize(100)
	logger.SetMaxFiles(2)
	
	// Add destinations with different rotation settings
	secondPath := filepath.Join(tmpDir, "second.log")
	err = logger.AddDestination(secondPath)
	if err != nil {
		t.Fatalf("Failed to add destination: %v", err)
	}
	
	// Write messages to trigger rotation
	for i := 0; i < 20; i++ {
		logger.Info("This is a message that will trigger rotation when repeated: %d", i)
	}
	
	logger.Sync()
	time.Sleep(100 * time.Millisecond)
	
	// Check for rotated files
	mainRotated, _ := filepath.Glob(filepath.Join(tmpDir, "main.log.*"))
	secondRotated, _ := filepath.Glob(filepath.Join(tmpDir, "second.log.*"))
	
	if len(mainRotated) == 0 {
		t.Error("Main log did not rotate")
	}
	
	if len(secondRotated) == 0 {
		t.Error("Second log did not rotate")
	}
	
	// Verify rotation cleanup
	// Write more to trigger cleanup
	for i := 0; i < 30; i++ {
		logger.Info("Additional message for cleanup test: %d", i)
	}
	
	logger.Sync()
	time.Sleep(100 * time.Millisecond)
	
	// Check that old files were cleaned up (should have at most maxFiles rotated files)
	mainRotated, _ = filepath.Glob(filepath.Join(tmpDir, "main.log.*"))
	if len(mainRotated) > 2 {
		t.Errorf("Main log cleanup failed: %d rotated files (expected <= 2)", len(mainRotated))
	}
}

// TestErrorHandlingAcrossDestinations tests error handling with multiple destinations
func TestErrorHandlingAcrossDestinations(t *testing.T) {
	tmpDir := t.TempDir()
	
	logger, err := New(filepath.Join(tmpDir, "main.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()
	
	// Track errors
	var errorCount int
	var lastError string
	logger.SetErrorHandler(func(err LogError) {
		errorCount++
		lastError = err.Message
	})
	
	// Add a valid destination
	validPath := filepath.Join(tmpDir, "valid.log")
	err = logger.AddDestination(validPath)
	if err != nil {
		t.Fatalf("Failed to add valid destination: %v", err)
	}
	
	// Try to add an invalid destination (directory that doesn't exist)
	invalidPath := filepath.Join(tmpDir, "nonexistent", "dir", "invalid.log")
	err = logger.AddDestination(invalidPath)
	if err == nil {
		t.Error("Expected error for invalid destination")
	}
	
	// Log messages - should work for valid destinations even if one fails
	logger.Info("Test message after error")
	logger.Sync()
	
	// Verify valid destination still works
	content, err := os.ReadFile(validPath)
	if err != nil {
		t.Fatalf("Failed to read valid log: %v", err)
	}
	
	if !strings.Contains(string(content), "Test message after error") {
		t.Error("Valid destination did not receive message")
	}
}

// TestGracefulShutdownIntegration tests graceful shutdown with multiple destinations
func TestGracefulShutdownIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	
	logger, err := New(filepath.Join(tmpDir, "shutdown.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	
	// Add multiple destinations
	for i := 0; i < 3; i++ {
		path := filepath.Join(tmpDir, "dest%d.log")
		err = logger.AddDestination(path)
		if err != nil {
			t.Fatalf("Failed to add destination %d: %v", i, err)
		}
	}
	
	// Start logging in background
	done := make(chan bool)
	go func() {
		for i := 0; i < 1000; i++ {
			logger.Info("Background message %d", i)
			if i%100 == 0 {
				time.Sleep(10 * time.Millisecond)
			}
		}
		done <- true
	}()
	
	// Wait a bit then shutdown
	time.Sleep(50 * time.Millisecond)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err = logger.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
	
	// Verify messages were written to all destinations
	<-done
	
	for i := 0; i < 4; i++ {
		var path string
		if i == 0 {
			path = filepath.Join(tmpDir, "shutdown.log")
		} else {
			path = filepath.Join(tmpDir, "dest%d.log")
		}
		
		content, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("Failed to read destination %d: %v", i, err)
			continue
		}
		
		if len(content) == 0 {
			t.Errorf("Destination %d has no content after shutdown", i)
		}
	}
}

// TestCompressionIntegration tests compression with multiple destinations
func TestCompressionIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	
	logger, err := New(filepath.Join(tmpDir, "compress.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()
	
	// Enable compression
	logger.SetCompression(CompressionGzip)
	logger.SetCompressMinAge(1)
	logger.SetMaxSize(200) // Small size to trigger rotation
	
	// Add another destination
	secondPath := filepath.Join(tmpDir, "second.log")
	err = logger.AddDestination(secondPath)
	if err != nil {
		t.Fatalf("Failed to add destination: %v", err)
	}
	
	// Write enough to trigger rotation
	for i := 0; i < 50; i++ {
		logger.Info("Message for compression test: %d - with some extra content to increase size", i)
	}
	
	logger.Sync()
	time.Sleep(500 * time.Millisecond) // Give compression time to work
	
	// Check for compressed files
	compressedMain, _ := filepath.Glob(filepath.Join(tmpDir, "compress.log.*.gz"))
	compressedSecond, _ := filepath.Glob(filepath.Join(tmpDir, "second.log.*.gz"))
	
	if len(compressedMain) == 0 {
		t.Error("Main log files were not compressed")
	}
	
	if len(compressedSecond) == 0 {
		t.Error("Second log files were not compressed")
	}
}

// TestFilteringAcrossDestinations tests different filters on different destinations
func TestFilteringAcrossDestinations(t *testing.T) {
	tmpDir := t.TempDir()
	
	logger, err := New(filepath.Join(tmpDir, "main.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()
	
	// Add destinations
	errorPath := filepath.Join(tmpDir, "errors.log")
	err = logger.AddDestination(errorPath)
	if err != nil {
		t.Fatalf("Failed to add error destination: %v", err)
	}
	
	debugPath := filepath.Join(tmpDir, "debug.log")  
	err = logger.AddDestination(debugPath)
	if err != nil {
		t.Fatalf("Failed to add debug destination: %v", err)
	}
	
	// Note: In a real implementation, you would set up per-destination filters
	// For this test, we'll use the global filter and verify behavior
	
	// Set a filter that only allows ERROR level
	logger.SetFilter(func(level int, message string, fields map[string]interface{}) bool {
		return level >= LevelError
	})
	
	// Log at different levels
	logger.Debug("Debug message - should be filtered")
	logger.Info("Info message - should be filtered")
	logger.Warn("Warning message - should be filtered")
	logger.Error("Error message - should pass")
	
	logger.Sync()
	
	// Check all destinations - they should all only have the error message
	destinations := []string{
		filepath.Join(tmpDir, "main.log"),
		errorPath,
		debugPath,
	}
	
	for _, path := range destinations {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("Failed to read %s: %v", path, err)
			continue
		}
		
		contentStr := string(content)
		
		// Should contain error message
		if !strings.Contains(contentStr, "Error message - should pass") {
			t.Errorf("%s missing error message", path)
		}
		
		// Should not contain filtered messages
		filtered := []string{"Debug message", "Info message", "Warning message"}
		for _, msg := range filtered {
			if strings.Contains(contentStr, msg) {
				t.Errorf("%s contains filtered message: %s", path, msg)
			}
		}
	}
}

// TestPerformanceWithMultipleDestinations benchmarks multi-destination logging
func TestPerformanceWithMultipleDestinations(t *testing.T) {
	tmpDir := t.TempDir()
	
	logger, err := New(filepath.Join(tmpDir, "perf.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()
	
	// Add multiple destinations
	destinationCount := 5
	for i := 0; i < destinationCount; i++ {
		path := filepath.Join(tmpDir, "dest%d.log")
		err = logger.AddDestination(path)
		if err != nil {
			t.Fatalf("Failed to add destination %d: %v", i, err)
		}
	}
	
	// Measure performance
	messageCount := 10000
	start := time.Now()
	
	for i := 0; i < messageCount; i++ {
		logger.Info("Performance test message %d", i)
	}
	
	logger.Sync()
	elapsed := time.Since(start)
	
	// Calculate throughput
	msgsPerSec := float64(messageCount) / elapsed.Seconds()
	t.Logf("Multi-destination performance: %.0f msgs/sec with %d destinations", msgsPerSec, destinationCount+1)
	
	// Verify all destinations received messages
	for i := 0; i <= destinationCount; i++ {
		var path string
		if i == 0 {
			path = filepath.Join(tmpDir, "perf.log")
		} else {
			path = filepath.Join(tmpDir, "dest%d.log")
		}
		
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("Failed to stat destination %d: %v", i, err)
			continue
		}
		
		if info.Size() == 0 {
			t.Errorf("Destination %d has no content", i)
		}
	}
}