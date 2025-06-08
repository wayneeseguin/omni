//go:build integration

package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

// TestNATSMonitoring provides monitoring and diagnostic capabilities
func TestNATSMonitoring(t *testing.T) {
	// Create backend
	backend, err := NewNATSBackend("nats://localhost:4222/monitor.test")
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Create monitoring subscriber
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		t.Fatalf("Failed to connect monitor: %v", err)
	}
	defer nc.Close()

	// Message statistics
	stats := &MessageStats{
		startTime: time.Now(),
	}

	// Subscribe to monitor messages
	sub, err := nc.Subscribe("monitor.test", func(msg *nats.Msg) {
		stats.ProcessMessage(msg.Data)
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	// Send various types of messages
	testMessages := []struct {
		level   string
		message string
		fields  map[string]interface{}
	}{
		{"INFO", "Application started", map[string]interface{}{"version": "1.0"}},
		{"DEBUG", "Processing request", map[string]interface{}{"id": "123"}},
		{"WARN", "High memory usage", map[string]interface{}{"usage": 85.5}},
		{"ERROR", "Database connection failed", map[string]interface{}{"retry": 3}},
		{"INFO", "Request completed", map[string]interface{}{"duration": 250}},
	}

	// Send messages
	for i, tm := range testMessages {
		entry := map[string]interface{}{
			"level":     tm.level,
			"message":   tm.message,
			"timestamp": time.Now().Format(time.RFC3339),
			"fields":    tm.fields,
			"seq":       i,
		}

		data, err := json.Marshal(entry)
		if err != nil {
			t.Errorf("Failed to marshal message %d: %v", i, err)
			continue
		}

		if _, err := backend.Write(data); err != nil {
			t.Errorf("Failed to write message %d: %v", i, err)
		}
	}

	// Flush and wait
	backend.Flush()
	time.Sleep(50 * time.Millisecond)

	// Report statistics
	stats.Report(t)
}

// MessageStats tracks message statistics
type MessageStats struct {
	mu          sync.Mutex
	startTime   time.Time
	totalCount  int
	levelCounts map[string]int
	totalBytes  int
	errors      int
	latencies   []time.Duration
	messages    []ParsedMessage
}

// ParsedMessage represents a parsed log message
type ParsedMessage struct {
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Timestamp string                 `json:"timestamp"`
	Fields    map[string]interface{} `json:"fields"`
	Seq       int                    `json:"seq"`
}

// ProcessMessage processes a received message
func (s *MessageStats) ProcessMessage(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.totalCount++
	s.totalBytes += len(data)

	// Parse message
	var msg ParsedMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		s.errors++
		return
	}

	// Track by level
	if s.levelCounts == nil {
		s.levelCounts = make(map[string]int)
	}
	s.levelCounts[msg.Level]++

	// Calculate latency if timestamp is present
	if msg.Timestamp != "" {
		if msgTime, err := time.Parse(time.RFC3339, msg.Timestamp); err == nil {
			latency := time.Since(msgTime)
			s.latencies = append(s.latencies, latency)
		}
	}

	s.messages = append(s.messages, msg)
}

// Report generates a monitoring report
func (s *MessageStats) Report(t *testing.T) {
	s.mu.Lock()
	defer s.mu.Unlock()

	duration := time.Since(s.startTime)

	t.Log("=== NATS Monitoring Report ===")
	t.Logf("Duration: %v", duration)
	t.Logf("Total messages: %d", s.totalCount)
	t.Logf("Total bytes: %d", s.totalBytes)
	t.Logf("Parse errors: %d", s.errors)

	if s.totalCount > 0 {
		t.Logf("Messages/sec: %.2f", float64(s.totalCount)/duration.Seconds())
		t.Logf("Bytes/sec: %.2f", float64(s.totalBytes)/duration.Seconds())
		t.Logf("Avg message size: %d bytes", s.totalBytes/s.totalCount)
	}

	// Level breakdown
	t.Log("\nMessage levels:")
	for level, count := range s.levelCounts {
		percentage := float64(count) / float64(s.totalCount) * 100
		t.Logf("  %s: %d (%.1f%%)", level, count, percentage)
	}

	// Latency stats
	if len(s.latencies) > 0 {
		var total time.Duration
		var min, max time.Duration = s.latencies[0], s.latencies[0]

		for _, l := range s.latencies {
			total += l
			if l < min {
				min = l
			}
			if l > max {
				max = l
			}
		}

		avg := total / time.Duration(len(s.latencies))
		t.Log("\nLatency statistics:")
		t.Logf("  Min: %v", min)
		t.Logf("  Max: %v", max)
		t.Logf("  Avg: %v", avg)
	}

	// Message sequence check
	t.Log("\nMessage sequence check:")
	if s.checkSequence() {
		t.Log("  ✓ All messages received in order")
	} else {
		t.Log("  ✗ Messages received out of order or missing")
	}
}

// checkSequence verifies message ordering
func (s *MessageStats) checkSequence() bool {
	if len(s.messages) == 0 {
		return true
	}

	// Sort by sequence number and check for gaps
	expected := 0
	for _, msg := range s.messages {
		if msg.Seq != expected {
			return false
		}
		expected++
	}
	return true
}

// TestNATSConnectionMonitoring monitors connection health
func TestNATSConnectionMonitoring(t *testing.T) {
	// Connection with monitoring callbacks
	opts := []nats.Option{
		nats.Name("omni-monitor-test"),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			t.Logf("NATS Disconnected: %v", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			t.Logf("NATS Reconnected to %s", nc.ConnectedUrl())
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			t.Log("NATS Connection closed")
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			t.Logf("NATS Error: %v", err)
		}),
	}

	nc, err := nats.Connect(nats.DefaultURL, opts...)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer nc.Close()

	// Monitor connection stats
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	done := make(chan bool)
	go func() {
		for i := 0; i < 5; i++ {
			select {
			case <-ticker.C:
				stats := nc.Stats()
				t.Logf("Connection stats at %ds:", i+1)
				t.Logf("  In msgs: %d, bytes: %d", stats.InMsgs, stats.InBytes)
				t.Logf("  Out msgs: %d, bytes: %d", stats.OutMsgs, stats.OutBytes)
				t.Logf("  Reconnects: %d", stats.Reconnects)
			case <-done:
				return
			}
		}
		done <- true
	}()

	// Send some test messages
	for i := 0; i < 10; i++ {
		msg := fmt.Sprintf("monitor test %d", i)
		if err := nc.Publish("monitor.health", []byte(msg)); err != nil {
			t.Errorf("Publish failed: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for monitoring to complete
	<-done
}
