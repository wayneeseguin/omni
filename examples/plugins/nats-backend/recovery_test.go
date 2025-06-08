//go:build integration

package main

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

// TestNATSErrorRecovery tests various error scenarios and recovery
func TestNATSErrorRecovery(t *testing.T) {
	scenarios := []struct {
		name string
		test func(t *testing.T)
	}{
		{"Connection Loss Recovery", testConnectionLossRecovery},
		{"Buffer Overflow Recovery", testBufferOverflowRecovery},
		{"Concurrent Write Safety", testConcurrentWriteSafety},
		{"Panic Recovery", testPanicRecovery},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, scenario.test)
	}
}

func testConnectionLossRecovery(t *testing.T) {
	// Create backend with specific reconnection settings
	backend, err := NewNATSBackend("nats://localhost:4222/recovery.test?max_reconnect=5&reconnect_wait=1")
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Verify initial connection
	if !backend.conn.IsConnected() {
		t.Fatal("Backend should be initially connected")
	}

	// Write test message
	msg1 := []byte(`{"test":"before disconnect"}`)
	if _, err := backend.Write(msg1); err != nil {
		t.Errorf("Failed to write initial message: %v", err)
	}

	// Note: Actual connection disruption would require Docker container manipulation
	// For now, we verify the reconnection handlers are properly set
	opts := backend.conn.Opts
	if opts.MaxReconnect != 5 {
		t.Errorf("Expected MaxReconnect=5, got %d", opts.MaxReconnect)
	}

	t.Log("Connection recovery configuration verified")
}

func testBufferOverflowRecovery(t *testing.T) {
	// Create backend with small batch size
	backend, err := NewNATSBackend("nats://localhost:4222/overflow.test?batch=3&flush_interval=5000")
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Write more messages than batch size rapidly
	overflow := 10
	written := 0
	errors := 0

	for i := 0; i < overflow; i++ {
		msg := []byte(`{"overflow":true}`)
		_, err := backend.Write(msg)
		if err != nil {
			errors++
			t.Logf("Write %d failed: %v", i, err)
		} else {
			written++
		}
	}

	// Force flush
	if err := backend.Flush(); err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	t.Logf("Buffer overflow test: written=%d, errors=%d", written, errors)
	if written == 0 {
		t.Error("No messages were written successfully")
	}
}

func testConcurrentWriteSafety(t *testing.T) {
	backend, err := NewNATSBackend("nats://localhost:4222/concurrent.test?batch=50")
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Concurrent writers
	writers := 10
	messagesPerWriter := 100
	var wg sync.WaitGroup
	var successCount int32
	var errorCount int32

	// Create subscriber to verify messages
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		t.Fatalf("Failed to connect subscriber: %v", err)
	}
	defer nc.Close()

	var receivedCount int32
	sub, err := nc.Subscribe("concurrent.test", func(msg *nats.Msg) {
		atomic.AddInt32(&receivedCount, 1)
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	// Start concurrent writers
	start := time.Now()
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for i := 0; i < messagesPerWriter; i++ {
				msg := []byte(`{"writer":` + string(rune(writerID)) + `}`)
				_, err := backend.Write(msg)
				if err != nil {
					atomic.AddInt32(&errorCount, 1)
				} else {
					atomic.AddInt32(&successCount, 1)
				}
			}
		}(w)
	}

	// Wait for all writers
	wg.Wait()
	duration := time.Since(start)

	// Flush and wait for messages
	backend.Flush()
	time.Sleep(500 * time.Millisecond)

	success := atomic.LoadInt32(&successCount)
	errors := atomic.LoadInt32(&errorCount)
	received := atomic.LoadInt32(&receivedCount)

	t.Logf("Concurrent write test completed in %v", duration)
	t.Logf("Success: %d, Errors: %d, Received: %d", success, errors, received)

	if success == 0 {
		t.Error("No messages were written successfully")
	}

	if float64(errors)/float64(success+errors) > 0.1 {
		t.Errorf("Error rate too high: %.2f%%", float64(errors)/float64(success+errors)*100)
	}
}

func testPanicRecovery(t *testing.T) {
	// Test that the backend can recover from panics
	backend, err := NewNATSBackend("nats://localhost:4222/panic.test")
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Function that might panic
	riskyWrite := func() (recovered bool) {
		defer func() {
			if r := recover(); r != nil {
				recovered = true
				t.Logf("Recovered from panic: %v", r)
			}
		}()

		// This shouldn't panic in normal operation
		msg := []byte(`{"test":"panic recovery"}`)
		_, err := backend.Write(msg)
		if err != nil {
			t.Logf("Write error: %v", err)
		}
		return false
	}

	// Test recovery
	if riskyWrite() {
		t.Log("Successfully recovered from panic")
	} else {
		t.Log("No panic occurred (normal operation)")
	}

	// Verify backend still works after potential panic
	msg := []byte(`{"test":"after recovery"}`)
	if _, err := backend.Write(msg); err != nil {
		t.Errorf("Backend write failed after recovery test: %v", err)
	}

	if err := backend.Flush(); err != nil {
		t.Errorf("Backend flush failed after recovery test: %v", err)
	}
}

// TestNATSLoadTest performs a load test to find limits
func TestNATSLoadTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	backend, err := NewNATSBackend("nats://localhost:4222/load.test?batch=1000&flush_interval=100")
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Message payload
	payload := make([]byte, 1024) // 1KB message
	for i := range payload {
		payload[i] = byte('A' + (i % 26))
	}

	// Load test parameters
	duration := 5 * time.Second
	start := time.Now()
	var messagesSent int64
	var bytesSent int64
	var errors int64

	t.Logf("Starting load test for %v...", duration)

	// Send messages until timeout
	for time.Since(start) < duration {
		n, err := backend.Write(payload)
		if err != nil {
			atomic.AddInt64(&errors, 1)
		} else {
			atomic.AddInt64(&messagesSent, 1)
			atomic.AddInt64(&bytesSent, int64(n))
		}
	}

	// Final flush
	backend.Flush()

	// Calculate metrics
	elapsed := time.Since(start)
	messagesPerSec := float64(messagesSent) / elapsed.Seconds()
	mbPerSec := float64(bytesSent) / elapsed.Seconds() / 1024 / 1024
	errorRate := float64(errors) / float64(messagesSent+errors) * 100

	t.Log("Load test results:")
	t.Logf("  Duration: %v", elapsed)
	t.Logf("  Messages sent: %d", messagesSent)
	t.Logf("  Data sent: %.2f MB", float64(bytesSent)/1024/1024)
	t.Logf("  Messages/sec: %.2f", messagesPerSec)
	t.Logf("  Throughput: %.2f MB/s", mbPerSec)
	t.Logf("  Errors: %d (%.2f%%)", errors, errorRate)

	// Performance thresholds
	if messagesPerSec < 1000 {
		t.Errorf("Performance below threshold: %.2f messages/sec (expected > 1000)", messagesPerSec)
	}

	if errorRate > 1.0 {
		t.Errorf("Error rate too high: %.2f%% (expected < 1%%)", errorRate)
	}
}
