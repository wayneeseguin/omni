package main

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/wayneeseguin/flexlog"
)

func main() {
	// Create a performance-optimized logger with batching enabled
	logger, err := flexlog.NewWithOptions(
		flexlog.WithPath("performance.log"),
		flexlog.WithLevel(flexlog.LevelInfo),
		flexlog.WithJSON(),                                                         // JSON format for better performance parsing
		flexlog.WithDefaultBatching(),                                              // Enable batching with default settings (64KB, 100 entries, 100ms)
		flexlog.WithChannelSize(10000),                                             // Large channel buffer for high throughput
		flexlog.WithRotation(100*1024*1024, 5),                                    // 100MB files, keep 5
		flexlog.WithGzipCompression(),                                              // Compress old files
	)
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Close()

	log.Printf("Performance testing with different log levels...")
	log.Printf("Current level: INFO (TRACE and DEBUG will be filtered)")

	// Test 1: Single-threaded logging performance
	log.Printf("Test 1: Single-threaded logging")
	start := time.Now()

	for i := 0; i < 10000; i++ {
		// These TRACE calls will be filtered out, showing performance benefit
		logger.Trace("This trace will be filtered") // Won't be logged
		logger.Debug("This debug will be filtered") // Won't be logged

		// Only INFO and above will be logged
		if i%1000 == 0 {
			logger.InfoWithFields("Performance test progress", map[string]interface{}{
				"iteration": i,
				"timestamp": time.Now().Unix(),
			})
		}
	}

	singleThreaded := time.Since(start)
	log.Printf("Single-threaded: %v for 10,000 iterations", singleThreaded)

	// Test 2: Multi-threaded logging performance
	log.Printf("Test 2: Multi-threaded logging (4 goroutines)")
	start = time.Now()

	var wg sync.WaitGroup
	numGoroutines := 4
	iterationsPerGoroutine := 2500

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for i := 0; i < iterationsPerGoroutine; i++ {
				// These will be filtered for performance
				logger.Trace("Goroutine trace", "goroutine", goroutineID) // Won't be logged
				logger.Debug("Goroutine debug", "goroutine", goroutineID) // Won't be logged

				// Only log occasionally to reduce overhead
				if i%500 == 0 {
					logger.InfoWithFields("Goroutine progress", map[string]interface{}{
						"goroutine": goroutineID,
						"iteration": i,
						"total":     iterationsPerGoroutine,
					})
				}
			}
		}(g)
	}

	wg.Wait()
	multiThreaded := time.Since(start)
	log.Printf("Multi-threaded: %v for 10,000 iterations (4 goroutines)", multiThreaded)

	// Test 3: Level filtering performance benefit
	log.Printf("Test 3: Demonstrating level filtering performance")

	// Test with TRACE level enabled (more expensive)
	logger.SetLevel(flexlog.LevelTrace)
	start = time.Now()

	for i := 0; i < 5000; i++ {
		logger.Trace("Expensive trace message with formatting", "value", i*2)
		logger.Debug("Debug message", "iteration", i)
		if i%1000 == 0 {
			logger.Info("Progress with TRACE enabled", "iteration", i)
		}
	}

	withTrace := time.Since(start)

	// Test with INFO level (filtered TRACE/DEBUG)
	logger.SetLevel(flexlog.LevelInfo)
	start = time.Now()

	for i := 0; i < 5000; i++ {
		logger.Trace("Expensive trace message with formatting", "value", i*2) // Filtered
		logger.Debug("Debug message", "iteration", i)                         // Filtered
		if i%1000 == 0 {
			logger.Info("Progress with TRACE filtered", "iteration", i)
		}
	}

	withoutTrace := time.Since(start)

	log.Printf("With TRACE enabled: %v", withTrace)
	log.Printf("With TRACE filtered: %v", withoutTrace)
	log.Printf("Performance improvement: %.2fx faster", float64(withTrace)/float64(withoutTrace))

	// Test 4: Memory allocation patterns
	log.Printf("Test 4: Memory usage patterns")
	var m1, m2 runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&m1)

	logger.SetLevel(flexlog.LevelTrace)
	for i := 0; i < 1000; i++ {
		logger.TraceWithFields("Memory test", map[string]interface{}{
			"iteration": i,
			"data":      fmt.Sprintf("test-data-%d", i),
			"timestamp": time.Now(),
		})
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	allocatedKB := (m2.TotalAlloc - m1.TotalAlloc) / 1024
	log.Printf("Memory allocated for 1000 TRACE messages: %d KB", allocatedKB)

	// Test 5: Batching vs Non-batching performance
	log.Printf("Test 5: Batching performance comparison")
	
	// Test without batching first (create new logger)
	nonBatchLogger, err := flexlog.NewWithOptions(
		flexlog.WithPath("performance-nobatch.log"),
		flexlog.WithLevel(flexlog.LevelInfo),
		flexlog.WithJSON(),
		flexlog.WithChannelSize(10000),
	)
	if err != nil {
		log.Printf("Error creating non-batch logger: %v", err)
	} else {
		start = time.Now()
		for i := 0; i < 5000; i++ {
			nonBatchLogger.InfoWithFields("Non-batch test", map[string]interface{}{
				"iteration": i,
				"data":      fmt.Sprintf("test-data-%d", i),
				"timestamp": time.Now().Unix(),
			})
		}
		nonBatchTime := time.Since(start)
		nonBatchLogger.Close()
		
		// Test with batching (use existing logger)
		start = time.Now()
		for i := 0; i < 5000; i++ {
			logger.InfoWithFields("Batch test", map[string]interface{}{
				"iteration": i,
				"data":      fmt.Sprintf("test-data-%d", i),
				"timestamp": time.Now().Unix(),
			})
		}
		batchTime := time.Since(start)
		
		log.Printf("Non-batching time: %v", nonBatchTime)
		log.Printf("Batching time: %v", batchTime)
		if nonBatchTime > batchTime {
			log.Printf("Batching improvement: %.2fx faster", float64(nonBatchTime)/float64(batchTime))
		} else {
			log.Printf("Batching overhead: %.2fx slower", float64(batchTime)/float64(nonBatchTime))
		}
	}

	// Final summary
	log.Printf("\nPerformance Summary:")
	log.Printf("  Single-threaded throughput: %.0f ops/sec", 10000.0/singleThreaded.Seconds())
	log.Printf("  Multi-threaded throughput: %.0f ops/sec", 10000.0/multiThreaded.Seconds())
	log.Printf("  Level filtering saves: %.1f%% time", (1.0-float64(withoutTrace)/float64(withTrace))*100)
	log.Printf("  Memory per TRACE message: %.1f bytes", float64(allocatedKB*1024)/1000.0)

	logger.InfoWithFields("Performance test completed", map[string]interface{}{
		"single_threaded_ms": singleThreaded.Milliseconds(),
		"multi_threaded_ms":  multiThreaded.Milliseconds(),
		"memory_kb":          allocatedKB,
	})

	fmt.Println("Check performance.log for detailed logs")
}
