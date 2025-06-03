package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/wayneeseguin/omni"
)

func TestMain(m *testing.M) {
	// Setup: clean up any existing test files
	os.RemoveAll("test_batch")
	
	code := m.Run()
	
	// Cleanup: remove test files
	os.RemoveAll("test_batch")
	os.Exit(code)
}

func TestBatchProcessingExample(t *testing.T) {
	// Create test directory
	testLogDir := "test_batch"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	// Test high-throughput logging with optimization
	logFile := filepath.Join(testLogDir, "test_optimized.log")
	logger, err := omni.NewWithOptions(
		omni.WithPath(logFile),
		omni.WithLevel(omni.LevelInfo),
		omni.WithChannelSize(1000),
	)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Write messages quickly
	start := time.Now()
	for i := 0; i < 100; i++ { // Reduced for testing
		logger.InfoWithFields("Test batch message", map[string]interface{}{
			"batch_id":   i / 10,
			"message_id": i,
		})
	}
	duration := time.Since(start)

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	logger.Close()

	t.Logf("Logged 100 messages in %v", duration)

	// Verify log file was created and has content
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file was not created: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

func TestHighThroughputLogging(t *testing.T) {
	testLogDir := "test_batch"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	logger, err := omni.NewWithOptions(
		omni.WithPath(filepath.Join(testLogDir, "throughput.log")),
		omni.WithLevel(omni.LevelInfo),
		omni.WithChannelSize(500),
	)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Measure throughput
	iterations := 500
	start := time.Now()

	for i := 0; i < iterations; i++ {
		logger.InfoWithFields("Throughput test message", map[string]interface{}{
			"iteration": i,
			"timestamp": time.Now().Unix(),
		})
	}

	duration := time.Since(start)
	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	logger.Close()

	throughput := float64(iterations) / duration.Seconds()
	t.Logf("Throughput: %.0f msgs/sec", throughput)

	// Should handle reasonable throughput
	if throughput < 1000 {
		t.Logf("Warning: Low throughput detected: %.0f msgs/sec", throughput)
	}

	// Verify log file
	logFile := filepath.Join(testLogDir, "throughput.log")
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

func TestBatchDataProcessing(t *testing.T) {
	testLogDir := "test_batch"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	logger, err := omni.NewWithOptions(
		omni.WithPath(filepath.Join(testLogDir, "data_batches.log")),
		omni.WithLevel(omni.LevelDebug),
		omni.WithJSON(),
	)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Simulate processing data in batches
	totalRecords := 100
	batchSize := 20

	for batchStart := 0; batchStart < totalRecords; batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > totalRecords {
			batchEnd = totalRecords
		}

		batchID := (batchStart / batchSize) + 1

		logger.InfoWithFields("Batch processing started", map[string]interface{}{
			"batch_id":    batchID,
			"batch_start": batchStart,
			"batch_end":   batchEnd,
			"batch_size":  batchEnd - batchStart,
		})

		// Process records in batch
		for i := batchStart; i < batchEnd; i++ {
			logger.DebugWithFields("Processing record", map[string]interface{}{
				"batch_id":  batchID,
				"record_id": i,
				"status":    "processing",
			})
		}

		logger.InfoWithFields("Batch processing completed", map[string]interface{}{
			"batch_id":         batchID,
			"records_processed": batchEnd - batchStart,
			"total_progress":   float64(batchEnd) / float64(totalRecords) * 100,
		})
	}

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	logger.Close()

	expectedBatches := (totalRecords + batchSize - 1) / batchSize
	t.Logf("Processed %d records in %d batches", totalRecords, expectedBatches)

	// Verify log file
	logFile := filepath.Join(testLogDir, "data_batches.log")
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

func TestConcurrentBatchProcessing(t *testing.T) {
	testLogDir := "test_batch"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	logger, err := omni.NewWithOptions(
		omni.WithPath(filepath.Join(testLogDir, "concurrent.log")),
		omni.WithLevel(omni.LevelInfo),
		omni.WithChannelSize(1000),
	)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Test concurrent batch processing
	numWorkers := 3
	itemsPerWorker := 50
	var wg sync.WaitGroup

	start := time.Now()

	for workerID := 0; workerID < numWorkers; workerID++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			logger.InfoWithFields("Worker started", map[string]interface{}{
				"worker_id": id,
				"items":     itemsPerWorker,
			})

			for i := 0; i < itemsPerWorker; i++ {
				logger.InfoWithFields("Processing item", map[string]interface{}{
					"worker_id": id,
					"item_id":   i,
					"thread":    "concurrent",
				})

				if i%10 == 0 {
					logger.InfoWithFields("Worker progress", map[string]interface{}{
						"worker_id": id,
						"progress":  float64(i) / float64(itemsPerWorker) * 100,
					})
				}
			}

			logger.InfoWithFields("Worker completed", map[string]interface{}{
				"worker_id": id,
				"items":     itemsPerWorker,
			})
		}(workerID)
	}

	wg.Wait()
	duration := time.Since(start)

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	logger.Close()

	totalItems := numWorkers * itemsPerWorker
	throughput := float64(totalItems) / duration.Seconds()
	t.Logf("Processed %d items concurrently by %d workers in %v (%.0f items/sec)",
		totalItems, numWorkers, duration, throughput)

	// Verify log file
	logFile := filepath.Join(testLogDir, "concurrent.log")
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

func TestOptimizedChannelSize(t *testing.T) {
	testLogDir := "test_batch"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	// Test different channel sizes
	channelSizes := []int{10, 100, 1000}
	messages := 200

	for _, size := range channelSizes {
		logFile := filepath.Join(testLogDir, fmt.Sprintf("channel_%d.log", size))
		logger, err := omni.NewWithOptions(
			omni.WithPath(logFile),
			omni.WithLevel(omni.LevelInfo),
			omni.WithChannelSize(size),
		)
		if err != nil {
			t.Fatalf("Failed to create logger with channel size %d: %v", size, err)
		}

		start := time.Now()
		for i := 0; i < messages; i++ {
			logger.InfoWithFields("Channel size test", map[string]interface{}{
				"channel_size": size,
				"message_id":   i,
			})
		}
		duration := time.Since(start)

		logger.FlushAll()
		time.Sleep(10 * time.Millisecond)
		logger.Close()

		throughput := float64(messages) / duration.Seconds()
		t.Logf("Channel size %d: %.0f msgs/sec", size, throughput)

		// Verify log file
		if stat, err := os.Stat(logFile); err != nil {
			t.Errorf("Log file error for channel size %d: %v", size, err)
		} else if stat.Size() == 0 {
			t.Errorf("Log file is empty for channel size %d", size)
		}
	}
}

func TestStructuredBatchLogging(t *testing.T) {
	testLogDir := "test_batch"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	logger, err := omni.NewWithOptions(
		omni.WithPath(filepath.Join(testLogDir, "structured_batch.log")),
		omni.WithLevel(omni.LevelInfo),
		omni.WithJSON(),
		omni.WithChannelSize(500),
	)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Test structured logging with complex data
	for i := 0; i < 50; i++ {
		logger.InfoWithFields("Complex structured data", map[string]interface{}{
			"batch_metadata": map[string]interface{}{
				"batch_id":    i / 10,
				"item_index":  i % 10,
				"total_items": 50,
			},
			"processing_info": map[string]interface{}{
				"start_time":     time.Now().Unix(),
				"worker_thread":  "main",
				"priority":       "normal",
			},
			"data_fields": map[string]interface{}{
				"user_id":      fmt.Sprintf("user_%d", i),
				"action_type":  "batch_process",
				"success":      i%5 != 0, // Some failures
				"duration_ms":  (i * 10) + 50,
			},
		})
	}

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	logger.Close()

	// Verify log file
	logFile := filepath.Join(testLogDir, "structured_batch.log")
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

// Benchmark tests
func BenchmarkHighThroughputLogging(b *testing.B) {
	testLogDir := "bench_batch"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	logger, err := omni.NewWithOptions(
		omni.WithPath(filepath.Join(testLogDir, "bench_throughput.log")),
		omni.WithLevel(omni.LevelInfo),
		omni.WithChannelSize(1000),
	)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark high throughput message")
	}
}

func BenchmarkBatchStructuredLogging(b *testing.B) {
	testLogDir := "bench_batch"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	logger, err := omni.NewWithOptions(
		omni.WithPath(filepath.Join(testLogDir, "bench_structured.log")),
		omni.WithLevel(omni.LevelInfo),
		omni.WithJSON(),
		omni.WithChannelSize(1000),
	)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	fields := map[string]interface{}{
		"batch_id": 1,
		"user_id":  "bench_user",
		"action":   "benchmark",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.InfoWithFields("Benchmark structured batch message", fields)
	}
}

func BenchmarkConcurrentBatchLogging(b *testing.B) {
	testLogDir := "bench_batch"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	logger, err := omni.NewWithOptions(
		omni.WithPath(filepath.Join(testLogDir, "bench_concurrent.log")),
		omni.WithLevel(omni.LevelInfo),
		omni.WithChannelSize(2000),
	)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("Concurrent benchmark message")
		}
	})
}