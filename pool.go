package flexlog

import (
	"bytes"
	"sync"
)

// BufferPool manages a pool of reusable byte buffers to reduce GC pressure
type BufferPool struct {
	pool sync.Pool
}

// NewBufferPool creates a new buffer pool
func NewBufferPool() *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				// Create new buffer with reasonable initial capacity
				return bytes.NewBuffer(make([]byte, 0, 512))
			},
		},
	}
}

// Get retrieves a buffer from the pool
func (bp *BufferPool) Get() *bytes.Buffer {
	return bp.pool.Get().(*bytes.Buffer)
}

// Put returns a buffer to the pool after resetting it
func (bp *BufferPool) Put(buf *bytes.Buffer) {
	// Reset buffer for reuse
	buf.Reset()
	
	// Only put back reasonably sized buffers
	// Overly large buffers should be GC'd
	if buf.Cap() <= 4096 {
		bp.pool.Put(buf)
	}
}

// Global buffer pool for the package
var bufferPool = NewBufferPool()

// GetBuffer gets a buffer from the global pool
func GetBuffer() *bytes.Buffer {
	return bufferPool.Get()
}

// PutBuffer returns a buffer to the global pool
func PutBuffer(buf *bytes.Buffer) {
	bufferPool.Put(buf)
}