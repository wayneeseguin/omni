package omni

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wayneeseguin/omni/pkg/formatters"
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

	// Sync to ensure all messages in the channel are processed
	if err := logger.Sync(); err != nil {
		t.Fatalf("Failed to sync: %v", err)
	}

	// Flush and verify
	if err := logger.FlushAll(); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	// Give time for flush to complete
	time.Sleep(50 * time.Millisecond)

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

// TestWithFields tests the WithFields functionality
func TestWithFields(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "with_fields.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test WithFields
	fields := map[string]interface{}{
		"user_id":   123,
		"action":    "login",
		"ip":        "192.168.1.1",
		"timestamp": time.Now(),
	}

	fieldsLogger := logger.WithFields(fields)
	if fieldsLogger == nil {
		t.Fatal("WithFields returned nil")
	}

	fieldsLogger.Info("User logged in")

	// Test WithField
	fieldLogger := logger.WithField("session_id", "abc123")
	if fieldLogger == nil {
		t.Fatal("WithField returned nil")
	}

	fieldLogger.Info("Session created")

	// Test WithError
	testErr := errors.New("test error")
	errorLogger := logger.WithError(testErr)
	if errorLogger == nil {
		t.Fatal("WithError returned nil")
	}

	errorLogger.Error("Operation failed")

	// Test WithError with nil
	nilErrorLogger := logger.WithError(nil)
	if nilErrorLogger != logger {
		t.Error("WithError(nil) should return the same logger")
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Verify log contents
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)
	expectedMessages := []string{
		"User logged in",
		"Session created", 
		"Operation failed",
	}

	for _, expected := range expectedMessages {
		if !strings.Contains(logContent, expected) {
			t.Errorf("Expected to find '%s' in log output", expected)
		}
	}
}

// TestFlushOperations tests various flush operations
func TestFlushOperations(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "flush_test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Add multiple destinations
	dest2 := filepath.Join(tmpDir, "dest2.log")
	dest3 := filepath.Join(tmpDir, "dest3.log")
	logger.AddDestination(dest2)
	logger.AddDestination(dest3)

	// Log some messages
	for i := 0; i < 10; i++ {
		logger.Infof("Flush test message %d", i)
	}

	// Test Flush (single destination)
	err = logger.Flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	// Test FlushAll
	err = logger.FlushAll()
	if err != nil {
		t.Errorf("FlushAll failed: %v", err)
	}

	// Test Sync
	err = logger.Sync()
	if err != nil {
		t.Errorf("Sync failed: %v", err)
	}

	// Verify all destinations have content
	destinations := []string{logFile, dest2, dest3}
	for i, dest := range destinations {
		content, err := os.ReadFile(dest)
		if err != nil {
			t.Errorf("Failed to read destination %d: %v", i, err)
			continue
		}

		if !strings.Contains(string(content), "Flush test message") {
			t.Errorf("Destination %d missing expected content", i)
		}
	}
}

// TestGetErrors tests error tracking and retrieval
func TestGetErrors(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "errors_test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Get initial errors (should be empty)
	errors := logger.GetErrors()
	if len(errors) != 0 {
		t.Errorf("Expected 0 initial errors, got %d", len(errors))
	}

	// Cause an error by closing the file
	logger.mu.RLock()
	var dest *Destination
	if logger.defaultDest != nil {
		dest = logger.defaultDest
	} else if len(logger.Destinations) > 0 {
		dest = logger.Destinations[0]
	}
	logger.mu.RUnlock()

	if dest != nil {
		dest.mu.Lock()
		if dest.File != nil {
			dest.File.Close()
		}
		dest.mu.Unlock()

		// Try to log (should cause error)
		logger.Info("This should cause an error")
		time.Sleep(100 * time.Millisecond)

		// Check errors
		errorChan := logger.GetErrors()
		errorCount := 0
		
		// Non-blocking read from error channel
		for {
			select {
			case err := <-errorChan:
				errorCount++
				t.Logf("Error: %v", err)
			default:
				goto checkCount
			}
		}
		
		checkCount:
		if errorCount == 0 {
			t.Error("Expected errors to be recorded")
		}

		t.Logf("Recorded %d errors", errorCount)
	}
}

// TestFilterOperations tests filter management
func TestFilterOperations(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "filter_test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Add a filter that blocks messages containing "blocked"
	logger.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
		return !strings.Contains(message, "blocked")
	})

	// Log messages
	logger.Info("This message should appear")
	logger.Info("This message is blocked")
	logger.Info("Another normal message")

	time.Sleep(100 * time.Millisecond)

	// Check log content
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)
	if !strings.Contains(logContent, "This message should appear") {
		t.Error("Expected unfiltered message to appear")
	}
	if !strings.Contains(logContent, "Another normal message") {
		t.Error("Expected second unfiltered message to appear")
	}
	if strings.Contains(logContent, "This message is blocked") {
		t.Error("Expected blocked message to not appear")
	}

	// Test ClearFilters
	logger.ClearFilters()

	// Log blocked message again (should now appear)
	logger.Info("Previously blocked message")
	time.Sleep(100 * time.Millisecond)

	// Check log content again
	content, err = os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "Previously blocked message") {
		t.Error("Expected message to appear after clearing filters")
	}

	// Test RemoveFilter (need to track filter reference)
	filterFunc := func(level int, message string, fields map[string]interface{}) bool {
		return level >= LevelWarn // Only warnings and errors
	}
	logger.AddFilter(filterFunc)

	logger.Info("This info should be filtered")
	logger.Warn("This warning should appear")
	time.Sleep(100 * time.Millisecond)

	// Remove the filter
	logger.RemoveFilter(filterFunc)

	logger.Info("This info should now appear")
	time.Sleep(100 * time.Millisecond)

	content, err = os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent = string(content)
	if !strings.Contains(logContent, "This warning should appear") {
		t.Error("Expected warning to appear with filter")
	}
	if !strings.Contains(logContent, "This info should now appear") {
		t.Error("Expected info to appear after removing filter")
	}
}

// TestSamplingOperations tests sampling functionality
func TestSamplingOperations(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "sampling_test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test SetSampling with 50% rate (using rate-based sampling)
	logger.SetSampling(1, 0.5) // Strategy 1 (rate-based), 50% rate

	// Check sampling rate
	rate := logger.GetSamplingRate()
	if rate != 0.5 {
		t.Errorf("Expected sampling rate 0.5, got %f", rate)
	}

	// Log many identical messages (should be sampled)
	numMessages := 100
	for i := 0; i < numMessages; i++ {
		logger.Info("Sampled message") // Same message = same key
	}

	time.Sleep(200 * time.Millisecond)

	// Check log content
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	messageCount := strings.Count(string(content), "Sampled message")
	t.Logf("Logged %d out of %d sampled messages", messageCount, numMessages)

	// With 50% sampling, should have significantly fewer than all messages
	if messageCount >= numMessages {
		t.Error("Expected sampling to reduce message count")
	}

	// But should have at least some messages
	if messageCount == 0 {
		t.Error("Expected at least some messages to pass sampling")
	}
}

// TestSetErrorHandlerFunc tests setting error handler via function
func TestSetErrorHandlerFunc(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "error_handler_func.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Track errors with function handler
	var capturedErrors []LogError
	var mu sync.Mutex

	logger.SetErrorHandlerFunc(func(source, destination, message string, err error) {
		mu.Lock()
		capturedErrors = append(capturedErrors, LogError{
			Operation:   source,
			Destination: destination,
			Message:     message,
			Err:         err,
			Timestamp:   time.Now(),
		})
		mu.Unlock()
	})

	// Cause an error
	logger.mu.RLock()
	var dest *Destination
	if logger.defaultDest != nil {
		dest = logger.defaultDest
	} else if len(logger.Destinations) > 0 {
		dest = logger.Destinations[0]
	}
	logger.mu.RUnlock()

	if dest != nil {
		dest.mu.Lock()
		if dest.File != nil {
			dest.File.Close()
		}
		dest.mu.Unlock()

		logger.Info("This should trigger error handler func")
		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		errorCount := len(capturedErrors)
		mu.Unlock()

		if errorCount == 0 {
			t.Error("Expected error handler function to be called")
		} else {
			t.Logf("Error handler function captured %d errors", errorCount)
		}
	}
}

// TestGetLastError tests getting the most recent error
func TestGetLastError(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "last_error_test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Initially should have no error
	lastError := logger.GetLastError()
	if lastError != nil && lastError.Message != "" {
		t.Error("Expected no initial error")
	}

	// Cause an error
	logger.mu.RLock()
	var dest *Destination
	if logger.defaultDest != nil {
		dest = logger.defaultDest
	} else if len(logger.Destinations) > 0 {
		dest = logger.Destinations[0]
	}
	logger.mu.RUnlock()

	if dest != nil {
		dest.mu.Lock()
		if dest.File != nil {
			dest.File.Close()
		}
		dest.mu.Unlock()

		logger.Info("This should cause an error")
		time.Sleep(100 * time.Millisecond)

		// Check last error
		lastError = logger.GetLastError()
		if lastError == nil || lastError.Message == "" {
			t.Error("Expected last error to be recorded")
		} else {
			t.Logf("Last error: %s", lastError.Message)
		}
	}
}

// TestGetErrorCount tests error counting
func TestGetErrorCount(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "error_count_test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Initially should have zero errors
	initialCount := logger.GetErrorCount()
	if initialCount != 0 {
		t.Errorf("Expected 0 initial errors, got %d", initialCount)
	}

	// Cause multiple errors
	logger.mu.RLock()
	var dest *Destination
	if logger.defaultDest != nil {
		dest = logger.defaultDest
	} else if len(logger.Destinations) > 0 {
		dest = logger.Destinations[0]
	}
	logger.mu.RUnlock()

	if dest != nil {
		dest.mu.Lock()
		if dest.File != nil {
			dest.File.Close()
		}
		dest.mu.Unlock()

		// Log multiple messages to cause multiple errors
		for i := 0; i < 5; i++ {
			logger.Infof("Error test message %d", i)
		}

		time.Sleep(200 * time.Millisecond)

		// Check error count
		finalCount := logger.GetErrorCount()
		if finalCount == 0 {
			t.Error("Expected errors to be counted")
		} else {
			t.Logf("Error count: %d", finalCount)
		}

		// Error count should be greater than initial
		if finalCount <= initialCount {
			t.Errorf("Expected error count to increase from %d to %d", initialCount, finalCount)
		}
	}
}

// TestSetFormatterOperation tests setting custom formatters
func TestSetFormatterOperation(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "formatter_test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Create a custom JSON formatter
	jsonFormatter := formatters.NewJSONFormatter()

	// Set the formatter
	logger.SetFormatter(jsonFormatter)

	// Get the formatter to verify it was set
	currentFormatter := logger.GetFormatter()
	if currentFormatter == nil {
		t.Error("Expected formatter to be set")
	}

	// Log a message
	logger.Info("Formatter test message")
	time.Sleep(100 * time.Millisecond)

	// Check log content for JSON format
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)
	if !strings.Contains(logContent, "{") || !strings.Contains(logContent, "}") {
		t.Error("Expected JSON format in log output")
	}

	if !strings.Contains(logContent, "Formatter test message") {
		t.Error("Expected message content in log output")
	}
}

// TestStructuredLoggingEnhanced tests enhanced structured logging functionality
func TestStructuredLoggingEnhanced(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "structured_test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()
	
	// Set level to trace to ensure all messages are logged
	logger.SetLevel(LevelTrace)

	// Test StructuredLog
	fields := map[string]interface{}{
		"user_id":    123,
		"action":     "purchase",
		"amount":     99.99,
		"product_id": "PROD-456",
	}

	logger.StructuredLog(LevelInfo, "Purchase completed", fields)

	// Test TraceWithFields
	logger.TraceWithFields("Trace with fields", map[string]interface{}{
		"trace_id": "abc123",
		"span_id":  "def456",
	})

	// Test DebugWithFields
	logger.DebugWithFields("Debug with fields", map[string]interface{}{
		"debug_info": "detailed info",
		"context":    "test",
	})

	// Test InfoWithFields (already has some coverage)
	logger.InfoWithFields("Info with fields", map[string]interface{}{
		"info_level": "standard",
	})

	// Test WarnWithFields
	logger.WarnWithFields("Warning with fields", map[string]interface{}{
		"warning_type": "performance",
		"threshold":    "exceeded",
	})

	// Test ErrorWithFields
	logger.ErrorWithFields("Error with fields", map[string]interface{}{
		"error_code": "E001",
		"severity":   "high",
	})

	time.Sleep(100 * time.Millisecond)

	// Verify log content
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)
	expectedMessages := []string{
		"Purchase completed",
		"Trace with fields",
		"Debug with fields", 
		"Info with fields",
		"Warning with fields",
		"Error with fields",
	}

	for _, expected := range expectedMessages {
		if !strings.Contains(logContent, expected) {
			t.Errorf("Expected to find '%s' in log output", expected)
		}
	}
}

// TestNewContextLogger tests context logger creation
func TestNewContextLogger(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "context_logger_test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Create context logger
	ctx := context.Background()
	contextLogger := NewContextLogger(logger, ctx)
	if contextLogger == nil {
		t.Fatal("NewContextLogger returned nil")
	}

	// Use context logger
	contextLogger.Info("Context logger test message")
	contextLogger.Error("Context logger error message")

	time.Sleep(100 * time.Millisecond)

	// Verify log content
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)
	if !strings.Contains(logContent, "Context logger test message") {
		t.Error("Expected context logger test message")
	}
	if !strings.Contains(logContent, "Context logger error message") {
		t.Error("Expected context logger error message")
	}
}

// TestCloseAllOperation tests the CloseAll functionality
func TestCloseAllOperation(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "close_all_test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Add multiple destinations
	for i := 0; i < 3; i++ {
		destFile := filepath.Join(tmpDir, fmt.Sprintf("dest_%d.log", i))
		err = logger.AddDestination(destFile)
		if err != nil {
			t.Fatalf("Failed to add destination %d: %v", i, err)
		}
	}

	// Log some messages
	logger.Info("CloseAll test message")
	time.Sleep(50 * time.Millisecond)

	// CloseAll should close all destinations
	err = logger.CloseAll()
	if err != nil {
		t.Errorf("CloseAll failed: %v", err)
	}

	// Verify logger is closed
	if !logger.IsClosed() {
		t.Error("Logger should be closed after CloseAll")
	}

	// Verify all destinations are removed
	destinations := logger.ListDestinations()
	if len(destinations) != 0 {
		t.Errorf("Expected 0 destinations after CloseAll, got %d", len(destinations))
	}
}

// TestRecoverFromError tests error recovery functionality
func TestRecoverFromError(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "recovery_test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Create a message and destination for testing
	msg := LogMessage{
		Level:     LevelInfo,
		Format:    "test message",
		Timestamp: time.Now(),
	}

	// Get a destination
	logger.mu.RLock()
	var dest *Destination
	if logger.defaultDest != nil {
		dest = logger.defaultDest
	} else if len(logger.Destinations) > 0 {
		dest = logger.Destinations[0]
	}
	logger.mu.RUnlock()

	if dest != nil {
		// Test RecoverFromError method
		testErr := errors.New("test error")
		logger.RecoverFromError(testErr, msg, dest)
		
		// If we get here without panic, the test passes
		t.Log("RecoverFromError executed successfully")
	}
}

// TestNewOmniError tests OmniError creation
func TestNewOmniError(t *testing.T) {
	// Test creating OmniError
	code := ErrCodeInvalidConfig
	source := "test_source"
	path := "/test/path"
	innerErr := errors.New("inner error")

	omniErr := NewOmniError(code, source, path, innerErr)
	if omniErr.Message == "" {
		t.Error("NewOmniError should have a message")
	}

	// Test with context
	contextErr := omniErr.WithContext("key", "value")
	if contextErr.Message == "" {
		t.Error("WithContext should return error with context")
	}

	// Test WithDestination
	destErr := omniErr.WithDestination("test_destination")
	if destErr.Message == "" {
		t.Error("WithDestination should return error with destination")
	}
}
