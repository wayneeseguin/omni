package omni

import (
	"time"
)

// RecoveryConfig contains configuration for error recovery
type RecoveryConfig struct {
	// Enable recovery mechanisms
	Enabled bool

	// Fallback file path for when primary destination fails
	FallbackPath string

	// Maximum number of retry attempts
	MaxRetries int

	// Delay between retry attempts
	RetryDelay time.Duration

	// Maximum time to spend on recovery attempts
	MaxRecoveryTime time.Duration

	// Strategy for handling unrecoverable errors
	UnrecoverableStrategy string // "discard", "stderr", "fallback"

	// Buffer size for storing messages during recovery
	RecoveryBufferSize int

	// Whether to log recovery operations themselves
	LogRecoveryOperations bool

	// Callback function for recovery events
	RecoveryCallback func(operation string, success bool, err error)

	// Backoff multiplier for exponential backoff
	BackoffMultiplier float64

	// Maximum retry delay
	MaxRetryDelay time.Duration
}
