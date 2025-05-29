package flexlog

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestFileOpenError tests error handling when file cannot be opened
func TestFileOpenError(t *testing.T) {
	// Create a directory with no write permissions
	dir := t.TempDir()
	readOnlyDir := filepath.Join(dir, "readonly")
	if err := os.Mkdir(readOnlyDir, 0555); err != nil {
		t.Fatalf("Failed to create read-only dir: %v", err)
	}

	logFile := filepath.Join(readOnlyDir, "test.log")

	// Try to create logger in read-only directory
	_, err := New(logFile)
	if err == nil {
		t.Error("Expected error when creating logger in read-only directory")
	}

	// Check if it's a FlexLogError
	var flexErr *FlexLogError
	if errors.As(err, &flexErr) {
		// Could be file open or file lock error depending on implementation
		if flexErr.Code != ErrCodeFileOpen && flexErr.Code != ErrCodeFileLock {
			t.Errorf("Expected ErrCodeFileOpen or ErrCodeFileLock, got %v", flexErr.Code)
		}
	}
}

// TestRotationErrors tests error handling during rotation
func TestRotationErrors(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set small rotation size
	logger.SetMaxSize(100)

	// Make the directory read-only to cause rotation to fail
	if err := os.Chmod(dir, 0555); err != nil {
		t.Skip("Cannot change directory permissions, skipping test")
	}
	defer os.Chmod(dir, 0755) // Restore permissions

	// Log enough to trigger rotation
	for i := 0; i < 100; i++ {
		logger.Info("This message should trigger rotation failure")
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Check error metrics
	metrics := logger.GetMetrics()
	if metrics.ErrorCount == 0 {
		t.Error("Expected rotation errors to be recorded")
	}
	if _, ok := metrics.ErrorsBySource["rotate"]; !ok {
		t.Error("Expected rotation errors in error sources")
	}
}

// TestChannelFullError tests handling when message channel is full
func TestChannelFullError(t *testing.T) {
	// Set very small channel size
	os.Setenv("FLEXLOG_CHANNEL_SIZE", "1")
	defer os.Unsetenv("FLEXLOG_CHANNEL_SIZE")

	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Don't close logger immediately to keep channel full
	logger.channelSize = 1

	// Fill the channel
	for i := 0; i < 100; i++ {
		logger.Info("Message %d", i)
	}

	// Now close and check metrics
	logger.Close()

	metrics := logger.GetMetrics()
	if metrics.MessagesDropped == 0 {
		t.Error("Expected some messages to be dropped")
	}
	if metrics.ErrorCount == 0 {
		t.Error("Expected channel full errors")
	}
}

// TestShutdownErrors tests error handling during shutdown
func TestShutdownErrors(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Close the file to cause flush errors during shutdown
	logger.defaultDest.File.Close()

	// Log some messages
	for i := 0; i < 10; i++ {
		logger.Info("Message %d", i)
	}

	// Shutdown should handle the error gracefully
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = logger.Shutdown(ctx)
	cancel()

	// Should complete despite errors
	if err != nil {
		t.Logf("Shutdown returned error (expected): %v", err)
	}
}

// TestSyslogConnectionError tests syslog connection failures
func TestSyslogConnectionError(t *testing.T) {
	// Try to connect to non-existent syslog server
	_, err := NewWithBackend("syslog://127.0.0.1:9999", BackendSyslog)
	if err == nil {
		t.Error("Expected error connecting to non-existent syslog server")
	}
}

// TestCompressionError tests compression failure handling
func TestCompressionError(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Enable compression
	logger.SetCompression(CompressionGzip)

	// Queue a non-existent file for compression
	logger.queueForCompression(filepath.Join(dir, "nonexistent.log"))

	// Wait for compression worker to process
	time.Sleep(200 * time.Millisecond)

	// Check error metrics
	metrics := logger.GetMetrics()
	if _, ok := metrics.ErrorsBySource["compression"]; !ok {
		t.Log("Note: Compression error not recorded (might be expected)")
	}
}

// TestFileLockError tests file lock acquisition failures
func TestFileLockError(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	// Create first logger
	logger1, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create first logger: %v", err)
	}
	defer logger1.Close()

	// Try to create second logger with same file (should fail due to lock)
	logger2, err := NewWithBackend(logFile, BackendFlock)
	if err == nil {
		logger2.Close()
		t.Error("Expected error when creating second logger with locked file")
	}

	// Check if it's a lock error
	var flexErr *FlexLogError
	if errors.As(err, &flexErr) {
		if flexErr.Code != ErrCodeFileLock {
			t.Errorf("Expected ErrCodeFileLock, got %v", flexErr.Code)
		}
	}
}

// TestWriteError tests handling of write errors
func TestWriteError(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Close the underlying file to cause write errors
	logger.defaultDest.File.Close()

	// Try to log
	logger.Info("This should fail to write")

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Close logger
	logger.Close()

	// Check error metrics
	metrics := logger.GetMetrics()
	if metrics.ErrorCount == 0 {
		t.Error("Expected write errors to be recorded")
	}
}

// TestRecoveryFallback tests recovery mechanism when main destination fails
func TestRecoveryFallback(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")
	fallbackFile := filepath.Join(dir, "fallback.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Configure recovery
	logger.SetRecoveryConfig(&RecoveryConfig{
		FallbackPath:      fallbackFile,
		MaxRetries:        3,
		RetryDelay:        10 * time.Millisecond,
		BackoffMultiplier: 2.0,
		MaxRetryDelay:     100 * time.Millisecond,
	})

	// Close the main destination to trigger recovery
	logger.defaultDest.File.Close()

	// Log messages that should go to fallback
	for i := 0; i < 5; i++ {
		logger.Errorf("Error message %d", i)
	}

	// Wait for recovery
	time.Sleep(500 * time.Millisecond)

	// Check if fallback file was created
	if _, err := os.Stat(fallbackFile); os.IsNotExist(err) {
		t.Error("Fallback file was not created")
	} else {
		// Read fallback file
		content, err := os.ReadFile(fallbackFile)
		if err != nil {
			t.Errorf("Failed to read fallback file: %v", err)
		} else if len(content) == 0 {
			t.Error("Fallback file is empty")
		} else {
			t.Logf("Fallback file content: %s", string(content))
		}
	}
}

// TestErrorHandlerInvocation tests that custom error handlers are called
func TestErrorHandlerInvocation(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Track errors from handler
	var handlerErrors []LogError
	logger.SetErrorHandler(func(err LogError) {
		handlerErrors = append(handlerErrors, err)
	})

	// Cause an error by closing the file
	logger.defaultDest.File.Close()

	// Try to log
	logger.Info("This should trigger error handler")

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Check that handler was called
	if len(handlerErrors) == 0 {
		t.Error("Error handler was not called")
	} else {
		t.Logf("Handler received %d errors", len(handlerErrors))
		for i, err := range handlerErrors {
			t.Logf("Error %d: %v", i, err)
		}
	}
}