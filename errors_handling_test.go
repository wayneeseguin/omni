package omni

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestErrorHandling(t *testing.T) {
	// Create logger
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"
	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Create a channel to capture errors
	errorChan := make(chan LogError, 10)
	logger.SetErrorHandler(ChannelErrorHandler(errorChan))

	// Force an error by closing the file
	logger.Destinations[0].File.Close()

	// Try to log something
	logger.Info("This should fail")

	// Wait for error
	select {
	case err := <-errorChan:
		// Could be write or flush error
		if err.Source != "write" && err.Source != "flush" {
			t.Errorf("Expected write or flush error, got %s", err.Source)
		}
		if err.Err == nil {
			t.Error("Expected non-nil error")
		}
		t.Logf("Got expected error: %s", err.Error())
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for error")
	}
}

func TestGetErrorCount(t *testing.T) {
	// Create logger
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"
	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Initial error count should be 0
	if count := logger.GetErrorCount(); count != 0 {
		t.Errorf("Expected 0 errors, got %d", count)
	}

	// Force an error
	logger.SetErrorHandler(SilentErrorHandler)
	logger.Destinations[0].File.Close()
	logger.Info("This should fail")

	// Give time for error to be processed
	time.Sleep(100 * time.Millisecond)

	// Error count should increase
	if count := logger.GetErrorCount(); count == 0 {
		t.Error("Expected error count to increase")
	}
}

func TestMultiErrorHandler(t *testing.T) {
	// Create logger
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"
	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Track errors from multiple handlers
	var mu sync.Mutex
	var errors1, errors2 []LogError

	handler1 := func(err LogError) {
		mu.Lock()
		errors1 = append(errors1, err)
		mu.Unlock()
	}

	handler2 := func(err LogError) {
		mu.Lock()
		errors2 = append(errors2, err)
		mu.Unlock()
	}

	// Set multi handler
	logger.SetErrorHandler(MultiErrorHandler(handler1, handler2))

	// Force an error
	logger.Destinations[0].File.Close()
	logger.Info("This should fail")

	// Wait for errors
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(errors1) == 0 {
		t.Error("Handler 1 didn't receive error")
	}
	if len(errors2) == 0 {
		t.Error("Handler 2 didn't receive error")
	}
}

func TestThresholdErrorHandler(t *testing.T) {
	// Create logger
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"
	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Capture only high severity errors
	var capturedErrors []LogError
	handler := func(err LogError) {
		capturedErrors = append(capturedErrors, err)
	}

	logger.SetErrorHandler(ThresholdErrorHandler(ErrorLevelHigh, handler))

	// Test low severity error (should not be captured)
	logger.logError("test", "", "Low severity error", nil, ErrorLevelLow)

	// Test high severity error (should be captured)
	logger.logError("test", "", "High severity error", nil, ErrorLevelHigh)

	// Give time for processing
	time.Sleep(50 * time.Millisecond)

	if len(capturedErrors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(capturedErrors))
	}
	if len(capturedErrors) > 0 && capturedErrors[0].Message != "High severity error" {
		t.Errorf("Wrong error captured: %s", capturedErrors[0].Message)
	}
}

func TestGetErrors(t *testing.T) {
	// Create logger
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"
	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Get error channel
	errorChan := logger.GetErrors()

	// Force an error
	logger.Destinations[0].File.Close()
	logger.Info("This should fail")

	// Read from error channel
	select {
	case err := <-errorChan:
		// Could be write or flush error
		if err.Source != "write" && err.Source != "flush" {
			t.Errorf("Expected write or flush error, got %s", err.Source)
		}
		t.Logf("Got expected error: %s", err.Error())
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for error on channel")
	}
}

func TestLogErrorFormat(t *testing.T) {
	err := LogError{
		Time:        time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
		Level:       ErrorLevelHigh,
		Source:      "write",
		Message:     "Failed to write",
		Err:         errors.New("disk full"),
		Destination: "main",
	}

	expected := "[2025-01-01 12:00:00] write error in main: Failed to write - disk full"
	if err.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, err.Error())
	}

	// Test without destination
	err.Destination = ""
	expected = "[2025-01-01 12:00:00] write error: Failed to write - disk full"
	if err.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, err.Error())
	}
}
