package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wayneeseguin/flexlog"
	natsplugin "github.com/wayneeseguin/flexlog/examples/plugins/nats-backend"
)

func main() {
	// Register the NATS backend plugin
	plugin := &natsplugin.NATSBackendPlugin{}
	if err := plugin.Initialize(nil); err != nil {
		log.Fatalf("Failed to initialize NATS plugin: %v", err)
	}
	
	if err := flexlog.RegisterBackendPlugin(plugin); err != nil {
		log.Fatalf("Failed to register NATS plugin: %v", err)
	}
	
	// Create a new FlexLog instance with builder
	logger, err := flexlog.NewBuilder().
		WithLevel(flexlog.LevelDebug).
		WithJSON().
		Build()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Add a local file destination for backup
	if err := logger.AddDestination("logs/app.log"); err != nil {
		log.Fatalf("Failed to add file destination: %v", err)
	}

	// Add NATS destination for distributed logging
	natsURI := os.Getenv("NATS_URI")
	if natsURI == "" {
		natsURI = "nats://localhost:4222/logs.myapp"
	}

	fmt.Printf("Connecting to NATS at %s\n", natsURI)
	
	// Example URIs:
	// Basic: "nats://localhost:4222/logs.myapp"
	// With queue group: "nats://localhost:4222/logs.myapp?queue=workers"
	// With batching: "nats://localhost:4222/logs.myapp?batch=200&flush_interval=100"
	// With auth: "nats://user:pass@nats-server:4222/logs.myapp"
	// With TLS: "nats://secure-nats:4222/logs.myapp?tls=true"
	
	if err := logger.AddDestination(natsURI); err != nil {
		log.Fatalf("Failed to add NATS destination: %v", err)
	}

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

func simulateWork(logger *flexlog.FlexLog) {
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
				"total_requests":  requestCounter,
				"total_errors":    errorCounter,
				"error_rate":      float64(errorCounter) / float64(requestCounter),
				"uptime_seconds":  requestCounter, // Simplified
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