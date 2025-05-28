package flexlog

import "errors"

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
)