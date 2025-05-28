package flexlog

import (
	"sync/atomic"
)

// AtomicInt64 provides atomic operations for int64 values
// This is a wrapper for compatibility with older Go versions
type AtomicInt64 struct {
	value int64
}

// Load atomically loads and returns the value
func (a *AtomicInt64) Load() int64 {
	return atomic.LoadInt64(&a.value)
}

// Store atomically stores the value
func (a *AtomicInt64) Store(val int64) {
	atomic.StoreInt64(&a.value, val)
}

// Add atomically adds delta to the value and returns the new value
func (a *AtomicInt64) Add(delta int64) int64 {
	return atomic.AddInt64(&a.value, delta)
}

// CompareAndSwap executes the compare-and-swap operation
func (a *AtomicInt64) CompareAndSwap(old, new int64) bool {
	return atomic.CompareAndSwapInt64(&a.value, old, new)
}

// Swap atomically stores new and returns the previous value
func (a *AtomicInt64) Swap(new int64) int64 {
	return atomic.SwapInt64(&a.value, new)
}