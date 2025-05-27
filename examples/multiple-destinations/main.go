package main

import (
	"log"
	"time"

	"github.com/wayneeseguin/flexlog"
)

func main() {
	// Create logger with custom configuration
	config := flexlog.Config{
		ChannelSize:   2000,
		DefaultLevel:  flexlog.INFO,
		EnableMetrics: true,
	}

	logger, err := flexlog.NewFlexLogWithConfig(config)
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Close()

	// Add JSON file destination for all logs
	err = logger.AddDestination("json-all", flexlog.DestinationConfig{
		Backend:    flexlog.BackendFile,
		FilePath:   "logs/all.json",
		Format:     flexlog.FormatJSON,
		MinLevel:   flexlog.DEBUG,
		MaxSize:    10 * 1024 * 1024, // 10MB
		MaxBackups: 3,
		Compress:   true,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Add text file destination for errors only
	err = logger.AddDestination("errors-only", flexlog.DestinationConfig{
		Backend:  flexlog.BackendFile,
		FilePath: "logs/errors.log",
		Format:   flexlog.FormatText,
		MinLevel: flexlog.ERROR,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Add syslog destination for warnings and above
	err = logger.AddDestination("syslog", flexlog.DestinationConfig{
		Backend:  flexlog.BackendSyslog,
		MinLevel: flexlog.WARN,
	})
	if err != nil {
		log.Printf("Failed to add syslog destination: %v", err)
	}

	// Simulate application activity
	for i := 0; i < 100; i++ {
		logger.Debug("Processing item",
			"item_id", i,
			"timestamp", time.Now(),
		)

		if i%10 == 0 {
			logger.Info("Progress update",
				"processed", i,
				"total", 100,
				"percentage", i,
			)
		}

		if i%25 == 0 {
			logger.Warn("Slow processing detected",
				"item_id", i,
				"duration_ms", 150,
			)
		}

		if i == 50 {
			logger.Error("Failed to process item",
				"item_id", i,
				"error", "database timeout",
				"retry_attempt", 1,
			)
		}

		time.Sleep(10 * time.Millisecond)
	}

	// Display metrics
	metrics := logger.GetMetrics()
	log.Printf("Logging metrics:")
	log.Printf("  Total messages: %d", metrics.TotalMessages)
	log.Printf("  Dropped messages: %d", metrics.DroppedMessages)
	log.Printf("  Messages by level:")
	for level, count := range metrics.MessagesByLevel {
		log.Printf("    %s: %d", level, count)
	}
}