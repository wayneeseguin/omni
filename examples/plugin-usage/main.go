package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/wayneeseguin/omni/pkg/omni"
)

func main() {
	// Example 1: Using different formatters
	fmt.Println("=== Example 1: JSON Formatter ===")
	
	// Create logger with JSON formatter
	jsonLogger, err := omni.NewWithOptions(
		omni.WithPath("/tmp/plugin_json.log"),
		omni.WithLevel(omni.LevelDebug),
		omni.WithJSON(),
	)
	if err != nil {
		log.Fatalf("Failed to create JSON logger: %v", err)
	}
	defer jsonLogger.Close()

	// Log structured data with JSON format
	jsonLogger.InfoWithFields("JSON formatting example", map[string]interface{}{
		"user_id":    "12345",
		"action":     "login",
		"success":    true,
		"timestamp":  time.Now().Unix(),
	})

	fmt.Println("✓ JSON formatter example completed")

	// Example 2: Using text formatter
	fmt.Println("\n=== Example 2: Text Formatter ===")
	
	textLogger, err := omni.NewWithOptions(
		omni.WithPath("/tmp/plugin_text.log"),
		omni.WithLevel(omni.LevelDebug),
		omni.WithText(),
	)
	if err != nil {
		log.Fatalf("Failed to create text logger: %v", err)
	}
	defer textLogger.Close()

	// Log with text format
	textLogger.Info("Text formatting example")
	textLogger.InfoWithFields("User action", map[string]interface{}{
		"user":   "john_doe",
		"action": "data_export",
	})

	fmt.Println("✓ Text formatter example completed")

	// Example 3: Using multiple destinations with different formats
	fmt.Println("\n=== Example 3: Multiple Destinations ===")
	
	multiLogger, err := omni.NewWithOptions(
		omni.WithPath("/tmp/plugin_multi.log"),
		omni.WithLevel(omni.LevelInfo),
		omni.WithJSON(),
	)
	if err != nil {
		log.Fatalf("Failed to create multi-destination logger: %v", err)
	}
	defer multiLogger.Close()

	// Add additional destinations
	if err := multiLogger.AddDestination("/tmp/plugin_backup.log"); err != nil {
		log.Printf("Failed to add backup destination: %v", err)
	}

	// Log to multiple destinations
	multiLogger.InfoWithFields("Multi-destination logging", map[string]interface{}{
		"destinations": []string{"/tmp/plugin_multi.log", "/tmp/plugin_backup.log"},
		"format":      "JSON",
		"level":       "INFO",
	})

	fmt.Println("✓ Multi-destination example completed")

	// Example 4: Advanced features demonstration
	fmt.Println("\n=== Example 4: Advanced Features ===")
	
	advancedLogger, err := omni.NewWithOptions(
		omni.WithPath("/tmp/plugin_advanced.log"),
		omni.WithLevel(omni.LevelDebug),
		omni.WithJSON(),
		omni.WithRotation(1024*1024, 3), // 1MB files, keep 3
		omni.WithGzipCompression(),
		omni.WithStackTrace(2048),
	)
	if err != nil {
		log.Fatalf("Failed to create advanced logger: %v", err)
	}
	defer advancedLogger.Close()

	// Test advanced features
	advancedLogger.InfoWithFields("Advanced logger features", map[string]interface{}{
		"rotation":    "1MB max size, 3 files",
		"compression": "gzip enabled",
		"stacktrace":  "enabled for errors",
	})

	// Generate some test data for rotation
	for i := 0; i < 100; i++ {
		advancedLogger.DebugWithFields("Test log entry", map[string]interface{}{
			"iteration": i,
			"data":      fmt.Sprintf("Test data chunk %d with some content to fill the log", i),
			"timestamp": time.Now().Unix(),
		})
	}

	fmt.Println("✓ Advanced features example completed")

	// Clean up test files
	cleanupFiles()
}

func cleanupFiles() {
	files := []string{
		"/tmp/plugin_json.log",
		"/tmp/plugin_text.log", 
		"/tmp/plugin_multi.log",
		"/tmp/plugin_backup.log",
		"/tmp/plugin_advanced.log",
	}

	for _, file := range files {
		if _, err := os.Stat(file); err == nil {
			_ = os.Remove(file) //nolint:gosec
		}
	}
}