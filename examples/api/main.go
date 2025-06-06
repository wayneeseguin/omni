package main

import (
	"fmt"
	"log"
	"time"

	"github.com/wayneeseguin/omni/pkg/omni"
)

func main() {
	// Example 1: Basic API Usage
	basicExample()

	// Example 2: Using Functional Options
	optionsExample()

	// Example 3: Advanced Configuration
	advancedExample()

	// Example 4: Error Handling
	errorHandlingExample()
}

func basicExample() {
	fmt.Println("=== Basic API Usage Example ===")

	// Create a basic logger
	logger, err := omni.New("/tmp/basic_api.log")
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Configure basic settings
	logger.SetLevel(omni.LevelInfo)
	logger.SetFormat(omni.FormatJSON)

	// Basic logging methods
	logger.Trace("This is a trace message")
	logger.Debug("This is a debug message") 
	logger.Info("This is an info message")
	logger.Warn("This is a warning message")
	logger.Error("This is an error message")

	// Formatted logging
	logger.Infof("User %s logged in at %s", "john_doe", time.Now().Format(time.RFC3339))

	// Structured logging with fields
	logger.InfoWithFields("User action performed", map[string]interface{}{
		"user_id":   "12345",
		"action":    "login", 
		"timestamp": time.Now().Unix(),
		"success":   true,
	})

	fmt.Println("✓ Basic API example completed")
}

func optionsExample() {
	fmt.Println("\n=== Functional Options Example ===")

	// Production configuration with options
	prodLogger, err := omni.NewWithOptions(
		omni.WithPath("/tmp/api_prod.log"),
		omni.WithLevel(omni.LevelInfo),
		omni.WithRotation(50*1024*1024, 5), // 50MB, keep 5 files
		omni.WithGzipCompression(),
		omni.WithJSON(),
		omni.WithChannelSize(1000),
	)
	if err != nil {
		log.Fatalf("Failed to create production logger: %v", err)
	}
	defer prodLogger.Close()

	// Development configuration with options
	devLogger, err := omni.NewWithOptions(
		omni.WithPath("/tmp/api_dev.log"),
		omni.WithLevel(omni.LevelTrace),
		omni.WithText(),
		omni.WithStackTrace(4096),
	)
	if err != nil {
		log.Fatalf("Failed to create dev logger: %v", err)
	}
	defer devLogger.Close()

	// Use the loggers
	prodLogger.Info("Production logger initialized with options")
	devLogger.Debug("Development logger initialized with detailed output")
	
	// Add filters to production logger
	prodLogger.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
		// Only allow INFO and above in production
		return level >= omni.LevelInfo
	})

	prodLogger.Debug("This debug message will be filtered out")
	prodLogger.Info("This info message will be logged")

	fmt.Println("✓ Functional options example completed")
}

func advancedExample() {
	fmt.Println("\n=== Advanced Configuration Example ===")

	// Create a logger with advanced configuration
	logger, err := omni.NewWithOptions(
		omni.WithPath("/tmp/api_advanced.log"),
		omni.WithLevel(omni.LevelDebug),
		omni.WithJSON(),
		omni.WithRotation(10*1024*1024, 3), // 10MB, keep 3 files
		omni.WithGzipCompression(),
		omni.WithStackTrace(4096), // Enable stack traces with 4KB buffer
	)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Add multiple destinations
	err = logger.AddDestination("/tmp/api_advanced_copy.log")
	if err != nil {
		log.Printf("Failed to add destination: %v", err)
	}

	// Test destination management
	destinations := logger.ListDestinations()
	fmt.Printf("Active destinations: %v\n", destinations)

	// Disable and re-enable a destination
	logger.DisableDestination("/tmp/api_advanced_copy.log")
	logger.Info("This message won't go to the disabled destination")
	
	logger.EnableDestination("/tmp/api_advanced_copy.log")
	logger.Info("This message will go to the re-enabled destination")

	// Use the logger with various methods
	doWork(logger)

	fmt.Println("✓ Advanced configuration example completed")
}

func doWork(logger *omni.Omni) {
	// Function that works with the Omni instance
	logger.Info("Starting work")
	
	// Structured logging with fields
	logger.InfoWithFields("Task completed", map[string]interface{}{
		"task":     "processing",
		"duration": "5s",
		"worker":   "api_example",
	})

	// Logging with error information
	err := fmt.Errorf("sample error for demonstration")
	logger.ErrorWithFields("Operation encountered an error", map[string]interface{}{
		"error":     err.Error(),
		"operation": "sample_work",
		"retryable": true,
	})

	// Generate some debug data
	debugData := expensiveDebugOperation()
	logger.DebugWithFields("Debug information collected", map[string]interface{}{
		"debug_data": debugData,
		"source":     "doWork",
	})

	// Test different log levels
	logger.Trace("Detailed trace information")
	logger.Debug("Debug information")
	logger.Info("General information")
	logger.Warn("Warning message")
	logger.Error("Error message")
}

func expensiveDebugOperation() string {
	// Simulate expensive operation
	time.Sleep(10 * time.Millisecond)
	return "detailed debug information"
}

func errorHandlingExample() {
	fmt.Println("\n=== Error Handling Example ===")

	// Test error handling with invalid path
	_, err := omni.New("/invalid/path/that/does/not/exist/test.log")
	if err != nil {
		fmt.Printf("Expected error for invalid path: %v\n", err)
	}

	// Create logger successfully
	logger, err := omni.New("/tmp/api_error_example.log")
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test various error conditions
	logger.Info("Testing error handling")

	// Test invalid level setting (should handle gracefully)
	logger.SetLevel(omni.LevelError)
	logger.Debug("This debug message should be filtered")
	logger.Error("This error message should be logged")

	// Test error recovery
	logger.ErrorWithFields("Simulated application error", map[string]interface{}{
		"error_type":   "demonstration",
		"error_code":   500,
		"recoverable":  true,
		"timestamp":    time.Now().Unix(),
	})

	// Test logging after errors
	logger.Info("Logger continues to work after errors")

	// Test flush and sync operations
	logger.FlushAll()
	logger.Sync()

	fmt.Println("✓ Error handling example completed")
}