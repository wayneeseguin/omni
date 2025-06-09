package utils

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestAtomicInt64_BasicOperations(t *testing.T) {
	a := NewAtomicInt64(10)

	// Test Load
	if got := a.Load(); got != 10 {
		t.Errorf("Load() = %d, want 10", got)
	}

	// Test Store
	a.Store(20)
	if got := a.Load(); got != 20 {
		t.Errorf("After Store(20), Load() = %d, want 20", got)
	}

	// Test Add
	result := a.Add(5)
	if result != 25 {
		t.Errorf("Add(5) = %d, want 25", result)
	}
	if got := a.Load(); got != 25 {
		t.Errorf("After Add(5), Load() = %d, want 25", got)
	}

	// Test negative Add
	result = a.Add(-10)
	if result != 15 {
		t.Errorf("Add(-10) = %d, want 15", result)
	}

	// Test CompareAndSwap success
	if !a.CompareAndSwap(15, 30) {
		t.Error("CompareAndSwap(15, 30) = false, want true")
	}
	if got := a.Load(); got != 30 {
		t.Errorf("After CompareAndSwap(15, 30), Load() = %d, want 30", got)
	}

	// Test CompareAndSwap failure
	if a.CompareAndSwap(15, 40) {
		t.Error("CompareAndSwap(15, 40) = true, want false")
	}
	if got := a.Load(); got != 30 {
		t.Errorf("After failed CompareAndSwap, Load() = %d, want 30", got)
	}

	// Test Swap
	oldValue := a.Swap(50)
	if oldValue != 30 {
		t.Errorf("Swap(50) = %d, want 30", oldValue)
	}
	if got := a.Load(); got != 50 {
		t.Errorf("After Swap(50), Load() = %d, want 50", got)
	}
}

func TestAtomicInt64_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		initial  int64
		expected int64
	}{
		{"Zero", 0, 0},
		{"Negative", -100, -100},
		{"MaxInt64", 9223372036854775807, 9223372036854775807},
		{"MinInt64", -9223372036854775808, -9223372036854775808},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewAtomicInt64(tt.initial)
			if got := a.Load(); got != tt.expected {
				t.Errorf("NewAtomicInt64(%d).Load() = %d, want %d", tt.initial, got, tt.expected)
			}
		})
	}
}

func TestAtomicInt64_ConcurrentAccess(t *testing.T) {
	const numGoroutines = 100
	const numOperations = 1000

	a := NewAtomicInt64(0)
	var wg sync.WaitGroup

	// Concurrent adds
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				a.Add(1)
			}
		}()
	}
	wg.Wait()

	expected := int64(numGoroutines * numOperations)
	if got := a.Load(); got != expected {
		t.Errorf("After concurrent adds, Load() = %d, want %d", got, expected)
	}

	// Concurrent compare-and-swap operations
	a.Store(0)
	successCounter := NewAtomicInt64(0)

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(val int64) {
			defer wg.Done()
			if a.CompareAndSwap(0, val) {
				successCounter.Add(1)
			}
		}(int64(i + 1))
	}
	wg.Wait()

	// Only one CAS should succeed
	if got := successCounter.Load(); got != 1 {
		t.Errorf("Expected exactly 1 successful CAS, got %d", got)
	}

	// Value should be one of the goroutine IDs (1 to numGoroutines)
	finalValue := a.Load()
	if finalValue < 1 || finalValue > numGoroutines {
		t.Errorf("Final value %d should be between 1 and %d", finalValue, numGoroutines)
	}
}

func TestAtomicInt64_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance test in short mode")
	}

	a := NewAtomicInt64(0)
	const iterations = 10000000

	start := time.Now()
	for i := 0; i < iterations; i++ {
		a.Add(1)
	}
	duration := time.Since(start)

	t.Logf("Performed %d atomic adds in %v (%.2f ns/op)", iterations, duration, float64(duration.Nanoseconds())/float64(iterations))

	if got := a.Load(); got != iterations {
		t.Errorf("Load() = %d, want %d", got, iterations)
	}
}

func TestAtomicUint64_BasicOperations(t *testing.T) {
	a := NewAtomicUint64(10)

	// Test Load
	if got := a.Load(); got != 10 {
		t.Errorf("Load() = %d, want 10", got)
	}

	// Test Store
	a.Store(20)
	if got := a.Load(); got != 20 {
		t.Errorf("After Store(20), Load() = %d, want 20", got)
	}

	// Test Add
	result := a.Add(5)
	if result != 25 {
		t.Errorf("Add(5) = %d, want 25", result)
	}
	if got := a.Load(); got != 25 {
		t.Errorf("After Add(5), Load() = %d, want 25", got)
	}

	// Test CompareAndSwap success
	if !a.CompareAndSwap(25, 30) {
		t.Error("CompareAndSwap(25, 30) = false, want true")
	}
	if got := a.Load(); got != 30 {
		t.Errorf("After CompareAndSwap(25, 30), Load() = %d, want 30", got)
	}

	// Test CompareAndSwap failure
	if a.CompareAndSwap(25, 40) {
		t.Error("CompareAndSwap(25, 40) = true, want false")
	}
	if got := a.Load(); got != 30 {
		t.Errorf("After failed CompareAndSwap, Load() = %d, want 30", got)
	}

	// Test Swap
	oldValue := a.Swap(50)
	if oldValue != 30 {
		t.Errorf("Swap(50) = %d, want 30", oldValue)
	}
	if got := a.Load(); got != 50 {
		t.Errorf("After Swap(50), Load() = %d, want 50", got)
	}
}

func TestAtomicUint64_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		initial  uint64
		expected uint64
	}{
		{"Zero", 0, 0},
		{"MaxUint64", 18446744073709551615, 18446744073709551615},
		{"LargeValue", 1000000000000000000, 1000000000000000000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewAtomicUint64(tt.initial)
			if got := a.Load(); got != tt.expected {
				t.Errorf("NewAtomicUint64(%d).Load() = %d, want %d", tt.initial, got, tt.expected)
			}
		})
	}
}

func TestAtomicUint64_ConcurrentAccess(t *testing.T) {
	const numGoroutines = 50
	const numOperations = 1000

	a := NewAtomicUint64(0)
	var wg sync.WaitGroup

	// Concurrent adds
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				a.Add(1)
			}
		}()
	}
	wg.Wait()

	expected := uint64(numGoroutines * numOperations)
	if got := a.Load(); got != expected {
		t.Errorf("After concurrent adds, Load() = %d, want %d", got, expected)
	}
}

func TestAtomicBool_BasicOperations(t *testing.T) {
	// Test with initial false
	a := NewAtomicBool(false)
	if got := a.Load(); got != false {
		t.Errorf("Load() = %t, want false", got)
	}

	// Test Store true
	a.Store(true)
	if got := a.Load(); got != true {
		t.Errorf("After Store(true), Load() = %t, want true", got)
	}

	// Test Store false
	a.Store(false)
	if got := a.Load(); got != false {
		t.Errorf("After Store(false), Load() = %t, want false", got)
	}

	// Test CompareAndSwap success
	if !a.CompareAndSwap(false, true) {
		t.Error("CompareAndSwap(false, true) = false, want true")
	}
	if got := a.Load(); got != true {
		t.Errorf("After CompareAndSwap(false, true), Load() = %t, want true", got)
	}

	// Test CompareAndSwap failure
	if a.CompareAndSwap(false, true) {
		t.Error("CompareAndSwap(false, true) = true, want false")
	}
	if got := a.Load(); got != true {
		t.Errorf("After failed CompareAndSwap, Load() = %t, want true", got)
	}

	// Test Swap
	oldValue := a.Swap(false)
	if oldValue != true {
		t.Errorf("Swap(false) = %t, want true", oldValue)
	}
	if got := a.Load(); got != false {
		t.Errorf("After Swap(false), Load() = %t, want false", got)
	}

	// Test with initial true
	b := NewAtomicBool(true)
	if got := b.Load(); got != true {
		t.Errorf("NewAtomicBool(true).Load() = %t, want true", got)
	}
}

func TestAtomicBool_ConcurrentAccess(t *testing.T) {
	const numGoroutines = 100

	a := NewAtomicBool(false)
	var wg sync.WaitGroup

	// Test concurrent toggle operations
	toggleCount := NewAtomicInt64(0)
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for {
				current := a.Load()
				if a.CompareAndSwap(current, !current) {
					toggleCount.Add(1)
					break
				}
				// Yield to other goroutines if CAS fails
				runtime.Gosched()
			}
		}()
	}
	wg.Wait()

	// All goroutines should have successfully toggled
	if got := toggleCount.Load(); got != numGoroutines {
		t.Errorf("Expected %d successful toggles, got %d", numGoroutines, got)
	}

	// Final value should reflect the number of toggles
	expectedFinal := numGoroutines%2 == 1 // odd number of toggles means true
	if got := a.Load(); got != expectedFinal {
		t.Errorf("After %d toggles, Load() = %t, want %t", numGoroutines, got, expectedFinal)
	}
}

func TestAtomicBool_CompareAndSwapEdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		initial    bool
		old        bool
		new        bool
		shouldSwap bool
		expected   bool
	}{
		{"false->true success", false, false, true, true, true},
		{"true->false success", true, true, false, true, false},
		{"false->true failure", false, true, true, false, false},
		{"true->false failure", true, false, false, false, true},
		{"same value success", true, true, true, true, true},
		{"same value failure", false, true, false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewAtomicBool(tt.initial)
			result := a.CompareAndSwap(tt.old, tt.new)
			if result != tt.shouldSwap {
				t.Errorf("CompareAndSwap(%t, %t) = %t, want %t", tt.old, tt.new, result, tt.shouldSwap)
			}
			if got := a.Load(); got != tt.expected {
				t.Errorf("After CompareAndSwap, Load() = %t, want %t", got, tt.expected)
			}
		})
	}
}

func TestAtomicBool_SwapOperations(t *testing.T) {
	tests := []struct {
		name     string
		initial  bool
		newValue bool
	}{
		{"false->true", false, true},
		{"true->false", true, false},
		{"false->false", false, false},
		{"true->true", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewAtomicBool(tt.initial)
			oldValue := a.Swap(tt.newValue)
			if oldValue != tt.initial {
				t.Errorf("Swap(%t) = %t, want %t", tt.newValue, oldValue, tt.initial)
			}
			if got := a.Load(); got != tt.newValue {
				t.Errorf("After Swap(%t), Load() = %t, want %t", tt.newValue, got, tt.newValue)
			}
		})
	}
}

// Benchmark tests for performance characteristics
func BenchmarkAtomicInt64_Load(b *testing.B) {
	a := NewAtomicInt64(42)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Load()
	}
}

func BenchmarkAtomicInt64_Store(b *testing.B) {
	a := NewAtomicInt64(0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Store(int64(i))
	}
}

func BenchmarkAtomicInt64_Add(b *testing.B) {
	a := NewAtomicInt64(0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Add(1)
	}
}

func BenchmarkAtomicInt64_CompareAndSwap(b *testing.B) {
	a := NewAtomicInt64(0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		current := a.Load()
		a.CompareAndSwap(current, current+1)
	}
}

func BenchmarkAtomicInt64_ConcurrentAdd(b *testing.B) {
	a := NewAtomicInt64(0)
	var wg sync.WaitGroup
	numGoroutines := runtime.NumCPU()
	
	b.ResetTimer()
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < b.N/numGoroutines; j++ {
				a.Add(1)
			}
		}()
	}
	wg.Wait()
}

func BenchmarkAtomicBool_Load(b *testing.B) {
	a := NewAtomicBool(true)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Load()
	}
}

func BenchmarkAtomicBool_Store(b *testing.B) {
	a := NewAtomicBool(false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Store(i%2 == 0)
	}
}

func BenchmarkAtomicBool_CompareAndSwap(b *testing.B) {
	a := NewAtomicBool(false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		current := a.Load()
		a.CompareAndSwap(current, !current)
	}
}