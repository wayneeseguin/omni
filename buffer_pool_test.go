package flexlog

import (
	"bytes"
	"sync"
	"testing"
)

func TestBufferPoolOperations(t *testing.T) {
	t.Run("GetAndPut", func(t *testing.T) {
		buf := GetBuffer()
		if buf == nil {
			t.Fatal("GetBuffer returned nil")
		}

		// Write some data
		testData := "test data"
		buf.WriteString(testData)

		// Put it back
		PutBuffer(buf)

		// Get another buffer - should be reset
		buf2 := GetBuffer()
		if buf2.Len() != 0 {
			t.Errorf("Buffer not reset after put, length: %d", buf2.Len())
		}
		PutBuffer(buf2)
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		var wg sync.WaitGroup
		iterations := 100

		for i := 0; i < iterations; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				buf := GetBuffer()
				buf.WriteString("test")
				PutBuffer(buf)
			}(i)
		}

		wg.Wait()
	})

	t.Run("BufferReuse", func(t *testing.T) {
		// Get and put a buffer
		buf1 := GetBuffer()
		buf1.WriteString("data")
		PutBuffer(buf1)

		// Get another buffer - it might be the same one
		buf2 := GetBuffer()
		if buf2.Len() != 0 {
			t.Error("Reused buffer not reset")
		}
		PutBuffer(buf2)
	})

	t.Run("LargeBuffer", func(t *testing.T) {
		buf := GetBuffer()
		defer PutBuffer(buf)

		// Write a large amount of data
		largeData := bytes.Repeat([]byte("x"), 10000)
		buf.Write(largeData)

		if buf.Len() != 10000 {
			t.Errorf("Expected buffer length 10000, got %d", buf.Len())
		}
	})
}

func BenchmarkBufferPoolOperations(b *testing.B) {
	b.Run("GetPut", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			buf := GetBuffer()
			buf.WriteString("benchmark test data")
			PutBuffer(buf)
		}
	})

	b.Run("NoPool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			buf := &bytes.Buffer{}
			buf.WriteString("benchmark test data")
		}
	})
}
