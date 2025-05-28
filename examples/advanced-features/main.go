package main

import (
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

	// Create a logger with file destination
	logger, err := flexlog.New("logs/app.log")
	if err != nil {
		log.Fatal(err)
	}
	defer logger.CloseAll()

	// Set level to TRACE to see all detailed diagnostic information
	logger.SetLevel(flexlog.LevelTrace)

	// Enable rotation for the logger
	logger.SetMaxSize(1024 * 1024) // 1MB for demo purposes
	logger.SetMaxFiles(3)
	logger.SetCompression(flexlog.CompressionGzip)

	// Enable stack traces for errors
	logger.EnableStackTraces(true)

	// Demonstrate TRACE level logging for very detailed diagnostics
	logger.Trace("Application initialization starting")
	logger.TraceWithFields("Function entry", map[string]interface{}{
		"function": "main",
		"args":     os.Args,
		"pid":      os.Getpid(),
	})

	// Demonstrate all logging levels with detailed flow
	processRequest("user123", "login")

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
				"batch_id":    i / 50,
				"processed":   i,
				"remaining":   1000 - i,
				"memory_mb":   rand.Intn(100) + 50,
			})
		}
	}

	// Demonstrate error with stack trace
	err = doSomethingThatFails()
	if err != nil {
		logger.ErrorWithFields("Operation failed", map[string]interface{}{
			"error":       err.Error(),
			"operation":   "data_processing",
			"retry_count": 3,
		})
	}

	logger.Trace("Application shutdown initiated")
	log.Printf("Check logs/ directory for generated log files")
}

func processRequest(userID, action string) {
	// Get logger from main or create a new instance for demonstration
	logger, _ := flexlog.New("logs/app.log")
	defer logger.CloseAll()
	logger.SetLevel(flexlog.LevelTrace)

	logger.Tracef("Entering processRequest: user=%s, action=%s", userID, action)
	
	// Simulate request processing with detailed tracing
	logger.TraceWithFields("Request validation", map[string]interface{}{
		"user_id": userID,
		"action":  action,
		"step":    "validation",
	})
	
	// Simulate validation
	time.Sleep(10 * time.Millisecond)
	logger.DebugWithFields("Validation completed", map[string]interface{}{
		"user_id": userID,
		"valid":   true,
		"took_ms": 10,
	})
	
	// Simulate business logic
	logger.TraceWithFields("Business logic processing", map[string]interface{}{
		"user_id": userID,
		"action":  action,
		"step":    "business_logic",
	})
	
	time.Sleep(50 * time.Millisecond)
	logger.InfoWithFields("Request processed successfully", map[string]interface{}{
		"user_id":     userID,
		"action":      action,
		"duration_ms": 60,
		"status":      "success",
	})
	
	logger.Tracef("Exiting processRequest: user=%s, action=%s", userID, action)
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