package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/wayneeseguin/omni/pkg/backends"
	"github.com/wayneeseguin/omni/pkg/features"
	"github.com/wayneeseguin/omni/pkg/omni"
)

func main() {
	// Example: Disk Full Handling with Automatic Recovery
	// This example demonstrates how Omni handles disk full scenarios
	// by automatically rotating logs and cleaning up old files.

	fmt.Println("=== Omni Disk Full Handling Example ===")

	// Create rotation manager with aggressive cleanup
	rotMgr := features.NewRotationManager()
	rotMgr.SetMaxFiles(3)                      // Keep only 3 rotated files
	rotMgr.SetMaxAge(24 * time.Hour)          // Keep logs for 24 hours
	rotMgr.SetCleanupInterval(5 * time.Minute) // Check every 5 minutes

	// Set up error handler to monitor disk issues
	errorCount := 0
	rotMgr.SetErrorHandler(func(source, dest, msg string, err error) {
		errorCount++
		fmt.Printf("[%s] %s: %s (error: %v)\n", source, dest, msg, err)
	})

	// Set up metrics handler to track rotations
	rotationCount := 0
	rotMgr.SetMetricsHandler(func(metric string) {
		if metric == "rotation_completed" {
			rotationCount++
			fmt.Printf("✓ Log rotation completed (total: %d)\n", rotationCount)
		}
	})

	// Create file backend with disk full handling
	logPath := "example-diskfull.log"
	backend, err := backends.NewFileBackendWithRotation(logPath, rotMgr)
	if err != nil {
		log.Fatalf("Failed to create backend: %v", err)
	}

	// Configure retry behavior
	backend.SetMaxRetries(3) // Retry up to 3 times on disk full

	// Set custom error handler for the backend
	diskFullEvents := 0
	backend.SetErrorHandler(func(source, dest, msg string, err error) {
		fmt.Printf("[Backend] %s -> %s: %s\n", source, dest, msg)
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
		}
		
		// Track disk full events
		if source == "write" && err != nil {
			diskFullEvents++
		}
	})

	// Create logger with our backend
	logger, err := omni.NewWithBackend(backend)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	fmt.Println("\nSimulating high-volume logging...")
	fmt.Println("(In production, disk full would trigger automatic rotation)\n")

	// Simulate heavy logging that might fill disk
	startTime := time.Now()
	messageCount := 0
	
	// Log messages with increasing data
	for i := 0; i < 1000; i++ {
		// Create a message with variable size
		data := make([]byte, 1024*(i%10+1)) // 1KB to 10KB messages
		for j := range data {
			data[j] = byte('A' + (j % 26))
		}
		
		message := fmt.Sprintf("Log entry %d: %s", i, string(data[:100]))
		
		// Log with structured data
		logger.InfoWithFields(message, map[string]interface{}{
			"iteration":    i,
			"size_bytes":   len(data),
			"timestamp":    time.Now().Unix(),
			"disk_full_events": diskFullEvents,
		})
		
		messageCount++
		
		// Simulate some processing time
		if i%100 == 0 {
			fmt.Printf("Progress: %d messages logged, %d rotations, %d disk full events\n", 
				messageCount, rotationCount, diskFullEvents)
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Force a sync to ensure all data is written
	logger.Sync()

	// Display summary
	duration := time.Since(startTime)
	fmt.Println("\n=== Summary ===")
	fmt.Printf("Messages logged: %d\n", messageCount)
	fmt.Printf("Log rotations: %d\n", rotationCount)
	fmt.Printf("Disk full events: %d\n", diskFullEvents)
	fmt.Printf("Errors handled: %d\n", errorCount)
	fmt.Printf("Duration: %v\n", duration)
	fmt.Printf("Rate: %.2f messages/second\n", float64(messageCount)/duration.Seconds())

	// Show rotated files
	rotatedFiles, err := rotMgr.GetRotatedFiles(logPath)
	if err == nil && len(rotatedFiles) > 0 {
		fmt.Println("\nRotated log files:")
		for _, file := range rotatedFiles {
			fmt.Printf("  - %s (%.2f KB, rotated at %s)\n", 
				file.Name, 
				float64(file.Size)/1024,
				file.RotationTime.Format("15:04:05"))
		}
	}

	// Demonstrate manual rotation
	fmt.Println("\nPerforming manual rotation...")
	if err := backend.Rotate(); err != nil {
		fmt.Printf("Manual rotation failed: %v\n", err)
	} else {
		fmt.Println("✓ Manual rotation completed")
	}

	// Clean up example files
	fmt.Println("\nCleaning up example files...")
	os.Remove(logPath)
	for _, file := range rotatedFiles {
		os.Remove(file.Path)
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