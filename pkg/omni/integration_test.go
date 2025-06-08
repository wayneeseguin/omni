package omni

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestMultiDestinationLogging tests logging to multiple destinations simultaneously
func TestMultiDestinationLogging(t *testing.T) {
	tempDir := t.TempDir()

	// Create multiple log files
	file1 := filepath.Join(tempDir, "app.log")
	file2 := filepath.Join(tempDir, "debug.log")
	file3 := filepath.Join(tempDir, "error.log")

	if testing.Verbose() {
		t.Logf("Creating logger with destinations:")
		t.Logf("  - Primary: %s", file1)
		t.Logf("  - Debug: %s", file2)
		t.Logf("  - Error: %s", file3)
	}

	// Create logger with first destination
	logger, err := New(file1)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Add additional destinations
	if err := logger.AddDestination(file2); err != nil {
		t.Fatalf("Failed to add destination 2: %v", err)
	}

	if err := logger.AddDestination(file3); err != nil {
		t.Fatalf("Failed to add destination 3: %v", err)
	}

	// Set debug level to ensure all messages are logged
	logger.SetLevel(LevelDebug)

	if testing.Verbose() {
		t.Log("Logging test messages...")
	}

	// Log messages of different levels
	logger.Info("Application started")
	logger.Debug("Debug information")
	logger.Warn("Warning message")
	logger.Error("Error occurred")
	logger.Info("Processing complete")

	// Give time for async processing
	time.Sleep(100 * time.Millisecond)

	// Flush all destinations
	if err := logger.FlushAll(); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	// Give additional time for writes to complete
	time.Sleep(100 * time.Millisecond)

	if testing.Verbose() {
		t.Log("Verifying file contents...")
	}

	// Verify all files have content
	files := []string{file1, file2, file3}
	for i, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Errorf("Failed to read file %d (%s): %v", i+1, file, err)
			continue
		}

		if len(content) == 0 {
			t.Errorf("File %d (%s) is empty", i+1, file)
		}

		// Check that it contains some of our test messages
		contentStr := string(content)
		if !strings.Contains(contentStr, "Application started") {
			t.Errorf("File %d (%s) missing expected content", i+1, file)
		}

		if testing.Verbose() {
			lines := strings.Count(contentStr, "\n")
			t.Logf("  - %s: %d bytes, %d lines", filepath.Base(file), len(content), lines)
		}
	}

	if testing.Verbose() {
		t.Log("Multi-destination logging test completed successfully")
	}
}

// TestDestinationEnableDisable tests enabling and disabling destinations
func TestDestinationEnableDisable(t *testing.T) {
	tempDir := t.TempDir()
	file1 := filepath.Join(tempDir, "main.log")
	file2 := filepath.Join(tempDir, "secondary.log")

	if testing.Verbose() {
		t.Log("Testing destination enable/disable functionality")
		t.Logf("  - Main log: %s", file1)
		t.Logf("  - Secondary log: %s", file2)
	}

	logger, err := New(file1)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Add second destination
	if err := logger.AddDestination(file2); err != nil {
		t.Fatalf("Failed to add destination: %v", err)
	}

	// Set debug level to ensure all messages are logged
	logger.SetLevel(LevelDebug)

	// Log with both destinations enabled
	if testing.Verbose() {
		t.Log("Phase 1: Logging with both destinations enabled")
	}
	logger.Info("Message 1")
	time.Sleep(50 * time.Millisecond)
	if err := logger.FlushAll(); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	// Disable second destination
	destinations := logger.ListDestinations()
	if len(destinations) < 2 {
		t.Fatalf("Expected at least 2 destinations, got %d", len(destinations))
	}

	if testing.Verbose() {
		t.Log("Phase 2: Disabling secondary destination")
	}
	// Disable second destination
	if err := logger.SetDestinationEnabled(1, false); err != nil {
		t.Fatalf("Failed to disable destination: %v", err)
	}

	// Log with second destination disabled
	logger.Info("Message 2")
	time.Sleep(50 * time.Millisecond)
	if err := logger.FlushAll(); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	if testing.Verbose() {
		t.Log("Phase 3: Re-enabling secondary destination")
	}
	// Re-enable and log again
	if err := logger.SetDestinationEnabled(1, true); err != nil {
		t.Fatalf("Failed to re-enable destination: %v", err)
	}
	logger.Info("Message 3")
	time.Sleep(50 * time.Millisecond)
	if err := logger.FlushAll(); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	if testing.Verbose() {
		t.Log("Verifying file contents...")
	}

	// Verify file contents
	content1, err := os.ReadFile(file1)
	if err != nil {
		t.Fatalf("Failed to read file1: %v", err)
	}

	content2, err := os.ReadFile(file2)
	if err != nil {
		t.Fatalf("Failed to read file2: %v", err)
	}

	// File1 should have all messages
	str1 := string(content1)
	for _, msg := range []string{"Message 1", "Message 2", "Message 3"} {
		if !strings.Contains(str1, msg) {
			t.Errorf("File1 missing message: %s", msg)
		}
	}

	// File2 should be missing "Message 2" (when disabled)
	str2 := string(content2)
	if !strings.Contains(str2, "Message 1") {
		t.Error("File2 missing Message 1")
	}
	if strings.Contains(str2, "Message 2") {
		t.Error("File2 should not contain Message 2 (was disabled)")
	}
	if !strings.Contains(str2, "Message 3") {
		t.Error("File2 missing Message 3")
	}

	if testing.Verbose() {
		t.Logf("Main log has %d messages", strings.Count(str1, "Message"))
		t.Logf("Secondary log has %d messages (should be 2)", strings.Count(str2, "Message"))
		t.Log("Destination enable/disable test completed successfully")
	}
}

// TestConcurrentMultiDestination tests concurrent logging to multiple destinations
func TestConcurrentMultiDestination(t *testing.T) {
	tempDir := t.TempDir()

	if testing.Verbose() {
		t.Log("Testing concurrent logging to multiple destinations")
	}

	// Set a larger channel size for this test
	oldSize := os.Getenv("OMNI_CHANNEL_SIZE")
	os.Setenv("OMNI_CHANNEL_SIZE", "1000")
	defer func() {
		if oldSize != "" {
			os.Setenv("OMNI_CHANNEL_SIZE", oldSize)
		} else {
			os.Unsetenv("OMNI_CHANNEL_SIZE")
		}
	}()

	// Create logger with multiple destinations
	logger, err := New(filepath.Join(tempDir, "concurrent1.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Add more destinations
	for i := 2; i <= 4; i++ {
		file := filepath.Join(tempDir, fmt.Sprintf("concurrent%d.log", i))
		if err := logger.AddDestination(file); err != nil {
			t.Fatalf("Failed to add destination %d: %v", i, err)
		}
	}

	if testing.Verbose() {
		t.Logf("Created logger with %d destinations", 4)
	}

	// Launch concurrent goroutines
	const numGoroutines = 10
	const messagesPerGoroutine = 50

	if testing.Verbose() {
		t.Logf("Launching %d goroutines, each writing %d messages", numGoroutines, messagesPerGoroutine)
	}

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	start := time.Now()
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				logger.Info(fmt.Sprintf("Goroutine %d Message %d", id, j))
			}
		}(i)
	}

	wg.Wait()

	if testing.Verbose() {
		t.Logf("All goroutines completed in %v", time.Since(start))
	}

	// Give time for messages to be processed - reduced for tests
	time.Sleep(50 * time.Millisecond)

	// Flush and verify
	if err := logger.FlushAll(); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	// Check metrics first
	metrics := logger.GetMetrics()
	if testing.Verbose() {
		t.Log("Logger metrics:")
		t.Logf("  - Messages logged: %+v", metrics.MessagesLogged)
		t.Logf("  - Messages dropped: %d", metrics.MessagesDropped)
		t.Logf("  - Error count: %d", metrics.ErrorCount)
		t.Logf("  - Bytes written: %d", metrics.BytesWritten)
	}

	// Check that all files have expected number of lines
	expectedLines := numGoroutines * messagesPerGoroutine
	for i := 1; i <= 4; i++ {
		file := filepath.Join(tempDir, fmt.Sprintf("concurrent%d.log", i))
		content, err := os.ReadFile(file)
		if err != nil {
			t.Errorf("Failed to read file %d: %v", i, err)
			continue
		}

		lines := strings.Split(strings.TrimSpace(string(content)), "\n")
		if len(lines) < expectedLines {
			t.Errorf("File %d has %d lines, expected at least %d", i, len(lines), expectedLines)
		}

		if testing.Verbose() {
			t.Logf("  - concurrent%d.log: %d lines, %d bytes", i, len(lines), len(content))
		}
	}

	if testing.Verbose() {
		t.Log("Concurrent multi-destination test completed successfully")
	}
}

// TestDestinationFailureRecovery tests behavior when a destination fails
func TestDestinationFailureRecovery(t *testing.T) {
	tempDir := t.TempDir()

	// Create logger
	goodFile := filepath.Join(tempDir, "good.log")
	logger, err := New(goodFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Create a read-only directory to test failure handling
	readOnlyDir := filepath.Join(tempDir, "readonly")
	if err := os.Mkdir(readOnlyDir, 0555); err != nil {
		t.Fatalf("Failed to create read-only directory: %v", err)
	}
	badFile := filepath.Join(readOnlyDir, "bad.log")
	
	// Try to add destination - this should fail due to permissions
	err = logger.AddDestination(badFile)
	if err != nil {
		t.Logf("AddDestination failed as expected: %v", err)
	} else {
		t.Log("AddDestination succeeded unexpectedly")
	}

	// Log messages - should still work for good destination
	logger.Info("Test message 1")
	logger.Info("Test message 2")

	// Give time for messages to be processed
	time.Sleep(100 * time.Millisecond)

	if err := logger.FlushAll(); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	// Additional wait after flush
	time.Sleep(100 * time.Millisecond)

	// Verify good file has content
	content, err := os.ReadFile(goodFile)
	if err != nil {
		t.Fatalf("Failed to read good file: %v", err)
	}

	if len(content) == 0 {
		t.Error("Good file should have content")
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "Test message 1") {
		t.Error("Missing test message 1")
	}
	if !strings.Contains(contentStr, "Test message 2") {
		t.Error("Missing test message 2")
	}
}

// TestFormattingConsistency tests that formatting is consistent across destinations
func TestFormattingConsistency(t *testing.T) {
	tempDir := t.TempDir()

	// Create logger
	logger, err := New(filepath.Join(tempDir, "format1.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set JSON format and debug level
	logger.SetFormat(FormatJSON)
	logger.SetLevel(LevelDebug)

	// Add second destination
	if err := logger.AddDestination(filepath.Join(tempDir, "format2.log")); err != nil {
		t.Fatalf("Failed to add destination: %v", err)
	}

	// Log structured message
	logger.InfoWithFields("User logged in", map[string]interface{}{
		"user_id": 123,
		"action":  "login",
		"ip":      "192.168.1.1",
	})

	time.Sleep(100 * time.Millisecond)
	if err := logger.FlushAll(); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	// Read both files and compare
	content1, err := os.ReadFile(filepath.Join(tempDir, "format1.log"))
	if err != nil {
		t.Fatalf("Failed to read format1.log: %v", err)
	}

	content2, err := os.ReadFile(filepath.Join(tempDir, "format2.log"))
	if err != nil {
		t.Fatalf("Failed to read format2.log: %v", err)
	}

	// Both should be identical
	if string(content1) != string(content2) {
		t.Error("Destination outputs should be identical")
		t.Logf("File 1:\n%s", content1)
		t.Logf("File 2:\n%s", content2)
	}

	// Both should be valid JSON
	if !strings.Contains(string(content1), `"user_id":123`) {
		t.Error("Missing expected JSON field in file 1")
	}
	if !strings.Contains(string(content2), `"user_id":123`) {
		t.Error("Missing expected JSON field in file 2")
	}
}

// TestDestinationMetrics tests metrics collection for multiple destinations
func TestDestinationMetrics(t *testing.T) {
	tempDir := t.TempDir()

	if testing.Verbose() {
		t.Log("Testing metrics collection for multiple destinations")
	}

	// Create logger with multiple destinations
	logger, err := New(filepath.Join(tempDir, "metrics1.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Add second destination
	if err := logger.AddDestination(filepath.Join(tempDir, "metrics2.log")); err != nil {
		t.Fatalf("Failed to add destination: %v", err)
	}

	// Get initial metrics
	initialMetrics := logger.GetMetrics()

	if testing.Verbose() {
		t.Log("Initial metrics:")
		t.Logf("  - Messages logged: %v", initialMetrics.MessagesLogged)
		t.Logf("  - Bytes written: %d", initialMetrics.BytesWritten)
		t.Logf("  - Active destinations: %d", initialMetrics.ActiveDestinations)
	}

	// Log some messages
	for i := 0; i < 10; i++ {
		logger.Info(fmt.Sprintf("Metrics test message %d", i))
	}

	// Wait for messages to be processed
	time.Sleep(100 * time.Millisecond)

	if err := logger.FlushAll(); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	// Get final metrics
	finalMetrics := logger.GetMetrics()

	if testing.Verbose() {
		t.Log("Final metrics:")
		t.Logf("  - Messages logged: %v", finalMetrics.MessagesLogged)
		t.Logf("  - Bytes written: %d", finalMetrics.BytesWritten)
		t.Logf("  - Messages dropped: %d", finalMetrics.MessagesDropped)
		t.Logf("  - Error count: %d", finalMetrics.ErrorCount)
		t.Logf("  - Active destinations: %d", finalMetrics.ActiveDestinations)
	}

	// Verify metrics increased
	if finalMetrics.MessagesLogged <= initialMetrics.MessagesLogged {
		t.Error("Message count should have increased")
	}

	if finalMetrics.BytesWritten <= initialMetrics.BytesWritten {
		t.Error("Bytes written should have increased")
	}

	// Should have metrics for multiple destinations
	// Note: Destinations field is not available in LoggerMetrics
	if finalMetrics.ActiveDestinations < 2 {
		t.Errorf("Expected at least 2 active destinations, got %d", finalMetrics.ActiveDestinations)
	}

	if testing.Verbose() {
		t.Log("Metrics test completed successfully")
	}
}

// TestGracefulShutdownMultiDestination tests graceful shutdown with multiple destinations
func TestGracefulShutdownMultiDestination(t *testing.T) {
	tempDir := t.TempDir()

	// Create logger with multiple destinations
	logger, err := New(filepath.Join(tempDir, "shutdown1.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Add more destinations
	for i := 2; i <= 3; i++ {
		file := filepath.Join(tempDir, fmt.Sprintf("shutdown%d.log", i))
		if err := logger.AddDestination(file); err != nil {
			t.Fatalf("Failed to add destination %d: %v", i, err)
		}
	}

	// Log some messages
	for i := 0; i < 5; i++ {
		logger.Info(fmt.Sprintf("Shutdown test message %d", i))
	}

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := logger.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	// Verify all files exist and have content
	for i := 1; i <= 3; i++ {
		file := filepath.Join(tempDir, fmt.Sprintf("shutdown%d.log", i))
		content, err := os.ReadFile(file)
		if err != nil {
			t.Errorf("Failed to read shutdown file %d: %v", i, err)
			continue
		}

		if len(content) == 0 {
			t.Errorf("Shutdown file %d is empty", i)
		}
	}

	// Verify logger is closed
	if !logger.IsClosed() {
		t.Error("Logger should be marked as closed after shutdown")
	}
}
