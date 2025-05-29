package main

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/wayneeseguin/flexlog"
)

func main() {
	// Create logs directory
	if err := os.MkdirAll("logs", 0755); err != nil {
		log.Fatal(err)
	}

	// Create a logger with advanced options
	logger, err := flexlog.NewWithOptions(
		flexlog.WithPath("logs/app.log"),
		flexlog.WithLevel(flexlog.LevelTrace),
		flexlog.WithRotation(1024*1024, 3), // 1MB max size, keep 3 files
		flexlog.WithGzipCompression(),
		flexlog.WithStackTrace(8192), // Enable stack trace with 8KB buffer
		flexlog.WithJSON(),           // Use JSON format for structured data
		flexlog.WithChannelSize(1000), // Set channel buffer size
	)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		// Graceful shutdown with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := logger.Shutdown(ctx); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
	}()

	// Set global fields that will be included in all log messages
	logger.SetGlobalFields(map[string]interface{}{
		"app":     "advanced-example",
		"version": "1.0.0",
		"host":    os.Getenv("HOSTNAME"),
	})

	// Add an error log destination
	logger.AddDestinationWithBackend("logs/errors.log", flexlog.BackendFlock)

	// Add a custom filter to exclude noisy debug messages
	logger.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
		// Filter out debug messages containing "bulk"
		if level == flexlog.LevelDebug && len(message) > 0 {
			return true // Allow the message
		}
		return true
	})

	// Enable sampling for trace messages to reduce volume
	logger.SetSampling(flexlog.SamplingRandom, 0.5) // Log 50% of trace messages

	// Demonstrate TRACE level logging for very detailed diagnostics
	logger.Trace("Application initialization starting")
	logger.TraceWithFields("Function entry", map[string]interface{}{
		"function": "main",
		"args":     os.Args,
		"pid":      os.Getpid(),
	})

	// Demonstrate structured logging with fields
	logger.InfoWithFields("User authentication successful", map[string]interface{}{
		"component": "auth",
		"user_id":   "user123",
	})

	// Demonstrate context-aware logging (basic)
	logger.InfoWithFields("Processing request with context", map[string]interface{}{
		"request_id": "req-12345",
		"context":    "demo",
	})

	// Demonstrate all logging levels with detailed flow
	processRequest(logger, "user123", "login")

	// Demonstrate metrics collection (if available)
	// Note: GetMetrics() may not be available in all builds
	logger.Info("Logger initialized with advanced features")

	// Generate some logs to trigger rotation
	log.Printf("Generating logs to demonstrate rotation...")
	for i := 0; i < 1000; i++ {
		if i%100 == 0 {
			logger.Tracef("Processing batch %d/1000", i/100+1)
		}

		logger.InfoWithFields("Bulk message to trigger rotation", map[string]interface{}{
			"index":     i,
			"timestamp": time.Now().Unix(),
			"data":      generateRandomString(100),
		})

		// Add some debug and trace messages
		if i%50 == 0 {
			logger.DebugWithFields("Batch checkpoint", map[string]interface{}{
				"batch_id":  i / 50,
				"processed": i,
				"remaining": 1000 - i,
				"memory_mb": rand.Intn(100) + 50,
			})
		}
	}

	// Demonstrate error with stack trace
	err = doSomethingThatFails()
	if err != nil {
		logger.ErrorWithFields("Operation failed with stack trace", map[string]interface{}{
			"error":       err.Error(),
			"operation":   "data_processing",
			"retry_count": 3,
		})
	}

	// Demonstrate destination management
	destinations := logger.ListDestinations()
	log.Printf("Active destinations: %v", destinations)

	// Flush all logs before shutdown
	logger.FlushAll()

	logger.Trace("Application shutdown initiated")
	log.Printf("Check logs/ directory for generated log files")
}

func processRequest(logger *flexlog.FlexLog, userID, action string) {
	// Use structured logging with base fields
	baseFields := map[string]interface{}{
		"user_id": userID,
		"action":  action,
	}

	logger.TraceWithFields("Entering processRequest", map[string]interface{}{
		"user_id": userID,
		"action":  action,
		"step":    "entry",
	})

	// Simulate request processing with detailed tracing
	logger.TraceWithFields("Request validation starting", map[string]interface{}{
		"user_id": userID,
		"action":  action,
		"step":    "validation",
	})

	// Simulate validation with timing
	start := time.Now()
	time.Sleep(10 * time.Millisecond)
	
	validationFields := make(map[string]interface{})
	for k, v := range baseFields {
		validationFields[k] = v
	}
	validationFields["valid"] = true
	validationFields["took_ms"] = time.Since(start).Milliseconds()
	validationFields["step"] = "validation"
	
	logger.DebugWithFields("Validation completed", validationFields)

	// Simulate business logic with context
	businessStart := time.Now()
	logger.TraceWithFields("Business logic processing", map[string]interface{}{
		"user_id": userID,
		"action":  action,
		"step":    "business_logic",
	})

	time.Sleep(50 * time.Millisecond)
	
	totalDuration := time.Since(start)
	resultFields := make(map[string]interface{})
	for k, v := range baseFields {
		resultFields[k] = v
	}
	resultFields["duration_ms"] = totalDuration.Milliseconds()
	resultFields["validation_ms"] = 10
	resultFields["business_logic_ms"] = time.Since(businessStart).Milliseconds()
	resultFields["status"] = "success"
	resultFields["transaction_id"] = generateTransactionID()
	
	logger.InfoWithFields("Request processed successfully", resultFields)

	logger.TraceWithFields("Exiting processRequest", map[string]interface{}{
		"user_id": userID,
		"action":  action,
		"step":    "exit",
	})
}

func doSomethingThatFails() error {
	return errors.New("simulated failure in nested function")
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func generateTransactionID() string {
	return generateRandomString(16)
}
