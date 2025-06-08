//go:build integration

package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestNATSBackendIntegration(t *testing.T) {
	// This test requires a running NATS server
	// The integration script should have started one on localhost:4222

	if testing.Verbose() {
		t.Log("Testing NATS backend integration")
		t.Log("Connecting to NATS server at localhost:4222")
	}

	// Create backend with actual connection and retry
	var backend *NATSBackend
	var err error
	for i := 0; i < 5; i++ {
		if testing.Verbose() && i > 0 {
			t.Logf("Retry attempt %d/5", i+1)
		}
		backend, err = NewNATSBackend("nats://localhost:4222/integration.test")
		if err == nil {
			break
		}
		if i < 4 {
			time.Sleep(time.Second)
		}
	}
	if err != nil {
		t.Fatalf("Failed to create NATS backend after retries: %v", err)
	}
	defer backend.Close()

	if testing.Verbose() {
		t.Log("Successfully connected to NATS backend")
	}

	// Test writing a message
	entry := []byte(`{"level":"INFO","message":"Integration test message","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`)

	if testing.Verbose() {
		t.Logf("Writing test message: %d bytes", len(entry))
	}

	n, err := backend.Write(entry)
	if err != nil {
		t.Fatalf("Failed to write to NATS: %v", err)
	}

	if n != len(entry) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(entry), n)
	}

	// Flush to ensure message is sent
	if err := backend.Flush(); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	if testing.Verbose() {
		t.Log("NATS backend integration test completed successfully")
	}
}

func TestNATSBackendQueueGroup(t *testing.T) {
	// Test queue group functionality
	// Note: Queue groups are for subscribers, not publishers
	// This test verifies that messages published by the backend
	// can be consumed by queue group subscribers

	if testing.Verbose() {
		t.Log("Testing NATS queue group functionality")
	}

	// Create a backend (publisher)
	var backend *NATSBackend
	var err error
	for i := 0; i < 5; i++ {
		backend, err = NewNATSBackend("nats://localhost:4222/queue.test")
		if err == nil {
			break
		}
		if i < 4 {
			time.Sleep(time.Second)
		}
	}
	if err != nil {
		t.Fatalf("Failed to create backend after retries: %v", err)
	}
	defer backend.Close()

	// Create NATS connection for subscribers with retry
	var nc *nats.Conn
	for i := 0; i < 5; i++ {
		nc, err = nats.Connect(nats.DefaultURL)
		if err == nil {
			break
		}
		if i < 4 {
			time.Sleep(time.Second)
		}
	}
	if err != nil {
		t.Fatalf("Failed to connect subscriber after retries: %v", err)
	}
	defer nc.Close()

	// Create two queue group subscribers that compete for messages
	var receivedCount1, receivedCount2 int32
	var wg sync.WaitGroup
	wg.Add(10) // Expecting 10 total messages

	// First worker in queue group
	sub1, err := nc.QueueSubscribe("queue.test", "workers", func(msg *nats.Msg) {
		atomic.AddInt32(&receivedCount1, 1)
		wg.Done()
	})
	if err != nil {
		t.Fatalf("Failed to create subscriber 1: %v", err)
	}
	defer sub1.Unsubscribe()

	// Second worker in queue group
	sub2, err := nc.QueueSubscribe("queue.test", "workers", func(msg *nats.Msg) {
		atomic.AddInt32(&receivedCount2, 1)
		wg.Done()
	})
	if err != nil {
		t.Fatalf("Failed to create subscriber 2: %v", err)
	}
	defer sub2.Unsubscribe()

	// Ensure subscribers are ready
	if err := nc.Flush(); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	if testing.Verbose() {
		t.Log("Sending 10 messages to queue...")
	}

	// Send 10 messages
	for i := 0; i < 10; i++ {
		msg := []byte(fmt.Sprintf(`{"id":%d,"message":"test message"}`, i))
		if _, err := backend.Write(msg); err != nil {
			t.Errorf("Failed to write message %d: %v", i, err)
		}
	}

	// Flush to ensure all messages are sent
	if err := backend.Flush(); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	// Wait for all messages to be received
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Good, all messages received
		if testing.Verbose() {
			count1 := atomic.LoadInt32(&receivedCount1)
			count2 := atomic.LoadInt32(&receivedCount2)
			t.Logf("Messages distributed: Worker 1 = %d, Worker 2 = %d", count1, count2)
			t.Log("Queue group test completed successfully")
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("Timeout waiting for messages, received %d+%d/10",
			atomic.LoadInt32(&receivedCount1), atomic.LoadInt32(&receivedCount2))
	}

	count1 := atomic.LoadInt32(&receivedCount1)
	count2 := atomic.LoadInt32(&receivedCount2)
	total := count1 + count2

	if total != 10 {
		t.Errorf("Expected 10 total messages, got %d", total)
	}

	// Verify load distribution (both workers should get some messages)
	if count1 == 0 || count2 == 0 {
		t.Errorf("Queue group load balancing failed: worker1=%d, worker2=%d", count1, count2)
	}

	t.Logf("Queue group test passed: worker1 received %d, worker2 received %d messages", count1, count2)
}

func TestNATSBackendBatching(t *testing.T) {
	// Test batching functionality with retry
	var backend *NATSBackend
	var err error
	for i := 0; i < 5; i++ {
		backend, err = NewNATSBackend("nats://localhost:4222/batch.test?batch=5&flush_interval=100")
		if err == nil {
			break
		}
		if i < 4 {
			time.Sleep(time.Second)
		}
	}
	if err != nil {
		t.Fatalf("Failed to create backend after retries: %v", err)
	}
	defer backend.Close()

	// Create subscriber with retry
	var nc *nats.Conn
	for i := 0; i < 5; i++ {
		nc, err = nats.Connect(nats.DefaultURL)
		if err == nil {
			break
		}
		if i < 4 {
			time.Sleep(time.Second)
		}
	}
	if err != nil {
		t.Fatalf("Failed to connect subscriber after retries: %v", err)
	}
	defer nc.Close()

	received := make(chan []byte, 10)
	sub, err := nc.Subscribe("batch.test", func(msg *nats.Msg) {
		received <- msg.Data
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	// Send 3 messages (less than batch size)
	for i := 0; i < 3; i++ {
		msg := []byte(fmt.Sprintf(`{"batch":%d}`, i))
		if _, err := backend.Write(msg); err != nil {
			t.Errorf("Failed to write message %d: %v", i, err)
		}
	}

	// Messages should not be sent yet (batch size is 5)
	select {
	case <-received:
		t.Error("Received message before batch was full")
	case <-time.After(50 * time.Millisecond):
		// Good, no messages yet
	}

	// Wait for flush interval to trigger
	time.Sleep(150 * time.Millisecond)

	// Now we should receive all 3 messages
	timeout := time.After(1 * time.Second)
	count := 0

	for count < 3 {
		select {
		case <-received:
			count++
		case <-timeout:
			t.Fatalf("Timeout waiting for batched messages, received %d/3", count)
		}
	}

	t.Logf("Successfully received all %d batched messages", count)
}

func TestNATSBackendReconnection(t *testing.T) {
	// Test reconnection behavior with retry
	var backend *NATSBackend
	var err error
	for i := 0; i < 5; i++ {
		backend, err = NewNATSBackend("nats://localhost:4222/reconnect.test?max_reconnect=5&reconnect_wait=1")
		if err == nil {
			break
		}
		if i < 4 {
			time.Sleep(time.Second)
		}
	}
	if err != nil {
		t.Fatalf("Failed to create backend after retries: %v", err)
	}
	defer backend.Close()

	// Verify connection is established
	if backend.conn == nil || !backend.conn.IsConnected() {
		t.Fatal("Backend should be connected")
	}

	// Write a test message
	msg := []byte(`{"test":"reconnection"}`)
	if _, err := backend.Write(msg); err != nil {
		t.Fatalf("Failed to write initial message: %v", err)
	}

	t.Log("NATS reconnection test completed")
}
