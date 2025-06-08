package omni

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// BenchmarkBaseline establishes performance baseline before optimizations
func BenchmarkBaseline(b *testing.B) {
	benchmarks := []struct {
		name      string
		parallel  int
		msgSize   int
		structured bool
	}{
		{"Serial/Small", 1, 100, false},
		{"Serial/Large", 1, 1000, false},
		{"Parallel/Small/4", 4, 100, false},
		{"Parallel/Small/8", 8, 100, false},
		{"Parallel/Large/4", 4, 1000, false},
		{"Structured/Serial", 1, 100, true},
		{"Structured/Parallel/4", 4, 100, true},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			dir := b.TempDir()
			logFile := filepath.Join(dir, "bench.log")

			logger, err := New(logFile)
			if err != nil {
				b.Fatalf("Failed to create logger: %v", err)
			}
			defer logger.Close()

			// Prepare message
			msg := generateMessage(bm.msgSize)
			fields := map[string]interface{}{
				"user_id":    12345,
				"request_id": "abc-123",
				"latency":    123.45,
			}

			b.ResetTimer()
			b.ReportAllocs()

			if bm.parallel > 1 {
				b.SetParallelism(bm.parallel)
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						if bm.structured {
							logger.WithFields(fields).Info(msg)
						} else {
							logger.Info(msg)
						}
					}
				})
			} else {
				for i := 0; i < b.N; i++ {
					if bm.structured {
						logger.WithFields(fields).Info(msg)
					} else {
						logger.Info(msg)
					}
				}
			}

			// Wait for messages to be processed
			time.Sleep(100 * time.Millisecond)
		})
	}
}

// BenchmarkAllocationProfile measures allocations in hot paths
func BenchmarkAllocationProfile(b *testing.B) {
	dir := b.TempDir()
	logFile := filepath.Join(dir, "bench.log")

	logger, err := New(logFile)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	b.Run("Simple", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			logger.Info("Simple log message")
		}
	})

	b.Run("Formatted", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			logger.Info("User %d performed action %s", 123, "login")
		}
	})

	b.Run("WithFields", func(b *testing.B) {
		b.ReportAllocs()
		fields := map[string]interface{}{
			"user_id": 123,
			"action":  "login",
		}
		for i := 0; i < b.N; i++ {
			logger.WithFields(fields).Info("User action")
		}
	})
}

// BenchmarkChannelThroughputBaseline measures message channel throughput
func BenchmarkChannelThroughputBaseline(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("ChannelSize-%d", size), func(b *testing.B) {
			dir := b.TempDir()
			logFile := filepath.Join(dir, "bench.log")

			// Create logger with specific channel size
			logger, err := New(logFile)
			if err != nil {
				b.Fatalf("Failed to create logger: %v", err)
			}
			logger.channelSize = size
			defer logger.Close()

			b.ResetTimer()
			b.ReportAllocs()

			start := time.Now()
			for i := 0; i < b.N; i++ {
				logger.Info("Throughput test message %d", i)
			}
			elapsed := time.Since(start)

			// Calculate messages per second
			throughput := float64(b.N) / elapsed.Seconds()
			b.ReportMetric(throughput, "msgs/sec")
		})
	}
}

// BenchmarkLatency measures end-to-end latency
func BenchmarkLatency(b *testing.B) {
	dir := b.TempDir()
	logFile := filepath.Join(dir, "bench.log")

	logger, err := New(logFile)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Measure individual message latency
	latencies := make([]time.Duration, 0, b.N)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		logger.Info("Latency test message %d", i)
		// Sync to ensure message is written
		logger.Sync()
		latencies = append(latencies, time.Since(start))
	}

	// Calculate percentiles
	if len(latencies) > 0 {
		p50 := percentile(latencies, 0.50)
		p95 := percentile(latencies, 0.95)
		p99 := percentile(latencies, 0.99)

		b.ReportMetric(float64(p50.Nanoseconds()), "p50-ns")
		b.ReportMetric(float64(p95.Nanoseconds()), "p95-ns")
		b.ReportMetric(float64(p99.Nanoseconds()), "p99-ns")
	}
}

// BenchmarkMemoryUsage tracks memory usage patterns
func BenchmarkMemoryUsage(b *testing.B) {
	dir := b.TempDir()
	logFile := filepath.Join(dir, "bench.log")

	logger, err := New(logFile)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Force GC before starting
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("Memory usage test message %d with some extra data", i)
		if i%1000 == 0 {
			runtime.GC()
		}
	}

	// Force GC and read final stats
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	// Report memory metrics
	heapGrowth := int64(m2.HeapAlloc) - int64(m1.HeapAlloc)
	b.ReportMetric(float64(heapGrowth)/float64(b.N), "heap-bytes/op")
	b.ReportMetric(float64(m2.NumGC-m1.NumGC), "gc-runs")
}

// Helper functions

func generateMessage(size int) string {
	if size <= 0 {
		size = 100
	}
	// Create a message of approximately the requested size
	base := "This is a log message with some content. "
	repeat := size / len(base)
	if repeat < 1 {
		repeat = 1
	}
	result := ""
	for i := 0; i < repeat; i++ {
		result += base
	}
	// Ensure we don't exceed the actual length of the result
	if len(result) > size {
		return result[:size]
	}
	// If result is shorter than requested size, pad it
	for len(result) < size {
		result += "x"
	}
	return result
}

func percentile(latencies []time.Duration, p float64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	
	// Simple percentile calculation (not fully accurate but good enough for benchmarks)
	index := int(float64(len(latencies)) * p)
	if index >= len(latencies) {
		index = len(latencies) - 1
	}
	return latencies[index]
}