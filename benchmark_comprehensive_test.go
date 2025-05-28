package flexlog

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// Baseline benchmarks for comparison

func BenchmarkStdLogger(b *testing.B) {
	// Create a discard writer for fair comparison
	logger := bufio.NewWriter(io.Discard)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fmt.Fprintf(logger, "[INFO] Benchmark message %d\n", i)
	}
	logger.Flush()
}

func BenchmarkFlexLogBasic(b *testing.B) {
	tmpDir := b.TempDir()
	logger, err := New(filepath.Join(tmpDir, "bench.log"))
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark message %d", i)
	}
	logger.Sync()
}

// Throughput benchmarks

func BenchmarkThroughput(b *testing.B) {
	scenarios := []struct {
		name         string
		messageSize  int
		destinations int
		compression  CompressionType
		batching     bool
	}{
		{"Small_1Dest", 50, 1, CompressionNone, false},
		{"Small_3Dest", 50, 3, CompressionNone, false},
		{"Large_1Dest", 500, 1, CompressionNone, false},
		{"Large_3Dest", 500, 3, CompressionNone, false},
		{"Small_1Dest_Gzip", 50, 1, CompressionGzip, false},
		{"Large_1Dest_Gzip", 500, 1, CompressionGzip, false},
		{"Small_1Dest_Batch", 50, 1, CompressionNone, true},
		{"Large_3Dest_Batch", 500, 3, CompressionNone, true},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			tmpDir := b.TempDir()
			logger, err := New(filepath.Join(tmpDir, "bench.log"))
			if err != nil {
				b.Fatalf("Failed to create logger: %v", err)
			}
			defer logger.Close()

			// Configure logger
			logger.SetCompression(scenario.compression)

			// Add destinations
			for i := 1; i < scenario.destinations; i++ {
				path := filepath.Join(tmpDir, fmt.Sprintf("dest%d.log", i))
				if err := logger.AddDestination(path); err != nil {
					b.Fatalf("Failed to add destination: %v", err)
				}
			}

			// Create message of specified size
			message := make([]byte, scenario.messageSize)
			for i := range message {
				message[i] = 'a' + byte(i%26)
			}
			messageStr := string(message)

			b.SetBytes(int64(scenario.messageSize))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				logger.Info(messageStr)
			}

			logger.Sync()
		})
	}
}

// Latency benchmarks

func BenchmarkLatency(b *testing.B) {
	tmpDir := b.TempDir()
	logger, err := New(filepath.Join(tmpDir, "bench.log"))
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Measure individual message latency
	latencies := make([]time.Duration, b.N)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		logger.Info("Latency test message %d", i)
		latencies[i] = time.Since(start)
	}

	// Calculate percentiles
	if b.N > 0 {
		p50 := latencies[b.N*50/100]
		p95 := latencies[b.N*95/100]
		p99 := latencies[b.N*99/100]

		b.Logf("Latency - P50: %v, P95: %v, P99: %v", p50, p95, p99)
	}
}

// Concurrent benchmarks

func BenchmarkConcurrent(b *testing.B) {
	concurrencyLevels := []int{1, 2, 4, 8, 16}

	for _, level := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency_%d", level), func(b *testing.B) {
			tmpDir := b.TempDir()
			logger, err := New(filepath.Join(tmpDir, "bench.log"))
			if err != nil {
				b.Fatalf("Failed to create logger: %v", err)
			}
			defer logger.Close()

			b.SetParallelism(level)
			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					logger.Info("Concurrent message %d", i)
					i++
				}
			})

			logger.Sync()
		})
	}
}

// Memory allocation benchmarks

func BenchmarkMemoryAllocation(b *testing.B) {
	scenarios := []struct {
		name       string
		bufferPool bool
		lazyFormat bool
	}{
		{"Default", false, false},
		{"WithBufferPool", true, false},
		{"WithLazyFormat", false, true},
		{"Optimized", true, true},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			tmpDir := b.TempDir()

			config := &Config{
				Path:             filepath.Join(tmpDir, "bench.log"),
				EnableBufferPool: scenario.bufferPool,
				EnableLazyFormat: scenario.lazyFormat,
			}

			logger, err := NewWithConfig(config)
			if err != nil {
				b.Fatalf("Failed to create logger: %v", err)
			}
			defer logger.Close()

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				logger.Info("Memory allocation test %d with some fields", i)
			}

			logger.Sync()
		})
	}
}

// Rotation performance benchmark

func BenchmarkRotation(b *testing.B) {
	tmpDir := b.TempDir()
	logger, err := New(filepath.Join(tmpDir, "bench.log"))
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set small rotation size
	logger.SetMaxSize(1024) // 1KB

	messageSize := 100
	message := make([]byte, messageSize)
	for i := range message {
		message[i] = 'x'
	}

	b.SetBytes(int64(messageSize))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		logger.Info(string(message))
	}

	logger.Sync()
}

// Structured logging benchmark

func BenchmarkStructuredLogging(b *testing.B) {
	tmpDir := b.TempDir()
	logger, err := New(filepath.Join(tmpDir, "bench.log"))
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set JSON format
	logger.SetFormat(FormatJSON)

	fields := map[string]interface{}{
		"user_id":    12345,
		"request_id": "abc-123",
		"duration":   1.234,
		"status":     "success",
		"count":      42,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		logger.WithFields(fields).Info("Structured log entry")
	}

	logger.Sync()
}

// Filtering performance benchmark

func BenchmarkFiltering(b *testing.B) {
	scenarios := []struct {
		name   string
		filter Filter
	}{
		{
			name:   "NoFilter",
			filter: nil,
		},
		{
			name: "LevelFilter",
			filter: func(level int, message string, fields map[string]interface{}) bool {
				return level >= LevelWarn
			},
		},
		{
			name: "ComplexFilter",
			filter: func(level int, message string, fields map[string]interface{}) bool {
				if level < LevelInfo {
					return false
				}
				if fields != nil && fields["skip"] == true {
					return false
				}
				return true
			},
		},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			tmpDir := b.TempDir()
			logger, err := New(filepath.Join(tmpDir, "bench.log"))
			if err != nil {
				b.Fatalf("Failed to create logger: %v", err)
			}
			defer logger.Close()

			if scenario.filter != nil {
				logger.SetFilter(scenario.filter)
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Mix of levels to test filtering
				switch i % 4 {
				case 0:
					logger.Debug("Debug message %d", i)
				case 1:
					logger.Info("Info message %d", i)
				case 2:
					logger.Warn("Warn message %d", i)
				case 3:
					logger.Error("Error message %d", i)
				}
			}

			logger.Sync()
		})
	}
}

// Buffer pool benchmark comparison

func BenchmarkBufferPoolComparison(b *testing.B) {
	b.Run("WithPool", func(b *testing.B) {
		pool := NewBufferPool()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := pool.Get()
			buf.WriteString("Test message with buffer pool")
			buf.WriteByte('\n')
			_ = buf.String()
			pool.Put(buf)
		}
	})

	b.Run("WithoutPool", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			buf.WriteString("Test message without buffer pool")
			buf.WriteByte('\n')
			_ = buf.String()
		}
	})
}

// End-to-end benchmark simulating real usage

func BenchmarkRealWorldScenario(b *testing.B) {
	tmpDir := b.TempDir()

	// Create logger with realistic configuration
	logger, err := New(filepath.Join(tmpDir, "app.log"))
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Configure for production-like settings
	logger.SetMaxSize(10 * 1024 * 1024) // 10MB
	logger.SetMaxFiles(5)
	logger.SetCompression(CompressionGzip)
	logger.SetLevel(LevelInfo) // Filter out debug

	// Add error log destination
	errorLog := filepath.Join(tmpDir, "errors.log")
	if err := logger.AddDestination(errorLog); err != nil {
		b.Fatalf("Failed to add error destination: %v", err)
	}

	// Simulate mixed workload
	b.ResetTimer()

	var wg sync.WaitGroup

	// Application logs
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < b.N/4; i++ {
			logger.Info("Request processed successfully for user %d", i%1000)
		}
	}()

	// Debug logs (filtered out)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < b.N/4; i++ {
			logger.Debug("Debug details: cache hit for key %d", i)
		}
	}()

	// Warning logs
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < b.N/4; i++ {
			if i%10 == 0 {
				logger.Warn("High memory usage detected: %d%%", 80+i%20)
			}
		}
	}()

	// Error logs
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < b.N/4; i++ {
			if i%100 == 0 {
				logger.Error("Database connection failed, retrying...")
			}
		}
	}()

	wg.Wait()
	logger.Sync()
}
