package metrics

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewCollector(t *testing.T) {
	c := NewCollector()
	if c == nil {
		t.Fatal("NewCollector() returned nil")
	}
	
	// Verify initial state
	if c.GetMessageCount(1) != 0 {
		t.Error("Expected initial message count to be 0")
	}
	if c.GetErrorCount() != 0 {
		t.Error("Expected initial error count to be 0")
	}
}

func TestTrackMessageLogged(t *testing.T) {
	c := NewCollector()
	
	tests := []struct {
		name  string
		level int
		count int
	}{
		{"Single message level 1", 1, 1},
		{"Multiple messages level 2", 2, 5},
		{"Many messages level 3", 3, 100},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < tt.count; i++ {
				c.TrackMessageLogged(tt.level)
			}
			
			if got := c.GetMessageCount(tt.level); got != uint64(tt.count) {
				t.Errorf("GetMessageCount(%d) = %d, want %d", tt.level, got, tt.count)
			}
		})
	}
}

func TestTrackMessageDropped(t *testing.T) {
	c := NewCollector()
	
	// Track dropped messages
	for i := 0; i < 10; i++ {
		c.TrackMessageDropped()
	}
	
	metrics := c.GetMetrics(0, 0, nil)
	if metrics.MessagesDropped != 10 {
		t.Errorf("MessagesDropped = %d, want 10", metrics.MessagesDropped)
	}
}

func TestTrackRotation(t *testing.T) {
	c := NewCollector()
	
	// Track rotations
	for i := 0; i < 5; i++ {
		c.TrackRotation()
	}
	
	metrics := c.GetMetrics(0, 0, nil)
	if metrics.RotationCount != 5 {
		t.Errorf("RotationCount = %d, want 5", metrics.RotationCount)
	}
}

func TestTrackCompression(t *testing.T) {
	c := NewCollector()
	
	// Track compressions
	for i := 0; i < 3; i++ {
		c.TrackCompression()
	}
	
	metrics := c.GetMetrics(0, 0, nil)
	if metrics.CompressionCount != 3 {
		t.Errorf("CompressionCount = %d, want 3", metrics.CompressionCount)
	}
}

func TestTrackWrite(t *testing.T) {
	c := NewCollector()
	
	tests := []struct {
		name     string
		bytes    int64
		duration time.Duration
	}{
		{"Small write", 100, 1 * time.Millisecond},
		{"Medium write", 1000, 5 * time.Millisecond},
		{"Large write", 10000, 10 * time.Millisecond},
		{"Zero bytes", 0, 1 * time.Millisecond},
		{"Negative bytes", -100, 1 * time.Millisecond}, // Should be ignored
	}
	
	var expectedBytes uint64
	var expectedCount uint64
	var maxDuration time.Duration
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.TrackWrite(tt.bytes, tt.duration)
			
			if tt.bytes > 0 {
				expectedBytes += uint64(tt.bytes)
			}
			expectedCount++
			
			if tt.duration > maxDuration {
				maxDuration = tt.duration
			}
		})
	}
	
	metrics := c.GetMetrics(0, 0, nil)
	
	if metrics.BytesWritten != expectedBytes {
		t.Errorf("BytesWritten = %d, want %d", metrics.BytesWritten, expectedBytes)
	}
	
	if metrics.MaxWriteTime != maxDuration {
		t.Errorf("MaxWriteTime = %v, want %v", metrics.MaxWriteTime, maxDuration)
	}
	
	// Check average calculation
	if metrics.AverageWriteTime == 0 {
		t.Error("AverageWriteTime should not be zero")
	}
	
	stats := c.GetStats()
	if stats.WriteCount != expectedCount {
		t.Errorf("WriteCount = %d, want %d", stats.WriteCount, expectedCount)
	}
	if stats.BytesWritten != expectedBytes {
		t.Errorf("Stats.BytesWritten = %d, want %d", stats.BytesWritten, expectedBytes)
	}
}

func TestTrackError(t *testing.T) {
	c := NewCollector()
	
	sources := []struct {
		source string
		count  int
	}{
		{"file_backend", 3},
		{"syslog_backend", 2},
		{"plugin_backend", 5},
	}
	
	var totalErrors uint64
	for _, s := range sources {
		for i := 0; i < s.count; i++ {
			c.TrackError(s.source)
			totalErrors++
		}
	}
	
	// Verify total error count
	if got := c.GetErrorCount(); got != totalErrors {
		t.Errorf("GetErrorCount() = %d, want %d", got, totalErrors)
	}
	
	// Verify per-source counts
	for _, s := range sources {
		if got := c.GetErrorCountBySource(s.source); got != uint64(s.count) {
			t.Errorf("GetErrorCountBySource(%s) = %d, want %d", s.source, got, s.count)
		}
	}
	
	// Verify non-existent source
	if got := c.GetErrorCountBySource("unknown"); got != 0 {
		t.Errorf("GetErrorCountBySource(unknown) = %d, want 0", got)
	}
	
	// Check in metrics
	metrics := c.GetMetrics(0, 0, nil)
	if metrics.ErrorCount != totalErrors {
		t.Errorf("metrics.ErrorCount = %d, want %d", metrics.ErrorCount, totalErrors)
	}
	
	for _, s := range sources {
		if metrics.ErrorsBySource[s.source] != uint64(s.count) {
			t.Errorf("metrics.ErrorsBySource[%s] = %d, want %d", 
				s.source, metrics.ErrorsBySource[s.source], s.count)
		}
	}
}

func TestGetMetrics(t *testing.T) {
	c := NewCollector()
	
	// Add various metrics
	c.TrackMessageLogged(1)
	c.TrackMessageLogged(1)
	c.TrackMessageLogged(2)
	c.TrackMessageDropped()
	c.TrackRotation()
	c.TrackCompression()
	c.TrackWrite(1024, 2*time.Millisecond)
	c.TrackError("test_source")
	
	// Create destination metrics
	destinations := []DestinationMetrics{
		{
			Name:           "file1",
			Type:           "file",
			Enabled:        true,
			BytesWritten:   1024,
			CurrentSize:    2048,
			Rotations:      1,
			Errors:         0,
			LastWrite:      time.Now(),
			AverageLatency: 1 * time.Millisecond,
		},
	}
	
	metrics := c.GetMetrics(10, 100, destinations)
	
	// Verify all metrics
	if metrics.MessagesLogged[1] != 2 {
		t.Errorf("MessagesLogged[1] = %d, want 2", metrics.MessagesLogged[1])
	}
	if metrics.MessagesLogged[2] != 1 {
		t.Errorf("MessagesLogged[2] = %d, want 1", metrics.MessagesLogged[2])
	}
	if metrics.MessagesDropped != 1 {
		t.Errorf("MessagesDropped = %d, want 1", metrics.MessagesDropped)
	}
	if metrics.QueueDepth != 10 {
		t.Errorf("QueueDepth = %d, want 10", metrics.QueueDepth)
	}
	if metrics.QueueCapacity != 100 {
		t.Errorf("QueueCapacity = %d, want 100", metrics.QueueCapacity)
	}
	if metrics.QueueUtilization != 0.1 {
		t.Errorf("QueueUtilization = %f, want 0.1", metrics.QueueUtilization)
	}
	if metrics.RotationCount != 1 {
		t.Errorf("RotationCount = %d, want 1", metrics.RotationCount)
	}
	if metrics.CompressionCount != 1 {
		t.Errorf("CompressionCount = %d, want 1", metrics.CompressionCount)
	}
	if metrics.BytesWritten != 1024 {
		t.Errorf("BytesWritten = %d, want 1024", metrics.BytesWritten)
	}
	if metrics.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d, want 1", metrics.ErrorCount)
	}
	if metrics.ErrorsBySource["test_source"] != 1 {
		t.Errorf("ErrorsBySource[test_source] = %d, want 1", metrics.ErrorsBySource["test_source"])
	}
	if metrics.DestinationCount != 1 {
		t.Errorf("DestinationCount = %d, want 1", metrics.DestinationCount)
	}
	if len(metrics.Destinations) != 1 {
		t.Errorf("len(Destinations) = %d, want 1", len(metrics.Destinations))
	}
}

func TestGetMetricsWithZeroCapacity(t *testing.T) {
	c := NewCollector()
	
	metrics := c.GetMetrics(0, 0, nil)
	if metrics.QueueUtilization != 0 {
		t.Errorf("QueueUtilization = %f, want 0 when capacity is 0", metrics.QueueUtilization)
	}
}

func TestResetMetrics(t *testing.T) {
	c := NewCollector()
	
	// Add various metrics
	c.TrackMessageLogged(1)
	c.TrackMessageLogged(2)
	c.TrackMessageDropped()
	c.TrackRotation()
	c.TrackCompression()
	c.TrackWrite(1024, 1*time.Millisecond)
	c.TrackError("source1")
	c.TrackError("source2")
	
	// Reset all metrics
	c.ResetMetrics()
	
	// Verify everything is reset
	metrics := c.GetMetrics(0, 0, nil)
	
	if len(metrics.MessagesLogged) != 0 {
		t.Errorf("MessagesLogged should be empty after reset, got %v", metrics.MessagesLogged)
	}
	if metrics.MessagesDropped != 0 {
		t.Errorf("MessagesDropped = %d, want 0 after reset", metrics.MessagesDropped)
	}
	if metrics.RotationCount != 0 {
		t.Errorf("RotationCount = %d, want 0 after reset", metrics.RotationCount)
	}
	if metrics.CompressionCount != 0 {
		t.Errorf("CompressionCount = %d, want 0 after reset", metrics.CompressionCount)
	}
	if metrics.BytesWritten != 0 {
		t.Errorf("BytesWritten = %d, want 0 after reset", metrics.BytesWritten)
	}
	if metrics.ErrorCount != 0 {
		t.Errorf("ErrorCount = %d, want 0 after reset", metrics.ErrorCount)
	}
	if len(metrics.ErrorsBySource) != 0 {
		t.Errorf("ErrorsBySource should be empty after reset, got %v", metrics.ErrorsBySource)
	}
	if metrics.AverageWriteTime != 0 {
		t.Errorf("AverageWriteTime = %v, want 0 after reset", metrics.AverageWriteTime)
	}
	if metrics.MaxWriteTime != 0 {
		t.Errorf("MaxWriteTime = %v, want 0 after reset", metrics.MaxWriteTime)
	}
	
	// Also check individual getters
	if c.GetMessageCount(1) != 0 {
		t.Error("GetMessageCount(1) should be 0 after reset")
	}
	if c.GetErrorCount() != 0 {
		t.Error("GetErrorCount() should be 0 after reset")
	}
	if c.GetErrorCountBySource("source1") != 0 {
		t.Error("GetErrorCountBySource(source1) should be 0 after reset")
	}
}

func TestConcurrentTracking(t *testing.T) {
	c := NewCollector()
	
	const (
		numGoroutines = 100
		numOperations = 1000
	)
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	// Concurrent message tracking
	for i := 0; i < numGoroutines; i++ {
		go func(level int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				c.TrackMessageLogged(level % 5)
				if j%10 == 0 {
					c.TrackMessageDropped()
				}
				if j%20 == 0 {
					c.TrackRotation()
				}
				if j%30 == 0 {
					c.TrackCompression()
				}
				if j%5 == 0 {
					c.TrackWrite(int64(j), time.Duration(j)*time.Microsecond)
				}
				if j%15 == 0 {
					c.TrackError("concurrent_source")
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify metrics are consistent
	metrics := c.GetMetrics(0, 0, nil)
	
	// Each goroutine tracks numOperations messages
	var totalMessages uint64
	for _, count := range metrics.MessagesLogged {
		totalMessages += count
	}
	expectedMessages := uint64(numGoroutines * numOperations)
	if totalMessages != expectedMessages {
		t.Errorf("Total messages = %d, want %d", totalMessages, expectedMessages)
	}
	
	// Verify dropped messages (every 10th operation)
	expectedDropped := uint64(numGoroutines * (numOperations / 10))
	if metrics.MessagesDropped != expectedDropped {
		t.Errorf("MessagesDropped = %d, want %d", metrics.MessagesDropped, expectedDropped)
	}
	
	// Verify rotations (every 20th operation)
	expectedRotations := uint64(numGoroutines * (numOperations / 20))
	if metrics.RotationCount != expectedRotations {
		t.Errorf("RotationCount = %d, want %d", metrics.RotationCount, expectedRotations)
	}
	
	// Verify compressions (every 30th operation, starting from j=0)
	// j goes from 0 to 999, so j%30==0 for j=0,30,60,...,990 (34 times)
	expectedCompressions := uint64(numGoroutines * 34)
	if metrics.CompressionCount != expectedCompressions {
		t.Errorf("CompressionCount = %d, want %d", metrics.CompressionCount, expectedCompressions)
	}
	
	// Verify errors (every 15th operation, starting from j=0)
	// j goes from 0 to 999, so j%15==0 for j=0,15,30,...,990 (67 times)
	expectedErrors := uint64(numGoroutines * 67)
	if metrics.ErrorCount != expectedErrors {
		t.Errorf("ErrorCount = %d, want %d", metrics.ErrorCount, expectedErrors)
	}
}

func TestAverageWriteTimeCalculation(t *testing.T) {
	tests := []struct {
		name        string
		writeCount  uint64
		totalTime   int64
		wantAverage time.Duration
	}{
		{
			name:        "Normal case",
			writeCount:  100,
			totalTime:   int64(100 * time.Millisecond),
			wantAverage: 1 * time.Millisecond,
		},
		{
			name:        "Large write count",
			writeCount:  1<<32 + 1, // Larger than maxInt64 when converted
			totalTime:   int64(1<<32+1) * int64(time.Microsecond),
			wantAverage: 1 * time.Microsecond,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCollector()
			
			// Manually set the internal counters
			atomic.StoreUint64(&c.writeCount, tt.writeCount)
			atomic.StoreInt64(&c.totalWriteTime, tt.totalTime)
			
			metrics := c.GetMetrics(0, 0, nil)
			
			// Allow small rounding differences
			diff := metrics.AverageWriteTime - tt.wantAverage
			if diff < 0 {
				diff = -diff
			}
			if diff > time.Nanosecond {
				t.Errorf("AverageWriteTime = %v, want %v (diff: %v)", 
					metrics.AverageWriteTime, tt.wantAverage, diff)
			}
		})
	}
}

func TestMaxWriteTimeUpdate(t *testing.T) {
	c := NewCollector()
	
	// Track writes with increasing durations
	durations := []time.Duration{
		1 * time.Millisecond,
		3 * time.Millisecond,
		2 * time.Millisecond, // Less than current max
		5 * time.Millisecond,
		4 * time.Millisecond, // Less than current max
	}
	
	expectedMax := 5 * time.Millisecond
	
	for _, d := range durations {
		c.TrackWrite(100, d)
	}
	
	metrics := c.GetMetrics(0, 0, nil)
	if metrics.MaxWriteTime != expectedMax {
		t.Errorf("MaxWriteTime = %v, want %v", metrics.MaxWriteTime, expectedMax)
	}
}

func TestGetMessageCountNonExistent(t *testing.T) {
	c := NewCollector()
	
	// Query non-existent level
	if count := c.GetMessageCount(999); count != 0 {
		t.Errorf("GetMessageCount(999) = %d, want 0", count)
	}
}

func TestAliases(t *testing.T) {
	c := NewCollector()
	
	// Test TrackMessage alias
	c.TrackMessage(1)
	if count := c.GetMessageCount(1); count != 1 {
		t.Errorf("TrackMessage didn't increment count, got %d", count)
	}
	
	// Test TrackDropped alias
	c.TrackDropped()
	metrics := c.GetMetrics(0, 0, nil)
	if metrics.MessagesDropped != 1 {
		t.Errorf("TrackDropped didn't increment dropped count, got %d", metrics.MessagesDropped)
	}
}

func TestDestinationMetrics(t *testing.T) {
	c := NewCollector()
	
	// Create some destination metrics
	now := time.Now()
	destinations := []DestinationMetrics{
		{
			Name:           "file_dest",
			Type:           "file",
			Enabled:        true,
			BytesWritten:   1024 * 1024,
			CurrentSize:    512 * 1024,
			Rotations:      3,
			Errors:         1,
			LastWrite:      now,
			AverageLatency: 500 * time.Microsecond,
		},
		{
			Name:           "syslog_dest",
			Type:           "syslog",
			Enabled:        false,
			BytesWritten:   2048,
			CurrentSize:    0,
			Rotations:      0,
			Errors:         5,
			LastWrite:      now.Add(-1 * time.Hour),
			AverageLatency: 1 * time.Millisecond,
		},
	}
	
	metrics := c.GetMetrics(0, 0, destinations)
	
	if metrics.DestinationCount != 2 {
		t.Errorf("DestinationCount = %d, want 2", metrics.DestinationCount)
	}
	
	if len(metrics.Destinations) != 2 {
		t.Fatalf("len(Destinations) = %d, want 2", len(metrics.Destinations))
	}
	
	// Verify destination metrics are preserved
	for i, dest := range metrics.Destinations {
		if dest.Name != destinations[i].Name {
			t.Errorf("Destination[%d].Name = %s, want %s", i, dest.Name, destinations[i].Name)
		}
		if dest.Type != destinations[i].Type {
			t.Errorf("Destination[%d].Type = %s, want %s", i, dest.Type, destinations[i].Type)
		}
		if dest.Enabled != destinations[i].Enabled {
			t.Errorf("Destination[%d].Enabled = %v, want %v", i, dest.Enabled, destinations[i].Enabled)
		}
		if dest.BytesWritten != destinations[i].BytesWritten {
			t.Errorf("Destination[%d].BytesWritten = %d, want %d", i, dest.BytesWritten, destinations[i].BytesWritten)
		}
		if dest.CurrentSize != destinations[i].CurrentSize {
			t.Errorf("Destination[%d].CurrentSize = %d, want %d", i, dest.CurrentSize, destinations[i].CurrentSize)
		}
		if dest.Rotations != destinations[i].Rotations {
			t.Errorf("Destination[%d].Rotations = %d, want %d", i, dest.Rotations, destinations[i].Rotations)
		}
		if dest.Errors != destinations[i].Errors {
			t.Errorf("Destination[%d].Errors = %d, want %d", i, dest.Errors, destinations[i].Errors)
		}
		if !dest.LastWrite.Equal(destinations[i].LastWrite) {
			t.Errorf("Destination[%d].LastWrite = %v, want %v", i, dest.LastWrite, destinations[i].LastWrite)
		}
		if dest.AverageLatency != destinations[i].AverageLatency {
			t.Errorf("Destination[%d].AverageLatency = %v, want %v", i, dest.AverageLatency, destinations[i].AverageLatency)
		}
	}
}

func BenchmarkTrackMessageLogged(b *testing.B) {
	c := NewCollector()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.TrackMessageLogged(i % 5)
	}
}

func BenchmarkTrackWrite(b *testing.B) {
	c := NewCollector()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.TrackWrite(1024, time.Millisecond)
	}
}

func BenchmarkGetMetrics(b *testing.B) {
	c := NewCollector()
	
	// Populate some data
	for i := 0; i < 100; i++ {
		c.TrackMessageLogged(i % 5)
		c.TrackError("error_source")
		c.TrackWrite(1024, time.Millisecond)
	}
	
	destinations := []DestinationMetrics{
		{Name: "dest1", Type: "file", Enabled: true},
		{Name: "dest2", Type: "syslog", Enabled: true},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.GetMetrics(10, 100, destinations)
	}
}

func BenchmarkConcurrentTracking(b *testing.B) {
	c := NewCollector()
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.TrackMessageLogged(i % 5)
			c.TrackWrite(1024, time.Millisecond)
			if i%10 == 0 {
				c.TrackError("bench_source")
			}
			i++
		}
	})
}