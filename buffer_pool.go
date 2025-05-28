package flexlog

import (
	"bytes"
	"strings"
	"sync"
)

// BufferPool manages a pool of reusable byte buffers to reduce allocations
// during log message formatting and writing operations.
type BufferPool struct {
	pool     sync.Pool
	capacity int
}

// NewBufferPool creates a new buffer pool with a default buffer size.
// The pool automatically grows and shrinks based on usage patterns.
func NewBufferPool() *BufferPool {
	return NewBufferPoolWithCapacity(512)
}

// NewBufferPoolWithCapacity creates a buffer pool with a specific initial capacity
func NewBufferPoolWithCapacity(capacity int) *BufferPool {
	bp := &BufferPool{
		capacity: capacity,
	}
	bp.pool = sync.Pool{
		New: func() interface{} {
			// Create buffers with the specified initial capacity
			// to minimize grow operations
			return bytes.NewBuffer(make([]byte, 0, capacity))
		},
	}
	return bp
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

// StringBuilderPool manages a pool of reusable string builders
type StringBuilderPool struct {
	pool sync.Pool
}

// NewStringBuilderPool creates a new string builder pool
func NewStringBuilderPool() *StringBuilderPool {
	return &StringBuilderPool{
		pool: sync.Pool{
			New: func() interface{} {
				var sb strings.Builder
				sb.Grow(256) // Pre-allocate reasonable capacity
				return &sb
			},
		},
	}
}

// Get retrieves a string builder from the pool
func (sbp *StringBuilderPool) Get() *strings.Builder {
	sb := sbp.pool.Get().(*strings.Builder)
	sb.Reset()
	return sb
}

// Put returns a string builder to the pool
func (sbp *StringBuilderPool) Put(sb *strings.Builder) {
	if sb == nil {
		return
	}

	// Don't pool extremely large builders
	if sb.Cap() > 32*1024 {
		return
	}

	sb.Reset()
	sbp.pool.Put(sb)
}

// Global buffer pools for different use cases
var (
	// Small buffers for short messages (timestamps, levels, etc.)
	smallBufferPool = NewBufferPoolWithCapacity(128)

	// Medium buffers for typical log messages
	mediumBufferPool = NewBufferPoolWithCapacity(512)

	// Large buffers for complex structured logging
	largeBufferPool = NewBufferPoolWithCapacity(2048)

	// String builder pool for efficient string construction
	stringBuilderPool = NewStringBuilderPool()
)

// GetBuffer retrieves a buffer from the appropriate global pool based on size hint
func GetBuffer() *bytes.Buffer {
	return mediumBufferPool.Get()
}

// GetSmallBuffer retrieves a small buffer optimized for short content
func GetSmallBuffer() *bytes.Buffer {
	return smallBufferPool.Get()
}

// GetLargeBuffer retrieves a large buffer optimized for complex content
func GetLargeBuffer() *bytes.Buffer {
	return largeBufferPool.Get()
}

// GetStringBuilder retrieves a string builder from the global pool
func GetStringBuilder() *strings.Builder {
	return stringBuilderPool.Get()
}

// PutBuffer returns a buffer to the appropriate global pool
func PutBuffer(buf *bytes.Buffer) {
	if buf == nil {
		return
	}

	// Route to appropriate pool based on capacity
	cap := buf.Cap()
	switch {
	case cap <= 256:
		smallBufferPool.Put(buf)
	case cap <= 1024:
		mediumBufferPool.Put(buf)
	default:
		largeBufferPool.Put(buf)
	}
}

// PutStringBuilder returns a string builder to the global pool
func PutStringBuilder(sb *strings.Builder) {
	stringBuilderPool.Put(sb)
}
