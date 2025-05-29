package flexlog

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)


func TestDefaultRecoveryConfig(t *testing.T) {
	config := DefaultRecoveryConfig()

	if config.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", config.MaxRetries)
	}
	if config.RetryDelay != 100*time.Millisecond {
		t.Errorf("RetryDelay = %v, want 100ms", config.RetryDelay)
	}
	if config.BackoffMultiplier != 2.0 {
		t.Errorf("BackoffMultiplier = %f, want 2.0", config.BackoffMultiplier)
	}
	if config.Strategy != RecoveryRetry {
		t.Errorf("Strategy = %v, want RecoveryRetry", config.Strategy)
	}
}

func TestNewRecoveryManager(t *testing.T) {
	// Test with nil config
	rm := NewRecoveryManager(nil)
	if rm.config == nil {
		t.Error("config should not be nil")
	}
	if rm.config.MaxRetries != 3 {
		t.Error("should use default config when nil is passed")
	}

	// Test with custom config
	config := &RecoveryConfig{
		MaxRetries: 5,
		RetryDelay: 200 * time.Millisecond,
		Strategy:   RecoveryFallback,
	}
	rm = NewRecoveryManager(config)
	if rm.config.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", rm.config.MaxRetries)
	}
	if rm.config.Strategy != RecoveryFallback {
		t.Errorf("Strategy = %v, want RecoveryFallback", rm.config.Strategy)
	}
}

func TestRecoveryManager_DetermineStrategy(t *testing.T) {
	rm := NewRecoveryManager(DefaultRecoveryConfig())

	tests := []struct {
		name     string
		err      error
		expected RecoveryStrategy
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: RecoveryDrop,
		},
		{
			name:     "retryable channel full error",
			err:      NewFlexLogError(ErrCodeChannelFull, "write", "", nil),
			expected: RecoveryBuffer,
		},
		{
			name:     "file write error",
			err:      NewFlexLogError(ErrCodeFileWrite, "write", "", nil),
			expected: RecoveryFallback,
		},
		{
			name:     "disabled destination",
			err:      NewFlexLogError(ErrCodeDestinationDisabled, "write", "", nil),
			expected: RecoveryDrop,
		},
		{
			name:     "retryable generic error",
			err:      errors.New("resource temporarily unavailable"),
			expected: RecoveryRetry,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := rm.determineStrategy(tt.err)
			if strategy != tt.expected {
				t.Errorf("determineStrategy() = %v, want %v", strategy, tt.expected)
			}
		})
	}
}

func TestRecoveryManager_BufferMessage(t *testing.T) {
	config := &RecoveryConfig{
		BufferSize: 2,
		Strategy:   RecoveryBuffer,
	}
	rm := NewRecoveryManager(config)

	// Create test messages
	msg1 := LogMessage{Format: "message 1"}
	msg2 := LogMessage{Format: "message 2"}
	msg3 := LogMessage{Format: "message 3"}

	// Buffer messages
	rm.bufferMessage(msg1)
	rm.bufferMessage(msg2)

	if len(rm.buffer) != 2 {
		t.Errorf("buffer length = %d, want 2", len(rm.buffer))
	}

	// Buffer should drop oldest when full
	rm.bufferMessage(msg3)
	if len(rm.buffer) != 2 {
		t.Errorf("buffer length = %d, want 2", len(rm.buffer))
	}
	if rm.buffer[0].Format != "message 2" {
		t.Errorf("oldest message = %q, want %q", rm.buffer[0].Format, "message 2")
	}
	if rm.buffer[1].Format != "message 3" {
		t.Errorf("newest message = %q, want %q", rm.buffer[1].Format, "message 3")
	}
}

func TestRecoveryManager_FallbackWrite(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	fallbackPath := filepath.Join(tempDir, "fallback.log")

	config := &RecoveryConfig{
		FallbackPath: fallbackPath,
		Strategy:     RecoveryFallback,
	}
	rm := NewRecoveryManager(config)
	defer rm.Close()

	// Create a mock logger
	logger := &FlexLog{}

	// Test structured message
	msg := LogMessage{
		Entry: &LogEntry{
			Level:     "info",
			Message:   "test message",
			Timestamp: "2023-01-01T12:00:00Z",
		},
	}

	rm.fallbackWrite(logger, msg)

	// Check if fallback file was created and has content
	if _, err := os.Stat(fallbackPath); os.IsNotExist(err) {
		t.Error("fallback file was not created")
	}

	content, err := os.ReadFile(fallbackPath)
	if err != nil {
		t.Fatalf("failed to read fallback file: %v", err)
	}

	contentStr := string(content)
	if !contains(contentStr, "FALLBACK") {
		t.Error("fallback file should contain FALLBACK marker")
	}
	if !contains(contentStr, "test message") {
		t.Error("fallback file should contain the message")
	}
}

func TestRecoveryManager_FlushBuffer(t *testing.T) {
	rm := NewRecoveryManager(DefaultRecoveryConfig())

	// Create a mock logger with a channel
	logger := &FlexLog{
		msgChan: make(chan LogMessage, 2),
	}

	// Buffer some messages
	msg1 := LogMessage{Format: "message 1"}
	msg2 := LogMessage{Format: "message 2"}
	msg3 := LogMessage{Format: "message 3"}

	rm.bufferMessage(msg1)
	rm.bufferMessage(msg2)
	rm.bufferMessage(msg3)

	initialBufferLen := len(rm.buffer)

	// Flush buffer
	rm.FlushBuffer(logger)

	// Check that some messages were sent to channel
	channelLen := len(logger.msgChan)
	if channelLen == 0 {
		t.Error("no messages were sent to channel")
	}

	// Buffer should be smaller or messages re-buffered if channel was full
	finalBufferLen := len(rm.buffer)
	if finalBufferLen >= initialBufferLen {
		t.Error("buffer should have been reduced or messages re-buffered")
	}
}

func TestRecoveryManager_HandleError(t *testing.T) {
	tests := []struct {
		name           string
		strategy       RecoveryStrategy
		err            error
		expectRetry    bool
		expectFallback bool
		expectBuffer   bool
		expectDrop     bool
	}{
		{
			name:        "retry strategy with retryable error",
			strategy:    RecoveryRetry,
			err:         errors.New("resource temporarily unavailable"),
			expectRetry: true,
		},
		{
			name:           "fallback strategy with write error",
			strategy:       RecoveryFallback,
			err:            NewFlexLogError(ErrCodeFileWrite, "write", "", nil),
			expectFallback: true,
		},
		{
			name:         "buffer strategy with channel full",
			strategy:     RecoveryBuffer,
			err:          NewFlexLogError(ErrCodeChannelFull, "write", "", nil),
			expectBuffer: true,
		},
		{
			name:       "drop strategy with disabled destination",
			strategy:   RecoveryDrop,
			err:        NewFlexLogError(ErrCodeDestinationDisabled, "write", "", nil),
			expectDrop: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &RecoveryConfig{
				Strategy:     tt.strategy,
				FallbackPath: filepath.Join(t.TempDir(), "fallback.log"),
			}
			rm := NewRecoveryManager(config)
			defer rm.Close()

			// Create mock logger and destination
			logger := &FlexLog{
				msgChan: make(chan LogMessage, 1),
			}
			dest := &Destination{Name: "test-dest"}
			msg := LogMessage{Format: "test message"}

			// Track initial state
			rm.bufferMu.Lock()
			initialBufferLen := len(rm.buffer)
			rm.bufferMu.Unlock()

			// Handle the error
			rm.HandleError(logger, tt.err, msg, dest)

			// Small delay to allow async operations
			time.Sleep(10 * time.Millisecond)

			// Verify expected behavior
			if tt.expectRetry {
				// Should have scheduled a retry (retry count incremented or timer set)
				// This is hard to test directly due to async nature
			}

			if tt.expectBuffer {
				rm.bufferMu.Lock()
				currentBufferLen := len(rm.buffer)
				rm.bufferMu.Unlock()
				
				if currentBufferLen <= initialBufferLen {
					t.Errorf("message should have been buffered: initial=%d, current=%d", initialBufferLen, currentBufferLen)
				}
			}

			if tt.expectFallback {
				// Check if fallback file exists
				if _, err := os.Stat(config.FallbackPath); os.IsNotExist(err) {
					t.Error("fallback file should have been created")
				}
			}

			// Note: expectDrop case is harder to test as it just calls trackMessageDropped
		})
	}
}

func TestRecoveryManager_RetryOperation(t *testing.T) {
	// Test exceeding max retries
	t.Run("exceed max retries drops message", func(t *testing.T) {
		config := &RecoveryConfig{
			MaxRetries:   2,
			RetryDelay:   10 * time.Millisecond,
			Strategy:     RecoveryRetry,
			FallbackPath: filepath.Join(t.TempDir(), "fallback.log"),
		}
		rm := NewRecoveryManager(config)
		defer rm.Close()

		// Create minimal mock logger
		logger := &FlexLog{
			messagesDropped: 0,
		}
		dest := &Destination{Name: "test-dest"}
		msg := LogMessage{Format: "test message"}

		// Set retry count to exceed max
		rm.retryMu.Lock()
		rm.retryMap["test-dest"] = 3 // Exceed max retries
		rm.retryMu.Unlock()

		initialDropped := logger.messagesDropped
		rm.retryOperation(logger, errors.New("test error"), msg, dest)

		// Should reset retry count
		rm.retryMu.Lock()
		_, exists := rm.retryMap["test-dest"]
		rm.retryMu.Unlock()

		if exists {
			t.Error("retry count should have been reset after exceeding max retries")
		}

		// Should have dropped the message
		if logger.messagesDropped <= initialDropped {
			t.Error("message should have been dropped after exceeding max retries")
		}
	})

	// Test fallback strategy when max retries exceeded
	t.Run("exceed max retries with fallback strategy", func(t *testing.T) {
		config := &RecoveryConfig{
			MaxRetries:   2,
			RetryDelay:   10 * time.Millisecond,
			Strategy:     RecoveryFallback,
			FallbackPath: filepath.Join(t.TempDir(), "fallback.log"),
		}
		rm := NewRecoveryManager(config)
		defer rm.Close()

		logger := &FlexLog{}
		dest := &Destination{Name: "test-dest"}
		msg := LogMessage{Format: "test message"}

		// Set retry count to exceed max
		rm.retryMu.Lock()
		rm.retryMap["test-dest"] = 3
		rm.retryMu.Unlock()

		rm.retryOperation(logger, errors.New("test error"), msg, dest)

		// Give a small delay for file write
		time.Sleep(10 * time.Millisecond)

		// Check fallback file exists
		if _, err := os.Stat(config.FallbackPath); os.IsNotExist(err) {
			t.Error("fallback file should have been created")
		}
	})

	// Test retry scheduling (without actually executing the retry)
	t.Run("schedules retry when under max retries", func(t *testing.T) {
		config := &RecoveryConfig{
			MaxRetries: 2,
			RetryDelay: 10 * time.Millisecond,
			Strategy:   RecoveryRetry,
		}
		rm := NewRecoveryManager(config)
		defer rm.Close()

		logger := &FlexLog{}
		dest := &Destination{Name: "test-dest"}
		msg := LogMessage{Format: "test message"}

		// First call should schedule a retry (but we won't wait for it)
		rm.retryOperation(logger, errors.New("test error"), msg, dest)

		// Retry count should still be 0 (not incremented until timer fires)
		rm.retryMu.Lock()
		retryCount := rm.retryMap["test-dest"]
		rm.retryMu.Unlock()

		if retryCount != 0 {
			t.Errorf("retry count should be 0 immediately after scheduling, got %d", retryCount)
		}
	})
}

func TestRecoveryManager_Close(t *testing.T) {
	tempDir := t.TempDir()
	fallbackPath := filepath.Join(tempDir, "fallback.log")

	config := &RecoveryConfig{
		FallbackPath: fallbackPath,
	}
	rm := NewRecoveryManager(config)

	// Create fallback file
	logger := &FlexLog{}
	msg := LogMessage{Format: "test"}
	rm.fallbackWrite(logger, msg)

	// Close should close the fallback file
	err := rm.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Fallback file should still exist
	if _, err := os.Stat(fallbackPath); os.IsNotExist(err) {
		t.Error("fallback file should still exist after close")
	}
}

func TestFlexLog_SetRecoveryConfig(t *testing.T) {
	logger := &FlexLog{}

	config := &RecoveryConfig{
		MaxRetries: 5,
		Strategy:   RecoveryFallback,
	}

	logger.SetRecoveryConfig(config)

	if logger.recoveryManager == nil {
		t.Error("recovery manager should be set")
	}

	if logger.recoveryManager.config.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", logger.recoveryManager.config.MaxRetries)
	}
}

// Benchmark tests
func BenchmarkRecoveryManager_DetermineStrategy(b *testing.B) {
	rm := NewRecoveryManager(DefaultRecoveryConfig())
	err := NewFlexLogError(ErrCodeFileWrite, "write", "", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rm.determineStrategy(err)
	}
}

func BenchmarkRecoveryManager_BufferMessage(b *testing.B) {
	rm := NewRecoveryManager(DefaultRecoveryConfig())
	msg := LogMessage{Format: "test message"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.bufferMessage(msg)
	}
}

func BenchmarkRecoveryManager_FallbackWrite(b *testing.B) {
	tempDir := b.TempDir()
	config := &RecoveryConfig{
		FallbackPath: filepath.Join(tempDir, "fallback.log"),
	}
	rm := NewRecoveryManager(config)
	defer rm.Close()

	logger := &FlexLog{}
	msg := LogMessage{Format: "test message"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.fallbackWrite(logger, msg)
	}
}
