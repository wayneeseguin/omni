package flexlog

import (
	"bytes"
	"strings"
	"sync"
)

// BufferPool manages a pool of reusable byte buffers to reduce allocations
// during log message formatting and writing operations.
// This significantly improves performance by reducing garbage collection pressure.
type BufferPool struct {
	pool     sync.Pool
	capacity int
}

// NewBufferPool creates a new buffer pool with a default buffer size.
// The pool automatically grows and shrinks based on usage patterns.
// The default capacity is 512 bytes, suitable for most log messages.
//
// Returns:
//   - *BufferPool: A new buffer pool instance
func NewBufferPool() *BufferPool {
	return NewBufferPoolWithCapacity(512)
}

// NewBufferPoolWithCapacity creates a buffer pool with a specific initial capacity.
// Use this when you know the typical size of your log messages.
//
// Parameters:
//   - capacity: Initial buffer capacity in bytes
//
// Returns:
//   - *BufferPool: A new buffer pool with the specified capacity
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
// The buffer is automatically reset before being returned.
//
// Returns:
//   - *bytes.Buffer: A clean buffer ready for use
func (bp *BufferPool) Get() *bytes.Buffer {
	v := bp.pool.Get()
	if v == nil {
		// This should never happen with our New function, but be defensive
		return &bytes.Buffer{}
	}
	buf, ok := v.(*bytes.Buffer)
	if !ok {
		// This should never happen, but be defensive
		return &bytes.Buffer{}
	}
	buf.Reset() // Ensure the buffer is clean
	return buf
}

// Put returns a buffer to the pool for reuse.
// The buffer is reset before being pooled to prevent data leaks.
// Extremely large buffers (>32KB) are not pooled to prevent memory bloat.
//
// Parameters:
//   - buf: The buffer to return to the pool
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

// StringBuilderPool manages a pool of reusable string builders.
// String builders are more efficient than buffers for string concatenation.
type StringBuilderPool struct {
	pool sync.Pool
}

// NewStringBuilderPool creates a new string builder pool.
// Builders are pre-allocated with 256 bytes capacity.
//
// Returns:
//   - *StringBuilderPool: A new string builder pool
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

// Get retrieves a string builder from the pool.
// The builder is automatically reset before being returned.
//
// Returns:
//   - *strings.Builder: A clean string builder ready for use
func (sbp *StringBuilderPool) Get() *strings.Builder {
	sb := sbp.pool.Get().(*strings.Builder)
	sb.Reset()
	return sb
}

// Put returns a string builder to the pool.
// Large builders (>32KB) are not pooled to prevent memory bloat.
//
// Parameters:
//   - sb: The string builder to return to the pool
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

// Global buffer pools for different use cases.
// These are shared across all logger instances to maximize reuse.
var (
	// smallBufferPool holds buffers for short messages (timestamps, levels, etc.)
	smallBufferPool = NewBufferPoolWithCapacity(128)

	// mediumBufferPool holds buffers for typical log messages
	mediumBufferPool = NewBufferPoolWithCapacity(512)

	// largeBufferPool holds buffers for complex structured logging
	largeBufferPool = NewBufferPoolWithCapacity(2048)

	// stringBuilderPool holds string builders for efficient string construction
	stringBuilderPool = NewStringBuilderPool()
)

// GetBuffer retrieves a buffer from the appropriate global pool based on size hint.
// This is the default function to use when you need a general-purpose buffer.
//
// Returns:
//   - *bytes.Buffer: A medium-sized buffer (512 bytes capacity)
func GetBuffer() *bytes.Buffer {
	return mediumBufferPool.Get()
}

// GetSmallBuffer retrieves a small buffer optimized for short content.
// Use this for formatting timestamps, log levels, and other small strings.
//
// Returns:
//   - *bytes.Buffer: A small buffer (128 bytes capacity)
func GetSmallBuffer() *bytes.Buffer {
	return smallBufferPool.Get()
}

// GetLargeBuffer retrieves a large buffer optimized for complex content.
// Use this for structured logging with many fields or large messages.
//
// Returns:
//   - *bytes.Buffer: A large buffer (2048 bytes capacity)
func GetLargeBuffer() *bytes.Buffer {
	return largeBufferPool.Get()
}

// GetStringBuilder retrieves a string builder from the global pool.
// Use this for efficient string concatenation operations.
//
// Returns:
//   - *strings.Builder: A string builder with 256 bytes initial capacity
func GetStringBuilder() *strings.Builder {
	return stringBuilderPool.Get()
}

// PutBuffer returns a buffer to the appropriate global pool.
// The pool is automatically selected based on the buffer's capacity.
// Always call this when done with a buffer obtained from GetBuffer.
//
// Parameters:
//   - buf: The buffer to return (can be nil)
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

// PutStringBuilder returns a string builder to the global pool.
// Always call this when done with a builder obtained from GetStringBuilder.
//
// Parameters:
//   - sb: The string builder to return (can be nil)
func PutStringBuilder(sb *strings.Builder) {
	stringBuilderPool.Put(sb)
}
