package features

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// RecoveryStrategy defines how to handle errors.
// Different strategies provide flexibility in error handling based on the situation.
type RecoveryStrategy int

const (
	// RecoveryRetry attempts to retry the operation with exponential backoff
	RecoveryRetry RecoveryStrategy = iota
	// RecoveryFallback falls back to an alternative destination (e.g., local file)
	RecoveryFallback
	// RecoveryDrop drops the message without retrying
	RecoveryDrop
	// RecoveryBuffer buffers messages temporarily for later processing
	RecoveryBuffer
)

// RecoveryConfig configures error recovery behavior.
// It provides comprehensive options for handling logging failures gracefully.
type RecoveryConfig struct {
	// Maximum retry attempts
	MaxRetries int
	// Delay between retries
	RetryDelay time.Duration
	// Exponential backoff multiplier
	BackoffMultiplier float64
	// Maximum retry delay
	MaxRetryDelay time.Duration
	// Fallback destination
	FallbackPath string
	// Buffer size for temporary storage
	BufferSize int
	// Recovery strategy
	Strategy RecoveryStrategy
}

// DefaultRecoveryConfig returns default recovery configuration.
// These defaults provide a reasonable balance between reliability and performance.
//
// Returns:
//   - *RecoveryConfig: Configuration with sensible defaults
func DefaultRecoveryConfig() *RecoveryConfig {
	return &RecoveryConfig{
		MaxRetries:        3,
		RetryDelay:        100 * time.Millisecond,
		BackoffMultiplier: 2.0,
		MaxRetryDelay:     5 * time.Second,
		FallbackPath:      "/tmp/omni-fallback.log",
		BufferSize:        1000,
		Strategy:          RecoveryRetry,
	}
}

// RecoveryManager handles error recovery for the logger.
// It implements various strategies to ensure log messages are not lost
// even when the primary logging destination fails.
type RecoveryManager struct {
	config         *RecoveryConfig
	buffer         []interface{}
	bufferMu       sync.Mutex
	fallback       *os.File
	retryMap       map[string]int // Track retry counts by destination
	retryMu        sync.Mutex
	errorHandler   func(source, dest, msg string, err error)
	metricsHandler func(event string)
}

// NewRecoveryManager creates a new recovery manager.
// If config is nil, default configuration is used.
//
// Parameters:
//   - config: Recovery configuration (can be nil)
//
// Returns:
//   - *RecoveryManager: A new recovery manager instance
func NewRecoveryManager(config *RecoveryConfig) *RecoveryManager {
	if config == nil {
		config = DefaultRecoveryConfig()
	}

	// Apply defaults for missing values
	if config.BufferSize == 0 {
		config.BufferSize = 1000 // Default buffer size
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 100 * time.Millisecond
	}
	if config.BackoffMultiplier == 0 {
		config.BackoffMultiplier = 2.0
	}
	if config.MaxRetryDelay == 0 {
		config.MaxRetryDelay = 5 * time.Second
	}

	return &RecoveryManager{
		config:   config,
		buffer:   make([]interface{}, 0, config.BufferSize),
		retryMap: make(map[string]int),
	}
}

// SetErrorHandler sets the error handling function
func (rm *RecoveryManager) SetErrorHandler(handler func(source, dest, msg string, err error)) {
	rm.errorHandler = handler
}

// SetMetricsHandler sets the metrics tracking function
func (rm *RecoveryManager) SetMetricsHandler(handler func(event string)) {
	rm.metricsHandler = handler
}

// HandleError handles an error with the configured recovery strategy.
func (rm *RecoveryManager) HandleError(err error, message interface{}, destName string, writeFunc func() error) {
	// Determine recovery strategy based on error type
	strategy := rm.determineStrategy(err)

	switch strategy {
	case RecoveryRetry:
		rm.retryOperation(destName, writeFunc)
	case RecoveryFallback:
		rm.fallbackWrite(message)
	case RecoveryBuffer:
		rm.bufferMessage(message)
	case RecoveryDrop:
		// Track that we're dropping the message
		if rm.metricsHandler != nil {
			rm.metricsHandler("message_dropped")
		}
	}
}

// determineStrategy determines the recovery strategy based on error type.
// This internal method maps specific error types to appropriate recovery strategies.
func (rm *RecoveryManager) determineStrategy(err error) RecoveryStrategy {
	if err == nil {
		return RecoveryDrop
	}

	// Note: Error type checking would need to be implemented in the omni package
	// where the specific error types are defined

	// Default to configured strategy
	return rm.config.Strategy
}

// retryOperation retries a failed operation with exponential backoff.
func (rm *RecoveryManager) retryOperation(destName string, writeFunc func() error) {
	rm.retryMu.Lock()
	retryCount := rm.retryMap[destName]
	rm.retryMu.Unlock()

	if retryCount >= rm.config.MaxRetries {
		if rm.errorHandler != nil {
			rm.errorHandler("recovery", destName, fmt.Sprintf("Max retries (%d) exceeded", rm.config.MaxRetries), nil)
		}
		return
	}

	// Calculate backoff delay
	delay := rm.config.RetryDelay
	for i := 0; i < retryCount; i++ {
		delay = time.Duration(float64(delay) * rm.config.BackoffMultiplier)
		if delay > rm.config.MaxRetryDelay {
			delay = rm.config.MaxRetryDelay
			break
		}
	}

	// Schedule retry
	time.AfterFunc(delay, func() {
		if err := writeFunc(); err != nil {
			// Update retry count
			rm.retryMu.Lock()
			rm.retryMap[destName]++
			rm.retryMu.Unlock()

			// Recursive retry
			rm.HandleError(err, nil, destName, writeFunc)
		} else {
			// Success - reset retry count
			rm.retryMu.Lock()
			delete(rm.retryMap, destName)
			rm.retryMu.Unlock()
		}
	})
}

// fallbackWrite writes to a fallback destination.
func (rm *RecoveryManager) fallbackWrite(message interface{}) {
	// Initialize fallback file if needed
	if rm.fallback == nil && rm.config.FallbackPath != "" {
		dir := filepath.Dir(rm.config.FallbackPath)
		// #nosec G301 - fallback log directory needs standard permissions
		if err := os.MkdirAll(dir, 0755); err != nil {
			if rm.errorHandler != nil {
				rm.errorHandler("recovery", "fallback", "Failed to create fallback directory", err)
			}
			return
		}

		file, err := os.OpenFile(rm.config.FallbackPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644) // #nosec G302 - fallback log file
		if err != nil {
			if rm.errorHandler != nil {
				rm.errorHandler("recovery", "fallback", "Failed to open fallback file", err)
			}
			return
		}
		rm.fallback = file
	}

	if rm.fallback != nil {
		// Write to fallback
		timestamp := time.Now().Format(time.RFC3339)
		data := fmt.Sprintf("[%s] FALLBACK: %v\n", timestamp, message)
		if _, err := rm.fallback.WriteString(data); err != nil {
			if rm.errorHandler != nil {
				rm.errorHandler("recovery", "fallback", "Failed to write to fallback", err)
			}
		}
	}
}

// bufferMessage adds a message to the recovery buffer.
func (rm *RecoveryManager) bufferMessage(message interface{}) {
	rm.bufferMu.Lock()
	defer rm.bufferMu.Unlock()

	if len(rm.buffer) < rm.config.BufferSize {
		rm.buffer = append(rm.buffer, message)
	} else {
		// Buffer full - drop oldest message
		copy(rm.buffer, rm.buffer[1:])
		rm.buffer[len(rm.buffer)-1] = message
		
		if rm.metricsHandler != nil {
			rm.metricsHandler("buffer_overflow")
		}
	}
}

// FlushBuffer processes all buffered messages.
func (rm *RecoveryManager) FlushBuffer(processFunc func(message interface{}) error) error {
	rm.bufferMu.Lock()
	messages := make([]interface{}, len(rm.buffer))
	copy(messages, rm.buffer)
	rm.buffer = rm.buffer[:0] // Clear buffer
	rm.bufferMu.Unlock()

	var errors []string
	for _, msg := range messages {
		if err := processFunc(msg); err != nil {
			errors = append(errors, err.Error())
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("flush buffer errors: %s", errors)
	}
	return nil
}

// GetBufferSize returns the current buffer size.
func (rm *RecoveryManager) GetBufferSize() int {
	rm.bufferMu.Lock()
	defer rm.bufferMu.Unlock()
	return len(rm.buffer)
}

// Close closes the recovery manager.
// This ensures any open fallback files are properly closed.
//
// Returns:
//   - error: Close error if fallback file fails to close
func (rm *RecoveryManager) Close() error {
	if rm.fallback != nil {
		return rm.fallback.Close()
	}
	return nil
}

// GetRetryCount returns the current retry count for a destination.
func (rm *RecoveryManager) GetRetryCount(destName string) int {
	rm.retryMu.Lock()
	defer rm.retryMu.Unlock()
	return rm.retryMap[destName]
}

// ResetRetryCount resets the retry count for a destination.
func (rm *RecoveryManager) ResetRetryCount(destName string) {
	rm.retryMu.Lock()
	defer rm.retryMu.Unlock()
	delete(rm.retryMap, destName)
}

// GetConfig returns the recovery configuration.
func (rm *RecoveryManager) GetConfig() *RecoveryConfig {
	return rm.config
}

// UpdateConfig updates the recovery configuration.
func (rm *RecoveryManager) UpdateConfig(config *RecoveryConfig) {
	if config != nil {
		rm.config = config
		
		// Resize buffer if needed
		if config.BufferSize != cap(rm.buffer) {
			rm.bufferMu.Lock()
			newBuffer := make([]interface{}, len(rm.buffer), config.BufferSize)
			copy(newBuffer, rm.buffer)
			rm.buffer = newBuffer
			rm.bufferMu.Unlock()
		}
	}
}