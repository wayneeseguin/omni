package utils

import (
	"sync/atomic"
)

// AtomicInt64 provides atomic operations for int64 values
type AtomicInt64 struct {
	value int64
}

// NewAtomicInt64 creates a new AtomicInt64 with the given initial value
func NewAtomicInt64(initial int64) *AtomicInt64 {
	return &AtomicInt64{value: initial}
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

// AtomicUint64 provides atomic operations for uint64 values
type AtomicUint64 struct {
	value uint64
}

// NewAtomicUint64 creates a new AtomicUint64 with the given initial value
func NewAtomicUint64(initial uint64) *AtomicUint64 {
	return &AtomicUint64{value: initial}
}

// Load atomically loads and returns the value
func (a *AtomicUint64) Load() uint64 {
	return atomic.LoadUint64(&a.value)
}

// Store atomically stores the value
func (a *AtomicUint64) Store(val uint64) {
	atomic.StoreUint64(&a.value, val)
}

// Add atomically adds delta to the value and returns the new value
func (a *AtomicUint64) Add(delta uint64) uint64 {
	return atomic.AddUint64(&a.value, delta)
}

// CompareAndSwap executes the compare-and-swap operation
func (a *AtomicUint64) CompareAndSwap(old, new uint64) bool {
	return atomic.CompareAndSwapUint64(&a.value, old, new)
}

// Swap atomically stores new and returns the previous value
func (a *AtomicUint64) Swap(new uint64) uint64 {
	return atomic.SwapUint64(&a.value, new)
}

// AtomicBool provides atomic operations for bool values
type AtomicBool struct {
	value int32
}

// NewAtomicBool creates a new AtomicBool with the given initial value
func NewAtomicBool(initial bool) *AtomicBool {
	var val int32
	if initial {
		val = 1
	}
	return &AtomicBool{value: val}
}

// Load atomically loads and returns the value
func (a *AtomicBool) Load() bool {
	return atomic.LoadInt32(&a.value) != 0
}

// Store atomically stores the value
func (a *AtomicBool) Store(val bool) {
	var v int32
	if val {
		v = 1
	}
	atomic.StoreInt32(&a.value, v)
}

// CompareAndSwap executes the compare-and-swap operation
func (a *AtomicBool) CompareAndSwap(old, new bool) bool {
	var oldVal, newVal int32
	if old {
		oldVal = 1
	}
	if new {
		newVal = 1
	}
	return atomic.CompareAndSwapInt32(&a.value, oldVal, newVal)
}

// Swap atomically stores new and returns the previous value
func (a *AtomicBool) Swap(new bool) bool {
	var newVal int32
	if new {
		newVal = 1
	}
	oldVal := atomic.SwapInt32(&a.value, newVal)
	return oldVal != 0
}
