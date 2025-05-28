package flexlog

import (
	"bytes"
	"sync"
)

// BufferPool manages a pool of reusable byte buffers to reduce allocations
// during log message formatting and writing operations.
type BufferPool struct {
	pool sync.Pool
}

// NewBufferPool creates a new buffer pool with a default buffer size.
// The pool automatically grows and shrinks based on usage patterns.
func NewBufferPool() *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				// Create buffers with a reasonable initial capacity
				// to minimize grow operations
				return bytes.NewBuffer(make([]byte, 0, 512))
			},
		},
	}
}

// Get retrieves a buffer from the pool or creates a new one if the pool is empty.
// The caller is responsible for returning the buffer to the pool using Put().
func (bp *BufferPool) Get() *bytes.Buffer {
	buf := bp.pool.Get().(*bytes.Buffer)
	buf.Reset() // Ensure the buffer is clean
	return buf
}

// Put returns a buffer to the pool for reuse.
// The buffer is reset before being pooled to prevent data leaks.
func (bp *BufferPool) Put(buf *bytes.Buffer) {
	if buf == nil {
		return
	}

	// Don't pool extremely large buffers to prevent memory bloat
	// Buffers larger than 32KB are discarded
	if buf.Cap() > 32*1024 {
		return
	}

	buf.Reset()
	bp.pool.Put(buf)
}

// Global buffer pool instance for the package
var globalBufferPool = NewBufferPool()

// GetBuffer retrieves a buffer from the global pool
func GetBuffer() *bytes.Buffer {
	return globalBufferPool.Get()
}

// PutBuffer returns a buffer to the global pool
func PutBuffer(buf *bytes.Buffer) {
	globalBufferPool.Put(buf)
}
