package main

import (
	"fmt"
	"time"

	"github.com/wayneeseguin/flocklogger"
)

func main() {
	// Create logger with small max size for demo purposes
	logger, err := flocklogger.NewFlockLogger("./logs/compressed.log")
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		return
	}

	// Configure logger
	logger.SetMaxSize(1024)                            // 1 KB for quick rotation
	logger.SetMaxFiles(5)                              // Keep 5 log files
	logger.SetCompression(flocklogger.CompressionGzip) // Enable gzip compression
	logger.SetCompressMinAge(1)                        // Compress files after 1 rotation
	logger.SetCompressWorkers(2)                       // Use 2 worker goroutines for compression

	// Generate some logs to trigger rotation
	for i := 0; i < 100; i++ {
		logger.Infof("Log message %d with some padding data to fill up space quickly: %s",
			i, "Lorem ipsum dolor sit amet, consectetur adipiscing elit.")
	}

	// Flush logs
	logger.Flush()

	// Log information about compressed files
	logger.Info("First rotation complete, some files should be compressing now")

	// Generate more logs to trigger multiple rotations
	for i := 0; i < 400; i++ {
		logger.Infof("Second batch log message %d with data: %s",
			i, "Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.")
	}

	// Give compression workers time to complete
	logger.Info("Waiting for compression workers to complete...")
	time.Sleep(500 * time.Millisecond)

	// Close the logger
	logger.Close()

	fmt.Println("Check logs directory for compressed log files (*.gz)")
	fmt.Println("Original logs were rotated and compressed to save space")
}
