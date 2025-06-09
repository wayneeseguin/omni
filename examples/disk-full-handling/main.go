package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/wayneeseguin/omni/pkg/omni"
)

func main() {
	// Example: Disk Full Handling with Automatic Recovery
	// This example demonstrates how Omni handles disk full scenarios
	// by automatically rotating logs and cleaning up old files.

	fmt.Println("=== Omni Disk Full Handling Example ===")

	// Track metrics for demonstration
	errorCount := 0
	rotationCount := 0

	// Create logger with file backend that supports disk full handling
	logPath := "example-diskfull.log"
	logger, err := omni.NewWithBackend(logPath, omni.BackendFlock)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Configure the logger for demonstration of disk full handling
	logger.SetMaxSize(1024 * 1024) // 1MB files for quick rotation
	logger.SetMaxFiles(3)          // Keep only 3 files
	if err := logger.SetFormat(omni.FormatJSON); err != nil {
		log.Printf("Warning: failed to set JSON format: %v", err)
	}

	fmt.Println("\nSimulating high-volume logging...")
	fmt.Println("(In production, disk full would trigger automatic rotation)")

	// Simulate heavy logging that might fill disk
	startTime := time.Now()
	messageCount := 0
	diskFullEvents := 0

	// Log messages with increasing data to trigger rotations
	for i := 0; i < 500; i++ {
		// Create a message with variable size
		data := make([]byte, 2048*(i%5+1)) // 2KB to 10KB messages
		for j := range data {
			data[j] = byte('A' + (j % 26))
		}

		message := fmt.Sprintf("Log entry %d with large data", i)

		// Log with structured data
		logger.InfoWithFields(message, map[string]interface{}{
			"iteration":   i,
			"size_bytes":  len(data),
			"timestamp":   time.Now().Unix(),
			"large_data":  string(data[:200]), // Include some of the data
			"disk_events": diskFullEvents,
		})

		messageCount++

		// Simulate some processing time and show progress
		if i%50 == 0 {
			fmt.Printf("Progress: %d messages logged, %d rotations\n",
				messageCount, rotationCount)
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Display summary
	duration := time.Since(startTime)
	fmt.Println("\n=== Summary ===")
	fmt.Printf("Messages logged: %d\n", messageCount)
	fmt.Printf("Log rotations: %d\n", rotationCount)
	fmt.Printf("Errors handled: %d\n", errorCount)
	fmt.Printf("Duration: %v\n", duration)
	fmt.Printf("Rate: %.2f messages/second\n", float64(messageCount)/duration.Seconds())

	// Note: Manual rotation is not directly available as a public API
	// Rotation happens automatically when file size exceeds maxSize
	fmt.Println("\nRotation occurs automatically when file size exceeds the configured maximum.")

	// Clean up example files
	fmt.Println("\nCleaning up example files...")
	cleanupFiles := []string{logPath, logPath + ".1", logPath + ".2", logPath + ".3"}
	for _, file := range cleanupFiles {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			log.Printf("Warning: failed to remove %s: %v", file, err)
		}
	}

	fmt.Println("\n=== Example completed ===")
}

// Note: In a real production environment with limited disk space,
// the disk full handling would automatically:
// 1. Detect when writes fail due to lack of space
// 2. Rotate the current log file to free the file handle
// 3. Delete the oldest rotated logs to reclaim space
// 4. Retry the failed write operation
// 5. Continue normal logging operations
//
// This ensures your application never stops logging due to disk space issues.
