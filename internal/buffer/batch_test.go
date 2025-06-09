package buffer

import (
	"bufio"
	"bytes"
	"errors"
	"sync"
	"testing"
	"time"
)

// mockWriter helps test error conditions
type mockWriter struct {
	buf       bytes.Buffer
	failWrite bool
	failFlush bool
	writeCalls int
	flushCalls int
	mu        sync.Mutex
}

func (m *mockWriter) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeCalls++
	if m.failWrite {
		return 0, errors.New("write failed")
	}
	return m.buf.Write(p)
}

func (m *mockWriter) Flush() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.flushCalls++
	if m.failFlush {
		return errors.New("flush failed")
	}
	return nil
}

func (m *mockWriter) String() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buf.String()
}

func TestNewBatchWriter(t *testing.T) {
	tests := []struct {
		name          string
		maxSize       int
		maxCount      int
		flushInterval time.Duration
		expectTimer   bool
	}{
		{
			name:          "with timer",
			maxSize:       1024,
			maxCount:      10,
			flushInterval: 100 * time.Millisecond,
			expectTimer:   true,
		},
		{
			name:          "without timer",
			maxSize:       1024,
			maxCount:      10,
			flushInterval: 0,
			expectTimer:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := bufio.NewWriter(&buf)
			bw := NewBatchWriter(writer, tt.maxSize, tt.maxCount, tt.flushInterval)

			if bw.writer != writer {
				t.Error("writer not set correctly")
			}
			if bw.maxSize != tt.maxSize {
				t.Errorf("maxSize = %d, want %d", bw.maxSize, tt.maxSize)
			}
			if bw.maxCount != tt.maxCount {
				t.Errorf("maxCount = %d, want %d", bw.maxCount, tt.maxCount)
			}
			if bw.flushInterval != tt.flushInterval {
				t.Errorf("flushInterval = %v, want %v", bw.flushInterval, tt.flushInterval)
			}
			if tt.expectTimer && bw.flushTimer == nil {
				t.Error("expected timer to be set")
			}
			if !tt.expectTimer && bw.flushTimer != nil {
				t.Error("expected timer to be nil")
			}

			// Cleanup
			bw.Close()
		})
	}
}

func TestBatchWriter_Write(t *testing.T) {
	tests := []struct {
		name      string
		writes    []string
		maxSize   int
		maxCount  int
		expectErr bool
	}{
		{
			name:     "single write below limits",
			writes:   []string{"hello world"},
			maxSize:  100,
			maxCount: 10,
		},
		{
			name:     "flush on size limit",
			writes:   []string{"hello", "world", "test"},
			maxSize:  10, // Will trigger after "hello" + "world"
			maxCount: 10,
		},
		{
			name:     "flush on count limit",
			writes:   []string{"a", "b", "c", "d"},
			maxSize:  100,
			maxCount: 3, // Will trigger after 3 writes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := bufio.NewWriter(&buf)
			bw := NewBatchWriter(writer, tt.maxSize, tt.maxCount, 0)
			defer bw.Close()

			for _, data := range tt.writes {
				n, err := bw.Write([]byte(data))
				if err != nil && !tt.expectErr {
					t.Errorf("unexpected error: %v", err)
				}
				if n != len(data) {
					t.Errorf("Write() returned %d, want %d", n, len(data))
				}
			}

			// Force final flush to check all data was written
			if err := bw.Flush(); err != nil {
				t.Errorf("Flush() error = %v", err)
			}

			// Verify all data was written
			expected := ""
			for _, s := range tt.writes {
				expected += s
			}
			if got := buf.String(); got != expected {
				t.Errorf("buffer content = %q, want %q", got, expected)
			}
		})
	}
}

func TestBatchWriter_WriteString(t *testing.T) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	bw := NewBatchWriter(writer, 1024, 10, 0)
	defer bw.Close()

	testStr := "test string"
	n, err := bw.WriteString(testStr)
	if err != nil {
		t.Errorf("WriteString() error = %v", err)
	}
	if n != len(testStr) {
		t.Errorf("WriteString() returned %d, want %d", n, len(testStr))
	}

	if err := bw.Flush(); err != nil {
		t.Errorf("Flush() error = %v", err)
	}

	if got := buf.String(); got != testStr {
		t.Errorf("buffer content = %q, want %q", got, testStr)
	}
}

func TestBatchWriter_WriteClosed(t *testing.T) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	bw := NewBatchWriter(writer, 1024, 10, 0)
	
	// Close the writer
	if err := bw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Try to write to closed writer
	_, err := bw.Write([]byte("test"))
	if err != ErrClosed {
		t.Errorf("Write() on closed writer error = %v, want %v", err, ErrClosed)
	}
}

func TestBatchWriter_FlushError(t *testing.T) {
	mock := &mockWriter{}
	// Use a small buffer size to ensure flush happens
	writer := bufio.NewWriterSize(mock, 1)
	bw := NewBatchWriter(writer, 10, 2, 0)
	defer bw.Close()

	// Write some data to force internal buffer operations
	bw.Write([]byte("test"))

	// Make write fail (since bufio.Writer doesn't expose flush errors directly)
	mock.failWrite = true
	
	err := bw.Flush()
	if err == nil {
		t.Error("expected write error during flush, got nil")
	}
}

func TestBatchWriter_WriteError(t *testing.T) {
	mock := &mockWriter{}
	writer := bufio.NewWriter(mock)
	bw := NewBatchWriter(writer, 5, 2, 0)
	defer bw.Close()

	// Write data to fill buffer
	bw.Write([]byte("test1"))
	
	// Make write fail on flush
	mock.failWrite = true
	
	// This should trigger a flush which will fail
	_, err := bw.Write([]byte("test2"))
	if err == nil {
		t.Error("expected write error during flush, got nil")
	}
}

func TestBatchWriter_TimedFlush(t *testing.T) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	bw := NewBatchWriter(writer, 1024, 10, 50*time.Millisecond)
	defer bw.Close()

	// Write data
	bw.Write([]byte("test"))

	// Wait for timer to trigger
	time.Sleep(100 * time.Millisecond)

	// Force flush to ensure all data is written and wait for completion
	if err := bw.Flush(); err != nil {
		t.Errorf("Flush() error = %v", err)
	}

	// Check data was flushed
	if got := buf.String(); got != "test" {
		t.Errorf("buffer content after timed flush = %q, want %q", got, "test")
	}
}

func TestBatchWriter_Stats(t *testing.T) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	bw := NewBatchWriter(writer, 1024, 10, 100*time.Millisecond)
	defer bw.Close()

	// Write some data
	bw.Write([]byte("hello"))
	bw.Write([]byte("world"))

	stats := bw.Stats()
	if stats.BufferedEntries != 2 {
		t.Errorf("BufferedEntries = %d, want 2", stats.BufferedEntries)
	}
	if stats.BufferedBytes != 10 { // "hello" + "world" = 10 bytes
		t.Errorf("BufferedBytes = %d, want 10", stats.BufferedBytes)
	}
	if stats.MaxEntries != 10 {
		t.Errorf("MaxEntries = %d, want 10", stats.MaxEntries)
	}
	if stats.MaxBytes != 1024 {
		t.Errorf("MaxBytes = %d, want 1024", stats.MaxBytes)
	}
	if stats.FlushInterval != 100*time.Millisecond {
		t.Errorf("FlushInterval = %v, want %v", stats.FlushInterval, 100*time.Millisecond)
	}
}

func TestBatchWriter_SetFlushInterval(t *testing.T) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	bw := NewBatchWriter(writer, 1024, 10, 100*time.Millisecond)
	defer bw.Close()

	// Change interval
	newInterval := 200 * time.Millisecond
	bw.SetFlushInterval(newInterval)

	stats := bw.Stats()
	if stats.FlushInterval != newInterval {
		t.Errorf("FlushInterval after update = %v, want %v", stats.FlushInterval, newInterval)
	}

	// Set to 0 (disable timer)
	bw.SetFlushInterval(0)
	stats = bw.Stats()
	if stats.FlushInterval != 0 {
		t.Errorf("FlushInterval after disable = %v, want 0", stats.FlushInterval)
	}
}

func TestBatchWriter_SetBatchSize(t *testing.T) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	bw := NewBatchWriter(writer, 1024, 10, 0)
	defer bw.Close()

	// Add data
	bw.Write([]byte("hello"))
	bw.Write([]byte("world"))

	// Reduce limits to trigger flush
	bw.SetBatchSize(5, 1)

	// Check data was flushed
	if got := buf.String(); got != "helloworld" {
		t.Errorf("buffer content after SetBatchSize = %q, want %q", got, "helloworld")
	}

	// Verify new limits
	stats := bw.Stats()
	if stats.MaxBytes != 5 {
		t.Errorf("MaxBytes = %d, want 5", stats.MaxBytes)
	}
	if stats.MaxEntries != 1 {
		t.Errorf("MaxEntries = %d, want 1", stats.MaxEntries)
	}
}

func TestBatchWriter_ConcurrentWrites(t *testing.T) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	bw := NewBatchWriter(writer, 1024, 100, 0)
	defer bw.Close()

	// Concurrent writes
	var wg sync.WaitGroup
	numGoroutines := 10
	writesPerGoroutine := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				data := []byte("x") // Single character for easy counting
				if _, err := bw.Write(data); err != nil {
					t.Errorf("goroutine %d: Write() error = %v", id, err)
				}
			}
		}(i)
	}

	wg.Wait()
	
	// Flush remaining data
	if err := bw.Flush(); err != nil {
		t.Errorf("Flush() error = %v", err)
	}

	// Verify all data was written
	expectedLen := numGoroutines * writesPerGoroutine
	if got := len(buf.String()); got != expectedLen {
		t.Errorf("total bytes written = %d, want %d", got, expectedLen)
	}
}

func TestBatchWriter_DoubleClose(t *testing.T) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	bw := NewBatchWriter(writer, 1024, 10, 0)

	// First close
	if err := bw.Close(); err != nil {
		t.Errorf("first Close() error = %v", err)
	}

	// Second close should not error
	if err := bw.Close(); err != nil {
		t.Errorf("second Close() error = %v", err)
	}
}

func TestBatchWriter_FlushEmpty(t *testing.T) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	bw := NewBatchWriter(writer, 1024, 10, 0)
	defer bw.Close()

	// Flush without any writes
	if err := bw.Flush(); err != nil {
		t.Errorf("Flush() on empty buffer error = %v", err)
	}
}

func TestBatchWriter_DataIntegrity(t *testing.T) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	bw := NewBatchWriter(writer, 50, 5, 0)
	defer bw.Close()

	// Write data that will cause multiple flushes
	testData := []string{
		"first line\n",
		"second line\n",
		"third line\n",
		"fourth line\n",
		"fifth line\n",
		"sixth line\n",
	}

	for _, data := range testData {
		if _, err := bw.Write([]byte(data)); err != nil {
			t.Errorf("Write(%q) error = %v", data, err)
		}
	}

	// Final flush
	if err := bw.Flush(); err != nil {
		t.Errorf("Flush() error = %v", err)
	}

	// Verify all data is present and in order
	expected := ""
	for _, s := range testData {
		expected += s
	}
	if got := buf.String(); got != expected {
		t.Errorf("buffer content = %q, want %q", got, expected)
	}
}