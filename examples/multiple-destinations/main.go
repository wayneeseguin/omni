package main

import (
	"log"
	"os"
	"time"

	"github.com/wayneeseguin/omni/pkg/omni"
)

func main() {
	// Create logs directory
	// #nosec G301 - Example code, 0755 permissions are acceptable
	if err := os.MkdirAll("logs", 0755); err != nil {
		log.Fatal(err)
	}

	// Create primary logger for all logs
	logger, err := omni.New("logs/all.log")
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Close()

	// Set level to TRACE to demonstrate all levels
	logger.SetLevel(omni.LevelTrace)

	// Add additional destinations to the same logger
	// Add an error-only destination
	err = logger.AddDestination("logs/errors.log")
	if err != nil {
		log.Fatal(err)
	}

	// Add a JSON-formatted destination
	err = logger.AddDestination("logs/structured.log")
	if err != nil {
		log.Fatal(err)
	}

	// Add an audit destination for important events
	err = logger.AddDestination("logs/audit.log")
	if err != nil {
		log.Fatal(err)
	}

	// List active destinations
	destinations := logger.ListDestinations()
	log.Printf("Active destinations: %v", destinations)

	log.Printf("Demonstrating multiple destinations with single logger...")

	// Simulate application activity with all log levels
	for i := 0; i < 50; i++ {
		// TRACE level - very detailed diagnostics (goes to all destinations)
		logger.TraceWithFields("Processing item trace", map[string]interface{}{
			"item_id":     i,
			"step":        "initialization",
			"memory_mb":   45 + i%10,
			"destination": "all",
		})

		// DEBUG level - detailed diagnostics (goes to all destinations)
		logger.DebugWithFields("Processing item debug", map[string]interface{}{
			"item_id":     i,
			"timestamp":   time.Now().Unix(),
			"destination": "all",
		})

		// INFO level - general information (goes to all destinations)
		if i%10 == 0 {
			logger.InfoWithFields("Progress update", map[string]interface{}{
				"processed":   i,
				"total":       50,
				"percentage":  (i * 100) / 50,
				"destination": "all",
			})
		}

		// WARN level - warnings (goes to all destinations)
		if i%15 == 0 {
			logger.WarnWithFields("Slow processing detected", map[string]interface{}{
				"item_id":     i,
				"duration_ms": 150,
				"severity":    "warning",
				"destination": "all",
			})
		}

		// ERROR level - errors (goes to all destinations)
		if i == 25 {
			logger.ErrorWithFields("Failed to process item", map[string]interface{}{
				"item_id":       i,
				"error":         "database timeout",
				"retry_attempt": 1,
				"severity":      "error",
				"destination":   "all",
			})
		}

		// Audit events - special structured logs
		if i%20 == 0 {
			logger.InfoWithFields("Audit event", map[string]interface{}{
				"event_type": "progress_checkpoint",
				"item_id":    i,
				"user_id":    "system",
				"timestamp":  time.Now().Format(time.RFC3339),
				"category":   "audit",
			})
		}

		time.Sleep(10 * time.Millisecond)
	}

	// Demonstrate destination management
	_ = logger.DisableDestination("logs/structured.log") //nolint:gosec
	logger.Info("This message will NOT go to structured.log (disabled)")

	_ = logger.EnableDestination("logs/structured.log") //nolint:gosec
	logger.Info("This message WILL go to structured.log (re-enabled)")

	// Demonstrate trace level for function flow
	logger.Trace("Application processing completed")
	logger.TraceWithFields("Cleanup starting", map[string]interface{}{
		"items_processed": 50,
		"cleanup_steps":   []string{"close_files", "free_memory"},
		"final_status":    "success",
	})

	// Flush all destinations before shutdown
	_ = logger.FlushAll() //nolint:gosec

	log.Printf("Logging complete! Check the following files:")
	log.Printf("  logs/all.log - Primary destination with all log levels")
	log.Printf("  logs/errors.log - Secondary destination (same content)")
	log.Printf("  logs/structured.log - Third destination (same content)")
	log.Printf("  logs/audit.log - Fourth destination (same content)")
	log.Printf("All destinations receive the same messages at the logger's level")
}
