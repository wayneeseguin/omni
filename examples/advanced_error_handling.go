package main

import (
	"fmt"
	"os"

	"github.com/wayneeseguin/flocklogger"
)

// Custom error types
type NotFoundError struct {
	Resource string
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf("resource not found: %s", e.Resource)
}

// Function with nested errors
func openConfig(path string) error {
	_, err := os.Open(path)
	if err != nil {
		// Wrap the error with context
		return fmt.Errorf("failed to open config file: %w", err)
	}
	return nil
}

// Function that uses the wrapped error
func loadConfiguration(logger *flocklogger.FlockLogger, configPath string) error {
	err := openConfig(configPath)
	if err != nil {
		// Wrap with stack trace
		return logger.WrapError(err, "configuration loading failed")
	}
	return nil
}

// Function that demonstrates error severity
func processData(logger *flocklogger.FlockLogger, data []byte) error {
	if len(data) == 0 {
		return logger.WrapErrorWithSeverity(
			NotFoundError{Resource: "data"},
			"empty data provided",
			flocklogger.SeverityMedium,
		)
	}
	return nil
}

func main() {
	// Create logger
	logger, err := flocklogger.NewFlockLogger("./logs/example.log")
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		return
	}
	defer logger.Close()

	// Enable stack traces
	logger.EnableStackTraces(true)

	// Set JSON format for structured logging
	logger.SetFormat(flocklogger.FormatJSON)

	// Example 1: Simple error logging
	if err := loadConfiguration(logger, "/nonexistent/config.json"); err != nil {
		// Log the error with enhanced details
		logger.ErrorWithError("Main configuration error", err)
	}

	// Example 2: Error with severity
	if err := processData(logger, []byte{}); err != nil {
		// Get the root cause
		rootCause := logger.CauseOf(err)
		logger.ErrorWithFields("Data processing failed", map[string]interface{}{
			"root_cause":   rootCause.Error(),
			"is_not_found": logger.IsErrorType(rootCause, NotFoundError{Resource: ""}),
		})
	}

	// Example 3: Safe goroutine with panic recovery
	logger.SafeGo(func() {
		// This will panic but be safely logged
		var nilSlice []string
		fmt.Println(nilSlice[0]) // Will cause panic
	})

	// Wait a moment for goroutine to complete
	fmt.Println("Program completed, check logs for details")
}
