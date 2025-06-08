//go:build integration && debug

package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

// TestNATSConnectionDebug provides detailed connection debugging
func TestNATSConnectionDebug(t *testing.T) {
	servers := []string{
		"nats://localhost:4222",
		"nats://127.0.0.1:4222",
		"localhost:4222",
	}

	t.Log("=== NATS Connection Debug Test ===")
	t.Logf("Testing connection to NATS servers: %v", servers)

	for _, server := range servers {
		t.Logf("\nTrying server: %s", server)

		// Try direct connection
		opts := []nats.Option{
			nats.Name("omni-debug-test"),
			nats.Timeout(2 * time.Second),
			nats.MaxReconnects(3),
			nats.ReconnectWait(1 * time.Second),
			nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
				t.Logf("Disconnected from %s: %v", server, err)
			}),
			nats.ReconnectHandler(func(nc *nats.Conn) {
				t.Logf("Reconnected to %s", server)
			}),
			nats.ClosedHandler(func(nc *nats.Conn) {
				t.Logf("Connection to %s closed", server)
			}),
			nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
				t.Logf("Error on %s: %v", server, err)
			}),
		}

		nc, err := nats.Connect(server, opts...)
		if err != nil {
			t.Logf("Failed to connect to %s: %v", server, err)
			continue
		}

		// Test connection
		t.Logf("Connected to %s successfully", server)
		t.Logf("  - Connected URL: %s", nc.ConnectedUrl())
		t.Logf("  - Server ID: %s", nc.ConnectedServerId())
		t.Logf("  - Max Payload: %d", nc.MaxPayload())

		// Test publish
		testSubject := "debug.test"
		testMsg := []byte("debug test message")

		err = nc.Publish(testSubject, testMsg)
		if err != nil {
			t.Logf("Failed to publish to %s: %v", testSubject, err)
		} else {
			t.Logf("Successfully published to %s", testSubject)
		}

		// Test subscribe
		received := make(chan bool, 1)
		sub, err := nc.Subscribe(testSubject, func(msg *nats.Msg) {
			t.Logf("Received message on %s: %s", msg.Subject, string(msg.Data))
			received <- true
		})
		if err != nil {
			t.Logf("Failed to subscribe to %s: %v", testSubject, err)
		} else {
			defer sub.Unsubscribe()

			// Publish another message
			nc.Publish(testSubject, []byte("test after subscribe"))

			// Wait for message
			select {
			case <-received:
				t.Log("Subscribe/publish test successful")
			case <-time.After(1 * time.Second):
				t.Log("Timeout waiting for subscribed message")
			}
		}

		// Check RTT
		if !nc.IsClosed() {
			start := time.Now()
			err = nc.FlushTimeout(2 * time.Second)
			rtt := time.Since(start)
			if err != nil {
				t.Logf("Flush failed: %v", err)
			} else {
				t.Logf("Round-trip time: %v", rtt)
			}
		}

		nc.Close()
		t.Logf("Closed connection to %s", server)
		break // If we connected successfully, no need to try other addresses
	}
}

// TestNATSBackendDebug provides detailed backend debugging
func TestNATSBackendDebug(t *testing.T) {
	t.Log("=== NATS Backend Debug Test ===")

	configs := []struct {
		name string
		uri  string
	}{
		{"Basic", "nats://localhost:4222/debug.logs"},
		{"With batching", "nats://localhost:4222/debug.batch?batch=10&flush_interval=500"},
		{"With queue", "nats://localhost:4222/debug.queue?queue=workers"},
		{"Sync mode", "nats://localhost:4222/debug.sync?async=false"},
	}

	for _, config := range configs {
		t.Logf("\nTesting configuration: %s", config.name)
		t.Logf("URI: %s", config.uri)

		backend, err := NewNATSBackend(config.uri)
		if err != nil {
			t.Logf("Failed to create backend: %v", err)
			continue
		}

		// Log backend configuration
		t.Logf("Backend configuration:")
		t.Logf("  - Subject: %s", backend.subject)
		t.Logf("  - Queue Group: %s", backend.queueGroup)
		t.Logf("  - Async: %v", backend.async)
		t.Logf("  - Batch Size: %d", backend.batchSize)
		t.Logf("  - Flush Interval: %v", backend.flushInterval)
		t.Logf("  - Format: %s", backend.format)

		// Test write
		testEntry := []byte(fmt.Sprintf(`{"config":"%s","timestamp":"%s"}`, config.name, time.Now().Format(time.RFC3339)))
		n, err := backend.Write(testEntry)
		if err != nil {
			t.Logf("Write failed: %v", err)
		} else {
			t.Logf("Successfully wrote %d bytes", n)
		}

		// Test flush
		err = backend.Flush()
		if err != nil {
			t.Logf("Flush failed: %v", err)
		} else {
			t.Log("Flush successful")
		}

		// Close backend
		err = backend.Close()
		if err != nil {
			t.Logf("Close failed: %v", err)
		} else {
			t.Log("Backend closed successfully")
		}
	}
}

// TestNATSPerformanceDebug tests performance characteristics
func TestNATSPerformanceDebug(t *testing.T) {
	t.Log("=== NATS Performance Debug Test ===")

	backend, err := NewNATSBackend("nats://localhost:4222/perf.test?batch=100")
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Test different message sizes
	sizes := []int{100, 1024, 10240, 102400}
	counts := []int{100, 100, 100, 10}

	for i, size := range sizes {
		count := counts[i]
		msg := make([]byte, size)
		for j := range msg {
			msg[j] = byte('A' + (j % 26))
		}

		start := time.Now()
		totalBytes := 0

		for j := 0; j < count; j++ {
			n, err := backend.Write(msg)
			if err != nil {
				t.Logf("Write error at message %d: %v", j, err)
				break
			}
			totalBytes += n
		}

		backend.Flush()
		duration := time.Since(start)

		throughput := float64(totalBytes) / duration.Seconds() / 1024 / 1024
		msgsPerSec := float64(count) / duration.Seconds()

		t.Logf("Message size: %d bytes, Count: %d", size, count)
		t.Logf("  - Duration: %v", duration)
		t.Logf("  - Throughput: %.2f MB/s", throughput)
		t.Logf("  - Messages/sec: %.2f", msgsPerSec)
		t.Logf("  - Avg latency: %.2f ms", duration.Seconds()*1000/float64(count))
	}
}
