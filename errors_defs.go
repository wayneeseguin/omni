package flexlog

import (
	"errors"
	"fmt"
)

// Common errors returned by FlexLog operations
var (
	// ErrLoggerClosed is returned when attempting to use a closed logger
	ErrLoggerClosed = errors.New("logger is closed")

	// ErrInvalidDestination is returned when a destination is invalid
	ErrInvalidDestination = errors.New("invalid destination")

	// ErrDestinationExists is returned when trying to add a duplicate destination
	ErrDestinationExists = errors.New("destination already exists")

	// ErrInvalidIndex is returned when a destination index is out of bounds
	ErrInvalidIndex = errors.New("invalid destination index")

	// ErrChannelFull is returned when the message channel is full
	ErrChannelFull = errors.New("message channel full")

	// ErrNilWriter is returned when attempting to write to a nil writer
	ErrNilWriter = errors.New("writer is nil")

	// ErrRotationFailed is returned when log rotation fails
	ErrRotationFailed = errors.New("log rotation failed")

	// ErrCompressionFailed is returned when log compression fails
	ErrCompressionFailed = errors.New("log compression failed")

	// ErrInvalidConfiguration is returned when configuration is invalid
	ErrInvalidConfiguration = errors.New("invalid configuration")
)

// FileError represents an error related to file operations
type FileError struct {
	Op   string // Operation that failed
	Path string // File path
	Err  error  // Underlying error
}

func (e *FileError) Error() string {
	return fmt.Sprintf("file %s error on %s: %v", e.Op, e.Path, e.Err)
}

func (e *FileError) Unwrap() error {
	return e.Err
}

// DestinationError represents an error related to a specific destination
type DestinationError struct {
	Name string // Destination name
	Op   string // Operation that failed
	Err  error  // Underlying error
}

func (e *DestinationError) Error() string {
	return fmt.Sprintf("destination %s: %s failed: %v", e.Name, e.Op, e.Err)
}

func (e *DestinationError) Unwrap() error {
	return e.Err
}

// ConfigError represents a configuration error
type ConfigError struct {
	Field string      // Configuration field
	Value interface{} // Invalid value
	Err   error       // Underlying error
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("config error: field %s with value %v: %v", e.Field, e.Value, e.Err)
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}
