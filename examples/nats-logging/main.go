package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wayneeseguin/omni/pkg/omni"
)

func main() {
	// Create a logger with JSON format for structured logging
	// Note: This example demonstrates distributed logging patterns
	// In production, you would connect this to actual NATS backend
	logger, err := omni.NewWithOptions(
		omni.WithPath("logs/distributed.log"),
		omni.WithLevel(omni.LevelDebug),
		omni.WithJSON(),
		omni.WithRotation(10*1024*1024, 5), // 10MB files, keep 5
	)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Add a backup destination
	if err := logger.AddDestination("logs/app_backup.log"); err != nil {
		log.Fatalf("Failed to add backup destination: %v", err)
	}

	fmt.Println("Distributed logging example started (file-based simulation)")
	fmt.Println("In production, this would connect to NATS for distributed logging")

	// Log some messages with fields
	logger.InfoWithFields("Application started", map[string]interface{}{
		"service": "example-app",
		"version": "1.0.0",
	})
	logger.Debug("Debug mode enabled")

	// Simulate application work
	go simulateWork(logger)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	fmt.Println("Application running. Press Ctrl+C to stop.")
	<-sigChan

	logger.Info("Shutting down application")
}

func simulateWork(logger *omni.Omni) {
	requestCounter := 0
	errorCounter := 0

	for {
		requestCounter++

		// Simulate a request
		requestFields := map[string]interface{}{
			"request_id": fmt.Sprintf("req-%d", requestCounter),
			"method":     "GET",
			"path":       "/api/users",
		}

		logger.InfoWithFields("Processing request", requestFields)

		// Simulate processing time
		processingTime := time.Duration(100+requestCounter%400) * time.Millisecond
		time.Sleep(processingTime)

		// Simulate occasional errors
		if requestCounter%10 == 0 {
			errorCounter++
			errorFields := mergeMaps(requestFields, map[string]interface{}{
				"error":           "Database connection timeout",
				"retry_count":     3,
				"processing_time": processingTime.Milliseconds(),
			})
			logger.ErrorWithFields("Request failed", errorFields)
		} else {
			successFields := mergeMaps(requestFields, map[string]interface{}{
				"status_code":     200,
				"processing_time": processingTime.Milliseconds(),
				"response_size":   1024 + requestCounter%2048,
			})
			logger.InfoWithFields("Request completed", successFields)
		}

		// Log metrics periodically
		if requestCounter%20 == 0 {
			logger.InfoWithFields("Application metrics", map[string]interface{}{
				"total_requests": requestCounter,
				"total_errors":   errorCounter,
				"error_rate":     float64(errorCounter) / float64(requestCounter),
				"uptime_seconds": requestCounter, // Simplified
			})
		}

		// Simulate varying load
		time.Sleep(time.Duration(500+requestCounter%1000) * time.Millisecond)
	}
}

// mergeMaps merges two maps, with values from the second map overriding the first
func mergeMaps(m1, m2 map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m1 {
		result[k] = v
	}
	for k, v := range m2 {
		result[k] = v
	}
	return result
}
