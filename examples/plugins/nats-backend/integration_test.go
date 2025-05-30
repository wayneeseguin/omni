// +build integration

package natsplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/wayneeseguin/flexlog"
)

// TestNATSBackend_Integration tests the NATS backend with a real NATS server
// Run with: go test -tags=integration
func TestNATSBackend_Integration(t *testing.T) {
	// Skip if NATS is not available
	conn, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		t.Skipf("NATS server not available: %v", err)
	}
	defer conn.Close()

	// Create a NATS backend
	backend, err := NewNATSBackend("nats://localhost:4222/test.logs")
	if err != nil {
		t.Fatalf("Failed to create NATS backend: %v", err)
	}
	defer backend.Close()

	// Subscribe to the subject to verify messages
	messages := make(chan *nats.Msg, 10)
	sub, err := conn.Subscribe("test.logs", func(msg *nats.Msg) {
		messages <- msg
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	// Test writing a message
	testMessage := []byte("Test log message")
	n, err := backend.Write(testMessage)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(testMessage) {
		t.Errorf("Expected %d bytes written, got %d", len(testMessage), n)
	}

	// Flush to ensure message is sent
	if err := backend.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Wait for message
	select {
	case msg := <-messages:
		if string(msg.Data) != string(testMessage) {
			t.Errorf("Expected message %s, got %s", testMessage, msg.Data)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for message")
	}
}

func TestNATSBackend_Batching_Integration(t *testing.T) {
	// Skip if NATS is not available
	conn, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		t.Skipf("NATS server not available: %v", err)
	}
	defer conn.Close()

	// Create a NATS backend with batching
	backend, err := NewNATSBackend("nats://localhost:4222/test.batch?batch=3&flush_interval=500")
	if err != nil {
		t.Fatalf("Failed to create NATS backend: %v", err)
	}
	defer backend.Close()

	// Subscribe to the subject
	messages := make(chan *nats.Msg, 10)
	sub, err := conn.Subscribe("test.batch", func(msg *nats.Msg) {
		messages <- msg
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	// Write 3 messages (should trigger batch flush)
	for i := 1; i <= 3; i++ {
		msg := fmt.Sprintf("Batch message %d", i)
		_, err := backend.Write([]byte(msg))
		if err != nil {
			t.Fatalf("Write %d failed: %v", i, err)
		}
	}

	// Collect messages
	received := 0
	timeout := time.After(2 * time.Second)
	for received < 3 {
		select {
		case msg := <-messages:
			t.Logf("Received: %s", msg.Data)
			received++
		case <-timeout:
			t.Fatalf("Timeout waiting for messages, received %d/3", received)
		}
	}
}

func TestNATSBackend_QueueGroup_Integration(t *testing.T) {
	// Skip if NATS is not available
	conn, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		t.Skipf("NATS server not available: %v", err)
	}
	defer conn.Close()

	// Create two NATS backends in the same queue group
	backend1, err := NewNATSBackend("nats://localhost:4222/test.queue?queue=workers")
	if err != nil {
		t.Fatalf("Failed to create NATS backend 1: %v", err)
	}
	defer backend1.Close()

	backend2, err := NewNATSBackend("nats://localhost:4222/test.queue?queue=workers")
	if err != nil {
		t.Fatalf("Failed to create NATS backend 2: %v", err)
	}
	defer backend2.Close()

	// Subscribe to the subject with queue group
	messages1 := make(chan *nats.Msg, 10)
	messages2 := make(chan *nats.Msg, 10)

	sub1, err := conn.QueueSubscribe("test.queue", "consumers", func(msg *nats.Msg) {
		messages1 <- msg
	})
	if err != nil {
		t.Fatalf("Failed to subscribe 1: %v", err)
	}
	defer sub1.Unsubscribe()

	sub2, err := conn.QueueSubscribe("test.queue", "consumers", func(msg *nats.Msg) {
		messages2 <- msg
	})
	if err != nil {
		t.Fatalf("Failed to subscribe 2: %v", err)
	}
	defer sub2.Unsubscribe()

	// Write a message
	testMessage := []byte("Queue group test")
	_, err = backend1.Write(testMessage)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if err := backend1.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Only one subscriber should receive the message
	var receivedCount int
	timeout := time.After(1 * time.Second)

	for {
		select {
		case msg := <-messages1:
			t.Logf("Subscriber 1 received: %s", msg.Data)
			receivedCount++
		case msg := <-messages2:
			t.Logf("Subscriber 2 received: %s", msg.Data)
			receivedCount++
		case <-timeout:
			if receivedCount != 1 {
				t.Errorf("Expected 1 message received, got %d", receivedCount)
			}
			return
		}
	}
}

func TestNATSBackend_JSONFormat_Integration(t *testing.T) {
	// Skip if NATS is not available
	conn, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		t.Skipf("NATS server not available: %v", err)
	}
	defer conn.Close()

	// Create a NATS backend with JSON format
	backend, err := NewNATSBackend("nats://localhost:4222/test.json?format=json")
	if err != nil {
		t.Fatalf("Failed to create NATS backend: %v", err)
	}
	defer backend.Close()

	// Subscribe to the subject
	messages := make(chan *nats.Msg, 10)
	sub, err := conn.Subscribe("test.json", func(msg *nats.Msg) {
		messages <- msg
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	// Write a JSON message
	logEntry := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"level":     "INFO",
		"message":   "JSON format test",
		"fields": map[string]interface{}{
			"user_id": 123,
			"action":  "login",
		},
	}

	jsonData, err := json.Marshal(logEntry)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	_, err = backend.Write(jsonData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if err := backend.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Wait for message
	select {
	case msg := <-messages:
		// Parse the received JSON
		var received map[string]interface{}
		if err := json.Unmarshal(msg.Data, &received); err != nil {
			t.Fatalf("Failed to unmarshal received message: %v", err)
		}

		if received["level"] != "INFO" {
			t.Errorf("Expected level INFO, got %v", received["level"])
		}
		if received["message"] != "JSON format test" {
			t.Errorf("Expected message 'JSON format test', got %v", received["message"])
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for message")
	}
}

func TestNATSBackend_WithFlexLog_Integration(t *testing.T) {
	// Skip if NATS is not available
	conn, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		t.Skipf("NATS server not available: %v", err)
	}
	defer conn.Close()

	// Register the NATS plugin
	plugin := &NATSBackendPlugin{}
	if err := plugin.Initialize(nil); err != nil {
		t.Fatalf("Failed to initialize plugin: %v", err)
	}

	if err := flexlog.RegisterBackendPlugin(plugin); err != nil {
		t.Fatalf("Failed to register plugin: %v", err)
	}

	// Create a FlexLog instance with NATS backend
	logger := flexlog.New()
	defer logger.CloseAll()

	// Add NATS destination
	if err := logger.AddDestination("nats://localhost:4222/test.flexlog"); err != nil {
		t.Fatalf("Failed to add NATS destination: %v", err)
	}

	// Subscribe to the subject
	messages := make(chan *nats.Msg, 10)
	sub, err := conn.Subscribe("test.flexlog", func(msg *nats.Msg) {
		messages <- msg
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	// Log some messages
	logger.Info("Test info message")
	logger.Error("Test error message")

	// Wait a bit for async processing
	time.Sleep(100 * time.Millisecond)

	// Collect messages
	received := 0
	timeout := time.After(2 * time.Second)
	for received < 2 {
		select {
		case msg := <-messages:
			t.Logf("Received: %s", msg.Data)
			received++
		case <-timeout:
			t.Fatalf("Timeout waiting for messages, received %d/2", received)
		}
	}
}

func TestNATSBackend_Reconnection_Integration(t *testing.T) {
	// This test would require stopping and starting NATS server
	// which is not practical in unit tests
	t.Skip("Reconnection test requires NATS server control")
}