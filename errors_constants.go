package omni

import (
	"errors"
	"fmt"
)

// Common errors that can be compared with errors.Is()
var (
	// ErrLoggerClosed is returned when attempting to use a closed logger
	ErrLoggerClosed = errors.New("logger is closed")

	// ErrInvalidLevel is returned when an invalid log level is specified
	ErrInvalidLevel = errors.New("invalid log level")

	// ErrInvalidFormat is returned when an invalid format is specified
	ErrInvalidFormat = errors.New("invalid log format")

	// ErrInvalidIndex is returned when an invalid destination index is used
	ErrInvalidIndex = errors.New("invalid destination index")

	// ErrDestinationNotFound is returned when a destination cannot be found
	ErrDestinationNotFound = errors.New("destination not found")

	// ErrDestinationExists is returned when trying to add a duplicate destination
	ErrDestinationExists = errors.New("destination already exists")

	// ErrChannelFull is returned when the message channel is full
	ErrChannelFull = errors.New("message channel full")

	// ErrNoDestinations is returned when no destinations are configured
	ErrNoDestinations = errors.New("no destinations configured")

	// ErrInvalidConfig is returned when configuration is invalid
	ErrInvalidConfig = errors.New("invalid configuration")

	// ErrCompressionFailed is returned when compression fails
	ErrCompressionFailed = errors.New("compression failed")

	// ErrRotationFailed is returned when log rotation fails
	ErrRotationFailed = errors.New("rotation failed")

	// ErrFlushTimeout is returned when flush operation times out
	ErrFlushTimeout = errors.New("flush timeout")

	// ErrShutdownTimeout is returned when shutdown times out
	ErrShutdownTimeout = errors.New("shutdown timeout")

	// ErrInvalidDestination is returned when destination is invalid
	ErrInvalidDestination = errors.New("invalid destination")

	// ErrNilWriter is returned when writer is nil
	ErrNilWriter = errors.New("writer cannot be nil")

	// ErrInvalidConfiguration is returned when configuration is invalid
	ErrInvalidConfiguration = errors.New("invalid configuration")
)

// ErrorCodeString returns a human-readable string for an error code
func ErrorCodeString(code ErrorCode) string {
	switch code {
	case ErrCodeUnknown:
		return "Unknown"
	case ErrCodeFileOpen:
		return "FileOpen"
	case ErrCodeFileClose:
		return "FileClose"
	case ErrCodeFileWrite:
		return "FileWrite"
	case ErrCodeFileFlush:
		return "FileFlush"
	case ErrCodeFileRotate:
		return "FileRotate"
	case ErrCodeFileLock:
		return "FileLock"
	case ErrCodeFileUnlock:
		return "FileUnlock"
	case ErrCodeFileStat:
		return "FileStat"
	case ErrCodeDestinationNotFound:
		return "DestinationNotFound"
	case ErrCodeDestinationDisabled:
		return "DestinationDisabled"
	case ErrCodeDestinationNil:
		return "DestinationNil"
	case ErrCodeChannelFull:
		return "ChannelFull"
	case ErrCodeChannelClosed:
		return "ChannelClosed"
	case ErrCodeInvalidConfig:
		return "InvalidConfig"
	case ErrCodeInvalidLevel:
		return "InvalidLevel"
	case ErrCodeInvalidFormat:
		return "InvalidFormat"
	case ErrCodeCompressionFailed:
		return "CompressionFailed"
	case ErrCodeCompressionQueueFull:
		return "CompressionQueueFull"
	case ErrCodeSyslogConnection:
		return "SyslogConnection"
	case ErrCodeSyslogWrite:
		return "SyslogWrite"
	case ErrCodeShutdownTimeout:
		return "ShutdownTimeout"
	case ErrCodeAlreadyClosed:
		return "AlreadyClosed"
	default:
		return fmt.Sprintf("ErrorCode(%d)", code)
	}
}

// String implements the Stringer interface for ErrorCode
func (e ErrorCode) String() string {
	return ErrorCodeString(e)
}

// Wrap wraps an error with additional context
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// Wrapf wraps an error with formatted message
func Wrapf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}

// IsFileError checks if an error is a file-related error
func IsFileError(err error) bool {
	var omniErr *OmniError
	if errors.As(err, &omniErr) {
		switch omniErr.Code {
		case ErrCodeFileOpen, ErrCodeFileClose, ErrCodeFileWrite,
			ErrCodeFileFlush, ErrCodeFileRotate, ErrCodeFileLock,
			ErrCodeFileUnlock, ErrCodeFileStat:
			return true
		}
	}
	return false
}

// IsDestinationError checks if an error is destination-related
func IsDestinationError(err error) bool {
	var omniErr *OmniError
	if errors.As(err, &omniErr) {
		switch omniErr.Code {
		case ErrCodeDestinationNotFound, ErrCodeDestinationDisabled,
			ErrCodeDestinationNil:
			return true
		}
	}
	return false
}

// IsConfigError checks if an error is configuration-related
func IsConfigError(err error) bool {
	var omniErr *OmniError
	if errors.As(err, &omniErr) {
		switch omniErr.Code {
		case ErrCodeInvalidConfig, ErrCodeInvalidLevel, ErrCodeInvalidFormat:
			return true
		}
	}
	return false
}

// IsTemporaryError checks if an error is temporary and can be retried
func IsTemporaryError(err error) bool {
	// Check if it's explicitly marked as retryable
	if IsRetryable(err) {
		return true
	}

	// Check specific error codes
	var omniErr *OmniError
	if errors.As(err, &omniErr) {
		switch omniErr.Code {
		case ErrCodeChannelFull, ErrCodeCompressionQueueFull,
			ErrCodeFileLock:
			return true
		}
	}

	// Check for specific error types
	return errors.Is(err, ErrChannelFull) ||
		errors.Is(err, ErrFlushTimeout)
}

// GetErrorCode extracts the error code from an error
func GetErrorCode(err error) ErrorCode {
	var omniErr *OmniError
	if errors.As(err, &omniErr) {
		return omniErr.Code
	}
	return ErrCodeUnknown
}

// GetErrorContext extracts context values from an error
func GetErrorContext(err error) map[string]interface{} {
	var omniErr *OmniError
	if errors.As(err, &omniErr) {
		return omniErr.Context
	}
	return nil
}

// ErrorBuilder provides a fluent interface for building errors
type ErrorBuilder struct {
	code    ErrorCode
	op      string
	path    string
	err     error
	dest    string
	context map[string]interface{}
}

// NewErrorBuilder creates a new error builder
func NewErrorBuilder(code ErrorCode) *ErrorBuilder {
	return &ErrorBuilder{
		code:    code,
		context: make(map[string]interface{}),
	}
}

// WithOp sets the operation
func (b *ErrorBuilder) WithOp(op string) *ErrorBuilder {
	b.op = op
	return b
}

// WithPath sets the path
func (b *ErrorBuilder) WithPath(path string) *ErrorBuilder {
	b.path = path
	return b
}

// WithError sets the underlying error
func (b *ErrorBuilder) WithError(err error) *ErrorBuilder {
	b.err = err
	return b
}

// WithDestination sets the destination
func (b *ErrorBuilder) WithDestination(dest string) *ErrorBuilder {
	b.dest = dest
	return b
}

// WithContext adds context
func (b *ErrorBuilder) WithContext(key string, value interface{}) *ErrorBuilder {
	b.context[key] = value
	return b
}

// Build creates the error
func (b *ErrorBuilder) Build() *OmniError {
	err := NewOmniError(b.code, b.op, b.path, b.err)
	if b.dest != "" {
		err.WithDestination(b.dest)
	}
	for k, v := range b.context {
		err.WithContext(k, v)
	}
	return err
}

// Common error creation helpers

// ErrInvalidParameter creates an invalid parameter error
func ErrInvalidParameter(param string, value interface{}) error {
	return NewErrorBuilder(ErrCodeInvalidConfig).
		WithOp("validate").
		WithContext("parameter", param).
		WithContext("value", value).
		Build()
}

// ErrOperationFailed creates an operation failed error
func ErrOperationFailed(op string, err error) error {
	return NewErrorBuilder(ErrCodeUnknown).
		WithOp(op).
		WithError(err).
		Build()
}