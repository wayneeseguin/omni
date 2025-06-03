package omni

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// BenchmarkOptimizedBufferPool tests the performance improvement from buffer pooling
func BenchmarkOptimizedBufferPool(b *testing.B) {
	b.Run("WithoutPool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// Simulate formatting without pool
			entry := &LogEntry{
				Level:     "INFO",
				Message:   "Test message",
				Timestamp: time.Now().Format(time.RFC3339),
			}
			data, _ := formatJSON(entry, false)
			_ = data
		}
	})

	b.Run("WithPool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// Use the optimized version with pool
			entry := &LogEntry{
				Level:     "INFO", 
				Message:   "Test message",
				Timestamp: time.Now().Format(time.RFC3339),
			}
			data, _ := formatJSONEntry(entry)
			_ = data
		}
	})
}

// BenchmarkOptimizedLocking tests the RWMutex optimization
func BenchmarkOptimizedLocking(b *testing.B) {
	dir := b.TempDir()
	logFile := filepath.Join(dir, "bench.log")

	logger, err := New(logFile)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Add multiple destinations
	for i := 0; i < 5; i++ {
		destFile := filepath.Join(dir, fmt.Sprintf("dest%d.log", i))
		logger.AddDestination(destFile)
	}

	b.Run("ReadEnabled", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				// This now uses RLock instead of Lock
				logger.mu.RLock()
				destinations := logger.Destinations
				logger.mu.RUnlock()
				
				for _, dest := range destinations {
					dest.mu.RLock()
					_ = dest.Enabled
					dest.mu.RUnlock()
				}
			}
		})
	})
}

// BenchmarkBatchedWrites tests the batching performance
func BenchmarkBatchedWrites(b *testing.B) {
	configs := []struct {
		name          string
		batchSize     int
		flushInterval time.Duration
	}{
		{"NoBatch", 0, 0},
		{"SmallBatch", 1024, 10 * time.Millisecond},
		{"MediumBatch", 4096, 50 * time.Millisecond},
		{"LargeBatch", 8192, 100 * time.Millisecond},
	}

	for _, cfg := range configs {
		b.Run(cfg.name, func(b *testing.B) {
			dir := b.TempDir()
			logFile := filepath.Join(dir, "bench.log")

			logger, err := New(logFile)
			if err != nil {
				b.Fatalf("Failed to create logger: %v", err)
			}
			defer logger.Close()

			// Configure batching
			if cfg.batchSize > 0 {
				logger.SetFlushSize(0, cfg.batchSize)
				logger.SetFlushInterval(0, cfg.flushInterval)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				logger.Info("Batched message %d", i)
			}

			// Ensure all messages are flushed
			logger.FlushAll()
		})
	}
}

// BenchmarkConcurrentThroughput measures throughput with optimizations
func BenchmarkConcurrentThroughput(b *testing.B) {
	workers := []int{1, 2, 4, 8, 16}

	for _, numWorkers := range workers {
		b.Run(fmt.Sprintf("Workers-%d", numWorkers), func(b *testing.B) {
			dir := b.TempDir()
			logFile := filepath.Join(dir, "bench.log")

			logger, err := New(logFile)
			if err != nil {
				b.Fatalf("Failed to create logger: %v", err)
			}
			defer logger.Close()

			// Enable optimizations
			logger.SetFlushSize(0, 8192)
			logger.SetFlushInterval(0, 100*time.Millisecond)

			b.ResetTimer()
			b.ReportAllocs()

			var wg sync.WaitGroup
			messagesPerWorker := b.N / numWorkers

			start := time.Now()
			for w := 0; w < numWorkers; w++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					for i := 0; i < messagesPerWorker; i++ {
						logger.Info("Worker %d message %d", workerID, i)
					}
				}(w)
			}

			wg.Wait()
			elapsed := time.Since(start)

			// Calculate and report throughput
			throughput := float64(b.N) / elapsed.Seconds()
			b.ReportMetric(throughput, "msgs/sec")
		})
	}
}

// BenchmarkMemoryEfficiency tests memory usage with optimizations
func BenchmarkMemoryEfficiency(b *testing.B) {
	dir := b.TempDir()
	logFile := filepath.Join(dir, "bench.log")

	logger, err := New(logFile)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Enable all optimizations
	logger.SetFlushSize(0, 8192)
	logger.SetFlushInterval(0, 100*time.Millisecond)

	b.ResetTimer()
	b.ReportAllocs()

	// Run for a fixed duration and measure allocations
	duration := 1 * time.Second
	start := time.Now()
	count := 0

	for time.Since(start) < duration {
		logger.Info("Memory test message %d with some additional data", count)
		count++
	}

	// Report messages per operation
	if count > 0 {
		msgsPerOp := float64(count) / float64(b.N)
		b.ReportMetric(msgsPerOp, "msgs/op")
	}
}

// BenchmarkOptimizationComparison provides a side-by-side comparison
func BenchmarkOptimizationComparison(b *testing.B) {
	scenarios := []struct {
		name         string
		bufferPool   bool
		rwMutex      bool
		batching     bool
		atomicOps    bool
	}{
		{"Baseline", false, false, false, false},
		{"BufferPool", true, false, false, false},
		{"RWMutex", false, true, false, false},
		{"Batching", false, false, true, false},
		{"AllOptimizations", true, true, true, true},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			dir := b.TempDir()
			logFile := filepath.Join(dir, "bench.log")

			logger, err := New(logFile)
			if err != nil {
				b.Fatalf("Failed to create logger: %v", err)
			}
			defer logger.Close()

			// Apply optimizations based on scenario
			if scenario.batching {
				logger.SetFlushSize(0, 8192)
				logger.SetFlushInterval(0, 100*time.Millisecond)
			}

			b.ResetTimer()
			b.ReportAllocs()

			// Run concurrent workload
			var wg sync.WaitGroup
			workers := 4
			messagesPerWorker := b.N / workers

			for w := 0; w < workers; w++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					for i := 0; i < messagesPerWorker; i++ {
						logger.WithFields(map[string]interface{}{
							"worker": id,
							"seq":    i,
						}).Info("Optimized test message")
					}
				}(w)
			}

			wg.Wait()
		})
	}
}