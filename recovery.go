package omni

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
	config   *RecoveryConfig
	buffer   []LogMessage
	bufferMu sync.Mutex
	fallback *os.File
	retryMap map[string]int // Track retry counts by destination
	retryMu  sync.Mutex
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
		buffer:   make([]LogMessage, 0, config.BufferSize),
		retryMap: make(map[string]int),
	}
}

// HandleError handles an error with the configured recovery strategy.
// It analyzes the error type and applies the appropriate recovery action.
//
// Parameters:
//   - f: The logger instance
//   - err: The error that occurred
//   - msg: The log message that failed
//   - dest: The destination that failed
func (rm *RecoveryManager) HandleError(f *Omni, err error, msg LogMessage, dest *Destination) {
	// Determine recovery strategy based on error type
	strategy := rm.determineStrategy(err)

	switch strategy {
	case RecoveryRetry:
		rm.retryOperation(f, err, msg, dest)
	case RecoveryFallback:
		rm.fallbackWrite(f, msg)
	case RecoveryBuffer:
		rm.bufferMessage(msg)
	case RecoveryDrop:
		// Log that we're dropping the message
		f.trackMessageDropped()
	}
}

// determineStrategy determines the recovery strategy based on error type.
// This internal method maps specific error types to appropriate recovery strategies.
func (rm *RecoveryManager) determineStrategy(err error) RecoveryStrategy {
	if err == nil {
		return RecoveryDrop
	}

	// Check for specific error types first, before generic retryable check
	if omniErr, ok := err.(*OmniError); ok {
		switch omniErr.Code {
		case ErrCodeChannelFull:
			return RecoveryBuffer
		case ErrCodeFileWrite, ErrCodeFileFlush:
			return RecoveryFallback
		case ErrCodeDestinationDisabled:
			return RecoveryDrop
		}
	}

	// Check if error is retryable (for other errors not handled above)
	if IsRetryable(err) {
		return RecoveryRetry
	}

	// Default to configured strategy
	return rm.config.Strategy
}

// retryOperation retries an operation with exponential backoff.
// It tracks retry counts per destination and schedules retries with increasing delays.
func (rm *RecoveryManager) retryOperation(f *Omni, err error, msg LogMessage, dest *Destination) {
	destName := dest.Name

	// Get current retry count
	rm.retryMu.Lock()
	retryCount := rm.retryMap[destName]
	rm.retryMu.Unlock()

	if retryCount >= rm.config.MaxRetries {
		// Max retries exceeded, fallback or drop
		if rm.config.Strategy == RecoveryFallback {
			rm.fallbackWrite(f, msg)
		} else {
			f.trackMessageDropped()
		}

		// Reset retry count
		rm.retryMu.Lock()
		delete(rm.retryMap, destName)
		rm.retryMu.Unlock()
		return
	}

	// Calculate delay with exponential backoff
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
		// Increment retry count
		rm.retryMu.Lock()
		rm.retryMap[destName] = retryCount + 1
		rm.retryMu.Unlock()

		// Retry the operation
		f.processMessage(msg, dest)
	})
}

// fallbackWrite writes to a fallback destination.
// This ensures critical log messages are preserved even when the primary destination fails.
// Messages are written with a [FALLBACK] prefix for easy identification.
func (rm *RecoveryManager) fallbackWrite(f *Omni, msg LogMessage) {
	// Ensure fallback file is open
	if rm.fallback == nil {
		// Create fallback directory if needed
		dir := filepath.Dir(rm.config.FallbackPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			f.logError("recovery", "", fmt.Sprintf("Failed to create fallback directory %s", dir), err, ErrorLevelHigh)
			f.trackMessageDropped()
			return
		}

		// Open fallback file
		file, err := os.OpenFile(rm.config.FallbackPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			f.logError("recovery", "", fmt.Sprintf("Failed to open fallback file %s", rm.config.FallbackPath), err, ErrorLevelHigh)
			f.trackMessageDropped()
			return
		}
		rm.fallback = file
	}

	// Format and write message
	var entry string
	if msg.Entry != nil {
		data, _ := formatJSONEntry(msg.Entry)
		entry = string(data)
	} else {
		entry = fmt.Sprintf(msg.Format, msg.Args...)
	}

	// Write to fallback with timestamp
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	fallbackEntry := fmt.Sprintf("[%s] [FALLBACK] %s\n", timestamp, entry)

	if _, err := rm.fallback.WriteString(fallbackEntry); err != nil {
		f.logError("recovery", "", "Failed to write to fallback", err, ErrorLevelHigh)
		f.trackMessageDropped()
	}
}

// bufferMessage buffers a message for later processing.
// When the buffer is full, the oldest message is dropped (FIFO).
func (rm *RecoveryManager) bufferMessage(msg LogMessage) {
	rm.bufferMu.Lock()
	defer rm.bufferMu.Unlock()

	// If buffer size is 0, don't buffer anything
	if rm.config.BufferSize <= 0 {
		return
	}

	// Check buffer capacity
	if len(rm.buffer) >= rm.config.BufferSize {
		// Buffer full, drop oldest message
		rm.buffer = rm.buffer[1:]
	}

	rm.buffer = append(rm.buffer, msg)
}

// FlushBuffer attempts to flush buffered messages.
// This is typically called when the primary destination recovers.
// Messages that still can't be sent are re-buffered.
func (rm *RecoveryManager) FlushBuffer(f *Omni) {
	rm.bufferMu.Lock()
	messages := make([]LogMessage, len(rm.buffer))
	copy(messages, rm.buffer)
	rm.buffer = rm.buffer[:0] // Clear buffer
	rm.bufferMu.Unlock()

	// Try to process buffered messages
	for _, msg := range messages {
		select {
		case f.msgChan <- msg:
			// Successfully sent
		default:
			// Channel still full, re-buffer
			rm.bufferMessage(msg)
		}
	}
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

// SetRecoveryConfig sets the recovery configuration for the logger.
// This allows runtime changes to recovery behavior.
//
// Parameters:
//   - config: The new recovery configuration
func (f *Omni) SetRecoveryConfig(config *RecoveryConfig) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Close existing recovery manager if any
	if f.recoveryManager != nil {
		f.recoveryManager.Close()
	}

	f.recoveryManager = NewRecoveryManager(config)
}

// RecoverFromError attempts to recover from an error.
// This is the main entry point for error recovery in the logging system.
//
// Parameters:
//   - err: The error that occurred
//   - msg: The log message that failed
//   - dest: The destination that failed
func (f *Omni) RecoverFromError(err error, msg LogMessage, dest *Destination) {
	if f.recoveryManager == nil {
		// No recovery configured, just drop the message
		f.trackMessageDropped()
		return
	}

	f.recoveryManager.HandleError(f, err, msg, dest)
}

