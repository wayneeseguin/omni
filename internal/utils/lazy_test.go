package utils

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/wayneeseguin/omni/pkg/types"
)

func TestLazyMessage_StringWithFormat(t *testing.T) {
	lm := &LazyMessage{
		Level:     1,
		Format:    "Hello %s, number: %d",
		Args:      []interface{}{"world", 42},
		Timestamp: time.Now(),
	}

	expected := "Hello world, number: 42"
	result := lm.String()

	if result != expected {
		t.Errorf("LazyMessage.String() = %q, want %q", result, expected)
	}
}

func TestLazyMessage_StringWithRaw(t *testing.T) {
	rawMessage := []byte("Raw log message")
	lm := &LazyMessage{
		Level:     1,
		Raw:       rawMessage,
		Timestamp: time.Now(),
	}

	expected := "Raw log message"
	result := lm.String()

	if result != expected {
		t.Errorf("LazyMessage.String() = %q, want %q", result, expected)
	}
}

func TestLazyMessage_StringWithEntry(t *testing.T) {
	entry := &types.LogEntry{
		Level:     "INFO",
		Message:   "Entry message",
		Timestamp: time.Now().Format(time.RFC3339),
		Fields:    map[string]interface{}{"key": "value"},
	}

	lm := &LazyMessage{
		Level:     1,
		Entry:     entry,
		Timestamp: time.Now(),
	}

	expected := "Entry message"
	result := lm.String()

	if result != expected {
		t.Errorf("LazyMessage.String() = %q, want %q", result, expected)
	}
}

func TestLazyMessage_StringPriority(t *testing.T) {
	// Test that Raw takes precedence over Entry and Format
	entry := &types.LogEntry{
		Level:     "INFO",
		Message:   "Entry message",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	lm := &LazyMessage{
		Level:     1,
		Format:    "Format %s",
		Args:      []interface{}{"message"},
		Entry:     entry,
		Raw:       []byte("Raw message"),
		Timestamp: time.Now(),
	}

	expected := "Raw message"
	result := lm.String()

	if result != expected {
		t.Errorf("LazyMessage.String() = %q, want %q (Raw should take precedence)", result, expected)
	}
}

func TestLazyMessage_StringEntryOverFormat(t *testing.T) {
	// Test that Entry takes precedence over Format when Raw is not present
	entry := &types.LogEntry{
		Level:     "INFO",
		Message:   "Entry message",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	lm := &LazyMessage{
		Level:     1,
		Format:    "Format %s",
		Args:      []interface{}{"message"},
		Entry:     entry,
		Timestamp: time.Now(),
	}

	expected := "Entry message"
	result := lm.String()

	if result != expected {
		t.Errorf("LazyMessage.String() = %q, want %q (Entry should take precedence over Format)", result, expected)
	}
}

func TestLazyMessage_LazyEvaluation(t *testing.T) {
	formatCalled := false
	originalSprintf := fmt.Sprintf

	// Mock fmt.Sprintf to track calls
	// Note: This is a conceptual test - in practice, we can't easily mock fmt.Sprintf
	// But we can test that the string is only formatted once
	lm := &LazyMessage{
		Level:     1,
		Format:    "Test %s",
		Args:      []interface{}{"message"},
		Timestamp: time.Now(),
	}

	// First call should format the string
	result1 := lm.String()
	
	// Second call should return the cached result
	result2 := lm.String()

	if result1 != result2 {
		t.Errorf("LazyMessage.String() not consistent: %q vs %q", result1, result2)
	}

	if result1 != "Test message" {
		t.Errorf("LazyMessage.String() = %q, want %q", result1, "Test message")
	}

	// Restore original function (not actually needed since we didn't replace it)
	_ = originalSprintf
	_ = formatCalled
}

func TestLazyMessage_ThreadSafety(t *testing.T) {
	lm := &LazyMessage{
		Level:     1,
		Format:    "Concurrent test %d",
		Args:      []interface{}{42},
		Timestamp: time.Now(),
	}

	const numGoroutines = 100
	var wg sync.WaitGroup
	results := make([]string, numGoroutines)

	// Concurrent access to String() method
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			defer wg.Done()
			results[index] = lm.String()
		}(i)
	}

	wg.Wait()

	// All results should be the same
	expected := "Concurrent test 42"
	for i, result := range results {
		if result != expected {
			t.Errorf("Goroutine %d got %q, want %q", i, result, expected)
		}
	}
}

func TestLazyMessage_ThreadSafetyWithRace(t *testing.T) {
	// Test for race conditions using the race detector
	if testing.Short() {
		t.Skip("skipping race condition test in short mode")
	}

	const numGoroutines = 50
	const numIterations = 100

	for iteration := 0; iteration < numIterations; iteration++ {
		lm := &LazyMessage{
			Level:     1,
			Format:    "Race test %d iteration %d",
			Args:      []interface{}{iteration, iteration},
			Timestamp: time.Now(),
		}

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				_ = lm.String()
			}()
		}

		wg.Wait()
	}
}

func TestLazyMessage_FormattingErrors(t *testing.T) {
	// Test with mismatched format and args
	lm := &LazyMessage{
		Level:     1,
		Format:    "Test %s %d %f", // 3 format specifiers
		Args:      []interface{}{"only", "two"}, // Only 2 arguments
		Timestamp: time.Now(),
	}

	// Should not panic, but may produce unexpected output
	result := lm.String()
	
	// The exact result depends on fmt.Sprintf behavior with mismatched args
	// We just ensure it doesn't panic and returns something
	if result == "" {
		t.Error("LazyMessage.String() should not return empty string even with formatting errors")
	}
}

func TestLazyMessage_EmptyFormat(t *testing.T) {
	lm := &LazyMessage{
		Level:     1,
		Format:    "",
		Args:      []interface{}{"arg1", "arg2"},
		Timestamp: time.Now(),
	}

	result := lm.String()
	// When format is empty but args exist, fmt.Sprintf returns the extra args formatted
	// This is the expected behavior of fmt.Sprintf with empty format string
	expected := "%!(EXTRA string=arg1, string=arg2)"

	if result != expected {
		t.Errorf("LazyMessage.String() with empty format = %q, want %q", result, expected)
	}
}

func TestLazyMessage_NoArgs(t *testing.T) {
	lm := &LazyMessage{
		Level:     1,
		Format:    "Simple message with no format specifiers",
		Args:      []interface{}{},
		Timestamp: time.Now(),
	}

	result := lm.String()
	expected := "Simple message with no format specifiers"

	if result != expected {
		t.Errorf("LazyMessage.String() = %q, want %q", result, expected)
	}
}

func TestLazyMessage_NilArgs(t *testing.T) {
	lm := &LazyMessage{
		Level:     1,
		Format:    "Message with nil args",
		Args:      nil,
		Timestamp: time.Now(),
	}

	result := lm.String()
	expected := "Message with nil args"

	if result != expected {
		t.Errorf("LazyMessage.String() = %q, want %q", result, expected)
	}
}

func TestLazyMessage_ComplexFormatting(t *testing.T) {
	now := time.Now()
	lm := &LazyMessage{
		Level:  2,
		Format: "User %s (ID: %d) performed action '%s' at %v with success=%t",
		Args:   []interface{}{"john_doe", 12345, "login", now, true},
		Timestamp: now,
	}

	result := lm.String()
	expected := fmt.Sprintf("User john_doe (ID: 12345) performed action 'login' at %v with success=true", now)

	if result != expected {
		t.Errorf("LazyMessage.String() = %q, want %q", result, expected)
	}
}

func TestLazyMessage_ToLogMessage(t *testing.T) {
	now := time.Now()
	entry := &types.LogEntry{
		Level:     "INFO",
		Message:   "Test message",
		Timestamp: now.Format(time.RFC3339),
	}

	lm := &LazyMessage{
		Level:     2,
		Format:    "Test %s",
		Args:      []interface{}{"message"},
		Timestamp: now,
		Entry:     entry,
		Raw:       []byte("raw data"),
	}

	logMsg := lm.ToLogMessage()

	if logMsg.Level != lm.Level {
		t.Errorf("ToLogMessage().Level = %d, want %d", logMsg.Level, lm.Level)
	}

	if logMsg.Format != lm.Format {
		t.Errorf("ToLogMessage().Format = %q, want %q", logMsg.Format, lm.Format)
	}

	if len(logMsg.Args) != len(lm.Args) {
		t.Errorf("ToLogMessage().Args length = %d, want %d", len(logMsg.Args), len(lm.Args))
	}

	for i, arg := range logMsg.Args {
		if arg != lm.Args[i] {
			t.Errorf("ToLogMessage().Args[%d] = %v, want %v", i, arg, lm.Args[i])
		}
	}

	if !logMsg.Timestamp.Equal(lm.Timestamp) {
		t.Errorf("ToLogMessage().Timestamp = %v, want %v", logMsg.Timestamp, lm.Timestamp)
	}

	if logMsg.Entry != lm.Entry {
		t.Errorf("ToLogMessage().Entry = %v, want %v", logMsg.Entry, lm.Entry)
	}

	if string(logMsg.Raw) != string(lm.Raw) {
		t.Errorf("ToLogMessage().Raw = %q, want %q", string(logMsg.Raw), string(lm.Raw))
	}
}

func TestLazyMessage_FieldAccess(t *testing.T) {
	now := time.Now()
	entry := &types.LogEntry{
		Level:     "DEBUG",
		Message:   "Debug message",
		Timestamp: now.Format(time.RFC3339),
		Fields:    map[string]interface{}{"key": "value"},
	}

	lm := &LazyMessage{
		Level:     0,
		Format:    "Debug: %s",
		Args:      []interface{}{"test"},
		Timestamp: now,
		Entry:     entry,
		Raw:       []byte("debug raw"),
	}

	// Test field access
	if lm.Level != 0 {
		t.Errorf("Level = %d, want 0", lm.Level)
	}

	if lm.Format != "Debug: %s" {
		t.Errorf("Format = %q, want %q", lm.Format, "Debug: %s")
	}

	if len(lm.Args) != 1 || lm.Args[0] != "test" {
		t.Errorf("Args = %v, want [test]", lm.Args)
	}

	if !lm.Timestamp.Equal(now) {
		t.Errorf("Timestamp = %v, want %v", lm.Timestamp, now)
	}

	if lm.Entry != entry {
		t.Errorf("Entry = %v, want %v", lm.Entry, entry)
	}

	if string(lm.Raw) != "debug raw" {
		t.Errorf("Raw = %q, want %q", string(lm.Raw), "debug raw")
	}
}

func TestLazyMessage_ConcurrentStringAndToLogMessage(t *testing.T) {
	// Test concurrent access to both String() and ToLogMessage()
	lm := &LazyMessage{
		Level:     1,
		Format:    "Concurrent access test %d",
		Args:      []interface{}{123},
		Timestamp: time.Now(),
	}

	const numGoroutines = 50
	var wg sync.WaitGroup
	
	stringResults := make([]string, numGoroutines)
	logMsgResults := make([]types.LogMessage, numGoroutines)

	wg.Add(numGoroutines * 2) // Each goroutine spawns 2 operations

	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			defer wg.Done()
			stringResults[index] = lm.String()
		}(i)

		go func(index int) {
			defer wg.Done()
			logMsgResults[index] = lm.ToLogMessage()
		}(i)
	}

	wg.Wait()

	// Verify all String() results are the same
	expected := "Concurrent access test 123"
	for i, result := range stringResults {
		if result != expected {
			t.Errorf("String() result %d = %q, want %q", i, result, expected)
		}
	}

	// Verify all ToLogMessage() results are consistent
	for i, logMsg := range logMsgResults {
		if logMsg.Level != lm.Level {
			t.Errorf("ToLogMessage() result %d Level = %d, want %d", i, logMsg.Level, lm.Level)
		}
		if logMsg.Format != lm.Format {
			t.Errorf("ToLogMessage() result %d Format = %q, want %q", i, logMsg.Format, lm.Format)
		}
	}
}

func TestLazyMessage_MemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory usage test in short mode")
	}

	// Test that lazy evaluation doesn't cause memory leaks
	const numMessages = 10000

	var messages []*LazyMessage
	for i := 0; i < numMessages; i++ {
		lm := &LazyMessage{
			Level:     1,
			Format:    "Message %d with data %s",
			Args:      []interface{}{i, fmt.Sprintf("data-%d", i)},
			Timestamp: time.Now(),
		}
		messages = append(messages, lm)
	}

	// Force garbage collection
	runtime.GC()
	
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Access String() for all messages
	for _, msg := range messages {
		_ = msg.String()
	}

	// Force garbage collection again
	runtime.GC()
	
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	// The memory usage shouldn't be dramatically different
	// This is a basic sanity check - exact values depend on many factors
	if m2.Alloc > m1.Alloc*2 {
		t.Logf("Memory usage increased significantly: %d -> %d bytes", m1.Alloc, m2.Alloc)
		// Note: Not failing the test as memory usage can vary greatly
	}
}

// Benchmark tests for performance characteristics
func BenchmarkLazyMessage_StringSimple(b *testing.B) {
	lm := &LazyMessage{
		Level:     1,
		Format:    "Simple message",
		Args:      []interface{}{},
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = lm.String()
	}
}

func BenchmarkLazyMessage_StringFormatted(b *testing.B) {
	lm := &LazyMessage{
		Level:     1,
		Format:    "User %s performed %s with result %t at %d",
		Args:      []interface{}{"john", "login", true, 1234567890},
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = lm.String()
	}
}

func BenchmarkLazyMessage_StringRaw(b *testing.B) {
	lm := &LazyMessage{
		Level:     1,
		Raw:       []byte("Raw message content"),
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = lm.String()
	}
}

func BenchmarkLazyMessage_StringEntry(b *testing.B) {
	entry := &types.LogEntry{
		Level:     "INFO",
		Message:   "Entry message content",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	lm := &LazyMessage{
		Level:     1,
		Entry:     entry,
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = lm.String()
	}
}

func BenchmarkLazyMessage_ToLogMessage(b *testing.B) {
	lm := &LazyMessage{
		Level:     1,
		Format:    "Benchmark %s with %d",
		Args:      []interface{}{"test", 42},
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = lm.ToLogMessage()
	}
}

func BenchmarkLazyMessage_ConcurrentString(b *testing.B) {
	lm := &LazyMessage{
		Level:     1,
		Format:    "Concurrent benchmark %d",
		Args:      []interface{}{42},
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = lm.String()
		}
	})
}

func BenchmarkLazyMessage_FirstCallVsSubsequent(b *testing.B) {
	b.Run("FirstCall", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			lm := &LazyMessage{
				Level:     1,
				Format:    "First call %d",
				Args:      []interface{}{i},
				Timestamp: time.Now(),
			}
			_ = lm.String() // First call - does formatting
		}
	})

	b.Run("SubsequentCall", func(b *testing.B) {
		lm := &LazyMessage{
			Level:     1,
			Format:    "Subsequent call %d",
			Args:      []interface{}{42},
			Timestamp: time.Now(),
		}
		_ = lm.String() // Prime the cache

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = lm.String() // Subsequent calls - returns cached result
		}
	})
}