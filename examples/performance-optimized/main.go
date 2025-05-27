package main

import (
	"context"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/wayneeseguin/flexlog"
)

func main() {
	// Performance-optimized configuration
	config := flexlog.Config{
		ChannelSize:   10000, // Large buffer for high throughput
		DefaultLevel:  flexlog.INFO,
		EnableMetrics: true,
		
		// Aggressive sampling for debug logs
		Sampling: flexlog.SamplingConfig{
			Enabled: true,
			Rate:    0.01, // Only 1% of debug logs
			Levels:  []flexlog.LogLevel{flexlog.DEBUG},
		},
		
		// Enable object pooling for reduced GC pressure
		EnablePooling: true,
	}

	logger, err := flexlog.NewFlexLogWithConfig(config)
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Close()

	// Add high-performance destination
	err = logger.AddDestination("perf", flexlog.DestinationConfig{
		Backend:      flexlog.BackendFile,
		FilePath:     "logs/performance.log",
		Format:       flexlog.FormatJSON,
		MinLevel:     flexlog.INFO,
		BufferSize:   8192, // Large write buffer
		AsyncWrites:  true, // Non-blocking writes
	})
	if err != nil {
		log.Fatal(err)
	}

	// Benchmark: Measure logging throughput
	log.Println("Starting performance benchmark...")
	startTime := time.Now()
	messageCount := 1000000
	
	// Use goroutines to simulate concurrent logging
	numGoroutines := runtime.NumCPU()
	messagesPerGoroutine := messageCount / numGoroutines
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()
			
			for i := 0; i < messagesPerGoroutine; i++ {
				// Use pre-allocated fields to reduce allocations
				logger.Info("High throughput message",
					"goroutine", goroutineID,
					"message", i,
					"timestamp", time.Now().UnixNano(),
				)
				
				// Occasional debug message (will be sampled)
				if i%100 == 0 {
					logger.Debug("Debug checkpoint",
						"goroutine", goroutineID,
						"progress", i,
					)
				}
			}
		}(g)
	}
	
	wg.Wait()
	duration := time.Since(startTime)
	
	// Calculate and display performance metrics
	throughput := float64(messageCount) / duration.Seconds()
	log.Printf("Performance Results:")
	log.Printf("  Total messages: %d", messageCount)
	log.Printf("  Duration: %v", duration)
	log.Printf("  Throughput: %.2f messages/second", throughput)
	log.Printf("  Latency: %.2f microseconds/message", duration.Seconds()*1000000/float64(messageCount))
	
	// Wait for async writes to complete
	time.Sleep(time.Second)
	
	// Display logger metrics
	metrics := logger.GetMetrics()
	log.Printf("\nLogger Metrics:")
	log.Printf("  Total logged: %d", metrics.TotalMessages)
	log.Printf("  Dropped: %d", metrics.DroppedMessages)
	log.Printf("  Buffer usage: %.2f%%", metrics.BufferUsage)
	log.Printf("  Pool hit rate: %.2f%%", metrics.PoolHitRate)
	
	// Memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	log.Printf("\nMemory Stats:")
	log.Printf("  Alloc: %d MB", m.Alloc/1024/1024)
	log.Printf("  TotalAlloc: %d MB", m.TotalAlloc/1024/1024)
	log.Printf("  NumGC: %d", m.NumGC)
	
	// Demonstrate zero-allocation logging path
	log.Println("\nTesting zero-allocation logging...")
	
	// Pre-allocate message
	msg := &flexlog.LogMessage{
		Level:     flexlog.INFO,
		Message:   "Zero allocation message",
		Timestamp: time.Now(),
		Fields:    make(map[string]interface{}, 4),
	}
	
	// Reuse message object
	for i := 0; i < 10000; i++ {
		msg.Timestamp = time.Now()
		msg.Fields["counter"] = i
		msg.Fields["thread"] = 0
		
		logger.LogMessage(msg)
		
		// Reset fields for reuse
		for k := range msg.Fields {
			delete(msg.Fields, k)
		}
	}
	
	log.Println("Performance benchmark completed!")
}

// Example of using logger in a high-performance server
func highPerformanceServer(logger *flexlog.FlexLog) {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	
	// Pre-allocated log entries for common scenarios
	successLog := &flexlog.LogMessage{
		Level:   flexlog.INFO,
		Message: "Request processed",
		Fields:  make(map[string]interface{}, 8),
	}
	
	errorLog := &flexlog.LogMessage{
		Level:   flexlog.ERROR,
		Message: "Request failed",
		Fields:  make(map[string]interface{}, 8),
	}
	
	// Simulate request handling
	for i := 0; i < 100000; i++ {
		select {
		case <-ctx.Done():
			return
		default:
			// Process request
			startTime := time.Now()
			
			// Simulate work
			time.Sleep(time.Microsecond)
			
			// Log result (reusing pre-allocated messages)
			if i%1000 == 0 {
				errorLog.Timestamp = time.Now()
				errorLog.Fields["request_id"] = i
				errorLog.Fields["error"] = "simulated error"
				errorLog.Fields["duration_us"] = time.Since(startTime).Microseconds()
				logger.LogMessage(errorLog)
			} else {
				successLog.Timestamp = time.Now()
				successLog.Fields["request_id"] = i
				successLog.Fields["duration_us"] = time.Since(startTime).Microseconds()
				logger.LogMessage(successLog)
			}
		}
	}
}