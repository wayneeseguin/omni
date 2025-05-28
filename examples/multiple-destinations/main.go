package main

import (
	"log"
	"os"
	"time"

	"github.com/wayneeseguin/flexlog"
)

func main() {
	// Create logs directory
	if err := os.MkdirAll("logs", 0755); err != nil {
		log.Fatal(err)
	}

	// Create primary logger for all logs
	logger, err := flexlog.New("logs/all.log")
	if err != nil {
		log.Fatal(err)
	}
	defer logger.CloseAll()

	// Set level to TRACE to demonstrate all levels
	logger.SetLevel(flexlog.LevelTrace)

	// Create error-specific logger
	errorLogger, err := flexlog.New("logs/errors.log")
	if err != nil {
		log.Fatal(err)
	}
	defer errorLogger.CloseAll()

	// Set error logger to only log warnings and errors
	errorLogger.SetLevel(flexlog.LevelWarn)

	log.Printf("Demonstrating multiple destinations with different log levels...")

	// Simulate application activity with all log levels
	for i := 0; i < 50; i++ {
		// TRACE level - very detailed diagnostics (only to all.log)
		logger.TraceWithFields("Processing item trace", map[string]interface{}{
			"item_id":   i,
			"step":      "initialization",
			"memory_mb": 45 + i%10,
		})

		// DEBUG level - detailed diagnostics (only to all.log)
		logger.DebugWithFields("Processing item debug", map[string]interface{}{
			"item_id":   i,
			"timestamp": time.Now().Unix(),
		})

		// INFO level - general information (only to all.log)
		if i%10 == 0 {
			logger.InfoWithFields("Progress update", map[string]interface{}{
				"processed":  i,
				"total":      50,
				"percentage": (i * 100) / 50,
			})
		}

		// WARN level - warnings (to both all.log and errors.log)
		if i%15 == 0 {
			// Log to both destinations
			logger.WarnWithFields("Slow processing detected", map[string]interface{}{
				"item_id":     i,
				"duration_ms": 150,
			})
			
			errorLogger.WarnWithFields("Slow processing detected", map[string]interface{}{
				"item_id":     i,
				"duration_ms": 150,
			})
		}

		// ERROR level - errors (to both all.log and errors.log)
		if i == 25 {
			// Log to both destinations
			logger.ErrorWithFields("Failed to process item", map[string]interface{}{
				"item_id":       i,
				"error":         "database timeout",
				"retry_attempt": 1,
			})
			
			errorLogger.ErrorWithFields("Failed to process item", map[string]interface{}{
				"item_id":       i,
				"error":         "database timeout",
				"retry_attempt": 1,
			})
		}

		time.Sleep(10 * time.Millisecond)
	}

	// Demonstrate trace level for function flow
	logger.Trace("Application processing completed")
	logger.TraceWithFields("Cleanup starting", map[string]interface{}{
		"items_processed": 50,
		"cleanup_steps":   []string{"close_files", "free_memory"},
	})

	log.Printf("Logging complete! Check the following files:")
	log.Printf("  logs/all.log - Contains ALL log levels (TRACE through ERROR)")
	log.Printf("  logs/errors.log - Contains only WARN and ERROR levels")
}