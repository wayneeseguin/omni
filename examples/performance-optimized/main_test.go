package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/wayneeseguin/omni/pkg/omni"
)

func TestMain(m *testing.M) {
	// Setup: clean up any existing test files
	os.Remove("test_performance.log")
	os.RemoveAll("test_perf")

	code := m.Run()

	// Cleanup: remove test files
	os.Remove("test_performance.log")
	os.RemoveAll("test_perf")
	os.Exit(code)
}

func TestPerformanceOptimizedExample(t *testing.T) {
	// Test basic performance logging functionality
	logger, err := omni.New("test_performance.log")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() {
		time.Sleep(10 * time.Millisecond)
		logger.Close()
	}()

	// Test with INFO level (filtering out TRACE/DEBUG)
	logger.SetLevel(omni.LevelInfo)

	// Test that TRACE and DEBUG are filtered
	start := time.Now()
	for i := 0; i < 1000; i++ {
		logger.Trace("This should be filtered")
		logger.Debug("This should be filtered")
		if i%100 == 0 {
			logger.Info("Progress update", "iteration", i)
		}
	}
	filteredDuration := time.Since(start)

	// Test with TRACE level (all messages logged)
	logger.SetLevel(omni.LevelTrace)
	start = time.Now()
	for i := 0; i < 1000; i++ {
		logger.Trace("This will be logged")
		logger.Debug("This will be logged")
		if i%100 == 0 {
			logger.Info("Progress update", "iteration", i)
		}
	}
	unfilteredDuration := time.Since(start)

	// Verify filtering provides performance benefit
	if filteredDuration >= unfilteredDuration {
		t.Logf("Warning: Expected filtered logging to be faster. Filtered: %v, Unfiltered: %v",
			filteredDuration, unfilteredDuration)
	}

	logger.FlushAll()
	time.Sleep(50 * time.Millisecond) // Allow time for file to be created

	// Verify log file was created
	if stat, err := os.Stat("test_performance.log"); err != nil {
		t.Errorf("Log file was not created: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

func TestSingleThreadedPerformance(t *testing.T) {
	testLogDir := "test_perf"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "single_thread.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	logger.SetLevel(omni.LevelInfo)

	// Measure single-threaded performance
	iterations := 1000
	start := time.Now()

	for i := 0; i < iterations; i++ {
		// These should be filtered for better performance
		logger.Trace("Filtered trace message")
		logger.Debug("Filtered debug message")

		// Only log occasionally
		if i%100 == 0 {
			logger.InfoWithFields("Performance test", map[string]interface{}{
				"iteration": i,
				"thread":    "single",
			})
		}
	}

	duration := time.Since(start)
	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	logger.Close()

	// Calculate throughput
	throughput := float64(iterations) / duration.Seconds()
	t.Logf("Single-threaded throughput: %.0f ops/sec", throughput)

	// Basic performance check - should handle at least 1000 ops/sec
	if throughput < 1000 {
		t.Logf("Warning: Low throughput detected: %.0f ops/sec", throughput)
	}

	// Verify log file has content
	logFile := filepath.Join(testLogDir, "single_thread.log")
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

func TestMultiThreadedPerformance(t *testing.T) {
	testLogDir := "test_perf"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "multi_thread.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	logger.SetLevel(omni.LevelInfo)

	// Test multi-threaded performance
	numGoroutines := 4
	iterationsPerGoroutine := 250
	totalIterations := numGoroutines * iterationsPerGoroutine

	start := time.Now()
	var wg sync.WaitGroup

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for i := 0; i < iterationsPerGoroutine; i++ {
				// These should be filtered
				logger.Trace("Goroutine trace", "goroutine", goroutineID)
				logger.Debug("Goroutine debug", "goroutine", goroutineID)

				// Log occasionally
				if i%50 == 0 {
					logger.InfoWithFields("Goroutine progress", map[string]interface{}{
						"goroutine": goroutineID,
						"iteration": i,
					})
				}
			}
		}(g)
	}

	wg.Wait()
	duration := time.Since(start)
	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	logger.Close()

	// Calculate throughput
	throughput := float64(totalIterations) / duration.Seconds()
	t.Logf("Multi-threaded throughput: %.0f ops/sec (%d goroutines)", throughput, numGoroutines)

	// Multi-threaded should generally be faster than single-threaded
	if throughput < 1000 {
		t.Logf("Warning: Low multi-threaded throughput: %.0f ops/sec", throughput)
	}

	// Verify log file has content
	logFile := filepath.Join(testLogDir, "multi_thread.log")
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

func TestLevelFilteringPerformance(t *testing.T) {
	testLogDir := "test_perf"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "level_filter.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	iterations := 1000

	// Test with TRACE level enabled (expensive)
	logger.SetLevel(omni.LevelTrace)
	start := time.Now()

	for i := 0; i < iterations; i++ {
		logger.Trace("Expensive trace message", "value", i*2)
		logger.Debug("Debug message", "iteration", i)
		if i%100 == 0 {
			logger.Info("Progress with TRACE enabled", "iteration", i)
		}
	}

	withTraceDuration := time.Since(start)

	// Test with INFO level (filtered TRACE/DEBUG)
	logger.SetLevel(omni.LevelInfo)
	start = time.Now()

	for i := 0; i < iterations; i++ {
		logger.Trace("Expensive trace message", "value", i*2) // Filtered
		logger.Debug("Debug message", "iteration", i)         // Filtered
		if i%100 == 0 {
			logger.Info("Progress with TRACE filtered", "iteration", i)
		}
	}

	withoutTraceDuration := time.Since(start)

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	logger.Close()

	t.Logf("With TRACE enabled: %v", withTraceDuration)
	t.Logf("With TRACE filtered: %v", withoutTraceDuration)

	// Filtered should be faster (though margin may vary)
	if withoutTraceDuration >= withTraceDuration {
		t.Logf("Note: Filtered logging was not significantly faster. With: %v, Without: %v",
			withTraceDuration, withoutTraceDuration)
	} else {
		improvement := float64(withTraceDuration) / float64(withoutTraceDuration)
		t.Logf("Performance improvement: %.2fx faster with filtering", improvement)
	}

	// Verify log file has content
	logFile := filepath.Join(testLogDir, "level_filter.log")
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

func TestMemoryUsagePatterns(t *testing.T) {
	testLogDir := "test_perf"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "memory_test.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	logger.SetLevel(omni.LevelTrace)

	var m1, m2 runtime.MemStats

	// Force GC and get baseline memory stats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// Generate logs that will allocate memory
	iterations := 100 // Reduced for testing
	for i := 0; i < iterations; i++ {
		logger.TraceWithFields("Memory test", map[string]interface{}{
			"iteration": i,
			"data":      fmt.Sprintf("test-data-%d", i),
			"timestamp": time.Now(),
		})
	}

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)

	// Force GC and get final memory stats
	runtime.GC()
	runtime.ReadMemStats(&m2)

	logger.Close()

	allocatedBytes := m2.TotalAlloc - m1.TotalAlloc
	allocatedKB := allocatedBytes / 1024

	t.Logf("Memory allocated for %d TRACE messages: %d KB (%d bytes)",
		iterations, allocatedKB, allocatedBytes)

	// Basic sanity check - shouldn't allocate excessive memory
	bytesPerMessage := float64(allocatedBytes) / float64(iterations)
	t.Logf("Average memory per message: %.1f bytes", bytesPerMessage)

	if bytesPerMessage > 10000 { // 10KB per message seems excessive
		t.Errorf("High memory usage per message: %.1f bytes", bytesPerMessage)
	}

	// Verify log file has content
	logFile := filepath.Join(testLogDir, "memory_test.log")
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

func TestPerformanceWithStructuredLogging(t *testing.T) {
	testLogDir := "test_perf"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "structured_perf.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	logger.SetLevel(omni.LevelInfo)

	iterations := 500
	start := time.Now()

	for i := 0; i < iterations; i++ {
		logger.InfoWithFields("Performance test with structured data", map[string]interface{}{
			"iteration":   i,
			"timestamp":   time.Now().Unix(),
			"user_id":     fmt.Sprintf("user-%d", i%100),
			"action":      "test_action",
			"duration_ms": i * 2,
			"success":     i%10 != 0,
		})
	}

	duration := time.Since(start)
	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	logger.Close()

	throughput := float64(iterations) / duration.Seconds()
	t.Logf("Structured logging throughput: %.0f ops/sec", throughput)

	// Structured logging should still be reasonably fast
	if throughput < 100 {
		t.Logf("Warning: Low structured logging throughput: %.0f ops/sec", throughput)
	}

	// Verify log file has content
	logFile := filepath.Join(testLogDir, "structured_perf.log")
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

// Benchmark tests
func BenchmarkBasicLogging(b *testing.B) {
	testLogDir := "bench_perf"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "bench_basic.log"))
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.SetLevel(omni.LevelInfo)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark message")
	}
}

func BenchmarkFilteredLogging(b *testing.B) {
	testLogDir := "bench_perf"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "bench_filtered.log"))
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set to INFO level so TRACE/DEBUG are filtered
	logger.SetLevel(omni.LevelInfo)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Trace("This will be filtered") // Should be very fast
		logger.Debug("This will be filtered") // Should be very fast
		if i%10 == 0 {
			logger.Info("Occasional info message")
		}
	}
}

func BenchmarkStructuredLogging(b *testing.B) {
	testLogDir := "bench_perf"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "bench_structured.log"))
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.SetLevel(omni.LevelInfo)

	fields := map[string]interface{}{
		"user_id": "bench_user",
		"action":  "bench_action",
		"count":   42,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.InfoWithFields("Benchmark structured message", fields)
	}
}

func BenchmarkConcurrentLogging(b *testing.B) {
	testLogDir := "bench_perf"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "bench_concurrent.log"))
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.SetLevel(omni.LevelInfo)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("Concurrent benchmark message")
		}
	})
}
