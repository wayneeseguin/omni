//go:build integration

package main

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestNATSRoutingIntegration(t *testing.T) {
	// Test complex routing scenarios with actual NATS server

	// Create multiple backends for different routing rules
	backends := map[string]*NATSBackend{
		"info":    nil,
		"error":   nil,
		"metrics": nil,
		"audit":   nil,
	}

	uris := map[string]string{
		"info":    "nats://localhost:4222/logs.app.info?queue=info-processors",
		"error":   "nats://localhost:4222/logs.app.error?queue=error-processors",
		"metrics": "nats://localhost:4222/metrics.app?batch=50&flush_interval=1000",
		"audit":   "nats://localhost:4222/audit.app.events?format=json",
	}

	// Create backends with retry
	for name, uri := range uris {
		var backend *NATSBackend
		var err error

		// Retry connection a few times
		for i := 0; i < 5; i++ {
			backend, err = NewNATSBackend(uri)
			if err == nil {
				break
			}
			if i < 4 {
				time.Sleep(100 * time.Millisecond)
			}
		}

		if err != nil {
			t.Fatalf("Failed to create %s backend after retries: %v", name, err)
		}
		backends[name] = backend
		defer backend.Close()
	}

	// Set up subscribers to verify routing with retry
	var nc *nats.Conn
	var err error
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
		t.Fatalf("Failed to connect to NATS after retries: %v", err)
	}
	defer nc.Close()

	// Message counters
	var mu sync.Mutex
	counters := map[string]int{
		"info":    0,
		"error":   0,
		"metrics": 0,
		"audit":   0,
	}

	// Subscribe to each subject
	subscriptions := make([]*nats.Subscription, 0)

	// Info subscriber
	sub, err := nc.QueueSubscribe("logs.app.info", "test-info", func(msg *nats.Msg) {
		mu.Lock()
		counters["info"]++
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("Failed to subscribe to info: %v", err)
	}
	subscriptions = append(subscriptions, sub)

	// Error subscriber
	sub, err = nc.QueueSubscribe("logs.app.error", "test-error", func(msg *nats.Msg) {
		mu.Lock()
		counters["error"]++
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("Failed to subscribe to error: %v", err)
	}
	subscriptions = append(subscriptions, sub)

	// Metrics subscriber
	sub, err = nc.Subscribe("metrics.app", func(msg *nats.Msg) {
		mu.Lock()
		counters["metrics"]++
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("Failed to subscribe to metrics: %v", err)
	}
	subscriptions = append(subscriptions, sub)

	// Audit subscriber
	sub, err = nc.Subscribe("audit.app.events", func(msg *nats.Msg) {
		mu.Lock()
		counters["audit"]++
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("Failed to subscribe to audit: %v", err)
	}
	subscriptions = append(subscriptions, sub)

	// Ensure all subscriptions are ready
	if err := nc.Flush(); err != nil {
		t.Fatalf("Failed to flush subscriptions: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	// Send test messages
	testCases := []struct {
		backend string
		count   int
		message string
	}{
		{"info", 5, `{"level":"INFO","message":"Info message %d"}`},
		{"error", 3, `{"level":"ERROR","message":"Error message %d"}`},
		{"metrics", 10, `{"metric":"cpu","value":%d}`},
		{"audit", 2, `{"event":"login","user":"user%d"}`},
	}

	for _, tc := range testCases {
		backend := backends[tc.backend]
		if backend == nil {
			t.Errorf("Backend %s is nil", tc.backend)
			continue
		}
		for i := 0; i < tc.count; i++ {
			msg := fmt.Sprintf(tc.message, i)
			if _, err := backend.Write([]byte(msg)); err != nil {
				t.Errorf("Failed to write to %s backend: %v", tc.backend, err)
			}
		}
		t.Logf("Sent %d messages to %s backend", tc.count, tc.backend)
	}

	// Flush all backends
	for name, backend := range backends {
		if err := backend.Flush(); err != nil {
			t.Errorf("Failed to flush %s backend: %v", name, err)
		}
	}

	// Wait for messages to be delivered - reduced for tests
	time.Sleep(200 * time.Millisecond)

	// Verify counters
	mu.Lock()
	defer mu.Unlock()

	if counters["info"] != 5 {
		t.Errorf("Expected 5 info messages, got %d", counters["info"])
	}
	if counters["error"] != 3 {
		t.Errorf("Expected 3 error messages, got %d", counters["error"])
	}
	if counters["metrics"] != 10 {
		t.Errorf("Expected 10 metrics messages, got %d", counters["metrics"])
	}
	if counters["audit"] != 2 {
		t.Errorf("Expected 2 audit messages, got %d", counters["audit"])
	}

	// Clean up subscriptions
	for _, sub := range subscriptions {
		sub.Unsubscribe()
	}

	t.Logf("Routing test completed: info=%d, error=%d, metrics=%d, audit=%d",
		counters["info"], counters["error"], counters["metrics"], counters["audit"])
}

func TestNATSHierarchicalRouting(t *testing.T) {
	// Test hierarchical subject routing with retry
	var nc *nats.Conn
	var err error
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
		t.Fatalf("Failed to connect to NATS after retries: %v", err)
	}
	defer nc.Close()

	// Set up hierarchical subjects
	subjects := []string{
		"logs.app.service1.info",
		"logs.app.service1.error",
		"logs.app.service2.info",
		"logs.app.service2.error",
		"logs.infra.database.slow",
		"logs.infra.cache.miss",
	}

	// Create backends for each subject with retry
	backends := make(map[string]*NATSBackend)
	for _, subject := range subjects {
		uri := fmt.Sprintf("nats://localhost:4222/%s", subject)

		var backend *NATSBackend
		var err error
		for i := 0; i < 5; i++ {
			backend, err = NewNATSBackend(uri)
			if err == nil {
				break
			}
			if i < 4 {
				time.Sleep(500 * time.Millisecond)
			}
		}

		if err != nil {
			t.Fatalf("Failed to create backend for %s after retries: %v", subject, err)
		}
		backends[subject] = backend
		defer backend.Close()
	}

	// Subscribe with wildcards
	var mu sync.Mutex
	wildcardCounts := map[string]int{
		"all_logs":   0,
		"all_app":    0,
		"all_errors": 0,
		"service1":   0,
		"infra":      0,
	}

	// Subscribe to all logs
	sub1, err := nc.Subscribe("logs.>", func(msg *nats.Msg) {
		mu.Lock()
		wildcardCounts["all_logs"]++
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("Failed to subscribe to logs.>: %v", err)
	}
	defer sub1.Unsubscribe()

	// Subscribe to all app logs
	sub2, err := nc.Subscribe("logs.app.>", func(msg *nats.Msg) {
		mu.Lock()
		wildcardCounts["all_app"]++
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("Failed to subscribe to logs.app.>: %v", err)
	}
	defer sub2.Unsubscribe()

	// Subscribe to all errors
	sub3, err := nc.Subscribe("logs.app.*.error", func(msg *nats.Msg) {
		mu.Lock()
		wildcardCounts["all_errors"]++
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("Failed to subscribe to logs.app.*.error: %v", err)
	}
	defer sub3.Unsubscribe()

	// Subscribe to service1 logs
	sub4, err := nc.Subscribe("logs.app.service1.*", func(msg *nats.Msg) {
		mu.Lock()
		wildcardCounts["service1"]++
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("Failed to subscribe to logs.app.service1.*: %v", err)
	}
	defer sub4.Unsubscribe()

	// Subscribe to infrastructure logs
	sub5, err := nc.Subscribe("logs.infra.>", func(msg *nats.Msg) {
		mu.Lock()
		wildcardCounts["infra"]++
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("Failed to subscribe to logs.infra.>: %v", err)
	}
	defer sub5.Unsubscribe()

	// Ensure subscriptions are ready
	if err := nc.Flush(); err != nil {
		t.Fatalf("Failed to flush subscriptions: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	// Send one message to each subject
	sentCount := 0
	for subject, backend := range backends {
		msg := fmt.Sprintf(`{"subject":"%s","timestamp":"%s"}`, subject, time.Now().Format(time.RFC3339))
		if _, err := backend.Write([]byte(msg)); err != nil {
			t.Errorf("Failed to write to %s: %v", subject, err)
		} else {
			sentCount++
		}
		if err := backend.Flush(); err != nil {
			t.Errorf("Failed to flush %s: %v", subject, err)
		}
	}
	t.Logf("Sent %d messages to NATS", sentCount)

	// Wait for delivery - reduced for tests
	time.Sleep(200 * time.Millisecond)

	// Verify wildcard subscriptions
	mu.Lock()
	defer mu.Unlock()

	t.Logf("Received counts: all_logs=%d, all_app=%d, all_errors=%d, service1=%d, infra=%d",
		wildcardCounts["all_logs"], wildcardCounts["all_app"], wildcardCounts["all_errors"],
		wildcardCounts["service1"], wildcardCounts["infra"])

	if wildcardCounts["all_logs"] != 6 {
		t.Errorf("Expected 6 total log messages, got %d", wildcardCounts["all_logs"])
	}

	if wildcardCounts["all_app"] != 4 {
		t.Errorf("Expected 4 app log messages, got %d", wildcardCounts["all_app"])
	}

	if wildcardCounts["all_errors"] != 2 {
		t.Errorf("Expected 2 error messages, got %d", wildcardCounts["all_errors"])
	}

	if wildcardCounts["service1"] != 2 {
		t.Errorf("Expected 2 service1 messages, got %d", wildcardCounts["service1"])
	}

	if wildcardCounts["infra"] != 2 {
		t.Errorf("Expected 2 infrastructure messages, got %d", wildcardCounts["infra"])
	}

	t.Log("Hierarchical routing test completed successfully")
}
