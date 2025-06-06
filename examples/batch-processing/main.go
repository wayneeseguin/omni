package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wayneeseguin/omni/pkg/omni"
)

func main() {
	// Create a temporary directory for the example
	tempDir, err := os.MkdirTemp("", "omni_batch_example")
	if err != nil {
		log.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fmt.Println("Omni Batch Processing Example")
	fmt.Println("================================")

	// Example 1: High-throughput logging with optimized settings
	fmt.Println("\n1. High-throughput logging with optimization:")
	
	logFile := filepath.Join(tempDir, "optimized.log")
	logger, err := omni.NewWithOptions(
		omni.WithPath(logFile),
		omni.WithLevel(omni.LevelInfo),
		omni.WithChannelSize(1000), // Larger channel buffer for batching effect
	)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}

	fmt.Printf("✓ Logger created with optimized channel size\n")

	// Write many messages quickly to demonstrate batching behavior
	start := time.Now()
	for i := 0; i < 1000; i++ {
		logger.InfoWithFields("High-throughput batch message", map[string]interface{}{
			"batch_id":   i / 100,
			"message_id": i,
			"timestamp":  time.Now().Unix(),
		})
	}
	optimizedDuration := time.Since(start)
	
	// Flush to ensure all messages are written
	logger.FlushAll()
	logger.Close()

	fmt.Printf("✓ Logged 1000 messages with optimization in %v\n", optimizedDuration)

	// Example 2: Processing batches of data with structured logging
	fmt.Println("\n2. Processing data in batches with structured logging:")
	
	logFile2 := filepath.Join(tempDir, "data_batches.log")
	logger2, err := omni.NewWithOptions(
		omni.WithPath(logFile2),
		omni.WithLevel(omni.LevelDebug),
		omni.WithJSON(), // Use JSON format for structured data
	)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}

	// Simulate processing data in batches
	totalRecords := 500
	batchSize := 50
	
	for batchStart := 0; batchStart < totalRecords; batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > totalRecords {
			batchEnd = totalRecords
		}
		
		batchID := (batchStart / batchSize) + 1
		
		logger2.InfoWithFields("Batch processing started", map[string]interface{}{
			"batch_id":    batchID,
			"batch_start": batchStart,
			"batch_end":   batchEnd,
			"batch_size":  batchEnd - batchStart,
		})
		
		// Process each record in the batch
		for i := batchStart; i < batchEnd; i++ {
			logger2.DebugWithFields("Processing record", map[string]interface{}{
				"batch_id":  batchID,
				"record_id": i,
				"status":    "processing",
			})
			
			// Simulate processing time
			if i%100 == 0 {
				time.Sleep(1 * time.Millisecond)
			}
		}
		
		logger2.InfoWithFields("Batch processing completed", map[string]interface{}{
			"batch_id":         batchID,
			"records_processed": batchEnd - batchStart,
			"total_progress":   float64(batchEnd) / float64(totalRecords) * 100,
		})
	}
	
	logger2.FlushAll()
	logger2.Close()

	fmt.Printf("✓ Processed %d records in %d batches\n", totalRecords, (totalRecords+batchSize-1)/batchSize)

	// Example 3: Concurrent batch processing
	fmt.Println("\n3. Concurrent batch processing:")
	
	logFile3 := filepath.Join(tempDir, "concurrent_batches.log")
	logger3, err := omni.NewWithOptions(
		omni.WithPath(logFile3),
		omni.WithLevel(omni.LevelInfo),
		omni.WithChannelSize(2000), // Large buffer for concurrent access
	)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger3.Close()

	// Start multiple goroutines to process batches concurrently
	var wg sync.WaitGroup
	numWorkers := 4
	itemsPerWorker := 100
	
	start = time.Now()
	
	for workerID := 0; workerID < numWorkers; workerID++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			logger3.InfoWithFields("Worker started", map[string]interface{}{
				"worker_id": id,
				"items":     itemsPerWorker,
			})
			
			for i := 0; i < itemsPerWorker; i++ {
				logger3.InfoWithFields("Processing item", map[string]interface{}{
					"worker_id": id,
					"item_id":   i,
					"thread":    "concurrent",
				})
				
				if i%25 == 0 {
					logger3.InfoWithFields("Worker progress", map[string]interface{}{
						"worker_id": id,
						"progress":  float64(i) / float64(itemsPerWorker) * 100,
					})
				}
			}
			
			logger3.InfoWithFields("Worker completed", map[string]interface{}{
				"worker_id": id,
				"items":     itemsPerWorker,
			})
		}(workerID)
	}
	
	wg.Wait()
	concurrentDuration := time.Since(start)
	
	logger3.FlushAll()

	fmt.Printf("✓ Processed %d items concurrently by %d workers in %v\n", 
		numWorkers*itemsPerWorker, numWorkers, concurrentDuration)

	// Performance summary
	fmt.Println("\n📊 Performance Summary:")
	fmt.Printf("  Optimized throughput: %.0f msgs/sec\n", 1000.0/optimizedDuration.Seconds())
	fmt.Printf("  Concurrent throughput: %.0f msgs/sec\n", float64(numWorkers*itemsPerWorker)/concurrentDuration.Seconds())
	
	fmt.Println("\n✅ Batch processing example completed!")
	fmt.Printf("📁 Log files created in: %s\n", tempDir)
	fmt.Println("   - optimized.log: High-throughput logging")
	fmt.Println("   - data_batches.log: Structured batch processing logs")
	fmt.Println("   - concurrent_batches.log: Concurrent processing logs")
}