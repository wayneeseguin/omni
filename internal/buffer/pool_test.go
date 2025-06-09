package buffer

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

func TestNewBufferPool(t *testing.T) {
	pool := NewBufferPool()
	if pool == nil {
		t.Fatal("NewBufferPool() returned nil")
	}
	if pool.capacity != 512 {
		t.Errorf("default capacity = %d, want 512", pool.capacity)
	}
}

func TestNewBufferPoolWithCapacity(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
	}{
		{"small", 64},
		{"medium", 256},
		{"large", 1024},
		{"very large", 4096},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := NewBufferPoolWithCapacity(tt.capacity)
			if pool == nil {
				t.Fatal("NewBufferPoolWithCapacity() returned nil")
			}
			if pool.capacity != tt.capacity {
				t.Errorf("capacity = %d, want %d", pool.capacity, tt.capacity)
			}

			// Test that buffers from pool have correct capacity
			buf := pool.Get()
			if buf.Cap() != tt.capacity {
				t.Errorf("buffer capacity = %d, want %d", buf.Cap(), tt.capacity)
			}
			pool.Put(buf)
		})
	}
}

func TestBufferPool_GetPut(t *testing.T) {
	pool := NewBufferPoolWithCapacity(256)

	// Get buffer
	buf := pool.Get()
	if buf == nil {
		t.Fatal("Get() returned nil")
	}
	if buf.Len() != 0 {
		t.Errorf("buffer length = %d, want 0", buf.Len())
	}
	if buf.Cap() != 256 {
		t.Errorf("buffer capacity = %d, want 256", buf.Cap())
	}

	// Write to buffer
	testData := "test data"
	buf.WriteString(testData)
	if buf.String() != testData {
		t.Errorf("buffer content = %q, want %q", buf.String(), testData)
	}

	// Put buffer back
	pool.Put(buf)

	// Get again - should be reset
	buf2 := pool.Get()
	if buf2.Len() != 0 {
		t.Errorf("recycled buffer length = %d, want 0", buf2.Len())
	}
	if buf2.String() != "" {
		t.Errorf("recycled buffer content = %q, want empty", buf2.String())
	}
}

func TestBufferPool_LargeBufferNotPooled(t *testing.T) {
	pool := NewBufferPool()

	// Create a large buffer
	buf := pool.Get()
	largeData := make([]byte, 33*1024) // 33KB
	buf.Write(largeData)

	// Put it back
	pool.Put(buf)

	// Get new buffer - should not be the large one
	buf2 := pool.Get()
	if buf2.Cap() > 32*1024 {
		t.Errorf("got large buffer from pool, capacity = %d", buf2.Cap())
	}
}

func TestBufferPool_PutNil(t *testing.T) {
	pool := NewBufferPool()
	// Should not panic
	pool.Put(nil)
}

func TestBufferPool_Concurrent(t *testing.T) {
	pool := NewBufferPoolWithCapacity(128)
	var wg sync.WaitGroup
	numGoroutines := 100
	iterations := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				buf := pool.Get()
				if buf == nil {
					t.Errorf("goroutine %d: Get() returned nil", id)
					return
				}

				// Use the buffer
				buf.WriteString("test")
				if buf.Len() < 4 {
					t.Errorf("goroutine %d: buffer write failed", id)
				}

				// Return to pool
				pool.Put(buf)
			}
		}(i)
	}

	wg.Wait()
}

func TestNewStringBuilderPool(t *testing.T) {
	pool := NewStringBuilderPool()
	if pool == nil {
		t.Fatal("NewStringBuilderPool() returned nil")
	}
}

func TestStringBuilderPool_GetPut(t *testing.T) {
	pool := NewStringBuilderPool()

	// Get builder
	sb := pool.Get()
	if sb == nil {
		t.Fatal("Get() returned nil")
	}
	if sb.Len() != 0 {
		t.Errorf("builder length = %d, want 0", sb.Len())
	}

	// Write to builder
	testData := "test string data"
	sb.WriteString(testData)
	if sb.String() != testData {
		t.Errorf("builder content = %q, want %q", sb.String(), testData)
	}

	// Put builder back
	pool.Put(sb)

	// Get again - should be reset
	sb2 := pool.Get()
	if sb2.Len() != 0 {
		t.Errorf("recycled builder length = %d, want 0", sb2.Len())
	}
	if sb2.String() != "" {
		t.Errorf("recycled builder content = %q, want empty", sb2.String())
	}
}

func TestStringBuilderPool_LargeBuilderNotPooled(t *testing.T) {
	pool := NewStringBuilderPool()

	// Create a large builder
	sb := pool.Get()
	largeString := strings.Repeat("x", 33*1024) // 33KB
	sb.WriteString(largeString)

	// Put it back
	pool.Put(sb)

	// Get new builder - should have reasonable capacity
	sb2 := pool.Get()
	if sb2.Cap() > 32*1024 {
		t.Errorf("got large builder from pool, capacity = %d", sb2.Cap())
	}
}

func TestStringBuilderPool_PutNil(t *testing.T) {
	pool := NewStringBuilderPool()
	// Should not panic
	pool.Put(nil)
}

func TestStringBuilderPool_Concurrent(t *testing.T) {
	pool := NewStringBuilderPool()
	var wg sync.WaitGroup
	numGoroutines := 100
	iterations := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				sb := pool.Get()
				if sb == nil {
					t.Errorf("goroutine %d: Get() returned nil", id)
					return
				}

				// Use the builder
				sb.WriteString("test")
				if sb.Len() < 4 {
					t.Errorf("goroutine %d: builder write failed", id)
				}

				// Return to pool
				pool.Put(sb)
			}
		}(i)
	}

	wg.Wait()
}

func TestGlobalBufferPools(t *testing.T) {
	tests := []struct {
		name    string
		getFunc func() *bytes.Buffer
		putFunc func(*bytes.Buffer)
		minCap  int
		maxCap  int
	}{
		{
			name:    "small buffer",
			getFunc: GetSmallBuffer,
			putFunc: PutBuffer,
			minCap:  128,
			maxCap:  256,
		},
		{
			name:    "medium buffer",
			getFunc: GetBuffer,
			putFunc: PutBuffer,
			minCap:  512,
			maxCap:  1024,
		},
		{
			name:    "large buffer",
			getFunc: GetLargeBuffer,
			putFunc: PutBuffer,
			minCap:  2048,
			maxCap:  4096,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := tt.getFunc()
			if buf == nil {
				t.Fatal("getFunc() returned nil")
			}
			if buf.Cap() < tt.minCap {
				t.Errorf("buffer capacity = %d, want at least %d", buf.Cap(), tt.minCap)
			}

			// Use buffer
			buf.WriteString("test data")

			// Return to pool
			tt.putFunc(buf)
		})
	}
}

func TestGlobalStringBuilderPool(t *testing.T) {
	sb := GetStringBuilder()
	if sb == nil {
		t.Fatal("GetStringBuilder() returned nil")
	}
	if sb.Len() != 0 {
		t.Errorf("builder length = %d, want 0", sb.Len())
	}

	// Use builder
	sb.WriteString("test string")

	// Return to pool
	PutStringBuilder(sb)
}

func TestPutBuffer_Routing(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		pool     string
	}{
		{"small", 128, "small"},
		{"small-edge", 256, "small"},
		{"medium", 512, "medium"},
		{"medium-edge", 1024, "medium"},
		{"large", 2048, "large"},
		{"extra-large", 4096, "large"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a buffer with specific capacity
			buf := bytes.NewBuffer(make([]byte, 0, tt.capacity))

			// Put it back
			PutBuffer(buf)

			// Get from appropriate pool and verify routing worked
			var newBuf *bytes.Buffer
			switch tt.pool {
			case "small":
				newBuf = GetSmallBuffer()
			case "medium":
				newBuf = GetBuffer()
			case "large":
				newBuf = GetLargeBuffer()
			}

			// Can't directly verify it's the same buffer, but we can
			// check that the pool is working
			if newBuf == nil {
				t.Error("failed to get buffer from pool")
			}
		})
	}
}

func TestPutBuffer_Nil(t *testing.T) {
	// Should not panic
	PutBuffer(nil)
}

func TestPutStringBuilder_Nil(t *testing.T) {
	// Should not panic
	PutStringBuilder(nil)
}

// Benchmark tests
func BenchmarkBufferPool_GetPut(b *testing.B) {
	pool := NewBufferPool()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf := pool.Get()
		buf.WriteString("benchmark test data")
		pool.Put(buf)
	}
}

func BenchmarkBufferPool_Parallel(b *testing.B) {
	pool := NewBufferPool()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Get()
			buf.WriteString("benchmark test data")
			pool.Put(buf)
		}
	})
}

func BenchmarkStringBuilderPool_GetPut(b *testing.B) {
	pool := NewStringBuilderPool()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sb := pool.Get()
		sb.WriteString("benchmark test data")
		pool.Put(sb)
	}
}

func BenchmarkNoPool_Buffer(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf := bytes.NewBuffer(make([]byte, 0, 512))
		buf.WriteString("benchmark test data")
		// buf goes out of scope and is garbage collected
	}
}

func BenchmarkNoPool_StringBuilder(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var sb strings.Builder
		sb.Grow(256)
		sb.WriteString("benchmark test data")
		// sb goes out of scope and is garbage collected
	}
}
