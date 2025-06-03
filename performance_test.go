package omni

import (
	"bytes"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestBufferPool(t *testing.T) {
	pool := NewBufferPool()

	// Test getting and putting buffers
	buf1 := pool.Get()
	buf1.WriteString("test data")

	// Return buffer to pool
	pool.Put(buf1)

	// Get another buffer - should be the same one, reset
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
	pool := NewBufferPool()

	// Create a large buffer that exceeds the 32KB limit
	buf := pool.Get()
	largeData := make([]byte, 33*1024) // Larger than 32KB limit
	buf.Write(largeData)

	// Put it back
	pool.Put(buf)

	// Get a new buffer - should not be the same one due to size limit
	buf2 := pool.Get()
	if buf2.Cap() > 1024 { // Default capacity is 512, so anything larger suggests reuse
		t.Errorf("Expected new buffer with default capacity, got %d", buf2.Cap())
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

func TestLazyMessage(t *testing.T) {
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
		pool := NewBufferPool()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := pool.Get()
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
