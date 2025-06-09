package features

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDefaultRecoveryConfig(t *testing.T) {
	config := DefaultRecoveryConfig()

	if config == nil {
		t.Fatal("DefaultRecoveryConfig returned nil")
	}

	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries 3, got %d", config.MaxRetries)
	}

	if config.RetryDelay != 100*time.Millisecond {
		t.Errorf("Expected RetryDelay 100ms, got %v", config.RetryDelay)
	}

	if config.BackoffMultiplier != 2.0 {
		t.Errorf("Expected BackoffMultiplier 2.0, got %f", config.BackoffMultiplier)
	}

	if config.MaxRetryDelay != 5*time.Second {
		t.Errorf("Expected MaxRetryDelay 5s, got %v", config.MaxRetryDelay)
	}

	if config.FallbackPath != "/tmp/omni-fallback.log" {
		t.Errorf("Expected FallbackPath /tmp/omni-fallback.log, got %s", config.FallbackPath)
	}

	if config.BufferSize != 1000 {
		t.Errorf("Expected BufferSize 1000, got %d", config.BufferSize)
	}

	if config.Strategy != RecoveryRetry {
		t.Errorf("Expected Strategy RecoveryRetry, got %v", config.Strategy)
	}
}

func TestNewRecoveryManager(t *testing.T) {
	// Test with nil config (should use defaults)
	rm := NewRecoveryManager(nil)
	if rm == nil {
		t.Fatal("NewRecoveryManager returned nil")
	}

	if rm.config == nil {
		t.Fatal("Recovery manager config is nil")
	}

	// Test with custom config
	customConfig := &RecoveryConfig{
		MaxRetries:        5,
		RetryDelay:        200 * time.Millisecond,
		BackoffMultiplier: 1.5,
		MaxRetryDelay:     10 * time.Second,
		FallbackPath:      "/custom/fallback.log",
		BufferSize:        2000,
		Strategy:          RecoveryFallback,
	}

	rm2 := NewRecoveryManager(customConfig)
	if rm2.config.MaxRetries != 5 {
		t.Errorf("Expected MaxRetries 5, got %d", rm2.config.MaxRetries)
	}

	// Test with partial config (should apply defaults for missing values)
	partialConfig := &RecoveryConfig{
		MaxRetries: 10,
	}

	rm3 := NewRecoveryManager(partialConfig)
	if rm3.config.BufferSize != 1000 {
		t.Errorf("Expected default BufferSize 1000, got %d", rm3.config.BufferSize)
	}
}

func TestBufferMessage(t *testing.T) {
	rm := NewRecoveryManager(&RecoveryConfig{
		BufferSize: 3,
		Strategy:   RecoveryBuffer,
	})

	// Track metrics
	var metricsEvents []string
	rm.SetMetricsHandler(func(event string) {
		metricsEvents = append(metricsEvents, event)
	})

	// Buffer messages
	messages := []string{"msg1", "msg2", "msg3", "msg4"}

	for _, msg := range messages {
		rm.bufferMessage(msg)
	}

	// Check buffer size
	if rm.GetBufferSize() != 3 {
		t.Errorf("Expected buffer size 3, got %d", rm.GetBufferSize())
	}

	// Check that buffer overflow was tracked
	overflowFound := false
	for _, event := range metricsEvents {
		if event == "buffer_overflow" {
			overflowFound = true
			break
		}
	}

	if !overflowFound {
		t.Error("Expected buffer_overflow metric event")
	}
}

func TestFlushBuffer(t *testing.T) {
	rm := NewRecoveryManager(&RecoveryConfig{
		BufferSize: 10,
		Strategy:   RecoveryBuffer,
	})

	// Buffer some messages
	messages := []interface{}{"msg1", "msg2", "msg3"}
	for _, msg := range messages {
		rm.bufferMessage(msg)
	}

	// Track processed messages
	var processed []interface{}

	processFunc := func(message interface{}) error {
		processed = append(processed, message)
		// Simulate error on second message
		if message == "msg2" {
			return errors.New("process error")
		}
		return nil
	}

	// Flush buffer
	err := rm.FlushBuffer(processFunc)

	// Should have error due to msg2
	if err == nil {
		t.Error("Expected error from flush buffer")
	}

	// All messages should have been attempted
	if len(processed) != 3 {
		t.Errorf("Expected 3 processed messages, got %d", len(processed))
	}

	// Buffer should be empty
	if rm.GetBufferSize() != 0 {
		t.Errorf("Expected empty buffer after flush, got size %d", rm.GetBufferSize())
	}
}

func TestFallbackWrite(t *testing.T) {
	tempDir := t.TempDir()
	fallbackPath := filepath.Join(tempDir, "fallback.log")

	rm := NewRecoveryManager(&RecoveryConfig{
		FallbackPath: fallbackPath,
		Strategy:     RecoveryFallback,
	})

	// Set error handler
	rm.SetErrorHandler(func(source, dest, msg string, err error) {
		// Error handler configured
	})

	// Write to fallback
	testMessage := "Test fallback message"
	rm.fallbackWrite(testMessage)

	// Close to flush
	rm.Close()

	// Read fallback file
	content, err := os.ReadFile(fallbackPath)
	if err != nil {
		t.Fatalf("Failed to read fallback file: %v", err)
	}

	if !contains(string(content), "FALLBACK: Test fallback message") {
		t.Errorf("Fallback file doesn't contain expected message. Got: %s", content)
	}

	if !contains(string(content), "[") && !contains(string(content), "]") {
		t.Error("Fallback file doesn't contain timestamp")
	}
}

func TestRetryOperation(t *testing.T) {
	rm := NewRecoveryManager(&RecoveryConfig{
		MaxRetries:        3,
		RetryDelay:        10 * time.Millisecond,
		BackoffMultiplier: 2.0,
		MaxRetryDelay:     100 * time.Millisecond,
		Strategy:          RecoveryRetry,
	})

	// Track retry attempts
	var attempts int
	var mu sync.Mutex

	testFunc := func() error {
		mu.Lock()
		attempts++
		mu.Unlock()

		// Fail first 2 attempts
		if attempts < 3 {
			return errors.New("simulated error")
		}
		return nil
	}

	// Start retry operation
	rm.retryOperation("test_dest", testFunc)

	// Wait for retries to complete
	time.Sleep(200 * time.Millisecond)

	// Check that retries happened
	mu.Lock()
	finalAttempts := attempts
	mu.Unlock()

	if finalAttempts != 3 {
		t.Errorf("Expected 3 retry attempts, got %d", finalAttempts)
	}

	// Check retry count was reset after success
	if rm.GetRetryCount("test_dest") != 0 {
		t.Errorf("Expected retry count to be reset, got %d", rm.GetRetryCount("test_dest"))
	}
}

func TestMaxRetriesExceeded(t *testing.T) {
	rm := NewRecoveryManager(&RecoveryConfig{
		MaxRetries:        2,
		RetryDelay:        10 * time.Millisecond,
		BackoffMultiplier: 2.0,
		Strategy:          RecoveryRetry,
	})

	// Track error handler
	var errorHandlerCalled bool
	var errorMsg string
	rm.SetErrorHandler(func(source, dest, msg string, err error) {
		errorHandlerCalled = true
		errorMsg = msg
	})

	// Function that always fails
	failFunc := func() error {
		return errors.New("always fails")
	}

	// Pre-populate retry count to max
	rm.retryMu.Lock()
	rm.retryMap["test_dest"] = rm.config.MaxRetries
	rm.retryMu.Unlock()

	// Try retry operation
	rm.retryOperation("test_dest", failFunc)

	// Give it a moment
	time.Sleep(50 * time.Millisecond)

	if !errorHandlerCalled {
		t.Error("Expected error handler to be called for max retries")
	}

	if !contains(errorMsg, "Max retries") {
		t.Errorf("Expected max retries message, got: %s", errorMsg)
	}
}

func TestHandleError(t *testing.T) {
	tests := []struct {
		name     string
		strategy RecoveryStrategy
		err      error
		message  interface{}
	}{
		{
			name:     "Retry strategy",
			strategy: RecoveryRetry,
			err:      errors.New("retry error"),
			message:  "test message",
		},
		{
			name:     "Fallback strategy",
			strategy: RecoveryFallback,
			err:      errors.New("fallback error"),
			message:  "test message",
		},
		{
			name:     "Buffer strategy",
			strategy: RecoveryBuffer,
			err:      errors.New("buffer error"),
			message:  "test message",
		},
		{
			name:     "Drop strategy",
			strategy: RecoveryDrop,
			err:      errors.New("drop error"),
			message:  "test message",
		},
		{
			name:     "Nil error",
			strategy: RecoveryRetry,
			err:      nil,
			message:  "test message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			rm := NewRecoveryManager(&RecoveryConfig{
				Strategy:     tt.strategy,
				FallbackPath: filepath.Join(tempDir, "fallback.log"),
				BufferSize:   10,
			})

			// Track metrics
			var metricEvent string
			rm.SetMetricsHandler(func(event string) {
				metricEvent = event
			})

			// Mock write function
			writeFunc := func() error {
				return errors.New("write error")
			}

			// Handle error
			rm.HandleError(tt.err, tt.message, "test_dest", writeFunc)

			// Give async operations time
			time.Sleep(50 * time.Millisecond)

			// Verify behavior based on strategy
			switch tt.strategy {
			case RecoveryDrop:
				if tt.err != nil && metricEvent != "message_dropped" {
					t.Errorf("Expected message_dropped metric for drop strategy")
				}
			case RecoveryBuffer:
				if tt.err != nil && rm.GetBufferSize() == 0 {
					t.Error("Expected message to be buffered")
				}
			}
		})
	}
}

func TestGetRetryCount(t *testing.T) {
	rm := NewRecoveryManager(nil)

	// Initially should be 0
	count := rm.GetRetryCount("test_dest")
	if count != 0 {
		t.Errorf("Expected initial retry count 0, got %d", count)
	}

	// Set retry count
	rm.retryMu.Lock()
	rm.retryMap["test_dest"] = 5
	rm.retryMu.Unlock()

	// Check count
	count = rm.GetRetryCount("test_dest")
	if count != 5 {
		t.Errorf("Expected retry count 5, got %d", count)
	}
}

func TestResetRetryCount(t *testing.T) {
	rm := NewRecoveryManager(nil)

	// Set retry count
	rm.retryMu.Lock()
	rm.retryMap["test_dest"] = 3
	rm.retryMu.Unlock()

	// Reset it
	rm.ResetRetryCount("test_dest")

	// Should be 0
	count := rm.GetRetryCount("test_dest")
	if count != 0 {
		t.Errorf("Expected retry count 0 after reset, got %d", count)
	}
}

func TestUpdateConfig(t *testing.T) {
	rm := NewRecoveryManager(&RecoveryConfig{
		BufferSize: 100,
		MaxRetries: 3,
	})

	// Buffer some messages
	for i := 0; i < 5; i++ {
		rm.bufferMessage(i)
	}

	// Update config with different buffer size
	newConfig := &RecoveryConfig{
		BufferSize: 200,
		MaxRetries: 5,
	}

	rm.UpdateConfig(newConfig)

	// Check that config was updated
	if rm.GetConfig().BufferSize != 200 {
		t.Errorf("Expected buffer size 200, got %d", rm.GetConfig().BufferSize)
	}

	if rm.GetConfig().MaxRetries != 5 {
		t.Errorf("Expected max retries 5, got %d", rm.GetConfig().MaxRetries)
	}

	// Check that existing buffer content was preserved
	if rm.GetBufferSize() != 5 {
		t.Errorf("Expected buffer to preserve 5 messages, got %d", rm.GetBufferSize())
	}
}

func TestConcurrentRetryOperations(t *testing.T) {
	rm := NewRecoveryManager(&RecoveryConfig{
		MaxRetries:        3,
		RetryDelay:        10 * time.Millisecond,
		BackoffMultiplier: 1.5,
		Strategy:          RecoveryRetry,
	})

	// Track successful operations using atomic operations
	var successCount int32

	// Run multiple concurrent retry operations
	var wg sync.WaitGroup
	numOperations := 10

	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		destName := string(rune('a' + i))

		go func(dest string) {
			defer wg.Done()

			attempts := 0
			writeFunc := func() error {
				attempts++
				if attempts < 2 {
					return errors.New("simulated error")
				}
				atomic.AddInt32(&successCount, 1)
				return nil
			}

			rm.HandleError(errors.New("initial error"), "message", dest, writeFunc)
		}(destName)
	}

	// Wait for all operations to complete
	wg.Wait()
	time.Sleep(200 * time.Millisecond) // Allow retries to complete

	// All operations should eventually succeed
	if final := atomic.LoadInt32(&successCount); int(final) != numOperations {
		t.Errorf("Expected %d successful operations, got %d", numOperations, final)
	}
}

func TestFallbackDirectoryCreation(t *testing.T) {
	tempDir := t.TempDir()
	nestedPath := filepath.Join(tempDir, "nested", "dir", "fallback.log")

	rm := NewRecoveryManager(&RecoveryConfig{
		FallbackPath: nestedPath,
		Strategy:     RecoveryFallback,
	})

	// Write to fallback (should create directories)
	rm.fallbackWrite("test message")

	// Check that directories were created
	dir := filepath.Dir(nestedPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("Expected fallback directories to be created")
	}

	rm.Close()
}

func TestErrorHandlerTracking(t *testing.T) {
	rm := NewRecoveryManager(nil)

	// Track all error handler calls
	var errorCalls []struct {
		source string
		dest   string
		msg    string
		err    error
	}

	rm.SetErrorHandler(func(source, dest, msg string, err error) {
		errorCalls = append(errorCalls, struct {
			source string
			dest   string
			msg    string
			err    error
		}{source, dest, msg, err})
	})

	// Trigger various errors

	// 1. Fallback directory creation error (use invalid path)
	rm.config.FallbackPath = "/\x00/invalid/path" // Null byte makes it invalid
	rm.fallbackWrite("test")

	// 2. Max retries exceeded
	rm.retryMu.Lock()
	rm.retryMap["test_dest"] = rm.config.MaxRetries
	rm.retryMu.Unlock()
	rm.retryOperation("test_dest", func() error { return errors.New("fail") })

	// Give time for async operations
	time.Sleep(50 * time.Millisecond)

	// Should have recorded errors
	if len(errorCalls) == 0 {
		t.Error("Expected error handler to be called")
	}
}

// Helper function
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
