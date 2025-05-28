package flexlog

import (
	"path/filepath"
	"testing"
	"time"
)

// BenchmarkLoggingWithoutPooling benchmarks logging performance without buffer pooling
func BenchmarkLoggingWithoutPooling(b *testing.B) {
	tempDir := b.TempDir()
	logFile := filepath.Join(tempDir, "bench_no_pool.log")

	// Temporarily disable buffer pooling (simulate old behavior)
	oldPool := globalBufferPool
	globalBufferPool = nil
	defer func() { globalBufferPool = oldPool }()

	logger, err := New(logFile)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Disable batching for fair comparison
	logger.SetFlushInterval(0, 0)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark message without pooling")
	}

	logger.FlushAll()
}

// BenchmarkLoggingWithPooling benchmarks logging performance with buffer pooling
func BenchmarkLoggingWithPooling(b *testing.B) {
	tempDir := b.TempDir()
	logFile := filepath.Join(tempDir, "bench_with_pool.log")

	logger, err := New(logFile)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Disable batching for fair comparison
	logger.SetFlushInterval(0, 0)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark message with pooling")
	}

	logger.FlushAll()
}

// BenchmarkLoggingWithBatching benchmarks logging performance with batching enabled
func BenchmarkLoggingWithBatching(b *testing.B) {
	tempDir := b.TempDir()
	logFile := filepath.Join(tempDir, "bench_batching.log")

	logger, err := New(logFile)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Enable batching
	logger.SetFlushInterval(0, 100*time.Millisecond)
	logger.SetFlushSize(0, 8192)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark message with batching")
	}

	// Wait for final flush
	time.Sleep(150 * time.Millisecond)
}

// BenchmarkStructuredLogging benchmarks structured logging performance
func BenchmarkStructuredLogging(b *testing.B) {
	tempDir := b.TempDir()
	logFile := filepath.Join(tempDir, "bench_structured.log")

	logger, err := New(logFile)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.SetFormat(FormatJSON)

	fields := map[string]interface{}{
		"user_id":    12345,
		"request_id": "abc-123-def",
		"duration":   123.45,
		"status":     "success",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		logger.InfoWithFields("Request completed", fields)
	}

	logger.FlushAll()
}

// BenchmarkConcurrentLogging benchmarks concurrent logging performance
func BenchmarkConcurrentLogging(b *testing.B) {
	tempDir := b.TempDir()
	logFile := filepath.Join(tempDir, "bench_concurrent.log")

	logger, err := New(logFile)
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

	logger.FlushAll()
}

// BenchmarkMultiDestination benchmarks logging to multiple destinations
func BenchmarkMultiDestination(b *testing.B) {
	tempDir := b.TempDir()

	logger, err := New(filepath.Join(tempDir, "bench_dest1.log"))
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Add more destinations
	for i := 2; i <= 3; i++ {
		if err := logger.AddDestination(filepath.Join(tempDir, "bench_dest"+string(rune('0'+i))+".log")); err != nil {
			b.Fatalf("Failed to add destination %d: %v", i, err)
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		logger.Info("Multi-destination benchmark message")
	}

	logger.FlushAll()
}

// BenchmarkMessageFormatting benchmarks just the message formatting performance
func BenchmarkMessageFormatting(b *testing.B) {
	logger := &FlexLog{
		formatOpts: defaultFormatOptions(),
		format:     FormatText,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = logger.formatMessage(LevelInfo, "Benchmark message %d with %s", i, "parameters")
	}
}

// BenchmarkBufferPoolGet benchmarks buffer pool Get operation
func BenchmarkBufferPoolGet(b *testing.B) {
	pool := NewBufferPool()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf := pool.Get()
		pool.Put(buf)
	}
}

// BenchmarkAllocations benchmarks memory allocations during logging
func BenchmarkAllocations(b *testing.B) {
	tempDir := b.TempDir()
	logFile := filepath.Join(tempDir, "bench_alloc.log")

	logger, err := New(logFile)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Use a larger channel to avoid overflow
	logger.SetLevel(LevelInfo)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		logger.Info("Allocation benchmark message")
	}

	logger.FlushAll()
}

// BenchmarkLargeMessages benchmarks performance with large log messages
func BenchmarkLargeMessages(b *testing.B) {
	tempDir := b.TempDir()
	logFile := filepath.Join(tempDir, "bench_large.log")

	logger, err := New(logFile)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Create a large message
	largeData := make([]byte, 1024)
	for i := range largeData {
		largeData[i] = 'X'
	}
	largeMsg := string(largeData)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		logger.Info("Large message: " + largeMsg)
	}

	logger.FlushAll()
}

// BenchmarkRotationOverhead benchmarks the overhead of file rotation
func BenchmarkRotationOverhead(b *testing.B) {
	tempDir := b.TempDir()
	logFile := filepath.Join(tempDir, "bench_rotation.log")

	logger, err := New(logFile)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set small file size to force frequent rotation
	logger.SetMaxSize(1024) // 1KB
	logger.SetMaxFiles(10)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		logger.Info("Rotation benchmark message that should trigger rotation periodically")
	}

	logger.FlushAll()
}

// BenchmarkChannelThroughput benchmarks message channel throughput
func BenchmarkChannelThroughput(b *testing.B) {
	ch := make(chan LogMessage, 1000)
	done := make(chan bool)

	// Consumer
	go func() {
		for range ch {
			// Just consume messages
		}
		done <- true
	}()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ch <- LogMessage{
			Level:     LevelInfo,
			Timestamp: time.Now(),
		}
	}

	close(ch)
	<-done
}
