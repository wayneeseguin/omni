package flexlog

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
	}
}

// TestDestinationEnableDisable tests enabling and disabling destinations
func TestDestinationEnableDisable(t *testing.T) {
	tempDir := t.TempDir()
	file1 := filepath.Join(tempDir, "main.log")
	file2 := filepath.Join(tempDir, "secondary.log")
	
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
	
	logger.SetDestinationEnabled(1, false)
	
	// Log with second destination disabled
	logger.Info("Message 2")
	time.Sleep(50 * time.Millisecond)
	if err := logger.FlushAll(); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	
	// Re-enable and log again
	logger.SetDestinationEnabled(1, true)
	logger.Info("Message 3")
	time.Sleep(50 * time.Millisecond)
	if err := logger.FlushAll(); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	
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
}

// TestConcurrentMultiDestination tests concurrent logging to multiple destinations
func TestConcurrentMultiDestination(t *testing.T) {
	tempDir := t.TempDir()
	
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
	
	// Launch concurrent goroutines
	const numGoroutines = 10
	const messagesPerGoroutine = 50
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				logger.Info(fmt.Sprintf("Goroutine %d Message %d", id, j))
			}
		}(i)
	}
	
	wg.Wait()
	
	// Flush and verify
	if err := logger.FlushAll(); err != nil {
		t.Fatalf("Failed to flush: %v", err)
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
	
	// Add a destination that will fail (non-existent directory)
	badFile := filepath.Join(tempDir, "nonexistent", "bad.log")
	err = logger.AddDestination(badFile)
	if err == nil {
		t.Log("Warning: Expected AddDestination to fail, but it didn't")
	}
	
	// Log messages - should still work for good destination
	logger.Info("Test message 1")
	logger.Info("Test message 2")
	
	if err := logger.FlushAll(); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}
	
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
	
	// Log some messages
	for i := 0; i < 10; i++ {
		logger.Info(fmt.Sprintf("Metrics test message %d", i))
	}
	
	if err := logger.FlushAll(); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}
	
	// Get final metrics
	finalMetrics := logger.GetMetrics()
	
	// Verify metrics increased
	finalTotal := uint64(0)
	initialTotal := uint64(0)
	for _, count := range finalMetrics.MessagesLogged {
		finalTotal += count
	}
	for _, count := range initialMetrics.MessagesLogged {
		initialTotal += count
	}
	
	if finalTotal <= initialTotal {
		t.Error("Message count should have increased")
	}
	
	if finalMetrics.BytesWritten <= initialMetrics.BytesWritten {
		t.Error("Bytes written should have increased")
	}
	
	// Should have metrics for multiple destinations
	if len(finalMetrics.Destinations) < 2 {
		t.Errorf("Expected metrics for at least 2 destinations, got %d", len(finalMetrics.Destinations))
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