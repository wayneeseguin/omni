package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/wayneeseguin/flexlog"
)

func main() {
	// Create a temporary directory for the example
	tempDir, err := os.MkdirTemp("", "flexlog_batch_example")
	if err != nil {
		log.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fmt.Println("FlexLog Batch Processing Example")
	fmt.Println("================================")

	// Example 1: High-throughput logging with batching
	fmt.Println("\n1. High-throughput logging with batching enabled:")
	
	logFile := filepath.Join(tempDir, "batched.log")
	config := flexlog.DefaultConfig()
	config.Path = logFile
	config.EnableBatching = true
	config.BatchMaxSize = 4096  // 4KB batch size
	config.BatchMaxCount = 50   // or 50 messages
	config.BatchFlushInterval = 100 * time.Millisecond // flush every 100ms
	
	logger, err := flexlog.NewWithConfig(config)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}

	// Check if batching is enabled
	if enabled, _ := logger.IsBatchingEnabled(0); enabled {
		fmt.Printf("✓ Batching is enabled (max size: %d bytes, max count: %d, interval: %v)\n", 
			config.BatchMaxSize, config.BatchMaxCount, config.BatchFlushInterval)
	}

	// Write many messages quickly
	start := time.Now()
	for i := 0; i < 100; i++ {
		logger.Info(fmt.Sprintf("High-throughput message %d with batching", i))
	}
	batchedDuration := time.Since(start)
	
	// Wait for final flush
	time.Sleep(150 * time.Millisecond)
	logger.Close()

	fmt.Printf("✓ Logged 100 messages with batching in %v\n", batchedDuration)

	// Example 2: Same load without batching (for comparison)
	fmt.Println("\n2. Same workload without batching:")
	
	logFile2 := filepath.Join(tempDir, "unbatched.log")
	config2 := flexlog.DefaultConfig()
	config2.Path = logFile2
	config2.EnableBatching = false // Disable batching
	
	logger2, err := flexlog.NewWithConfig(config2)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}

	// Check if batching is disabled
	if enabled, _ := logger2.IsBatchingEnabled(0); !enabled {
		fmt.Printf("✓ Batching is disabled (immediate writes)\n")
	}

	// Write the same messages without batching
	start = time.Now()
	for i := 0; i < 100; i++ {
		logger2.Info(fmt.Sprintf("High-throughput message %d without batching", i))
	}
	unbatchedDuration := time.Since(start)
	
	logger2.Close()

	fmt.Printf("✓ Logged 100 messages without batching in %v\n", unbatchedDuration)

	// Show performance improvement
	if batchedDuration < unbatchedDuration {
		improvement := float64(unbatchedDuration-batchedDuration) / float64(unbatchedDuration) * 100
		fmt.Printf("\n🚀 Batching provided %.1f%% performance improvement!\n", improvement)
	}

	// Example 3: Dynamic batching control
	fmt.Println("\n3. Dynamic batching control:")
	
	logFile3 := filepath.Join(tempDir, "dynamic.log")
	logger3, err := flexlog.New(logFile3)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger3.Close()

	// Start without batching
	fmt.Println("✓ Starting without batching")
	logger3.Info("Message without batching")

	// Enable batching dynamically
	if err := logger3.EnableBatching(0, true); err != nil {
		log.Fatalf("Failed to enable batching: %v", err)
	}
	fmt.Println("✓ Enabled batching dynamically")
	
	logger3.Info("Message with batching enabled")
	logger3.Info("Another batched message")
	
	// Configure batch settings
	if err := logger3.SetBatchingConfig(0, 2048, 25, 50*time.Millisecond); err != nil {
		log.Fatalf("Failed to configure batching: %v", err)
	}
	fmt.Println("✓ Updated batch configuration (2KB, 25 messages, 50ms)")
	
	logger3.Info("Message with updated batch config")
	
	// Disable batching
	if err := logger3.EnableBatching(0, false); err != nil {
		log.Fatalf("Failed to disable batching: %v", err)
	}
	fmt.Println("✓ Disabled batching dynamically")
	
	logger3.Info("Back to immediate writes")

	fmt.Println("\n✅ Batch processing example completed!")
	fmt.Printf("📁 Log files created in: %s\n", tempDir)
	fmt.Println("You can examine the log files to see the batching behavior.")
}