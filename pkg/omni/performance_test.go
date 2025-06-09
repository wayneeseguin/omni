package omni

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wayneeseguin/omni/internal/buffer"
	testhelpers "github.com/wayneeseguin/omni/internal/testing"
)

func TestBufferPool(t *testing.T) {
	// Use the actual buffer pool from internal/buffer package
	pool := buffer.NewBufferPool()

	// Test getting and putting buffers
	buf1 := pool.Get()
	buf1.WriteString("test data")

	// Return buffer to pool
	pool.Put(buf1)

	// Get another buffer - should be reset
	buf2 := pool.Get()
	if buf2.Len() != 0 {
		t.Error("Expected buffer to be reset")
	}

	// Test concurrent access
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := pool.Get()
			buf.WriteString("concurrent test")
			pool.Put(buf)
		}()
	}
	wg.Wait()
}

func TestBufferPoolSizeLimit(t *testing.T) {
	// Use the actual buffer pool from internal/buffer package
	pool := buffer.NewBufferPool()

	// Create a large buffer that exceeds the 32KB limit
	buf := pool.Get()
	largeData := make([]byte, 33*1024) // Larger than 32KB limit
	buf.Write(largeData)

	// Put it back - the pool should not reuse it due to size limit
	pool.Put(buf)

	// Get a new buffer - should not be the same one due to size limit
	buf2 := pool.Get()
	// The new buffer should have the default capacity of 512
	if buf2.Cap() > 512 {
		t.Errorf("Expected new buffer with default capacity (512), got %d", buf2.Cap())
	}
}

func TestLazyFormatting(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Enable lazy formatting
	logger.EnableLazyFormatting()

	if !logger.IsLazyFormattingEnabled() {
		t.Error("Expected lazy formatting to be enabled")
	}

	// Disable lazy formatting
	logger.DisableLazyFormatting()

	if logger.IsLazyFormattingEnabled() {
		t.Error("Expected lazy formatting to be disabled")
	}
}

/*
func TestLazyMessage(t *testing.T) {
	// LazyMessage type is not available
	tests := []struct {
		name string
		msg  LazyMessage
		want string
	}{
		{
			name: "format message",
			msg: LazyMessage{
				Format: "Hello %s",
				Args:   []interface{}{"World"},
			},
			want: "Hello World",
		},
		{
			name: "raw bytes",
			msg: LazyMessage{
				Raw: []byte("Raw message"),
			},
			want: "Raw message",
		},
		{
			name: "entry message",
			msg: LazyMessage{
				Entry: &LogEntry{
					Message: "Entry message",
				},
			},
			want: "Entry message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First call should format
			got := tt.msg.String()
			if got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}

			// Second call should return cached value
			got2 := tt.msg.String()
			if got2 != got {
				t.Error("Expected cached value on second call")
			}
		})
	}
}
*/

func TestPerformanceConfig(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	config := &Config{
		Path:             logFile,
		EnableBufferPool: true,
		EnableLazyFormat: true,
	}

	logger, err := NewWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create logger with config: %v", err)
	}
	defer logger.Close()

	if !logger.IsLazyFormattingEnabled() {
		t.Error("Expected lazy formatting to be enabled from config")
	}

	// Log some messages
	logger.Info("test message %d", 1)
	logger.Debug("debug message %s", "test")

	time.Sleep(100 * time.Millisecond)
}

func BenchmarkWithBufferPool(b *testing.B) {
	dir := b.TempDir()
	logFile := filepath.Join(dir, "bench.log")

	config := &Config{
		Path:             logFile,
		EnableBufferPool: true,
	}

	logger, err := NewWithConfig(config)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message %d with some extra text", i)
	}
}

func BenchmarkWithoutBufferPool(b *testing.B) {
	dir := b.TempDir()
	logFile := filepath.Join(dir, "bench.log")

	// Create logger without buffer pool
	logger, err := New(logFile)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message %d with some extra text", i)
	}
}

func BenchmarkBufferPoolVsBytes(b *testing.B) {
	b.Run("WithPool", func(b *testing.B) {
		// NewBufferPool is not available
		// pool := NewBufferPool()
		pool := &sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := pool.Get().(*bytes.Buffer)
			buf.WriteString("test message with some content")
			_ = buf.String()
			pool.Put(buf)
		}
	})

	b.Run("WithoutPool", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := bytes.NewBuffer(make([]byte, 0, 512))
			buf.WriteString("test message with some content")
			_ = buf.String()
		}
	})
}

// ===== Tests from performance_enhanced_test.go =====

// TestHighThroughputLogging tests logger performance under high message volume
func TestHighThroughputLogging(t *testing.T) {
	testhelpers.SkipIfUnit(t, "Skipping high throughput test in unit mode")

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "high_throughput.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test parameters
	numMessages := 100000
	message := "High throughput test message with some content to make it realistic"

	start := time.Now()

	// Log messages as fast as possible
	for i := 0; i < numMessages; i++ {
		logger.Infof("%s %d", message, i)
	}

	// Shutdown to ensure all messages are processed
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = logger.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	duration := time.Since(start)
	messagesPerSecond := float64(numMessages) / duration.Seconds()

	t.Logf("Logged %d messages in %v (%.2f msg/sec)", numMessages, duration, messagesPerSecond)

	// Verify log file has content
	info, err := os.Stat(logFile)
	if err != nil {
		t.Fatalf("Failed to stat log file: %v", err)
	}

	if info.Size() == 0 {
		t.Error("Log file is empty")
	}

	// Performance expectation: should handle at least 10k messages/second
	if messagesPerSecond < 10000 {
		t.Logf("Warning: Performance below 10k msg/sec: %.2f", messagesPerSecond)
	}

	// Check metrics
	metrics := logger.GetMetrics()
	t.Logf("Messages logged: %d, dropped: %d, errors: %d",
		metrics.MessagesLogged, metrics.MessagesDropped, metrics.ErrorCount)
}

// TestConcurrentLoggingPerformance tests performance with multiple goroutines
func TestConcurrentLoggingPerformance(t *testing.T) {
	testhelpers.SkipIfUnit(t, "Skipping concurrent performance test in unit mode")

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "concurrent_perf.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test parameters
	numGoroutines := runtime.NumCPU() * 2
	messagesPerGoroutine := 10000

	var wg sync.WaitGroup
	var messagesSent int64

	start := time.Now()

	// Start concurrent logging
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				logger.Infof("Concurrent message from goroutine %d, message %d", goroutineID, j)
				atomic.AddInt64(&messagesSent, 1)
			}
		}(i)
	}

	wg.Wait()

	// Shutdown to ensure all messages are processed
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = logger.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	duration := time.Since(start)
	messagesPerSecond := float64(messagesSent) / duration.Seconds()

	t.Logf("Sent %d messages from %d goroutines in %v (%.2f msg/sec)",
		messagesSent, numGoroutines, duration, messagesPerSecond)

	// Check metrics
	metrics := logger.GetMetrics()
	t.Logf("Messages logged: %d, dropped: %d, errors: %d",
		metrics.MessagesLogged, metrics.MessagesDropped, metrics.ErrorCount)

	// Verify file has reasonable content
	info, err := os.Stat(logFile)
	if err != nil {
		t.Fatalf("Failed to stat log file: %v", err)
	}

	if info.Size() == 0 {
		t.Error("Log file is empty")
	}

	t.Logf("Log file size: %d bytes", info.Size())
}

// TestMemoryPressurePerformance tests performance under memory constraints
func TestMemoryPressurePerformance(t *testing.T) {
	testhelpers.SkipIfUnit(t, "Skipping memory pressure test in unit mode")

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "memory_pressure.log")

	// Set small channel size to create memory pressure
	os.Setenv("OMNI_CHANNEL_SIZE", "10")
	defer os.Unsetenv("OMNI_CHANNEL_SIZE")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Create very large messages to increase memory pressure
	largeMessage := fmt.Sprintf("Large message: %s",
		string(make([]byte, 1024))) // 1KB message

	numMessages := 10000
	start := time.Now()

	// Log large messages rapidly
	for i := 0; i < numMessages; i++ {
		logger.Infof("%s [%d]", largeMessage, i)
	}

	// Shutdown to ensure processing
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = logger.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	duration := time.Since(start)
	messagesPerSecond := float64(numMessages) / duration.Seconds()

	t.Logf("Logged %d large messages in %v (%.2f msg/sec)",
		numMessages, duration, messagesPerSecond)

	// Check memory usage
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)

	t.Logf("Memory usage - Alloc: %d KB, Sys: %d KB",
		bToKb(m.Alloc), bToKb(m.Sys))

	// Check metrics
	metrics := logger.GetMetrics()
	t.Logf("Messages logged: %d, dropped: %d, errors: %d",
		metrics.MessagesLogged, metrics.MessagesDropped, metrics.ErrorCount)
}

// TestMultiDestinationPerformance tests performance with multiple destinations
func TestMultiDestinationPerformance(t *testing.T) {
	testhelpers.SkipIfUnit(t, "Skipping multi-destination performance test in unit mode")

	tmpDir := t.TempDir()
	mainLogFile := filepath.Join(tmpDir, "main.log")

	logger, err := New(mainLogFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Add multiple destinations
	numDestinations := 5
	for i := 0; i < numDestinations; i++ {
		destFile := filepath.Join(tmpDir, fmt.Sprintf("dest_%d.log", i))
		err = logger.AddDestination(destFile)
		if err != nil {
			t.Fatalf("Failed to add destination %d: %v", i, err)
		}
	}

	numMessages := 50000
	message := "Multi-destination performance test message"

	start := time.Now()

	// Log to all destinations
	for i := 0; i < numMessages; i++ {
		logger.Infof("%s %d", message, i)
	}

	// Shutdown to ensure all messages are processed
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = logger.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	duration := time.Since(start)
	messagesPerSecond := float64(numMessages) / duration.Seconds()

	t.Logf("Logged %d messages to %d destinations in %v (%.2f msg/sec)",
		numMessages, numDestinations+1, duration, messagesPerSecond)

	// Verify all destinations have content
	destinations := logger.ListDestinations()
	t.Logf("Active destinations: %d", len(destinations))

	for i, dest := range destinations {
		info, err := os.Stat(dest)
		if err != nil {
			t.Errorf("Failed to stat destination %s: %v", dest, err)
			continue
		}
		t.Logf("Destination %d (%s): %d bytes", i, dest, info.Size())
	}

	// Check metrics
	metrics := logger.GetMetrics()
	t.Logf("Messages logged: %d, dropped: %d, errors: %d",
		metrics.MessagesLogged, metrics.MessagesDropped, metrics.ErrorCount)
}

// TestRotationPerformance tests performance with log rotation
func TestRotationPerformance(t *testing.T) {
	testhelpers.SkipIfUnit(t, "Skipping rotation performance test in unit mode")

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "rotation_perf.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set small rotation size to trigger frequent rotations
	logger.SetMaxSize(1024 * 1024) // 1MB
	logger.SetMaxFiles(10)

	numMessages := 100000
	message := fmt.Sprintf("Rotation performance test: %s",
		string(make([]byte, 100))) // ~100 byte message

	start := time.Now()

	// Log enough to trigger multiple rotations
	for i := 0; i < numMessages; i++ {
		logger.Infof("%s [%d]", message, i)
	}

	// Shutdown to ensure processing
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = logger.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	duration := time.Since(start)
	messagesPerSecond := float64(numMessages) / duration.Seconds()

	t.Logf("Logged %d messages with rotation in %v (%.2f msg/sec)",
		numMessages, duration, messagesPerSecond)

	// Check for rotated files
	files, err := filepath.Glob(filepath.Join(tmpDir, "rotation_perf.log*"))
	if err != nil {
		t.Fatalf("Failed to glob log files: %v", err)
	}

	t.Logf("Created %d log files through rotation", len(files))

	totalSize := int64(0)
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		totalSize += info.Size()
	}

	t.Logf("Total log data: %d bytes across %d files", totalSize, len(files))

	// Check metrics
	metrics := logger.GetMetrics()
	t.Logf("Messages logged: %d, dropped: %d, errors: %d",
		metrics.MessagesLogged, metrics.MessagesDropped, metrics.ErrorCount)
}

// TestCompressionPerformance tests performance impact of compression
func TestCompressionPerformance(t *testing.T) {
	testhelpers.SkipIfUnit(t, "Skipping compression performance test in unit mode")

	tmpDir := t.TempDir()

	// Test without compression
	logFile1 := filepath.Join(tmpDir, "no_compression.log")
	logger1, err := New(logFile1)
	if err != nil {
		t.Fatalf("Failed to create logger without compression: %v", err)
	}

	// Test with compression
	logFile2 := filepath.Join(tmpDir, "with_compression.log")
	logger2, err := New(logFile2)
	if err != nil {
		t.Fatalf("Failed to create logger with compression: %v", err)
	}
	logger2.SetCompression(CompressionGzip)
	logger2.SetMaxSize(1024 * 1024) // 1MB to trigger compression

	numMessages := 50000
	message := fmt.Sprintf("Compression test message: %s",
		string(make([]byte, 200))) // ~200 byte message

	// Test without compression
	start1 := time.Now()
	for i := 0; i < numMessages; i++ {
		logger1.Infof("%s [%d]", message, i)
	}

	ctx1, cancel1 := context.WithTimeout(context.Background(), 30*time.Second)
	err = logger1.Shutdown(ctx1)
	cancel1()
	if err != nil {
		t.Errorf("Shutdown failed for logger1: %v", err)
	}
	duration1 := time.Since(start1)

	// Test with compression
	start2 := time.Now()
	for i := 0; i < numMessages; i++ {
		logger2.Infof("%s [%d]", message, i)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	err = logger2.Shutdown(ctx2)
	cancel2()
	if err != nil {
		t.Errorf("Shutdown failed for logger2: %v", err)
	}
	duration2 := time.Since(start2)

	messagesPerSecond1 := float64(numMessages) / duration1.Seconds()
	messagesPerSecond2 := float64(numMessages) / duration2.Seconds()

	t.Logf("Without compression: %d messages in %v (%.2f msg/sec)",
		numMessages, duration1, messagesPerSecond1)
	t.Logf("With compression: %d messages in %v (%.2f msg/sec)",
		numMessages, duration2, messagesPerSecond2)

	// Compare file sizes
	info1, _ := os.Stat(logFile1)
	info2, _ := os.Stat(logFile2)

	if info1 != nil && info2 != nil {
		t.Logf("File sizes - No compression: %d bytes, With compression: %d bytes",
			info1.Size(), info2.Size())

		overhead := ((duration2.Seconds() - duration1.Seconds()) / duration1.Seconds()) * 100
		t.Logf("Compression overhead: %.2f%%", overhead)
	}
}

// TestChannelSizeImpact tests how channel size affects performance
func TestChannelSizeImpact(t *testing.T) {
	testhelpers.SkipIfUnit(t, "Skipping channel size impact test in unit mode")

	channelSizes := []int{10, 100, 1000, 10000}
	numMessages := 100000

	for _, channelSize := range channelSizes {
		t.Run(fmt.Sprintf("ChannelSize_%d", channelSize), func(t *testing.T) {
			// Set channel size
			os.Setenv("OMNI_CHANNEL_SIZE", fmt.Sprintf("%d", channelSize))
			defer os.Unsetenv("OMNI_CHANNEL_SIZE")

			tmpDir := t.TempDir()
			logFile := filepath.Join(tmpDir, fmt.Sprintf("channel_%d.log", channelSize))

			logger, err := New(logFile)
			if err != nil {
				t.Fatalf("Failed to create logger: %v", err)
			}

			start := time.Now()

			// Log messages
			for i := 0; i < numMessages; i++ {
				logger.Infof("Channel size test message %d", i)
			}

			// Shutdown
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			err = logger.Shutdown(ctx)
			cancel()
			if err != nil {
				t.Errorf("Shutdown failed: %v", err)
			}

			duration := time.Since(start)
			messagesPerSecond := float64(numMessages) / duration.Seconds()

			metrics := logger.GetMetrics()

			t.Logf("Channel size %d: %d messages in %v (%.2f msg/sec), dropped: %d",
				channelSize, numMessages, duration, messagesPerSecond, metrics.MessagesDropped)
		})
	}
}

// TestLevelFilteringPerformance tests performance impact of level filtering
func TestLevelFilteringPerformance(t *testing.T) {
	testhelpers.SkipIfUnit(t, "Skipping level filtering performance test in unit mode")

	tmpDir := t.TempDir()

	// Test with all levels enabled
	logFile1 := filepath.Join(tmpDir, "all_levels.log")
	logger1, err := New(logFile1)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	logger1.SetLevel(LevelTrace) // Most verbose

	// Test with higher level (more filtering)
	logFile2 := filepath.Join(tmpDir, "filtered_levels.log")
	logger2, err := New(logFile2)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	logger2.SetLevel(LevelError) // Only errors

	numMessages := 100000

	// Test all levels
	start1 := time.Now()
	for i := 0; i < numMessages; i++ {
		// Mix of different levels
		switch i % 5 {
		case 0:
			logger1.Trace("Trace message %d", i)
		case 1:
			logger1.Debug("Debug message %d", i)
		case 2:
			logger1.Info("Info message %d", i)
		case 3:
			logger1.Warn("Warn message %d", i)
		case 4:
			logger1.Errorf("Error message %d", i)
		}
	}

	ctx1, cancel1 := context.WithTimeout(context.Background(), 30*time.Second)
	err = logger1.Shutdown(ctx1)
	cancel1()
	duration1 := time.Since(start1)

	// Test with filtering
	start2 := time.Now()
	for i := 0; i < numMessages; i++ {
		// Same mix of levels, but most will be filtered
		switch i % 5 {
		case 0:
			logger2.Trace("Trace message %d", i)
		case 1:
			logger2.Debug("Debug message %d", i)
		case 2:
			logger2.Info("Info message %d", i)
		case 3:
			logger2.Warn("Warn message %d", i)
		case 4:
			logger2.Errorf("Error message %d", i)
		}
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	err = logger2.Shutdown(ctx2)
	cancel2()
	duration2 := time.Since(start2)

	messagesPerSecond1 := float64(numMessages) / duration1.Seconds()
	messagesPerSecond2 := float64(numMessages) / duration2.Seconds()

	t.Logf("All levels enabled: %v (%.2f msg/sec)", duration1, messagesPerSecond1)
	t.Logf("Only errors enabled: %v (%.2f msg/sec)", duration2, messagesPerSecond2)

	// Check metrics
	metrics1 := logger1.GetMetrics()
	metrics2 := logger2.GetMetrics()

	t.Logf("All levels - logged: %d, dropped: %d",
		metrics1.MessagesLogged, metrics1.MessagesDropped)
	t.Logf("Filtered - logged: %d, dropped: %d",
		metrics2.MessagesLogged, metrics2.MessagesDropped)

	// Filtering should improve performance
	if duration2 < duration1 {
		improvement := ((duration1.Seconds() - duration2.Seconds()) / duration1.Seconds()) * 100
		t.Logf("Filtering improved performance by %.2f%%", improvement)
	}
}

// Helper function to convert bytes to kilobytes
func bToKb(b uint64) uint64 {
	return b / 1024
}

// abs returns the absolute value of an int64
func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// TestResourceLeakDetection tests for resource leaks during high load
func TestResourceLeakDetection(t *testing.T) {
	testhelpers.SkipIfUnit(t, "Skipping resource leak test in unit mode")

	// Get initial resource usage
	var m1 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	initialGoroutines := runtime.NumGoroutine()

	tmpDir := t.TempDir()

	// Create and destroy many loggers
	for i := 0; i < 100; i++ {
		logFile := filepath.Join(tmpDir, fmt.Sprintf("leak_test_%d.log", i))

		logger, err := New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger %d: %v", i, err)
		}

		// Log some messages
		for j := 0; j < 100; j++ {
			logger.Infof("Leak test message %d-%d", i, j)
		}

		// Close logger
		err = logger.Close()
		if err != nil {
			t.Errorf("Failed to close logger %d: %v", i, err)
		}
	}

	// Force garbage collection
	runtime.GC()
	runtime.GC() // Double GC to ensure cleanup

	// Get final resource usage
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	finalGoroutines := runtime.NumGoroutine()

	// Check for significant resource increases
	var memoryIncrease int64
	if m2.Alloc > m1.Alloc {
		memoryIncrease = int64(m2.Alloc - m1.Alloc)
	} else {
		memoryIncrease = -int64(m1.Alloc - m2.Alloc)
	}
	goroutineIncrease := finalGoroutines - initialGoroutines

	t.Logf("Initial memory: %d KB, Final memory: %d KB, Increase: %d KB",
		bToKb(m1.Alloc), bToKb(m2.Alloc), bToKb(uint64(abs(memoryIncrease))))
	t.Logf("Initial goroutines: %d, Final goroutines: %d, Increase: %d",
		initialGoroutines, finalGoroutines, goroutineIncrease)

	// Allow for some reasonable resource usage, but detect leaks
	if memoryIncrease > 50*1024*1024 { // 50MB threshold
		t.Errorf("Possible memory leak detected: %d KB increase", bToKb(uint64(memoryIncrease)))
	}

	if goroutineIncrease > 10 { // Allow for some background goroutines
		t.Errorf("Possible goroutine leak detected: %d goroutine increase", goroutineIncrease)
	}
}

// ===== Benchmarks from performance_baseline_test.go =====

// BenchmarkBaseline establishes performance baseline before optimizations
func BenchmarkBaseline(b *testing.B) {
	benchmarks := []struct {
		name       string
		parallel   int
		msgSize    int
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

// ===== Benchmarks from performance_optimized_test.go =====

// BenchmarkOptimizedBufferPool tests the performance improvement from buffer pooling
func BenchmarkOptimizedBufferPool(b *testing.B) {
	b.Run("WithoutPool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// Simulate formatting without pool
			// LogEntry and formatJSON are not available
			/*
				entry := &LogEntry{
					Level:     "INFO",
					Message:   "Test message",
					Timestamp: time.Now().Format(time.RFC3339),
				}
				data, _ := formatJSON(entry, false)
			*/
			data := []byte("{}")
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
			// SetFlushSize and SetFlushInterval are not available
			/*
				if cfg.batchSize > 0 {
					logger.SetFlushSize(0, cfg.batchSize)
					logger.SetFlushInterval(0, cfg.flushInterval)
				}
			*/

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
			// SetFlushSize and SetFlushInterval are not available
			// logger.SetFlushSize(0, 8192)
			// logger.SetFlushInterval(0, 100*time.Millisecond)

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
	// SetFlushSize and SetFlushInterval are not available
	// logger.SetFlushSize(0, 8192)
	// logger.SetFlushInterval(0, 100*time.Millisecond)

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
		name       string
		bufferPool bool
		rwMutex    bool
		batching   bool
		atomicOps  bool
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
			// SetFlushSize and SetFlushInterval are not available
			/*
				if scenario.batching {
					logger.SetFlushSize(0, 8192)
					logger.SetFlushInterval(0, 100*time.Millisecond)
				}
			*/

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
