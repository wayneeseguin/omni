package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/wayneeseguin/flexlog"
)

func main() {
	// Example 1: Using the Builder Pattern
	builderExample()

	// Example 2: Using Functional Options
	optionsExample()

	// Example 3: Using Interfaces
	interfaceExample()

	// Example 4: Error Handling
	errorHandlingExample()
}

func builderExample() {
	fmt.Println("=== Builder Pattern Example ===")

	logger, err := flexlog.NewBuilder().
		WithLevel(flexlog.LevelInfo).
		WithJSON().
		WithDestination("/tmp/app.log",
			flexlog.WithBatching(8192, 100*time.Millisecond),
			flexlog.WithDestinationName("primary")).
		WithSyslogDestination("syslog://localhost:514",
			flexlog.WithDestinationName("syslog")).
		WithRotation(100*1024*1024, 10). // 100MB, keep 10 files
		WithCompression(flexlog.CompressionGzip, 2).
		WithErrorHandler(flexlog.StderrErrorHandler).
		WithTimezone(time.UTC).
		WithStackTrace(true, 4096).
		WithFilter(func(level int, message string, fields map[string]interface{}) bool {
			// Filter out debug messages in production
			return level >= flexlog.LevelInfo
		}).
		Build()

	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Use the logger
	logger.Info("Application started")
	logger.WithFields(map[string]interface{}{
		"version": "1.0.0",
		"env":     "production",
	}).Info("Configuration loaded")
}

func optionsExample() {
	fmt.Println("\n=== Functional Options Example ===")

	// Production configuration
	prodLogger, err := flexlog.NewWithOptions(
		flexlog.WithPath("/var/log/app.log"),
		flexlog.WithProductionDefaults(),
		flexlog.WithRotation(100*1024*1024, 10),
		flexlog.WithGzipCompression(),
		flexlog.WithRateSampling(0.1), // Sample 10% of debug logs
		flexlog.WithRedaction([]string{
			`\b\d{16}\b`,              // Credit card numbers
			`\b[A-Za-z0-9+/]{40}\b`,   // API keys
		}, "[REDACTED]"),
		flexlog.WithRecovery("/var/log/app-fallback.log", 3),
	)
	if err != nil {
		log.Fatalf("Failed to create production logger: %v", err)
	}
	defer prodLogger.Close()

	// Development configuration
	devLogger, err := flexlog.NewWithOptions(
		flexlog.WithPath("./dev.log"),
		flexlog.WithDevelopmentDefaults(),
		flexlog.WithLevelFilter(flexlog.LevelDebug),
		flexlog.WithTimestampFormat("15:04:05.000"),
	)
	if err != nil {
		log.Fatalf("Failed to create dev logger: %v", err)
	}
	defer devLogger.Close()

	// Use the loggers
	prodLogger.Info("Production logger initialized")
	devLogger.Debug("Development logger initialized with detailed output")
}

func interfaceExample() {
	fmt.Println("\n=== Interface Example ===")

	// Create a logger
	flexLogger, err := flexlog.New("/tmp/interface-example.log")
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer flexLogger.Close()

	// Use it through the Logger interface
	var logger flexlog.Logger = flexlog.NewLoggerAdapter(flexLogger)

	// Now you can pass this logger to functions that expect the Logger interface
	doWork(logger)

	// Use it through the Manager interface for configuration
	var manager flexlog.Manager = flexLogger
	manager.SetMaxSize(50 * 1024 * 1024)
	manager.SetCompression(flexlog.CompressionGzip)

	// Add another destination
	err = manager.AddDestination("/tmp/interface-example-2.log")
	if err != nil {
		log.Printf("Failed to add destination: %v", err)
	}

	// Get metrics
	metrics := manager.GetMetrics()
	fmt.Printf("Messages logged: %+v\n", metrics.MessagesLogged)

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := manager.Shutdown(ctx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
}

func doWork(logger flexlog.Logger) {
	// Function that accepts the Logger interface
	logger.Info("Starting work")
	
	// Structured logging with fields
	logger.WithFields(map[string]interface{}{
		"task":     "processing",
		"duration": "5s",
	}).Info("Task completed")

	// Logging with error
	err := fmt.Errorf("sample error")
	logger.WithError(err).Error("Operation failed")

	// Check if debug is enabled before expensive operation
	if logger.IsLevelEnabled(flexlog.LevelDebug) {
		debugData := expensiveDebugOperation()
		logger.Debug("Debug data: ", debugData)
	}
}

func expensiveDebugOperation() string {
	// Simulate expensive operation
	time.Sleep(10 * time.Millisecond)
	return "detailed debug information"
}

func errorHandlingExample() {
	fmt.Println("\n=== Error Handling Example ===")

	// Try to create logger with invalid configuration
	_, err := flexlog.NewBuilder().
		WithLevel(-1). // Invalid level
		Build()

	if err != nil {
		// Check error type
		if flexlog.IsConfigError(err) {
			fmt.Println("Configuration error detected:", err)
		}

		// Get error code
		code := flexlog.GetErrorCode(err)
		fmt.Printf("Error code: %s\n", code)

		// Get error context
		context := flexlog.GetErrorContext(err)
		fmt.Printf("Error context: %+v\n", context)
	}

	// Create logger with error handler
	logger, err := flexlog.NewBuilder().
		WithPath("/tmp/error-example.log").
		WithErrorHandler(func(logErr flexlog.LogError) {
			// Custom error handling
			fmt.Printf("Logger error: [%s] %s: %v\n", 
				logErr.Time.Format("15:04:05"),
				logErr.Source,
				logErr.Err)
		}).
		Build()

	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Get error channel for monitoring
	errorChan := logger.GetErrors()
	go func() {
		for logErr := range errorChan {
			fmt.Printf("Received error: %v\n", logErr)
		}
	}()

	// Simulate an error by closing the file
	logger.Close()
	logger.Info("This will cause an error")
}